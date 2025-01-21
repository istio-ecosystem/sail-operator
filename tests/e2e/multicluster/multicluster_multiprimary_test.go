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
	"time"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversions"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/certs"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	. "github.com/istio-ecosystem/sail-operator/tests/e2e/util/gomega"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/istioctl"
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
	debugInfoLogged := false

	BeforeAll(func(ctx SpecContext) {
		if !skipDeploy {
			// Deploy the Sail Operator on both clusters
			Expect(k1.CreateNamespace(namespace)).To(Succeed(), "Namespace failed to be created on Cluster #1")
			Expect(k2.CreateNamespace(namespace)).To(Succeed(), "Namespace failed to be created on Cluster #2")

			Expect(common.InstallOperatorViaHelm("--kubeconfig", kubeconfig)).
				To(Succeed(), "Operator failed to be deployed in Cluster #1")

			Expect(common.InstallOperatorViaHelm("--kubeconfig "+kubeconfig2)).
				To(Succeed(), "Operator failed to be deployed in Cluster #2")

			Eventually(common.GetObject).
				WithArguments(ctx, clPrimary, kube.Key(deploymentName, namespace), &appsv1.Deployment{}).
				Should(HaveCondition(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Error getting Istio CRD")
			Success("Operator is deployed in the Cluster #1 namespace and Running")

			Eventually(common.GetObject).
				WithArguments(ctx, clRemote, kube.Key(deploymentName, namespace), &appsv1.Deployment{}).
				Should(HaveCondition(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Error getting Istio CRD")
			Success("Operator is deployed in the Cluster #2 namespace and Running")
		}
	})

	Describe("Multi-Primary Multi-Network configuration", func() {
		// Test the Multi-Primary Multi-Network configuration for each supported Istio version
		for _, version := range istioversions.List {
			Context(fmt.Sprintf("Istio version %s", version.Version), func() {
				When("Istio resources are created in both clusters", func() {
					BeforeAll(func(ctx SpecContext) {
						Expect(k1.CreateNamespace(controlPlaneNamespace)).To(Succeed(), "Namespace failed to be created")
						Expect(k2.CreateNamespace(controlPlaneNamespace)).To(Succeed(), "Namespace failed to be created")

						// Push the intermediate CA to both clusters
						Expect(certs.PushIntermediateCA(k1, controlPlaneNamespace, "east", "network1", artifacts, clPrimary)).To(Succeed())
						Expect(certs.PushIntermediateCA(k2, controlPlaneNamespace, "west", "network2", artifacts, clRemote)).To(Succeed())

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
apiVersion: sailoperator.io/v1
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
						Expect(k1.CreateFromString(multiclusterCluster1YAML)).To(Succeed(), "Istio Resource creation failed on Cluster #1")

						multiclusterCluster2YAML := fmt.Sprintf(multiclusterYAML, version.Name, controlPlaneNamespace, "mesh1", "cluster2", "network2")
						Log("Istio CR Cluster #2: ", multiclusterCluster2YAML)
						Expect(k2.CreateFromString(multiclusterCluster2YAML)).To(Succeed(), "Istio Resource creation failed on Cluster #2")
					})

					It("updates both Istio CR status to Ready", func(ctx SpecContext) {
						Eventually(common.GetObject).
							WithArguments(ctx, clPrimary, kube.Key(istioName), &v1.Istio{}).
							Should(HaveCondition(v1.IstioConditionReady, metav1.ConditionTrue), "Istio is not Ready on Cluster #1; unexpected Condition")
						Success("Istio CR is Ready on Cluster #1")

						Eventually(common.GetObject).
							WithArguments(ctx, clRemote, kube.Key(istioName), &v1.Istio{}).
							Should(HaveCondition(v1.IstioConditionReady, metav1.ConditionTrue), "Istio is not Ready on Cluster #2; unexpected Condition")
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
						Success("Istiod is deployed in the namespace and Running on Cluster #2")
					})
				})

				When("Gateway is created in both clusters", func() {
					BeforeAll(func(ctx SpecContext) {
						Expect(k1.WithNamespace(controlPlaneNamespace).Apply(eastGatewayYAML)).To(Succeed(), "Gateway creation failed on Cluster #1")
						Expect(k2.WithNamespace(controlPlaneNamespace).Apply(westGatewayYAML)).To(Succeed(), "Gateway creation failed on Cluster #2")

						// Expose the Gateway service in both clusters
						Expect(k1.WithNamespace(controlPlaneNamespace).Apply(exposeServiceYAML)).To(Succeed(), "Expose Service creation failed on Cluster #1")
						Expect(k2.WithNamespace(controlPlaneNamespace).Apply(exposeServiceYAML)).To(Succeed(), "Expose Service creation failed on Cluster #2")
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
						internalIPCluster1, err := k1.GetInternalIP("node-role.kubernetes.io/control-plane")
						Expect(err).NotTo(HaveOccurred())
						Expect(internalIPCluster1).NotTo(BeEmpty(), "Internal IP is empty for Cluster #1")

						internalIPCluster2, err := k2.GetInternalIP("node-role.kubernetes.io/control-plane")
						Expect(internalIPCluster2).NotTo(BeEmpty(), "Internal IP is empty for Cluster #2")
						Expect(err).NotTo(HaveOccurred())

						// Install a remote secret in Cluster #1 that provides access to the  Cluster #2 API server.
						secret, err := istioctl.CreateRemoteSecret(kubeconfig2, "cluster2", internalIPCluster2)
						Expect(err).NotTo(HaveOccurred())
						Expect(k1.ApplyString(secret)).To(Succeed(), "Remote secret creation failed on Cluster #1")

						// Install a remote secret in  Cluster #2 that provides access to the Cluster #1 API server.
						secret, err = istioctl.CreateRemoteSecret(kubeconfig, "cluster1", internalIPCluster1)
						Expect(err).NotTo(HaveOccurred())
						Expect(k2.ApplyString(secret)).To(Succeed(), "Remote secret creation failed on Cluster #1")
					})

					It("remote secrets are created", func(ctx SpecContext) {
						secret, err := common.GetObject(ctx, clPrimary, kube.Key("istio-remote-secret-cluster2", controlPlaneNamespace), &corev1.Secret{})
						Expect(err).NotTo(HaveOccurred())
						Expect(secret).NotTo(BeNil(), "Secret is not created on Cluster #1")

						secret, err = common.GetObject(ctx, clRemote, kube.Key("istio-remote-secret-cluster1", controlPlaneNamespace), &corev1.Secret{})
						Expect(err).NotTo(HaveOccurred())
						Expect(secret).NotTo(BeNil(), "Secret is not created on Cluster #2")
						Success("Remote secrets are created in both clusters")
					})
				})

				When("sample apps are deployed in both clusters", func() {
					BeforeAll(func(ctx SpecContext) {
						// Create the namespace
						Expect(k1.CreateNamespace("sample")).To(Succeed(), "Namespace failed to be created")
						Expect(k2.CreateNamespace("sample")).To(Succeed(), "Namespace failed to be created")

						// Label the sample namespace
						Expect(k1.Patch("namespace", "sample", "merge", `{"metadata":{"labels":{"istio-injection":"enabled"}}}`)).
							To(Succeed(), "Error patching sample namespace")
						Expect(k2.Patch("namespace", "sample", "merge", `{"metadata":{"labels":{"istio-injection":"enabled"}}}`)).
							To(Succeed(), "Error patching sample namespace")

						// Deploy the sample app in both clusters
						helloWorldURL := common.GetSampleYAML(version, "helloworld")
						sleepURL := common.GetSampleYAML(version, "sleep")

						// On Cluster 0, create a service for the helloworld app v1
						Expect(k1.WithNamespace("sample").ApplyWithLabels(helloWorldURL, "service=helloworld")).To(Succeed(), "Failed to deploy helloworld service")
						Expect(k1.WithNamespace("sample").ApplyWithLabels(helloWorldURL, "version=v1")).To(Succeed(), "Failed to deploy helloworld service")
						Expect(k1.WithNamespace("sample").Apply(sleepURL)).To(Succeed(), "Failed to deploy sleep service")

						// On Cluster 1, create a service for the helloworld app v2
						Expect(k2.WithNamespace("sample").ApplyWithLabels(helloWorldURL, "service=helloworld")).To(Succeed(), "Failed to deploy helloworld service")
						Expect(k2.WithNamespace("sample").ApplyWithLabels(helloWorldURL, "version=v2")).To(Succeed(), "Failed to deploy helloworld service")
						Expect(k2.WithNamespace("sample").Apply(sleepURL)).To(Succeed(), "Failed to deploy sleep service")
						Success("Sample app is deployed in both clusters")
					})

					It("updates the pods status to Ready", func(ctx SpecContext) {
						samplePodsCluster1 := &corev1.PodList{}

						Expect(clPrimary.List(ctx, samplePodsCluster1, client.InNamespace("sample"))).To(Succeed())
						Expect(samplePodsCluster1.Items).ToNot(BeEmpty(), "No pods found in sample namespace")

						for _, pod := range samplePodsCluster1.Items {
							Eventually(common.GetObject).
								WithArguments(ctx, clPrimary, kube.Key(pod.Name, "sample"), &corev1.Pod{}).
								Should(HaveCondition(corev1.PodReady, metav1.ConditionTrue), "Pod is not Ready on Cluster #1; unexpected Condition")
						}

						samplePodsCluster2 := &corev1.PodList{}
						Expect(clRemote.List(ctx, samplePodsCluster2, client.InNamespace("sample"))).To(Succeed())
						Expect(samplePodsCluster2.Items).ToNot(BeEmpty(), "No pods found in sample namespace")

						for _, pod := range samplePodsCluster2.Items {
							Eventually(common.GetObject).
								WithArguments(ctx, clRemote, kube.Key(pod.Name, "sample"), &corev1.Pod{}).
								Should(HaveCondition(corev1.PodReady, metav1.ConditionTrue), "Pod is not Ready on Cluster #2; unexpected Condition")
						}
						Success("Sample app is created in both clusters and Running")
					})

					It("can access the sample app from both clusters", func(ctx SpecContext) {
						verifyResponsesAreReceivedFromBothClusters(k1, "Cluster #1")
						verifyResponsesAreReceivedFromBothClusters(k2, "Cluster #2")
						Success("Sample app is accessible from both clusters")
					})
				})

				When("istio CR is deleted in both clusters", func() {
					BeforeEach(func() {
						// Delete the Istio CR in both clusters
						Expect(k1.WithNamespace(controlPlaneNamespace).Delete("istio", istioName)).To(Succeed(), "Istio CR failed to be deleted")
						Expect(k2.WithNamespace(controlPlaneNamespace).Delete("istio", istioName)).To(Succeed(), "Istio CR failed to be deleted")
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
					if CurrentSpecReport().Failed() {
						common.LogDebugInfo(common.MultiCluster, k1, k2)
						debugInfoLogged = true
					}

					// Delete namespaces to ensure clean up for new tests iteration
					Expect(k1.DeleteNamespaceNoWait(controlPlaneNamespace)).To(Succeed(), "Namespace failed to be deleted on Cluster #1")
					Expect(k2.DeleteNamespaceNoWait(controlPlaneNamespace)).To(Succeed(), "Namespace failed to be deleted on Cluster #2")
					Expect(k1.DeleteNamespaceNoWait("sample")).To(Succeed(), "Namespace failed to be deleted on Cluster #1")
					Expect(k2.DeleteNamespaceNoWait("sample")).To(Succeed(), "Namespace failed to be deleted on Cluster #2")

					Expect(k1.WaitNamespaceDeleted(controlPlaneNamespace)).To(Succeed())
					Expect(k2.WaitNamespaceDeleted(controlPlaneNamespace)).To(Succeed())
					Success("ControlPlane Namespaces are empty")

					Expect(k1.WaitNamespaceDeleted("sample")).To(Succeed())
					Expect(k2.WaitNamespaceDeleted("sample")).To(Succeed())
					Success("Sample app is deleted in both clusters")
				})
			})
		}
	})

	AfterAll(func(ctx SpecContext) {
		if CurrentSpecReport().Failed() && !debugInfoLogged {
			common.LogDebugInfo(common.MultiCluster, k1, k2)
			debugInfoLogged = true
		}

		// Delete the Sail Operator from both clusters
		Expect(k1.DeleteNamespaceNoWait(namespace)).To(Succeed(), "Namespace failed to be deleted on Cluster #1")
		Expect(k2.DeleteNamespaceNoWait(namespace)).To(Succeed(), "Namespace failed to be deleted on Cluster #2")
		Expect(k1.WaitNamespaceDeleted(namespace)).To(Succeed())
		Expect(k2.WaitNamespaceDeleted(namespace)).To(Succeed())
	})
})
