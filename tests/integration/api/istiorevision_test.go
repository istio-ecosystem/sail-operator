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
	"net/http"
	"time"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/enqueuelogger"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversions"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/common/expfmt"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"istio.io/istio/pkg/ptr"
)

var _ = Describe("IstioRevision resource", Ordered, func() {
	const (
		revName        = "test-istiorevision"
		istioNamespace = "istiorevision-test"

		pilotImage = "sail-operator/test:latest"
	)

	SetDefaultEventuallyPollingInterval(time.Second)
	SetDefaultEventuallyTimeout(30 * time.Second)

	enqueuelogger.LogEnqueueEvents = true

	ctx := context.Background()

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: istioNamespace,
		},
	}

	revKey := client.ObjectKey{Name: revName}
	istiodKey := client.ObjectKey{Name: "istiod-" + revName, Namespace: istioNamespace}

	BeforeAll(func() {
		Step("Creating the Namespace to perform the tests")
		Expect(k8sClient.Create(ctx, namespace)).To(Succeed())
	})

	AfterAll(func() {
		// TODO(user): Attention if you improve this code by adding other context test you MUST
		// be aware of the current delete namespace limitations. More info: https://book.kubebuilder.io/reference/envtest.html#testing-considerations
		Step("Deleting the Namespace to perform the tests")
		Expect(k8sClient.Delete(ctx, namespace)).To(Succeed())

		Eventually(k8sClient.DeleteAllOf).WithArguments(ctx, &v1.IstioRevision{}).Should(Succeed())
		Eventually(func(g Gomega) {
			list := &v1.IstioRevisionList{}
			g.Expect(k8sClient.List(ctx, list)).To(Succeed())
			g.Expect(list.Items).To(BeEmpty())
		}).Should(Succeed())
	})

	rev := &v1.IstioRevision{}

	Describe("validation", func() {
		AfterEach(func() {
			Eventually(k8sClient.DeleteAllOf).WithArguments(ctx, &v1.IstioRevision{}).Should(Succeed())
		})

		It("rejects an IstioRevision where spec.values.global.istioNamespace doesn't match spec.namespace", func() {
			rev = &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: revName,
				},
				Spec: v1.IstioRevisionSpec{
					Version:   istioversions.Default,
					Namespace: istioNamespace,
					Values: &v1.Values{
						Revision: ptr.Of(revName),
						Global: &v1.GlobalConfig{
							IstioNamespace: ptr.Of("wrong-namespace"),
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, rev)).To(Not(Succeed()))
		})

		It("rejects an IstioRevision where spec.values.revision doesn't match metadata.name (when name is not default)", func() {
			rev = &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: revName,
				},
				Spec: v1.IstioRevisionSpec{
					Version:   istioversions.Default,
					Namespace: istioNamespace,
					Values: &v1.Values{
						Revision: ptr.Of("is-not-" + revName),
						Global: &v1.GlobalConfig{
							IstioNamespace: ptr.Of(istioNamespace),
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, rev)).To(Not(Succeed()))
		})

		It("rejects an IstioRevision where metadata.name is default and spec.values.revision isn't empty", func() {
			rev = &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1.IstioRevisionSpec{
					Version:   istioversions.Default,
					Namespace: istioNamespace,
					Values: &v1.Values{
						Revision: ptr.Of("default"), // this must be rejected, because revision needs to be '' when metadata.name is 'default'
						Global: &v1.GlobalConfig{
							IstioNamespace: ptr.Of(istioNamespace),
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, rev)).To(Not(Succeed()))
		})

		It("accepts an IstioRevision where metadata.name is default and spec.values.revision is empty", func() {
			rev = &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1.IstioRevisionSpec{
					Version:   istioversions.Default,
					Namespace: istioNamespace,
					Values: &v1.Values{
						Revision: ptr.Of(""),
						Global: &v1.GlobalConfig{
							IstioNamespace: ptr.Of(istioNamespace),
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, rev)).To(Succeed())
		})
	})

	Describe("reconciles immediately after target namespace is created", func() {
		nsName := "nonexistent-namespace-" + rand.String(8)
		BeforeAll(func() {
			Step("Creating the IstioRevision resource without the namespace")
			rev = &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: revName,
				},
				Spec: v1.IstioRevisionSpec{
					Version:   istioversions.Default,
					Namespace: nsName,
					Values: &v1.Values{
						Revision: ptr.Of(revName),
						Global: &v1.GlobalConfig{
							IstioNamespace: &nsName,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, rev)).To(Succeed())
		})

		AfterAll(func() {
			Expect(k8sClient.Delete(ctx, rev)).To(Succeed())
			Eventually(k8sClient.Get).WithArguments(ctx, kube.Key(revName), rev).Should(ReturnNotFoundError())
		})

		It("indicates in the status that the namespace doesn't exist", func() {
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, revKey, rev)).To(Succeed())
				g.Expect(rev.Status.ObservedGeneration).To(Equal(rev.ObjectMeta.Generation))

				reconciled := rev.Status.GetCondition(v1.IstioRevisionConditionReconciled)
				g.Expect(reconciled.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(reconciled.Reason).To(Equal(v1.IstioRevisionReasonReconcileError))
				g.Expect(reconciled.Message).To(ContainSubstring(fmt.Sprintf("namespace %q doesn't exist", nsName)))
			}).Should(Succeed())
		})

		When("the namespace is created", func() {
			var ns *corev1.Namespace

			BeforeAll(func() {
				ns = &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: nsName,
					},
				}
				Expect(k8sClient.Create(ctx, ns)).To(Succeed())
			})

			It("reconciles immediately", func() {
				Step("Checking if istiod is deployed immediately")
				istiod := &appsv1.Deployment{}
				istiodKey := client.ObjectKey{Name: "istiod-" + revName, Namespace: ns.Name}
				Eventually(k8sClient.Get).WithArguments(ctx, istiodKey, istiod).WithTimeout(10 * time.Second).Should(Succeed())

				Step("Checking if the status is updated")
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, revKey, rev)).To(Succeed())
					g.Expect(rev.Status.ObservedGeneration).To(Equal(rev.ObjectMeta.Generation))
					reconciled := rev.Status.GetCondition(v1.IstioRevisionConditionReconciled)
					g.Expect(reconciled.Status).To(Equal(metav1.ConditionTrue))
				}).Should(Succeed())
			})
		})
	})

	It("successfully reconciles the resource", func() {
		Step("Creating the IstioRevision")
		rev = &v1.IstioRevision{
			ObjectMeta: metav1.ObjectMeta{
				Name: revName,
			},
			Spec: v1.IstioRevisionSpec{
				Version:   istioversions.Default,
				Namespace: istioNamespace,
				Values: &v1.Values{
					Global: &v1.GlobalConfig{
						IstioNamespace: ptr.Of(istioNamespace),
					},
					Revision: ptr.Of(revName),
					Pilot: &v1.PilotConfig{
						Image: ptr.Of(pilotImage),
					},
				},
			},
		}

		Expect(k8sClient.Create(ctx, rev)).To(Succeed())

		Step("Checking if the resource was successfully created")
		Eventually(k8sClient.Get).WithArguments(ctx, revKey, rev).Should(Succeed())

		istiod := &appsv1.Deployment{}
		Step("Checking if Deployment was successfully created in the reconciliation")
		Eventually(k8sClient.Get).WithArguments(ctx, istiodKey, istiod).Should(Succeed())
		Expect(istiod.Spec.Template.Spec.Containers[0].Image).To(Equal(pilotImage))
		Expect(istiod.ObjectMeta.OwnerReferences).To(ContainElement(NewOwnerReference(rev)))

		Step("Checking if the status is updated")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, revKey, rev)).To(Succeed())
			g.Expect(rev.Status.ObservedGeneration).To(Equal(rev.ObjectMeta.Generation))
		}).Should(Succeed())
	})

	When("istiod readiness changes", func() {
		It("updates the status of the IstioRevision resource", func() {
			By("setting the Ready condition status to true when istiod is ready", func() {
				Expect(k8sClient.Get(ctx, revKey, rev)).To(Succeed())
				readyCondition := rev.Status.GetCondition(v1.IstioRevisionConditionReady)
				Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))

				istiod := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, istiodKey, istiod)).To(Succeed())
				istiod.Status.Replicas = 1
				istiod.Status.ReadyReplicas = 1
				Expect(k8sClient.Status().Update(ctx, istiod)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, revKey, rev)).To(Succeed())
					readyCondition := rev.Status.GetCondition(v1.IstioRevisionConditionReady)
					g.Expect(readyCondition.Status).To(Equal(metav1.ConditionTrue))
				}).Should(Succeed())
			})

			By("setting the Ready condition status to false when istiod isn't ready", func() {
				istiod := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, istiodKey, istiod)).To(Succeed())

				istiod.Status.ReadyReplicas = 0
				Expect(k8sClient.Status().Update(ctx, istiod)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, revKey, rev)).To(Succeed())
					readyCondition := rev.Status.GetCondition(v1.IstioRevisionConditionReady)
					g.Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
				}).Should(Succeed())
			})
		})
	})

	DescribeTable("reconciles owned resource",
		func(obj client.Object, modify func(obj client.Object), validate func(g Gomega, obj client.Object)) {
			By("on update", func() {
				// ensure all in-flight reconcile operations finish before the test
				waitForInFlightReconcileToFinish()

				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())

				modify(obj)
				Expect(k8sClient.Update(ctx, obj)).To(Succeed())

				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
					validate(g, obj)
				}).Should(Succeed())
			})

			By("on delete", func() {
				Expect(k8sClient.Delete(ctx, obj)).To(Succeed())
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
					g.Expect(obj.GetOwnerReferences()).To(ContainElement(NewOwnerReference(rev)))
					validate(g, obj)
				}).Should(Succeed())
			})
		},
		Entry("Deployment",
			&appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      istiodKey.Name,
					Namespace: istiodKey.Namespace,
				},
			}, func(obj client.Object) {
				deployment := obj.(*appsv1.Deployment)
				deployment.Spec.Template.Spec.Containers[0].Image = "xyz"
			}, func(g Gomega, obj client.Object) {
				deployment := obj.(*appsv1.Deployment)
				g.Expect(deployment.Spec.Template.Spec.Containers[0].Image).ToNot(Equal("xyz"))
			}),
		Entry("MutatingWebhookConfiguration",
			&admissionv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "istio-sidecar-injector-" + revName + "-" + istioNamespace,
				},
			}, func(obj client.Object) {
				webhook := obj.(*admissionv1.MutatingWebhookConfiguration)
				webhook.Webhooks[0].Name = "xyz.xyz.xyz"
			}, func(g Gomega, obj client.Object) {
				webhook := obj.(*admissionv1.MutatingWebhookConfiguration)
				g.Expect(webhook.Webhooks[0].Name).ToNot(Equal("xyz.xyz.xyz"))
			}),
		Entry("HorizontalPodAutoscaler",
			&autoscalingv2.HorizontalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "istiod-" + revName,
					Namespace: istioNamespace,
				},
			}, func(obj client.Object) {
				hpa := obj.(*autoscalingv2.HorizontalPodAutoscaler)
				hpa.Spec.MaxReplicas = 123
			}, func(g Gomega, obj client.Object) {
				hpa := obj.(*autoscalingv2.HorizontalPodAutoscaler)
				g.Expect(hpa.Spec.MaxReplicas).ToNot(Equal(int32(123)))
			}),
	)

	DescribeTable("skips reconcile when only the status of the owned resource is updated",
		func(obj client.Object, modify func(obj client.Object)) {
			waitForInFlightReconcileToFinish()

			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())

			beforeCount := getIstioRevisionReconcileCount(Default)

			By("modifying object")
			modify(obj)
			Expect(k8sClient.Status().Update(ctx, obj)).To(Succeed())

			Consistently(func(g Gomega) {
				afterCount := getIstioRevisionReconcileCount(g)
				g.Expect(afterCount).To(Equal(beforeCount))
			}, 5*time.Second).Should(Succeed())
		},
		Entry("HorizontalPodAutoscaler",
			&autoscalingv2.HorizontalPodAutoscaler{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "istiod-" + revName,
					Namespace: istioNamespace,
				},
			},
			func(obj client.Object) {
				hpa := obj.(*autoscalingv2.HorizontalPodAutoscaler)
				hpa.Status.CurrentReplicas = 123
			},
		),
		Entry("PodDisruptionBudget",
			&policyv1.PodDisruptionBudget{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "istiod-" + revName,
					Namespace: istioNamespace,
				},
			},
			func(obj client.Object) {
				pdb := obj.(*policyv1.PodDisruptionBudget)
				pdb.Status.CurrentHealthy = 123
			},
		),
	)

	It("skips reconcile when a pull secret is added to service account", func() {
		waitForInFlightReconcileToFinish()

		sa := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "istiod-" + revName,
				Namespace: istioNamespace,
			},
		}
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(sa), sa)).To(Succeed())

		GinkgoWriter.Println("sa:", sa)

		beforeCount := getIstioRevisionReconcileCount(Default)

		By("adding pull secret to ServiceAccount")
		sa.ImagePullSecrets = append(sa.ImagePullSecrets, corev1.LocalObjectReference{Name: "other-pull-secret"})
		Expect(k8sClient.Update(ctx, sa)).To(Succeed())

		Consistently(func(g Gomega) {
			afterCount := getIstioRevisionReconcileCount(g)
			g.Expect(afterCount).To(Equal(beforeCount))
		}, 5*time.Second).Should(Succeed(), "IstioRevision was reconciled when it shouldn't have been")

		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(sa), sa)).To(Succeed())
		Expect(sa.ImagePullSecrets).To(ContainElement(corev1.LocalObjectReference{Name: "other-pull-secret"}))
	})

	It("supports concurrent deployment of two control planes", func() {
		rev2Name := revName + "2"
		rev2Key := client.ObjectKey{Name: rev2Name}
		istiod2Key := client.ObjectKey{Name: "istiod-" + rev2Name, Namespace: istioNamespace}

		Step("Creating the second IstioRevision instance")
		rev2 := &v1.IstioRevision{
			ObjectMeta: metav1.ObjectMeta{
				Name: rev2Key.Name,
			},
			Spec: v1.IstioRevisionSpec{
				Version:   istioversions.Default,
				Namespace: istioNamespace,
				Values: &v1.Values{
					Global: &v1.GlobalConfig{
						IstioNamespace: ptr.Of(istioNamespace),
					},
					Revision: &rev2Key.Name,
					Pilot: &v1.PilotConfig{
						Image: ptr.Of(pilotImage),
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, rev2)).To(Succeed())

		Step("Checking if the resource was successfully created")
		Eventually(k8sClient.Get).WithArguments(ctx, rev2Key, rev2).Should(Succeed())

		Step("Checking if the status is updated")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, rev2Key, rev2)).To(Succeed())
			g.Expect(rev2.Status.ObservedGeneration).To(Equal(rev2.ObjectMeta.Generation))
		}).Should(Succeed())

		Step("Checking if Deployment was successfully created in the reconciliation")
		istiod := &appsv1.Deployment{}
		Eventually(k8sClient.Get).WithArguments(ctx, istiod2Key, istiod).Should(Succeed())
		Expect(istiod.Spec.Template.Spec.Containers[0].Image).To(Equal(pilotImage))
		Expect(istiod.ObjectMeta.OwnerReferences).To(ContainElement(NewOwnerReference(rev2)))
	})
})

func waitForInFlightReconcileToFinish() {
	// wait for the in-flight reconcile operations to finish
	// unfortunately, I don't see a good way to do this other than by waiting
	time.Sleep(5 * time.Second)
}

func getIstioRevisionReconcileCount(g Gomega) float64 {
	return getReconcileCount(g, "istiorevision")
}

func getIstioCNIReconcileCount(g Gomega) float64 {
	return getReconcileCount(g, "istiocni")
}

func getReconcileCount(g Gomega, controllerName string) float64 {
	resp, err := http.Get("http://localhost:8080/metrics")
	g.Expect(err).NotTo(HaveOccurred())
	defer resp.Body.Close()

	parser := expfmt.TextParser{}
	metricFamilies, err := parser.TextToMetricFamilies(resp.Body)
	g.Expect(err).NotTo(HaveOccurred())

	metricName := "controller_runtime_reconcile_total"
	mf := metricFamilies[metricName]
	sum := float64(0)
	for _, metric := range mf.Metric {
		for _, l := range metric.Label {
			if *l.Name == "controller" && *l.Value == controllerName {
				sum += metric.GetCounter().GetValue()
			}
		}
	}
	return sum
}
