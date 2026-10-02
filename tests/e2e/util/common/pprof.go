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
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/google/pprof/profile"
)

// HeapMetrics holds allocation counters extracted from a Go heap pprof profile.
type HeapMetrics struct {
	AllocBytes   int64
	AllocObjects int64
}

// FetchHeapProfile port-forwards to the operator pprof endpoint on pprofPort and
// returns the raw heap profile bytes.
func FetchHeapProfile(operatorNs, deploymentName string) ([]byte, error) {
	localPort, err := freePort()
	if err != nil {
		return nil, fmt.Errorf("finding free port: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	args := []string{
		"port-forward", "-n", operatorNs,
		"deploy/" + deploymentName,
		fmt.Sprintf("%d:6060", localPort),
	}
	if kc := os.Getenv("KUBECONFIG"); kc != "" {
		args = append(args, "--kubeconfig", kc)
	}
	pfCmd := exec.CommandContext(ctx, "kubectl", args...)
	if err := pfCmd.Start(); err != nil {
		return nil, fmt.Errorf("starting pprof port-forward: %w", err)
	}
	defer func() {
		_ = pfCmd.Process.Kill()
		_ = pfCmd.Wait()
	}()

	if err := waitForPort(localPort, 10*time.Second); err != nil {
		return nil, fmt.Errorf("pprof port-forward not ready on :%d: %w", localPort, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("http://127.0.0.1:%d/debug/pprof/heap", localPort), nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching heap profile: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("pprof endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// ParseHeapMetrics parses a raw heap profile and returns cumulative alloc_space
// and alloc_objects — the two most stable regression signals.
func ParseHeapMetrics(data []byte) (HeapMetrics, error) {
	p, err := profile.ParseData(data)
	if err != nil {
		return HeapMetrics{}, fmt.Errorf("parsing heap profile: %w", err)
	}

	var allocBytesIdx, allocObjectsIdx int = -1, -1
	for i, st := range p.SampleType {
		switch st.Type {
		case "alloc_space":
			allocBytesIdx = i
		case "alloc_objects":
			allocObjectsIdx = i
		}
	}
	if allocBytesIdx < 0 || allocObjectsIdx < 0 {
		return HeapMetrics{}, fmt.Errorf("heap profile missing alloc_space or alloc_objects sample types")
	}

	var m HeapMetrics
	for _, s := range p.Sample {
		m.AllocBytes += s.Value[allocBytesIdx]
		m.AllocObjects += s.Value[allocObjectsIdx]
	}
	return m, nil
}

// DeltaHeapMetrics returns the allocation delta between a before and after snapshot.
// Both must come from the same process (cumulative counters only go up).
func DeltaHeapMetrics(before, after HeapMetrics) HeapMetrics {
	return HeapMetrics{
		AllocBytes:   after.AllocBytes - before.AllocBytes,
		AllocObjects: after.AllocObjects - before.AllocObjects,
	}
}
