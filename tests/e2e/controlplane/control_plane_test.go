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
	"regexp"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/cleaner"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/istioctl"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"istio.io/istio/pkg/ptr"
)

var _ = Describe("Control Plane Installation", Label("smoke", "control-plane", "slow"), Ordered, func() {
	SetDefaultEventuallyTimeout(180 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)
	debugInfoLogged := false

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
				Expect(cni.Spec.Version).To(Equal(istioversion.Default))
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
				Expect(istio.Spec.Version).To(Equal(istioversion.Default))
				Expect(istio.Spec.Namespace).To(Equal("istio-system"))
				Expect(istio.Spec.UpdateStrategy).ToNot(BeNil())
				Expect(istio.Spec.UpdateStrategy.Type).To(Equal(v1.UpdateStrategyTypeInPlace))

				Expect(cl.Delete(ctx, istio)).To(Succeed())
				Eventually(cl.Get).WithArguments(ctx, kube.Key("default"), istio).Should(ReturnNotFoundError())
			},
		)
	})

	Describe("given Istio version", func() {
		for _, version := range istioversion.GetLatestPatchVersions() {
			Context(version.Name, func() {
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

					It("uses the correct image", func(ctx SpecContext) {
						Expect(common.GetObject(ctx, cl, kube.Key("istio-cni-node", istioCniNamespace), &appsv1.DaemonSet{})).
							To(common.HaveContainersThat(HaveEach(common.ImageFromRegistry(expectedRegistry))))
					})

					It("updates the status to Reconciled", func(ctx SpecContext) {
						common.AwaitCondition(ctx, v1.IstioCNIConditionReconciled, kube.Key(istioCniName), &v1.IstioCNI{}, k, cl)
					})

					It("updates the status to Ready", func(ctx SpecContext) {
						common.AwaitCondition(ctx, v1.IstioCNIConditionReady, kube.Key(istioCniName), &v1.IstioCNI{}, k, cl)
					})

					It("doesn't continuously reconcile the IstioCNI CR", func() {
						Eventually(k.WithNamespace(namespace).Logs).WithArguments("deploy/"+deploymentName, ptr.Of(30*time.Second)).
							ShouldNot(ContainSubstring("Reconciliation done"), "IstioCNI is continuously reconciling")
						Success("IstioCNI stopped reconciling")
					})
				})

				When("the Istio CR is created", func() {
					BeforeAll(func() {
						common.CreateIstio(k, version.Name)
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

					It("doesn't continuously reconcile the Istio CR", func() {
						Eventually(k.WithNamespace(namespace).Logs).WithArguments("deploy/"+deploymentName, ptr.Of(30*time.Second)).
							ShouldNot(ContainSubstring("Reconciliation done"), "Istio CR is continuously reconciling")
						Success("Istio CR stopped reconciling")
					})
				})

				When("sample pod is deployed", func() {
					BeforeAll(func(ctx SpecContext) {
						Expect(k.CreateNamespace(sampleNamespace)).To(Succeed(), "Sample namespace failed to be created")
						Expect(k.Label("namespace", sampleNamespace, "istio-injection", "enabled")).To(Succeed(), "Error labeling sample namespace")
						Expect(k.WithNamespace(sampleNamespace).
							ApplyKustomize("helloworld", "version=v1")).
							To(Succeed(), "Error deploying sample")
						Success("sample deployed")
					})

					samplePods := &corev1.PodList{}
					It("updates the pods status to Running", func(ctx SpecContext) {
						Eventually(common.CheckSamplePodsReady).WithArguments(ctx, cl).Should(Succeed(), "Error checking status of sample pods")
						Expect(cl.List(ctx, samplePods, client.InNamespace(sampleNamespace))).To(Succeed(), "Error getting the pods in sample namespace")

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
				common.LogDebugInfo(common.ControlPlane, k)
				debugInfoLogged = true
			}
		})
	})

	AfterAll(func() {
		if CurrentSpecReport().Failed() {
			if !debugInfoLogged {
				common.LogDebugInfo(common.ControlPlane, k)
				debugInfoLogged = true
			}
		}
	})
})

func getProxyVersion(podName, namespace string) (*semver.Version, error) {
	proxyStatus, err := istioctl.GetProxyStatus("--namespace " + namespace)
	if err != nil {
		return nil, fmt.Errorf("error getting sidecar version: %w", err)
	}

	lines := strings.Split(proxyStatus, "\n")
	colSplit := regexp.MustCompile(`\s{2,}`)

	versionIdx := -1
	headers := colSplit.Split(strings.TrimSpace(lines[0]), -1)
	for i, header := range headers {
		if header == "VERSION" {
			versionIdx = i
			break
		}
	}
	if versionIdx == -1 {
		return nil, fmt.Errorf("VERSION header not found")
	}

	var versionStr string
	for _, line := range lines[1:] {
		if strings.Contains(line, podName+"."+namespace) {
			values := colSplit.Split(strings.TrimSpace(line), -1)
			versionStr = values[versionIdx]
			break
		}
	}

	if versionStr == "" {
		return nil, fmt.Errorf("pod %s not found in proxy status output for namespace %s", podName, namespace)
	}
	version, err := semver.NewVersion(versionStr)
	if err != nil {
		return version, fmt.Errorf("error parsing sidecar version %q: %w", versionStr, err)
	}
	return version, err
}
