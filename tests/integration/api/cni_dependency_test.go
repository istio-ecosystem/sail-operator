//go:build integration

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

package integration

import (
	"context"
	"fmt"
	"time"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"istio.io/istio/pkg/ptr"
)

var _ = Describe("CNI Dependency", Ordered, func() {
	const (
		istioName         = "test-istio-cni"
		istioNamespace    = "istio-system"
		istioCNINamespace = "istio-cni"
	)

	var (
		ctx      = context.Background()
		istio    = &v1.Istio{}
		istiocni = &v1.IstioCNI{}
	)

	istioNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: istioNamespace,
		},
	}

	istiocniNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: istioCNINamespace,
		},
	}

	SetDefaultEventuallyTimeout(30 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)

	daemonsetKey := client.ObjectKey{Name: "istio-cni-node", Namespace: istioCNINamespace}

	BeforeAll(func() {
		Step("Creating required namespaces")
		Expect(k8sClient.Create(ctx, istioNS)).To(Succeed())
		Expect(k8sClient.Create(ctx, istiocniNS)).To(Succeed())
	})

	AfterAll(func() {
		// TODO(user): Attention if you improve this code by adding other context test you MUST
		// be aware of the current delete istioNS limitations.
		// More info: https://book.kubebuilder.io/reference/envtest.html#testing-considerations
		Step("Deleting the Namespaces")
		Expect(k8sClient.Delete(ctx, istioNS)).To(Succeed())
		Expect(k8sClient.Delete(ctx, istiocniNS)).To(Succeed())

		deleteAllIstiosAndRevisions(ctx)
	})

	Context("Non-OpenShift Platform", func() {
		When("CNI is enabled", func() {
			BeforeAll(func() {
				Step("Creating Istio resource with CNI enabled")
				istio = &v1.Istio{
					ObjectMeta: metav1.ObjectMeta{
						Name: istioName,
					},
					Spec: v1.IstioSpec{
						Version:   istioversion.Default,
						Namespace: istioNamespace,
						Values: &v1.Values{
							Pilot: &v1.PilotConfig{
								Cni: &v1.CNIUsageConfig{
									Enabled: ptr.Of(true),
								},
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, istio)).To(Succeed())
			})

			It("should show IstioCNINotFound condition", func() {
				Eventually(func(g Gomega) {
					err := k8sClient.Get(ctx, kube.Key(istio.Name, istio.Namespace), istio)
					g.Expect(err).NotTo(HaveOccurred())

					var depCondition *v1.IstioCondition
					for _, cond := range istio.Status.Conditions {
						if cond.Type == "DependenciesHealthy" {
							depCondition = &cond
							break
						}
					}
					g.Expect(depCondition).NotTo(BeNil())
					g.Expect(depCondition.Status).To(Equal(metav1.ConditionFalse))
					g.Expect(depCondition.Reason).To(Equal(v1.IstioReasonIstioCNINotFound))
				}).Should(Succeed())
			})

			AfterAll(func() {
				Step("Cleaning up Istio resource")
				deleteAllIstiosAndRevisions(ctx)
			})
		})

		When("CNI is disabled", func() {
			BeforeAll(func() {
				Step("Creating Istio resource with CNI disabled")
				istio = &v1.Istio{
					ObjectMeta: metav1.ObjectMeta{
						Name: istioName + "-no-cni",
					},
					Spec: v1.IstioSpec{
						Version:   istioversion.Default,
						Namespace: istioNamespace,
						Values:    &v1.Values{
							// not setting anything as CNI should be disabled by default on non-OCP platforms
						},
					},
				}
				Expect(k8sClient.Create(ctx, istio)).To(Succeed())
			})

			It("should show Healthy condition", func() {
				Eventually(func(g Gomega) {
					err := k8sClient.Get(ctx, kube.Key(istio.Name, istio.Namespace), istio)
					g.Expect(err).NotTo(HaveOccurred())

					var depCondition *v1.IstioCondition
					for _, cond := range istio.Status.Conditions {
						if cond.Type == "DependenciesHealthy" {
							depCondition = &cond
							break
						}
					}
					g.Expect(depCondition).NotTo(BeNil())
					g.Expect(depCondition.Status).To(Equal(metav1.ConditionTrue))
				}).Should(Succeed())
			})

			AfterAll(func() {
				Step("Cleaning up Istio resource")
				deleteAllIstiosAndRevisions(ctx)
			})
		})
	})

	Context("OpenShift Platform", func() {
		BeforeAll(func() {
			Step("Setting platform to OpenShift")
			istioReconciler.Config.Platform = config.PlatformOpenShift
			istioRevisionReconciler.Config.Platform = config.PlatformOpenShift
		})
		AfterAll(func() {
			Step("Setting platform back to Kubernetes")
			istioReconciler.Config.Platform = config.PlatformKubernetes
			istioRevisionReconciler.Config.Platform = config.PlatformKubernetes
		})
		When("CNI is enabled", func() {
			BeforeAll(func() {
				Step("Creating IstioCNI resource")
				istiocni = &v1.IstioCNI{
					ObjectMeta: metav1.ObjectMeta{
						Name: "default",
					},
					Spec: v1.IstioCNISpec{
						Version:   istioversion.Default,
						Namespace: istioCNINamespace,
					},
				}

				ds := &appsv1.DaemonSet{}
				Expect(k8sClient.Create(ctx, istiocni)).To(Succeed())
				Eventually(k8sClient.Get).WithArguments(ctx, daemonsetKey, ds).Should(Succeed())
				ds.Status.CurrentNumberScheduled = 1
				ds.Status.NumberReady = 1
				Expect(k8sClient.Status().Update(ctx, ds)).To(Succeed())

				Step("Creating Istio resource with CNI enabled")
				istio = &v1.Istio{
					ObjectMeta: metav1.ObjectMeta{
						Name: istioName,
					},
					Spec: v1.IstioSpec{
						Version:   istioversion.Default,
						Namespace: istioNamespace,
						Values:    &v1.Values{
							// we're not setting values as CNI should be enabled by default
						},
					},
				}
				Expect(k8sClient.Create(ctx, istio)).To(Succeed())
			})

			It("should show Healthy condition", func() {
				Eventually(func(g Gomega) {
					err := k8sClient.Get(ctx, kube.Key(istio.Name, istio.Namespace), istio)
					g.Expect(err).NotTo(HaveOccurred())

					var depCondition *v1.IstioCondition
					for _, cond := range istio.Status.Conditions {
						if cond.Type == "DependenciesHealthy" {
							depCondition = &cond
							break
						}
					}
					g.Expect(istio.Generation).To(Equal(istio.Status.ObservedGeneration))
					g.Expect(depCondition).NotTo(BeNil())
					g.Expect(depCondition.Status).To(Equal(metav1.ConditionTrue),
						fmt.Sprintf("Expected DependenciesHealthy condition to be True, got %v. Full conditions: %#v",
							depCondition.Status, istio.Status.Conditions))
				}).Should(Succeed())
			})

			AfterAll(func() {
				Step("Cleaning up Istio and IstioCNI resources")
				deleteAllIstioCNIs(ctx)
				deleteAllIstiosAndRevisions(ctx)
			})
		})

		When("CNI is explicitly disabled", func() {
			BeforeAll(func() {
				Step("Creating Istio resource with CNI explicitly disabled")
				istio = &v1.Istio{
					ObjectMeta: metav1.ObjectMeta{
						Name: istioName + "-ocp-no-cni",
					},
					Spec: v1.IstioSpec{
						Version:   istioversion.Default,
						Namespace: istioNamespace,
						Values: &v1.Values{
							Pilot: &v1.PilotConfig{
								Cni: &v1.CNIUsageConfig{
									Enabled: ptr.Of(false),
								},
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, istio)).To(Succeed())
			})

			It("should show Healthy condition", func() {
				Eventually(func(g Gomega) {
					err := k8sClient.Get(ctx, kube.Key(istio.Name, istio.Namespace), istio)
					g.Expect(err).NotTo(HaveOccurred())

					var depCondition *v1.IstioCondition
					for _, cond := range istio.Status.Conditions {
						if cond.Type == "DependenciesHealthy" {
							depCondition = &cond
							break
						}
					}
					g.Expect(depCondition).NotTo(BeNil())
					g.Expect(depCondition.Status).To(Equal(metav1.ConditionTrue))
				}).Should(Succeed())
			})

			AfterAll(func() {
				Step("Cleaning up Istio resource")
				deleteAllIstiosAndRevisions(ctx)
			})
		})

		When("CNI is not explicitly configured", func() {
			BeforeAll(func() {
				// Clean up any existing IstioCNI resources
				Expect(k8sClient.DeleteAllOf(ctx, &v1.IstioCNI{})).To(Succeed())
				Eventually(func(g Gomega) {
					list := &v1.IstioCNIList{}
					g.Expect(k8sClient.List(ctx, list)).To(Succeed())
					g.Expect(list.Items).To(BeEmpty())
				}).Should(Succeed())

				Step("Creating Istio resource with CNI not configured")
				istio = &v1.Istio{
					ObjectMeta: metav1.ObjectMeta{
						Name: istioName + "-ocp-default",
					},
					Spec: v1.IstioSpec{
						Version:   istioversion.Default,
						Namespace: istioNamespace,
						Values:    &v1.Values{
							// Not configuring CNI - should default to enabled on OpenShift
						},
					},
				}
				Expect(k8sClient.Create(ctx, istio)).To(Succeed())

				Step("Creating IstioCNI resource")
				istiocni = &v1.IstioCNI{
					ObjectMeta: metav1.ObjectMeta{
						Name: "default",
					},
					Spec: v1.IstioCNISpec{
						Version:   istioversion.Default,
						Namespace: istioCNINamespace,
					},
				}

				ds := &appsv1.DaemonSet{}
				Expect(k8sClient.Create(ctx, istiocni)).To(Succeed())
				Eventually(k8sClient.Get).WithArguments(ctx, daemonsetKey, ds).Should(Succeed())
				ds.Status.CurrentNumberScheduled = 1
				ds.Status.NumberReady = 1
				Expect(k8sClient.Status().Update(ctx, ds)).To(Succeed())
			})

			It("should show Healthy condition", func() {
				Eventually(func(g Gomega) {
					err := k8sClient.Get(ctx, kube.Key(istio.Name, istio.Namespace), istio)
					g.Expect(err).NotTo(HaveOccurred())

					var depCondition *v1.IstioCondition
					for _, cond := range istio.Status.Conditions {
						if cond.Type == "DependenciesHealthy" {
							depCondition = &cond
							break
						}
					}
					g.Expect(depCondition).NotTo(BeNil())
					g.Expect(depCondition.Status).To(Equal(metav1.ConditionTrue))
				}).Should(Succeed())
			})

			AfterAll(func() {
				Step("Cleaning up Istio resource")
				deleteAllIstioCNIs(ctx)
				deleteAllIstiosAndRevisions(ctx)
			})
		})
		When("CNI is not explicitly configured, not deployed", func() {
			BeforeAll(func() {
				// Clean up any existing IstioCNI resources
				Expect(k8sClient.DeleteAllOf(ctx, &v1.IstioCNI{})).To(Succeed())
				Eventually(func(g Gomega) {
					list := &v1.IstioCNIList{}
					g.Expect(k8sClient.List(ctx, list)).To(Succeed())
					g.Expect(list.Items).To(BeEmpty())
				}).Should(Succeed())

				Step("Creating Istio resource with CNI not configured")
				istio = &v1.Istio{
					ObjectMeta: metav1.ObjectMeta{
						Name: istioName + "-ocp-default",
					},
					Spec: v1.IstioSpec{
						Version:   istioversion.Default,
						Namespace: istioNamespace,
						Values:    &v1.Values{
							// Not configuring CNI - should default to enabled on OpenShift
						},
					},
				}
				Expect(k8sClient.Create(ctx, istio)).To(Succeed())
			})

			It("should not show Healthy condition", func() {
				Eventually(func(g Gomega) {
					err := k8sClient.Get(ctx, kube.Key(istio.Name, istio.Namespace), istio)
					g.Expect(err).NotTo(HaveOccurred())

					var depCondition *v1.IstioCondition
					for _, cond := range istio.Status.Conditions {
						if cond.Type == "DependenciesHealthy" {
							depCondition = &cond
							break
						}
					}
					g.Expect(depCondition).NotTo(BeNil())
					g.Expect(depCondition.Status).To(Equal(metav1.ConditionFalse))
				}).Should(Succeed())
			})

			AfterAll(func() {
				Step("Cleaning up Istio resource")
				deleteAllIstioCNIs(ctx)
				deleteAllIstiosAndRevisions(ctx)
			})
		})
	})
})
