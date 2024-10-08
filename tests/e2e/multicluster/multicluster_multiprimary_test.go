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
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/certs"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	. "github.com/istio-ecosystem/sail-operator/tests/e2e/util/gomega"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/helm"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/istioctl"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/kubectl"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Multicluster deployment models", Ordered, func() {
	SetDefaultEventuallyTimeout(180 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)

	BeforeAll(func(ctx SpecContext) {
		if !skipDeploy {
			// Deploy the Sail Operator on both clusters
			Expect(kubectlClient1.CreateNamespace(namespace)).To(Succeed(), "Namespace failed to be created on Cluster #1")
			Expect(kubectlClient2.CreateNamespace(namespace)).To(Succeed(), "Namespace failed to be created on  Cluster #2")

			Expect(helm.Install("sail-operator", filepath.Join(project.RootDir, "chart"), "--namespace "+namespace, "--set=image="+image, "--kubeconfig "+kubeconfig)).
				To(Succeed(), "Operator failed to be deployed in Cluster #1")

			Eventually(common.GetObject).
				WithArguments(ctx, clPrimary, kube.Key(deploymentName, namespace), &appsv1.Deployment{}).
				Should(HaveCondition(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Error getting Istio CRD")
			Success("Operator is deployed in the Cluster #1 namespace and Running")

			Expect(helm.Install("sail-operator", filepath.Join(project.RootDir, "chart"), "--namespace "+namespace, "--set=image="+image, "--kubeconfig "+kubeconfig2)).
				To(Succeed(), "Operator failed to be deployed in  Cluster #2")

			Eventually(common.GetObject).
				WithArguments(ctx, clRemote, kube.Key(deploymentName, namespace), &appsv1.Deployment{}).
				Should(HaveCondition(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Error getting Istio CRD")
			Success("Operator is deployed in the Cluster #2 namespace and Running")
		}
	})

	Describe("Multi-Primary Multi-Network configuration", func() {
		// Test the Multi-Primary Multi-Network configuration for each supported Istio version
		for _, version := range supportedversion.List {
			Context("Istio version is: "+version.Version, func() {
				When("Istio resources are created in both clusters with multicluster configuration", func() {
					BeforeAll(func(ctx SpecContext) {
						Expect(kubectlClient1.CreateNamespace(controlPlaneNamespace)).To(Succeed(), "Namespace failed to be created")
						Expect(kubectlClient2.CreateNamespace(controlPlaneNamespace)).To(Succeed(), "Namespace failed to be created")

						// Push the intermediate CA to both clusters
						Expect(certs.PushIntermediateCA(controlPlaneNamespace, kubeconfig, "east", "network1", artifacts, clPrimary)).To(Succeed())
						Expect(certs.PushIntermediateCA(controlPlaneNamespace, kubeconfig2, "west", "network2", artifacts, clRemote)).To(Succeed())

						// Wait for the secret to be created in both clusters
						Eventually(func() error {
							_, err := common.GetObject(context.Background(), clPrimary, kube.Key("cacerts", controlPlaneNamespace), &corev1.Secret{})
							return err
						}).ShouldNot(HaveOccurred(), "Secret is not created on Cluster #1")

						Eventually(func() error {
							_, err := common.GetObject(context.Background(), clRemote, kube.Key("cacerts", controlPlaneNamespace), &corev1.Secret{})
							return err
						}).ShouldNot(HaveOccurred(), "Secret is not created on Cluster #1")

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
						multiclusterCluster1YAML := fmt.Sprintf(multiclusterYAML, version.Name, controlPlaneNamespace, "mesh1", "cluster1", "network1")
						Log("Istio CR Cluster #1: ", multiclusterCluster1YAML)
						Expect(kubectlClient1.CreateFromString(multiclusterCluster1YAML)).To(Succeed(), "Istio Resource creation failed on Cluster #1")

						multiclusterCluster2YAML := fmt.Sprintf(multiclusterYAML, version.Name, controlPlaneNamespace, "mesh1", "cluster2", "network2")
						Log("Istio CR Cluster #2: ", multiclusterCluster2YAML)
						Expect(kubectlClient2.CreateFromString(multiclusterCluster2YAML)).To(Succeed(), "Istio Resource creation failed on  Cluster #2")
					})

					It("updates both Istio CR status to Ready", func(ctx SpecContext) {
						Eventually(common.GetObject).
							WithArguments(ctx, clPrimary, kube.Key(istioName), &v1alpha1.Istio{}).
							Should(HaveCondition(v1alpha1.IstioConditionReady, metav1.ConditionTrue), "Istio is not Ready on Cluster #1; unexpected Condition")
						Success("Istio CR is Ready on Cluster #1")

						Eventually(common.GetObject).
							WithArguments(ctx, clRemote, kube.Key(istioName), &v1alpha1.Istio{}).
							Should(HaveCondition(v1alpha1.IstioConditionReady, metav1.ConditionTrue), "Istio is not Ready on Cluster #2; unexpected Condition")
						Success("Istio CR is Ready on Cluster #1")
					})

					It("deploys istiod", func(ctx SpecContext) {
						Eventually(common.GetObject).
							WithArguments(ctx, clPrimary, kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(HaveCondition(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Istiod is not Available on Cluster #1; unexpected Condition")
						Expect(common.GetVersionFromIstiod()).To(Equal(version.Version), "Unexpected istiod version")
						Success("Istiod is deployed in the namespace and Running on Cluster #1")

						Eventually(common.GetObject).
							WithArguments(ctx, clRemote, kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(HaveCondition(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Istiod is not Available on Cluster #2; unexpected Condition")
						Expect(common.GetVersionFromIstiod()).To(Equal(version.Version), "Unexpected istiod version")
						Success("Istiod is deployed in the namespace and Running on  Cluster #2")
					})
				})

				When("Gateway is created in both clusters", func() {
					BeforeAll(func(ctx SpecContext) {
						Expect(kubectlClient1.SetNamespace(controlPlaneNamespace).Apply(eastGatewayYAML)).To(Succeed(), "Gateway creation failed on Cluster #1")

						Expect(kubectlClient2.SetNamespace(controlPlaneNamespace).Apply(westGatewayYAML)).To(Succeed(), "Gateway creation failed on  Cluster #2")

						// Expose the Gateway service in both clusters
						Expect(kubectlClient1.SetNamespace(controlPlaneNamespace).Apply(exposeServiceYAML)).To(Succeed(), "Expose Service creation failed on Cluster #1")
						Expect(kubectlClient2.SetNamespace(controlPlaneNamespace).Apply(exposeServiceYAML)).To(Succeed(), "Expose Service creation failed on  Cluster #2")
					})

					It("updates both Gateway status to Available", func(ctx SpecContext) {
						Eventually(common.GetObject).
							WithArguments(ctx, clPrimary, kube.Key("istio-eastwestgateway", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(HaveCondition(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Gateway is not Ready on Cluster #1; unexpected Condition")

						Eventually(common.GetObject).
							WithArguments(ctx, clRemote, kube.Key("istio-eastwestgateway", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(HaveCondition(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Gateway is not Ready on Cluster #2; unexpected Condition")
						Success("Gateway is created and available in both clusters")
					})
				})

				When("are installed remote secrets on each cluster", func() {
					BeforeAll(func(ctx SpecContext) {
						// Get the internal IP of the control plane node in both clusters
						internalIPCluster1, err := kubectlClient1.GetInternalIP("node-role.kubernetes.io/control-plane")
						Expect(err).NotTo(HaveOccurred())
						Expect(internalIPCluster1).NotTo(BeEmpty(), "Internal IP is empty for Cluster #1")

						internalIPCluster2, err := kubectlClient2.GetInternalIP("node-role.kubernetes.io/control-plane")
						Expect(internalIPCluster2).NotTo(BeEmpty(), "Internal IP is empty for  Cluster #2")
						Expect(err).NotTo(HaveOccurred())

						// Install a remote secret in Cluster #1 that provides access to the  Cluster #2 API server.
						secret, err := istioctl.CreateRemoteSecret(kubeconfig2, "cluster2", internalIPCluster2)
						Expect(err).NotTo(HaveOccurred())
						Expect(kubectlClient1.ApplyString(secret)).To(Succeed(), "Remote secret creation failed on Cluster #1")

						// Install a remote secret in  Cluster #2 that provides access to the Cluster #1 API server.
						secret, err = istioctl.CreateRemoteSecret(kubeconfig, "cluster1", internalIPCluster1)
						Expect(err).NotTo(HaveOccurred())
						Expect(kubectlClient2.ApplyString(secret)).To(Succeed(), "Remote secret creation failed on Cluster #1")
					})

					It("remote secrets are created", func(ctx SpecContext) {
						secret, err := common.GetObject(ctx, clPrimary, kube.Key("istio-remote-secret-cluster2", controlPlaneNamespace), &corev1.Secret{})
						Expect(err).NotTo(HaveOccurred())
						Expect(secret).NotTo(BeNil(), "Secret is not created on Cluster #1")

						secret, err = common.GetObject(ctx, clRemote, kube.Key("istio-remote-secret-cluster1", controlPlaneNamespace), &corev1.Secret{})
						Expect(err).NotTo(HaveOccurred())
						Expect(secret).NotTo(BeNil(), "Secret is not created on  Cluster #2")
						Success("Remote secrets are created in both clusters")
					})
				})

				When("sample apps are deployed in both clusters", func() {
					BeforeAll(func(ctx SpecContext) {
						// Deploy the sample app in both clusters
						deploySampleApp("sample", version)
						Success("Sample app is deployed in both clusters")
					})

					It("updates the pods status to Ready", func(ctx SpecContext) {
						samplePodsCluster1 := &corev1.PodList{}

						Expect(clPrimary.List(ctx, samplePodsCluster1, client.InNamespace("sample"))).To(Succeed())
						Expect(samplePodsCluster1.Items).ToNot(BeEmpty(), "No pods found in bookinfo namespace")

						for _, pod := range samplePodsCluster1.Items {
							Eventually(common.GetObject).
								WithArguments(ctx, clPrimary, kube.Key(pod.Name, "sample"), &corev1.Pod{}).
								Should(HaveCondition(corev1.PodReady, metav1.ConditionTrue), "Pod is not Ready on Cluster #1; unexpected Condition")
						}

						samplePodsCluster2 := &corev1.PodList{}
						Expect(clRemote.List(ctx, samplePodsCluster2, client.InNamespace("sample"))).To(Succeed())
						Expect(samplePodsCluster2.Items).ToNot(BeEmpty(), "No pods found in bookinfo namespace")

						for _, pod := range samplePodsCluster2.Items {
							Eventually(common.GetObject).
								WithArguments(ctx, clRemote, kube.Key(pod.Name, "sample"), &corev1.Pod{}).
								Should(HaveCondition(corev1.PodReady, metav1.ConditionTrue), "Pod is not Ready on Cluster #2; unexpected Condition")
						}
						Success("Sample app is created in both clusters and Running")
					})

					It("can access the sample app from both clusters", func(ctx SpecContext) {
						sleepPodNameCluster1, err := common.GetPodNameByLabel(ctx, clPrimary, "sample", "app", "sleep")
						Expect(sleepPodNameCluster1).NotTo(BeEmpty(), "Sleep pod not found on Cluster #1")
						Expect(err).NotTo(HaveOccurred(), "Error getting sleep pod name on Cluster #1")

						sleepPodNameCluster2, err := common.GetPodNameByLabel(ctx, clRemote, "sample", "app", "sleep")
						Expect(sleepPodNameCluster2).NotTo(BeEmpty(), "Sleep pod not found on  Cluster #2")
						Expect(err).NotTo(HaveOccurred(), "Error getting sleep pod name on  Cluster #2")

						// Run the curl command from the sleep pod in the  Cluster #2 and get response list to validate that we get responses from both clusters
						Cluster2Responses := strings.Join(getListCurlResponses(kubectlClient2, sleepPodNameCluster2), "\n")
						Expect(Cluster2Responses).To(ContainSubstring("Hello version: v1"), "Responses from  Cluster #2 are not the expected")
						Expect(Cluster2Responses).To(ContainSubstring("Hello version: v2"), "Responses from  Cluster #2 are not the expected")

						// Run the curl command from the sleep pod in the Cluster #1 and get response list to validate that we get responses from both clusters
						Cluster1Responses := strings.Join(getListCurlResponses(kubectlClient1, sleepPodNameCluster1), "\n")
						Expect(Cluster1Responses).To(ContainSubstring("Hello version: v1"), "Responses from Cluster #1 are not the expected")
						Expect(Cluster1Responses).To(ContainSubstring("Hello version: v2"), "Responses from Cluster #1 are not the expected")
						Success("Sample app is accessible from both clusters")
					})
				})

				When("istio CR is deleted in both clusters", func() {
					BeforeEach(func() {
						// Delete the Istio CR in both clusters
						Expect(kubectlClient1.SetNamespace(controlPlaneNamespace).Delete("istio", istioName)).To(Succeed(), "Istio CR failed to be deleted")
						Expect(kubectlClient2.SetNamespace(controlPlaneNamespace).Delete("istio", istioName)).To(Succeed(), "Istio CR failed to be deleted")
						Success("Istio CR is deleted in both clusters")
					})

					It("removes istiod pod", func(ctx SpecContext) {
						// Check istiod pod is deleted in both clusters
						Eventually(clPrimary.Get).WithArguments(ctx, kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(ReturnNotFoundError(), "Istiod should not exist anymore on Cluster #1")
						Eventually(clRemote.Get).WithArguments(ctx, kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(ReturnNotFoundError(), "Istiod should not exist anymore on Cluster #2")
					})
				})

				AfterAll(func(ctx SpecContext) {
					// Delete namespace to ensure clean up for new tests iteration
					Expect(kubectlClient1.DeleteNamespace(controlPlaneNamespace)).To(Succeed(), "Namespace failed to be deleted on Cluster #1")
					Expect(kubectlClient2.DeleteNamespace(controlPlaneNamespace)).To(Succeed(), "Namespace failed to be deleted on Cluster #2")

					common.CheckNamespaceEmpty(ctx, clPrimary, controlPlaneNamespace)
					common.CheckNamespaceEmpty(ctx, clRemote, controlPlaneNamespace)
					Success("ControlPlane Namespaces are empty")

					// Delete the entire sample namespace in both clusters
					Expect(kubectlClient1.DeleteNamespace("sample")).To(Succeed(), "Namespace failed to be deleted on Cluster #1")
					Expect(kubectlClient2.DeleteNamespace("sample")).To(Succeed(), "Namespace failed to be deleted on  Cluster #2")

					common.CheckNamespaceEmpty(ctx, clPrimary, "sample")
					common.CheckNamespaceEmpty(ctx, clRemote, "sample")
					Success("Sample app is deleted in both clusters")
				})
			})
		}
	})

	AfterAll(func(ctx SpecContext) {
		// Delete the Sail Operator from both clusters
		Expect(kubectlClient1.DeleteNamespace(namespace)).To(Succeed(), "Namespace failed to be deleted on Cluster #1")
		Expect(kubectlClient2.DeleteNamespace(namespace)).To(Succeed(), "Namespace failed to be deleted on  Cluster #2")

		// Delete the intermediate CA from both clusters
		common.CheckNamespaceEmpty(ctx, clPrimary, namespace)
		common.CheckNamespaceEmpty(ctx, clRemote, namespace)
	})
})

// deploySampleApp deploys the sample app in the given cluster
func deploySampleApp(ns string, istioVersion supportedversion.VersionInfo) {
	// Create the namespace
	Expect(kubectlClient1.CreateNamespace(ns)).To(Succeed(), "Namespace failed to be created")
	Expect(kubectlClient2.CreateNamespace(ns)).To(Succeed(), "Namespace failed to be created")

	// Label the namespace
	Expect(kubectlClient1.Patch("namespace", ns, "merge", `{"metadata":{"labels":{"istio-injection":"enabled"}}}`)).
		To(Succeed(), "Error patching sample namespace")
	Expect(kubectlClient2.Patch("namespace", ns, "merge", `{"metadata":{"labels":{"istio-injection":"enabled"}}}`)).
		To(Succeed(), "Error patching sample namespace")

	version := istioVersion.Version
	// Deploy the sample app from upstream URL in both clusters
	if istioVersion.Name == "latest" {
		version = "master"
	}
	helloWorldURL := fmt.Sprintf("https://raw.githubusercontent.com/istio/istio/%s/samples/helloworld/helloworld.yaml", version)
	Expect(kubectlClient1.SetNamespace(ns).ApplyWithLabels(helloWorldURL, "service=helloworld")).To(Succeed(), "Sample service deploy failed on Cluster #1")
	Expect(kubectlClient2.SetNamespace(ns).ApplyWithLabels(helloWorldURL, "service=helloworld")).To(Succeed(), "Sample service deploy failed on  Cluster #2")

	Expect(kubectlClient1.SetNamespace(ns).ApplyWithLabels(helloWorldURL, "version=v1")).To(Succeed(), "Sample service deploy failed on Cluster #1")
	Expect(kubectlClient2.SetNamespace(ns).ApplyWithLabels(helloWorldURL, "version=v2")).To(Succeed(), "Sample service deploy failed on  Cluster #2")

	sleepURL := fmt.Sprintf("https://raw.githubusercontent.com/istio/istio/%s/samples/sleep/sleep.yaml", version)
	Expect(kubectlClient1.SetNamespace(ns).Apply(sleepURL)).To(Succeed(), "Sample sleep deploy failed on Cluster #1")
	Expect(kubectlClient2.SetNamespace(ns).Apply(sleepURL)).To(Succeed(), "Sample sleep deploy failed on  Cluster #2")
}

// getListCurlResponses runs the curl command 10 times from the sleep pod in the given cluster and get response list
func getListCurlResponses(k *kubectl.KubectlBuilder, podName string) []string {
	var responses []string
	for i := 0; i < 10; i++ {
		response, err := k.SetNamespace("sample").Exec(podName, "sleep", "curl -sS helloworld.sample:5000/hello")
		Expect(err).NotTo(HaveOccurred())
		responses = append(responses, response)
	}
	return responses
}
