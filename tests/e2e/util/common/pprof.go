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
	"fmt"

	"github.com/google/pprof/profile"
)

// HeapMetrics holds allocation and live-heap counters extracted from a Go heap pprof profile.
type HeapMetrics struct {
	AllocBytes   int64
	AllocObjects int64
	InuseBytes   int64
	InuseObjects int64
}

// FetchHeapProfile port-forwards to the operator pprof endpoint and returns the raw
// heap profile bytes. When forceGC is true, ?gc=1 is appended to the request URL so
// the runtime performs a full GC before sampling — use this for the after-snapshot to
// get a clean picture of truly retained (live) heap.
func FetchHeapProfile(operatorNs, deploymentName string, forceGC bool) ([]byte, error) {
	path := "/debug/pprof/heap"
	if forceGC {
		path += "?gc=1"
	}
	data, err := portForwardedGet(operatorNs, deploymentName, 6060, "http", path, "")
	if err != nil {
		return nil, fmt.Errorf("fetching heap profile: %w", err)
	}
	return data, nil
}

// ParseHeapMetrics parses a raw heap profile and returns cumulative alloc counters
// (alloc_space, alloc_objects) and live-heap counters (inuse_space, inuse_objects).
func ParseHeapMetrics(data []byte) (HeapMetrics, error) {
	p, err := profile.ParseData(data)
	if err != nil {
		return HeapMetrics{}, fmt.Errorf("parsing heap profile: %w", err)
	}

	idx := map[string]int{
		"alloc_space": -1, "alloc_objects": -1,
		"inuse_space": -1, "inuse_objects": -1,
	}
	for i, st := range p.SampleType {
		if _, ok := idx[st.Type]; ok {
			idx[st.Type] = i
		}
	}
	if idx["alloc_space"] < 0 || idx["alloc_objects"] < 0 {
		return HeapMetrics{}, fmt.Errorf("heap profile missing alloc_space or alloc_objects sample types")
	}

	var m HeapMetrics
	for _, s := range p.Sample {
		m.AllocBytes += s.Value[idx["alloc_space"]]
		m.AllocObjects += s.Value[idx["alloc_objects"]]
		if idx["inuse_space"] >= 0 {
			m.InuseBytes += s.Value[idx["inuse_space"]]
		}
		if idx["inuse_objects"] >= 0 {
			m.InuseObjects += s.Value[idx["inuse_objects"]]
		}
	}
	return m, nil
}

// DeltaHeapMetrics returns the delta between a before and after snapshot.
// AllocBytes/AllocObjects are cumulative (monotonically increasing); InuseBytes/InuseObjects
// are point-in-time live heap, so their delta represents net memory growth during the suite.
func DeltaHeapMetrics(before, after HeapMetrics) HeapMetrics {
	return HeapMetrics{
		AllocBytes:   after.AllocBytes - before.AllocBytes,
		AllocObjects: after.AllocObjects - before.AllocObjects,
		InuseBytes:   after.InuseBytes - before.InuseBytes,
		InuseObjects: after.InuseObjects - before.InuseObjects,
	}
}
