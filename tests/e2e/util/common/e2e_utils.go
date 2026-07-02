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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
