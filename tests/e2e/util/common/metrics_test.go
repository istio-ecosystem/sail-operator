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

package common

import (
	"strings"
	"testing"
)

func TestMetricReaders(t *testing.T) {
	snapshot, err := parseMetrics(strings.NewReader(`
# TYPE process_resident_memory_bytes gauge
process_resident_memory_bytes 1048576
# TYPE rest_client_requests_total counter
rest_client_requests_total{method="GET"} 4
# TYPE controller_runtime_reconcile_time_seconds histogram
controller_runtime_reconcile_time_seconds_bucket{le="+Inf"} 3
controller_runtime_reconcile_time_seconds_sum 1.5
controller_runtime_reconcile_time_seconds_count 3
`))
	if err != nil {
		t.Fatalf("parse metrics: %v", err)
	}

	memory, err := snapshot.GetGaugeValue("process_resident_memory_bytes", nil)
	if err != nil || memory != 1048576 {
		t.Errorf("GetGaugeValue() = (%v, %v), want (1048576, nil)", memory, err)
	}

	requests, err := snapshot.GetCounterValue("rest_client_requests_total", map[string]string{"method": "POST"})
	if err != nil || requests != 0 {
		t.Errorf("GetCounterValue() for missing label series = (%v, %v), want (0, nil)", requests, err)
	}

	sum, count, err := snapshot.GetHistogramSumAndCount("controller_runtime_reconcile_time_seconds", nil)
	if err != nil || sum != 1.5 || count != 3 {
		t.Errorf("GetHistogramSumAndCount() = (%v, %v, %v), want (1.5, 3, nil)", sum, count, err)
	}

	if _, err := snapshot.GetGaugeValue("missing_metric", nil); err == nil {
		t.Error("GetGaugeValue() for missing metric returned no error")
	}
	if _, err := CounterDelta(snapshot, snapshot, "missing_metric", nil); err == nil {
		t.Error("CounterDelta() for missing metric returned no error")
	}
}
