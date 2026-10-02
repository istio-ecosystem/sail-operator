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

// SuiteProfile is the allocation delta for one E2E suite, written to
// $ARTIFACTS/profiles/<suite>.json and read by the performance suite.
type SuiteProfile struct {
	Suite        string    `json:"suite"`
	CapturedAt   time.Time `json:"capturedAt"`
	AllocBytes   int64     `json:"allocBytes"`
	AllocObjects int64     `json:"allocObjects"`
	CPUSeconds   float64   `json:"cpuSeconds"`
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

	beforeHeap, beforeCPU, err := snapshot(operatorNs, deploymentName, token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[profiling] WARNING: before-snapshot failed for suite %q: %v\n", suiteName, err)
		fn()
		return
	}

	fn()

	afterHeap, afterCPU, err := snapshot(operatorNs, deploymentName, token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[profiling] WARNING: after-snapshot failed for suite %q: %v\n", suiteName, err)
		return
	}

	delta := common.DeltaHeapMetrics(beforeHeap, afterHeap)
	prof := SuiteProfile{
		Suite:        suiteName,
		CapturedAt:   time.Now().UTC(),
		AllocBytes:   delta.AllocBytes,
		AllocObjects: delta.AllocObjects,
		CPUSeconds:   afterCPU - beforeCPU,
	}

	if err := saveProfile(suiteName, prof); err != nil {
		fmt.Fprintf(os.Stderr, "[profiling] WARNING: could not save profile for suite %q: %v\n", suiteName, err)
	} else {
		fmt.Fprintf(os.Stdout, "[profiling] suite=%s allocBytes=%d allocObjects=%d cpuSeconds=%.3f\n",
			suiteName, prof.AllocBytes, prof.AllocObjects, prof.CPUSeconds)
	}
}

func snapshot(operatorNs, deploymentName, metricsToken string) (common.HeapMetrics, float64, error) {
	raw, err := common.FetchHeapProfile(operatorNs, deploymentName)
	if err != nil {
		return common.HeapMetrics{}, 0, fmt.Errorf("fetching heap profile: %w", err)
	}
	heap, err := common.ParseHeapMetrics(raw)
	if err != nil {
		return common.HeapMetrics{}, 0, fmt.Errorf("parsing heap profile: %w", err)
	}

	metrics, err := common.ScrapeOperatorMetrics(operatorNs, deploymentName, metricsToken)
	if err != nil {
		return heap, 0, fmt.Errorf("scraping metrics: %w", err)
	}
	cpuSeconds, err := metrics.GetCounterValue("process_cpu_seconds_total", nil)
	if err != nil {
		return heap, 0, fmt.Errorf("reading process CPU metric: %w", err)
	}
	return heap, cpuSeconds, nil
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
