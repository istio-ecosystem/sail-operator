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
	istioCniNamespace     = env.Get("ISTIOCNI_NAMESPACE", "istio-cni")
	istioCniName          = env.Get("ISTIOCNI_NAME", "default")
)

// key returns the client.ObjectKey for the given name and namespace. If no namespace is provided, it returns a key cluster scoped
func Key(name string, namespace ...string) client.ObjectKey {
	if len(namespace) > 1 {
		panic("you can only provide one namespace")
	} else if len(namespace) == 1 {
		return client.ObjectKey{Name: name, Namespace: namespace[0]}
	}
	return client.ObjectKey{Name: name}
}

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

	GinkgoWriter.Println("The test run has failures and the debug information is as follows:")
	// Display Operator information
	operator, err := kubectl.GetYAML(namespace, "deployment", deploymentName)
	if err != nil {
		GinkgoWriter.Println("Error getting operator deployment yaml: ", err)
	}
	GinkgoWriter.Println("Operator deployment: \n", operator)

	describe, err := kubectl.Describe(namespace, "deployment", deploymentName)
	if err != nil {
		GinkgoWriter.Println("Error getting operator deployment describe: ", err)
	}
	GinkgoWriter.Println("Operator deployment describe: \n", describe)

	logs, err := kubectl.Logs(namespace, "deploy/"+deploymentName, ptr.Of(120*time.Second))
	if err != nil {
		GinkgoWriter.Println("Error getting logs from the operator: ", err)
	}
	GinkgoWriter.Println("Logs from sail-operator pod: \n", logs)

	// Display Istio CR information
	resource, err := kubectl.GetYAML(controlPlaneNamespace, "istio", istioName)
	if err != nil {
		GinkgoWriter.Println("Error getting Istio CR: ", err)
	}
	GinkgoWriter.Println("Istio CR: \n", resource)

	output, err := kubectl.GetPods(controlPlaneNamespace, "-o wide")
	if err != nil {
		GinkgoWriter.Println("Error getting pods: ", err)
	}
	GinkgoWriter.Println("Pods in Istio CR namespace: \n", output)

	logs, err = kubectl.Logs(controlPlaneNamespace, "deploy/istiod", ptr.Of(120*time.Second))
	if err != nil {
		GinkgoWriter.Println("Error getting logs from the istiod: ", err)
	}
	GinkgoWriter.Println("Logs from istiod pod: \n", logs)

	// Display Istio CNI information.
	cni, err := kubectl.GetYAML(istioCniNamespace, "daemonset", istioCniName)
	if err != nil {
		GinkgoWriter.Println("Error getting Istio CNI daemonset yaml: ", err)
	}
	GinkgoWriter.Println("Istio CNI daemonset: \n", cni)

	describe, err = kubectl.Describe(istioCniNamespace, "daemonset", istioCniName)
	if err != nil {
		GinkgoWriter.Println("Error getting Istio CNI daemonset describe: ", err)
	}
	GinkgoWriter.Println("Istio CNI daemonset describe: \n", describe)
}
