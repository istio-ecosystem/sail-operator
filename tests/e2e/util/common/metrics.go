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
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
)

// MetricsSnapshot is a parsed Prometheus metric family map keyed by metric name.
type MetricsSnapshot map[string]*dto.MetricFamily

// ScrapeOperatorMetrics port-forwards to the operator deployment's metrics endpoint
// (port 8443, TLS with skip-verify) and returns a parsed MetricsSnapshot.
// The caller must supply a valid bearer token that has the metrics-reader ClusterRole.
func ScrapeOperatorMetrics(operatorNs, deploymentName, bearerToken string) (MetricsSnapshot, error) {
	body, err := portForwardedGet(operatorNs, deploymentName, 8443, "https", "/metrics", bearerToken)
	if err != nil {
		return nil, fmt.Errorf("scraping metrics: %w", err)
	}
	return parseMetrics(bytes.NewReader(body))
}

// GetServiceAccountToken returns a short-lived bearer token for the named service account.
func GetServiceAccountToken(namespace, serviceAccountName string) (string, error) {
	const reqBody = `{"apiVersion":"authentication.k8s.io/v1","kind":"TokenRequest"}`

	f, err := os.CreateTemp("", "token-request-*.json")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(reqBody); err != nil {
		return "", err
	}
	f.Close()

	args := []string{
		"create", "--raw",
		fmt.Sprintf("/api/v1/namespaces/%s/serviceaccounts/%s/token", namespace, serviceAccountName),
		"-f", f.Name(),
	}
	if kc := os.Getenv("KUBECONFIG"); kc != "" {
		args = append(args, "--kubeconfig", kc)
	}
	out, err := exec.Command("kubectl", args...).Output()
	if err != nil {
		return "", fmt.Errorf("kubectl token request for %s/%s: %w", namespace, serviceAccountName, err)
	}

	var tr struct {
		Status struct {
			Token string `json:"token"`
		} `json:"status"`
	}
	if err := json.Unmarshal(out, &tr); err != nil {
		return "", fmt.Errorf("parsing token response: %w", err)
	}
	return tr.Status.Token, nil
}

// GetGaugeValue returns the sum of gauge values from the snapshot that match all label filters.
// Pass an empty map to match all series regardless of labels. It returns an error when the
// metric is absent or is not a gauge. No matching series returns zero.
func (s MetricsSnapshot) GetGaugeValue(metricName string, labels map[string]string) (float64, error) {
	mf, ok := s[metricName]
	if !ok {
		return 0, fmt.Errorf("gauge metric %q is missing from snapshot", metricName)
	}
	if mf.GetType() != dto.MetricType_GAUGE {
		return 0, fmt.Errorf("metric %q has type %s, expected GAUGE", metricName, mf.GetType())
	}
	var sum float64
	for _, m := range mf.Metric {
		if matchesLabels(m.Label, labels) {
			sum += m.GetGauge().GetValue()
		}
	}
	return sum, nil
}

// GetCounterValue returns the sum of counter values from the snapshot that match all label filters.
// It returns an error when the metric is absent or is not a counter. No matching series returns zero.
func (s MetricsSnapshot) GetCounterValue(metricName string, labels map[string]string) (float64, error) {
	mf, ok := s[metricName]
	if !ok {
		return 0, fmt.Errorf("counter metric %q is missing from snapshot", metricName)
	}
	if mf.GetType() != dto.MetricType_COUNTER {
		return 0, fmt.Errorf("metric %q has type %s, expected COUNTER", metricName, mf.GetType())
	}
	var sum float64
	for _, m := range mf.Metric {
		if matchesLabels(m.Label, labels) {
			sum += m.GetCounter().GetValue()
		}
	}
	return sum, nil
}

// CounterDelta returns the increase in a counter metric between two snapshots.
// It returns an error if the counter decreased (indicating an operator restart /
// counter reset during the measurement window), which would produce an invalid delta.
func CounterDelta(before, after MetricsSnapshot, metric string, labels map[string]string) (float64, error) {
	b, err := before.GetCounterValue(metric, labels)
	if err != nil {
		return 0, fmt.Errorf("reading counter before measurement: %w", err)
	}
	a, err := after.GetCounterValue(metric, labels)
	if err != nil {
		return 0, fmt.Errorf("reading counter after measurement: %w", err)
	}
	if a < b {
		return 0, fmt.Errorf("counter %q decreased from %.0f to %.0f — operator may have restarted during the measurement window", metric, b, a)
	}
	return a - b, nil
}

// GetHistogramSumAndCount returns the sum and sample count for a histogram metric
// matching the given labels. Use the count as the denominator when computing averages
// rather than a separately-registered counter, as they are guaranteed to stay in sync.
// It returns an error when the metric is absent or is not a histogram. No matching
// series returns zero.
func (s MetricsSnapshot) GetHistogramSumAndCount(metricName string, labels map[string]string) (sum float64, count uint64, err error) {
	mf, ok := s[metricName]
	if !ok {
		return 0, 0, fmt.Errorf("histogram metric %q is missing from snapshot", metricName)
	}
	if mf.GetType() != dto.MetricType_HISTOGRAM {
		return 0, 0, fmt.Errorf("metric %q has type %s, expected HISTOGRAM", metricName, mf.GetType())
	}
	for _, m := range mf.Metric {
		if matchesLabels(m.Label, labels) {
			h := m.GetHistogram()
			sum += h.GetSampleSum()
			count += h.GetSampleCount()
		}
	}
	return sum, count, nil
}

// portForwardedGet starts a kubectl port-forward from deploy/<deploymentName> in operatorNs,
// forwarding <targetPort> to a random local port, then issues a GET and returns the body.
// Use scheme "http" or "https" (https skips TLS verification for local port-forwards).
// Pass a non-empty bearerToken to set an Authorization header.
func portForwardedGet(operatorNs, deploymentName string, targetPort int, scheme, path, bearerToken string) ([]byte, error) {
	localPort, err := freePort()
	if err != nil {
		return nil, fmt.Errorf("finding free port: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	args := []string{
		"port-forward", "-n", operatorNs,
		"deploy/" + deploymentName,
		fmt.Sprintf("%d:%d", localPort, targetPort),
	}
	if kc := os.Getenv("KUBECONFIG"); kc != "" {
		args = append(args, "--kubeconfig", kc)
	}
	pfCmd := exec.CommandContext(ctx, "kubectl", args...)
	if err := pfCmd.Start(); err != nil {
		return nil, fmt.Errorf("starting port-forward: %w", err)
	}
	defer func() {
		_ = pfCmd.Process.Kill()
		_ = pfCmd.Wait()
	}()

	if err := waitForPort(localPort, 10*time.Second); err != nil {
		return nil, fmt.Errorf("port-forward not ready on :%d: %w", localPort, err)
	}

	httpClient := http.DefaultClient
	if scheme == "https" {
		httpClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // intentional for local port-forward
			},
			Timeout: 10 * time.Second,
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s://127.0.0.1:%d%s", scheme, localPort, path), nil)
	if err != nil {
		return nil, err
	}
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("endpoint returned %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func parseMetrics(r io.Reader) (MetricsSnapshot, error) {
	parser := expfmt.NewTextParser(model.UTF8Validation)
	families, err := parser.TextToMetricFamilies(r)
	// expfmt returns io.EOF on clean end-of-input; that is not a real error.
	if err != nil && len(families) == 0 {
		return nil, err
	}
	return MetricsSnapshot(families), nil
}

func matchesLabels(pairs []*dto.LabelPair, filters map[string]string) bool {
	for k, v := range filters {
		found := false
		for _, p := range pairs {
			if p.GetName() == k && p.GetValue() == v {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func freePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port, nil
}

func waitForPort(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("port %d not ready after %s", port, timeout)
}
