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
	"github.com/istio-ecosystem/sail-operator/pkg/enqueuelogger"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("IstioCNI", Ordered, func() {
	const (
		cniName      = "default"
		cniNamespace = "istiocni-test"
	)

	SetDefaultEventuallyPollingInterval(time.Second)
	SetDefaultEventuallyTimeout(30 * time.Second)

	enqueuelogger.LogEnqueueEvents = true

	ctx := context.Background()

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: cniNamespace,
		},
	}

	cniKey := client.ObjectKey{Name: cniName}
	daemonsetKey := client.ObjectKey{Name: "istio-cni-node", Namespace: cniNamespace}

	cni := &v1.IstioCNI{}
	cniList := &v1.IstioCNIList{}
	ds := &appsv1.DaemonSet{}

	BeforeAll(func() {
		Expect(k8sClient.Create(ctx, namespace)).To(Succeed())
	})

	AfterAll(func() {
		Expect(k8sClient.Delete(ctx, namespace)).To(Succeed())
	})

	Describe("validation", func() {
		It("only accepts IstioCNI with the name 'default'", func() {
			cni = &v1.IstioCNI{
				ObjectMeta: metav1.ObjectMeta{
					Name: "not-default",
				},
				Spec: v1.IstioCNISpec{
					Version:   istioversion.Default,
					Namespace: cniNamespace,
				},
			}
			Expect(k8sClient.Create(ctx, cni)).To(Not(Succeed()))
		})
	})

	Describe("basic operation", func() {
		Describe("reconciles immediately after target namespace is created", func() {
			nsName := "nonexistent-namespace-" + rand.String(8)
			BeforeAll(func() {
				By("Creating the IstioCNI resource without the namespace")
				cni = &v1.IstioCNI{
					ObjectMeta: metav1.ObjectMeta{
						Name: cniName,
					},
					Spec: v1.IstioCNISpec{
						Version:   istioversion.Default,
						Namespace: nsName,
					},
				}
				Expect(k8sClient.Create(ctx, cni)).To(Succeed())
			})

			AfterAll(func() {
				Expect(k8sClient.Delete(ctx, cni)).To(Succeed())
				Eventually(k8sClient.Get).WithArguments(ctx, kube.Key(cniName), cni).Should(ReturnNotFoundError())
			})

			It("indicates in the status that the namespace doesn't exist", func() {
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, cniKey, cni)).To(Succeed())
					g.Expect(cni.Status.ObservedGeneration).To(Equal(cni.ObjectMeta.Generation))

					reconciled := cni.Status.GetCondition(v1.IstioCNIConditionReconciled)
					g.Expect(reconciled.Status).To(Equal(metav1.ConditionFalse))
					g.Expect(reconciled.Reason).To(Equal(v1.IstioCNIReasonReconcileError))
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
					Step("Checking if istio-cni-node DaemonSet is deployed immediately")
					dsKey := client.ObjectKey{Name: "istio-cni-node", Namespace: nsName}
					Eventually(k8sClient.Get).WithArguments(ctx, dsKey, ds).WithTimeout(10 * time.Second).Should(Succeed())

					Step("Checking if the status is updated")
					Eventually(func(g Gomega) {
						g.Expect(k8sClient.Get(ctx, cniKey, cni)).To(Succeed())
						g.Expect(cni.Status.ObservedGeneration).To(Equal(cni.ObjectMeta.Generation))
						reconciled := cni.Status.GetCondition(v1.IstioCNIConditionReconciled)
						g.Expect(reconciled.Status).To(Equal(metav1.ConditionTrue))
					}).Should(Succeed())
				})
			})
		})

		When("the resource is created", func() {
			BeforeAll(func() {
				cni = &v1.IstioCNI{
					ObjectMeta: metav1.ObjectMeta{
						Name: cniName,
					},
					Spec: v1.IstioCNISpec{
						Version:   istioversion.Default,
						Namespace: cniNamespace,
					},
				}
				Expect(k8sClient.Create(ctx, cni)).To(Succeed())
			})

			It("creates the istio-cni-node DaemonSet", func() {
				Eventually(k8sClient.Get).WithArguments(ctx, daemonsetKey, ds).Should(Succeed())
				Expect(ds.ObjectMeta.OwnerReferences).To(ContainElement(NewOwnerReference(cni)))
			})

			It("updates the status of the IstioCNI resource", func() {
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, cniKey, cni)).To(Succeed())
					g.Expect(cni.Status.ObservedGeneration).To(Equal(cni.ObjectMeta.Generation))
				}).Should(Succeed())
			})
		})

		Context("status changes", func() {
			When("DaemonSet becomes ready", func() {
				BeforeAll(func() {
					Expect(k8sClient.Get(ctx, daemonsetKey, ds)).To(Succeed())
					ds.Status.CurrentNumberScheduled = 3
					ds.Status.NumberReady = 3
					Expect(k8sClient.Status().Update(ctx, ds)).To(Succeed())
				})

				It("marks the IstioCNI resource as ready", func() {
					Eventually(func(g Gomega) {
						g.Expect(k8sClient.Get(ctx, cniKey, cni)).To(Succeed())
						readyCondition := cni.Status.GetCondition(v1.IstioCNIConditionReady)
						g.Expect(readyCondition.Status).To(Equal(metav1.ConditionTrue))
					}).Should(Succeed())
				})
			})

			When("DaemonSet becomes not ready", func() {
				BeforeAll(func() {
					Expect(k8sClient.Get(ctx, daemonsetKey, ds)).To(Succeed())
					ds.Status.CurrentNumberScheduled = 3
					ds.Status.NumberReady = 2
					Expect(k8sClient.Status().Update(ctx, ds)).To(Succeed())
				})

				It("marks the IstioCNI resource as not ready", func() {
					Eventually(func(g Gomega) {
						g.Expect(k8sClient.Get(ctx, cniKey, cni)).To(Succeed())
						readyCondition := cni.Status.GetCondition(v1.IstioCNIConditionReady)
						g.Expect(readyCondition.Status).To(Equal(metav1.ConditionFalse))
					}).Should(Succeed())
				})
			})
		})

		Context("changes to owned resources", func() {
			When("DaemonSet is deleted", func() {
				BeforeAll(func() {
					Expect(k8sClient.Delete(ctx, ds)).To(Succeed())
				})

				It("recreates the DaemonSet", func() {
					Eventually(k8sClient.Get).WithArguments(ctx, daemonsetKey, ds).Should(Succeed())
					Expect(ds.ObjectMeta.OwnerReferences).To(ContainElement(NewOwnerReference(cni)))
				})
			})

			When("DaemonSet is modified", func() {
				var originalImage string

				BeforeAll(func() {
					Expect(k8sClient.Get(ctx, daemonsetKey, ds)).To(Succeed())
					originalImage = ds.Spec.Template.Spec.Containers[0].Image

					ds.Spec.Template.Spec.Containers[0].Image = "user-supplied-image"
					Expect(k8sClient.Update(ctx, ds)).To(Succeed())
				})

				It("reverts the changes", func() {
					Eventually(func(g Gomega) {
						g.Expect(k8sClient.Get(ctx, daemonsetKey, ds)).To(Succeed())
						g.Expect(ds.Spec.Template.Spec.Containers[0].Image).To(Equal(originalImage))
					}).Should(Succeed())
				})
			})

			It("skips reconcile when a pull secret is added to service account", func() {
				waitForInFlightReconcileToFinish()

				sa := &corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istio-cni",
						Namespace: cniNamespace,
					},
				}
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(sa), sa)).To(Succeed())

				beforeCount := getIstioCNIReconcileCount(Default)

				By("adding pull secret to ServiceAccount")
				sa.ImagePullSecrets = append(sa.ImagePullSecrets, corev1.LocalObjectReference{Name: "other-pull-secret"})
				Expect(k8sClient.Update(ctx, sa)).To(Succeed())

				Consistently(func(g Gomega) {
					afterCount := getIstioCNIReconcileCount(g)
					g.Expect(afterCount).To(Equal(beforeCount))
				}, 5*time.Second).Should(Succeed(), "IstioRevision was reconciled when it shouldn't have been")

				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(sa), sa)).To(Succeed())
				Expect(sa.ImagePullSecrets).To(ContainElement(corev1.LocalObjectReference{Name: "other-pull-secret"}))
			})
		})

		When("the resource is deleted", func() {
			BeforeAll(func() {
				Expect(k8sClient.DeleteAllOf(ctx, &v1.IstioCNI{})).To(Succeed())
			})

			It("deletes the istio-cni-node DaemonSet", func() {
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.List(ctx, cniList)).To(Succeed())
					g.Expect(cniList.Items).To(BeEmpty())
				}).Should(Succeed())
			})
		})
	})
})
