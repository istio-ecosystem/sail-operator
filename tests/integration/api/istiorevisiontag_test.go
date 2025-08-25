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
	"time"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"istio.io/istio/pkg/ptr"
)

const (
	defaultTagName = "default"
)

var defaultTagKey = client.ObjectKey{Name: defaultTagName}

var _ = Describe("IstioRevisionTag resource", Label("istiorevisiontag"), Ordered, func() {
	const (
		istioName                 = "test-istio"
		istioRevisionTagNamespace = "istiorevisiontag-test"
		workloadNamespace         = "istiorevisiontag-test-workloads"

		gracePeriod = 5 * time.Second
	)
	istio := &v1.Istio{}
	istio2 := &v1.Istio{}
	istioKey := client.ObjectKey{Name: istioName}
	workloadNamespaceKey := client.ObjectKey{Name: workloadNamespace}
	tag := &v1.IstioRevisionTag{}

	SetDefaultEventuallyTimeout(30 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)

	SetDefaultConsistentlyDuration(10 * time.Second)
	SetDefaultConsistentlyPollingInterval(time.Second)

	ctx := context.Background()

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: istioRevisionTagNamespace,
		},
	}
	workloadNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: workloadNamespace,
		},
	}
	workload := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-workload",
			Namespace: workloadNamespace,
			Labels: map[string]string{
				"istio.io/rev": "default",
			},
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
	BeforeAll(func() {
		Step("Creating the Namespaces to perform the tests")
		Expect(k8sClient.Create(ctx, namespace)).To(Succeed())
		Expect(k8sClient.Create(ctx, workloadNs)).To(Succeed())
	})

	AfterAll(func() {
		// TODO(user): Attention if you improve this code by adding other context test you MUST
		// be aware of the current delete namespace limitations.
		// More info: https://book.kubebuilder.io/reference/envtest.html#testing-considerations
		Step("Deleting the Namespace to perform the tests")
		Expect(k8sClient.Delete(ctx, namespace)).To(Succeed())
	})

	for _, referencedResource := range []string{v1.IstioKind, v1.IstioRevisionKind} {
		for _, updateStrategy := range []v1.UpdateStrategyType{v1.UpdateStrategyTypeRevisionBased, v1.UpdateStrategyTypeInPlace} {
			Describe("referencing "+referencedResource+" resource, "+string(updateStrategy)+" update", func() {
				BeforeAll(func() {
					Step("Creating the Istio resource")
					istio = &v1.Istio{
						ObjectMeta: metav1.ObjectMeta{
							Name: istioName,
						},
						Spec: v1.IstioSpec{
							Version:   istioversion.Base,
							Namespace: istioRevisionTagNamespace,
							UpdateStrategy: &v1.IstioUpdateStrategy{
								Type: updateStrategy,
								InactiveRevisionDeletionGracePeriodSeconds: ptr.Of(int64(gracePeriod.Seconds())),
							},
						},
					}
					Expect(k8sClient.Create(ctx, istio)).To(Succeed())
				})

				AfterAll(func() {
					deleteAllIstioRevisionTags(ctx)
					deleteAllIstiosAndRevisions(ctx)
				})

				When("creating the IstioRevisionTag", func() {
					BeforeAll(func() {
						targetRef := v1.IstioRevisionTagTargetReference{
							Kind: referencedResource,
							Name: getRevisionName(istio, istio.Spec.Version),
						}
						if referencedResource == v1.IstioKind {
							targetRef.Name = istioName
						}
						tag = &v1.IstioRevisionTag{
							ObjectMeta: metav1.ObjectMeta{
								Name: "default",
							},
							Spec: v1.IstioRevisionTagSpec{
								TargetRef: targetRef,
							},
						}
						Expect(k8sClient.Create(ctx, tag)).To(Succeed())
					})
					It("updates IstioRevisionTag status", func() {
						expectTagInUse(ctx, metav1.ConditionFalse, getRevisionName(istio, istioversion.Base))
					})
				})
				When("workload ns is labeled with istio-injection label", func() {
					BeforeAll(func() {
						Expect(k8sClient.Get(ctx, workloadNamespaceKey, workloadNs)).To(Succeed())
						workloadNs.Labels["istio-injection"] = "enabled"
						Expect(k8sClient.Update(ctx, workloadNs)).To(Succeed())
					})
					It("updates IstioRevisionTag status and detects that the revision tag is in use", func() {
						expectTagInUse(ctx, metav1.ConditionTrue, "")
					})
				})

				When("updating the Istio control plane version", func() {
					BeforeAll(func() {
						Expect(k8sClient.Get(ctx, istioKey, istio)).To(Succeed())
						istio.Spec.Version = istioversion.Default
						Expect(k8sClient.Update(ctx, istio)).To(Succeed())
					})

					if referencedResource == v1.IstioRevisionKind {
						It("updates IstioRevisionTag status and still references old revision", func() {
							expectTagInUse(ctx, metav1.ConditionTrue, getRevisionName(istio, istioversion.Base))
						})
					} else {
						It("updates IstioRevisionTag status and shows new referenced revision", func() {
							expectTagInUse(ctx, metav1.ConditionTrue, getRevisionName(istio, istioversion.New))
						})
					}
				})

				When("deleting the label on the workload namespace", func() {
					BeforeAll(func() {
						Expect(k8sClient.Get(ctx, workloadNamespaceKey, workloadNs)).To(Succeed())
						delete(workloadNs.Labels, "istio-injection")
						Expect(k8sClient.Update(ctx, workloadNs)).To(Succeed())
					})

					It("updates IstioRevisionTag status and detects that the tag is no longer in use", func() {
						expectTagInUse(ctx, metav1.ConditionFalse, "")
					})
					if referencedResource == v1.IstioRevisionKind && updateStrategy == v1.UpdateStrategyTypeRevisionBased {
						It("does not delete the referenced IstioRevision even though it is no longer in use and not the active revision", func() {
							revKey := client.ObjectKey{Name: getRevisionName(istio, istioversion.Base)}
							rev := &v1.IstioRevision{}
							Consistently(k8sClient.Get).WithArguments(ctx, revKey, rev).Should(Succeed())
						})
					}
				})

				When("creating a Pod that references the tag", func() {
					BeforeAll(func() {
						Expect(k8sClient.Create(ctx, workload.DeepCopy())).To(Succeed())
					})

					AfterAll(func() {
						deletePod(ctx, workload)
					})

					It("updates IstioRevisionTag status and detects that the revision tag is in use", func() {
						expectTagInUse(ctx, metav1.ConditionTrue, "")
					})
				})
			})
		}
	}

	When("Creating an Istio with name 'default' and attempting to create another IstioRevisionTag with the same name", func() {
		BeforeAll(func() {
			Step("Creating the Istio resources")
			istio = &v1.Istio{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1.IstioSpec{
					Version:   istioversion.Base,
					Namespace: istioRevisionTagNamespace,
					UpdateStrategy: &v1.IstioUpdateStrategy{
						Type: v1.UpdateStrategyTypeInPlace,
						InactiveRevisionDeletionGracePeriodSeconds: ptr.Of(int64(gracePeriod.Seconds())),
					},
				},
			}
			Expect(k8sClient.Create(ctx, istio)).To(Succeed())
			istio = &v1.Istio{
				ObjectMeta: metav1.ObjectMeta{
					Name: istioName,
				},
				Spec: v1.IstioSpec{
					Version:   istioversion.Base,
					Namespace: istioRevisionTagNamespace,
					UpdateStrategy: &v1.IstioUpdateStrategy{
						Type: v1.UpdateStrategyTypeInPlace,
						InactiveRevisionDeletionGracePeriodSeconds: ptr.Of(int64(gracePeriod.Seconds())),
					},
				},
			}
			Expect(k8sClient.Create(ctx, istio)).To(Succeed())
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, defaultTagKey, istio)).To(Succeed())
				g.Expect(istio.Status.ObservedGeneration).To(Equal(istio.Generation))
			}).Should(Succeed())
			tag = &v1.IstioRevisionTag{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1.IstioRevisionTagSpec{
					TargetRef: v1.IstioRevisionTagTargetReference{
						Kind: "Istio",
						Name: istioName,
					},
				},
			}
			Expect(k8sClient.Create(ctx, tag)).To(Succeed())
		})

		AfterAll(func() {
			deleteAllIstiosAndRevisions(ctx)
			deleteAllIstioRevisionTags(ctx)
		})

		It("fails to reconcile IstioRevisionTag", func() {
			expectTagNotReconciled(ctx, v1.IstioRevisionTagReasonNameAlreadyExists)
		})

		It("still reconciles the IstioRevision", func() {
			// update Istio as well to make sure it's still reconciled
			Expect(k8sClient.Get(ctx, defaultTagKey, istio)).To(Succeed())
			istio.Spec.Values = &v1.Values{
				Pilot: &v1.PilotConfig{
					Env: map[string]string{"test": "test"},
				},
			}
			Expect(k8sClient.Update(ctx, istio)).To(Succeed())
			Eventually(func(g Gomega) {
				rev := &v1.IstioRevision{}
				g.Expect(k8sClient.Get(ctx, defaultTagKey, rev)).To(Succeed())
				g.Expect(rev.Spec.Values).To(Not(BeNil()))
				g.Expect(rev.Spec.Values.Pilot).To(Not(BeNil()))
				g.Expect(rev.Spec.Values.Pilot.Env["test"]).To(Equal("test"))
				g.Expect(rev.Status.GetCondition(v1.IstioRevisionConditionReconciled).Status).To(Equal(metav1.ConditionTrue))
			}).Should(Succeed())
		})
	})

	When("Creating an IstioRevisionTag with a dangling TargetRef", func() {
		BeforeAll(func() {
			tag = &v1.IstioRevisionTag{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1.IstioRevisionTagSpec{
					TargetRef: v1.IstioRevisionTagTargetReference{
						Kind: "Istio",
						Name: istioName,
					},
				},
			}
			Expect(k8sClient.Create(ctx, tag)).To(Succeed())
		})

		AfterAll(func() {
			deleteAllIstiosAndRevisions(ctx)
			deleteAllIstioRevisionTags(ctx)
		})

		It("fails to reconcile IstioRevisionTag", func() {
			expectTagNotReconciled(ctx, v1.IstioRevisionTagReasonReferenceNotFound)
		})

		When("attempting to create IstioRevision with same name as the tag's", func() {
			BeforeAll(func() {
				istio = &v1.Istio{
					ObjectMeta: metav1.ObjectMeta{
						Name: "default",
					},
					Spec: v1.IstioSpec{
						Version:   istioversion.Base,
						Namespace: istioRevisionTagNamespace,
						UpdateStrategy: &v1.IstioUpdateStrategy{
							Type: v1.UpdateStrategyTypeInPlace,
							InactiveRevisionDeletionGracePeriodSeconds: ptr.Of(int64(gracePeriod.Seconds())),
						},
					},
				}
				Expect(k8sClient.Create(ctx, istio)).To(Succeed())
			})

			It("fails with ValidationError", func() {
				Eventually(func(g Gomega) {
					rev := &v1.IstioRevision{}
					g.Expect(k8sClient.Get(ctx, defaultTagKey, rev)).To(Succeed())
					condition := rev.Status.GetCondition(v1.IstioRevisionConditionReconciled)
					g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
					g.Expect(condition.Message).To(ContainSubstring("an IstioRevisionTag exists with this name"))
				}).Should(Succeed())
			})
		})
	})

	When("Changing the targetRef of a tag to an IstioRevision in another namespace", func() {
		BeforeAll(func() {
			Step("Create primary Istio")
			istio = &v1.Istio{
				ObjectMeta: metav1.ObjectMeta{
					Name: istioName,
				},
				Spec: v1.IstioSpec{
					Version:   istioversion.Base,
					Namespace: istioRevisionTagNamespace,
					UpdateStrategy: &v1.IstioUpdateStrategy{
						Type: v1.UpdateStrategyTypeInPlace,
						InactiveRevisionDeletionGracePeriodSeconds: ptr.Of(int64(gracePeriod.Seconds())),
					},
				},
			}
			Expect(k8sClient.Create(ctx, istio)).To(Succeed())
			Step("Create secondary Istio")
			istio2 = &v1.Istio{
				ObjectMeta: metav1.ObjectMeta{
					Name: istioName + "2",
				},
				Spec: v1.IstioSpec{
					Version:   istioversion.Base,
					Namespace: workloadNamespace,
					UpdateStrategy: &v1.IstioUpdateStrategy{
						Type: v1.UpdateStrategyTypeInPlace,
						InactiveRevisionDeletionGracePeriodSeconds: ptr.Of(int64(gracePeriod.Seconds())),
					},
				},
			}
			Expect(k8sClient.Create(ctx, istio2)).To(Succeed())
			Step("Create IstioRevisionTag default")
			tag = &v1.IstioRevisionTag{
				ObjectMeta: metav1.ObjectMeta{
					Name: defaultTagName,
				},
				Spec: v1.IstioRevisionTagSpec{
					TargetRef: v1.IstioRevisionTagTargetReference{
						Kind: "Istio",
						Name: istioName,
					},
				},
			}
			Expect(k8sClient.Create(ctx, tag)).To(Succeed())
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, defaultTagKey, tag)).To(Succeed())
				g.Expect(tag.Status.IstiodNamespace).To(Equal(istioRevisionTagNamespace))
			}).Should(Succeed())
			Step("Switch IstioRevisionTag to secondary istio")
			tag.Spec.TargetRef.Name = istio2.Name
			Expect(k8sClient.Update(ctx, tag)).To(Succeed())
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, defaultTagKey, tag)).To(Succeed())
				g.Expect(tag.Status.IstiodNamespace).To(Equal(workloadNamespace))
			}).Should(Succeed())

			deleteAllIstiosAndRevisions(ctx)
			deleteAllIstioRevisionTags(ctx)

			Step("Create conflicting Istio and IstioRevisionTags")
			istio = &v1.Istio{
				ObjectMeta: metav1.ObjectMeta{
					Name: defaultTagName,
				},
				Spec: v1.IstioSpec{
					Version:   istioversion.Base,
					Namespace: istioRevisionTagNamespace,
					UpdateStrategy: &v1.IstioUpdateStrategy{
						Type: v1.UpdateStrategyTypeInPlace,
						InactiveRevisionDeletionGracePeriodSeconds: ptr.Of(int64(gracePeriod.Seconds())),
					},
				},
			}
			Expect(k8sClient.Create(ctx, istio)).To(Succeed())
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, defaultTagKey, istio)).To(Succeed())
				g.Expect(istio.Status.ObservedGeneration).To(Equal(istio.ObjectMeta.Generation))
			}).Should(Succeed())

			tag = &v1.IstioRevisionTag{
				ObjectMeta: metav1.ObjectMeta{
					Name: defaultTagName,
				},
				Spec: v1.IstioRevisionTagSpec{
					TargetRef: v1.IstioRevisionTagTargetReference{
						Kind: "Istio",
						Name: istioName,
					},
				},
			}
			Expect(k8sClient.Create(ctx, tag)).To(Succeed())
			expectTagNotReconciled(ctx, v1.IstioRevisionTagReasonNameAlreadyExists)
		})

		AfterAll(func() {
			deleteAllIstiosAndRevisions(ctx)
			deleteAllIstioRevisionTags(ctx)
		})

		It("can still delete the IstioRevisionTag", func() {
			Eventually(k8sClient.Delete).WithArguments(ctx, tag).Should(Succeed())
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, defaultTagKey, tag)).To(ReturnNotFoundError())
			}).Should(Succeed())
		})
	})
})

func deleteAllIstioRevisionTags(ctx context.Context) {
	Step("Deleting all IstioRevisionTags")
	Eventually(k8sClient.DeleteAllOf).WithArguments(ctx, &v1.IstioRevisionTag{}).Should(Succeed())
	Eventually(func(g Gomega) {
		list := &v1.IstioRevisionTagList{}
		g.Expect(k8sClient.List(ctx, list)).To(Succeed())
		g.Expect(list.Items).To(BeEmpty())
	}).Should(Succeed())
}

func deletePod(ctx context.Context, pod *corev1.Pod) {
	Step("Deleting pod")
	Eventually(k8sClient.Delete).WithArguments(ctx, pod).Should(Succeed())
	Eventually(func(g Gomega) {
		p := &corev1.Pod{}
		g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, p)).To(ReturnNotFoundError())
	}).Should(Succeed())
}

// expectTagInUse to match the expected status for the default IstioRevisionTag.
// Optionally, the istio revision can be specified to also be checked, if not empty.
func expectTagInUse(ctx context.Context, status metav1.ConditionStatus, revision string) {
	Eventually(func(g Gomega) {
		tag := &v1.IstioRevisionTag{}
		g.Expect(k8sClient.Get(ctx, defaultTagKey, tag)).To(Succeed())
		g.Expect(tag.Status.ObservedGeneration).To(Equal(tag.ObjectMeta.Generation))
		g.Expect(tag.Status.GetCondition(v1.IstioRevisionTagConditionInUse).Status).To(Equal(status))
		if revision != "" {
			g.Expect(tag.Status.IstioRevision).To(Equal(revision))
		}
	}).Should(Succeed())
}

func expectTagNotReconciled(ctx context.Context, reason v1.IstioRevisionTagConditionReason) {
	tag := &v1.IstioRevisionTag{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, defaultTagKey, tag)).To(Succeed())
		g.Expect(tag.Status.ObservedGeneration).To(Equal(tag.Generation))
	}).Should(Succeed())
	Consistently(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, defaultTagKey, tag)).To(Succeed())
		g.Expect(tag.Status.GetCondition(v1.IstioRevisionTagConditionReconciled).Status).To(Equal(metav1.ConditionFalse))
		g.Expect(tag.Status.GetCondition(v1.IstioRevisionTagConditionReconciled).Reason).To(Equal(reason))
	}).Should(Succeed())
}
