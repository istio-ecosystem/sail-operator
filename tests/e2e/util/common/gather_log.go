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
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/istio-ecosystem/sail-operator/pkg/env"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/istioctl"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/kubectl"
	. "github.com/onsi/ginkgo/v2"
)

// Supported resource types for debug collection
const (
	ResourceTypeDeployment = "deployment"
	ResourceTypeDaemonSet  = "daemonset"
	ResourceTypeConfigMap  = "configmap"
	ResourceTypeSecret     = "secret"
	ResourceTypeIstio      = "istio"
	ResourceTypeIstioCNI   = "istiocni"
	ResourceTypeZTunnel    = "ztunnel"
)

// resourceSpec defines a resource to collect during debug gathering
type resourceSpec struct {
	resourceType    string // "deployment", "daemonset", "configmap", "secret", "istio", "istiocni", "ztunnel"
	name            string // resource name
	includeDescribe bool   // whether to collect kubectl describe output
	outputCaption   string // caption for console output
	outputFilename  string // filename for file output (without extension)
}

// debugConfig defines what debug information to collect for a namespace
type debugConfig struct {
	namespace           string
	customResources     []resourceSpec                                                               // Custom resources (Istio, IstioCNI, ZTunnel)
	workloads           []resourceSpec                                                               // Deployments, DaemonSets
	additionalResources []resourceSpec                                                               // ConfigMaps, Secrets, etc.
	customActions       []func(k kubectl.Kubectl, artifactsDir, clusterName string, printDebug bool) // Custom collection logic
}

// collectProxyStatus collects istioctl proxy-status output
func collectProxyStatus(k kubectl.Kubectl, artifactsDir, clusterName string, printDebug bool) {
	proxyStatus, err := istioctl.GetProxyStatus()
	writeDebugFile(artifactsDir, clusterName, ControlPlaneNamespace, "proxy-status.txt", formatOutput(proxyStatus, err))
	if printDebug {
		printDebugOutput("=====Istioctl Proxy Status=====", proxyStatus, err)
	}
}

// buildOperatorInfrastructureConfigs returns debug configurations for Sail operator, Istio, IstioCNI, and ZTunnel namespaces
func buildOperatorInfrastructureConfigs(suite testSuite) []debugConfig {
	// Common configs for all test suites
	configs := []debugConfig{
		// Operator namespace
		{
			namespace: OperatorNamespace,
			workloads: []resourceSpec{
				{
					resourceType:    ResourceTypeDeployment,
					name:            deploymentName,
					includeDescribe: true,
					outputCaption:   "Operator Deployment YAML",
					outputFilename:  "deployment-" + deploymentName,
				},
			},
		},
		// Istio control plane namespace
		{
			namespace: ControlPlaneNamespace,
			customResources: []resourceSpec{
				{
					resourceType:   ResourceTypeIstio,
					name:           istioName,
					outputCaption:  "Istio YAML",
					outputFilename: "istio-cr",
				},
			},
			additionalResources: []resourceSpec{
				{
					resourceType:   ResourceTypeConfigMap,
					name:           "istio",
					outputCaption:  "Istio Mesh ConfigMap",
					outputFilename: "configmap-istio",
				},
				{
					resourceType:   ResourceTypeSecret,
					name:           "cacerts",
					outputCaption:  "CA certs in " + ControlPlaneNamespace,
					outputFilename: "secret-cacerts",
				},
			},
			customActions: []func(kubectl.Kubectl, string, string, bool){
				collectProxyStatus,
			},
		},
		// CNI namespace
		{
			namespace: IstioCniNamespace,
			customResources: []resourceSpec{
				{
					resourceType:   ResourceTypeIstioCNI,
					name:           istioCniName,
					outputCaption:  "IstioCNI YAML",
					outputFilename: "istiocni-cr",
				},
			},
			workloads: []resourceSpec{
				{
					resourceType:    ResourceTypeDaemonSet,
					name:            "istio-cni-node",
					includeDescribe: true,
					outputCaption:   "Istio CNI DaemonSet YAML",
					outputFilename:  "daemonset-istio-cni-node",
				},
			},
		},
	}

	// Add ZTunnel config for Ambient test suite
	if suite == Ambient {
		configs = append(configs, debugConfig{
			namespace: ZtunnelNamespace,
			customResources: []resourceSpec{
				{
					resourceType:   ResourceTypeZTunnel,
					name:           "default",
					outputCaption:  "ZTunnel YAML",
					outputFilename: "ztunnel-cr",
				},
			},
			workloads: []resourceSpec{
				{
					resourceType:    ResourceTypeDaemonSet,
					name:            "ztunnel",
					includeDescribe: true,
					outputCaption:   "ZTunnel DaemonSet YAML",
					outputFilename:  "daemonset-ztunnel",
				},
			},
		})
	}

	return configs
}

// LogDebugInfo collects comprehensive debug information from Kubernetes clusters when tests fail.
// It organizes output by cluster and namespace, writing individual files for each debug element.
//
// Debug information includes:
// - Custom Resource YAMLs (Istio, IstioCNI, ZTunnel)
// - Deployment/DaemonSet YAMLs and descriptions
// - Pod lists and individual pod logs (current and previous)
// - Events, metrics, and resource status
// - Namespace-specific resources (deployments, services, endpoints, network policies)
//
// Output structure: $ARTIFACTS/debug/<cluster-name>/<namespace>/<file>
//
// Environment variables:
// - ARTIFACTS: Base directory for debug output (default: /tmp/artifacts)
// - E2E_PRINT_DEBUG: When true, also prints debug info to console (default: false)
func LogDebugInfo(suite testSuite, kubectls ...kubectl.Kubectl) {
	artifactsDir := env.Get("ARTIFACTS", "/tmp/artifacts")
	printDebug := env.GetBool("E2E_PRINT_DEBUG", false)

	if printDebug {
		GinkgoWriter.Println()
		GinkgoWriter.Println("The test run has failures and the debug information is as follows:")
		GinkgoWriter.Println()
	}

	// Get debug configurations for this test suite
	configs := buildOperatorInfrastructureConfigs(suite)

	for _, k := range kubectls {
		clusterName := k.ClusterName
		if clusterName == "" {
			clusterName = "default"
		}

		if printDebug && k.ClusterName != "" {
			GinkgoWriter.Println("=========================================================")
			GinkgoWriter.Println("CLUSTER:", k.ClusterName)
			GinkgoWriter.Println("=========================================================")
		}

		// Collect debug info in parallel
		var wg sync.WaitGroup

		// Collect debug info for each configured namespace
		wg.Add(len(configs))
		for _, config := range configs {
			go func(cfg debugConfig) {
				defer wg.Done()
				collectDebugInfo(k, artifactsDir, clusterName, cfg, printDebug)
			}(config)
		}

		// Collect sample namespace info
		wg.Add(1)
		go func() {
			defer wg.Done()
			logTestApplicationNamespaces(k, suite, artifactsDir, clusterName, printDebug)
		}()

		wg.Wait()

		if printDebug {
			GinkgoWriter.Println("=========================================================")
			GinkgoWriter.Println()
		}
	}
}

// writeDebugFile writes the collected debug information to a file under the artifacts directory.
// Files are organized by cluster and namespace: $ARTIFACTS/debug/<cluster-name>/<namespace>/<filename>
func writeDebugFile(artifactsDir, clusterName, namespace, filename, content string) error {
	debugDir := filepath.Join(artifactsDir, "debug", clusterName, namespace)
	if err := os.MkdirAll(debugDir, 0o755); err != nil {
		GinkgoWriter.Printf("Warning: failed to create debug directory %s: %v\n", debugDir, err)
		return err
	}

	filePath := filepath.Join(debugDir, filename)
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		GinkgoWriter.Printf("Warning: failed to write debug file %s: %v\n", filePath, err)
		return err
	}
	return nil
}

// printDebugOutput prints debug information to GinkgoWriter when E2E_PRINT_DEBUG is enabled
func printDebugOutput(caption, content string, err error) {
	GinkgoWriter.Println("\n" + caption + ":")
	if err != nil {
		GinkgoWriter.Println(Indent(truncateForJUnit(err.Error())))
	} else {
		GinkgoWriter.Println(Indent(truncateForJUnit(strings.TrimSpace(content))))
	}
}

// formatOutput formats content or error message for writing to file
func formatOutput(content string, err error) string {
	if err != nil {
		return err.Error()
	}
	return strings.TrimSpace(content)
}

func getPodsInNamespace(k kubectl.Kubectl, namespace string) ([]string, error) {
	podList, err := k.WithNamespace(namespace).GetPods("", "-o name")
	if err != nil {
		return nil, err
	}

	var pods []string
	lines := strings.Split(strings.TrimSpace(podList), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Remove "pod/" prefix if present
		podName := strings.TrimPrefix(line, "pod/")
		pods = append(pods, podName)
	}
	return pods, nil
}

// collectResource collects a single resource based on its specification
func collectResource(k kubectl.Kubectl, artifactsDir, clusterName, namespace string, spec resourceSpec, printDebug bool) {
	// Collect the resource YAML (kubectl will error if resource type is invalid)
	content, err := k.GetYAML(spec.resourceType, spec.name)

	// Write YAML output
	writeDebugFile(artifactsDir, clusterName, namespace, spec.outputFilename+".yaml", formatOutput(content, err))
	if printDebug {
		printDebugOutput("====="+spec.outputCaption+"=====", content, err)
	}

	// If requested, also collect describe output
	if spec.includeDescribe && (spec.resourceType == ResourceTypeDeployment || spec.resourceType == ResourceTypeDaemonSet) {
		describe, descErr := k.Describe(spec.resourceType, spec.name)
		writeDebugFile(artifactsDir, clusterName, namespace, spec.outputFilename+"-describe.txt", formatOutput(describe, descErr))
		if printDebug {
			printDebugOutput("====="+spec.outputCaption+" describe=====", describe, descErr)
		}
	}
}

// collectCommonNamespaceResources collects common resources present in all namespaces
func collectCommonNamespaceResources(k kubectl.Kubectl, artifactsDir, clusterName, namespace string, printDebug bool) {
	// Events
	events, err := k.GetEvents()
	writeDebugFile(artifactsDir, clusterName, namespace, "events.txt", formatOutput(events, err))
	if printDebug {
		printDebugOutput("=====Events in "+namespace+"=====", events, err)
	}

	// Pods list
	pods, err := k.GetPods("", "-o wide")
	writeDebugFile(artifactsDir, clusterName, namespace, "pods.txt", formatOutput(pods, err))
	if printDebug {
		printDebugOutput("=====Pods in "+namespace+"=====", pods, err)
	}

	// Resource metrics
	metrics, err := k.TopPods()
	writeDebugFile(artifactsDir, clusterName, namespace, "metrics.txt", formatOutput(metrics, err))
	if printDebug {
		printDebugOutput("=====Resource metrics in "+namespace+"=====", metrics, err)
	}
}

// collectDebugInfo is a generic function to collect debug information based on configuration
func collectDebugInfo(k kubectl.Kubectl, artifactsDir, clusterName string, config debugConfig, printDebug bool) {
	namespace := config.namespace

	// Collect custom resources (these are typically cluster-scoped or collected before setting namespace)
	for _, spec := range config.customResources {
		collectResource(k, artifactsDir, clusterName, namespace, spec, printDebug)
	}

	// Set namespace context for namespaced resources
	k = k.WithNamespace(namespace)

	// Collect workload resources (deployments, daemonsets)
	for _, spec := range config.workloads {
		collectResource(k, artifactsDir, clusterName, namespace, spec, printDebug)
	}

	// Collect common namespace resources (events, pods, metrics)
	collectCommonNamespaceResources(k, artifactsDir, clusterName, namespace, printDebug)

	// Collect additional resources (configmaps, secrets, etc.)
	for _, spec := range config.additionalResources {
		collectResource(k, artifactsDir, clusterName, namespace, spec, printDebug)
	}

	// Execute custom actions (like istioctl commands)
	for _, action := range config.customActions {
		action(k, artifactsDir, clusterName, printDebug)
	}

	// Collect individual pod logs
	collectPodsLogsInNamespace(k, artifactsDir, clusterName, namespace, printDebug)
}

// collectPodsLogsInNamespace collects logs from all pods in a namespace
func collectPodsLogsInNamespace(k kubectl.Kubectl, artifactsDir, clusterName, namespace string, printDebug bool) {
	pods, err := getPodsInNamespace(k, namespace)
	if err != nil {
		if printDebug {
			printDebugOutput("=====Failed to get pods in "+namespace+"=====", "", err)
		}
		writeDebugFile(artifactsDir, clusterName, namespace, "pods-error.txt", formatOutput("", err))
		return
	}

	for _, podName := range pods {
		// Collect current logs
		logs, err := k.WithNamespace(namespace).Logs(podName, nil)
		filename := "pod-" + podName + ".log"
		writeDebugFile(artifactsDir, clusterName, namespace, filename, formatOutput(logs, err))
		if printDebug {
			printDebugOutput("=====Logs for pod "+podName+"=====", logs, err)
		}

		// Collect previous logs if available
		prevLogs, err := k.WithNamespace(namespace).LogsPrevious(podName, nil)
		if err == nil && prevLogs != "" {
			prevFilename := "pod-" + podName + "-previous.log"
			writeDebugFile(artifactsDir, clusterName, namespace, prevFilename, prevLogs)
			if printDebug {
				printDebugOutput("=====Previous logs for pod "+podName+" (container restarted)=====", prevLogs, nil)
			}
		}
	}
}

func logTestApplicationNamespaces(k kubectl.Kubectl, suite testSuite, artifactsDir, clusterName string, printDebug bool) {
	// Common sample namespaces used across different test suites
	sampleNamespaces := []string{SleepNamespace, HttpbinNamespace}

	// Add additional namespaces based on test suite
	switch suite {
	case MultiCluster, ControlPlane, MultiControlPlane:
		sampleNamespaces = append(sampleNamespaces, "sample")
	case DualStack:
		// Dual-stack tests use specific namespaces for TCP services
		sampleNamespaces = append(sampleNamespaces, "dual-stack", "ipv4", "ipv6")
	}

	for _, ns := range sampleNamespaces {
		logTestApplicationNamespace(k, ns, artifactsDir, clusterName, printDebug)
	}
}

func logTestApplicationNamespace(k kubectl.Kubectl, namespace, artifactsDir, clusterName string, printDebug bool) {
	// Check if namespace exists
	nsInfo, err := k.GetYAML("namespace", namespace)
	if err != nil {
		// Namespace doesn't exist, write error and return
		writeDebugFile(artifactsDir, clusterName, namespace, "namespace-not-found.txt", formatOutput("", err))
		if printDebug {
			printDebugOutput("=====Namespace "+namespace+" (not found)=====", "", err)
		}
		return
	}

	writeDebugFile(artifactsDir, clusterName, namespace, "namespace.yaml", formatOutput(nsInfo, nil))
	if printDebug {
		printDebugOutput("=====Namespace "+namespace+" YAML=====", nsInfo, nil)
	}

	k = k.WithNamespace(namespace)

	pods, err := k.GetPods("", "-o wide")
	writeDebugFile(artifactsDir, clusterName, namespace, "pods.txt", formatOutput(pods, err))
	if printDebug {
		printDebugOutput("=====Pods in "+namespace+"=====", pods, err)
	}

	podDescribe, err := k.Describe("pods", "")
	writeDebugFile(artifactsDir, clusterName, namespace, "pods-describe.txt", formatOutput(podDescribe, err))
	if printDebug {
		printDebugOutput("=====Pod descriptions in "+namespace+"=====", podDescribe, err)
	}

	events, err := k.GetEvents()
	writeDebugFile(artifactsDir, clusterName, namespace, "events.txt", formatOutput(events, err))
	if printDebug {
		printDebugOutput("=====Events in "+namespace+"=====", events, err)
	}

	deployments, err := k.GetYAML("deployments", "")
	writeDebugFile(artifactsDir, clusterName, namespace, "deployments.yaml", formatOutput(deployments, err))
	if printDebug {
		printDebugOutput("=====Deployments in "+namespace+"=====", deployments, err)
	}

	services, err := k.GetYAML("services", "")
	writeDebugFile(artifactsDir, clusterName, namespace, "services.yaml", formatOutput(services, err))
	if printDebug {
		printDebugOutput("=====Services in "+namespace+"=====", services, err)
	}

	endpoints, err := k.GetYAML("endpoints", "")
	writeDebugFile(artifactsDir, clusterName, namespace, "endpoints.yaml", formatOutput(endpoints, err))
	if printDebug {
		printDebugOutput("=====Endpoints in "+namespace+"=====", endpoints, err)
	}

	networkPolicies, err := k.GetYAML("networkpolicies", "")
	writeDebugFile(artifactsDir, clusterName, namespace, "networkpolicies.yaml", formatOutput(networkPolicies, err))
	if printDebug {
		printDebugOutput("=====NetworkPolicies in "+namespace+"=====", networkPolicies, err)
	}

	metrics, err := k.TopPods()
	writeDebugFile(artifactsDir, clusterName, namespace, "metrics.txt", formatOutput(metrics, err))
	if printDebug {
		printDebugOutput("=====Resource metrics in "+namespace+"=====", metrics, err)
	}

	collectPodsLogsInNamespace(k, artifactsDir, clusterName, namespace, printDebug)
}

// truncateForJUnit truncates strings that exceed maxJUnitErrorMessageSize
// to prevent junit XML files from becoming excessively large.
func truncateForJUnit(s string) string {
	if len(s) <= maxJUnitErrorMessageSize {
		return s
	}

	const truncationMsg = "\n\n... [truncated due to size limit] ..."
	truncateAt := maxJUnitErrorMessageSize - len(truncationMsg)
	if truncateAt < 0 {
		truncateAt = 0
	}

	return s[:truncateAt] + truncationMsg
}
