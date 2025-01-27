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

package ambient

import (
	"fmt"
	"time"

	"github.com/Masterminds/semver/v3"
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/pkg/test/util/supportedversion"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	. "github.com/istio-ecosystem/sail-operator/tests/e2e/util/gomega"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	sleepNamespace   = "sleep"
	httpbinNamespace = "httpbin"
	defaultTimeout   = 180
)

var _ = Describe("Ambient configuration ", Ordered, func() {
	SetDefaultEventuallyTimeout(defaultTimeout * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)

	debugInfoLogged := false

	BeforeAll(func(ctx SpecContext) {
		Expect(k.CreateNamespace(operatorNamespace)).To(Succeed(), "Namespace failed to be created")

		if skipDeploy {
			Success("Skipping operator installation because it was deployed externally")
		} else {
			Expect(common.InstallOperatorViaHelm()).
				To(Succeed(), "Operator failed to be deployed")
		}

		Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(deploymentName, operatorNamespace), &appsv1.Deployment{}).
			Should(HaveCondition(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Error getting Istio CRD")
		Success("Operator is deployed in the namespace and Running")
	})

	Describe("for supported versions", func() {
		for _, version := range supportedversion.List {
			// The minimum supported version is 1.24 (and above)
			if version.Version.LessThan(semver.MustParse("1.24.0")) {
				continue
			}

			Context(fmt.Sprintf("Istio version %s", version.Version), func() {
				BeforeAll(func() {
					Expect(k.CreateNamespace(controlPlaneNamespace)).To(Succeed(), "Istio namespace failed to be created")
					Expect(k.CreateNamespace(istioCniNamespace)).To(Succeed(), "IstioCNI namespace failed to be created")
					Expect(k.CreateNamespace(ztunnelNamespace)).To(Succeed(), "ZTunnel namespace failed to be created")
				})

				When("the IstioCNI CR is created with ambient profile", func() {
					BeforeAll(func() {
						cniYAML := `
apiVersion: sailoperator.io/v1
kind: IstioCNI
metadata:
  name: default
spec:
  profile: ambient
  version: %s
  namespace: %s`
						cniYAML = fmt.Sprintf(cniYAML, version.Name, istioCniNamespace)
						Log("IstioCNI YAML:", cniYAML)
						Expect(k.CreateFromString(cniYAML)).To(Succeed(), "IstioCNI creation failed")
						Success("IstioCNI created")
					})

					It("deploys the CNI DaemonSet", func(ctx SpecContext) {
						Eventually(func(g Gomega) {
							daemonset := &appsv1.DaemonSet{}
							g.Expect(cl.Get(ctx, kube.Key("istio-cni-node", istioCniNamespace), daemonset)).To(Succeed(), "Error getting IstioCNI DaemonSet")
							g.Expect(daemonset.Status.NumberAvailable).
								To(Equal(daemonset.Status.CurrentNumberScheduled), "CNI DaemonSet Pods not Available; expected numberAvailable to be equal to currentNumberScheduled")
						}).Should(Succeed(), "CNI DaemonSet Pods are not Available")
						Success("CNI DaemonSet is deployed in the namespace and Running")
					})
				})

				When("the Istio CR is created with ambient profile", func() {
					BeforeAll(func() {
						istioYAML := `
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  values:
    pilot:
      trustedZtunnelNamespace: ztunnel
  profile: ambient
  version: %s
  namespace: %s`
						istioYAML = fmt.Sprintf(istioYAML, version.Name, controlPlaneNamespace)
						Log("Istio YAML:", istioYAML)
						Expect(k.CreateFromString(istioYAML)).
							To(Succeed(), "Istio CR failed to be created")
						Success("Istio CR created")
					})

					It("updates the Istio CR status to Reconciled", func(ctx SpecContext) {
						Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(istioName), &v1.Istio{}).
							Should(HaveCondition(v1.IstioConditionReconciled, metav1.ConditionTrue), "Istio is not Reconciled; unexpected Condition")
						Success("Istio CR is Reconciled")
					})

					It("updates the Istio CR status to Ready", func(ctx SpecContext) {
						Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(istioName), &v1.Istio{}).
							Should(HaveCondition(v1.IstioConditionReady, metav1.ConditionTrue), "Istio is not Ready; unexpected Condition")
						Success("Istio CR is Ready")
					})

					It("deploys istiod", func(ctx SpecContext) {
						Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(HaveCondition(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Istiod is not Available; unexpected Condition")
						Expect(common.GetVersionFromIstiod()).To(Equal(version.Version), "Unexpected istiod version")
						Success("Istiod is deployed in the namespace and Running")
					})

					It("uses the correct image", func(ctx SpecContext) {
						Expect(common.GetObject(ctx, cl, kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{})).
							To(HaveContainersThat(HaveEach(ImageFromRegistry(expectedRegistry))))
					})

					It("has istiod with appropriate env variables set", func(ctx SpecContext) {
						var istiodObj appsv1.Deployment

						Eventually(func() error {
							_, err := common.GetObject(ctx, cl, kube.Key("istiod", controlPlaneNamespace), &istiodObj)
							return err
						}).Should(Succeed(), "Expected to retrieve the 'istiod' deployment")

						Expect(istiodObj).To(HaveContainersThat(ContainElement(WithTransform(getEnvVars,
							ContainElement(corev1.EnvVar{Name: "PILOT_ENABLE_AMBIENT", Value: "true"})))),
							"Expected PILOT_ENABLE_AMBIENT to be set to true, but not found")

						Expect(istiodObj).To(HaveContainersThat(ContainElement(WithTransform(getEnvVars,
							ContainElement(corev1.EnvVar{Name: "CA_TRUSTED_NODE_ACCOUNTS", Value: "ztunnel/ztunnel"})))),
							"Expected CA_TRUSTED_NODE_ACCOUNTS to be set to ztunnel/ztunnel, but not found")
					})
				})

				When("the ZTunnel CR is created", func() {
					BeforeAll(func() {
						ztunnelYaml := `
apiVersion: sailoperator.io/v1alpha1
kind: ZTunnel
metadata:
  name: default
spec:
  profile: ambient
  version: %s
  namespace: %s
  values:
    ztunnel:
      env:
        CUSTOM_ENV_VAR: "true"`
						ztunnelYaml = fmt.Sprintf(ztunnelYaml, version.Name, ztunnelNamespace)
						Log("ZTunnel YAML:", ztunnelYaml)
						Expect(k.CreateFromString(ztunnelYaml)).To(Succeed(), "ZTunnel creation failed")
						Success("ZTunnel created")
					})

					It("deploys the ZTunnel DaemonSet", func(ctx SpecContext) {
						Eventually(func(g Gomega) {
							daemonset := &appsv1.DaemonSet{}
							g.Expect(cl.Get(ctx, kube.Key("ztunnel", ztunnelNamespace), daemonset)).To(Succeed(), "Error getting ZTunnel DaemonSet")
							g.Expect(daemonset.Status.NumberAvailable).
								To(Equal(daemonset.Status.CurrentNumberScheduled),
									"ZTunnel DaemonSet Pods not Available; expected numberAvailable to be equal to currentNumberScheduled")
						}).Should(Succeed(), "ZTunnel DaemonSet Pods are not Available")
						Success("ZTunnel DaemonSet is deployed and Running")
					})

					It("has ztunnel running with appropriate env variables set", func(ctx SpecContext) {
						var ztunnelObj appsv1.DaemonSet

						Eventually(func() error {
							_, err := common.GetObject(ctx, cl, kube.Key("ztunnel", ztunnelNamespace), &ztunnelObj)
							return err
						}).Should(Succeed(), "Expected to retrieve the 'ztunnel' daemonSet")

						Expect(ztunnelObj).To(HaveContainersThat(ContainElement(WithTransform(getEnvVars,
							ContainElement(corev1.EnvVar{Name: "XDS_ADDRESS", Value: "istiod.istio-system.svc:15012"})))),
							"Expected XDS_ADDRESS to be set to istiod.istio-system.svc:15012, but not found")

						Expect(ztunnelObj).To(HaveContainersThat(ContainElement(WithTransform(getEnvVars,
							ContainElement(corev1.EnvVar{Name: "ISTIO_META_ENABLE_HBONE", Value: "true"})))),
							"Expected ISTIO_META_ENABLE_HBONE to be set to true, but not found")

						Expect(ztunnelObj).To(HaveContainersThat(ContainElement(WithTransform(getEnvVars,
							ContainElement(corev1.EnvVar{Name: "CUSTOM_ENV_VAR", Value: "true"})))),
							"Expected CUSTOM_ENV_VAR to be set to true, but not found")
					})
				})

				// We spawn the following pods to verify the data-path connectivity.
				// an httpbin service in httpbin namespace that listens of port 8000
				// using a sleep pod from the sleep namespace, we try to connect to the httpbin service to verify that connectivity is successful.
				When("sample apps are deployed in the cluster", func() {
					BeforeAll(func(ctx SpecContext) {
						Expect(k.CreateNamespace(sleepNamespace)).To(Succeed(), "Failed to create sleep namespace")
						Expect(k.CreateNamespace(httpbinNamespace)).To(Succeed(), "Failed to create httpbin namespace")

						// Add the necessary ambient labels on the namespaces.
						Expect(k.Patch("namespace", sleepNamespace, "merge", `{"metadata":{"labels":{"istio.io/dataplane-mode":"ambient"}}}`)).
							To(Succeed(), "Error patching sleep namespace")
						Expect(k.Patch("namespace", httpbinNamespace, "merge", `{"metadata":{"labels":{"istio.io/dataplane-mode":"ambient"}}}`)).
							To(Succeed(), "Error patching httpbin namespace")

						// Deploy the test pods.
						Expect(k.WithNamespace(sleepNamespace).Apply(common.GetSampleYAML(version, "sleep"))).To(Succeed(), "error deploying sleep pod")
						Expect(k.WithNamespace(httpbinNamespace).Apply(common.GetSampleYAML(version, "httpbin"))).To(Succeed(), "error deploying httpbin pod")

						Success("Ambient validation pods deployed")
					})

					sleepPod := &corev1.PodList{}
					It("updates the status of pods to Running", func(ctx SpecContext) {
						sleepPod, err = common.CheckPodsReady(ctx, cl, sleepNamespace)
						Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("Error checking status of sleep pod: %v", err))

						_, err = common.CheckPodsReady(ctx, cl, httpbinNamespace)
						Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("Error checking status of httpbin pod: %v", err))

						Success("Pods are ready")
					})

					It("has the ztunnel proxy sockets configured in the pod network namespace", func(ctx SpecContext) {
						checkZtunnelPort(sleepPod.Items[0].Name, sleepNamespace)
					})

					It("can access the httpbin service from the sleep pod", func(ctx SpecContext) {
						checkPodConnectivity(sleepPod.Items[0].Name, sleepNamespace, httpbinNamespace)
					})

					AfterAll(func(ctx SpecContext) {
						By("Deleting the pods")
						Expect(k.DeleteNamespace(httpbinNamespace, sleepNamespace)).
							To(Succeed(), "Failed to delete namespaces")
						Success("Ambient validation pods deleted")
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

				When("the ZTunnel CR is deleted", func() {
					BeforeEach(func() {
						Expect(k.Delete("ztunnel", "default")).To(Succeed(), "ZTunnel CR failed to be deleted")
						Success("ZTunnel deleted")
					})

					It("removes everything from the ztunnel namespace", func(ctx SpecContext) {
						daemonset := &appsv1.DaemonSet{}
						Eventually(cl.Get).WithArguments(ctx, kube.Key("ztunnel", ztunnelNamespace), daemonset).
							Should(ReturnNotFoundError(), "ztunnel daemonSet should not exist anymore")
						common.CheckNamespaceEmpty(ctx, cl, ztunnelNamespace)
						Success("ztunnel namespace is empty")
					})
				})
			})
		}

		AfterAll(func(ctx SpecContext) {
			if CurrentSpecReport().Failed() {
				common.LogDebugInfo(k)
				debugInfoLogged = true
			}

			By("Cleaning up the Istio namespace")
			Expect(k.DeleteNamespace(controlPlaneNamespace)).To(Succeed(), "Istio Namespace failed to be deleted")

			By("Cleaning up the IstioCNI namespace")
			Expect(k.DeleteNamespace(istioCniNamespace)).To(Succeed(), "IstioCNI Namespace failed to be deleted")

			By("Cleaning up the ZTunnel namespace")
			Expect(k.DeleteNamespace(ztunnelNamespace)).To(Succeed(), "ZTunnel Namespace failed to be deleted")
		})
	})

	AfterAll(func() {
		if CurrentSpecReport().Failed() && !debugInfoLogged {
			common.LogDebugInfo(k)
			debugInfoLogged = true
		}

		if skipDeploy {
			Success("Skipping operator undeploy because it was deployed externally")
			return
		}

		By("Deleting operator deployment")
		Expect(common.UninstallOperator()).
			To(Succeed(), "Operator failed to be deleted")
		GinkgoWriter.Println("Operator uninstalled")

		Expect(k.DeleteNamespace(operatorNamespace)).To(Succeed(), "Namespace failed to be deleted")
		Success("Namespace deleted")
	})
})

func HaveContainersThat(matcher types.GomegaMatcher) types.GomegaMatcher {
	return HaveField("Spec.Template.Spec.Containers", matcher)
}

func ImageFromRegistry(regexp string) types.GomegaMatcher {
	return HaveField("Image", MatchRegexp(regexp))
}

func getEnvVars(container corev1.Container) []corev1.EnvVar {
	return container.Env
}

func checkPodConnectivity(podName, srcNamespace, destNamespace string) {
	command := fmt.Sprintf(`curl -o /dev/null -s -w "%%{http_code}\n" httpbin.%s.svc.cluster.local:8000/get`, destNamespace)
	response, err := k.WithNamespace(srcNamespace).Exec(podName, srcNamespace, command)
	Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("error connecting to the %q pod", podName))
	Expect(response).To(ContainSubstring("200"), fmt.Sprintf("Unexpected response from %s pod", podName))
}

func checkZtunnelPort(podName, srcNamespace string) {
	response, err := k.WithNamespace(srcNamespace).Exec(podName, srcNamespace, "netstat -tlpn")
	Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("error validating the proxy sockets in the %q pod", podName))
	// Verify that the HBONE mTLS tunnel port (15008) is listed in the output.
	Expect(response).To(ContainSubstring("15008"), fmt.Sprintf("Unexpected response from %s pod", podName))
}
