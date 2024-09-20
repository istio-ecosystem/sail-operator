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
	certs "github.com/istio-ecosystem/sail-operator/tests/e2e/util/certs"
	common "github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
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
		}
	})

	Describe("Primary-Remote - Multi-Network configuration", func() {
		// Test the Primary-Remote - Multi-Network configuration for each supported Istio version
		for _, version := range supportedversion.List {
			// The Primary-Remote - Multi-Network configuration is only supported in Istio 1.23 and later
			if version.Major < 1 || (version.Major == 1 && version.Minor < 23) {
				continue
			}

			Context("Istio version is: "+version.Version, func() {
				When("Istio resources are created in both clusters", func() {
					BeforeAll(func(ctx SpecContext) {
						Expect(kubectl.CreateNamespace(controlPlaneNamespace, kubeconfig)).To(Succeed(), "Namespace failed to be created")
						Expect(kubectl.CreateNamespace(controlPlaneNamespace, kubeconfig2)).To(Succeed(), "Namespace failed to be created")

						// Push the intermediate CA to both clusters
						Expect(certs.PushIntermediateCA(controlPlaneNamespace, kubeconfig, "east", "network1", artifacts, clPrimary)).
							To(Succeed(), "Error pushing intermediate CA to Primary Cluster")
						Expect(certs.PushIntermediateCA(controlPlaneNamespace, kubeconfig2, "west", "network2", artifacts, clRemote)).
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

						PrimaryYAML := `
apiVersion: sailoperator.io/v1alpha1
kind: Istio
metadata:
  name: default
spec:
  version: %s
  namespace: %s
  values:
    pilot:
      env:
        EXTERNAL_ISTIOD: "true"
    global:
      meshID: %s
      multiCluster:
        clusterName: %s
      network: %s`
						multiclusterPrimaryYAML := fmt.Sprintf(PrimaryYAML, version.Name, controlPlaneNamespace, "mesh1", "cluster1", "network1")
						Log("Istio CR Primary: ", multiclusterPrimaryYAML)
						Expect(kubectl.CreateFromString(multiclusterPrimaryYAML, kubeconfig)).To(Succeed(), "Istio Resource creation failed on Primary Cluster")
					})

					It("updates Istio CR on Primary cluster status to Ready", func(ctx SpecContext) {
						Eventually(common.GetObject).
							WithArguments(ctx, clPrimary, kube.Key(istioName), &v1alpha1.Istio{}).
							Should(HaveCondition(v1alpha1.IstioConditionReady, metav1.ConditionTrue), "Istio is not Ready on Primary; unexpected Condition")
						Success("Istio CR is Ready on Primary Cluster")
					})

					It("deploys istiod", func(ctx SpecContext) {
						Eventually(common.GetObject).
							WithArguments(ctx, clPrimary, kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(HaveCondition(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Istiod is not Available on Primary; unexpected Condition")
						Expect(common.GetVersionFromIstiod()).To(Equal(version.Version), "Unexpected istiod version")
						Success("Istiod is deployed in the namespace and Running on Primary Cluster")
					})
				})

				When("Gateway is created on Primary cluster ", func() {
					BeforeAll(func(ctx SpecContext) {
						Expect(kubectl.Apply(controlPlaneNamespace, eastGatewayYAML, kubeconfig)).To(Succeed(), "Gateway creation failed on Primary Cluster")

						// Expose istiod service in Primary cluster
						Expect(kubectl.Apply(controlPlaneNamespace, exposeIstiodYAML, kubeconfig)).To(Succeed(), "Expose Istiod creation failed on Primary Cluster")

						// Expose the Gateway service in both clusters
						Expect(kubectl.Apply(controlPlaneNamespace, exposeServiceYAML, kubeconfig)).To(Succeed(), "Expose Service creation failed on Primary Cluster")
					})

					It("updates Gateway status to Available", func(ctx SpecContext) {
						Eventually((common.GetObject)).
							WithArguments(ctx, clPrimary, kube.Key("istio-eastwestgateway", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(HaveCondition(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Gateway is not Ready on Primary; unexpected Condition")
					})
				})

				When("RemoteIstio is created in Remote cluster", func() {
					BeforeAll(func(ctx SpecContext) {
						RemoteYAML := `
apiVersion: sailoperator.io/v1alpha1
kind: RemoteIstio
metadata:
  name: default
spec:
  version: %s
  namespace: istio-system
  values:
    istiodRemote:
      injectionPath: /inject/cluster/remote/net/network2
    global:
      remotePilotAddress: %s`

						remotePilotAddress, err := common.GetSVCLoadBalancerAddress(ctx, clPrimary, controlPlaneNamespace, "istio-eastwestgateway")
						Expect(remotePilotAddress).NotTo(BeEmpty(), "Remote Pilot Address is empty")
						Expect(err).NotTo(HaveOccurred(), "Error getting Remote Pilot Address")
						remoteIstioYAML := fmt.Sprintf(RemoteYAML, version.Name, remotePilotAddress)
						Log("RemoteIstio CR: ", remoteIstioYAML)
						By("Creating RemoteIstio CR on Remote Cluster")
						Expect(kubectl.CreateFromString(remoteIstioYAML, kubeconfig2)).To(Succeed(), "RemoteIstio Resource creation failed on Remote Cluster")

						// Set the controlplane cluster and network for Remote namespace
						By("Patching the istio-system namespace on Remote Cluster")
						Expect(
							kubectl.Patch("",
								"namespace",
								controlPlaneNamespace,
								"merge",
								`{"metadata":{"annotations":{"topology.istio.io/controlPlaneClusters":"cluster1"}}}`,
								kubeconfig2)).
							To(Succeed(), "Error patching istio-system namespace")
						Expect(
							kubectl.Patch("",
								"namespace",
								controlPlaneNamespace,
								"merge",
								`{"metadata":{"labels":{"topology.istio.io/network":"network2"}}}`,
								kubeconfig2)).
							To(Succeed(), "Error patching istio-system namespace")

						// To be able to access the remote cluster from the primary cluster, we need to create a secret in the primary cluster
						// RemoteIstio resource will not be Ready until the secret is created
						// Get the internal IP of the control plane node in Remote cluster
						internalIPRemote, err := kubectl.GetInternalIP("node-role.kubernetes.io/control-plane", kubeconfig2)
						Expect(internalIPRemote).NotTo(BeEmpty(), "Internal IP is empty for Remote Cluster")
						Expect(err).NotTo(HaveOccurred())

						// Wait for the RemoteIstio CR to be created, this can be moved to a condition verification, but the resource it not will be Ready at this point
						time.Sleep(5 * time.Second)

						// Install a remote secret in Primary cluster that provides access to the Remote cluster API server.
						By("Creating Remote Secret on Primary Cluster")
						secret, err := istioctl.CreateRemoteSecret(kubeconfig2, "remote", internalIPRemote)
						Expect(err).NotTo(HaveOccurred())
						Expect(kubectl.ApplyString("", secret, kubeconfig)).To(Succeed(), "Remote secret creation failed on Primary Cluster")
					})

					It("secret is created", func(ctx SpecContext) {
						secret, err := common.GetObject(ctx, clPrimary, kube.Key("istio-remote-secret-remote", controlPlaneNamespace), &corev1.Secret{})
						Expect(err).NotTo(HaveOccurred())
						Expect(secret).NotTo(BeNil(), "Secret is not created on Primary Cluster")
						Success("Remote secret is created in Primary cluster")
					})

					It("updates RemoteIstio CR status to Ready", func(ctx SpecContext) {
						Eventually(common.GetObject).
							WithArguments(ctx, clRemote, kube.Key(istioName), &v1alpha1.RemoteIstio{}).
							Should(HaveCondition(v1alpha1.IstioConditionReady, metav1.ConditionTrue), "Istio is not Ready on Remote; unexpected Condition")
						Success("RemoteIstio CR is Ready on Remote Cluster")
					})
				})

				When("gateway is created in Remote cluster", func() {
					BeforeAll(func(ctx SpecContext) {
						Expect(kubectl.Apply(controlPlaneNamespace, westGatewayYAML, kubeconfig2)).To(Succeed(), "Gateway creation failed on Remote Cluster")
						Success("Gateway is created in Remote cluster")
					})

					It("updates Gateway status to Available", func(ctx SpecContext) {
						Eventually((common.GetObject)).
							WithArguments(ctx, clRemote, kube.Key("istio-eastwestgateway", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(HaveCondition(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Gateway is not Ready on Remote; unexpected Condition")
						Success("Gateway is created and available in Remote cluster")
					})
				})

				When("sample apps are deployed in both clusters", func() {
					BeforeAll(func(ctx SpecContext) {
						// Deploy the sample app in both clusters
						deploySampleApp("sample", version, kubeconfig, kubeconfig2)
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

					It("can access the sample app from both clusters", func(ctx SpecContext) {
						sleepPodNamePrimary, err := common.GetPodNameByLabel(ctx, clPrimary, "sample", "app", "sleep")
						Expect(sleepPodNamePrimary).NotTo(BeEmpty(), "Sleep pod not found on Primary Cluster")
						Expect(err).NotTo(HaveOccurred(), "Error getting sleep pod name on Primary Cluster")

						sleepPodNameRemote, err := common.GetPodNameByLabel(ctx, clRemote, "sample", "app", "sleep")
						Expect(sleepPodNameRemote).NotTo(BeEmpty(), "Sleep pod not found on Remote Cluster")
						Expect(err).NotTo(HaveOccurred(), "Error getting sleep pod name on Remote Cluster")

						// Run the curl command from the sleep pod in the Remote Cluster and get response list to validate that we get responses from both clusters
						remoteResponses := strings.Join(getListCurlResponses(sleepPodNameRemote, kubeconfig2), "\n")
						Expect(remoteResponses).To(ContainSubstring("Hello version: v1"), "Responses from Remote Cluster are not the expected")
						Expect(remoteResponses).To(ContainSubstring("Hello version: v2"), "Responses from Remote Cluster are not the expected")

						// Run the curl command from the sleep pod in the Primary Cluster and get response list to validate that we get responses from both clusters
						primaryResponses := strings.Join(getListCurlResponses(sleepPodNamePrimary, kubeconfig), "\n")
						Expect(primaryResponses).To(ContainSubstring("Hello version: v1"), "Responses from Primary Cluster are not the expected")
						Expect(primaryResponses).To(ContainSubstring("Hello version: v2"), "Responses from Primary Cluster are not the expected")
						Success("Sample app is accessible from both clusters")
					})
				})

				When("Istio CR and RemoteIstio CR are deleted in both clusters", func() {
					BeforeEach(func() {
						Expect(kubectl.Delete(controlPlaneNamespace, "istio", istioName, kubeconfig)).To(Succeed(), "Istio CR failed to be deleted")
						Expect(kubectl.Delete(controlPlaneNamespace, "remoteistio", istioName, kubeconfig2)).To(Succeed(), "RemoteIstio CR failed to be deleted")
						Success("Istio and RemoteIstio are deleted")
					})

					It("removes istiod on Primary", func(ctx SpecContext) {
						Eventually(clPrimary.Get).WithArguments(ctx, kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(ReturnNotFoundError(), "Istiod should not exist anymore")
						Success("Istiod is deleted on Primary Cluster")
					})
				})

				AfterAll(func(ctx SpecContext) {
					// Delete namespace to ensure clean up for new tests iteration
					Expect(kubectl.DeleteNamespace(controlPlaneNamespace, kubeconfig)).To(Succeed(), "Namespace failed to be deleted on Primary Cluster")
					Expect(kubectl.DeleteNamespace(controlPlaneNamespace, kubeconfig2)).To(Succeed(), "Namespace failed to be deleted on Remote Cluster")

					common.CheckNamespaceEmpty(ctx, clPrimary, controlPlaneNamespace)
					common.CheckNamespaceEmpty(ctx, clRemote, controlPlaneNamespace)
					Success("ControlPlane Namespaces are empty")

					// Delete the entire sample namespace in both clusters
					Expect(kubectl.DeleteNamespace("sample", kubeconfig)).To(Succeed(), "Namespace failed to be deleted on Primary Cluster")
					Expect(kubectl.DeleteNamespace("sample", kubeconfig2)).To(Succeed(), "Namespace failed to be deleted on Remote Cluster")

					common.CheckNamespaceEmpty(ctx, clPrimary, "sample")
					common.CheckNamespaceEmpty(ctx, clRemote, "sample")
					Success("Sample app is deleted in both clusters")
				})
			})
		}
	})

	AfterAll(func(ctx SpecContext) {
		// Delete the Sail Operator from both clusters
		Expect(kubectl.DeleteNamespace(namespace, kubeconfig)).To(Succeed(), "Namespace failed to be deleted on Primary Cluster")
		Expect(kubectl.DeleteNamespace(namespace, kubeconfig2)).To(Succeed(), "Namespace failed to be deleted on Remote Cluster")

		// Check that the namespace is empty
		common.CheckNamespaceEmpty(ctx, clPrimary, namespace)
		common.CheckNamespaceEmpty(ctx, clRemote, namespace)
	})
})
