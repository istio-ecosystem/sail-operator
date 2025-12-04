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
	"regexp"
	"strings"
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
	SleepNamespace   = "sleep"
	HttpbinNamespace = "httpbin"
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
	// General debugging information to help diagnose the failure
	// TODO: Add the creation of file with this information to be attached to the test report

	GinkgoWriter.Println()
	GinkgoWriter.Println("The test run has failures and the debug information is as follows:")
	GinkgoWriter.Println()
	for _, k := range kubectls {
		if k.ClusterName != "" {
			GinkgoWriter.Println("=========================================================")
			GinkgoWriter.Println("CLUSTER:", k.ClusterName)
			GinkgoWriter.Println("=========================================================")
		}
		logOperatorDebugInfo(k)
		GinkgoWriter.Println("=========================================================")
		logIstioDebugInfo(k)
		GinkgoWriter.Println("=========================================================")
		logCNIDebugInfo(k)
		GinkgoWriter.Println("=========================================================")
		logCertsDebugInfo(k)
		GinkgoWriter.Println("=========================================================")
		logSampleNamespacesDebugInfo(k, suite)
		GinkgoWriter.Println("=========================================================")
		GinkgoWriter.Println()

		if suite == Ambient {
			logZtunnelDebugInfo(k)
			describe, err := k.WithNamespace(SleepNamespace).Describe("deployment", "sleep")
			logDebugElement("=====sleep deployment describe=====", describe, err)
			describe, err = k.WithNamespace(HttpbinNamespace).Describe("deployment", "httpbin")
			logDebugElement("=====httpbin deployment describe=====", describe, err)
		}
	}
}

func logOperatorDebugInfo(k kubectl.Kubectl) {
	k = k.WithNamespace(OperatorNamespace)
	operator, err := k.GetYAML("deployment", deploymentName)
	logDebugElement("=====Operator Deployment YAML=====", operator, err)

	logs, err := k.Logs("deploy/"+deploymentName, ptr.Of(120*time.Second))
	logDebugElement("=====Operator logs=====", logs, err)

	events, err := k.GetEvents()
	logDebugElement("=====Events in "+OperatorNamespace+"=====", events, err)

	// Temporary information to gather more details about failure
	pods, err := k.GetPods("", "-o wide")
	logDebugElement("=====Pods in "+OperatorNamespace+"=====", pods, err)

	describe, err := k.Describe("deployment", deploymentName)
	logDebugElement("=====Operator Deployment describe=====", describe, err)
}

func logIstioDebugInfo(k kubectl.Kubectl) {
	resource, err := k.GetYAML("istio", istioName)
	logDebugElement("=====Istio YAML=====", resource, err)

	output, err := k.WithNamespace(ControlPlaneNamespace).GetPods("", "-o wide")
	logDebugElement("=====Pods in "+ControlPlaneNamespace+"=====", output, err)

	logs, err := k.WithNamespace(ControlPlaneNamespace).Logs("deploy/istiod", ptr.Of(120*time.Second))
	logDebugElement("=====Istiod logs=====", logs, err)

	events, err := k.WithNamespace(ControlPlaneNamespace).GetEvents()
	logDebugElement("=====Events in "+ControlPlaneNamespace+"=====", events, err)

	// Running istioctl proxy-status to get the status of the proxies.
	proxyStatus, err := istioctl.GetProxyStatus()
	logDebugElement("=====Istioctl Proxy Status=====", proxyStatus, err)
}

func logCNIDebugInfo(k kubectl.Kubectl) {
	resource, err := k.GetYAML("istiocni", istioCniName)
	logDebugElement("=====IstioCNI YAML=====", resource, err)

	ds, err := k.WithNamespace(IstioCniNamespace).GetYAML("daemonset", "istio-cni-node")
	logDebugElement("=====Istio CNI DaemonSet YAML=====", ds, err)

	events, err := k.WithNamespace(IstioCniNamespace).GetEvents()
	logDebugElement("=====Events in "+IstioCniNamespace+"=====", events, err)

	// Temporary information to gather more details about failure
	pods, err := k.WithNamespace(IstioCniNamespace).GetPods("", "-o wide")
	logDebugElement("=====Pods in "+IstioCniNamespace+"=====", pods, err)

	describe, err := k.WithNamespace(IstioCniNamespace).Describe("daemonset", "istio-cni-node")
	logDebugElement("=====Istio CNI DaemonSet describe=====", describe, err)

	logs, err := k.WithNamespace(IstioCniNamespace).Logs("daemonset/istio-cni-node", ptr.Of(120*time.Second))
	logDebugElement("=====Istio CNI logs=====", logs, err)
}

func logZtunnelDebugInfo(k kubectl.Kubectl) {
	resource, err := k.GetYAML("ztunnel", "default")
	logDebugElement("=====ZTunnel YAML=====", resource, err)

	ds, err := k.WithNamespace(ZtunnelNamespace).GetYAML("daemonset", "ztunnel")
	logDebugElement("=====ZTunnel DaemonSet YAML=====", ds, err)

	events, err := k.WithNamespace(ZtunnelNamespace).GetEvents()
	logDebugElement("=====Events in "+ZtunnelNamespace+"=====", events, err)

	describe, err := k.WithNamespace(ZtunnelNamespace).Describe("daemonset", "ztunnel")
	logDebugElement("=====ZTunnel DaemonSet describe=====", describe, err)

	logs, err := k.WithNamespace(ZtunnelNamespace).Logs("daemonset/ztunnel", ptr.Of(120*time.Second))
	logDebugElement("=====ztunnel logs=====", logs, err)
}

func logCertsDebugInfo(k kubectl.Kubectl) {
	certs, err := k.WithNamespace(ControlPlaneNamespace).GetSecret("cacerts")
	logDebugElement("=====CA certs in "+ControlPlaneNamespace+"=====", certs, err)
}

func logSampleNamespacesDebugInfo(k kubectl.Kubectl, suite testSuite) {
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
		logSampleNamespaceInfo(k, ns)
	}
}

func logSampleNamespaceInfo(k kubectl.Kubectl, namespace string) {
	// Check if namespace exists
	nsInfo, err := k.GetYAML("namespace", namespace)
	if err != nil {
		logDebugElement("=====Namespace "+namespace+" (not found)=====", "", err)
		return
	}
	logDebugElement("=====Namespace "+namespace+" YAML=====", nsInfo, err)

	// Get pods in the namespace with wide output for more details
	pods, err := k.WithNamespace(namespace).GetPods("", "-o wide")
	logDebugElement("=====Pods in "+namespace+"=====", pods, err)

	// Get events in the namespace
	events, err := k.WithNamespace(namespace).GetEvents()
	logDebugElement("=====Events in "+namespace+"=====", events, err)

	// Get deployments
	deployments, err := k.WithNamespace(namespace).GetYAML("deployments", "")
	logDebugElement("=====Deployments in "+namespace+"=====", deployments, err)

	// Get services
	services, err := k.WithNamespace(namespace).GetYAML("services", "")
	logDebugElement("=====Services in "+namespace+"=====", services, err)

	// Describe failed or non-ready pods specifically
	logFailedPodsDetails(k, namespace)
}

func logFailedPodsDetails(k kubectl.Kubectl, namespace string) {
	// Describe all pods in the namespace for detailed troubleshooting
	// This provides comprehensive information about pod status, events, and configuration
	describe, err := k.WithNamespace(namespace).Describe("pods", "")
	logDebugElement("=====Pod descriptions in "+namespace+"=====", describe, err)
}

func logDebugElement(caption string, info string, err error) {
	GinkgoWriter.Println("\n" + caption + ":")
	if err != nil {
		GinkgoWriter.Println(Indent(err.Error()))
	} else {
		GinkgoWriter.Println(Indent(strings.TrimSpace(info)))
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
apiVersion: sailoperator.io/v1alpha1
kind: ZTunnel
metadata:
  name: default
spec:
  profile: ambient
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

func CheckPodConnectivity(podName, srcNamespace, destNamespace string, k kubectl.Kubectl) {
	command := fmt.Sprintf(`curl -o /dev/null -s -w "%%{http_code}\n" httpbin.%s.svc.cluster.local:8000/get`, destNamespace)
	response, err := k.WithNamespace(srcNamespace).Exec(podName, srcNamespace, command)
	Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("error connecting to the %q pod", podName))
	Expect(response).To(ContainSubstring("200"), fmt.Sprintf("Unexpected response from %s pod", podName))
}

func HaveContainersThat(matcher types.GomegaMatcher) types.GomegaMatcher {
	return HaveField("Spec.Template.Spec.Containers", matcher)
}

func ImageFromRegistry(regexp string) types.GomegaMatcher {
	return HaveField("Image", MatchRegexp(regexp))
}
