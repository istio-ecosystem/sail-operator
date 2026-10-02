//go:build e2e

// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package profiling provides per-suite pprof collection for E2E tests.
// Each suite calls WrapSuite around RunSpecs. When PPROF_ENABLED=true the
// wrapper captures a heap snapshot before and after the suite, computes the
// allocation delta, and writes a SuiteProfile JSON file to $ARTIFACTS/profiles/.
// The performance suite reads those files and compares them against the baseline.
package profiling

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/istio-ecosystem/sail-operator/pkg/env"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
)

// SuiteProfile is the metric delta for one E2E suite, written to
// $ARTIFACTS/profiles/<suite>.json and read by the performance suite.
type SuiteProfile struct {
	Suite         string    `json:"suite"`
	CapturedAt    time.Time `json:"capturedAt"`
	AllocBytes    int64     `json:"allocBytes"`
	AllocObjects  int64     `json:"allocObjects"`
	InuseBytes    int64     `json:"inuseBytes"`
	InuseObjects  int64     `json:"inuseObjects"`
	CPUSeconds    float64   `json:"cpuSeconds"`
	APICallsPatch int64     `json:"apiCallsPatch"`
}

type snapshotResult struct {
	heap       common.HeapMetrics
	cpuSecs    float64
	patchCalls int64
}

// WrapSuite runs fn (which should call RunSpecs) and, when PPROF_ENABLED=true,
// captures heap and CPU metrics before and after to produce a SuiteProfile.
// When PPROF_ENABLED is not set the wrapper is a no-op pass-through.
func WrapSuite(suiteName string, fn func()) {
	if os.Getenv("PPROF_ENABLED") != "true" {
		fn()
		return
	}

	operatorNs := common.OperatorNamespace
	deploymentName := env.Get("DEPLOYMENT_NAME", "sail-operator")

	token, err := common.GetServiceAccountToken(operatorNs, deploymentName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[profiling] WARNING: cannot get metrics token for suite %q: %v\n", suiteName, err)
		fn()
		return
	}

	before, err := snapshot(operatorNs, deploymentName, token, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[profiling] WARNING: before-snapshot failed for suite %q: %v\n", suiteName, err)
		fn()
		return
	}

	fn()

	after, err := snapshot(operatorNs, deploymentName, token, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[profiling] WARNING: after-snapshot failed for suite %q: %v\n", suiteName, err)
		return
	}

	delta := common.DeltaHeapMetrics(before.heap, after.heap)
	prof := SuiteProfile{
		Suite:         suiteName,
		CapturedAt:    time.Now().UTC(),
		AllocBytes:    delta.AllocBytes,
		AllocObjects:  delta.AllocObjects,
		InuseBytes:    delta.InuseBytes,
		InuseObjects:  delta.InuseObjects,
		CPUSeconds:    after.cpuSecs - before.cpuSecs,
		APICallsPatch: after.patchCalls - before.patchCalls,
	}

	if err := saveProfile(suiteName, prof); err != nil {
		fmt.Fprintf(os.Stderr, "[profiling] WARNING: could not save profile for suite %q: %v\n", suiteName, err)
	} else {
		fmt.Fprintf(os.Stdout,
			"[profiling] suite=%s allocBytes=%d allocObjects=%d inuseBytes=%d inuseObjects=%d cpuSeconds=%.3f apiCallsPatch=%d\n",
			suiteName, prof.AllocBytes, prof.AllocObjects, prof.InuseBytes, prof.InuseObjects, prof.CPUSeconds, prof.APICallsPatch)
	}
}

func snapshot(operatorNs, deploymentName, metricsToken string, forceGC bool) (snapshotResult, error) {
	raw, err := common.FetchHeapProfile(operatorNs, deploymentName, forceGC)
	if err != nil {
		return snapshotResult{}, fmt.Errorf("fetching heap profile: %w", err)
	}
	heap, err := common.ParseHeapMetrics(raw)
	if err != nil {
		return snapshotResult{}, fmt.Errorf("parsing heap profile: %w", err)
	}

	metrics, err := common.ScrapeOperatorMetrics(operatorNs, deploymentName, metricsToken)
	if err != nil {
		return snapshotResult{heap: heap}, fmt.Errorf("scraping metrics: %w", err)
	}
	cpuSeconds, err := metrics.GetCounterValue("process_cpu_seconds_total", nil)
	if err != nil {
		return snapshotResult{heap: heap}, fmt.Errorf("reading process CPU metric: %w", err)
	}
	patchCalls, err := metrics.GetCounterValue("rest_client_requests_total", map[string]string{"method": "PATCH"})
	if err != nil {
		// PATCH counter may be absent if no PATCH calls were made yet; treat as 0
		patchCalls = 0
	}
	return snapshotResult{heap: heap, cpuSecs: cpuSeconds, patchCalls: int64(patchCalls)}, nil
}

func saveProfile(suiteName string, prof SuiteProfile) error {
	artifactsDir := os.Getenv("ARTIFACTS")
	if artifactsDir == "" {
		artifactsDir = os.TempDir()
	}
	dir := filepath.Join(artifactsDir, "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(prof, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, suiteName+".json"), data, 0o644)
}
