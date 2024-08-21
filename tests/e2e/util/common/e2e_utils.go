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
	"regexp"
	"strings"
	"time"

	env "github.com/istio-ecosystem/sail-operator/tests/e2e/util/env"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/kubectl"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"istio.io/istio/pkg/ptr"
)

var (
	namespace             = env.Get("NAMESPACE", "sail-operator")
	deploymentName        = env.Get("DEPLOYMENT_NAME", "sail-operator")
	controlPlaneNamespace = env.Get("CONTROL_PLANE_NS", "istio-system")
	istioName             = env.Get("ISTIO_NAME", "default")
	istioCniName          = env.Get("ISTIOCNI_NAME", "default")
	istioCniNamespace     = env.Get("ISTIOCNI_NAMESPACE", "istio-cni")

	// version can have one of the following formats:
	// - 1.22.2
	// - 1.23.0-rc.1
	// - 1.24-alpha
	istiodVersionRegex = regexp.MustCompile(`Version:"(\d+\.\d+(\.\d+)?(-\w+(\.\d+)?)?)`)
)

// getObject returns the object with the given key
func GetObject(ctx context.Context, cl client.Client, key client.ObjectKey, obj client.Object) (client.Object, error) {
	err := cl.Get(ctx, key, obj)
	return obj, err
}

// getList invokes client.List and returns the list
func GetList(ctx context.Context, cl client.Client, list client.ObjectList, opts ...client.ListOption) (client.ObjectList, error) {
	err := cl.List(ctx, list, opts...)
	return list, err
}

// checkNamespaceEmpty checks if the given namespace is empty
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

func LogDebugInfo() {
	// General debugging information to help diagnose the failure
	// TODO: Add the creation of file with this information to be attached to the test report

	GinkgoWriter.Println()
	GinkgoWriter.Println("The test run has failures and the debug information is as follows:")
	GinkgoWriter.Println("=========================================================")
	logOperatorDebugInfo()
	GinkgoWriter.Println("=========================================================")
	logIstioDebugInfo()
	GinkgoWriter.Println("=========================================================")
	logCNIDebugInfo()
	GinkgoWriter.Println("=========================================================")
}

func logOperatorDebugInfo() {
	operator, err := kubectl.GetYAML(namespace, "deployment", deploymentName)
	logDebugElement("Operator Deployment YAML", operator, err)

	logs, err := kubectl.Logs(namespace, "deploy/"+deploymentName, ptr.Of(120*time.Second))
	logDebugElement("Operator logs", logs, err)

	events, err := kubectl.GetEvents(namespace)
	logDebugElement("Events in "+namespace, events, err)

	// Temporaty information to gather more details about failure
	pods, err := kubectl.GetPods(namespace, "", "-o wide")
	logDebugElement("Pods in "+namespace, pods, err)

	describe, err := kubectl.Describe(namespace, "deployment", deploymentName)
	logDebugElement("Operator Deployment describe", describe, err)
}

func logIstioDebugInfo() {
	resource, err := kubectl.GetYAML("", "istio", istioName)
	logDebugElement("Istio YAML", resource, err)

	output, err := kubectl.GetPods(controlPlaneNamespace, "", "-o wide")
	logDebugElement("Pods in "+controlPlaneNamespace, output, err)

	logs, err := kubectl.Logs(controlPlaneNamespace, "deploy/istiod", ptr.Of(120*time.Second))
	logDebugElement("Istiod logs", logs, err)

	events, err := kubectl.GetEvents(controlPlaneNamespace)
	logDebugElement("Events in "+controlPlaneNamespace, events, err)
}

func logCNIDebugInfo() {
	resource, err := kubectl.GetYAML("", "istiocni", istioCniName)
	logDebugElement("IstioCNI YAML", resource, err)

	ds, err := kubectl.GetYAML(istioCniNamespace, "daemonset", "istio-cni-node")
	logDebugElement("Istio CNI DaemonSet YAML", ds, err)

	events, err := kubectl.GetEvents(istioCniNamespace)
	logDebugElement("Events in "+istioCniNamespace, events, err)

	// Temporaty information to gather more details about failure
	pods, err := kubectl.GetPods(istioCniNamespace, "", "-o wide")
	logDebugElement("Pods in "+istioCniNamespace, pods, err)

	describe, err := kubectl.Describe(istioCniNamespace, "daemonset", "istio-cni-node")
	logDebugElement("Istio CNI DaemonSet describe", describe, err)
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

func GetVersionFromIstiod() (string, error) {
	output, err := kubectl.Exec(controlPlaneNamespace, "deploy/istiod", "", "pilot-discovery version")
	if err != nil {
		return "", fmt.Errorf("error getting version from istiod: %w", err)
	}

	matches := istiodVersionRegex.FindStringSubmatch(output)
	if len(matches) > 1 && matches[1] != "" {
		return matches[1], nil
	}
	return "", fmt.Errorf("error getting version from istiod: version not found in output: %s", output)
}
