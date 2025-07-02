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
// WITHOUT WARRANTIES OR Condition OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package certmanager

import (
	"fmt"
	"strings"
	"time"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/cleaner"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	. "github.com/istio-ecosystem/sail-operator/tests/e2e/util/gomega"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"istio.io/istio/pkg/ptr"
)

var latestVersion = istioversion.GetLatestPatchVersions()[0]

var _ = Describe("Cert-manager Installation", Label("smoke", "cert-manager", "slow"), Ordered, func() {
	SetDefaultEventuallyTimeout(180 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)
	debugInfoLogged := false

	Describe(fmt.Sprintf("Istio version: %s", latestVersion.Name), func() {
		clr := cleaner.New(cl)
		BeforeAll(func(ctx SpecContext) {
			clr.Record(ctx)
			Expect(k.CreateNamespace(controlPlaneNamespace)).To(Succeed(), "Istio namespace failed to be created")
			Expect(k.CreateNamespace(istioCniNamespace)).To(Succeed(), "IstioCNI namespace failed to be created")
			Expect(k.CreateNamespace(istioCSRNamespace)).To(Succeed(), "IstioCSR Namespace failed to be created")
			Expect(k.CreateNamespace(certManagerOperatorNamespace)).To(Succeed(), "Cert Manager Operator Namespace failed to be created")
			Expect(k.CreateNamespace(certManagerNamespace)).To(Succeed(), "Cert Manager Namespace failed to be created")
		})

		When("the Cert Manager Operator is deployed", func() {
			BeforeAll(func() {
				// Apply OperatorGroup YAML
				operatorGroupYaml := `
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: openshift-cert-manager-operator
  namespace: cert-manager-operator
spec:
  targetNamespaces: []
  spec: {}`
				Expect(k.WithNamespace(certManagerOperatorNamespace).CreateFromString(operatorGroupYaml)).To(Succeed(), "OperatorGroup creation failed")

				// Apply Subscription YAML
				subscriptionYaml := `
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: openshift-cert-manager-operator
  namespace: cert-manager-operator
spec:
  channel: stable-v1
  name: openshift-cert-manager-operator
  source: redhat-operators
  sourceNamespace: openshift-marketplace
  installPlanApproval: Automatic`
				Expect(k.WithNamespace(certManagerOperatorNamespace).CreateFromString(subscriptionYaml)).To(Succeed(), "Subscription creation failed")
			})

			It("should have subscription created successfully", func() {
				output, err := k.WithNamespace(certManagerOperatorNamespace).GetYAML("subscription", certManagerDeploymentName)
				Expect(err).NotTo(HaveOccurred(), "error getting subscription YAML")
				Expect(output).To(ContainSubstring(certManagerDeploymentName), "Subscription is not created")
			})

			It("verifies all cert-manager pods are Ready", func(ctx SpecContext) {
				Eventually(common.CheckPodsReady).
					WithArguments(ctx, cl, certManagerNamespace).
					Should(Succeed(), fmt.Sprintf("Some pods in namespace %q are not ready", certManagerNamespace))

				Success("All cert-manager pods are ready")
			})
		})

		// Workaround until https://issues.redhat.com/browse/OCPBUGS-56758 is fixed
		When("the Cert Manager Operator is patched", func() {
			BeforeAll(func() {
				roleBindingYaml := `
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  labels:
    app: cert-manager
    app.kubernetes.io/component: controller
    app.kubernetes.io/instance: cert-manager
    app.kubernetes.io/name: cert-manager
    app.kubernetes.io/version: v1.16.4
  name: cert-manager-cert-manager-tokenrequest
  namespace: cert-manager
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: cert-manager-tokenrequest
subjects:
  - kind: ServiceAccount
    name: cert-manager
    namespace: cert-manager`
				Expect(k.WithNamespace(certManagerNamespace).CreateFromString(roleBindingYaml)).To(Succeed(), "RoleBinding creation failed")
			})

			It("should succeed", func() {
				roleYaml := `
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  labels:
    app: cert-manager
    app.kubernetes.io/component: controller
    app.kubernetes.io/instance: cert-manager
    app.kubernetes.io/name: cert-manager
    app.kubernetes.io/version: v1.16.4
  name: cert-manager-tokenrequest
  namespace: cert-manager
rules:
  - apiGroups:
      - ""
    resourceNames:
      - cert-manager
    resources:
      - serviceaccounts/token
    verbs:
      - create`
				Expect(k.WithNamespace(certManagerNamespace).CreateFromString(roleYaml)).To(Succeed(), "Role creation failed")
				Success("Cert Manager Operator patched successfully")
			})
		})

		When("root CA issuer for the IstioCSR agent is created", func() {
			BeforeAll(func() {
				Expect(
					k.WithNamespace(certManagerOperatorNamespace).Patch(
						"subscription",
						"openshift-cert-manager-operator",
						"merge",
						`{"spec":{"config":{"env":[{"name":"UNSUPPORTED_ADDON_FEATURES","value":"IstioCSR=true"}]}}}`,
					),
				).To(Succeed(), "Error patching cert manager")
				Success("Cert Manager subscription patched")
				issuerYaml := `
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: selfsigned
  namespace: %s
spec:
  selfSigned: {}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: istio-ca
  namespace: %s
spec:
  isCA: true
  duration: 87600h # 10 years
  secretName: istio-ca
  commonName: istio-ca
  privateKey:
    algorithm: ECDSA
    size: 256
  subject:
    organizations:
      - cluster.local
      - cert-manager
  issuerRef:
    name: selfsigned
    kind: Issuer
    group: cert-manager.io
---
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: istio-ca
  namespace: %s
spec:
  ca:
    secretName: istio-ca`
				issuerYaml = fmt.Sprintf(issuerYaml, controlPlaneNamespace, controlPlaneNamespace, controlPlaneNamespace)

				Expect(k.WithNamespace(controlPlaneNamespace).ApplyString(issuerYaml)).To(Succeed(), "Issuer creation failed")
			})

			It("creates certificate Issuer", func() {
				Eventually(func() string {
					output, _ := k.WithNamespace(controlPlaneNamespace).GetYAML("issuer", "istio-ca")
					return output
				}, 120*time.Second, 5*time.Second).Should(ContainSubstring("True"), "Issuer is not ready")
			})
		})

		When("custom resource for the IstioCSR is created", func() {
			BeforeAll(func() {
				istioCsrYaml := `
apiVersion: operator.openshift.io/v1alpha1
kind: IstioCSR
metadata:
  name: default
  namespace: %s
spec:
  istioCSRConfig:
    certManager:
      issuerRef:
        name: istio-ca
        kind: Issuer
        group: cert-manager.io
    istiodTLSConfig:
      trustDomain: cluster.local
    istio:
      namespace: %s`
				istioCsrYaml = fmt.Sprintf(istioCsrYaml, istioCSRNamespace, controlPlaneNamespace)
				Expect(k.CreateFromString(istioCsrYaml)).To(Succeed(), "IstioCsr custom resource creation failed")
			})

			It("has IstioCSR pods running", func(ctx SpecContext) {
				Eventually(common.CheckPodsReady).
					WithArguments(ctx, cl, certManagerNamespace).
					Should(Succeed(), fmt.Sprintf("Some pods in namespace %q are not ready", certManagerNamespace))

				Success("All cert-manager pods are ready")
			})
		})

		When("the IstioCNI CR is created", func() {
			BeforeAll(func() {
				yaml := `
apiVersion: sailoperator.io/v1
kind: IstioCNI
metadata:
  name: default
spec:
  version: %s
  namespace: %s`
				yaml = fmt.Sprintf(yaml, latestVersion.Name, istioCniNamespace)
				Log("IstioCNI YAML:", common.Indent(yaml))
				Expect(k.CreateFromString(yaml)).To(Succeed(), "IstioCNI creation failed")
				Success("IstioCNI created")
			})

			It("updates the status to Ready", func(ctx SpecContext) {
				Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(istioCniName), &v1.IstioCNI{}).
					Should(HaveConditionStatus(v1.IstioCNIConditionReady, metav1.ConditionTrue), "IstioCNI is not Ready; unexpected Condition")
				Success("IstioCNI is Ready")
			})

			It("doesn't continuously reconcile the IstioCNI CR", func() {
				Eventually(k.WithNamespace(namespace).Logs).WithArguments("deploy/"+deploymentName, ptr.Of(30*time.Second)).
					ShouldNot(ContainSubstring("Reconciliation done"), "IstioCNI is continuously reconciling")
				Success("IstioCNI stopped reconciling")
			})
		})

		When("the Istio CR is created", func() {
			BeforeAll(func() {
				istioYAML := `
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  values:
    global:
      caAddress: cert-manager-istio-csr.istio-csr.svc:443
    pilot:
      env:
        ENABLE_CA_SERVER: "false"
  version: %s
  namespace: %s`
				istioYAML = fmt.Sprintf(istioYAML, latestVersion.Name, controlPlaneNamespace)
				Log("Istio YAML:", common.Indent(istioYAML))
				Expect(k.CreateFromString(istioYAML)).
					To(Succeed(), "Istio CR failed to be created")
				Success("Istio CR created")
				// This sleep is necessary for cert-manager to send certificate to istiod
				time.Sleep(120 * time.Second)
			})

			It("updates the Istio CR status to Ready", func(ctx SpecContext) {
				Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(istioName), &v1.Istio{}).
					Should(HaveConditionStatus(v1.IstioConditionReady, metav1.ConditionTrue), "Istio is not Ready; unexpected Condition")
				Success("Istio CR is Ready")
			})

			It("deploys istiod", func(ctx SpecContext) {
				Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{}).
					Should(HaveConditionStatus(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Istiod is not Available; unexpected Condition")
				Expect(common.GetVersionFromIstiod()).To(Equal(latestVersion.Version), "Unexpected istiod version")
				Success("Istiod is deployed in the namespace and Running")
			})

			It("doesn't continuously reconcile the Istio CR", func() {
				Eventually(k.WithNamespace(namespace).Logs).WithArguments("deploy/"+deploymentName, ptr.Of(30*time.Second)).
					ShouldNot(ContainSubstring("Reconciliation done"), "Istio CR is continuously reconciling")
				Success("Istio CR stopped reconciling")
			})
		})

		When("sample apps are deployed in the cluster", func() {
			BeforeAll(func(ctx SpecContext) {
				Expect(k.CreateNamespace(common.SleepNamespace)).To(Succeed(), "Failed to create sleep namespace")
				Expect(k.CreateNamespace(common.HttpbinNamespace)).To(Succeed(), "Failed to create httpbin namespace")
				Expect(k.Label("namespace", common.SleepNamespace, "istio-injection", "enabled")).To(Succeed(), "Error labeling sample namespace")
				Expect(k.Label("namespace", common.HttpbinNamespace, "istio-injection", "enabled")).To(Succeed(), "Error labeling sample namespace")
				Expect(k.WithNamespace(common.SleepNamespace).Apply(common.GetSampleYAML(latestVersion, "sleep"))).To(Succeed(), "error deploying sleep pod")
				Expect(k.WithNamespace(common.HttpbinNamespace).Apply(common.GetSampleYAML(latestVersion, "httpbin"))).To(Succeed(), "error deploying httpbin pod")
			})

			It("waits for sample pods to be ready", func(ctx SpecContext) {
				Eventually(common.CheckPodsReady).WithArguments(ctx, cl, common.SleepNamespace).Should(Succeed(), "Error checking status of sleep pod")
				Eventually(common.CheckPodsReady).WithArguments(ctx, cl, common.HttpbinNamespace).Should(Succeed(), "Error checking status of httpbin pod")
				Success("Pods are ready")
			})

			It("can access the httpbin service from the sleep pod", func(ctx SpecContext) {
				sleepPod := &corev1.PodList{}
				Expect(cl.List(ctx, sleepPod, client.InNamespace(common.SleepNamespace))).To(Succeed(), "Failed to list sleep pods")
				Expect(sleepPod.Items).ToNot(BeEmpty(), "No sleep pods found")

				// Any logging or diagnostics should also be inside this block
				time.Sleep(60 * time.Second)
				common.CheckPodConnectivity(sleepPod.Items[0].Name, common.SleepNamespace, common.HttpbinNamespace, k)
			})
		})

		When("the Istio CR is deleted", func() {
			BeforeEach(func() {
				Expect(k.Delete("istio", istioName)).To(Succeed(), "Istio CR failed to be deleted")
				Success("Istio CR deleted")
			})

			It("removes everything from the namespace", func(ctx SpecContext) {
				Eventually(cl.Get).WithArguments(ctx, kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{}).
					Should(ReturnNotFoundError(), "Istiod should not exist anymore")
				common.CheckNamespaceEmpty(ctx, cl, controlPlaneNamespace)
				Success("Namespace is empty")
			})
		})

		When("the IstioCNI CR is deleted", func() {
			BeforeEach(func() {
				Expect(k.Delete("istiocni", istioCniName)).To(Succeed(), "IstioCNI CR failed to be deleted")
				Success("IstioCNI deleted")
			})

			It("removes everything from the CNI namespace", func(ctx SpecContext) {
				daemonset := &appsv1.DaemonSet{}
				Eventually(cl.Get).WithArguments(ctx, kube.Key("istio-cni-node", istioCniNamespace), daemonset).
					Should(ReturnNotFoundError(), "IstioCNI DaemonSet should not exist anymore")
				common.CheckNamespaceEmpty(ctx, cl, istioCniNamespace)
				Success("CNI namespace is empty")
			})
		})

		When("the cert-manager-operator resources are deleted", func() {
			BeforeEach(func() {
				err := k.WithNamespace(certManagerOperatorNamespace).Delete("subscription", "openshift-cert-manager-operator")
				if err != nil && !strings.Contains(err.Error(), "NotFound") {
					Fail("Failed to delete Subscription: " + err.Error())
				}

				err = k.WithNamespace(certManagerOperatorNamespace).Delete("operatorgroup", "openshift-cert-manager-operator")
				if err != nil && !strings.Contains(err.Error(), "NotFound") {
					Fail("Failed to delete OperatorGroup: " + err.Error())
				}
			})

			It("removes subscription from the cert-manager-operator namespace", func() {
				Eventually(func() string {
					output, _ := k.WithNamespace(certManagerOperatorNamespace).GetYAML("subscription", "openshift-cert-manager-operator")
					return strings.TrimSpace(output)
				}, 60*time.Second, 5*time.Second).Should(BeEmpty(), "subscription is not removed")
				Success("subscription is removed")
			})

			It("removes operatorgroup from the cert-manager-operator namespace", func() {
				Eventually(func() string {
					output, _ := k.WithNamespace(certManagerOperatorNamespace).GetYAML("operatorgroup", "openshift-cert-manager-operator")
					return strings.TrimSpace(output)
				}, 60*time.Second, 5*time.Second).Should(BeEmpty(), "operatorgroup is not removed")
				Success("operatorgroup is removed")
			})
		})

		When("the cert-manager resources are deleted", func() {
			BeforeEach(func() {
				err = k.WithNamespace(certManagerNamespace).Delete("rolebinding", "cert-manager-cert-manager-tokenrequest")
				if err != nil && !strings.Contains(err.Error(), "NotFound") {
					Fail("Failed to delete rolebinding: " + err.Error())
				}

				err = k.WithNamespace(certManagerNamespace).Delete("role", "cert-manager-tokenrequest")
				if err != nil && !strings.Contains(err.Error(), "NotFound") {
					Fail("Failed to delete role: " + err.Error())
				}
			})

			It("removes rolebinding cert-manager-tokenrequest from the cluster", func() {
				Eventually(func() string {
					output, _ := k.WithNamespace(certManagerNamespace).GetYAML("rolebinding", "cert-manager-cert-manager-tokenrequest")
					return strings.TrimSpace(output)
				}, 60*time.Second, 5*time.Second).Should(BeEmpty(), "rolebinding cert-manager-tokenrequest is not removed")
				Success("rolebinding cert-manager-tokenrequest is removed")
			})

			It("removes role cert-manager-tokenrequest from the cluster", func() {
				Eventually(func() string {
					output, _ := k.WithNamespace(certManagerNamespace).GetYAML("role", "cert-manager-tokenrequest")
					return strings.TrimSpace(output)
				}, 60*time.Second, 5*time.Second).Should(BeEmpty(), "role cert-manager-tokenrequest is not removed")
				Success("role cert-manager-tokenrequest is removed")
			})
		})

		// We are unable to use the standard cleanup method from other tests.
		// Before deleting istio-csr we need to delete components that reference to istio-csr.
		// For details, see: https://github.com/openshift-service-mesh/sail-operator/tree/main/docs/ossm/cert-manager
		When("the IstioCSR is deleted", func() {
			BeforeEach(func() {
				Expect(k.WithNamespace(istioCSRNamespace).Delete("istiocsrs.operator.openshift.io", "default")).To(Succeed(), "Failed to delete istio-csr")
				// Namespaced resources
				Expect(k.WithNamespace(istioCSRNamespace).Delete("deployments.apps", "cert-manager-istio-csr")).To(Succeed(), "Failed to delete deployment")
				Expect(k.WithNamespace(istioCSRNamespace).Delete("services", "cert-manager-istio-csr")).To(Succeed(), "Failed to delete service")
				Expect(k.WithNamespace(istioCSRNamespace).Delete("serviceaccounts", "cert-manager-istio-csr")).To(Succeed(), "Failed to delete service account")
				Expect(k.Delete("namespace", "cert-manager-operator")).To(Succeed(), "Failed to delete namespace")
				Expect(k.Delete("namespace", "cert-manager")).To(Succeed(), "Failed to delete namespace")
				Expect(k.Delete("namespace", "istio-system")).To(Succeed(), "Failed to delete namespace")
				Expect(k.Delete("namespace", "istio-cni")).To(Succeed(), "Failed to delete namespace")
				Expect(k.Delete("namespace", "httpbin")).To(Succeed(), "Failed to delete namespace")
				Expect(k.Delete("namespace", "sleep")).To(Succeed(), "Failed to delete namespace")

				By("Attempting to patch istiocsrs if it exists")
				err := k.WithNamespace(istioCSRNamespace).Patch(
					"istiocsrs.operator.openshift.io",
					"default",
					"merge",
					`{"metadata":{"finalizers":[]}}`,
				)
				if err != nil {
					// Small check to control istiocsr resources are not stuck
					if strings.Contains(err.Error(), `"default" not found`) {
						fmt.Println("Skipping patch. Cleanup succeeded")
					} else {
						Fail(fmt.Sprintf("Unexpected error patching istiocsrs: %v", err))
					}
				}
				Expect(k.Delete("namespace", "istio-csr")).To(Succeed(), "Failed to delete namespace")
			})

			It("removes cert-manager-istio-csr resources from the cluster", func() {
				Eventually(func() string {
					out, _ := k.WithNamespace(istioCSRNamespace).GetYAML("deployments.apps", "cert-manager-istio-csr")
					return strings.TrimSpace(out)
				}, 30*time.Second, 5*time.Second).Should(BeEmpty(), "deployment not removed")

				Eventually(func() string {
					out, _ := k.WithNamespace(istioCSRNamespace).GetYAML("services", "cert-manager-istio-csr")
					return strings.TrimSpace(out)
				}, 30*time.Second, 5*time.Second).Should(BeEmpty(), "service not removed")

				Eventually(func() string {
					out, _ := k.WithNamespace(istioCSRNamespace).GetYAML("serviceaccounts", "cert-manager-istio-csr")
					return strings.TrimSpace(out)
				}, 30*time.Second, 5*time.Second).Should(BeEmpty(), "service account not removed")

				Success("All cert-manager-istio-csr resources are removed")
			})
		})

		AfterAll(func(ctx SpecContext) {
			if CurrentSpecReport().Failed() {
				common.LogDebugInfo(common.ControlPlane, k)
				debugInfoLogged = true
			}

			clr.Cleanup(ctx)
		})
	})

	AfterAll(func() {
		if CurrentSpecReport().Failed() && !debugInfoLogged {
			common.LogDebugInfo(common.ControlPlane, k)
			debugInfoLogged = true
		}
	})
})
