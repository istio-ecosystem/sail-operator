//go:build e2e

// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package debugcollector

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/istio-ecosystem/sail-operator/pkg/env"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/istioctl"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/kubectl"
	. "github.com/onsi/ginkgo/v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DebugCollector records namespaces created during tests and collects comprehensive debug information on test failure.
// It saves all debug information to the artifacts directory for easier debugging.
type DebugCollector struct {
	cl              client.Client
	kubectl         kubectl.Kubectl
	ctx             []string
	recorded        bool
	artifactsDir    string
	recordedNS      map[string]struct{}
	clusterScoped   bool
	collectionDepth string
}

// New returns a DebugCollector which can record namespaces and collect debug information on test failure.
// It needs an initialized client and kubectl instance, and has optional (string) context which can be used to distinguish its output.
func New(cl client.Client, k kubectl.Kubectl, ctx ...string) DebugCollector {
	artifactsDir := env.Get("ARTIFACTS", os.TempDir())
	collectionDepth := env.Get("DEBUG_COLLECTOR_DEPTH", "full") // Options: full, minimal, logs-only

	return DebugCollector{
		cl:              cl,
		kubectl:         k,
		ctx:             ctx,
		recordedNS:      make(map[string]struct{}),
		artifactsDir:    artifactsDir,
		clusterScoped:   true,
		collectionDepth: collectionDepth,
	}
}

// Record will save the state of all namespaces that exist so they won't be included in debug collection.
// This allows the collector to focus on namespaces created during the test.
func (d *DebugCollector) Record(ctx context.Context) {
	d.recorded = true

	// Record all existing namespaces so we can focus on test-created ones during collection
	namespaceList := &corev1.NamespaceList{}
	if err := d.cl.List(ctx, namespaceList); err != nil {
		GinkgoWriter.Printf("Warning: Failed to list namespaces during Record: %v\n", err)
		return
	}

	for _, ns := range namespaceList.Items {
		d.recordedNS[ns.Name] = struct{}{}
	}
}

// CollectAndSave collects comprehensive debug information and saves it to the artifacts directory.
// It creates a timestamped directory structure for organized artifact storage.
func (d *DebugCollector) CollectAndSave(ctx context.Context) string {
	if !d.recorded {
		GinkgoWriter.Println("Warning: DebugCollector.Record() was not called. Collecting all namespaces.")
	}

	timestamp := time.Now().Format("20060102-150405")
	contextStr := strings.Join(d.ctx, "-")
	if contextStr != "" {
		contextStr = "-" + contextStr
	}

	debugDir := filepath.Join(d.artifactsDir, fmt.Sprintf("debug%s-%s", contextStr, timestamp))

	if err := os.MkdirAll(debugDir, 0755); err != nil {
		GinkgoWriter.Printf("Error creating debug directory %s: %v\n", debugDir, err)
		return debugDir
	}

	By(fmt.Sprintf("Collecting debug information to %s", debugDir))

	// Collect cluster-scoped resources
	if d.clusterScoped {
		d.collectClusterScopedResources(ctx, debugDir)
	}

	// Collect namespace-scoped resources
	d.collectNamespaceResources(ctx, debugDir)

	// Collect istioctl proxy-status
	d.collectIstioctlInfo(debugDir)

	Success(fmt.Sprintf("Debug information saved to: %s", debugDir))

	return debugDir
}

// collectClusterScopedResources collects cluster-wide resources for debugging.
func (d *DebugCollector) collectClusterScopedResources(ctx context.Context, debugDir string) {
	clusterDir := filepath.Join(debugDir, "cluster-scoped")
	if err := os.MkdirAll(clusterDir, 0755); err != nil {
		GinkgoWriter.Printf("Error creating cluster-scoped directory: %v\n", err)
		return
	}

	// Collect Istio CRs
	d.collectCustomResources(ctx, clusterDir, "sailoperator.io", "v1", "Istio")
	d.collectCustomResources(ctx, clusterDir, "sailoperator.io", "v1", "IstioCNI")
	d.collectCustomResources(ctx, clusterDir, "sailoperator.io", "v1alpha1", "ZTunnel")
	d.collectCustomResources(ctx, clusterDir, "sailoperator.io", "v1", "IstioRevision")
	d.collectCustomResources(ctx, clusterDir, "sailoperator.io", "v1", "IstioRevisionTag")

	// Collect nodes information - use GetYAML for cluster-scoped resources
	// For nodes, we'll use kubectl directly with empty namespace
	k := d.kubectl.WithNamespace("")
	if output, err := k.GetYAML("nodes", ""); err == nil {
		d.writeToFile(filepath.Join(clusterDir, "nodes.yaml"), output)
	}
}

// collectCustomResources collects custom resources of a specific GVK.
func (d *DebugCollector) collectCustomResources(ctx context.Context, dir, group, version, kind string) {
	gvk := schema.GroupVersionKind{
		Group:   group,
		Version: version,
		Kind:    kind + "List",
	}

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(gvk)

	if err := d.cl.List(ctx, list); err != nil {
		GinkgoWriter.Printf("Warning: Failed to list %s: %v\n", kind, err)
		return
	}

	for _, item := range list.Items {
		filename := fmt.Sprintf("%s-%s.yaml", strings.ToLower(kind), item.GetName())
		if output, err := d.kubectl.GetYAML(strings.ToLower(kind), item.GetName()); err == nil {
			d.writeToFile(filepath.Join(dir, filename), output)
		}
	}
}

// collectNamespaceResources collects resources from namespaces created during the test.
func (d *DebugCollector) collectNamespaceResources(ctx context.Context, debugDir string) {
	namespaceList := &corev1.NamespaceList{}
	if err := d.cl.List(ctx, namespaceList); err != nil {
		GinkgoWriter.Printf("Error listing namespaces: %v\n", err)
		return
	}

	namespacesDir := filepath.Join(debugDir, "namespaces")
	if err := os.MkdirAll(namespacesDir, 0755); err != nil {
		GinkgoWriter.Printf("Error creating namespaces directory: %v\n", err)
		return
	}

	for _, ns := range namespaceList.Items {
		// Skip namespaces that existed before the test
		if _, recorded := d.recordedNS[ns.Name]; recorded {
			continue
		}

		// Skip system namespaces that might have been auto-created
		if d.isSystemNamespace(ns.Name) {
			continue
		}

		d.collectNamespaceDebugInfo(ctx, namespacesDir, ns.Name)
	}
}

// isSystemNamespace checks if a namespace should be skipped from collection.
func (d *DebugCollector) isSystemNamespace(ns string) bool {
	systemNamespaces := []string{
		"kube-system",
		"kube-public",
		"kube-node-lease",
		"default",
		"local-path-storage",
	}

	for _, sysNS := range systemNamespaces {
		if ns == sysNS {
			return true
		}
	}

	return false
}

// collectNamespaceDebugInfo collects all debug information for a specific namespace.
func (d *DebugCollector) collectNamespaceDebugInfo(ctx context.Context, namespacesDir, ns string) {
	nsDir := filepath.Join(namespacesDir, ns)
	if err := os.MkdirAll(nsDir, 0755); err != nil {
		GinkgoWriter.Printf("Error creating namespace directory %s: %v\n", ns, err)
		return
	}

	// Create subdirectories
	resourcesDir := filepath.Join(nsDir, "resources")
	logsDir := filepath.Join(nsDir, "logs")

	if err := os.MkdirAll(resourcesDir, 0755); err != nil {
		GinkgoWriter.Printf("Error creating resources directory for %s: %v\n", ns, err)
		return
	}

	if d.collectionDepth != "minimal" {
		if err := os.MkdirAll(logsDir, 0755); err != nil {
			GinkgoWriter.Printf("Error creating logs directory for %s: %v\n", ns, err)
			return
		}
	}

	k := d.kubectl.WithNamespace(ns)

	// Collect resources (always collected regardless of depth)
	d.collectResourcesInNamespace(ctx, ns, resourcesDir, k)

	// Collect events
	if events, err := k.GetEvents(); err == nil {
		d.writeToFile(filepath.Join(nsDir, "events.yaml"), events)
	}

	// Collect pod logs (skipped in minimal mode)
	if d.collectionDepth != "minimal" {
		d.collectPodLogs(ctx, ns, logsDir)
	}
}

// collectResourcesInNamespace collects various Kubernetes resources in a specific namespace.
func (d *DebugCollector) collectResourcesInNamespace(ctx context.Context, ns, resourcesDir string, k kubectl.Kubectl) {
	// Collect Deployments
	deploymentList := &appsv1.DeploymentList{}
	if err := d.cl.List(ctx, deploymentList, client.InNamespace(ns)); err == nil {
		for _, deploy := range deploymentList.Items {
			if output, err := k.GetYAML("deployment", deploy.Name); err == nil {
				d.writeToFile(filepath.Join(resourcesDir, fmt.Sprintf("deployment-%s.yaml", deploy.Name)), output)
			}
			// Also collect describe output for more details
			if describe, err := k.Describe("deployment", deploy.Name); err == nil {
				d.writeToFile(filepath.Join(resourcesDir, fmt.Sprintf("deployment-%s.describe.txt", deploy.Name)), describe)
			}
		}
	}

	// Collect DaemonSets
	daemonsetList := &appsv1.DaemonSetList{}
	if err := d.cl.List(ctx, daemonsetList, client.InNamespace(ns)); err == nil {
		for _, ds := range daemonsetList.Items {
			if output, err := k.GetYAML("daemonset", ds.Name); err == nil {
				d.writeToFile(filepath.Join(resourcesDir, fmt.Sprintf("daemonset-%s.yaml", ds.Name)), output)
			}
			if describe, err := k.Describe("daemonset", ds.Name); err == nil {
				d.writeToFile(filepath.Join(resourcesDir, fmt.Sprintf("daemonset-%s.describe.txt", ds.Name)), describe)
			}
		}
	}

	// Collect Services
	serviceList := &corev1.ServiceList{}
	if err := d.cl.List(ctx, serviceList, client.InNamespace(ns)); err == nil {
		for _, svc := range serviceList.Items {
			if output, err := k.GetYAML("service", svc.Name); err == nil {
				d.writeToFile(filepath.Join(resourcesDir, fmt.Sprintf("service-%s.yaml", svc.Name)), output)
			}
		}
	}

	// Collect Pods (list view)
	if pods, err := k.GetPods("", "-o wide"); err == nil {
		d.writeToFile(filepath.Join(resourcesDir, "pods-list.txt"), pods)
	}

	// Collect individual pod YAMLs
	podList := &corev1.PodList{}
	if err := d.cl.List(ctx, podList, client.InNamespace(ns)); err == nil {
		for _, pod := range podList.Items {
			if output, err := k.GetYAML("pod", pod.Name); err == nil {
				d.writeToFile(filepath.Join(resourcesDir, fmt.Sprintf("pod-%s.yaml", pod.Name)), output)
			}
		}
	}

	// Collect ConfigMaps
	cmList := &corev1.ConfigMapList{}
	if err := d.cl.List(ctx, cmList, client.InNamespace(ns)); err == nil {
		for _, cm := range cmList.Items {
			if output, err := k.GetYAML("configmap", cm.Name); err == nil {
				d.writeToFile(filepath.Join(resourcesDir, fmt.Sprintf("configmap-%s.yaml", cm.Name)), output)
			}
		}
	}

	// Collect Secrets list (we get the list but individual secrets may contain sensitive data)
	// Using GetYAML on the resource type will list all secrets
	secretsList := &corev1.SecretList{}
	if err := d.cl.List(ctx, secretsList, client.InNamespace(ns)); err == nil {
		// Just save the names and types, not the actual secret data
		var secretsInfo strings.Builder
		secretsInfo.WriteString(fmt.Sprintf("Secrets in namespace %s:\n", ns))
		for _, secret := range secretsList.Items {
			secretsInfo.WriteString(fmt.Sprintf("  - Name: %s, Type: %s\n", secret.Name, secret.Type))
		}
		d.writeToFile(filepath.Join(resourcesDir, "secrets-list.txt"), secretsInfo.String())
	}
}

// collectPodLogs collects logs from all pods in a namespace.
func (d *DebugCollector) collectPodLogs(ctx context.Context, ns, logsDir string) {
	podList := &corev1.PodList{}
	if err := d.cl.List(ctx, podList, client.InNamespace(ns)); err != nil {
		GinkgoWriter.Printf("Error listing pods in namespace %s: %v\n", ns, err)
		return
	}

	k := d.kubectl.WithNamespace(ns)
	logsSince := 120 * time.Second

	for _, pod := range podList.Items {
		// Collect logs from all containers in the pod
		for _, container := range pod.Spec.Containers {
			logFile := fmt.Sprintf("%s-%s.log", pod.Name, container.Name)
			if logs, err := k.Logs(fmt.Sprintf("pod/%s -c %s", pod.Name, container.Name), &logsSince); err == nil {
				d.writeToFile(filepath.Join(logsDir, logFile), logs)
			} else {
				d.writeToFile(filepath.Join(logsDir, logFile), fmt.Sprintf("Error collecting logs: %v", err))
			}
		}

		// Collect logs from init containers if any
		for _, container := range pod.Spec.InitContainers {
			logFile := fmt.Sprintf("%s-%s-init.log", pod.Name, container.Name)
			if logs, err := k.Logs(fmt.Sprintf("pod/%s -c %s", pod.Name, container.Name), &logsSince); err == nil {
				d.writeToFile(filepath.Join(logsDir, logFile), logs)
			}
		}
	}
}

// collectIstioctlInfo collects istioctl debug information.
func (d *DebugCollector) collectIstioctlInfo(debugDir string) {
	if d.collectionDepth == "minimal" {
		return
	}

	istioctlDir := filepath.Join(debugDir, "istioctl")
	if err := os.MkdirAll(istioctlDir, 0755); err != nil {
		GinkgoWriter.Printf("Error creating istioctl directory: %v\n", err)
		return
	}

	// Collect proxy-status
	if proxyStatus, err := istioctl.GetProxyStatus(); err == nil {
		d.writeToFile(filepath.Join(istioctlDir, "proxy-status.txt"), proxyStatus)
	} else {
		d.writeToFile(filepath.Join(istioctlDir, "proxy-status.txt"), fmt.Sprintf("Error: %v", err))
	}
}

// writeToFile writes content to a file, creating parent directories if needed.
func (d *DebugCollector) writeToFile(filepath, content string) {
	if err := os.WriteFile(filepath, []byte(content), 0644); err != nil {
		GinkgoWriter.Printf("Error writing to file %s: %v\n", filepath, err)
	}
}

