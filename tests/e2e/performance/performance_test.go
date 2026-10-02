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

// Performance tests compare per-suite pprof profiles captured during the E2E run
// against committed baseline values. No new workload is generated here; the
// performance suite relies on profiling.WrapSuite having been called in every
// other suite to write $ARTIFACTS/profiles/<suite>.json files.
//
// The performance package runs last alphabetically, so by the time these tests
// execute every other suite has already completed and its profile written.
//
// # Running
//
//	PPROF_ENABLED=true make test.e2e.kind
//
// # Updating the baseline
//
// Run the full E2E suite with PPROF_ENABLED=true, read the actual values from
// the test output, update tests/e2e/performance/baseline.json, and commit.
//
// # Environment variables
//
//	PERF_DEGRADATION_FACTOR  – multiplier applied to baseline values (default: 1.2)
//	PERF_BASELINE_FILE       – path to an alternative baseline.json
package performance

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/istio-ecosystem/sail-operator/pkg/env"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/profiling"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// suites is the ordered list of E2E suites that contribute profiles.
// Matches the suite names passed to profiling.WrapSuite in each *_suite_test.go.
var suites = []string{
	"ambient",
	"controlplane",
	"dualstack",
	"gatewaycontroller",
	"library",
	"migration",
	"monitoring",
	"multicluster",
	"multicontrolplane",
	"operator",
}

var _ = Describe("Performance", Label("performance", "slow"), Ordered, ContinueOnFailure, func() {
	SetDefaultEventuallyTimeout(time.Duration(env.GetInt("DEFAULT_TEST_TIMEOUT", 300)) * time.Second)

	var (
		degradationFactor float64
		baseline          *Baseline
		profiles          map[string]profiling.SuiteProfile
	)

	BeforeAll(func() {
		if os.Getenv("PPROF_ENABLED") != "true" {
			Skip("PPROF_ENABLED is not set; skipping profile comparison (re-run with PPROF_ENABLED=true)")
		}

		degradationFactor = 1.2
		if raw := env.Get("PERF_DEGRADATION_FACTOR", ""); raw != "" {
			if f, err := strconv.ParseFloat(raw, 64); err == nil && f > 0 {
				degradationFactor = f
			}
		}

		By("loading performance baseline")
		var err error
		baseline, err = LoadBaseline()
		Expect(err).NotTo(HaveOccurred(), "Failed to load performance baseline")
		GinkgoWriter.Printf("Baseline loaded (capturedAt=%s env=%s degradationFactor=%.2f)\n",
			baseline.CapturedAt, baseline.Environment, degradationFactor)

		By("loading suite profiles from $ARTIFACTS/profiles/")
		profiles, err = loadSuiteProfiles()
		Expect(err).NotTo(HaveOccurred(), "Failed to load suite profiles")
		GinkgoWriter.Printf("Loaded %d suite profile(s)\n", len(profiles))
	})

	for _, suite := range suites {
		It(fmt.Sprintf("%s suite allocation and CPU are within baseline", suite), func() {
			prof, ok := profiles[suite]
			if !ok {
				Skip(fmt.Sprintf("no profile found for suite %q (was PPROF_ENABLED=true during that suite?)", suite))
			}

			thresholds := baseline.ThresholdsFor(suite, degradationFactor)
			if thresholds == nil {
				Skip(fmt.Sprintf("no baseline entry for suite %q", suite))
			}

			GinkgoWriter.Printf("--- %s ACTUAL VALUES ---\n", suite)
			GinkgoWriter.Printf("  allocBytes=%d   (threshold=%d)\n", prof.AllocBytes, thresholds.MaxAllocBytes)
			GinkgoWriter.Printf("  allocObjects=%d (threshold=%d)\n", prof.AllocObjects, thresholds.MaxAllocObjects)
			GinkgoWriter.Printf("  cpuSeconds=%.3f (threshold=%.3f)\n", prof.CPUSeconds, thresholds.MaxCPUSeconds)

			Expect(prof.AllocBytes).To(BeNumerically("<=", thresholds.MaxAllocBytes),
				"suite %q: heap allocations exceeded baseline×%.2f", suite, degradationFactor)
			Expect(prof.AllocObjects).To(BeNumerically("<=", thresholds.MaxAllocObjects),
				"suite %q: alloc objects exceeded baseline×%.2f", suite, degradationFactor)
			Expect(prof.CPUSeconds).To(BeNumerically("<=", thresholds.MaxCPUSeconds),
				"suite %q: CPU seconds exceeded baseline×%.2f", suite, degradationFactor)
		})
	}
})

// loadSuiteProfiles scans $ARTIFACTS/profiles/ and loads all <suite>.json files.
func loadSuiteProfiles() (map[string]profiling.SuiteProfile, error) {
	artifactsDir := env.Get("ARTIFACTS", "/tmp/artifacts")
	dir := filepath.Join(artifactsDir, "profiles")

	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return map[string]profiling.SuiteProfile{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading profiles dir %s: %w", dir, err)
	}

	result := make(map[string]profiling.SuiteProfile, len(entries))
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", e.Name(), err)
		}
		var p profiling.SuiteProfile
		if err := json.Unmarshal(data, &p); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", e.Name(), err)
		}
		result[p.Suite] = p
	}
	return result, nil
}
