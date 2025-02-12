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

package controlplane

import (
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversions"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	. "github.com/istio-ecosystem/sail-operator/tests/e2e/util/gomega"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"istio.io/istio/pkg/ptr"
)

var _ = Describe("Control Plane Installation", Ordered, func() {
	SetDefaultEventuallyTimeout(180 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)
	debugInfoLogged := false

	BeforeAll(func(ctx SpecContext) {
		Expect(k.CreateNamespace(namespace)).To(Succeed(), "Namespace failed to be created")

		if skipDeploy {
			Success("Skipping operator installation because it was deployed externally")
		} else {
			Expect(common.InstallOperatorViaHelm()).
				To(Succeed(), "Operator failed to be deployed")
		}

		Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(deploymentName, namespace), &appsv1.Deployment{}).
			Should(HaveCondition(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Error getting Istio CRD")
		Success("Operator is deployed in the namespace and Running")
	})

	Describe("defaulting", func() {
		DescribeTable("IstioCNI",
			Entry("no spec", ""),
			Entry("empty spec", "spec: {}"),
			func(ctx SpecContext, spec string) {
				yaml := `
apiVersion: sailoperator.io/v1
kind: IstioCNI
metadata:
  name: default
` + spec
				Expect(k.CreateFromString(yaml)).To(Succeed(), "IstioCNI creation failed")
				Success("IstioCNI created")

				cni := &v1.IstioCNI{}
				Expect(cl.Get(ctx, kube.Key("default"), cni)).To(Succeed())
				Expect(cni.Spec.Version).To(Equal(istioversions.Default))
				Expect(cni.Spec.Namespace).To(Equal("istio-cni"))

				Expect(cl.Delete(ctx, cni)).To(Succeed())
				Eventually(cl.Get).WithArguments(ctx, kube.Key("default"), cni).Should(ReturnNotFoundError())
			},
		)

		DescribeTable("Istio",
			Entry("no spec", ""),
			Entry("empty spec", "spec: {}"),
			Entry("empty updateStrategy", "spec: {updateStrategy: {}}"),
			func(ctx SpecContext, spec string) {
				yaml := `
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
` + spec
				Expect(k.CreateFromString(yaml)).To(Succeed(), "Istio creation failed")
				Success("Istio created")

				istio := &v1.Istio{}
				Expect(cl.Get(ctx, kube.Key("default"), istio)).To(Succeed())
				Expect(istio.Spec.Version).To(Equal(istioversions.Default))
				Expect(istio.Spec.Namespace).To(Equal("istio-system"))
				Expect(istio.Spec.UpdateStrategy).ToNot(BeNil())
				Expect(istio.Spec.UpdateStrategy.Type).To(Equal(v1.UpdateStrategyTypeInPlace))

				Expect(cl.Delete(ctx, istio)).To(Succeed())
				Eventually(cl.Get).WithArguments(ctx, kube.Key("default"), istio).Should(ReturnNotFoundError())
			},
		)
	})

	Describe("given Istio version", func() {
		for name, version := range istioversions.Map {
			Context(name, func() {
				BeforeAll(func() {
					Expect(k.CreateNamespace(controlPlaneNamespace)).To(Succeed(), "Istio namespace failed to be created")
					Expect(k.CreateNamespace(istioCniNamespace)).To(Succeed(), "IstioCNI namespace failed to be created")
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
						yaml = fmt.Sprintf(yaml, name, istioCniNamespace)
						Log("IstioCNI YAML:", indent(2, yaml))
						Expect(k.CreateFromString(yaml)).To(Succeed(), "IstioCNI creation failed")
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

					It("uses the correct image", func(ctx SpecContext) {
						Expect(common.GetObject(ctx, cl, kube.Key("istio-cni-node", istioCniNamespace), &appsv1.DaemonSet{})).
							To(HaveContainersThat(HaveEach(ImageFromRegistry(expectedRegistry))))
					})

					It("updates the status to Reconciled", func(ctx SpecContext) {
						Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(istioCniName), &v1.IstioCNI{}).
							Should(HaveCondition(v1.IstioCNIConditionReconciled, metav1.ConditionTrue), "IstioCNI is not Reconciled; unexpected Condition")
						Success("IstioCNI is Reconciled")
					})

					It("updates the status to Ready", func(ctx SpecContext) {
						Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(istioCniName), &v1.IstioCNI{}).
							Should(HaveCondition(v1.IstioCNIConditionReady, metav1.ConditionTrue), "IstioCNI is not Ready; unexpected Condition")
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
  version: %s
  namespace: %s`
						istioYAML = fmt.Sprintf(istioYAML, name, controlPlaneNamespace)
						Log("Istio YAML:", indent(2, istioYAML))
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

					It("doesn't continuously reconcile the Istio CR", func() {
						Eventually(k.WithNamespace(namespace).Logs).WithArguments("deploy/"+deploymentName, ptr.Of(30*time.Second)).
							ShouldNot(ContainSubstring("Reconciliation done"), "Istio CR is continuously reconciling")
						Success("Istio CR stopped reconciling")
					})
				})

				When("sample pod is deployed", func() {
					BeforeAll(func() {
						if isAlias(name, version) {
							Skip("Skipping test for alias version")
						}

						Expect(k.CreateNamespace(sampleNamespace)).To(Succeed(), "Sample namespace failed to be created")
						Expect(k.Patch("namespace", sampleNamespace, "merge", `{"metadata":{"labels":{"istio-injection":"enabled"}}}`)).
							To(Succeed(), "Error patching sample namespace")
						Expect(k.WithNamespace(sampleNamespace).
							ApplyWithLabels(common.GetSampleYAML(version, sampleNamespace), "version=v1")).
							To(Succeed(), "Error deploying sample")
						Success("sample deployed")
					})

					samplePods := &corev1.PodList{}

					It("updates the pods status to Running", func(ctx SpecContext) {
						Eventually(func() bool {
							// Wait until the sample pod exists. Is wraped inside a function to avoid failure on the first iteration
							Expect(cl.List(ctx, samplePods, client.InNamespace(sampleNamespace))).To(Succeed())
							return len(samplePods.Items) > 0
						}).Should(BeTrue(), "No sample pods found")

						Expect(cl.List(ctx, samplePods, client.InNamespace(sampleNamespace))).To(Succeed())
						Expect(samplePods.Items).ToNot(BeEmpty(), "No pods found in sample namespace")

						for _, pod := range samplePods.Items {
							Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(pod.Name, sampleNamespace), &corev1.Pod{}).
								Should(HaveCondition(corev1.PodReady, metav1.ConditionTrue), "Pod is not Ready")
						}
						Success("sample pods are ready")
					})

					It("has sidecars with the correct istio version", func(ctx SpecContext) {
						for _, pod := range samplePods.Items {
							sidecarVersion, err := getProxyVersion(pod.Name, sampleNamespace)
							Expect(err).NotTo(HaveOccurred(), "Error getting sidecar version")
							Expect(sidecarVersion).To(Equal(version.Version), "Sidecar Istio version does not match the expected version")
						}
						Success("Istio sidecar version matches the expected Istio version")
					})

					AfterAll(func(ctx SpecContext) {
						By("Deleting sample")
						Expect(k.DeleteNamespace(sampleNamespace)).To(Succeed(), "sample namespace failed to be deleted")
						Success("sample deleted")
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
			})
		}

		AfterAll(func(ctx SpecContext) {
			if CurrentSpecReport().Failed() {
				common.LogDebugInfo(common.ControlPlane, k)
				debugInfoLogged = true
			}

			By("Cleaning up the Istio namespace")
			Expect(k.DeleteNamespace(controlPlaneNamespace)).To(Succeed(), "Istio Namespace failed to be deleted")

			By("Cleaning up the IstioCNI namespace")
			Expect(k.DeleteNamespace(istioCniNamespace)).To(Succeed(), "IstioCNI Namespace failed to be deleted")

			Success("Cleanup done")
		})
	})

	AfterAll(func() {
		if CurrentSpecReport().Failed() && !debugInfoLogged {
			common.LogDebugInfo(common.ControlPlane, k)
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

		Expect(k.DeleteNamespace(namespace)).To(Succeed(), "Namespace failed to be deleted")
		Success("Namespace deleted")
	})
})

func isAlias(name string, version istioversions.VersionInfo) bool {
	return name != version.Name
}

func HaveContainersThat(matcher types.GomegaMatcher) types.GomegaMatcher {
	return HaveField("Spec.Template.Spec.Containers", matcher)
}

func ImageFromRegistry(regexp string) types.GomegaMatcher {
	return HaveField("Image", MatchRegexp(regexp))
}

func indent(level int, str string) string {
	indent := strings.Repeat(" ", level)
	return indent + strings.ReplaceAll(str, "\n", "\n"+indent)
}

func getProxyVersion(podName, namespace string) (*semver.Version, error) {
	output, err := k.WithNamespace(namespace).Exec(
		podName,
		"istio-proxy",
		`curl -s http://localhost:15000/server_info | grep "ISTIO_VERSION" | awk -F '"' '{print $4}'`)
	if err != nil {
		return nil, fmt.Errorf("error getting sidecar version: %w", err)
	}

	versionStr := strings.TrimSpace(output)
	version, err := semver.NewVersion(versionStr)
	if err != nil {
		return version, fmt.Errorf("error parsing sidecar version %q: %w", versionStr, err)
	}
	return version, err
}
