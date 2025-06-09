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
	"time"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	"github.com/istio-ecosystem/sail-operator/pkg/test/project"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/certs"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/cleaner"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	. "github.com/istio-ecosystem/sail-operator/tests/e2e/util/gomega"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/helm"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/istioctl"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Multicluster deployment models", Label("multicluster", "multicluster-multiprimary"), Ordered, func() {
	SetDefaultEventuallyTimeout(180 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)
	debugInfoLogged := false
	clr1 := cleaner.New(clPrimary, "cluster=primary")
	clr2 := cleaner.New(clRemote, "cluster=remote")

	BeforeAll(func(ctx SpecContext) {
		clr1.Record(ctx)
		clr2.Record(ctx)

		if !skipDeploy {
			// Deploy the Sail Operator on both clusters
			Expect(k1.CreateNamespace(namespace)).To(Succeed(), "Namespace failed to be created on Cluster #1")
			Expect(k2.CreateNamespace(namespace)).To(Succeed(), "Namespace failed to be created on Cluster #2")

			Expect(helm.Install("sail-operator", filepath.Join(project.RootDir, "chart"), "--namespace "+namespace, "--set=image="+image, "--kubeconfig "+kubeconfig)).
				To(Succeed(), "Operator failed to be deployed in Cluster #1")

			Expect(helm.Install("sail-operator", filepath.Join(project.RootDir, "chart"), "--namespace "+namespace, "--set=image="+image, "--kubeconfig "+kubeconfig2)).
				To(Succeed(), "Operator failed to be deployed in Cluster #2")

			Eventually(common.GetObject).
				WithArguments(ctx, clPrimary, kube.Key(deploymentName, namespace), &appsv1.Deployment{}).
				Should(HaveConditionStatus(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Error getting Istio CRD")
			Success("Operator is deployed in the Cluster #1 namespace and Running")

			Eventually(common.GetObject).
				WithArguments(ctx, clRemote, kube.Key(deploymentName, namespace), &appsv1.Deployment{}).
				Should(HaveConditionStatus(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Error getting Istio CRD")
			Success("Operator is deployed in the Cluster #2 namespace and Running")
		}
	})

	Describe("Multi-Primary Multi-Network configuration", func() {
		// Test the Multi-Primary Multi-Network configuration for each supported Istio version
		for _, version := range istioversion.GetLatestPatchVersions() {
			Context(fmt.Sprintf("Istio version %s", version.Version), func() {
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
						Expect(k1.CreateNamespace(istioCniNamespace)).To(Succeed(), "Istio CNI namespace failed to be created")
						Expect(k2.CreateNamespace(istioCniNamespace)).To(Succeed(), "Istio CNI namespace failed to be created")

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

						multiclusterIstioCNIYAML := `
apiVersion: sailoperator.io/v1
kind: IstioCNI
metadata:
  name: default
spec:
  version: %s
  namespace: %s`
						multiclusterIstioCNICluster1YAML := fmt.Sprintf(multiclusterIstioCNIYAML, version.Name, istioCniNamespace)
						Log("Istio CNI CR Cluster #1: ", multiclusterIstioCNICluster1YAML)
						Expect(k1.CreateFromString(multiclusterIstioCNICluster1YAML)).To(Succeed(), "Istio CNI Resource creation failed on Cluster #1")

						multiclusterIstioCNICluster2YAML := fmt.Sprintf(multiclusterIstioCNIYAML, version.Name, istioCniNamespace)
						Log("Istio CNI CR Cluster #2: ", multiclusterIstioCNICluster2YAML)
						Expect(k2.CreateFromString(multiclusterIstioCNICluster2YAML)).To(Succeed(), "Istio CNI Resource creation failed on Cluster #2")

						multiclusterIstioYAML := `
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
						multiclusterIstioCluster1YAML := fmt.Sprintf(multiclusterIstioYAML, version.Name, controlPlaneNamespace, "mesh1", "cluster1", "network1")
						Log("Istio CR Cluster #1: ", multiclusterIstioCluster1YAML)
						Expect(k1.CreateFromString(multiclusterIstioCluster1YAML)).To(Succeed(), "Istio Resource creation failed on Cluster #1")

						multiclusterIstioCluster2YAML := fmt.Sprintf(multiclusterIstioYAML, version.Name, controlPlaneNamespace, "mesh1", "cluster2", "network2")
						Log("Istio CR Cluster #2: ", multiclusterIstioCluster2YAML)
						Expect(k2.CreateFromString(multiclusterIstioCluster2YAML)).To(Succeed(), "Istio Resource creation failed on Cluster #2")
					})

					It("updates both Istio CR status to Ready", func(ctx SpecContext) {
						Eventually(common.GetObject).
							WithArguments(ctx, clPrimary, kube.Key(istioName), &v1.Istio{}).
							Should(HaveConditionStatus(v1.IstioConditionReady, metav1.ConditionTrue), "Istio is not Ready on Cluster #1; unexpected Condition")
						Success("Istio CR is Ready on Cluster #1")

						Eventually(common.GetObject).
							WithArguments(ctx, clRemote, kube.Key(istioName), &v1.Istio{}).
							Should(HaveConditionStatus(v1.IstioConditionReady, metav1.ConditionTrue), "Istio is not Ready on Cluster #2; unexpected Condition")
						Success("Istio CR is Ready on Cluster #2")
					})

					It("updates both IstioCNI CR status to Ready", func(ctx SpecContext) {
						Eventually(common.GetObject).
							WithArguments(ctx, clPrimary, kube.Key(istioCniName), &v1.IstioCNI{}).
							Should(HaveConditionStatus(v1.IstioCNIConditionReady, metav1.ConditionTrue), "Istio CNI is not Ready on Cluster #1; unexpected Condition")
						Success("Istio CNI CR is Ready on Cluster #1")

						Eventually(common.GetObject).
							WithArguments(ctx, clRemote, kube.Key(istioCniName), &v1.IstioCNI{}).
							Should(HaveConditionStatus(v1.IstioCNIConditionReady, metav1.ConditionTrue), "Istio CNI is not Ready on Cluster #2; unexpected Condition")
						Success("Istio CNI CR is Ready on Cluster #2")
					})

					It("deploys istiod", func(ctx SpecContext) {
						Eventually(common.GetObject).
							WithArguments(ctx, clPrimary, kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(HaveConditionStatus(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Istiod is not Available on Cluster #1; unexpected Condition")
						Expect(common.GetVersionFromIstiod()).To(Equal(version.Version), "Unexpected istiod version")
						Success("Istiod is deployed in the namespace and Running on Cluster #1")

						Eventually(common.GetObject).
							WithArguments(ctx, clRemote, kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(HaveConditionStatus(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Istiod is not Available on Cluster #2; unexpected Condition")
						Expect(common.GetVersionFromIstiod()).To(Equal(version.Version), "Unexpected istiod version")
						Success("Istiod is deployed in the namespace and Running on Cluster #2")
					})

					It("deploys istio-cni-node", func(ctx SpecContext) {
						Eventually(func() bool {
							daemonset := &appsv1.DaemonSet{}
							if err := clPrimary.Get(ctx, kube.Key("istio-cni-node", istioCniNamespace), daemonset); err != nil {
								return false
							}
							return daemonset.Status.NumberAvailable == daemonset.Status.CurrentNumberScheduled
						}).Should(BeTrue(), "CNI DaemonSet Pods are not Available on Cluster #1")
						Success("CNI DaemonSet is deployed in the namespace and Running on Cluster #1")

						Eventually(func() bool {
							daemonset := &appsv1.DaemonSet{}
							if err := clRemote.Get(ctx, kube.Key("istio-cni-node", istioCniNamespace), daemonset); err != nil {
								return false
							}
							return daemonset.Status.NumberAvailable == daemonset.Status.CurrentNumberScheduled
						}).Should(BeTrue(), "IstioCNI DaemonSet Pods are not Available on Cluster #2")
						Success("IstioCNI DaemonSet is deployed in the namespace and Running on Cluster #2")
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
							Should(HaveConditionStatus(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Gateway is not Ready on Cluster #1; unexpected Condition")

						Eventually(common.GetObject).
							WithArguments(ctx, clRemote, kube.Key("istio-eastwestgateway", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(HaveConditionStatus(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Gateway is not Ready on Cluster #2; unexpected Condition")
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
						// Create namespace
						Expect(k1.CreateNamespace(sampleNamespace)).To(Succeed(), "Namespace failed to be created on Cluster #1")
						Expect(k2.CreateNamespace(sampleNamespace)).To(Succeed(), "Namespace failed to be created on Cluster #2")

						// Label the namespace
						Expect(k1.Label("namespace", sampleNamespace, "istio-injection", "enabled")).To(Succeed(), "Error labeling sample namespace")
						Expect(k2.Label("namespace", sampleNamespace, "istio-injection", "enabled")).To(Succeed(), "Error labeling sample namespace")

						// Deploy the sample app in both clusters
						deploySampleAppToClusters(sampleNamespace, version, []ClusterDeployment{
							{Kubectl: k1, AppVersion: "v1"},
							{Kubectl: k2, AppVersion: "v2"},
						})
						Success("Sample app is deployed in both clusters")
					})

					It("updates the pods status to Ready", func(ctx SpecContext) {
						samplePodsCluster1 := &corev1.PodList{}

						Expect(clPrimary.List(ctx, samplePodsCluster1, client.InNamespace(sampleNamespace))).To(Succeed())
						Expect(samplePodsCluster1.Items).ToNot(BeEmpty(), "No pods found in sample namespace")

						for _, pod := range samplePodsCluster1.Items {
							Eventually(common.GetObject).
								WithArguments(ctx, clPrimary, kube.Key(pod.Name, sampleNamespace), &corev1.Pod{}).
								Should(HaveConditionStatus(corev1.PodReady, metav1.ConditionTrue), "Pod is not Ready on Cluster #1; unexpected Condition")
						}

						samplePodsCluster2 := &corev1.PodList{}
						Expect(clRemote.List(ctx, samplePodsCluster2, client.InNamespace(sampleNamespace))).To(Succeed())
						Expect(samplePodsCluster2.Items).ToNot(BeEmpty(), "No pods found in sample namespace")

						for _, pod := range samplePodsCluster2.Items {
							Eventually(common.GetObject).
								WithArguments(ctx, clRemote, kube.Key(pod.Name, sampleNamespace), &corev1.Pod{}).
								Should(HaveConditionStatus(corev1.PodReady, metav1.ConditionTrue), "Pod is not Ready on Cluster #2; unexpected Condition")
						}
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

	AfterAll(func(ctx SpecContext) {
		if CurrentSpecReport().Failed() {
			if !debugInfoLogged {
				common.LogDebugInfo(common.MultiCluster, k1, k2)
				debugInfoLogged = true
			}

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
