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
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	"github.com/istio-ecosystem/sail-operator/pkg/test/project"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/env"
	. "github.com/istio-ecosystem/sail-operator/tests/e2e/util/gomega"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/helm"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/kubectl"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"istio.io/istio/pkg/ptr"
)

var (
	OperatorImage     = env.Get("IMAGE", "quay.io/maistra-dev/sail-operator:latest")
	OperatorNamespace = env.Get("NAMESPACE", "sail-operator")

	deploymentName        = env.Get("DEPLOYMENT_NAME", "sail-operator")
	controlPlaneNamespace = env.Get("CONTROL_PLANE_NS", "istio-system")
	istioName             = env.Get("ISTIO_NAME", "default")
	istioCniName          = env.Get("ISTIOCNI_NAME", "default")
	istioCniNamespace     = env.Get("ISTIOCNI_NAMESPACE", "istio-cni")

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
func GetSVCLoadBalancerAddress(ctx context.Context, cl client.Client, ns, svcName string) (string, error) {
	svc := &corev1.Service{}
	err := cl.Get(ctx, client.ObjectKey{Namespace: ns, Name: svcName}, svc)
	if err != nil {
		return "", err
	}

	// To avoid flakiness, wait for the LoadBalancer to be ready
	Eventually(func() ([]corev1.LoadBalancerIngress, error) {
		err := cl.Get(ctx, client.ObjectKey{Namespace: ns, Name: svcName}, svc)
		return svc.Status.LoadBalancer.Ingress, err
	}, "1m", "1s").ShouldNot(BeEmpty(), "LoadBalancer should be ready")

	return svc.Status.LoadBalancer.Ingress[0].IP, nil
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

func LogDebugInfo(k kubectl.Kubectl) {
	// General debugging information to help diagnose the failure
	// TODO: Add the creation of file with this information to be attached to the test report

	GinkgoWriter.Println()
	GinkgoWriter.Printf("The test run has failures and the debug information is as follows from cluster: %q:\n", k.GetClusterName())
	GinkgoWriter.Println("=========================================================")
	logOperatorDebugInfo(k)
	GinkgoWriter.Println("=========================================================")
	logIstioDebugInfo(k)
	GinkgoWriter.Println("=========================================================")
	logCNIDebugInfo(k)
	GinkgoWriter.Println("=========================================================")
	logCertsDebugInfo(k)
	GinkgoWriter.Println("=========================================================")
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

	output, err := k.WithNamespace(controlPlaneNamespace).GetPods("", "-o wide")
	logDebugElement("=====Pods in "+controlPlaneNamespace+"=====", output, err)

	logs, err := k.WithNamespace(controlPlaneNamespace).Logs("deploy/istiod", ptr.Of(120*time.Second))
	logDebugElement("=====Istiod logs=====", logs, err)

	events, err := k.WithNamespace(controlPlaneNamespace).GetEvents()
	logDebugElement("=====Events in "+controlPlaneNamespace+"=====", events, err)
}

func logCNIDebugInfo(k kubectl.Kubectl) {
	resource, err := k.GetYAML("istiocni", istioCniName)
	logDebugElement("=====IstioCNI YAML=====", resource, err)

	ds, err := k.WithNamespace(istioCniNamespace).GetYAML("daemonset", "istio-cni-node")
	logDebugElement("=====Istio CNI DaemonSet YAML=====", ds, err)

	events, err := k.WithNamespace(istioCniNamespace).GetEvents()
	logDebugElement("=====Events in "+istioCniNamespace+"=====", events, err)

	// Temporary information to gather more details about failure
	pods, err := k.WithNamespace(istioCniNamespace).GetPods("", "-o wide")
	logDebugElement("=====Pods in "+istioCniNamespace+"=====", pods, err)

	describe, err := k.WithNamespace(istioCniNamespace).Describe("daemonset", "istio-cni-node")
	logDebugElement("=====Istio CNI DaemonSet describe=====", describe, err)

	logs, err := k.WithNamespace(istioCniNamespace).Logs("daemonset/istio-cni-node", ptr.Of(120*time.Second))
	logDebugElement("=====Istio CNI logs=====", logs, err)
}

func logCertsDebugInfo(k kubectl.Kubectl) {
	certs, err := k.WithNamespace(controlPlaneNamespace).GetSecret("cacerts")
	logDebugElement("=====CA certs=====", certs, err)
}

func logDebugElement(caption string, info string, err error) {
	GinkgoWriter.Println("\n" + caption + ":")
	indent := "  "
	if err != nil {
		GinkgoWriter.Println(indent + err.Error())
	} else {
		GinkgoWriter.Println(indent + strings.ReplaceAll(strings.TrimSpace(info), "\n", "\n"+indent))
	}
}

func GetVersionFromIstiod() (*semver.Version, error) {
	k := kubectl.New("testCluster")
	output, err := k.WithNamespace(controlPlaneNamespace).Exec("deploy/istiod", "", "pilot-discovery version")
	if err != nil {
		return nil, fmt.Errorf("error getting version from istiod: %w", err)
	}

	matches := istiodVersionRegex.FindStringSubmatch(output)
	if len(matches) > 1 && matches[1] != "" {
		return semver.NewVersion(matches[1])
	}
	return nil, fmt.Errorf("error getting version from istiod: version not found in output: %s", output)
}

func CheckPodsReady(ctx SpecContext, cl client.Client, namespace string) (*corev1.PodList, error) {
	podList := &corev1.PodList{}

	err := cl.List(ctx, podList, client.InNamespace(namespace))
	if err != nil {
		return nil, fmt.Errorf("failed to list pods in %s namespace: %w", namespace, err)
	}

	Expect(podList.Items).ToNot(BeEmpty(), fmt.Sprintf("No pods found in %s namespace", namespace))

	for _, pod := range podList.Items {
		Eventually(GetObject).WithArguments(ctx, cl, kube.Key(pod.Name, namespace), &corev1.Pod{}).
			Should(HaveCondition(corev1.PodReady, metav1.ConditionTrue), fmt.Sprintf("%q Pod in %q namespace is not Ready", pod.Name, namespace))
	}

	return podList, nil
}

func InstallOperatorViaHelm(extraArgs ...string) error {
	args := []string{
		"--namespace " + OperatorNamespace,
		"--set image=" + OperatorImage,
		"--set operatorLogLevel=3",
	}
	args = append(args, extraArgs...)

	return helm.Install("sail-operator", filepath.Join(project.RootDir, "chart"), args...)
}

func UninstallOperator() error {
	return helm.Uninstall("sail-operator", "--namespace", OperatorNamespace)
}
