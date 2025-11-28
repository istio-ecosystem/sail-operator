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
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/cleaner"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultTimeout = 180
)

var _ = Describe("Ambient configuration ", Label("smoke", "ambient"), Ordered, func() {
	SetDefaultEventuallyTimeout(defaultTimeout * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)

	debugInfoLogged := false

	Describe("for supported versions", func() {
		for _, version := range istioversion.GetLatestPatchVersions() {
			// The minimum supported version is 1.24 (and above)
			if version.Version.LessThan(semver.MustParse("1.24.0")) {
				continue
			}

			Context(fmt.Sprintf("Istio version %s", version.Version), func() {
				clr := cleaner.New(cl)
				BeforeAll(func(ctx SpecContext) {
					clr.Record(ctx)
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
  values:
    cni:
      ambient:
        dnsCapture: true
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

					It("uses the configured values in the istio-cni-config config map", func(ctx SpecContext) {
						cm := corev1.ConfigMap{}

						Eventually(func() error {
							if _, err := common.GetObject(ctx, cl, kube.Key("istio-cni-config", istioCniNamespace), &cm); err != nil {
								return err
							}

							if val, ok := cm.Data["AMBIENT_DNS_CAPTURE"]; !ok || val != "true" {
								return fmt.Errorf("expected AMBIENT_DNS_CAPTURE=true, got %q", val)
							}
							return nil
						}).Should(Succeed(), "Expected 'AMBIENT_DNS_CAPTURE' to be set to 'true'")
					})
				})

				When("the Istio CR is created with ambient profile", func() {
					BeforeAll(func() {
						common.CreateIstio(k, version.Name, `
values:
  pilot:
    trustedZtunnelNamespace: ztunnel
profile: ambient`)
					})

					It("updates the Istio CR status to Reconciled", func(ctx SpecContext) {
						common.AwaitCondition(ctx, v1.IstioConditionReconciled, kube.Key(istioName), &v1.Istio{}, k, cl)
					})

					It("updates the Istio CR status to Ready", func(ctx SpecContext) {
						common.AwaitCondition(ctx, v1.IstioConditionReady, kube.Key(istioName), &v1.Istio{}, k, cl)
					})

					It("deploys istiod", func(ctx SpecContext) {
						common.AwaitDeployment(ctx, "istiod", k, cl)
						Expect(common.GetVersionFromIstiod()).To(Equal(version.Version), "Unexpected istiod version")
					})

					It("uses the correct image", func(ctx SpecContext) {
						Expect(common.GetObject(ctx, cl, kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{})).
							To(common.HaveContainersThat(HaveEach(common.ImageFromRegistry(expectedRegistry))))
					})

					It("has istiod with appropriate env variables set", func(ctx SpecContext) {
						var istiodObj appsv1.Deployment

						Eventually(func() error {
							_, err := common.GetObject(ctx, cl, kube.Key("istiod", controlPlaneNamespace), &istiodObj)
							return err
						}).Should(Succeed(), "Expected to retrieve the 'istiod' deployment")

						Expect(istiodObj).To(common.HaveContainersThat(ContainElement(WithTransform(getEnvVars,
							ContainElement(corev1.EnvVar{Name: "PILOT_ENABLE_AMBIENT", Value: "true"})))),
							"Expected PILOT_ENABLE_AMBIENT to be set to true, but not found")

						Expect(istiodObj).To(common.HaveContainersThat(ContainElement(WithTransform(getEnvVars,
							ContainElement(corev1.EnvVar{Name: "CA_TRUSTED_NODE_ACCOUNTS", Value: "ztunnel/ztunnel"})))),
							"Expected CA_TRUSTED_NODE_ACCOUNTS to be set to ztunnel/ztunnel, but not found")
					})
				})

				When("the ZTunnel CR is created", func() {
					BeforeAll(func() {
						ztunnelYaml := `
apiVersion: sailoperator.io/v1
kind: ZTunnel
metadata:
  name: default
spec:
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

						Expect(ztunnelObj).To(common.HaveContainersThat(ContainElement(WithTransform(getEnvVars,
							ContainElement(corev1.EnvVar{Name: "XDS_ADDRESS", Value: "istiod.istio-system.svc:15012"})))),
							"Expected XDS_ADDRESS to be set to istiod.istio-system.svc:15012, but not found")

						Expect(ztunnelObj).To(common.HaveContainersThat(ContainElement(WithTransform(getEnvVars,
							ContainElement(corev1.EnvVar{Name: "ISTIO_META_ENABLE_HBONE", Value: "true"})))),
							"Expected ISTIO_META_ENABLE_HBONE to be set to true, but not found")

						Expect(ztunnelObj).To(common.HaveContainersThat(ContainElement(WithTransform(getEnvVars,
							ContainElement(corev1.EnvVar{Name: "CUSTOM_ENV_VAR", Value: "true"})))),
							"Expected CUSTOM_ENV_VAR to be set to true, but not found")
					})
				})

				// We spawn the following pods to verify the data-path connectivity.
				// an httpbin service in httpbin namespace that listens of port 8000
				// using a sleep pod from the sleep namespace, we try to connect to the httpbin service to verify that connectivity is successful.
				When("sample apps are deployed in the cluster", func() {
					BeforeAll(func(ctx SpecContext) {
						Expect(k.CreateNamespace(common.SleepNamespace)).To(Succeed(), "Failed to create sleep namespace")
						Expect(k.CreateNamespace(common.HttpbinNamespace)).To(Succeed(), "Failed to create httpbin namespace")

						// Add the necessary ambient labels on the namespaces.
						Expect(k.Label("namespace", common.SleepNamespace, "istio.io/dataplane-mode", "ambient")).To(Succeed(), "Error labeling sleep namespace")
						Expect(k.Label("namespace", common.HttpbinNamespace, "istio.io/dataplane-mode", "ambient")).To(Succeed(), "Error labeling httpbin namespace")

						// Deploy the test pods.
						Expect(k.WithNamespace(common.SleepNamespace).ApplyKustomize("sleep")).To(Succeed(), "Error deploying sleep pod")
						Expect(k.WithNamespace(common.HttpbinNamespace).ApplyKustomize("httpbin")).To(Succeed(), "Error deploying httpbin pod")

						Success("Ambient validation pods deployed")
					})

					sleepPod := &corev1.PodList{}
					It("updates the status of pods to Running", func(ctx SpecContext) {
						Eventually(common.CheckPodsReady).WithArguments(ctx, cl, common.SleepNamespace).Should(Succeed(), "Error checking status of sleep pod")
						Eventually(common.CheckPodsReady).WithArguments(ctx, cl, common.HttpbinNamespace).Should(Succeed(), "Error checking status of httpbin pod")
						Expect(cl.List(ctx, sleepPod, client.InNamespace(common.SleepNamespace))).To(Succeed(), "Error getting the pod in sleep namespace")

						Success("Pods are ready")
					})

					It("has the ztunnel proxy sockets configured in the pod network namespace", func(ctx SpecContext) {
						checkZtunnelPort(sleepPod.Items[0].Name, common.SleepNamespace)
					})

					It("can access the httpbin service from the sleep pod", func(ctx SpecContext) {
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

				AfterAll(func(ctx SpecContext) {
					if CurrentSpecReport().Failed() && keepOnFailure {
						return
					}

					clr.Cleanup(ctx)
				})
			})
		}

		AfterAll(func(ctx SpecContext) {
			if CurrentSpecReport().Failed() {
				common.LogDebugInfo(common.Ambient, k)
				debugInfoLogged = true
			}
		})
	})

	AfterAll(func(ctx SpecContext) {
		if CurrentSpecReport().Failed() && !debugInfoLogged {
			common.LogDebugInfo(common.Ambient, k)
			debugInfoLogged = true
		}
	})
})

func getEnvVars(container corev1.Container) []corev1.EnvVar {
	return container.Env
}

func checkZtunnelPort(podName, srcNamespace string) {
	response, err := k.WithNamespace(srcNamespace).Exec(podName, srcNamespace, "netstat -tlpn")
	Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("error validating the proxy sockets in the %q pod", podName))
	// Verify that the HBONE mTLS tunnel port (15008) is listed in the output.
	Expect(response).To(ContainSubstring("15008"), fmt.Sprintf("Unexpected response from %s pod", podName))
}
