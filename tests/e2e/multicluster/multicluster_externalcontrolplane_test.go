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
	"path/filepath"
	"time"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	"github.com/istio-ecosystem/sail-operator/pkg/test/project"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/pkg/version"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	. "github.com/istio-ecosystem/sail-operator/tests/e2e/util/gomega"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/helm"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/istioctl"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	externalIstioName = "external-istiod"
)

var _ = Describe("Multicluster deployment models", Label("multicluster", "multicluster-external"), Ordered, func() {
	SetDefaultEventuallyTimeout(180 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)
	debugInfoLogged := false

	BeforeAll(func(ctx SpecContext) {
		if !skipDeploy {
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

	Describe("External Control Plane Multi-Network configuration", func() {
		// Test the External Control Plane Multi-Network configuration for each supported Istio version
		for _, v := range istioversion.List {
			// The configuration is only supported in Istio 1.24+.
			if version.Constraint("<1.24").Check(v.Version) {
				Log(fmt.Sprintf("Skipping test, because Istio version %s does not support External Control Plane Multi-Network configuration", v.Version))
				continue
			}

			Context(fmt.Sprintf("Istio version %s", v.Version), func() {
				When("default Istio is created in Cluster #1 to handle ingress to External Control Plane", func() {
					BeforeAll(func(ctx SpecContext) {
						Expect(k1.CreateNamespace(controlPlaneNamespace)).To(Succeed(), "Namespace failed to be created")
						Expect(k1.CreateNamespace(istioCniNamespace)).To(Succeed(), "Istio CNI namespace failed to be created")

						multiclusterCNIYAML := `
apiVersion: sailoperator.io/v1
kind: IstioCNI
metadata:
  name: {{ .Name }}
spec:
  version: {{ .Version }}
  namespace: {{ .Namespace }}`
						multiclusterCNIYAML = genTemplate(multiclusterCNIYAML, map[string]any{
							"Name":      istioCniName,
							"Namespace": istioCniNamespace,
							"Version":   v.Name,
						})
						Log("Istio CNI CR Cluster #1: ", multiclusterCNIYAML)
						Expect(k1.CreateFromString(multiclusterCNIYAML)).To(Succeed(), "Istio CNI Resource creation failed on Cluster #1")

						multiclusterYAML := `
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: {{ .Name }}
spec:
  version: {{ .Version }}
  namespace: {{ .Namespace }}
  values:
    global:
      network: {{ .Network }}`
						multiclusterYAML = genTemplate(multiclusterYAML, map[string]any{
							"Name":      istioName,
							"Namespace": controlPlaneNamespace,
							"Network":   "network1",
							"Version":   v.Name,
						})
						Log("Istio CR Cluster #1: ", multiclusterYAML)
						Expect(k1.CreateFromString(multiclusterYAML)).To(Succeed(), "Istio Resource creation failed on Cluster #1")
					})

					It("updates the default Istio CR status to Ready", func(ctx SpecContext) {
						Eventually(common.GetObject).
							WithArguments(ctx, clPrimary, kube.Key(istioName), &v1.Istio{}).
							Should(HaveConditionStatus(v1.IstioConditionReady, metav1.ConditionTrue), "Istio is not Ready on Cluster #1; unexpected Condition")
						Success("Istio CR is Ready on Cluster #1")
					})

					It("deploys istiod", func(ctx SpecContext) {
						Eventually(common.GetObject).
							WithArguments(ctx, clPrimary, kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(HaveConditionStatus(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Istiod is not Available on Cluster #1; unexpected Condition")
						Expect(common.GetVersionFromIstiod()).To(Equal(v.Version), "Unexpected istiod version")
						Success("Istiod is deployed in the namespace and Running on Exernal Cluster")
					})
				})

				When("Gateway is created in Cluster #1", func() {
					BeforeAll(func(ctx SpecContext) {
						Expect(k1.WithNamespace(controlPlaneNamespace).Apply(controlPlaneGatewayYAML)).To(Succeed(), "Gateway creation failed on Cluster #1")
					})

					It("updates Gateway status to Available", func(ctx SpecContext) {
						Eventually(common.GetObject).
							WithArguments(ctx, clPrimary, kube.Key("istio-ingressgateway", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(HaveConditionStatus(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Gateway is not Ready on Cluster #1; unexpected Condition")

						Success("Gateway is created and available in both clusters")
					})
				})

				When("Istio external is installed in Cluster #2", func() {
					BeforeAll(func(ctx SpecContext) {
						Expect(k2.CreateNamespace(externalControlPlaneNamespace)).To(Succeed(), "Namespace failed to be created")
						Expect(clRemote.Get(ctx, client.ObjectKey{Name: externalControlPlaneNamespace}, &corev1.Namespace{})).To(Succeed())

						Expect(k2.CreateNamespace(istioCniNamespace)).To(Succeed(), "Istio CNI namespace failed to be created")
						remoteIstioCNIYAML := `
apiVersion: sailoperator.io/v1
kind: IstioCNI
metadata:
  name: {{ .Name }}
spec:
  version: {{ .Version }}
  namespace: {{ .Namespace }}`
						remoteIstioCNIYAML = genTemplate(remoteIstioCNIYAML, map[string]any{
							"Name":      istioCniName,
							"Namespace": istioCniNamespace,
							"Version":   v.Name,
						})
						Log("Istio CNI CR Cluster #2: ", remoteIstioCNIYAML)
						Expect(k2.CreateFromString(remoteIstioCNIYAML)).To(Succeed(), "Istio CNI Resource creation failed on Cluster #2")

						remotePilotAddress := common.GetSVCLoadBalancerAddress(ctx, clPrimary, controlPlaneNamespace, "istio-ingressgateway")
						remotePilotIP, err := common.ResolveHostDomainToIP(remotePilotAddress)
						Expect(remotePilotIP).NotTo(BeEmpty(), "Remote Pilot IP is empty")
						Expect(err).NotTo(HaveOccurred(), "Error getting Remote Pilot IP")

						remoteIstioYAML := `
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: {{ .Name }}
spec:
  version: {{ .Version }}
  namespace: {{ .Namespace }}
  profile: remote
  values:
    defaultRevision: {{ .Name }}
    global:
      istioNamespace: {{ .Namespace }}
      remotePilotAddress: {{ .RemotePilotAddress }}
      configCluster: true
    pilot:
      configMap: true
    istiodRemote:
      injectionPath: /inject/cluster/cluster2/net/network1
`
						remoteIstioYAML = genTemplate(remoteIstioYAML, map[string]any{
							"Name":               externalIstioName,
							"Namespace":          externalControlPlaneNamespace,
							"RemotePilotAddress": remotePilotIP,
							"Version":            v.Name,
						})
						Log("Istio external-istiod CR: ", remoteIstioYAML)
						By("Creating Istio external-istiod CR on Cluster #2")
						Expect(k2.CreateFromString(remoteIstioYAML)).To(Succeed(), "Istio external-istiod Resource creation failed on Cluster #2")
					})

					// This is needed for istioctl create-remote-secret in a later test but we can't check
					// the Istio external-istiod status for Ready because the webhook readiness check will fail
					// since the External Control Plane cluster doesn't exist yet.
					It("has a service account for the remote profile", func(ctx SpecContext) {
						Eventually(func() error {
							_, err := common.GetObject(ctx, clRemote, kube.Key("istiod-"+externalIstioName, externalControlPlaneNamespace), &corev1.ServiceAccount{})
							return err
						}).ShouldNot(HaveOccurred(), "Service Account is not created on Cluster #2")
					})
				})

				When("a remote secret is installed on the External Control Plane cluster", func() {
					BeforeAll(func(ctx SpecContext) {
						Expect(k1.CreateNamespace(externalControlPlaneNamespace)).To(Succeed(), "Namespace failed to be created")
						_, err := common.GetObject(ctx, clPrimary, types.NamespacedName{Name: externalControlPlaneNamespace}, &corev1.Namespace{})
						Expect(err).NotTo(HaveOccurred())

						externalSVCAccountYAML := `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: istiod-service-account
`
						k1.WithNamespace(externalControlPlaneNamespace).CreateFromString(externalSVCAccountYAML)

						apiURLCluster2, err := k2.GetClusterAPIURL()
						Expect(apiURLCluster2).NotTo(BeEmpty(), "API URL is empty for the Cluster #2")
						Expect(err).NotTo(HaveOccurred())

						secret, err := istioctl.CreateRemoteSecret(
							kubeconfig2,
							externalControlPlaneNamespace,
							"cluster2",
							apiURLCluster2,
							"--type=config",
							"--service-account=istiod-"+externalIstioName,
							"--create-service-account=false",
						)
						Expect(err).NotTo(HaveOccurred())
						Expect(k1.ApplyString(secret)).To(Succeed(), "Remote secret creation failed on Cluster #1")
					})

					It("has a remote secret in the External Control Plane namespace", func(ctx SpecContext) {
						secret, err := common.GetObject(ctx, clPrimary, kube.Key("istio-kubeconfig", externalControlPlaneNamespace), &corev1.Secret{})
						Expect(err).NotTo(HaveOccurred())
						Expect(secret).NotTo(BeNil(), "Secret is not created on the Cluster #1")

						Success("Remote secrets is created in the External Control Plane namespace")
					})
				})

				When("the External Control Plane Istio is installed on the Cluster #1", func() {
					BeforeAll(func(ctx SpecContext) {
						externalControlPlaneYAML := `
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: {{ .Name }}
spec:
  version: {{ .Version }}
  namespace: {{ .Namespace }}
  profile: empty
  values:
    meshConfig:
      rootNamespace: {{ .Namespace }}
      defaultConfig:
        discoveryAddress: {{ .ExternalIstiodAddr }}:15012
    pilot:
      enabled: true
      volumes:
        - name: config-volume
          configMap:
            name: istio-{{ .Name }}
        - name: inject-volume
          configMap:
            name: istio-sidecar-injector-{{ .Name }}
      volumeMounts:
        - name: config-volume
          mountPath: /etc/istio/config
        - name: inject-volume
          mountPath: /var/lib/istio/inject
      env:
        INJECTION_WEBHOOK_CONFIG_NAME: "istio-sidecar-injector-{{ .Name }}-{{ .Namespace }}"
        VALIDATION_WEBHOOK_CONFIG_NAME: "istio-validator-{{ .Name }}-{{ .Namespace }}"
        EXTERNAL_ISTIOD: "true"
        LOCAL_CLUSTER_SECRET_WATCHER: "true"
        CLUSTER_ID: cluster2
        SHARED_MESH_CONFIG: istio
    global:
      caAddress: {{ .ExternalIstiodAddr }}:15012
      istioNamespace: {{ .Namespace }}
      operatorManageWebhooks: true
      configValidation: false
      meshID: mesh1
      multiCluster:
        clusterName: cluster2
      network: network1
`
						externalIstiodAddr := common.GetSVCLoadBalancerAddress(ctx, clPrimary, controlPlaneNamespace, "istio-ingressgateway")
						externalControlPlaneYAML = genTemplate(externalControlPlaneYAML, map[string]any{
							"ExternalIstiodAddr": externalIstiodAddr,
							"Namespace":          externalControlPlaneNamespace,
							"Name":               externalIstioName,
							"Version":            v.Name,
						})
						Log("Istio CR Cluster #1: ", externalControlPlaneYAML)
						Expect(k1.CreateFromString(externalControlPlaneYAML)).To(Succeed(), "Istio Resource creation failed on Cluster #1")
					})

					It("updates both Istio CR status to Ready", func(ctx SpecContext) {
						Eventually(common.GetObject).
							WithArguments(ctx, clPrimary, kube.Key("external-istiod"), &v1.Istio{}).
							Should(HaveConditionStatus(v1.IstioConditionReady, metav1.ConditionTrue), "Istio is not Ready on Cluster #1; unexpected Condition")
						Success("Istio CR is Ready on Cluster #1")
					})
				})

				When("Gateway and VirtualService resources are created to route traffic from the ingress gateway to the external contorlplane", func() {
					BeforeAll(func(ctx SpecContext) {
						routingResourcesYAML := `
apiVersion: networking.istio.io/v1
kind: Gateway
metadata:
  name: {{ .Name }}-gw
  namespace: {{ .Namespace }}
spec:
  selector:
    istio: ingressgateway
  servers:
    - port:
        number: 15012
        protocol: tls
        name: tls-XDS
      tls:
        mode: PASSTHROUGH
      hosts:
      - "*"
    - port:
        number: 15017
        protocol: tls
        name: tls-WEBHOOK
      tls:
        mode: PASSTHROUGH
      hosts:
      - "*"
---
apiVersion: networking.istio.io/v1
kind: VirtualService
metadata:
  name: {{ .Name }}-vs
  namespace: {{ .Namespace }}
spec:
    hosts:
    - "*"
    gateways:
    - {{ .Name }}-gw
    tls:
    - match:
      - port: 15012
        sniHosts:
        - "*"
      route:
      - destination:
          host: istiod-{{ .Name }}.{{ .Namespace }}.svc.cluster.local
          port:
            number: 15012
    - match:
      - port: 15017
        sniHosts:
        - "*"
      route:
      - destination:
          host: istiod-{{ .Name }}.{{ .Namespace }}.svc.cluster.local
          port:
            number: 443
`
						routingResourcesYAML = genTemplate(routingResourcesYAML, map[string]any{
							"Name":      externalIstioName,
							"Namespace": externalControlPlaneNamespace,
						})
						Expect(k1.ApplyString(routingResourcesYAML)).To(Succeed())
					})

					It("updates remote Istio CR status to Ready on Cluster #2", func(ctx SpecContext) {
						Eventually(common.GetObject).
							WithArguments(ctx, clRemote, kube.Key(externalIstioName), &v1.Istio{}).
							Should(HaveConditionStatus(v1.IstioConditionReady, metav1.ConditionTrue), "Istio is not Ready on Remote; unexpected Condition")
						Success("Remote Istio CR is Ready on Cluster #2")
					})
				})

				When("sample app deployed in Cluster #2", func() {
					BeforeAll(func(ctx SpecContext) {
						Expect(k2.CreateNamespace(sampleNamespace)).To(Succeed(), "Namespace failed to be created")
						// Label the namespace with the istio revision name
						Expect(k2.Label("namespace", sampleNamespace, "istio.io/rev", "external-istiod")).To(Succeed(), "Labeling failed on Cluster #2")

						deploySampleApp(k2, sampleNamespace, v, "v1")
						Success("Sample app is deployed in Cluster #2")
					})

					samplePodsCluster2 := &corev1.PodList{}
					It("updates the pods status to Ready", func(ctx SpecContext) {
						Expect(clRemote.List(ctx, samplePodsCluster2, client.InNamespace(sampleNamespace))).To(Succeed())
						Expect(samplePodsCluster2.Items).ToNot(BeEmpty(), "No pods found in sample namespace")

						for _, pod := range samplePodsCluster2.Items {
							Eventually(common.GetObject).
								WithArguments(ctx, clRemote, kube.Key(pod.Name, sampleNamespace), &corev1.Pod{}).
								Should(HaveConditionStatus(corev1.PodReady, metav1.ConditionTrue), "Pod is not Ready on Cluster #2; unexpected Condition")
						}
						Success("Sample app is Running")
					})

					It("has istio.io/rev annotation external-istiod", func(ctx SpecContext) {
						Expect(clRemote.List(ctx, samplePodsCluster2, client.InNamespace(sampleNamespace))).To(Succeed())
						Expect(samplePodsCluster2.Items).ToNot(BeEmpty(), "No pods found in sample namespace")

						for _, pod := range samplePodsCluster2.Items {
							Expect(pod.Annotations).To(HaveKeyWithValue("istio.io/rev", "external-istiod"), "The pod dom't have expected annotation")
						}
						Success("Sample pods has expected annotation")
					})

					It("can access the sample app from the local service", func(ctx SpecContext) {
						verifyResponsesAreReceivedFromExpectedVersions(k2, "v1")
						Success("Sample app is accessible from hello service in Cluster #2")
					})
				})

				When("istio and istio CNI CR are deleted in both clusters", func() {
					BeforeAll(func() {
						// Delete the Istio, IstioCNI and remote Istio CRs in both clusters
						Expect(k1.Delete("istio", istioName)).To(Succeed(), "Istio CR failed to be deleted")
						Expect(k1.Delete("istio", externalIstioName)).To(Succeed(), "Istio CR failed to be deleted")
						Expect(k1.Delete("istiocni", istioCniName)).To(Succeed(), "Istio CNI CR failed to be deleted")
						Expect(k2.Delete("istio", externalIstioName)).To(Succeed(), "Remote Istio CR failed to be deleted")
						Expect(k2.Delete("istiocni", istioCniName)).To(Succeed(), "Remote Istio CNI CR failed to be deleted")

						Success("Istio and Istio CNI resources are deleted in both clusters")
					})

					It("removes istiod pod", func(ctx SpecContext) {
						// Check istiod pod is deleted in both clusters
						Eventually(clPrimary.Get).WithArguments(ctx, kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(ReturnNotFoundError(), "Istiod should not exist anymore on Cluster #1")
					})

					It("removes mutating webhook from Remote cluster", func(ctx SpecContext) {
						Eventually(clRemote.Get).WithArguments(ctx, kube.Key("istiod-"+externalIstioName), &admissionregistrationv1.MutatingWebhookConfiguration{}).
							Should(ReturnNotFoundError(), "Remote webhook should not exist anymore on Cluster #2")
					})
				})

				AfterAll(func(ctx SpecContext) {
					if CurrentSpecReport().Failed() {
						common.LogDebugInfo(common.MultiCluster, k1, k2)
						debugInfoLogged = true
					}

					// Delete namespaces to ensure clean up for new tests iteration
					Expect(k1.DeleteNamespaceNoWait(controlPlaneNamespace)).To(Succeed(), "Namespace failed to be deleted on Cluster #1")
					Expect(k1.DeleteNamespaceNoWait(externalControlPlaneNamespace)).To(Succeed(), "Namespace failed to be deleted on Cluster #1")
					Expect(k1.DeleteNamespaceNoWait(istioCniNamespace)).To(Succeed(), "Namespace failed to be deleted on Cluster #1")
					Expect(k2.DeleteNamespaceNoWait(externalControlPlaneNamespace)).To(Succeed(), "Namespace failed to be deleted on Cluster #2")
					Expect(k2.DeleteNamespaceNoWait(sampleNamespace)).To(Succeed(), "Namespace failed to be deleted on Cluster #2")
					Expect(k2.DeleteNamespaceNoWait(istioCniNamespace)).To(Succeed(), "Namespace failed to be deleted on Cluster #2")

					Expect(k1.WaitNamespaceDeleted(controlPlaneNamespace)).To(Succeed())
					Expect(k1.WaitNamespaceDeleted(externalControlPlaneNamespace)).To(Succeed())
					Expect(k2.WaitNamespaceDeleted(externalControlPlaneNamespace)).To(Succeed())
					Expect(k1.WaitNamespaceDeleted(istioCniNamespace)).To(Succeed())
					Expect(k2.WaitNamespaceDeleted(istioCniNamespace)).To(Succeed())
					Success("ControlPlane Namespaces are empty")

					Expect(k2.WaitNamespaceDeleted(sampleNamespace)).To(Succeed())
					Success("Sample app is deleted in Cluster #2")
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
