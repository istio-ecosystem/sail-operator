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

package controlplane

import (
	"context"
	"fmt"
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
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("NetworkPolicy", Label("networkpolicy", "slow"), Ordered, func() {
	SetDefaultEventuallyTimeout(180 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)
	debugInfoLogged := false

	const (
		networkPolicyName = "istio-istiod"
	)

	Describe("NetworkPolicy creation and lifecycle", func() {
		for _, version := range istioversion.GetLatestPatchVersions() {
			Context(version.Name, func() {
				clr := cleaner.New(cl)

				BeforeAll(func(ctx SpecContext) {
					clr.Record(ctx)
					Expect(k.CreateNamespace(controlPlaneNamespace)).To(Succeed(), "Istio namespace failed to be created")
					Expect(k.CreateNamespace(istioCniNamespace)).To(Succeed(), "IstioCNI namespace failed to be created")

					common.CreateIstioCNI(k, version.Name)
					Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(istioCniName), &v1.IstioCNI{}).
						Should(HaveConditionStatus(v1.IstioCNIConditionReady, metav1.ConditionTrue), "IstioCNI is not Ready; unexpected Condition")
					Success("IstioCNI is Ready")
				})

				When("the Istio CR is created without createNetworkPolicy field", func() {
					BeforeAll(func() {
						common.CreateIstio(k, version.Name)
					})

					AfterAll(func() {
						Expect(k.Delete("istio", istioName)).To(Succeed(), "Istio CR failed to be deleted")
						Eventually(cl.Get).WithArguments(context.Background(), kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(ReturnNotFoundError(), "Istiod should be deleted")
						Success("Istio cleanup successful")
					})

					It("deploys istiod successfully", func(ctx SpecContext) {
						Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(istioName), &v1.Istio{}).
							Should(HaveConditionStatus(v1.IstioConditionReady, metav1.ConditionTrue), "Istio is not Ready; unexpected Condition")
						Success("Istio is Ready")

						Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(HaveConditionStatus(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Istiod is not Available; unexpected Condition")
						Success("Istiod is deployed and Available")
					})

					It("does not create a NetworkPolicy", func(ctx SpecContext) {
						networkPolicy := &networkingv1.NetworkPolicy{}
						err := cl.Get(ctx, kube.Key(networkPolicyName, controlPlaneNamespace), networkPolicy)
						Expect(apierrors.IsNotFound(err)).To(BeTrue(), "NetworkPolicy should not exist when createNetworkPolicy is not set")
						Success("NetworkPolicy correctly not created when flag not set")
					})
				})

				When("the Istio CR is created with createNetworkPolicy set to false", func() {
					BeforeAll(func() {
						istioYAML := fmt.Sprintf(`
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: %s
spec:
  version: %s
  namespace: %s
  createNetworkPolicy: false`, istioName, version.Name, controlPlaneNamespace)
						Log("Istio YAML:", common.Indent(istioYAML))
						Expect(k.CreateFromString(istioYAML)).To(Succeed(), "Istio CR failed to be created")
						Success("Istio CR created with NetworkPolicy flag set to false")
					})

					AfterAll(func() {
						Expect(k.Delete("istio", istioName)).To(Succeed(), "Istio CR failed to be deleted")
						Eventually(cl.Get).WithArguments(context.Background(), kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(ReturnNotFoundError(), "Istiod should be deleted")
						Success("Istio cleanup successful")
					})

					It("deploys istiod successfully", func(ctx SpecContext) {
						Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(istioName), &v1.Istio{}).
							Should(HaveConditionStatus(v1.IstioConditionReady, metav1.ConditionTrue), "Istio is not Ready; unexpected Condition")
						Success("Istio is Ready")
					})

					It("does not create a NetworkPolicy", func(ctx SpecContext) {
						networkPolicy := &networkingv1.NetworkPolicy{}
						err := cl.Get(ctx, kube.Key(networkPolicyName, controlPlaneNamespace), networkPolicy)
						Expect(apierrors.IsNotFound(err)).To(BeTrue(), "NetworkPolicy should not exist when createNetworkPolicy is false")
						Success("NetworkPolicy correctly not created when flag is false")
					})
				})

				When("the Istio CR is created with createNetworkPolicy set to true", func() {
					BeforeAll(func() {
						istioYAML := fmt.Sprintf(`
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: %s
spec:
  version: %s
  namespace: %s
  createNetworkPolicy: true`, istioName, version.Name, controlPlaneNamespace)
						Log("Istio YAML:", common.Indent(istioYAML))
						Expect(k.CreateFromString(istioYAML)).To(Succeed(), "Istio CR failed to be created")
						Success("Istio CR created with NetworkPolicy flag set to true")
					})

					It("deploys istiod successfully", func(ctx SpecContext) {
						Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(istioName), &v1.Istio{}).
							Should(HaveConditionStatus(v1.IstioConditionReady, metav1.ConditionTrue), "Istio is not Ready; unexpected Condition")
						Success("Istio is Ready")

						Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(HaveConditionStatus(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Istiod is not Available; unexpected Condition")
						Success("Istiod is deployed and Available")
					})

					It("creates the correct NetworkPolicy", func(ctx SpecContext) {
						networkPolicy := &networkingv1.NetworkPolicy{}
						Eventually(func() error {
							return cl.Get(ctx, kube.Key(networkPolicyName, controlPlaneNamespace), networkPolicy)
						}).Should(Succeed(), "NetworkPolicy should be created")
						Success("NetworkPolicy created successfully")

						// Verify NetworkPolicy has correct metadata
						Expect(networkPolicy.Name).To(Equal(networkPolicyName), "NetworkPolicy name should be correct")
						Expect(networkPolicy.Namespace).To(Equal(controlPlaneNamespace), "NetworkPolicy namespace should be correct")

						// Verify NetworkPolicy has owner reference to IstioRevision
						Expect(networkPolicy.OwnerReferences).ToNot(BeEmpty(), "NetworkPolicy should have owner references")
						hasIstioRevisionOwner := false
						for _, owner := range networkPolicy.OwnerReferences {
							if owner.Kind == "IstioRevision" {
								hasIstioRevisionOwner = true
								break
							}
						}
						Expect(hasIstioRevisionOwner).To(BeTrue(), "NetworkPolicy should have IstioRevision as owner")

						// Verify pod selector
						Expect(networkPolicy.Spec.PodSelector.MatchLabels).To(HaveKeyWithValue("app", "istiod"), "Pod selector should match istiod app")

						// Verify ingress rules - should have 3 rules
						Expect(networkPolicy.Spec.Ingress).To(HaveLen(3), "Should have 3 ingress rules")

						// Check webhook ingress rule (port 15017)
						webhookRule := networkPolicy.Spec.Ingress[0]
						Expect(webhookRule.Ports).To(HaveLen(1), "Webhook rule should have 1 port")
						Expect(webhookRule.Ports[0].Port.IntVal).To(Equal(int32(15017)), "Webhook port should be 15017")
						Expect(webhookRule.From).To(HaveLen(1), "Webhook rule should have 1 from rule")
						Expect(webhookRule.From[0].NamespaceSelector.MatchLabels).To(
							HaveKeyWithValue("kubernetes.io/metadata.name", "kube-system"),
							"Webhook rule should allow from kube-system namespace")

						// Check xDS ingress rule (ports 15010, 15011, 15012, 8080, 15014)
						xdsRule := networkPolicy.Spec.Ingress[1]
						Expect(xdsRule.Ports).To(HaveLen(5), "xDS rule should have 5 ports")
						expectedPorts := []int32{15010, 15011, 15012, 8080, 15014}
						actualPorts := make([]int32, len(xdsRule.Ports))
						for i, port := range xdsRule.Ports {
							actualPorts[i] = port.Port.IntVal
						}
						Expect(actualPorts).To(ConsistOf(expectedPorts), "xDS ports should be correct")

						// Check Kiali ingress rule (ports 8080, 15014)
						kialiRule := networkPolicy.Spec.Ingress[2]
						Expect(kialiRule.Ports).To(HaveLen(2), "Kiali rule should have 2 ports")
						expectedKialiPorts := []int32{8080, 15014}
						actualKialiPorts := make([]int32, len(kialiRule.Ports))
						for i, port := range kialiRule.Ports {
							actualKialiPorts[i] = port.Port.IntVal
						}
						Expect(actualKialiPorts).To(ConsistOf(expectedKialiPorts), "Kiali ports should be correct")
						Expect(kialiRule.From).To(HaveLen(1), "Kiali rule should have 1 from rule")
						Expect(kialiRule.From[0].PodSelector).ToNot(BeNil(), "Kiali rule should have pod selector")
						Expect(kialiRule.From[0].PodSelector.MatchLabels).To(HaveKeyWithValue("app.kubernetes.io/name", "kiali"), "Kiali rule should match kiali pods")

						Success("NetworkPolicy has correct configuration")
					})

					When("createNetworkPolicy is updated to false", func() {
						BeforeAll(func() {
							Expect(k.Patch("istio", istioName, "merge", `{"spec":{"createNetworkPolicy":false}}`)).
								To(Succeed(), "Failed to update Istio CR to disable NetworkPolicy")
							Success("Istio CR updated to disable NetworkPolicy")
						})

						It("removes the NetworkPolicy", func(ctx SpecContext) {
							networkPolicy := &networkingv1.NetworkPolicy{}
							Eventually(cl.Get).WithArguments(ctx, kube.Key(networkPolicyName, controlPlaneNamespace), networkPolicy).
								Should(ReturnNotFoundError(), "NetworkPolicy should be deleted when flag is set to false")
							Success("NetworkPolicy successfully removed when flag disabled")
						})

						It("istiod continues to run normally", func(ctx SpecContext) {
							Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{}).
								Should(HaveConditionStatus(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Istiod should still be Available; unexpected Condition")
							Success("Istiod continues to run after NetworkPolicy removal")
						})

						When("createNetworkPolicy is updated back to true", func() {
							BeforeAll(func() {
								Expect(k.Patch("istio", istioName, "merge", `{"spec":{"createNetworkPolicy":true}}`)).
									To(Succeed(), "Failed to update Istio CR to re-enable NetworkPolicy")
								Success("Istio CR updated to re-enable NetworkPolicy")
							})

							It("recreates the NetworkPolicy", func(ctx SpecContext) {
								networkPolicy := &networkingv1.NetworkPolicy{}
								Eventually(func() error {
									return cl.Get(ctx, kube.Key(networkPolicyName, controlPlaneNamespace), networkPolicy)
								}).Should(Succeed(), "NetworkPolicy should be recreated when flag is enabled again")
								Success("NetworkPolicy successfully recreated when flag re-enabled")

								// Basic verification that it has the correct structure
								Expect(networkPolicy.Spec.PodSelector.MatchLabels).To(HaveKeyWithValue("app", "istiod"), "Recreated NetworkPolicy should have correct pod selector")
								Expect(networkPolicy.Spec.Ingress).To(HaveLen(3), "Recreated NetworkPolicy should have correct ingress rules")
								Success("Recreated NetworkPolicy has correct configuration")
							})
						})
					})

					AfterAll(func() {
						Expect(k.Delete("istio", istioName)).To(Succeed(), "Istio CR failed to be deleted")
						Eventually(cl.Get).WithArguments(context.Background(), kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(ReturnNotFoundError(), "Istiod should be deleted")

						networkPolicy := &networkingv1.NetworkPolicy{}
						Eventually(cl.Get).WithArguments(context.Background(), kube.Key(networkPolicyName, controlPlaneNamespace), networkPolicy).
							Should(ReturnNotFoundError(), "NetworkPolicy should be deleted")

						Success("Istio cleanup successful")
					})
				})

				AfterAll(func(ctx SpecContext) {
					if CurrentSpecReport().Failed() {
						common.LogDebugInfo(common.ControlPlane, k)
						debugInfoLogged = true
						if keepOnFailure {
							return
						}
					}

					clr.Cleanup(ctx)
				})
			})
		}

		AfterAll(func(ctx SpecContext) {
			if CurrentSpecReport().Failed() {
				if !debugInfoLogged {
					common.LogDebugInfo(common.ControlPlane, k)
					debugInfoLogged = true
				}
			}
		})
	})
})
