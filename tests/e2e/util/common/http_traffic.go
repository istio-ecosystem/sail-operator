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
	"sync"
	"sync/atomic"
	"time"

	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/kubectl"
	. "github.com/onsi/ginkgo/v2"
)

// CheckHTTPConnectivity performs an HTTP GET request from a source pod to a target URL
// and verifies that the response matches the expected HTTP status code.
// The targetURL can be a short name (e.g., "httpbin:8000/get") or a fully qualified URL.
// The expectedStatus should be the expected HTTP status code (e.g., "200", "403", "503").
// Returns nil if the check succeeds, or an error describing what failed.
// Designed for use with Eventually: Eventually(func() error { return CheckHTTPConnectivity(...) }).Should(Succeed())
func CheckHTTPConnectivity(
	k kubectl.Kubectl,
	namespace string,
	podName string,
	containerName string,
	targetURL string,
	timeoutSeconds int,
	expectedStatus string,
) error {
	cmd := fmt.Sprintf("curl -s -o /dev/null -w '%%{http_code}' %s --max-time %d", targetURL, timeoutSeconds)
	status, err := k.WithNamespace(namespace).Exec(podName, containerName, cmd)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	if status != expectedStatus {
		return fmt.Errorf("expected HTTP status %s, got %s", expectedStatus, status)
	}
	return nil
}

// HTTPTrafficStats tracks continuous HTTP traffic statistics with thread-safe counters
type HTTPTrafficStats struct {
	totalRequests   atomic.Int64
	failedRequests  atomic.Int64
	successRequests atomic.Int64
	mu              sync.Mutex
	errors          []string
}

// RecordSuccess increments the success counter
func (ts *HTTPTrafficStats) RecordSuccess() {
	ts.totalRequests.Add(1)
	ts.successRequests.Add(1)
}

// RecordFailure increments the failure counter and stores the error message
func (ts *HTTPTrafficStats) RecordFailure(errMsg string) {
	ts.totalRequests.Add(1)
	ts.failedRequests.Add(1)
	ts.mu.Lock()
	ts.errors = append(ts.errors, errMsg)
	ts.mu.Unlock()
}

// GetStats returns the current statistics (total, success, failed, errors)
func (ts *HTTPTrafficStats) GetStats() (total, success, failed int64, errors []string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return ts.totalRequests.Load(), ts.successRequests.Load(), ts.failedRequests.Load(), append([]string{}, ts.errors...)
}

// StartContinuousHTTPTraffic starts sending continuous HTTP GET requests in the background.
// If existingStats is nil, creates a new stats object. Otherwise reuses the existing one.
// Returns the stats tracker and a cancel function to stop the traffic.
// The kubectl instance k must be accessible (typically via closure in test context).
func StartContinuousHTTPTraffic(
	ctx context.Context,
	k kubectl.Kubectl,
	namespace string,
	clientPod string,
	containerName string,
	targetURL string,
	interval time.Duration,
	existingStats *HTTPTrafficStats,
) (*HTTPTrafficStats, context.CancelFunc) {
	stats := existingStats
	if stats == nil {
		stats = &HTTPTrafficStats{}
	}

	trafficCtx, cancel := context.WithCancel(ctx)

	go func() {
		defer GinkgoRecover()
		Log(fmt.Sprintf("[Traffic] Starting continuous traffic from %s to %s (interval: %v)", clientPod, targetURL, interval))

		// sendRequest spawns a goroutine to send a request without blocking the ticker
		sendRequest := func() {
			go func() {
				defer GinkgoRecover()
				defer func() {
					if r := recover(); r != nil {
						errMsg := fmt.Sprintf("panic in sendRequest: %v", r)
						stats.RecordFailure(errMsg)
						Log(fmt.Sprintf("[Traffic] %s", errMsg))
					}
				}()

				cmd := fmt.Sprintf("curl -s -o /dev/null -w '%%{http_code}' %s --max-time 2", targetURL)
				output, err := k.WithNamespace(namespace).Exec(clientPod, containerName, cmd)
				if err != nil {
					errMsg := fmt.Sprintf("request failed: %v", err)
					stats.RecordFailure(errMsg)
					Log(fmt.Sprintf("[Traffic] %s", errMsg))
				} else if output != "200" {
					errMsg := fmt.Sprintf("unexpected status code: %s", output)
					stats.RecordFailure(errMsg)
					Log(fmt.Sprintf("[Traffic] %s", errMsg))
				} else {
					stats.RecordSuccess()
				}
			}()
		}

		// Send first request immediately
		sendRequest()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-trafficCtx.Done():
				Log("[Traffic] Stopping continuous traffic (context cancelled)")
				return
			case <-ticker.C:
				sendRequest()
			}
		}
	}()

	return stats, cancel
}
