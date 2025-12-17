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
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	"github.com/istio-ecosystem/sail-operator/pkg/enqueuelogger"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"istio.io/istio/pkg/ptr"
)

var _ = Describe("IstioRevision resource", Label("istiorevision"), Ordered, func() {
	const (
		pilotImage = "sail-operator/test:latest"
	)

	SetDefaultEventuallyPollingInterval(time.Second)
	SetDefaultEventuallyTimeout(30 * time.Second)

	enqueuelogger.LogEnqueueEvents = true

	ctx := context.Background()

	istioNamespace := "istiorevision-test"
	revName := "test-istiorevision"
	revKey := client.ObjectKey{Name: revName}
	defaultKey := client.ObjectKey{Name: "default"}
	istiodKey := client.ObjectKey{Name: "istiod-" + revName, Namespace: istioNamespace}

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: istioNamespace,
		},
	}
	BeforeAll(func() {
		Step("Creating the Namespace to perform the tests")
		Expect(k8sClient.Create(ctx, namespace)).To(Succeed())
	})

	AfterAll(func() {
		// TODO(user): Attention if you improve this code by adding other context test you MUST
		// be aware of the current delete namespace limitations. More info: https://book.kubebuilder.io/reference/envtest.html#testing-considerations
		Step("Deleting the Namespace to perform the tests")
		Expect(k8sClient.Delete(ctx, namespace)).To(Succeed())

		deleteAllIstioRevisions(ctx)
	})

	rev := &v1.IstioRevision{}
	tag := &v1.IstioRevisionTag{}

	Describe("validation", func() {
		AfterEach(func() {
			deleteAllIstioRevisions(ctx)
		})

		It("rejects an IstioRevision where spec.values.global.istioNamespace doesn't match spec.namespace", func() {
			rev = &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: revName,
				},
				Spec: v1.IstioRevisionSpec{
					Version:   istioversion.Default,
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
					Version:   istioversion.Default,
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
					Version:   istioversion.Default,
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
					Version:   istioversion.Default,
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

	Describe("IstioCNI dependency checks", func() {
		cni := &v1.IstioCNI{
			ObjectMeta: metav1.ObjectMeta{
				Name: cniName,
			},
			Spec: v1.IstioCNISpec{
				Version:   istioversion.Default,
				Namespace: istioNamespace,
			},
		}

		BeforeAll(func() {
			rev = &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: revName,
				},
				Spec: v1.IstioRevisionSpec{
					Version:   istioversion.Default,
					Namespace: istioNamespace,
					Values: &v1.Values{
						Revision: ptr.Of(revName),
						Global: &v1.GlobalConfig{
							IstioNamespace: ptr.Of(istioNamespace),
							Platform:       ptr.Of(string(config.PlatformOpenShift)),
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, rev)).To(Succeed())
		})

		AfterAll(func() {
			deleteAllIstioRevisions(ctx)

			Expect(k8sClient.Delete(ctx, cni)).To(Succeed())
			Eventually(k8sClient.Get).WithArguments(ctx, cniKey, cni).Should(ReturnNotFoundError())
		})

		It("shows dependencies unhealthy when IstioCNI is missing", func() {
			expectCondition(ctx, revName, v1.IstioRevisionConditionDependenciesHealthy, metav1.ConditionFalse,
				func(g Gomega, condition *v1.IstioRevisionCondition) {
					g.Expect(condition.Reason).To(Equal(v1.IstioRevisionReasonIstioCNINotFound))
				})
		})

		It("shows dependencies unhealthy when IstioCNI is unhealthy", func() {
			Expect(k8sClient.Create(ctx, cni)).To(Succeed())
			expectCondition(ctx, revName, v1.IstioRevisionConditionDependenciesHealthy, metav1.ConditionFalse,
				func(g Gomega, condition *v1.IstioRevisionCondition) {
					g.Expect(condition.Reason).To(Equal(v1.IstioRevisionReasonIstioCNINotHealthy))
				})
		})

		It("shows dependencies healthy when IstioCNI is healthy", func() {
			dsKey := client.ObjectKey{Name: "istio-cni-node", Namespace: cni.Spec.Namespace}
			ds := &appsv1.DaemonSet{}
			Expect(k8sClient.Get(ctx, dsKey, ds)).To(Succeed())
			ds.Status.CurrentNumberScheduled = 3
			ds.Status.NumberReady = 3
			Expect(k8sClient.Status().Update(ctx, ds)).To(Succeed())

			expectCNICondition(ctx, v1.IstioCNIConditionReady, metav1.ConditionTrue)
			expectCondition(ctx, revName, v1.IstioRevisionConditionDependenciesHealthy, metav1.ConditionTrue)
		})
	})

	Describe("ZTunnel dependency checks", func() {
		ztunnelName := "default"
		ztunnelKey := client.ObjectKey{Name: ztunnelName}
		ztunnel := &v1.ZTunnel{
			ObjectMeta: metav1.ObjectMeta{
				Name: ztunnelName,
			},
			Spec: v1.ZTunnelSpec{
				Version:   istioversion.Default,
				Namespace: istioNamespace,
			},
		}

		// Create IstioCNI as ambient mode requires it
		cni := &v1.IstioCNI{
			ObjectMeta: metav1.ObjectMeta{
				Name: cniName,
			},
			Spec: v1.IstioCNISpec{
				Version:   istioversion.Default,
				Namespace: istioNamespace,
			},
		}

		BeforeAll(func() {
			Expect(k8sClient.Create(ctx, cni)).To(Succeed())
			// Wait for CNI DaemonSet to be created
			dsKey := client.ObjectKey{Name: "istio-cni-node", Namespace: istioNamespace}
			ds := &appsv1.DaemonSet{}
			Eventually(k8sClient.Get).WithArguments(ctx, dsKey, ds).Should(Succeed())
			// Make CNI healthy
			ds.Status.CurrentNumberScheduled = 3
			ds.Status.NumberReady = 3
			Expect(k8sClient.Status().Update(ctx, ds)).To(Succeed())
			expectCNICondition(ctx, v1.IstioCNIConditionReady, metav1.ConditionTrue)

			rev = &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: revName,
				},
				Spec: v1.IstioRevisionSpec{
					Version:   istioversion.Default,
					Namespace: istioNamespace,
					Values: &v1.Values{
						Revision: ptr.Of(revName),
						Profile:  ptr.Of("ambient"),
						Global: &v1.GlobalConfig{
							IstioNamespace: ptr.Of(istioNamespace),
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, rev)).To(Succeed())
		})

		AfterAll(func() {
			deleteAllIstioRevisions(ctx)

			Expect(k8sClient.Delete(ctx, ztunnel)).To(Succeed())
			Eventually(k8sClient.Get).WithArguments(ctx, ztunnelKey, ztunnel).Should(ReturnNotFoundError())

			Expect(k8sClient.Delete(ctx, cni)).To(Succeed())
			Eventually(k8sClient.Get).WithArguments(ctx, cniKey, cni).Should(ReturnNotFoundError())
		})

		It("shows dependencies unhealthy when ZTunnel is missing", func() {
			expectCondition(ctx, revName, v1.IstioRevisionConditionDependenciesHealthy, metav1.ConditionFalse,
				func(g Gomega, condition *v1.IstioRevisionCondition) {
					g.Expect(condition.Reason).To(Equal(v1.IstioRevisionReasonZTunnelNotFound))
				})
		})

		It("shows dependencies unhealthy when ZTunnel is unhealthy", func() {
			Expect(k8sClient.Create(ctx, ztunnel)).To(Succeed())
			expectCondition(ctx, revName, v1.IstioRevisionConditionDependenciesHealthy, metav1.ConditionFalse,
				func(g Gomega, condition *v1.IstioRevisionCondition) {
					g.Expect(condition.Reason).To(Equal(v1.IstioRevisionReasonZTunnelNotHealthy))
				})
		})

		It("shows dependencies healthy when ZTunnel is healthy", func() {
			dsKey := client.ObjectKey{Name: "ztunnel", Namespace: ztunnel.Spec.Namespace}
			ds := &appsv1.DaemonSet{}
			Expect(k8sClient.Get(ctx, dsKey, ds)).To(Succeed())
			ds.Status.CurrentNumberScheduled = 3
			ds.Status.NumberReady = 3
			Expect(k8sClient.Status().Update(ctx, ds)).To(Succeed())

			expectZTunnelCondition(ctx, v1.ZTunnelConditionReady, metav1.ConditionTrue)
			expectCondition(ctx, revName, v1.IstioRevisionConditionDependenciesHealthy, metav1.ConditionTrue)
		})
	})

	Describe("target namespace dependency checks", func() {
		When("IstioRevision is created before the target namespace", func() {
			nsName := "nonexistent-namespace-" + rand.String(8)
			BeforeAll(func() {
				Step("Creating the IstioRevision resource without the namespace")
				rev = &v1.IstioRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: revName,
					},
					Spec: v1.IstioRevisionSpec{
						Version:   istioversion.Default,
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
				deleteAllIstioRevisions(ctx)
			})

			It("indicates in the status that the namespace doesn't exist", func() {
				expectCondition(ctx, revName, v1.IstioRevisionConditionReconciled, metav1.ConditionFalse,
					func(g Gomega, condition *v1.IstioRevisionCondition) {
						g.Expect(condition.Reason).To(Equal(v1.IstioRevisionReasonReconcileError))
						g.Expect(condition.Message).To(ContainSubstring(fmt.Sprintf("namespace %q doesn't exist", nsName)))
					})
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
				AfterAll(func() {
					Expect(k8sClient.Delete(ctx, ns)).To(Succeed())
				})

				It("reconciles immediately", func() {
					Step("Checking if istiod is deployed immediately")
					istiod := &appsv1.Deployment{}
					istiodKey := client.ObjectKey{Name: "istiod-" + revName, Namespace: ns.Name}
					Eventually(k8sClient.Get).WithArguments(ctx, istiodKey, istiod).WithTimeout(10 * time.Second).Should(Succeed())

					Step("Checking if the status is updated")
					expectCondition(ctx, revName, v1.IstioRevisionConditionReconciled, metav1.ConditionTrue)
				})
			})
		})

		When("target namespace is created before the IstioRevision", func() {
			BeforeAll(func() {
				// Uses istioNamespace which has already been created by suite setup
				rev = &v1.IstioRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: revName,
					},
					Spec: v1.IstioRevisionSpec{
						Version:   istioversion.Default,
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
			})

			AfterAll(func() {
				deleteAllIstioRevisions(ctx)
			})

			It("reconciles immediately", func() {
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
		})
	})

	Describe("istiod readiness changes", func() {
		BeforeAll(func() {
			rev = &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: revName,
				},
				Spec: v1.IstioRevisionSpec{
					Version:   istioversion.Default,
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
		})

		AfterAll(func() {
			deleteAllIstioRevisions(ctx)
		})

		It("has an initial Ready condition status of false when istiod isn't ready", func() {
			expectCondition(ctx, revName, v1.IstioRevisionConditionReady, metav1.ConditionFalse)
		})

		It("updates the status of the IstioRevision resource", func() {
			By("setting the Ready condition status to true when istiod is ready", func() {
				istiod := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, istiodKey, istiod)).To(Succeed())
				istiod.Status.Replicas = 1
				istiod.Status.ReadyReplicas = 1
				Expect(k8sClient.Status().Update(ctx, istiod)).To(Succeed())

				expectCondition(ctx, revName, v1.IstioRevisionConditionReady, metav1.ConditionTrue)
			})

			By("setting the Ready condition status to false when istiod isn't ready", func() {
				istiod := &appsv1.Deployment{}
				Expect(k8sClient.Get(ctx, istiodKey, istiod)).To(Succeed())

				istiod.Status.ReadyReplicas = 0
				Expect(k8sClient.Status().Update(ctx, istiod)).To(Succeed())

				expectCondition(ctx, revName, v1.IstioRevisionConditionReady, metav1.ConditionFalse)
			})
		})
	})

	Describe("owned resource reconciliations", func() {
		BeforeAll(func() {
			rev = &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: revName,
				},
				Spec: v1.IstioRevisionSpec{
					Version:   istioversion.Default,
					Namespace: istioNamespace,
					Values: &v1.Values{
						Global: &v1.GlobalConfig{
							IstioNamespace: ptr.Of(istioNamespace),
						},
						Revision: ptr.Of(revName),
						Pilot: &v1.PilotConfig{
							Image:        ptr.Of(pilotImage),
							AutoscaleMin: ptr.Of(uint32(2)),
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, rev)).To(Succeed())
		})

		AfterAll(func() {
			deleteAllIstioRevisions(ctx)
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
			Entry("ValidatingWebhookConfiguration",
				&admissionv1.ValidatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("istio-validator-%s-%s", revName, istioNamespace),
					},
				}, func(obj client.Object) {
					webhook := obj.(*admissionv1.ValidatingWebhookConfiguration)
					webhook.Webhooks[0].Name = "xyz.xyz.xyz"
					webhook.Webhooks[0].FailurePolicy = ptr.Of(admissionv1.Fail)
				}, func(g Gomega, obj client.Object) {
					webhook := obj.(*admissionv1.ValidatingWebhookConfiguration)
					g.Expect(webhook.Webhooks[0].Name).ToNot(Equal("xyz.xyz.xyz"))
					// FailurePolicy should not be changed because we have a post-render
					// step to remove it from the Helm generated YAML so it stays as-is
					// in cluster.
					g.Expect(webhook.Webhooks[0].FailurePolicy).To(HaveValue(Equal(admissionv1.Fail)))
				}),
		)

		DescribeTable("skips reconcile when only the status of the owned resource is updated",
			func(obj client.Object, modify func(obj client.Object)) {
				waitForInFlightReconcileToFinish()

				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
				expectNoReconciliation(istioRevisionController, func() {
					By("modifying object")
					modify(obj)
					Expect(k8sClient.Status().Update(ctx, obj)).To(Succeed())
				})
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

			expectNoReconciliation(istioRevisionController, func() {
				By("adding pull secret to ServiceAccount")
				sa.ImagePullSecrets = append(sa.ImagePullSecrets, corev1.LocalObjectReference{Name: "other-pull-secret"})
				Expect(k8sClient.Update(ctx, sa)).To(Succeed())
			})

			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(sa), sa)).To(Succeed())
			Expect(sa.ImagePullSecrets).To(ContainElement(corev1.LocalObjectReference{Name: "other-pull-secret"}))
		})

		It("skips reconcile when sailoperator.io/ignore annotation is set to true on a resource", func() {
			waitForInFlightReconcileToFinish()

			webhook := &admissionv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "istio-sidecar-injector-" + revName + "-" + istioNamespace,
				},
			}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(webhook), webhook)).To(Succeed())

			GinkgoWriter.Println("webhook:", webhook)

			expectNoReconciliation(istioRevisionController, func() {
				By("adding sailoperator.io/ignore annotation to ConfigMap")
				webhook.Annotations = map[string]string{
					"sailoperator.io/ignore": "true",
				}
				webhook.Labels["app"] = "sidecar-injector-test"
				Expect(k8sClient.Update(ctx, webhook)).To(Succeed())
			})

			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(webhook), webhook)).To(Succeed())
			Expect(webhook.Annotations["sailoperator.io/ignore"]).To(Equal("true"))
			Expect(webhook.Labels["app"]).To(Equal("sidecar-injector-test"))
		})
	})

	DescribeTableSubtree("reconciling when revision is in use",
		func(name, revision string, nsLabels, podLabels map[string]string) {
			BeforeAll(func() {
				rev := &v1.IstioRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: name,
					},
					Spec: v1.IstioRevisionSpec{
						Version:   istioversion.Default,
						Namespace: istioNamespace,
						Values: &v1.Values{
							Global: &v1.GlobalConfig{
								IstioNamespace: ptr.Of(istioNamespace),
							},
							Revision: &revision,
						},
					},
				}
				Expect(k8sClient.Create(ctx, rev)).To(Succeed())
			})

			AfterAll(func() {
				deleteAllIstioRevisions(ctx)
			})

			When("watching namespaces", func() {
				It("reconciles when a namespace marked for injection is created", func() {
					expectCondition(ctx, name, v1.IstioRevisionConditionInUse, metav1.ConditionFalse)
					ns := createOrUpdateNamespace(ctx, "injected-"+name, nsLabels)
					expectCondition(ctx, name, v1.IstioRevisionConditionInUse, metav1.ConditionTrue)

					// Clean up after test (envtest doesn't support namespace deletion, so set labels to nil)
					createOrUpdateNamespace(ctx, ns.Name, nil)
				})

				It("doesn't reconcile when a regular namespace is created", func() {
					waitForInFlightReconcileToFinish()
					expectNoReconciliation(istioRevisionController, func() {
						createOrUpdateNamespace(ctx, "not-injected-"+name, nil)
					})
				})
			})

			When("watching pods", func() {
				It("reconciles when a pod marked for injection is created in a regular namespace", func() {
					ns := createOrUpdateNamespace(ctx, "non-injected-"+name, nil)
					waitForInFlightReconcileToFinish()
					expectCondition(ctx, name, v1.IstioRevisionConditionInUse, metav1.ConditionFalse)

					pod := createPod(ctx, "injected-pod", ns.Name, podLabels)
					expectCondition(ctx, name, v1.IstioRevisionConditionInUse, metav1.ConditionTrue)

					// Clean up pod after the test
					Expect(k8sClient.Delete(ctx, pod)).To(Succeed())
				})

				It("doesn't reconcile when a pod marked not to inject is created in a namespace marked for injection", func() {
					ns := createOrUpdateNamespace(ctx, "injected-"+name, nsLabels)
					waitForInFlightReconcileToFinish()
					expectNoReconciliation(istioRevisionController, func() {
						createPod(ctx, "not-injected", ns.Name, map[string]string{constants.IstioSidecarInjectLabel: "false"})
					})

					// Clean up after test
					pod := &corev1.Pod{}
					Expect(k8sClient.Get(ctx, kube.Key("not-injected", ns.Name), pod)).To(Succeed())
					deletePod(ctx, pod)
					createOrUpdateNamespace(ctx, ns.Name, nil)
				})
			})
		},
		Entry("using the default IstioRevision", "default", "",
			map[string]string{constants.IstioInjectionLabel: constants.IstioInjectionEnabledValue},
			map[string]string{constants.IstioSidecarInjectLabel: "true"},
		),
		Entry("using a specific IstioRevision", revName, revName,
			map[string]string{constants.IstioRevLabel: revName},
			map[string]string{constants.IstioRevLabel: revName},
		),
	)

	Describe("multiple control planes", func() {
		BeforeAll(func() {
			rev = &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: revName,
				},
				Spec: v1.IstioRevisionSpec{
					Version:   istioversion.Default,
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
		})

		AfterAll(func() {
			deleteAllIstioRevisions(ctx)
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
					Version:   istioversion.Default,
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

			Step("Checking if the status is updated for both IstioRevisions")
			got := &v1.IstioRevision{}
			for _, key := range []client.ObjectKey{revKey, rev2Key} {
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, key, got)).To(Succeed())
					g.Expect(got.Status.ObservedGeneration).To(Equal(got.ObjectMeta.Generation))
				}).Should(Succeed())
			}

			Step("Checking if istiod Deployment was successfully created in the reconciliation for both IstioRevisions")
			istiod := &appsv1.Deployment{}

			for key, owner := range map[client.ObjectKey]*v1.IstioRevision{
				istiodKey:  rev,
				istiod2Key: rev2,
			} {
				Eventually(k8sClient.Get).WithArguments(ctx, key, istiod).Should(Succeed())
				Expect(istiod.Spec.Template.Spec.Containers[0].Image).To(Equal(pilotImage))
				Expect(istiod.ObjectMeta.OwnerReferences).To(ContainElement(NewOwnerReference(owner)))
			}
		})
	})

	When("Creating an IstioRevisionTag with name 'default' and attempting to create another IstioRevision with the same name", func() {
		BeforeAll(func() {
			deleteAllIstiosAndRevisions(ctx)
			deleteAllIstioRevisionTags(ctx)

			rev = &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: revName,
				},
				Spec: v1.IstioRevisionSpec{
					Version:   istioversion.Base,
					Namespace: istioNamespace,
					Values: &v1.Values{
						Revision: &revName,
						Global: &v1.GlobalConfig{
							IstioNamespace: &istioNamespace,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, rev)).To(Succeed())
			Step("Creating the IstioRevisionTag")
			tag = &v1.IstioRevisionTag{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1.IstioRevisionTagSpec{
					TargetRef: v1.IstioRevisionTagTargetReference{
						Kind: "IstioRevision",
						Name: revName,
					},
				},
			}
			Expect(k8sClient.Create(ctx, tag)).To(Succeed())
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, defaultKey, tag)).To(Succeed())
				g.Expect(tag.Status.ObservedGeneration).To(Equal(tag.Generation))
				g.Expect(tag.Status.GetCondition(v1.IstioRevisionTagConditionReconciled).Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())
			rev = &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1.IstioRevisionSpec{
					Version:   istioversion.Base,
					Namespace: istioNamespace,
					Values: &v1.Values{
						Global: &v1.GlobalConfig{
							IstioNamespace: &istioNamespace,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, rev)).To(Succeed())
		})

		AfterAll(func() {
			deleteAllIstioRevisionTags(ctx)
			deleteAllIstiosAndRevisions(ctx)
		})

		It("fails to reconcile IstioRevision", func() {
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, defaultKey, rev)).To(Succeed())
				g.Expect(rev.Status.ObservedGeneration).To(Equal(rev.Generation))
			}).Should(Succeed())
			Consistently(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, defaultKey, rev)).To(Succeed())
				g.Expect(rev.Status.GetCondition(v1.IstioRevisionConditionReconciled).Status).To(Equal(metav1.ConditionFalse))
				g.Expect(rev.Status.GetCondition(v1.IstioRevisionConditionReconciled).Reason).To(Equal(v1.IstioRevisionReasonNameAlreadyExists))
			}).Should(Succeed())
		})

		It("still reconciles the IstioRevisionTag", func() {
			rev = &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "something-else",
				},
				Spec: v1.IstioRevisionSpec{
					Version:   istioversion.Base,
					Namespace: istioNamespace,
					Values: &v1.Values{
						Revision: ptr.Of("something-else"),
						Global: &v1.GlobalConfig{
							IstioNamespace: &istioNamespace,
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, rev)).To(Succeed())
			// update Istio as well to make sure it's still reconciled
			Expect(k8sClient.Get(ctx, defaultKey, tag)).To(Succeed())
			tag.Spec.TargetRef.Kind = "IstioRevision"
			tag.Spec.TargetRef.Name = "something-else"
			Expect(k8sClient.Update(ctx, tag)).To(Succeed())
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, defaultKey, tag)).To(Succeed())
				g.Expect(tag.Generation).To(Equal(tag.Status.ObservedGeneration))
				g.Expect(tag.Status.GetCondition(v1.IstioRevisionTagConditionReconciled).Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())
		})
	})

	When("the IstioRevision has Spec.Values.GatewayClasses set", func() {
		BeforeAll(func() {
			rev = &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: revName,
				},
				Spec: v1.IstioRevisionSpec{
					Version:   istioversion.Default,
					Namespace: istioNamespace,
					Values: &v1.Values{
						Global: &v1.GlobalConfig{
							IstioNamespace: ptr.Of(istioNamespace),
						},
						Revision: ptr.Of(revName),
						Pilot: &v1.PilotConfig{
							Image: ptr.Of(pilotImage),
						},
						GatewayClasses: []byte(`{"istio":{"service":{"spec":{"type":"ClusterIP"}}}}`),
					},
				},
			}
			Expect(k8sClient.Create(ctx, rev)).To(Succeed())
		})

		AfterAll(func() {
			deleteAllIstioRevisions(ctx)
		})

		It("creates a configmap in the istiorevision-test namespace with the gateway class customization", func() {
			// Format of ConfigMap name is "istio-<revision>-gatewayclass-<gatewayclass-name>"
			cmKey := client.ObjectKey{Name: "istio-test-istiorevision-gatewayclass-istio", Namespace: istioNamespace}
			cm := &corev1.ConfigMap{}
			Eventually(k8sClient.Get).WithArguments(ctx, cmKey, cm).Should(Succeed())
			Expect(cm.Labels).To(HaveKeyWithValue("gateway.istio.io/defaults-for-class", "istio"))
			Expect(cm.Data).To(HaveKeyWithValue("service", `spec:
  type: ClusterIP
`))
		})
	})
})

func waitForInFlightReconcileToFinish() {
	// wait for the in-flight reconcile operations to finish
	// unfortunately, I don't see a good way to do this other than by waiting
	time.Sleep(5 * time.Second)
}

func deleteAllIstioRevisions(ctx context.Context) {
	Step("Deleting all IstioRevisions")
	Eventually(k8sClient.DeleteAllOf).WithArguments(ctx, &v1.IstioRevision{}).Should(Succeed())
	Eventually(func(g Gomega) {
		list := &v1.IstioRevisionList{}
		g.Expect(k8sClient.List(ctx, list)).To(Succeed())
		g.Expect(list.Items).To(BeEmpty())
	}).Should(Succeed())
}

func createOrUpdateNamespace(ctx context.Context, name string, labels map[string]string) *corev1.Namespace {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}

	err := k8sClient.Create(ctx, ns)
	if errors.IsAlreadyExists(err) {
		err = k8sClient.Update(ctx, ns)
	}

	Expect(err).ToNot(HaveOccurred())
	return ns
}

func createPod(ctx context.Context, name, ns string, labels map[string]string) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels:    labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test",
					Image: "test",
				},
			},
		},
	}
	Expect(k8sClient.Create(ctx, pod)).To(Succeed())
	return pod
}

// expectCondition on a given IstioRevision (identified by name) to eventually have a given status.
// Additional checks on the status can be done via an optional supplementary functions.
func expectCondition(ctx context.Context, name string, condition v1.IstioRevisionConditionType, status metav1.ConditionStatus,
	extraChecks ...func(Gomega, *v1.IstioRevisionCondition),
) {
	Eventually(func(g Gomega) {
		rev := &v1.IstioRevision{}
		g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: name}, rev)).To(Succeed())
		g.Expect(rev.Status.ObservedGeneration).To(Equal(rev.ObjectMeta.Generation))

		condition := rev.Status.GetCondition(condition)
		g.Expect(condition.Status).To(Equal(status))
		for _, check := range extraChecks {
			check(g, &condition)
		}
	}).Should(Succeed(), "Expected condition %q to be %q on IstioRevision: %s", condition, status, name)
}

// expectZTunnelCondition on the ZTunnel resource to eventually have a given status.
func expectZTunnelCondition(ctx context.Context, condition v1.ZTunnelConditionType, status metav1.ConditionStatus,
	extraChecks ...func(Gomega, *v1.ZTunnelCondition),
) {
	ztunnelKey := client.ObjectKey{Name: "default"}
	ztunnel := v1.ZTunnel{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, ztunnelKey, &ztunnel)).To(Succeed())
		g.Expect(ztunnel.Status.ObservedGeneration).To(Equal(ztunnel.ObjectMeta.Generation))

		condition := ztunnel.Status.GetCondition(condition)
		g.Expect(condition.Status).To(Equal(status))
		for _, check := range extraChecks {
			check(g, &condition)
		}
	}).Should(Succeed())
}
