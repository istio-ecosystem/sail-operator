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
// WITHOUT WARRANTIES OR Condition OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"context"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/kubectl"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DataplaneMode represents the Istio dataplane mode
type DataplaneMode string

const (
	// DataplaneModeSidecar represents sidecar mode (proxy injected as sidecar container)
	DataplaneModeSidecar DataplaneMode = "sidecar"
	// DataplaneModeAmbient represents ambient mode (proxy in ZTunnel DaemonSet)
	DataplaneModeAmbient DataplaneMode = "ambient"
)

// WorkloadValidator manages test workload deployment and validation
type WorkloadValidator struct {
	K             kubectl.Kubectl
	Cl            client.Client
	Namespace     string
	DataplaneMode DataplaneMode
}

// DeployWorkload deploys sample workloads (sleep + httpbin) based on dataplane mode
func (w *WorkloadValidator) DeployWorkload(ctx context.Context) error {
	// Create workload namespace
	if err := w.K.CreateNamespace(w.Namespace); err != nil {
		return fmt.Errorf("failed to create namespace %s: %w", w.Namespace, err)
	}

	// Label namespace based on dataplane mode
	switch w.DataplaneMode {
	case DataplaneModeSidecar:
		if err := w.K.Label("namespace", w.Namespace, "istio-injection", "enabled"); err != nil {
			return fmt.Errorf("failed to label namespace for sidecar injection: %w", err)
		}
	case DataplaneModeAmbient:
		if err := w.K.Label("namespace", w.Namespace, "istio.io/dataplane-mode", "ambient"); err != nil {
			return fmt.Errorf("failed to label namespace for ambient mode: %w", err)
		}
	default:
		return fmt.Errorf("unsupported dataplane mode: %s", w.DataplaneMode)
	}

	// Create httpbin namespace if it doesn't exist
	if err := w.K.CreateNamespace(HttpbinNamespace); err != nil {
		// Ignore error if namespace already exists
		if !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("failed to create httpbin namespace: %w", err)
		}
	}

	// Label httpbin namespace based on dataplane mode
	switch w.DataplaneMode {
	case DataplaneModeSidecar:
		if err := w.K.Label("namespace", HttpbinNamespace, "istio-injection", "enabled"); err != nil {
			return fmt.Errorf("failed to label httpbin namespace for sidecar injection: %w", err)
		}
	case DataplaneModeAmbient:
		if err := w.K.Label("namespace", HttpbinNamespace, "istio.io/dataplane-mode", "ambient"); err != nil {
			return fmt.Errorf("failed to label httpbin namespace for ambient mode: %w", err)
		}
	}

	// Deploy sleep in workload namespace
	if err := w.K.WithNamespace(w.Namespace).ApplyKustomize(SleepContainerName); err != nil {
		return fmt.Errorf("failed to deploy sleep: %w", err)
	}

	// Deploy httpbin in httpbin namespace
	if err := w.K.WithNamespace(HttpbinNamespace).ApplyKustomize(HttpbinContainerName); err != nil {
		return fmt.Errorf("failed to deploy httpbin: %w", err)
	}

	return nil
}

// ValidateConnectivity validates that workloads can communicate
func (w *WorkloadValidator) ValidateConnectivity(ctx context.Context) error {
	// Wait for pods to be ready in workload namespace
	if err := CheckPodsReady(ctx, w.Cl, w.Namespace); err != nil {
		return fmt.Errorf("workload pods not ready in %s: %w", w.Namespace, err)
	}

	// Wait for pods to be ready in httpbin namespace
	if err := CheckPodsReady(ctx, w.Cl, HttpbinNamespace); err != nil {
		return fmt.Errorf("httpbin pods not ready in %s: %w", HttpbinNamespace, err)
	}

	// Get sleep pod
	sleepPods := &corev1.PodList{}
	if err := w.Cl.List(ctx, sleepPods, client.InNamespace(w.Namespace)); err != nil {
		return fmt.Errorf("failed to list pods in %s: %w", w.Namespace, err)
	}
	if len(sleepPods.Items) == 0 {
		return fmt.Errorf("no pods found in %s namespace", w.Namespace)
	}

	// Test connectivity from sleep to httpbin
	CheckPodConnectivity(sleepPods.Items[0].Name, SleepContainerName, w.Namespace, HttpbinNamespace, w.K)
	return nil
}

// ValidateProxyVersion validates proxy version based on dataplane mode
func (w *WorkloadValidator) ValidateProxyVersion(ctx context.Context, expectedVersion *semver.Version) error {
	switch w.DataplaneMode {
	case DataplaneModeSidecar:
		return w.validateSidecarProxyVersion(ctx, expectedVersion)
	case DataplaneModeAmbient:
		return w.validateZTunnelVersion(ctx, expectedVersion)
	default:
		return fmt.Errorf("unsupported dataplane mode: %s", w.DataplaneMode)
	}
}

// validateSidecarProxyVersion checks proxy container version in workload pods
func (w *WorkloadValidator) validateSidecarProxyVersion(ctx context.Context, expectedVersion *semver.Version) error {
	pods := &corev1.PodList{}
	if err := w.Cl.List(ctx, pods, client.InNamespace(w.Namespace)); err != nil {
		return fmt.Errorf("failed to list pods in %s: %w", w.Namespace, err)
	}

	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods found in %s namespace", w.Namespace)
	}

	for _, pod := range pods.Items {
		proxyVersion, err := GetProxyVersion(pod.Name, w.Namespace)
		if err != nil {
			return fmt.Errorf("failed to get proxy version for pod %s: %w", pod.Name, err)
		}
		if !proxyVersion.Equal(expectedVersion) {
			return fmt.Errorf("pod %s has proxy version %s, expected %s",
				pod.Name, proxyVersion, expectedVersion)
		}
	}
	return nil
}

// validateZTunnelVersion checks ZTunnel DaemonSet version by reading image tag
func (w *WorkloadValidator) validateZTunnelVersion(ctx context.Context, expectedVersion *semver.Version) error {
	daemonset := &appsv1.DaemonSet{}
	if err := w.Cl.Get(ctx, kube.Key("ztunnel", ZtunnelNamespace), daemonset); err != nil {
		return fmt.Errorf("failed to get ZTunnel DaemonSet: %w", err)
	}

	// Extract version from ZTunnel container image tag
	// Image format: gcr.io/istio-release/ztunnel:1.29.2
	for _, container := range daemonset.Spec.Template.Spec.Containers {
		if container.Name == "istio-proxy" {
			parts := strings.Split(container.Image, ":")
			if len(parts) != 2 {
				return fmt.Errorf("unexpected ZTunnel image format: %s", container.Image)
			}
			tag := parts[1]

			// Parse version from tag
			version, err := semver.NewVersion(tag)
			if err != nil {
				return fmt.Errorf("failed to parse ZTunnel version from tag %s: %w", tag, err)
			}

			if !version.Equal(expectedVersion) {
				return fmt.Errorf("ZTunnel has version %s, expected %s", version, expectedVersion)
			}
			return nil
		}
	}

	return fmt.Errorf("istio-proxy container not found in ZTunnel DaemonSet")
}

// Cleanup removes workload and httpbin namespaces
// Note: In practice, cleanup is handled by the cleaner.New() pattern in tests,
// so this method is a no-op.
func (w *WorkloadValidator) Cleanup(ctx context.Context) error {
	// Cleanup is handled by cleaner.New() in the test setup
	// which records the initial state and cleans up everything created during the test
	return nil
}
