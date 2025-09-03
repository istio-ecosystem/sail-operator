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

package dualstack

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
	. "github.com/istio-ecosystem/sail-operator/tests/e2e/util/gomega"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DualStackNamespace = "dual-stack"
	IPv4Namespace      = "ipv4"
	IPv6Namespace      = "ipv6"
	SleepNamespace     = "sleep"
)

var _ = Describe("DualStack configuration ", Label("dualstack"), Ordered, func() {
	SetDefaultEventuallyTimeout(180 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)

	debugInfoLogged := false

	Describe("for supported versions", func() {
		for _, version := range istioversion.GetLatestPatchVersions() {
			// The minimum supported version is 1.23 (and above)
			if version.Version.LessThan(semver.MustParse("1.23.0")) {
				continue
			}

			Context(fmt.Sprintf("Istio version %s", version.Version), func() {
				clr := cleaner.New(cl)
				BeforeAll(func(ctx SpecContext) {
					clr.Record(ctx)
					Expect(k.CreateNamespace(controlPlaneNamespace)).To(Succeed(), "Istio namespace failed to be created")
					Expect(k.CreateNamespace(istioCniNamespace)).To(Succeed(), "IstioCNI namespace failed to be created")
				})

				When("the IstioCNI CR is created", func() {
					BeforeAll(func() {
						common.CreateIstioCNI(k, version.Name)
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

				When("the Istio CR is created with DualStack configuration", func() {
					BeforeAll(func() {
						spec := `
values:
  meshConfig:
    defaultConfig:
      proxyMetadata:
        ISTIO_DUAL_STACK: "true"
  pilot:
    ipFamilyPolicy: %s
    env:
      ISTIO_DUAL_STACK: "true"
    cni:
      enabled: true`
						common.CreateIstio(k, version.Name, fmt.Sprintf(spec, corev1.IPFamilyPolicyRequireDualStack))
					})

					It("updates the Istio CR status to Reconciled", func(ctx SpecContext) {
						Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(istioName), &v1.Istio{}).
							Should(HaveConditionStatus(v1.IstioConditionReconciled, metav1.ConditionTrue), "Istio is not Reconciled; unexpected Condition")
						Success("Istio CR is Reconciled")
					})

					It("updates the Istio CR status to Ready", func(ctx SpecContext) {
						Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(istioName), &v1.Istio{}).
							Should(HaveConditionStatus(v1.IstioConditionReady, metav1.ConditionTrue), "Istio is not Ready; unexpected Condition")
						Success("Istio CR is Ready")
					})

					It("deploys istiod", func(ctx SpecContext) {
						Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(HaveConditionStatus(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Istiod is not Available; unexpected Condition")
						Expect(common.GetVersionFromIstiod()).To(Equal(version.Version), "Unexpected istiod version")
						Success("Istiod is deployed in the namespace and Running")
					})

					It("uses the correct image", func(ctx SpecContext) {
						Expect(common.GetObject(ctx, cl, kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{})).
							To(HaveContainersThat(HaveEach(ImageFromRegistry(expectedRegistry))))
					})

					It("has ISTIO_DUAL_STACK env variable set", func(ctx SpecContext) {
						Expect(common.GetObject(ctx, cl, kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{})).
							To(HaveContainersThat(ContainElement(WithTransform(getEnvVars, ContainElement(corev1.EnvVar{Name: "ISTIO_DUAL_STACK", Value: "true"})))),
								"Expected ISTIO_DUAL_STACK to be set to true, but not found")
					})

					It("deploys istiod service in dualStack mode", func(ctx SpecContext) {
						var istiodSvcObj corev1.Service

						Eventually(func() error {
							_, err := common.GetObject(ctx, cl, kube.Key("istiod", controlPlaneNamespace), &istiodSvcObj)
							return err
						}).Should(Succeed(), "Expected to retrieve the 'istiod' service")

						Expect(istiodSvcObj.Spec.IPFamilyPolicy).ToNot(BeNil(), "Expected IPFamilyPolicy to be set")
						Expect(*istiodSvcObj.Spec.IPFamilyPolicy).To(Equal(corev1.IPFamilyPolicyRequireDualStack), "Expected ipFamilyPolicy to be 'RequireDualStack'")
						Success("Istio Service is deployed in the namespace and Running")
					})
				})

				// We spawn the following pods to verify the data-path connectivity.
				// 1. a dualStack service in dual-stack namespace which listens on both IPv4 and IPv6 addresses
				// 2. an ipv4 only service in ipv4 namespace which listens only on IPv4 address
				// 3. an ipv6 only service in ipv6 namespace which listens only on IPv6 address
				// Using a sleep pod from the sleep namespace, we try to connect to all the three services to verify that connectivity is successful.
				When("sample apps are deployed in the cluster", func() {
					BeforeAll(func(ctx SpecContext) {
						Expect(k.CreateNamespace(DualStackNamespace)).To(Succeed(), "Failed to create dual-stack namespace")
						Expect(k.CreateNamespace(IPv4Namespace)).To(Succeed(), "Failed to create ipv4 namespace")
						Expect(k.CreateNamespace(IPv6Namespace)).To(Succeed(), "Failed to create ipv6 namespace")
						Expect(k.CreateNamespace(SleepNamespace)).To(Succeed(), "Failed to create sleep namespace")

						Expect(k.Label("namespace", DualStackNamespace, "istio-injection", "enabled")).To(Succeed(), "Error labeling dual-stack namespace")
						Expect(k.Label("namespace", IPv4Namespace, "istio-injection", "enabled")).To(Succeed(), "Error labeling ipv4 namespace")
						Expect(k.Label("namespace", IPv6Namespace, "istio-injection", "enabled")).To(Succeed(), "Error labeling ipv6 namespace")
						Expect(k.Label("namespace", SleepNamespace, "istio-injection", "enabled")).To(Succeed(), "Error labeling sleep namespace")

						Expect(k.WithNamespace(DualStackNamespace).
							ApplyKustomize("tcp-echo-dual-stack")).
							To(Succeed(), "error deploying tcpDualStack pod")
						Expect(k.WithNamespace(IPv4Namespace).
							ApplyKustomize("tcp-echo-ipv4")).
							To(Succeed(), "error deploying ipv4 pod")
						Expect(k.WithNamespace(IPv6Namespace).
							ApplyKustomize("tcp-echo-ipv6")).
							To(Succeed(), "error deploying ipv6 pod")
						Expect(k.WithNamespace(SleepNamespace).
							ApplyKustomize("sleep")).
							To(Succeed(), "error deploying sleep pod")

						Success("dualStack validation pods deployed")
					})

					sleepPod := &corev1.PodList{}
					It("updates the status of pods to Running", func(ctx SpecContext) {
						Eventually(common.CheckPodsReady).WithArguments(ctx, cl, DualStackNamespace).Should(Succeed(), "Error checking status of dual-stack pod")
						Eventually(common.CheckPodsReady).WithArguments(ctx, cl, IPv4Namespace).Should(Succeed(), "Error checking status of ipv4 pod")
						Eventually(common.CheckPodsReady).WithArguments(ctx, cl, IPv6Namespace).Should(Succeed(), "Error checking status of ipv6 pod")
						Eventually(common.CheckPodsReady).WithArguments(ctx, cl, SleepNamespace).Should(Succeed(), "Error checking status of sleep pod")
						Expect(cl.List(ctx, sleepPod, client.InNamespace(SleepNamespace))).To(Succeed(), "Error getting the pod in sleep namespace")

						Success("Pods are ready")
					})

					It("can access the dual-stack service from the sleep pod", func(ctx SpecContext) {
						checkTCPEchoConnectivity(sleepPod.Items[0].Name, SleepNamespace, DualStackNamespace)
					})

					It("can access the ipv4 only service from the sleep pod", func(ctx SpecContext) {
						checkTCPEchoConnectivity(sleepPod.Items[0].Name, SleepNamespace, IPv4Namespace)
					})

					It("can access the ipv6 only service from the sleep pod", func(ctx SpecContext) {
						checkTCPEchoConnectivity(sleepPod.Items[0].Name, SleepNamespace, IPv6Namespace)
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
				common.LogDebugInfo(common.DualStack, k)
				debugInfoLogged = true
			}
		})
	})

	AfterAll(func(ctx SpecContext) {
		if CurrentSpecReport().Failed() && !debugInfoLogged {
			common.LogDebugInfo(common.DualStack, k)
			debugInfoLogged = true
		}
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

func checkTCPEchoConnectivity(podName, namespace, echoStr string) {
	command := fmt.Sprintf(`sh -c 'echo %s | nc tcp-echo.%s 9000'`, echoStr, echoStr)
	response, err := k.WithNamespace(namespace).Exec(podName, "sleep", command)
	Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("error connecting to the %q pod", podName))
	Expect(response).To(ContainSubstring(fmt.Sprintf("hello %s", echoStr)), fmt.Sprintf("Unexpected response from %s pod", podName))
}
