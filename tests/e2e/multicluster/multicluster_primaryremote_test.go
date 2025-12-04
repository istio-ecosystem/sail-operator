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

var _ = Describe("Multicluster deployment models", Label("multicluster", "multicluster-primaryremote"), Ordered, func() {
	profile := "default"
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
						createIstioNamespaces(k1, "network1", profile)
						createIstioNamespaces(k2, "network2", profile)

						// Push the intermediate CA to both clusters
						createIntermediateCA(k1, "east", "network1", artifacts, clPrimary)
						createIntermediateCA(k2, "west", "network2", artifacts, clRemote)

						// Wait for the secret to be created in both clusters
						awaitSecretCreation(k1.ClusterName, clPrimary)
						awaitSecretCreation(k2.ClusterName, clRemote)

						pilot := `
pilot:
  env:
    EXTERNAL_ISTIOD: "true"`
						createIstioResources(k1, v.Name, "cluster1", "network1", profile, pilot)
					})

					It("updates Istio CR on Primary cluster status to Ready", func(ctx SpecContext) {
						common.AwaitCondition(ctx, v1.IstioConditionReady, kube.Key(istioName), &v1.Istio{}, k1, clPrimary)
					})

					It("updates IstioCNI CR on Primary cluster status to Ready", func(ctx SpecContext) {
						common.AwaitCondition(ctx, v1.IstioCNIConditionReady, kube.Key(istioCniName), &v1.IstioCNI{}, k1, clPrimary)
					})

					It("deploys istiod", func(ctx SpecContext) {
						common.AwaitDeployment(ctx, "istiod", k1, clPrimary)
						Expect(common.GetVersionFromIstiod()).To(Equal(v.Version), "Unexpected istiod version")
					})

					It("deploys istio-cni-node", func(ctx SpecContext) {
						common.AwaitCniDaemonSet(ctx, k1, clPrimary)
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
						common.AwaitDeployment(ctx, "istio-eastwestgateway", k1, clPrimary)
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
						common.AwaitCondition(ctx, v1.IstioConditionReady, kube.Key(istioName), &v1.Istio{}, k2, clRemote, 10*time.Minute)
					})
				})

				When("gateway is created in Remote cluster", func() {
					BeforeAll(func(ctx SpecContext) {
						Expect(k2.WithNamespace(controlPlaneNamespace).Apply(westGatewayYAML)).To(Succeed(), "Gateway creation failed on Remote Cluster")
						Success("Gateway is created in Remote cluster")
					})

					It("updates Gateway status to Available", func(ctx SpecContext) {
						common.AwaitDeployment(ctx, "istio-eastwestgateway", k2, clRemote)
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
						Eventually(common.CheckSamplePodsReady).WithArguments(ctx, clPrimary).Should(Succeed(), "Error checking status of sample pods on Primary cluster")
						Eventually(common.CheckSamplePodsReady).WithArguments(ctx, clRemote).Should(Succeed(), "Error checking status of sample pods on Remote cluster")
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
