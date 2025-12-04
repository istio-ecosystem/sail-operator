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
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	externalIstioName = "external-istiod"
)

var _ = Describe("Multicluster deployment models", Label("multicluster", "multicluster-external"), Ordered, func() {
	SetDefaultEventuallyTimeout(180 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)

	Describe("External Control Plane Multi-Network configuration", func() {
		// Test the External Control Plane Multi-Network configuration for each supported Istio version
		for _, v := range istioversion.GetLatestPatchVersions() {
			// The configuration is only supported in Istio 1.24+.
			if version.Constraint("<1.24").Check(v.Version) {
				Log(fmt.Sprintf("Skipping test, because Istio version %s does not support External Control Plane Multi-Network configuration", v.Version))
				continue
			}

			Context(fmt.Sprintf("Istio version %s", v.Version), func() {
				clr1 := cleaner.New(clPrimary, "cluster=primary")
				clr2 := cleaner.New(clRemote, "cluster=remote")

				BeforeAll(func(ctx SpecContext) {
					clr1.Record(ctx)
					clr2.Record(ctx)
				})

				When("default Istio is created in Cluster #1 to handle ingress to External Control Plane", func() {
					BeforeAll(func(ctx SpecContext) {
						createIstioNamespaces(k1, "network1", "default")

						common.CreateIstioCNI(k1, v.Name)
						common.CreateIstio(k1, v.Name, `
values:
  global:
    network: network1`)
					})

					It("updates the default Istio CR status to Ready", func(ctx SpecContext) {
						common.AwaitCondition(ctx, v1.IstioConditionReady, kube.Key(istioName), &v1.Istio{}, k1, clPrimary)
					})

					It("deploys istiod", func(ctx SpecContext) {
						common.AwaitDeployment(ctx, "istiod", k1, clPrimary)
						Expect(common.GetVersionFromIstiod()).To(Equal(v.Version), "Unexpected istiod version")
					})
				})

				When("Gateway is created in Cluster #1", func() {
					BeforeAll(func(ctx SpecContext) {
						Expect(k1.WithNamespace(controlPlaneNamespace).Apply(controlPlaneGatewayYAML)).To(Succeed(), "Gateway creation failed on Cluster #1")
					})

					It("updates Gateway status to Available", func(ctx SpecContext) {
						common.AwaitDeployment(ctx, "istio-ingressgateway", k1, clPrimary)
					})
				})

				When("Istio external is installed in Cluster #2", func() {
					BeforeAll(func(ctx SpecContext) {
						Expect(k2.CreateNamespace(externalControlPlaneNamespace)).To(Succeed(), "Namespace failed to be created")
						Expect(clRemote.Get(ctx, client.ObjectKey{Name: externalControlPlaneNamespace}, &corev1.Namespace{})).To(Succeed())

						Expect(k2.CreateNamespace(istioCniNamespace)).To(Succeed(), "Istio CNI namespace failed to be created")
						common.CreateIstioCNI(k2, v.Name)
						Log("Istio CNI created on Cluster #2")

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

						var secret string
						Eventually(func() error {
							secret, err = istioctl.CreateRemoteSecret(
								kubeconfig2,
								externalControlPlaneNamespace,
								"cluster2",
								apiURLCluster2,
								"--type=config",
								"--service-account=istiod-"+externalIstioName,
								"--create-service-account=false",
							)

							return err
						}).ShouldNot(HaveOccurred(), "Remote secret generation failed")
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

					It("updates external Istio CR status to Ready", func(ctx SpecContext) {
						common.AwaitCondition(ctx, v1.IstioConditionReady, kube.Key("external-istiod"), &v1.Istio{}, k1, clPrimary)
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
						common.AwaitCondition(ctx, v1.IstioConditionReady, kube.Key(externalIstioName), &v1.Istio{}, k2, clRemote, 10*time.Minute)
					})
				})

				When("sample app deployed in Cluster #2", func() {
					BeforeAll(func(ctx SpecContext) {
						Expect(k2.CreateNamespace(sampleNamespace)).To(Succeed(), "Namespace failed to be created")
						// Label the namespace with the istio revision name
						Expect(k2.Label("namespace", sampleNamespace, "istio.io/rev", "external-istiod")).To(Succeed(), "Labeling failed on Cluster #2")

						deploySampleApp(k2, sampleNamespace, "v1", "default")
						Success("Sample app is deployed in Cluster #2")
					})

					It("updates the pods status to Ready", func(ctx SpecContext) {
						Eventually(common.CheckSamplePodsReady).WithArguments(ctx, clRemote).Should(Succeed(), "Error checking status of sample pods on Cluster #2")
						Success("Sample app is Running")
					})

					It("has istio.io/rev annotation external-istiod", func(ctx SpecContext) {
						samplePodsCluster2 := &corev1.PodList{}
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
