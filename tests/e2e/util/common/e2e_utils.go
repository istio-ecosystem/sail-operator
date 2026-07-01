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
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/istio-ecosystem/sail-operator/pkg/env"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/istioctl"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/kubectl"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"istio.io/istio/pkg/ptr"
)

type testSuite string

const (
	Ambient           testSuite = "ambient"
	ControlPlane      testSuite = "control-plane"
	DualStack         testSuite = "dual-stack"
	MultiCluster      testSuite = "multi-cluster"
	Operator          testSuite = "operator"
	MultiControlPlane testSuite = "multi-control-plane"
)

const (
	SleepNamespace       = "sleep"
	HttpbinNamespace     = "httpbin"
	SleepContainerName   = "sleep"
	HttpbinContainerName = "httpbin"

	// maxJUnitErrorMessageSize is the maximum size (in bytes) for error messages
	// written to junit XML files. Messages exceeding this size will be truncated.
	maxJUnitErrorMessageSize = 10 * 1024 // 10KB
)

var (
	ControlPlaneNamespace = env.Get("CONTROL_PLANE_NS", "istio-system")
	IstioCniNamespace     = env.Get("ISTIOCNI_NAMESPACE", "istio-cni")
	OperatorImage         = env.Get("IMAGE", "quay.io/sail-dev/sail-operator:latest")
	OperatorNamespace     = env.Get("NAMESPACE", "sail-operator")
	ZtunnelNamespace      = env.Get("ZTUNNEL_NAMESPACE", "ztunnel")

	deploymentName  = env.Get("DEPLOYMENT_NAME", "sail-operator")
	istioName       = env.Get("ISTIO_NAME", "default")
	istioCniName    = env.Get("ISTIOCNI_NAME", "default")
	sampleNamespace = env.Get("SAMPLE_NAMESPACE", "sample")

	// version can have one of the following formats:
	// - 1.22.2
	// - 1.23.0-rc.1
	// - 1.24-alpha.feabc1234
	// matching only the version before first '_' which is used in the downstream builds, e.g. "1.23.2_ossm_tp.2"
	istiodVersionRegex = regexp.MustCompile(`Version:"([^"_]*)[^"]*"`)
)

// GetObject returns the object with the given key
func GetObject(ctx context.Context, cl client.Client, key client.ObjectKey, obj client.Object) (client.Object, error) {
	err := cl.Get(ctx, key, obj)
	return obj, err
}

// GetList invokes client.List and returns the list
func GetList(ctx context.Context, cl client.Client, list client.ObjectList, opts ...client.ListOption) (client.ObjectList, error) {
	err := cl.List(ctx, list, opts...)
	return list, err
}

// GetPodNameByLabel returns the name of the pod with the given label
func GetPodNameByLabel(ctx context.Context, cl client.Client, ns, labelKey, labelValue string) (string, error) {
	podList := &corev1.PodList{}
	err := cl.List(ctx, podList, client.InNamespace(ns), client.MatchingLabels{labelKey: labelValue})
	if err != nil {
		return "", err
	}
	if len(podList.Items) == 0 {
		return "", fmt.Errorf("no pod found with label %s=%s", labelKey, labelValue)
	}
	return podList.Items[0].Name, nil
}

// GetSVCLoadBalancerAddress returns the address of the service with the given name
func GetSVCLoadBalancerAddress(ctx context.Context, cl client.Client, ns, svcName string) string {
	svc := &corev1.Service{}
	err := cl.Get(ctx, client.ObjectKey{Namespace: ns, Name: svcName}, svc)
	Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Error getting LoadBalancer Service '%s/%s'", ns, svcName))

	// To avoid flakiness, wait for the LoadBalancer to be ready
	Eventually(func() ([]corev1.LoadBalancerIngress, error) {
		err := cl.Get(ctx, client.ObjectKey{Namespace: ns, Name: svcName}, svc)
		return svc.Status.LoadBalancer.Ingress, err
	}, "3m", "1s").ShouldNot(BeEmpty(), "LoadBalancer should be ready")

	if svc.Status.LoadBalancer.Ingress[0].IP != "" {
		return svc.Status.LoadBalancer.Ingress[0].IP
	} else if svc.Status.LoadBalancer.Ingress[0].Hostname != "" {
		return svc.Status.LoadBalancer.Ingress[0].Hostname
	}

	return ""
}

// CheckNamespaceEmpty checks if the given namespace is empty
func CheckNamespaceEmpty(ctx SpecContext, cl client.Client, ns string) {
	// TODO: Check to add more validations
	Eventually(func() ([]corev1.Pod, error) {
		podList := &corev1.PodList{}
		err := cl.List(ctx, podList, client.InNamespace(ns))
		return podList.Items, err
	}).Should(BeEmpty(), "No pods should be present in the namespace")

	Eventually(func() ([]appsv1.Deployment, error) {
		deploymentList := &appsv1.DeploymentList{}
		err := cl.List(ctx, deploymentList, client.InNamespace(ns))
		return deploymentList.Items, err
	}).Should(BeEmpty(), "No Deployments should be present in the namespace")

	Eventually(func() ([]appsv1.DaemonSet, error) {
		daemonsetList := &appsv1.DaemonSetList{}
		err := cl.List(ctx, daemonsetList, client.InNamespace(ns))
		return daemonsetList.Items, err
	}).Should(BeEmpty(), "No DaemonSets should be present in the namespace")

	Eventually(func() ([]corev1.Service, error) {
		serviceList := &corev1.ServiceList{}
		err := cl.List(ctx, serviceList, client.InNamespace(ns))
		return serviceList.Items, err
	}).Should(BeEmpty(), "No Services should be present in the namespace")
}

func LogDebugInfo(suite testSuite, kubectls ...kubectl.Kubectl) {
	artifactsDir := env.Get("ARTIFACTS", "/tmp/artifacts")
	printDebug := env.GetBool("E2E_PRINT_DEBUG", false)

	if printDebug {
		GinkgoWriter.Println()
		GinkgoWriter.Println("The test run has failures and the debug information is as follows:")
		GinkgoWriter.Println()
	}

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

		wg.Add(5)
		go func() { defer wg.Done(); logOperatorDebugInfo(k, artifactsDir, clusterName, printDebug) }()
		go func() { defer wg.Done(); logIstioDebugInfo(k, artifactsDir, clusterName, printDebug) }()
		go func() { defer wg.Done(); logCNIDebugInfo(k, artifactsDir, clusterName, printDebug) }()
		go func() { defer wg.Done(); logCertsDebugInfo(k, artifactsDir, clusterName, printDebug) }()
		go func() { defer wg.Done(); logSampleNamespacesDebugInfo(k, suite, artifactsDir, clusterName, printDebug) }()

		if suite == Ambient {
			wg.Add(1)
			go func() { defer wg.Done(); logZtunnelDebugInfo(k, artifactsDir, clusterName, printDebug) }()
		}

		wg.Wait()

		if printDebug {
			GinkgoWriter.Println("=========================================================")
			GinkgoWriter.Println()
		}
	}
}

// writeDebugFile writes the collected debug information to a file under the artifacts directory.
func writeDebugFile(artifactsDir, clusterName, section string, buf *strings.Builder) {
	debugDir := filepath.Join(artifactsDir, "debug", clusterName)
	if err := os.MkdirAll(debugDir, 0o755); err != nil {
		GinkgoWriter.Printf("Warning: failed to create debug directory %s: %v\n", debugDir, err)
		return
	}

	fileName := filepath.Join(debugDir, section+".log")
	if err := os.WriteFile(fileName, []byte(buf.String()), 0o644); err != nil {
		GinkgoWriter.Printf("Warning: failed to write debug file %s: %v\n", fileName, err)
		return
	}
	GinkgoWriter.Printf("Debug info written to %s\n", fileName)
}

func logOperatorDebugInfo(k kubectl.Kubectl, artifactsDir, clusterName string, printDebug bool) {
	var buf strings.Builder
	k = k.WithNamespace(OperatorNamespace)

	operator, err := k.GetYAML("deployment", deploymentName)
	logDebugElement("=====Operator Deployment YAML=====", operator, err, &buf, printDebug)

	logs, err := k.Logs("deploy/"+deploymentName, nil)
	logDebugElement("=====Operator logs=====", logs, err, &buf, printDebug)

	logPreviousLogsIfAvailable(k, "deploy/"+deploymentName, &buf, printDebug)

	events, err := k.GetEvents()
	logDebugElement("=====Events in "+OperatorNamespace+"=====", events, err, &buf, printDebug)

	pods, err := k.GetPods("", "-o wide")
	logDebugElement("=====Pods in "+OperatorNamespace+"=====", pods, err, &buf, printDebug)

	describe, err := k.Describe("deployment", deploymentName)
	logDebugElement("=====Operator Deployment describe=====", describe, err, &buf, printDebug)

	metrics, err := k.TopPods()
	logDebugElement("=====Resource metrics in "+OperatorNamespace+"=====", metrics, err, &buf, printDebug)

	writeDebugFile(artifactsDir, clusterName, "operator", &buf)
}

func logIstioDebugInfo(k kubectl.Kubectl, artifactsDir, clusterName string, printDebug bool) {
	var buf strings.Builder

	resource, err := k.GetYAML("istio", istioName)
	logDebugElement("=====Istio YAML=====", resource, err, &buf, printDebug)

	k = k.WithNamespace(ControlPlaneNamespace)

	output, err := k.GetPods("", "-o wide")
	logDebugElement("=====Pods in "+ControlPlaneNamespace+"=====", output, err, &buf, printDebug)

	logs, err := k.Logs("deploy/istiod", ptr.Of(120*time.Second))
	logDebugElement("=====Istiod logs=====", logs, err, &buf, printDebug)

	logPreviousLogsIfAvailable(k, "deploy/istiod", &buf, printDebug)

	meshConfig, err := k.GetYAML("configmap", "istio")
	logDebugElement("=====Istio Mesh ConfigMap=====", meshConfig, err, &buf, printDebug)

	events, err := k.GetEvents()
	logDebugElement("=====Events in "+ControlPlaneNamespace+"=====", events, err, &buf, printDebug)

	metrics, err := k.TopPods()
	logDebugElement("=====Resource metrics in "+ControlPlaneNamespace+"=====", metrics, err, &buf, printDebug)

	proxyStatus, err := istioctl.GetProxyStatus()
	logDebugElement("=====Istioctl Proxy Status=====", proxyStatus, err, &buf, printDebug)

	writeDebugFile(artifactsDir, clusterName, "istio", &buf)
}

func logCNIDebugInfo(k kubectl.Kubectl, artifactsDir, clusterName string, printDebug bool) {
	var buf strings.Builder

	resource, err := k.GetYAML("istiocni", istioCniName)
	logDebugElement("=====IstioCNI YAML=====", resource, err, &buf, printDebug)

	k = k.WithNamespace(IstioCniNamespace)

	ds, err := k.GetYAML("daemonset", "istio-cni-node")
	logDebugElement("=====Istio CNI DaemonSet YAML=====", ds, err, &buf, printDebug)

	events, err := k.GetEvents()
	logDebugElement("=====Events in "+IstioCniNamespace+"=====", events, err, &buf, printDebug)

	pods, err := k.GetPods("", "-o wide")
	logDebugElement("=====Pods in "+IstioCniNamespace+"=====", pods, err, &buf, printDebug)

	describe, err := k.Describe("daemonset", "istio-cni-node")
	logDebugElement("=====Istio CNI DaemonSet describe=====", describe, err, &buf, printDebug)

	logs, err := k.Logs("daemonset/istio-cni-node", ptr.Of(120*time.Second))
	logDebugElement("=====Istio CNI logs=====", logs, err, &buf, printDebug)

	logPreviousLogsIfAvailable(k, "daemonset/istio-cni-node", &buf, printDebug)

	metrics, err := k.TopPods()
	logDebugElement("=====Resource metrics in "+IstioCniNamespace+"=====", metrics, err, &buf, printDebug)

	writeDebugFile(artifactsDir, clusterName, "cni", &buf)
}

func logZtunnelDebugInfo(k kubectl.Kubectl, artifactsDir, clusterName string, printDebug bool) {
	var buf strings.Builder

	resource, err := k.GetYAML("ztunnel", "default")
	logDebugElement("=====ZTunnel YAML=====", resource, err, &buf, printDebug)

	k = k.WithNamespace(ZtunnelNamespace)

	ds, err := k.GetYAML("daemonset", "ztunnel")
	logDebugElement("=====ZTunnel DaemonSet YAML=====", ds, err, &buf, printDebug)

	events, err := k.GetEvents()
	logDebugElement("=====Events in "+ZtunnelNamespace+"=====", events, err, &buf, printDebug)

	describe, err := k.Describe("daemonset", "ztunnel")
	logDebugElement("=====ZTunnel DaemonSet describe=====", describe, err, &buf, printDebug)

	logs, err := k.Logs("daemonset/ztunnel", ptr.Of(120*time.Second))
	logDebugElement("=====ztunnel logs=====", logs, err, &buf, printDebug)

	logPreviousLogsIfAvailable(k, "daemonset/ztunnel", &buf, printDebug)

	metrics, err := k.TopPods()
	logDebugElement("=====Resource metrics in "+ZtunnelNamespace+"=====", metrics, err, &buf, printDebug)

	writeDebugFile(artifactsDir, clusterName, "ztunnel", &buf)
}

func logCertsDebugInfo(k kubectl.Kubectl, artifactsDir, clusterName string, printDebug bool) {
	var buf strings.Builder

	certs, err := k.WithNamespace(ControlPlaneNamespace).GetSecret("cacerts")
	logDebugElement("=====CA certs in "+ControlPlaneNamespace+"=====", certs, err, &buf, printDebug)

	writeDebugFile(artifactsDir, clusterName, "certs", &buf)
}

func logSampleNamespacesDebugInfo(k kubectl.Kubectl, suite testSuite, artifactsDir, clusterName string, printDebug bool) {
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
		var buf strings.Builder
		logSampleNamespaceInfo(k, ns, &buf, printDebug)
		writeDebugFile(artifactsDir, clusterName, "namespace-"+ns, &buf)
	}
}

func logSampleNamespaceInfo(k kubectl.Kubectl, namespace string, buf *strings.Builder, printDebug bool) {
	// Check if namespace exists
	nsInfo, err := k.GetYAML("namespace", namespace)
	if err != nil {
		logDebugElement("=====Namespace "+namespace+" (not found)=====", "", err, buf, printDebug)
		return
	}
	logDebugElement("=====Namespace "+namespace+" YAML=====", nsInfo, nil, buf, printDebug)

	k = k.WithNamespace(namespace)

	pods, err := k.GetPods("", "-o wide")
	logDebugElement("=====Pods in "+namespace+"=====", pods, err, buf, printDebug)

	events, err := k.GetEvents()
	logDebugElement("=====Events in "+namespace+"=====", events, err, buf, printDebug)

	deployments, err := k.GetYAML("deployments", "")
	logDebugElement("=====Deployments in "+namespace+"=====", deployments, err, buf, printDebug)

	services, err := k.GetYAML("services", "")
	logDebugElement("=====Services in "+namespace+"=====", services, err, buf, printDebug)

	endpoints, err := k.GetYAML("endpoints", "")
	logDebugElement("=====Endpoints in "+namespace+"=====", endpoints, err, buf, printDebug)

	networkPolicies, err := k.GetYAML("networkpolicies", "")
	logDebugElement("=====NetworkPolicies in "+namespace+"=====", networkPolicies, err, buf, printDebug)

	metrics, err := k.TopPods()
	logDebugElement("=====Resource metrics in "+namespace+"=====", metrics, err, buf, printDebug)

	// Describe failed or non-ready pods specifically
	logFailedPodsDetails(k, namespace, buf, printDebug)
}

func logFailedPodsDetails(k kubectl.Kubectl, namespace string, buf *strings.Builder, printDebug bool) {
	describe, err := k.WithNamespace(namespace).Describe("pods", "")
	logDebugElement("=====Pod descriptions in "+namespace+"=====", describe, err, buf, printDebug)
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

func logDebugElement(caption string, info string, err error, buf *strings.Builder, printDebug bool) {
	buf.WriteString("\n" + caption + ":\n")
	if err != nil {
		buf.WriteString(Indent(err.Error()) + "\n")
	} else {
		buf.WriteString(Indent(strings.TrimSpace(info)) + "\n")
	}

	if printDebug {
		GinkgoWriter.Println("\n" + caption + ":")
		if err != nil {
			GinkgoWriter.Println(Indent(truncateForJUnit(err.Error())))
		} else {
			GinkgoWriter.Println(Indent(truncateForJUnit(strings.TrimSpace(info))))
		}
	}
}

// logPreviousLogsIfAvailable attempts to collect previous container logs for a resource
// This is useful when pods have restarted - it captures logs from before the restart
func logPreviousLogsIfAvailable(k kubectl.Kubectl, resource string, buf *strings.Builder, printDebug bool) {
	prevLogs, err := k.LogsPrevious(resource, nil)
	if err == nil && prevLogs != "" {
		logDebugElement(fmt.Sprintf("=====Previous logs for %s (container restarted)=====", resource), prevLogs, nil, buf, printDebug)
	}
}

func GetVersionFromIstiod() (*semver.Version, error) {
	k := kubectl.New()
	output, err := k.WithNamespace(ControlPlaneNamespace).Exec("deploy/istiod", "", "pilot-discovery version")
	if err != nil {
		return nil, fmt.Errorf("error getting version from istiod: %w", err)
	}

	matches := istiodVersionRegex.FindStringSubmatch(output)
	if len(matches) > 1 && matches[1] != "" {
		return semver.NewVersion(matches[1])
	}
	return nil, fmt.Errorf("error getting version from istiod: version not found in output: %s", output)
}

// Resolve domain name and return ip address.
// By default, return ipv4 address and if missing, return ipv6.
func ResolveHostDomainToIP(hostDomain string) (string, error) {
	const maxRetries = 5
	const delayRetry = 10 * time.Second

	var lastErr error

	for i := 0; i < maxRetries; i++ {
		ips, err := net.LookupIP(hostDomain)
		if err == nil {
			var ipv6Addr string
			for _, ip := range ips {
				if ip.To4() != nil {
					return ip.String(), nil
				} else if ipv6Addr == "" {
					ipv6Addr = ip.String()
				}
			}
			if ipv6Addr != "" {
				return ipv6Addr, nil
			}
			return "", fmt.Errorf("no IP address found for hostname: %s", hostDomain)
		}

		lastErr = err
		waitTime := delayRetry * (1 << i)
		time.Sleep(waitTime)
	}

	return "", fmt.Errorf("failed to resolve hostname %s after %d retries: %w", hostDomain, maxRetries, lastErr)
}

// CreateIstio custom resource using a given `kubectl` client and with the specified version.
// An optional spec list can be given to inject into the CR's spec.
func CreateIstio(k kubectl.Kubectl, version string, specs ...string) {
	yaml := `
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: %s
spec:
  version: %s
  namespace: %s`
	yaml = fmt.Sprintf(yaml, istioName, version, ControlPlaneNamespace)
	createResource(k, "Istio", yaml, specs...)
}

// CreateIstioCNI custom resource using a given `kubectl` client and with the specified version.
func CreateIstioCNI(k kubectl.Kubectl, version string, specs ...string) {
	yaml := `
apiVersion: sailoperator.io/v1
kind: IstioCNI
metadata:
  name: %s
spec:
  version: %s
  namespace: %s`
	yaml = fmt.Sprintf(yaml, istioCniName, version, IstioCniNamespace)
	createResource(k, "IstioCNI", yaml, specs...)
}

func CreateZTunnel(k kubectl.Kubectl, version string, specs ...string) {
	yaml := `
apiVersion: sailoperator.io/v1
kind: ZTunnel
metadata:
  name: default
spec:
  version: %s
  namespace: %s`
	yaml = fmt.Sprintf(yaml, version, ZtunnelNamespace)
	createResource(k, "ZTunnel", yaml, specs...)
}

func CreateAmbientGateway(k kubectl.Kubectl, namespace, network string) {
	yaml := `kind: Gateway
apiVersion: gateway.networking.k8s.io/v1
metadata:
  name: istio-eastwestgateway
  namespace: %s
  labels:
    topology.istio.io/network: %s
spec:
  gatewayClassName: istio-east-west
  listeners:
  - name: mesh
    port: 15008
    protocol: HBONE
    tls:
      mode: Terminate
      options:
        gateway.istio.io/tls-terminate-mode: ISTIO_MUTUAL`
	yaml = fmt.Sprintf(yaml, namespace, network)
	createResource(k, "Gateway", yaml)
}

func createResource(k kubectl.Kubectl, kind, yaml string, specs ...string) {
	for _, spec := range specs {
		yaml += Indent(spec)
	}

	Log(fmt.Sprintf("%s YAML:", kind), Indent(yaml))
	Expect(k.CreateFromString(yaml)).To(Succeed(), withClusterName(fmt.Sprintf("%s creation failed:", kind), k))
	Success(withClusterName(fmt.Sprintf("%s created", kind), k))
}

func Indent(str string) string {
	indent := "  "
	return indent + strings.ReplaceAll(str, "\n", "\n"+indent)
}

func withClusterName(m string, k kubectl.Kubectl) string {
	if k.ClusterName == "" {
		return m
	}

	return m + " on " + k.ClusterName
}

// CheckPodConnectivityWithError tests connectivity from podName to httpbin in destNamespace
// and returns an error instead of calling Expect directly. This allows callers wrapped in
// Eventually to retry on transient failures (e.g. 503 during proxy startup/upgrade).
func CheckPodConnectivityWithError(podName, containerName, srcNamespace, destNamespace string, k kubectl.Kubectl) error {
	command := fmt.Sprintf(`curl -o /dev/null -s -w "%%{http_code}\n" httpbin.%s.svc.cluster.local:8000/get`, destNamespace)
	response, err := k.WithNamespace(srcNamespace).Exec(podName, containerName, command)
	if err != nil {
		return fmt.Errorf("error connecting to the %q pod: %w", podName, err)
	}
	if !strings.Contains(response, "200") {
		return fmt.Errorf("unexpected response from %s pod: %s", podName, strings.TrimSpace(response))
	}
	return nil
}

func CheckPodConnectivity(podName, containerName, srcNamespace, destNamespace string, k kubectl.Kubectl) {
	Expect(CheckPodConnectivityWithError(podName, containerName, srcNamespace, destNamespace, k)).To(Succeed())
}

func HaveContainersThat(matcher types.GomegaMatcher) types.GomegaMatcher {
	return HaveField("Spec.Template.Spec.Containers", matcher)
}

func ImageFromRegistry(regexp string) types.GomegaMatcher {
	return HaveField("Image", MatchRegexp(regexp))
}

func EnsureNamespace(ctx context.Context, ctrlclient client.Client, namespace string) *corev1.Namespace {
	GinkgoHelper()
	ns := &corev1.Namespace{}
	if err := ctrlclient.Get(ctx, client.ObjectKey{Name: namespace}, ns); apierrors.IsNotFound(err) {
		ns.Name = namespace
		if err := ctrlclient.Create(ctx, ns); err != nil && !apierrors.IsAlreadyExists(err) {
			Fail(fmt.Sprintf("Failed to create namespace: %s", err))
		}
	} else if err != nil {
		Fail(fmt.Sprintf("Failed to get namespace: %s", err))
	}
	return ns
}

func EnsureNamespaceWithCleanup(k kubectl.Kubectl, namespace string) {
	GinkgoHelper()
	Expect(k.CreateNamespace(namespace)).To(Succeed())
	DeferCleanup(func() {
		if err := k.Delete("namespace", namespace); err != nil {
			Log(fmt.Sprintf("Failed to delete namespace: %s", err))
		}
	})
}

// GetProxyVersion extracts the Istio proxy version from a pod using istioctl proxy-status
func GetProxyVersion(podName, namespace string) (*semver.Version, error) {
	proxyStatus, err := istioctl.GetProxyStatus("--namespace " + namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting proxy version: %w", err)
	}

	lines := strings.Split(proxyStatus, "\n")
	colSplit := regexp.MustCompile(`\s{2,}`)

	versionIdx := -1
	headers := colSplit.Split(strings.TrimSpace(lines[0]), -1)
	for i, header := range headers {
		if header == "VERSION" {
			versionIdx = i
			break
		}
	}
	if versionIdx == -1 {
		return nil, fmt.Errorf("VERSION header not found")
	}

	var versionStr string
	for _, line := range lines[1:] {
		if strings.Contains(line, podName+"."+namespace) {
			values := colSplit.Split(strings.TrimSpace(line), -1)
			if versionIdx < len(values) {
				versionStr = values[versionIdx]
				break
			}
		}
	}

	if versionStr == "" {
		return nil, fmt.Errorf("pod %s not found in proxy status output for namespace %s", podName, namespace)
	}
	version, err := semver.NewVersion(versionStr)
	if err != nil {
		return version, fmt.Errorf("error parsing proxy version %q: %w", versionStr, err)
	}
	return version, err
}

// GetIstioProxyContainer finds and returns the istio-proxy container from a pod
// It checks both regular containers and init containers (for persistent init containers in K8s 1.28+)
// Returns the container if found, nil otherwise
func GetIstioProxyContainer(pod corev1.Pod) *corev1.Container {
	// Check regular containers
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == "istio-proxy" {
			return &pod.Spec.Containers[i]
		}
	}

	// Check init containers
	for i := range pod.Spec.InitContainers {
		if pod.Spec.InitContainers[i].Name == "istio-proxy" {
			return &pod.Spec.InitContainers[i]
		}
	}

	return nil
}

// HasSidecarInjected checks if a pod has the istio-proxy sidecar injected
func HasSidecarInjected(pod corev1.Pod) bool {
	return GetIstioProxyContainer(pod) != nil
}

// HasHBONEEnabled checks if the istio-proxy sidecar has HBONE capability enabled
// by verifying the ISTIO_META_ENABLE_HBONE environment variable is set to "true"
func HasHBONEEnabled(pod corev1.Pod) bool {
	container := GetIstioProxyContainer(pod)
	if container == nil {
		return false
	}

	for _, env := range container.Env {
		if env.Name == "ISTIO_META_ENABLE_HBONE" && env.Value == "true" {
			return true
		}
	}

	return false
}
