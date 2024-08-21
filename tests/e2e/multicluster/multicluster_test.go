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

package multicluster

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/istio-ecosystem/sail-operator/api/v1alpha1"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	"github.com/istio-ecosystem/sail-operator/pkg/test/project"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/pkg/test/util/supportedversion"
	. "github.com/istio-ecosystem/sail-operator/tests/e2e/util/gomega"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/helm"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/istioctl"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/kubectl"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	common "github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Multicluster deployment models", Ordered, func() {
	SetDefaultEventuallyTimeout(180 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)

	// debugInfoLogged := false

	BeforeAll(func(ctx SpecContext) {
		// Deploy the Sail Operator on both clusters
		Expect(kubectl.CreateNamespace(namespace, kubeconfig)).To(Succeed(), "Namespace failed to be created on Primary Cluster")
		Expect(kubectl.CreateNamespace(namespace, kubeconfig2)).To(Succeed(), "Namespace failed to be created on Remote Cluster")

		Expect(helm.Install("sail-operator", filepath.Join(project.RootDir, "chart"), "--namespace "+namespace, "--set=image="+image, "--kubeconfig "+kubeconfig)).
			To(Succeed(), "Operator failed to be deployed in Primary Cluster")

		Eventually(common.GetObject).
			WithArguments(ctx, clPrimary, kube.Key(deploymentName, namespace), &appsv1.Deployment{}).
			Should(HaveCondition(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Error getting Istio CRD")
		Success("Operator is deployed in the Primary namespace and Running")

		Expect(helm.Install("sail-operator", filepath.Join(project.RootDir, "chart"), "--namespace "+namespace, "--set=image="+image, "--kubeconfig "+kubeconfig2)).
			To(Succeed(), "Operator failed to be deployed in Remote Cluster")

		Eventually(common.GetObject).
			WithArguments(ctx, clRemote, kube.Key(deploymentName, namespace), &appsv1.Deployment{}).
			Should(HaveCondition(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Error getting Istio CRD")
		Success("Operator is deployed in the Remote namespace and Running")
	})

	Describe("Multi-Primary Multi-Network configuration", func() {
		// Test the Multi-Primary Multi-Network configuration for each supported Istio version
		for _, version := range supportedversion.List {
			Context("Istio version is: "+version.Version, func() {
				When("Istio resources are created in both clusters with multicluster configuration", func() {
					BeforeAll(func(ctx SpecContext) {
						Expect(kubectl.CreateNamespace(controlPlaneNamespace, kubeconfig)).To(Succeed(), "Namespace failed to be created on Primary Cluster")
						Expect(kubectl.CreateNamespace(controlPlaneNamespace, kubeconfig2)).To(Succeed(), "Namespace failed to be created on Remote Cluster")

						multiclusterYAML := `
apiVersion: sailoperator.io/v1alpha1
kind: Istio
metadata:
  name: default
spec:
  version: %s
  namespace: %s
  values:
    global:
      meshID: %s
      multiCluster:
        clusterName: %s
      network: %s`
						multiclusterPrimaryYAML := fmt.Sprintf(multiclusterYAML, version.Name, controlPlaneNamespace, "mesh1", "cluster1", "network1")
						Log("Istio CR Primary: ", multiclusterPrimaryYAML)
						Expect(kubectl.CreateFromString(multiclusterPrimaryYAML, kubeconfig)).To(Succeed(), "Istio Resource creation failed on Primary Cluster")

						multiclusterRemoteYAML := fmt.Sprintf(multiclusterYAML, version.Name, controlPlaneNamespace, "mesh1", "cluster2", "network2")
						Log("Istio CR Remote: ", multiclusterRemoteYAML)
						Expect(kubectl.CreateFromString(multiclusterRemoteYAML, kubeconfig2)).To(Succeed(), "Istio Resource creation failed on Remote Cluster")
					})

					It("updates both Istio CR status to Ready", func(ctx SpecContext) {
						Eventually(common.GetObject).
							WithArguments(ctx, clPrimary, kube.Key(istioName), &v1alpha1.Istio{}).
							Should(HaveCondition(v1alpha1.IstioConditionReady, metav1.ConditionTrue), "Istio is not Ready on Primary; unexpected Condition")
						Success("Istio CR is Ready on Primary Cluster")

						Eventually(common.GetObject).
							WithArguments(ctx, clRemote, kube.Key(istioName), &v1alpha1.Istio{}).
							Should(HaveCondition(v1alpha1.IstioConditionReady, metav1.ConditionTrue), "Istio is not Ready on Remote; unexpected Condition")
						Success("Istio CR is Ready on Primary Cluster")
					})

					It("deploys istiod", func(ctx SpecContext) {
						Eventually(common.GetObject).
							WithArguments(ctx, clPrimary, kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(HaveCondition(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Istiod is not Available on Primary; unexpected Condition")
						Expect(common.GetVersionFromIstiod()).To(Equal(version.Version), "Unexpected istiod version")
						Success("Istiod is deployed in the namespace and Running on Primary Cluster")

						Eventually(common.GetObject).
							WithArguments(ctx, clRemote, kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(HaveCondition(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Istiod is not Available on Remote; unexpected Condition")
						Expect(common.GetVersionFromIstiod()).To(Equal(version.Version), "Unexpected istiod version")
						Success("Istiod is deployed in the namespace and Running on Remote Cluster")
					})

				})

				When("Gateway is created in both clusters", func() {
					BeforeAll(func(ctx SpecContext) {
						eastGatewayURL := "https://raw.githubusercontent.com/istio-ecosystem/sail-operator/main/docs/multicluster/east-west-gateway-net1.yaml"
						Expect(kubectl.Apply(controlPlaneNamespace, eastGatewayURL, kubeconfig)).To(Succeed(), "Gateway creation failed on Primary Cluster")

						westGatewayURL := "https://raw.githubusercontent.com/istio-ecosystem/sail-operator/main/docs/multicluster/east-west-gateway-net2.yaml"
						Expect(kubectl.Apply(controlPlaneNamespace, westGatewayURL, kubeconfig2)).To(Succeed(), "Gateway creation failed on Remote Cluster")

						// Expose the Gateway service in both clusters
						exposeServiceURL := "https://raw.githubusercontent.com/istio-ecosystem/sail-operator/main/docs/multicluster/expose-services.yaml"
						Expect(kubectl.Apply(controlPlaneNamespace, exposeServiceURL, kubeconfig)).To(Succeed(), "Expose Service creation failed on Primary Cluster")
						Expect(kubectl.Apply(controlPlaneNamespace, exposeServiceURL, kubeconfig2)).To(Succeed(), "Expose Service creation failed on Primary Cluster")
					})

					It("updates both Gateway status to Available", func(ctx SpecContext) {
						Eventually((common.GetObject)).
							WithArguments(ctx, clPrimary, kube.Key("istio-eastwestgateway", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(HaveCondition(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Gateway is not Ready on Primary; unexpected Condition")

						Eventually((common.GetObject)).
							WithArguments(ctx, clRemote, kube.Key("istio-eastwestgateway", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(HaveCondition(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Gateway is not Ready on Remote; unexpected Condition")
						Success("Gateway is created and available in both clusters")
					})
				})

				When("are installed remote secrets on each cluster", func() {
					BeforeAll(func(ctx SpecContext) {
						// Get the internal IP of the control plane node in both clusters
						internalIPPrimary, err := kubectl.GetInternalIP("node-role.kubernetes.io/control-plane", kubeconfig)
						Expect(err).NotTo(HaveOccurred())
						Expect(internalIPPrimary).NotTo(BeEmpty(), "Internal IP is empty for Primary Cluster")

						internalIPRemote, err := kubectl.GetInternalIP("node-role.kubernetes.io/control-plane", kubeconfig2)
						Expect(internalIPRemote).NotTo(BeEmpty(), "Internal IP is empty for Remote Cluster")
						Expect(err).NotTo(HaveOccurred())

						// Install a remote secret in Primary cluster that provides access to the Remote cluster API server.
						secret, err := istioctl.CreateRemoteSecret(kubeconfig2, "cluster2", internalIPRemote)
						Expect(err).NotTo(HaveOccurred())
						Expect(kubectl.ApplyString("", secret, kubeconfig)).To(Succeed(), "Remote secret creation failed on Primary Cluster")

						// Install a remote secret in Remote cluster that provides access to the Primary cluster API server.
						secret, err = istioctl.CreateRemoteSecret(kubeconfig, "cluster1", internalIPPrimary)
						Expect(err).NotTo(HaveOccurred())
						Expect(kubectl.ApplyString("", secret, kubeconfig2)).To(Succeed(), "Remote secret creation failed on Primary Cluster")
					})

					It("secrets are created", func(ctx SpecContext) {
						secret, err := common.GetObject(ctx, clPrimary, kube.Key("istio-remote-secret-cluster2", controlPlaneNamespace), &corev1.Secret{})
						Expect(err).NotTo(HaveOccurred())
						Expect(secret).NotTo(BeNil(), "Secret is not created on Primary Cluster")

						secret, err = common.GetObject(ctx, clRemote, kube.Key("istio-remote-secret-cluster1", controlPlaneNamespace), &corev1.Secret{})
						Expect(err).NotTo(HaveOccurred())
						Expect(secret).NotTo(BeNil(), "Secret is not created on Remote Cluster")
						Success("Remote secrets are created in both clusters")
					})
				})

				When("sample apps are deployed in both clusters", func() {
					BeforeAll(func(ctx SpecContext) {
						// Deploy the sample app in both clusters
						deploySampleApp("sample", version.Version, kubeconfig, kubeconfig2)
						Success("Sample app is deployed in both clusters")
					})

					It("updates the pods status to Ready", func(ctx SpecContext) {
						samplePodsPrimary := &corev1.PodList{}

						clPrimary.List(ctx, samplePodsPrimary, client.InNamespace("sample"))
						Expect(samplePodsPrimary.Items).ToNot(BeEmpty(), "No pods found in bookinfo namespace")

						for _, pod := range samplePodsPrimary.Items {
							Eventually(common.GetObject).
								WithArguments(ctx, clPrimary, kube.Key(pod.Name, "sample"), &corev1.Pod{}).
								Should(HaveCondition(corev1.PodReady, metav1.ConditionTrue), "Pod is not Ready on Primary; unexpected Condition")
						}

						samplePodsRemote := &corev1.PodList{}
						clRemote.List(ctx, samplePodsRemote, client.InNamespace("sample"))
						Expect(samplePodsRemote.Items).ToNot(BeEmpty(), "No pods found in bookinfo namespace")

						for _, pod := range samplePodsRemote.Items {
							Eventually(common.GetObject).
								WithArguments(ctx, clRemote, kube.Key(pod.Name, "sample"), &corev1.Pod{}).
								Should(HaveCondition(corev1.PodReady, metav1.ConditionTrue), "Pod is not Ready on Remote; unexpected Condition")
						}
						Success("Sample app is created in both clusters and Running")
					})

					It("can access the sample app from the remote cluster", func(ctx SpecContext) {
						sleepPodNamePrimary := getSleepPodName(ctx, clPrimary, "sample", "sleep")
						Expect(sleepPodNamePrimary).NotTo(BeEmpty(), "Sleep pod not found on Primary Cluster")

						sleepPodNameRemote := getSleepPodName(ctx, clRemote, "sample", "sleep")
						Expect(sleepPodNameRemote).NotTo(BeEmpty(), "Sleep pod not found on Remote Cluster")

						// Run the curl command from the sleep pod in the Remote Cluster and get response list to validate that we get responses from both clusters
						remoteResponses := strings.Join(getListCurlResponses("sample", sleepPodNameRemote, kubeconfig2), "\n")
						Log("Remote Responses: ", remoteResponses)
						Expect(remoteResponses).To(ContainSubstring("Hello version: v1"), "Responses from Remote Cluster are not the expected")
						Expect(remoteResponses).To(ContainSubstring("Hello version: v2"), "Responses from Remote Cluster are not the expected")

						// Run the curl command from the sleep pod in the Primary Cluster and get response list to validate that we get responses from both clusters
						primaryResponses := strings.Join(getListCurlResponses("sample", sleepPodNamePrimary, kubeconfig), "\n")
						Log("Primary Responses: ", primaryResponses)
						Expect(primaryResponses).To(ContainSubstring("Hello version: v1"), "Responses from Primary Cluster are not the expected")
						Expect(primaryResponses).To(ContainSubstring("Hello version: v2"), "Responses from Primary Cluster are not the expected")
						Success("Sample app is accessible from both clusters")
					})
				})

				When("sample apps are deleted in both clusters", func() {
					BeforeAll(func(ctx SpecContext) {
						// Delete the entire sample namespace in both clusters
						Expect(kubectl.DeleteNamespace("sample", kubeconfig)).To(Succeed(), "Namespace failed to be deleted on Primary Cluster")
						Expect(kubectl.DeleteNamespace("sample", kubeconfig2)).To(Succeed(), "Namespace failed to be deleted on Remote Cluster")
					})

					It("sample app is deleted in both clusters", func(ctx SpecContext) {
						Eventually(common.GetObject).
							WithArguments(ctx, clPrimary, kube.Key("helloworld-v1", "sample"), &appsv1.Deployment{}).
							Should(ReturnNotFoundError(), "HelloWorld v1 is not deleted on Primary Cluster")

						Eventually(common.GetObject).
							WithArguments(ctx, clRemote, kube.Key("helloworld-v2", "sample"), &appsv1.Deployment{}).
							Should(ReturnNotFoundError(), "HelloWorld v2 is not deleted on Remote Cluster")
						Success("Sample app is deleted in both clusters")
					})
				})

				When("control plane namespace is deleted in both clusters", func() {
					BeforeEach(func() {
						// Delete the Istio CR in both clusters
						Expect(kubectl.Delete(controlPlaneNamespace, "istio", istioName, kubeconfig)).To(Succeed(), "Istio CR failed to be deleted")
						Expect(kubectl.Delete(controlPlaneNamespace, "istio", istioName, kubeconfig2)).To(Succeed(), "Istio CR failed to be deleted")
						Success("Istio CR is deleted in both clusters")

						// Delete the control plane namespace in both clusters
						Expect(kubectl.DeleteNamespace(controlPlaneNamespace, kubeconfig)).To(Succeed(), "Istio CR failed to be deleted")
						Expect(kubectl.DeleteNamespace(controlPlaneNamespace, kubeconfig2)).To(Succeed(), "Istio CR failed to be deleted")
						Success("Control Plane namespace is deleted in both clusters")
					})

					It("removes everything from the namespace", func(ctx SpecContext) {
						Eventually(clPrimary.Get).WithArguments(ctx, kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(ReturnNotFoundError(), "Istiod should not exist anymore")
						common.CheckNamespaceEmpty(ctx, clPrimary, controlPlaneNamespace)
						Success("Namespace is empty")
					})
				})
			})
		}
	})
})

// deploySampleApp deploys the sample app in the given cluster
func deploySampleApp(ns, istioVersion, kubeconfig, kubeconfig2 string) {
	// Create the namespace
	Expect(kubectl.CreateNamespace(ns, kubeconfig)).To(Succeed(), "Namespace failed to be created")
	Expect(kubectl.CreateNamespace(ns, kubeconfig2)).To(Succeed(), "Namespace failed to be created")

	// Label the namespace
	Expect(kubectl.Patch("", "namespace", ns, "merge", `{"metadata":{"labels":{"istio-injection":"enabled"}}}`)).
		To(Succeed(), "Error patching sample namespace")
	Expect(kubectl.Patch("", "namespace", ns, "merge", `{"metadata":{"labels":{"istio-injection":"enabled"}}}`, kubeconfig2)).
		To(Succeed(), "Error patching sample namespace")

	// Deploy the sample app from upstream URL in both clusters
	if istioVersion == "latest" {
		istioVersion = "master"
	}
	helloWorldURL := fmt.Sprintf("https://raw.githubusercontent.com/istio/istio/%s/samples/helloworld/helloworld.yaml", istioVersion)
	Expect(kubectl.ApplyWithLabels(ns, helloWorldURL, "service=helloworld", kubeconfig)).To(Succeed(), "Sample service deploy failed on Primary Cluster")
	Expect(kubectl.ApplyWithLabels(ns, helloWorldURL, "service=helloworld", kubeconfig2)).To(Succeed(), "Sample service deploy failed on Remote Cluster")

	Expect(kubectl.ApplyWithLabels(ns, helloWorldURL, "version=v1", kubeconfig)).To(Succeed(), "Sample service deploy failed on Primary Cluster")
	Expect(kubectl.ApplyWithLabels(ns, helloWorldURL, "version=v2", kubeconfig2)).To(Succeed(), "Sample service deploy failed on Remote Cluster")

	sleepURL := fmt.Sprintf("https://raw.githubusercontent.com/istio/istio/%s/samples/sleep/sleep.yaml", istioVersion)
	Expect(kubectl.Apply(ns, sleepURL, kubeconfig)).To(Succeed(), "Sample sleep deploy failed on Primary Cluster")
	Expect(kubectl.Apply(ns, sleepURL, kubeconfig2)).To(Succeed(), "Sample sleep deploy failed on Remote Cluster")
}

// getPodName returns the pod name of the given deployment in the given namespace
func getSleepPodName(ctx context.Context, cl client.Client, ns, deploymentName string) string {
	samplePods := &corev1.PodList{}
	cl.List(ctx, samplePods, client.InNamespace(ns))
	var sleepPodName string
	for _, pod := range samplePods.Items {
		if pod.Labels["app"] == deploymentName {
			sleepPodName = pod.GetName()
			break
		}
	}

	return sleepPodName
}

// getListCurlResponses runs the curl command 10 times from the sleep pod in the given cluster and get response list
func getListCurlResponses(ns, podName, kubeconfig string) []string {
	var responses []string
	for i := 0; i < 10; i++ {
		response, err := kubectl.Exec(ns, podName, "sleep", "curl -sS helloworld.sample:5000/hello", kubeconfig)
		Expect(err).NotTo(HaveOccurred())
		responses = append(responses, response)
	}
	return responses
}
