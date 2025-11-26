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
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/pkg/version"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/certs"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/cleaner"
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

var _ = Describe("Multicluster deployment models", Label("multicluster", "multicluster-primaryremote"), Ordered, func() {
	SetDefaultEventuallyTimeout(180 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)

	Describe("Primary-Remote - Multi-Network configuration", func() {
		// Test the Primary-Remote - Multi-Network configuration for each supported Istio version
		for _, v := range istioversion.GetLatestPatchVersions() {
			// The Primary-Remote - Multi-Network configuration is only supported in Istio 1.24+.
			if version.Constraint("<1.24").Check(v.Version) {
				Log(fmt.Sprintf("Skipping test, because Istio version %s does not support Primary-Remote Multi-Network configuration", v.Version))
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
						Expect(k1.CreateNamespace(controlPlaneNamespace)).To(Succeed(), "Istio namespace failed to be created")
						Expect(k2.CreateNamespace(controlPlaneNamespace)).To(Succeed(), "Istio namespace failed to be created")
						Expect(k1.CreateNamespace(istioCniNamespace)).To(Succeed(), "Istio CNI Namespace failed to be created")
						Expect(k2.CreateNamespace(istioCniNamespace)).To(Succeed(), "Istio CNI Namespace failed to be created")

						// Push the intermediate CA to both clusters
						Expect(certs.PushIntermediateCA(k1, controlPlaneNamespace, "east", "network1", artifacts, clPrimary)).
							To(Succeed(), "Error pushing intermediate CA to Primary Cluster")
						Expect(certs.PushIntermediateCA(k2, controlPlaneNamespace, "west", "network2", artifacts, clRemote)).
							To(Succeed(), "Error pushing intermediate CA to Remote Cluster")

						// Wait for the secret to be created in both clusters
						Eventually(func() error {
							_, err := common.GetObject(context.Background(), clPrimary, kube.Key("cacerts", controlPlaneNamespace), &corev1.Secret{})
							return err
						}).ShouldNot(HaveOccurred(), "Secret is not created on Primary Cluster")

						Eventually(func() error {
							_, err := common.GetObject(context.Background(), clRemote, kube.Key("cacerts", controlPlaneNamespace), &corev1.Secret{})
							return err
						}).ShouldNot(HaveOccurred(), "Secret is not created on Primary Cluster")

						common.CreateIstioCNI(k1, v.Name)

						spec := `
values:
  pilot:
    env:
      EXTERNAL_ISTIOD: "true"
  global:
    meshID: mesh1
    multiCluster:
      clusterName: cluster1
    network: network1`
						common.CreateIstio(k1, v.Name, spec)
					})

					It("updates Istio CR on Primary cluster status to Ready", func(ctx SpecContext) {
						Eventually(common.GetObject).
							WithArguments(ctx, clPrimary, kube.Key(istioName), &v1.Istio{}).
							Should(HaveConditionStatus(v1.IstioConditionReady, metav1.ConditionTrue), "Istio is not Ready on Primary; unexpected Condition")
						Success("Istio CR is Ready on Primary Cluster")
					})

					It("updates IstioCNI CR on Primary cluster status to Ready", func(ctx SpecContext) {
						Eventually(common.GetObject).
							WithArguments(ctx, clPrimary, kube.Key(istioCniName), &v1.Istio{}).
							Should(HaveConditionStatus(v1.IstioCNIConditionReady, metav1.ConditionTrue), "IstioCNI is not Ready on Primary; unexpected Condition")
						Success("IstioCNI CR is Ready on Primary Cluster")
					})

					It("deploys istiod", func(ctx SpecContext) {
						Eventually(common.GetObject).
							WithArguments(ctx, clPrimary, kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(HaveConditionStatus(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Istiod is not Available on Primary; unexpected Condition")
						Expect(common.GetVersionFromIstiod()).To(Equal(v.Version), "Unexpected istiod version")
						Success("Istiod is deployed in the namespace and Running on Primary Cluster")
					})

					It("deploys istio-cni-node", func(ctx SpecContext) {
						Eventually(func() bool {
							daemonset := &appsv1.DaemonSet{}
							if err := clPrimary.Get(ctx, kube.Key("istio-cni-node", istioCniNamespace), daemonset); err != nil {
								return false
							}
							return daemonset.Status.NumberAvailable == daemonset.Status.CurrentNumberScheduled
						}).Should(BeTrue(), "IstioCNI DaemonSet Pods are not Available on Primary Cluster")
						Success("IstioCNI DaemonSet is deployed in the namespace and Running on Primary Cluster")
					})
				})

				When("Gateway is created on Primary cluster ", func() {
					BeforeAll(func(ctx SpecContext) {
						Expect(k1.WithNamespace(controlPlaneNamespace).Apply(eastGatewayYAML)).To(Succeed(), "Gateway creation failed on Primary Cluster")

						// Expose istiod service in Primary cluster
						Expect(k1.WithNamespace(controlPlaneNamespace).Apply(exposeIstiodYAML)).To(Succeed(), "Expose Istiod creation failed on Primary Cluster")

						// Expose the Gateway service in both clusters
						Expect(k1.WithNamespace(controlPlaneNamespace).Apply(exposeServiceYAML)).To(Succeed(), "Expose Service creation failed on Primary Cluster")
					})

					It("updates Gateway status to Available", func(ctx SpecContext) {
						Eventually(common.GetObject).
							WithArguments(ctx, clPrimary, kube.Key("istio-eastwestgateway", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(HaveConditionStatus(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Gateway is not Ready on Primary; unexpected Condition")
					})
				})

				When("Istio and IstioCNI are created in Remote cluster", func() {
					BeforeAll(func(ctx SpecContext) {
						common.CreateIstioCNI(k2, v.Name)

						spec := `
values:
  profile: remote
  istiodRemote:
    injectionPath: /inject/cluster/remote/net/network2
  global:
    remotePilotAddress: %s`

						remotePilotAddress := common.GetSVCLoadBalancerAddress(ctx, clPrimary, controlPlaneNamespace, "istio-eastwestgateway")
						Expect(remotePilotAddress).NotTo(BeEmpty(), "Remote Pilot Address is empty")
						Expect(err).NotTo(HaveOccurred(), "Error getting Remote Pilot Address")
						remotePilotIP, err := common.ResolveHostDomainToIP(remotePilotAddress)
						Expect(remotePilotIP).NotTo(BeEmpty(), "Remote Pilot IP is empty")
						Expect(err).NotTo(HaveOccurred(), "Error getting Remote Pilot IP")
						common.CreateIstio(k2, v.Name, fmt.Sprintf(spec, remotePilotIP))

						// Set the controlplane cluster and network for Remote namespace
						By("Patching the istio-system namespace on Remote Cluster")
						Expect(
							k2.Patch(
								"namespace",
								controlPlaneNamespace,
								"merge",
								`{"metadata":{"annotations":{"topology.istio.io/controlPlaneClusters":"cluster1"}}}`)).
							To(Succeed(), "Error patching istio-system namespace")

						// To be able to access the remote cluster from the primary cluster, we need to create a secret in the primary cluster
						// Remote Istio resource will not be Ready until the secret is created
						// Get the cluster API URL of the Remote cluster
						RemoteClusterAPIURL, err := k2.GetClusterAPIURL()
						Expect(RemoteClusterAPIURL).NotTo(BeEmpty(), "API URL is empty for Remote Cluster")
						Expect(err).NotTo(HaveOccurred())

						// Wait for the remote Istio CR to be created, this can be moved to a condition verification, but the resource it not will be Ready at this point
						time.Sleep(5 * time.Second)

						// Install a remote secret in Primary cluster that provides access to the Remote cluster API server.
						By("Creating Remote Secret on Primary Cluster")
						secret, err := istioctl.CreateRemoteSecret(kubeconfig2, controlPlaneNamespace, "remote", RemoteClusterAPIURL)
						Expect(err).NotTo(HaveOccurred())
						Expect(k1.WithNamespace(controlPlaneNamespace).ApplyString(secret)).To(Succeed(), "Remote secret creation failed on Primary Cluster")
					})

					It("secret is created", func(ctx SpecContext) {
						secret, err := common.GetObject(ctx, clPrimary, kube.Key("istio-remote-secret-remote", controlPlaneNamespace), &corev1.Secret{})
						Expect(err).NotTo(HaveOccurred())
						Expect(secret).NotTo(BeNil(), "Secret is not created on Primary Cluster")
						Success("Remote secret is created in Primary cluster")
					})

					It("updates remote Istio CR status to Ready", func(ctx SpecContext) {
						Eventually(common.GetObject, 10*time.Minute).
							WithArguments(ctx, clRemote, kube.Key(istioName), &v1.Istio{}).
							Should(HaveConditionStatus(v1.IstioConditionReady, metav1.ConditionTrue), "Istio is not Ready on Remote; unexpected Condition")
						Success("Istio CR is Ready on Remote Cluster")
					})
				})

				When("gateway is created in Remote cluster", func() {
					BeforeAll(func(ctx SpecContext) {
						Expect(k2.WithNamespace(controlPlaneNamespace).Apply(westGatewayYAML)).To(Succeed(), "Gateway creation failed on Remote Cluster")
						Success("Gateway is created in Remote cluster")
					})

					It("updates Gateway status to Available", func(ctx SpecContext) {
						Eventually(common.GetObject).
							WithArguments(ctx, clRemote, kube.Key("istio-eastwestgateway", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(HaveConditionStatus(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Gateway is not Ready on Remote; unexpected Condition")
						Success("Gateway is created and available in Remote cluster")
					})
				})

				When("sample apps are deployed in both clusters", func() {
					BeforeAll(func(ctx SpecContext) {
						// Create namespace
						Expect(k1.CreateNamespace(sampleNamespace)).To(Succeed(), "Namespace failed to be created on Cluster #1")
						Expect(k2.CreateNamespace(sampleNamespace)).To(Succeed(), "Namespace failed to be created on Cluster #2")

						// Label the namespace
						Expect(k1.Label("namespace", sampleNamespace, "istio-injection", "enabled")).To(Succeed(), "Error labeling sample namespace")
						Expect(k2.Label("namespace", sampleNamespace, "istio-injection", "enabled")).To(Succeed(), "Error labeling sample namespace")

						// Deploy the sample app in both clusters
						deploySampleAppToClusters(sampleNamespace, []ClusterDeployment{
							{Kubectl: k1, AppVersion: "v1"},
							{Kubectl: k2, AppVersion: "v2"},
						})
						Success("Sample app is deployed in both clusters")
					})

					It("updates the pods status to Ready", func(ctx SpecContext) {
						samplePodsPrimary := &corev1.PodList{}

						Expect(clPrimary.List(ctx, samplePodsPrimary, client.InNamespace(sampleNamespace))).To(Succeed())
						Expect(samplePodsPrimary.Items).ToNot(BeEmpty(), "No pods found in sample namespace")

						for _, pod := range samplePodsPrimary.Items {
							Eventually(common.GetObject).
								WithArguments(ctx, clPrimary, kube.Key(pod.Name, sampleNamespace), &corev1.Pod{}).
								Should(HaveConditionStatus(corev1.PodReady, metav1.ConditionTrue), "Pod is not Ready on Primary; unexpected Condition")
						}

						samplePodsRemote := &corev1.PodList{}
						Expect(clRemote.List(ctx, samplePodsRemote, client.InNamespace(sampleNamespace))).To(Succeed())
						Expect(samplePodsRemote.Items).ToNot(BeEmpty(), "No pods found in sample namespace")

						for _, pod := range samplePodsRemote.Items {
							Eventually(common.GetObject).
								WithArguments(ctx, clRemote, kube.Key(pod.Name, sampleNamespace), &corev1.Pod{}).
								Should(HaveConditionStatus(corev1.PodReady, metav1.ConditionTrue), "Pod is not Ready on Remote; unexpected Condition")
						}
						Success("Sample app is created in both clusters and Running")
					})

					It("can access the sample app from both clusters", func(ctx SpecContext) {
						verifyResponsesAreReceivedFromExpectedVersions(k1)
						verifyResponsesAreReceivedFromExpectedVersions(k2)
						Success("Sample app is accessible from both clusters")
					})
				})

				When("Istio CR is deleted in both clusters", func() {
					BeforeEach(func() {
						Expect(k1.WithNamespace(controlPlaneNamespace).Delete("istio", istioName)).To(Succeed(), "primary Istio CR failed to be deleted")
						Expect(k2.WithNamespace(controlPlaneNamespace).Delete("istio", istioName)).To(Succeed(), "remote Istio CR failed to be deleted")
						Success("Primary and Remote Istio resources are deleted")
					})

					It("removes istiod on Primary", func(ctx SpecContext) {
						Eventually(clPrimary.Get).WithArguments(ctx, kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(ReturnNotFoundError(), "Istiod should not exist anymore")
						Success("Istiod is deleted on Primary Cluster")
					})
				})

				When("IstioCNI CR is deleted in both clusters", func() {
					BeforeEach(func() {
						Expect(k1.WithNamespace(istioCniNamespace).Delete("istiocni", istioCniName)).To(Succeed(), "primary IstioCNI CR failed to be deleted")
						Expect(k2.WithNamespace(istioCniNamespace).Delete("istiocni", istioCniName)).To(Succeed(), "remote IstioCNI CR failed to be deleted")
						Success("Primary and Remote IstioCNI resources are deleted")
					})

					It("removes istio-cni-node on Primary", func(ctx SpecContext) {
						daemonset := &appsv1.DaemonSet{}
						Eventually(clPrimary.Get).WithArguments(ctx, kube.Key("istio-cni-node", istioCniNamespace), daemonset).
							Should(ReturnNotFoundError(), "IstioCNI DaemonSet should not exist anymore")
						Success("IstioCNI is deleted on Primary Cluster")
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
})
