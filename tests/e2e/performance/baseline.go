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

package performance

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"

	"github.com/istio-ecosystem/sail-operator/pkg/env"
	"github.com/istio-ecosystem/sail-operator/pkg/test/project"
)

func baselineFilePath() string {
	if p := env.Get("PERF_BASELINE_FILE", ""); p != "" {
		return p
	}
	return filepath.Join(project.RootDir, "tests", "e2e", "performance", "baseline.json")
}

// SuiteBaseline holds the baseline allocation and CPU values for one E2E suite.
type SuiteBaseline struct {
	AllocBytes   int64   `json:"allocBytes"`
	AllocObjects int64   `json:"allocObjects"`
	CPUSeconds   float64 `json:"cpuSeconds"`
}

// SuiteThresholds holds the maximum-acceptable values for one suite derived by
// multiplying the baseline by a degradation factor.
type SuiteThresholds struct {
	MaxAllocBytes   int64
	MaxAllocObjects int64
	MaxCPUSeconds   float64
}

// Baseline is the schema of baseline.json.
type Baseline struct {
	CapturedAt  string                   `json:"capturedAt"`
	Environment string                   `json:"environment"`
	Suites      map[string]SuiteBaseline `json:"suites"`
}

// LoadBaseline reads the baseline file. Path is controlled by PERF_BASELINE_FILE
// env var; falls back to the committed baseline.json in the repo.
func LoadBaseline() (*Baseline, error) {
	data, err := os.ReadFile(baselineFilePath())
	if os.IsNotExist(err) {
		b := defaultBaseline
		return &b, nil
	}
	if err != nil {
		return nil, err
	}
	var b Baseline
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

// ThresholdsFor returns the maximum-acceptable values for suiteName at the given factor.
// Returns nil if the suite has no baseline entry (test will be skipped).
func (b *Baseline) ThresholdsFor(suiteName string, factor float64) *SuiteThresholds {
	sb, ok := b.Suites[suiteName]
	if !ok {
		return nil
	}
	return &SuiteThresholds{
		MaxAllocBytes:   int64(math.Ceil(float64(sb.AllocBytes) * factor)),
		MaxAllocObjects: int64(math.Ceil(float64(sb.AllocObjects) * factor)),
		MaxCPUSeconds:   sb.CPUSeconds * factor,
	}
}

// defaultBaseline is used when no baseline.json file exists yet.
// All suites start with generous initial values; replace after first real run.
var defaultBaseline = Baseline{
	CapturedAt:  "initial-defaults",
	Environment: "unknown",
	Suites: map[string]SuiteBaseline{
		"ambient":           {AllocBytes: 500_000_000, AllocObjects: 3_000_000, CPUSeconds: 5.0},
		"controlplane":      {AllocBytes: 400_000_000, AllocObjects: 2_500_000, CPUSeconds: 4.0},
		"dualstack":         {AllocBytes: 400_000_000, AllocObjects: 2_500_000, CPUSeconds: 4.0},
		"gatewaycontroller": {AllocBytes: 300_000_000, AllocObjects: 2_000_000, CPUSeconds: 3.0},
		"library":           {AllocBytes: 300_000_000, AllocObjects: 2_000_000, CPUSeconds: 3.0},
		"migration":         {AllocBytes: 500_000_000, AllocObjects: 3_000_000, CPUSeconds: 5.0},
		"monitoring":        {AllocBytes: 300_000_000, AllocObjects: 2_000_000, CPUSeconds: 3.0},
		"multicluster":      {AllocBytes: 600_000_000, AllocObjects: 4_000_000, CPUSeconds: 6.0},
		"multicontrolplane": {AllocBytes: 500_000_000, AllocObjects: 3_000_000, CPUSeconds: 5.0},
		"operator":          {AllocBytes: 200_000_000, AllocObjects: 1_500_000, CPUSeconds: 2.0},
	},
}
