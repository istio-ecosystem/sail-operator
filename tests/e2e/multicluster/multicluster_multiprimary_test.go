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
	"fmt"
	"time"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/pkg/version"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/cleaner"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/istioctl"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Multicluster deployment models", Label("multicluster", "multicluster-multiprimary"), Ordered, func() {
	SetDefaultEventuallyTimeout(180 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)

	Context("Sidecar", func() {
		generateMultiPrimaryTestCases("default")
	})
	Context("Ambient", Label("ambient"), func() {
		generateMultiPrimaryTestCases("ambient")
	})
})

func generateMultiPrimaryTestCases(profile string) {
	Describe("Multi-Primary Multi-Network configuration", func() {
		// Test the Multi-Primary Multi-Network configuration for each supported Istio version
		for _, v := range istioversion.GetLatestPatchVersions() {
			// Ambient multi-cluster is supported only since 1.27
			if profile == "ambient" && version.Constraint("<1.27").Check(v.Version) {
				Log(fmt.Sprintf("Skipping test, because Istio version %s does not support Ambient Multi-Cluster configuration", v.Version))
				continue
			}

			Context(fmt.Sprintf("Istio version %s", v.Version), func() {
				clr1 := cleaner.New(clPrimary, "cluster=primary")
				clr2 := cleaner.New(clRemote, "cluster=remote")

				BeforeAll(func(ctx SpecContext) {
					clr1.Record(ctx)
					clr2.Record(ctx)
				})

				When("Istio and IstioCNI resources are created in both clusters", func() {
					BeforeAll(func(ctx SpecContext) {
						createIstioNamespaces(k1, "network1", profile)
						createIstioNamespaces(k2, "network2", profile)

						// Push the intermediate CA to both clusters
						createIntermediateCA(k1, "east", "network1", artifacts, clPrimary)
						createIntermediateCA(k2, "west", "network2", artifacts, clRemote)

						// Wait for the secret to be created in both clusters
						awaitSecretCreation(k1.ClusterName, clPrimary)
						awaitSecretCreation(k2.ClusterName, clRemote)

						createIstioResources(k1, v.Name, "cluster1", "network1", profile)
						createIstioResources(k2, v.Name, "cluster2", "network2", profile)
					})

					It("updates both Istio CR status to Ready", func(ctx SpecContext) {
						common.AwaitCondition(ctx, v1.IstioConditionReady, kube.Key(istioName), &v1.Istio{}, k1, clPrimary)
						common.AwaitCondition(ctx, v1.IstioConditionReady, kube.Key(istioName), &v1.Istio{}, k2, clRemote)
					})

					It("updates both IstioCNI CR status to Ready", func(ctx SpecContext) {
						common.AwaitCondition(ctx, v1.IstioCNIConditionReady, kube.Key(istioCniName), &v1.IstioCNI{}, k1, clPrimary)
						common.AwaitCondition(ctx, v1.IstioCNIConditionReady, kube.Key(istioCniName), &v1.IstioCNI{}, k2, clRemote)
					})

					It("deploys istiod", func(ctx SpecContext) {
						common.AwaitDeployment(ctx, "istiod", k1, clPrimary)
						Expect(common.GetVersionFromIstiod()).To(Equal(v.Version), "Unexpected istiod version")

						common.AwaitDeployment(ctx, "istiod", k2, clRemote)
						Expect(common.GetVersionFromIstiod()).To(Equal(v.Version), "Unexpected istiod version")
					})

					It("deploys istio-cni-node", func(ctx SpecContext) {
						common.AwaitCniDaemonSet(ctx, k1, clPrimary)
						common.AwaitCniDaemonSet(ctx, k2, clRemote)
					})
				})

				When("Gateway is created in both clusters", func() {
					BeforeAll(func(ctx SpecContext) {
						if profile == "ambient" {
							common.CreateAmbientGateway(k1, controlPlaneNamespace, "network1")
							common.CreateAmbientGateway(k2, controlPlaneNamespace, "network2")
						} else {
							Expect(k1.WithNamespace(controlPlaneNamespace).Apply(eastGatewayYAML)).To(Succeed(), "Gateway creation failed on Cluster #1")
							Expect(k2.WithNamespace(controlPlaneNamespace).Apply(westGatewayYAML)).To(Succeed(), "Gateway creation failed on Cluster #2")

							// Expose the Gateway service in both clusters
							Expect(k1.WithNamespace(controlPlaneNamespace).Apply(exposeServiceYAML)).To(Succeed(), "Expose Service creation failed on Cluster #1")
							Expect(k2.WithNamespace(controlPlaneNamespace).Apply(exposeServiceYAML)).To(Succeed(), "Expose Service creation failed on Cluster #2")
						}
					})

					It("updates both Gateway status to Available", func(ctx SpecContext) {
						common.AwaitDeployment(ctx, "istio-eastwestgateway", k1, clPrimary)
						common.AwaitDeployment(ctx, "istio-eastwestgateway", k2, clRemote)
						Success("Gateway is created and available in both clusters")
					})
				})

				When("are installed remote secrets on each cluster", func() {
					BeforeAll(func(ctx SpecContext) {
						// Get the cluster API URL in both clusters
						apiURLCluster1, err := k1.GetClusterAPIURL()
						Expect(err).ToNot(HaveOccurred())
						Expect(apiURLCluster1).NotTo(BeEmpty(), "API URL is empty for Cluster #1")

						apiURLCluster2, err := k2.GetClusterAPIURL()
						Expect(err).ToNot(HaveOccurred())
						Expect(apiURLCluster2).NotTo(BeEmpty(), "API URL is empty for Cluster #2")

						// Install a remote secret in Cluster #1 that provides access to the  Cluster #2 API server.
						secret, err := istioctl.CreateRemoteSecret(kubeconfig2, controlPlaneNamespace, "cluster2", apiURLCluster2)
						Expect(err).NotTo(HaveOccurred())
						Expect(k1.ApplyString(secret)).To(Succeed(), "Remote secret creation failed on Cluster #1")

						// Install a remote secret in  Cluster #2 that provides access to the Cluster #1 API server.
						secret, err = istioctl.CreateRemoteSecret(kubeconfig, controlPlaneNamespace, "cluster1", apiURLCluster1)
						Expect(err).NotTo(HaveOccurred())
						Expect(k2.ApplyString(secret)).To(Succeed(), "Remote secret creation failed on Cluster #2")
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
						deploySampleAppToClusters(sampleNamespace, profile, []ClusterDeployment{
							{Kubectl: k1, AppVersion: "v1"},
							{Kubectl: k2, AppVersion: "v2"},
						})
						Success("Sample app is deployed in both clusters")
					})

					It("updates the pods status to Ready", func(ctx SpecContext) {
						Eventually(common.CheckSamplePodsReady).WithArguments(ctx, clPrimary).Should(Succeed(), "Error checking status of sample pods on Cluster #1")
						Eventually(common.CheckSamplePodsReady).WithArguments(ctx, clRemote).Should(Succeed(), "Error checking status of sample pods on Cluster #2")
						Success("Sample app is created in both clusters and Running")
					})

					It("can access the sample app from both clusters", func(ctx SpecContext) {
						verifyResponsesAreReceivedFromExpectedVersions(k1)
						verifyResponsesAreReceivedFromExpectedVersions(k2)
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

				When("istio CNI CR is deleted in both clusters", func() {
					BeforeEach(func() {
						// Delete the IstioCNI CR in both clusters
						Expect(k1.WithNamespace(istioCniNamespace).Delete("istiocni", istioCniName)).To(Succeed(), "Istio CNI CR failed to be deleted")
						Expect(k2.WithNamespace(istioCniNamespace).Delete("istiocni", istioCniName)).To(Succeed(), "Istio CNI CR failed to be deleted")
						Success("IstioCNI CR is deleted in both clusters")
					})

					It("removes istio-cni-node pods", func(ctx SpecContext) {
						// Check istio-cni-node pods are deleted in both clusters
						daemonset := &appsv1.DaemonSet{}
						Eventually(clPrimary.Get).WithArguments(ctx, kube.Key("istio-cni-node", istioCniNamespace), daemonset).
							Should(ReturnNotFoundError(), "IstioCNI DaemonSet should not exist anymore on Cluster #1")
						Eventually(clRemote.Get).WithArguments(ctx, kube.Key("istio-cni-node", istioCniNamespace), daemonset).
							Should(ReturnNotFoundError(), "IstioCNI DaemonSet should not exist anymore on Cluster #2")
					})
				})

				AfterAll(func(ctx SpecContext) {
					if CurrentSpecReport().Failed() {
						common.LogDebugInfo(common.MultiCluster, k1, k2)
						debugInfoLogged = true
						if keepOnFailure {
							return
						}
					}

					c1Deleted := clr1.CleanupNoWait(ctx)
					c2Deleted := clr2.CleanupNoWait(ctx)
					clr1.WaitForDeletion(ctx, c1Deleted)
					clr2.WaitForDeletion(ctx, c2Deleted)
				})
			})
		}
	})
}
