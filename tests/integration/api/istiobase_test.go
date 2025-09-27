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
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	"github.com/istio-ecosystem/sail-operator/pkg/enqueuelogger"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"istio.io/istio/pkg/ptr"
)

var _ = Describe("base chart support", Ordered, func() {
	const (
		istioNamespace  = "istiobase-test"
		istioNamespace2 = "istiobase-test2"
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

	saKey := client.ObjectKey{Name: "istio-reader-service-account", Namespace: istioNamespace}
	validatingWebhookKey := client.ObjectKey{Name: "istiod-default-validator", Namespace: istioNamespace}

	BeforeAll(func() {
		Step(fmt.Sprintf("Creating namespace %q", istioNamespace))
		Expect(k8sClient.Create(ctx, namespace)).To(Succeed())
	})

	AfterAll(func() {
		Step(fmt.Sprintf("Deleting namespace %q", istioNamespace))
		Expect(k8sClient.Delete(ctx, namespace)).To(Succeed())

		Eventually(k8sClient.DeleteAllOf).WithArguments(ctx, &v1.IstioRevision{}).Should(Succeed())
		Eventually(func(g Gomega) {
			list := &v1.IstioRevisionList{}
			g.Expect(k8sClient.List(ctx, list)).To(Succeed())
			g.Expect(list.Items).To(BeEmpty())
		}).Should(Succeed())
	})

	Describe("default IstioRevision", func() {
		rev := &v1.IstioRevision{}

		It("deploys base chart when default IstioRevision is created", func() {
			Step("Creating the IstioRevision")
			rev = &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1.IstioRevisionSpec{
					Version:   istioversion.Default,
					Namespace: istioNamespace,
					Values: &v1.Values{
						Global: &v1.GlobalConfig{
							IstioNamespace: ptr.Of(istioNamespace),
						},
						Revision: ptr.Of(""),
					},
				},
			}
			Expect(k8sClient.Create(ctx, rev)).To(Succeed())

			sa := &corev1.ServiceAccount{}
			Step("Checking if istio-reader ServiceAccount was successfully created")
			Eventually(k8sClient.Get).WithArguments(ctx, saKey, sa).Should(Succeed())

			webhook := &admissionv1.ValidatingWebhookConfiguration{}
			Step("Checking if default ValidatingWebhookConfiguration was successfully created")
			Eventually(k8sClient.Get).WithArguments(ctx, validatingWebhookKey, webhook).Should(Succeed())
		})

		It("undeploys base chart when default IstioRevision is deleted", func() {
			Step("Deleting the default IstioRevision")
			Expect(k8sClient.Delete(ctx, rev)).To(Succeed())

			sa := &corev1.ServiceAccount{}
			Step("Checking if istio-reader ServiceAccount was deleted")
			Eventually(k8sClient.Get).WithArguments(ctx, saKey, sa).Should(ReturnNotFoundError())

			webhook := &admissionv1.ValidatingWebhookConfiguration{}
			Step("Checking if default ValidatingWebhookConfiguration was deleted")
			Eventually(k8sClient.Get).WithArguments(ctx, validatingWebhookKey, webhook).Should(ReturnNotFoundError())
		})
	})

	Describe("default IstioRevisionTag", func() {
		tag := &v1.IstioRevisionTag{}

		BeforeAll(func() {
			tag = &v1.IstioRevisionTag{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1.IstioRevisionTagSpec{
					TargetRef: v1.IstioRevisionTagTargetReference{
						Kind: "IstioRevision",
						Name: "my-rev",
					},
				},
			}
			Expect(k8sClient.Create(ctx, tag)).To(Succeed())
			Step("Creating the IstioRevision")
			rev := &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-rev",
				},
				Spec: v1.IstioRevisionSpec{
					Version:   istioversion.Default,
					Namespace: istioNamespace,
					Values: &v1.Values{
						Global: &v1.GlobalConfig{
							IstioNamespace: ptr.Of(istioNamespace),
						},
						Revision: ptr.Of("my-rev"),
					},
				},
			}
			Expect(k8sClient.Create(ctx, rev)).To(Succeed())
		})

		AfterAll(func() {
			deleteAllIstioRevisionTags(ctx)
			deleteAllIstiosAndRevisions(ctx)

			sa := &corev1.ServiceAccount{}
			Step("Checking if istio-reader ServiceAccount was deleted")
			Eventually(k8sClient.Get).WithArguments(ctx, saKey, sa).Should(ReturnNotFoundError())

			webhook := &admissionv1.ValidatingWebhookConfiguration{}
			Step("Checking if default ValidatingWebhookConfiguration was deleted")
			Eventually(k8sClient.Get).WithArguments(ctx, validatingWebhookKey, webhook).Should(ReturnNotFoundError())
		})

		It("deploys base chart when default IstioRevisionTag is created", func() {
			sa := &corev1.ServiceAccount{}
			Step("Checking if istio-reader ServiceAccount was successfully created")
			Eventually(k8sClient.Get).WithArguments(ctx, saKey, sa).Should(Succeed())

			webhook := &admissionv1.ValidatingWebhookConfiguration{}
			Step("Checking if default ValidatingWebhookConfiguration was successfully created")
			Eventually(k8sClient.Get).WithArguments(ctx, validatingWebhookKey, webhook).Should(Succeed())
		})
	})

	Describe("reconciles when owned resources are modified", func() {
		rev := &v1.IstioRevision{}

		BeforeAll(func() {
			rev = &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1.IstioRevisionSpec{
					Version:   istioversion.Default,
					Namespace: istioNamespace,
					Values: &v1.Values{
						Global: &v1.GlobalConfig{
							IstioNamespace: ptr.Of(istioNamespace),
						},
						Revision: ptr.Of(""),
					},
				},
			}
			Expect(k8sClient.Create(ctx, rev)).To(Succeed())
			Eventually(k8sClient.Get).WithArguments(ctx, saKey, &corev1.ServiceAccount{}).Should(Succeed())
		})

		AfterAll(func() {
			Expect(k8sClient.Delete(ctx, rev)).To(Succeed())
			Eventually(k8sClient.Get).WithArguments(ctx, saKey, &corev1.ServiceAccount{}).Should(ReturnNotFoundError())
		})

		DescribeTable("reconciles owned resource",
			func(obj client.Object, modify func(obj client.Object), validate func(g Gomega, obj client.Object)) {
				By("on update", func() {
					if modify == nil {
						Skip("skipping on update test, because no modify function was provided")
					}
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
						g.Expect(obj.GetLabels()[constants.ManagedByLabelKey]).To(Equal(constants.ManagedByLabelValue))
						validate(g, obj)
					}).Should(Succeed())
				})
			},
			Entry("ServiceAccount",
				&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istio-reader-service-account",
						Namespace: istioNamespace,
					},
				}, nil, func(g Gomega, obj client.Object) {
					sa := obj.(*corev1.ServiceAccount)
					g.Expect(sa.Labels["app.kubernetes.io/name"]).To(Equal("istio-reader"))
				}),
			Entry("ValidatingWebhookConfiguration",
				&admissionv1.ValidatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name: "istiod-default-validator",
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

		It("skips reconcile when a pull secret is added to the istio-reader service account", func() {
			waitForInFlightReconcileToFinish()

			sa := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "istio-reader-service-account",
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
	})

	Describe("IstioRevisionTag is updated to point to a new IstioRevision", func() {
		var rev1, rev2, rev3 v1.IstioRevision
		var sa1, sa2 corev1.ServiceAccount
		var tag v1.IstioRevisionTag
		ns1 := istioNamespace
		ns2 := istioNamespace2
		saKey1 := client.ObjectKey{Name: "istio-reader-service-account", Namespace: ns1}
		saKey2 := client.ObjectKey{Name: "istio-reader-service-account", Namespace: ns2}

		BeforeAll(func() {
			Step("Creating the IstioRevision")
			rev1 = v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "rev1",
				},
				Spec: v1.IstioRevisionSpec{
					Version:   istioversion.Default,
					Namespace: ns1,
					Values: &v1.Values{
						Global: &v1.GlobalConfig{
							IstioNamespace: ptr.Of(ns1),
						},
						Revision: ptr.Of("rev1"),
					},
				},
			}
			Expect(k8sClient.Create(ctx, &rev1)).To(Succeed())

			Step("Creating the IstioRevisionTag")
			tag = v1.IstioRevisionTag{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1.IstioRevisionTagSpec{
					TargetRef: v1.IstioRevisionTagTargetReference{
						Kind: "IstioRevision",
						Name: "rev1",
					},
				},
			}
			Expect(k8sClient.Create(ctx, &tag)).To(Succeed())

			Step("Checking if istio-reader ServiceAccount was successfully created")
			Eventually(k8sClient.Get).WithArguments(ctx, saKey1, &sa1).Should(Succeed())
		})

		It("new IstioRevision in same namespace", func() {
			rev2 = v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "rev2",
				},
				Spec: v1.IstioRevisionSpec{
					Version:   istioversion.Default,
					Namespace: ns1,
					Values: &v1.Values{
						Global: &v1.GlobalConfig{
							IstioNamespace: ptr.Of(ns1),
						},
						Revision: ptr.Of("rev2"),
					},
				},
			}
			Expect(k8sClient.Create(ctx, &rev2)).To(Succeed())

			Step("Updating IstioRevisionTag to point to a new IstioRevision in same namespace")
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&tag), &tag)).To(Succeed())
			tag.Spec.TargetRef.Name = rev2.Name
			Expect(k8sClient.Update(ctx, &tag)).To(Succeed())

			Step("Checking if istio-reader ServiceAccount has been preserved")
			sa1UID := sa1.UID
			Consistently(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, saKey1, &sa1)).To(Succeed())
				g.Expect(sa1.UID).To(Equal(sa1UID), "istio-reader ServiceAccount was deleted and recreated when it shouldn't have been")
			}).Should(Succeed())
		})

		It("new IstioRevision in different namespace", func() {
			Step(fmt.Sprintf("Creating namespace %q", ns2))
			namespace2 := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: ns2,
				},
			}
			Expect(k8sClient.Create(ctx, namespace2)).To(Succeed())

			rev3 = v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "rev3",
				},
				Spec: v1.IstioRevisionSpec{
					Version:   istioversion.Default,
					Namespace: ns2,
					Values: &v1.Values{
						Global: &v1.GlobalConfig{
							IstioNamespace: ptr.Of(ns2),
						},
						Revision: ptr.Of("rev3"),
					},
				},
			}
			Step(fmt.Sprintf("Creating IstioRevision %q", rev3.Name))
			Expect(k8sClient.Create(ctx, &rev3)).To(Succeed())

			Step("Updating IstioRevisionTag to point to a new IstioRevision in different namespace")
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&tag), &tag)).To(Succeed())
			tag.Spec.TargetRef.Name = rev3.Name
			Expect(k8sClient.Update(ctx, &tag)).To(Succeed())

			Step("Checking if istio-reader ServiceAccount in 1st namespace was deleted")
			Eventually(k8sClient.Get).WithArguments(ctx, saKey1, &sa1).Should(ReturnNotFoundError())

			Step("Checking if istio-reader ServiceAccount in 2nd namespace was successfully created")
			Eventually(k8sClient.Get).WithArguments(ctx, saKey2, &sa2).Should(Succeed())
		})

		AfterAll(func() {
			deleteAllIstioRevisionTags(ctx)
			deleteAllIstiosAndRevisions(ctx)
			Step("Checking if istio-reader ServiceAccount was deleted")
			Eventually(k8sClient.Get).WithArguments(ctx, saKey2, &sa2).Should(ReturnNotFoundError())
		})
	})
})
