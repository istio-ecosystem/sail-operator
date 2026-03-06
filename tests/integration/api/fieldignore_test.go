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

var _ = Describe("FieldIgnore rules", Label("fieldignore"), Ordered, func() {
	const (
		pilotImage = "sail-operator/test:latest"
	)

	SetDefaultEventuallyPollingInterval(time.Second)
	SetDefaultEventuallyTimeout(30 * time.Second)

	ctx := context.Background()

	istioNamespace := "fieldignore-test"
	revName := "fieldignore-rev"
	mutatingWebhookName := "istio-sidecar-injector-" + revName + "-" + istioNamespace
	validatingWebhookName := fmt.Sprintf("istio-validator-%s-%s", revName, istioNamespace)
	saName := "istiod-" + revName

	rev := &v1.IstioRevision{}

	BeforeAll(func() {
		Step("Creating namespace")
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: istioNamespace}}
		Expect(k8sClient.Create(ctx, ns)).To(Succeed())

		Step("Creating IstioRevision")
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

		Step("Waiting for webhook configurations and ServiceAccount to be created")
		Eventually(k8sClient.Get).WithArguments(ctx,
			client.ObjectKey{Name: mutatingWebhookName},
			&admissionv1.MutatingWebhookConfiguration{}).Should(Succeed())
		Eventually(k8sClient.Get).WithArguments(ctx,
			client.ObjectKey{Name: validatingWebhookName},
			&admissionv1.ValidatingWebhookConfiguration{}).Should(Succeed())
		Eventually(k8sClient.Get).WithArguments(ctx,
			client.ObjectKey{Name: saName, Namespace: istioNamespace},
			&corev1.ServiceAccount{}).Should(Succeed())
	})

	AfterAll(func() {
		Step("Deleting IstioRevision")
		deleteAllIstioRevisions(ctx)

		Step("Deleting namespace")
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: istioNamespace}}
		Expect(k8sClient.Delete(ctx, ns)).To(Succeed())
	})

	Describe("Helm post-rendering on initial install", func() {
		It("sets failurePolicy on ValidatingWebhookConfiguration (Scope=ReconcileAndUpgrade)", func() {
			webhook := &admissionv1.ValidatingWebhookConfiguration{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: validatingWebhookName}, webhook)).To(Succeed())
			Expect(webhook.Webhooks).ToNot(BeEmpty())
			Expect(webhook.Webhooks[0].FailurePolicy).ToNot(BeNil(),
				"failurePolicy should be set on initial install because ReconcileAndUpgrade rules are not applied on install")
		})

		It("strips caBundle from ValidatingWebhookConfiguration (Scope=Always)", func() {
			webhook := &admissionv1.ValidatingWebhookConfiguration{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: validatingWebhookName}, webhook)).To(Succeed())
			Expect(webhook.Webhooks).ToNot(BeEmpty())
			Expect(webhook.Webhooks[0].ClientConfig.CABundle).To(BeEmpty(),
				"caBundle should be stripped on initial install because the rule has Scope=Always")
		})

		It("strips caBundle from MutatingWebhookConfiguration (Scope=Always)", func() {
			webhook := &admissionv1.MutatingWebhookConfiguration{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: mutatingWebhookName}, webhook)).To(Succeed())
			Expect(webhook.Webhooks).ToNot(BeEmpty())
			Expect(webhook.Webhooks[0].ClientConfig.CABundle).To(BeEmpty(),
				"caBundle should be stripped on initial install because the rule has Scope=Always")
		})
	})

	// ── Predicate: reconciliation skipping ───────────────────────────────

	Describe("predicate filtering", func() {
		It("skips reconciliation when only caBundle changes on MutatingWebhookConfiguration", func() {
			waitForInFlightReconcileToFinish()

			webhook := &admissionv1.MutatingWebhookConfiguration{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: mutatingWebhookName}, webhook)).To(Succeed())

			expectNoReconciliation(istioRevisionController, func() {
				for i := range webhook.Webhooks {
					webhook.Webhooks[i].ClientConfig.CABundle = []byte("injected-ca-bundle")
				}
				Expect(k8sClient.Update(ctx, webhook)).To(Succeed())
			})
		})

		It("skips reconciliation when only caBundle changes on ValidatingWebhookConfiguration", func() {
			waitForInFlightReconcileToFinish()

			webhook := &admissionv1.ValidatingWebhookConfiguration{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: validatingWebhookName}, webhook)).To(Succeed())

			expectNoReconciliation(istioRevisionController, func() {
				for i := range webhook.Webhooks {
					webhook.Webhooks[i].ClientConfig.CABundle = []byte("injected-ca-bundle")
				}
				Expect(k8sClient.Update(ctx, webhook)).To(Succeed())
			})
		})

		It("skips reconciliation when only failurePolicy changes on ValidatingWebhookConfiguration", func() {
			waitForInFlightReconcileToFinish()

			webhook := &admissionv1.ValidatingWebhookConfiguration{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: validatingWebhookName}, webhook)).To(Succeed())

			expectNoReconciliation(istioRevisionController, func() {
				for i := range webhook.Webhooks {
					webhook.Webhooks[i].FailurePolicy = ptr.Of(admissionv1.Ignore)
				}
				Expect(k8sClient.Update(ctx, webhook)).To(Succeed())
			})
		})

		It("skips reconciliation when both caBundle and failurePolicy change on ValidatingWebhookConfiguration", func() {
			waitForInFlightReconcileToFinish()

			webhook := &admissionv1.ValidatingWebhookConfiguration{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: validatingWebhookName}, webhook)).To(Succeed())

			expectNoReconciliation(istioRevisionController, func() {
				for i := range webhook.Webhooks {
					webhook.Webhooks[i].ClientConfig.CABundle = []byte("another-ca-bundle")
					webhook.Webhooks[i].FailurePolicy = ptr.Of(admissionv1.Fail)
				}
				Expect(k8sClient.Update(ctx, webhook)).To(Succeed())
			})
		})

		It("skips reconciliation when Azure matchExpression is added to MutatingWebhookConfiguration", func() {
			waitForInFlightReconcileToFinish()

			webhook := &admissionv1.MutatingWebhookConfiguration{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: mutatingWebhookName}, webhook)).To(Succeed())

			expectNoReconciliation(istioRevisionController, func() {
				for i := range webhook.Webhooks {
					if webhook.Webhooks[i].NamespaceSelector == nil {
						webhook.Webhooks[i].NamespaceSelector = &metav1.LabelSelector{}
					}
					webhook.Webhooks[i].NamespaceSelector.MatchExpressions = append(
						webhook.Webhooks[i].NamespaceSelector.MatchExpressions,
						metav1.LabelSelectorRequirement{
							Key:      "kubernetes.azure.com/managedby",
							Operator: metav1.LabelSelectorOpNotIn,
							Values:   []string{"aks"},
						},
					)
				}
				Expect(k8sClient.Update(ctx, webhook)).To(Succeed())
			})
		})

		It("skips reconciliation when imagePullSecrets changes on ServiceAccount", func() {
			waitForInFlightReconcileToFinish()

			sa := &corev1.ServiceAccount{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: saName, Namespace: istioNamespace}, sa)).To(Succeed())

			expectNoReconciliation(istioRevisionController, func() {
				sa.ImagePullSecrets = append(sa.ImagePullSecrets, corev1.LocalObjectReference{Name: "injected-pull-secret"})
				Expect(k8sClient.Update(ctx, sa)).To(Succeed())
			})
		})

		It("skips reconciliation when automountServiceAccountToken changes on ServiceAccount", func() {
			waitForInFlightReconcileToFinish()

			sa := &corev1.ServiceAccount{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: saName, Namespace: istioNamespace}, sa)).To(Succeed())

			expectNoReconciliation(istioRevisionController, func() {
				sa.AutomountServiceAccountToken = ptr.Of(false)
				Expect(k8sClient.Update(ctx, sa)).To(Succeed())
			})
		})

		It("skips reconciliation when secrets changes on ServiceAccount", func() {
			waitForInFlightReconcileToFinish()

			sa := &corev1.ServiceAccount{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: saName, Namespace: istioNamespace}, sa)).To(Succeed())

			expectNoReconciliation(istioRevisionController, func() {
				sa.Secrets = append(sa.Secrets, corev1.ObjectReference{Name: "auto-token-xyz"})
				Expect(k8sClient.Update(ctx, sa)).To(Succeed())
			})
		})

		It("triggers reconciliation when a non-ignored field changes on MutatingWebhookConfiguration", func() {
			waitForInFlightReconcileToFinish()

			webhook := &admissionv1.MutatingWebhookConfiguration{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: mutatingWebhookName}, webhook)).To(Succeed())
			originalName := webhook.Webhooks[0].Name

			webhook.Webhooks[0].Name = "tampered.webhook.test"
			Expect(k8sClient.Update(ctx, webhook)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: mutatingWebhookName}, webhook)).To(Succeed())
				g.Expect(webhook.Webhooks[0].Name).To(Equal(originalName))
			}).Should(Succeed(), "non-ignored field should be reverted by reconciliation")
		})
	})

	Describe("Helm post-rendering on upgrade", func() {
		It("preserves in-cluster failurePolicy after reconciliation", func() {
			Step("Setting failurePolicy to a custom value in-cluster")
			webhook := &admissionv1.ValidatingWebhookConfiguration{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: validatingWebhookName}, webhook)).To(Succeed())
			for i := range webhook.Webhooks {
				webhook.Webhooks[i].FailurePolicy = ptr.Of(admissionv1.Ignore)
			}
			Expect(k8sClient.Update(ctx, webhook)).To(Succeed())

			Step("Triggering reconciliation by modifying a non-ignored field")
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: validatingWebhookName}, webhook)).To(Succeed())
			webhook.Labels["app"] = "tampered"
			Expect(k8sClient.Update(ctx, webhook)).To(Succeed())

			Step("Verifying label is reverted but failurePolicy is preserved")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: validatingWebhookName}, webhook)).To(Succeed())
				g.Expect(webhook.Labels["app"]).To(Equal("istiod"),
					"non-ignored field should be reverted")
				g.Expect(webhook.Webhooks[0].FailurePolicy).To(HaveValue(Equal(admissionv1.Ignore)),
					"failurePolicy should be preserved because Scope=ReconcileAndUpgrade strips it from Helm output on upgrade")
			}).Should(Succeed())
		})

		It("preserves in-cluster caBundle on MutatingWebhookConfiguration after reconciliation", func() {
			Step("Setting caBundle in-cluster")
			webhook := &admissionv1.MutatingWebhookConfiguration{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: mutatingWebhookName}, webhook)).To(Succeed())
			for i := range webhook.Webhooks {
				webhook.Webhooks[i].ClientConfig.CABundle = []byte("istiod-injected-ca")
			}
			Expect(k8sClient.Update(ctx, webhook)).To(Succeed())

			Step("Triggering reconciliation by modifying a non-ignored field")
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: mutatingWebhookName}, webhook)).To(Succeed())
			webhook.Labels["app"] = "tampered"
			Expect(k8sClient.Update(ctx, webhook)).To(Succeed())

			Step("Verifying label is reverted but caBundle is preserved")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: mutatingWebhookName}, webhook)).To(Succeed())
				g.Expect(webhook.Labels["app"]).To(Equal("sidecar-injector"),
					"non-ignored field should be reverted")
				g.Expect(webhook.Webhooks[0].ClientConfig.CABundle).To(Equal([]byte("istiod-injected-ca")),
					"caBundle should be preserved because the rule strips it from Helm output")
			}).Should(Succeed())
		})

		It("preserves in-cluster caBundle on ValidatingWebhookConfiguration after reconciliation", func() {
			Step("Setting caBundle in-cluster")
			webhook := &admissionv1.ValidatingWebhookConfiguration{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: validatingWebhookName}, webhook)).To(Succeed())
			for i := range webhook.Webhooks {
				webhook.Webhooks[i].ClientConfig.CABundle = []byte("istiod-injected-ca")
			}
			Expect(k8sClient.Update(ctx, webhook)).To(Succeed())

			Step("Triggering reconciliation by modifying a non-ignored field")
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: validatingWebhookName}, webhook)).To(Succeed())
			webhook.Labels["app"] = "tampered"
			Expect(k8sClient.Update(ctx, webhook)).To(Succeed())

			Step("Verifying label is reverted but caBundle is preserved")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: validatingWebhookName}, webhook)).To(Succeed())
				g.Expect(webhook.Labels["app"]).To(Equal("istiod"),
					"non-ignored field should be reverted")
				g.Expect(webhook.Webhooks[0].ClientConfig.CABundle).To(Equal([]byte("istiod-injected-ca")),
					"caBundle should be preserved because the rule strips it from Helm output")
			}).Should(Succeed())
		})

		It("preserves in-cluster imagePullSecrets on ServiceAccount after reconciliation", func() {
			Step("Setting imagePullSecrets in-cluster")
			sa := &corev1.ServiceAccount{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: saName, Namespace: istioNamespace}, sa)).To(Succeed())
			sa.ImagePullSecrets = append(sa.ImagePullSecrets, corev1.LocalObjectReference{Name: "preserved-pull-secret"})
			Expect(k8sClient.Update(ctx, sa)).To(Succeed())

			Step("Triggering reconciliation by modifying a non-ignored field (label)")
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: saName, Namespace: istioNamespace}, sa)).To(Succeed())
			sa.Labels["app"] = "tampered"
			Expect(k8sClient.Update(ctx, sa)).To(Succeed())

			Step("Verifying label is reverted but imagePullSecrets is preserved")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: saName, Namespace: istioNamespace}, sa)).To(Succeed())
				g.Expect(sa.Labels["app"]).To(Equal("istiod"),
					"non-ignored label should be reverted by reconciliation")
				g.Expect(sa.ImagePullSecrets).To(ContainElement(corev1.LocalObjectReference{Name: "preserved-pull-secret"}),
					"imagePullSecrets should be preserved because the rule strips it from Helm output")
			}).Should(Succeed())
		})

		It("preserves in-cluster automountServiceAccountToken on ServiceAccount after reconciliation", func() {
			Step("Setting automountServiceAccountToken in-cluster")
			sa := &corev1.ServiceAccount{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: saName, Namespace: istioNamespace}, sa)).To(Succeed())
			sa.AutomountServiceAccountToken = ptr.Of(false)
			Expect(k8sClient.Update(ctx, sa)).To(Succeed())

			Step("Triggering reconciliation by modifying a non-ignored field (label)")
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: saName, Namespace: istioNamespace}, sa)).To(Succeed())
			sa.Labels["app"] = "tampered"
			Expect(k8sClient.Update(ctx, sa)).To(Succeed())

			Step("Verifying label is reverted but automountServiceAccountToken is preserved")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: saName, Namespace: istioNamespace}, sa)).To(Succeed())
				g.Expect(sa.Labels["app"]).To(Equal("istiod"),
					"non-ignored label should be reverted by reconciliation")
				g.Expect(sa.AutomountServiceAccountToken).To(HaveValue(Equal(false)),
					"automountServiceAccountToken should be preserved because the rule strips it from Helm output")
			}).Should(Succeed())
		})

		It("preserves in-cluster secrets on ServiceAccount after reconciliation", func() {
			Step("Setting secrets in-cluster")
			sa := &corev1.ServiceAccount{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: saName, Namespace: istioNamespace}, sa)).To(Succeed())
			sa.Secrets = append(sa.Secrets, corev1.ObjectReference{Name: "preserved-token-secret"})
			Expect(k8sClient.Update(ctx, sa)).To(Succeed())

			Step("Triggering reconciliation by modifying a non-ignored field (label)")
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: saName, Namespace: istioNamespace}, sa)).To(Succeed())
			sa.Labels["app"] = "tampered"
			Expect(k8sClient.Update(ctx, sa)).To(Succeed())

			Step("Verifying label is reverted but secrets is preserved")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: saName, Namespace: istioNamespace}, sa)).To(Succeed())
				g.Expect(sa.Labels["app"]).To(Equal("istiod"),
					"non-ignored label should be reverted by reconciliation")
				g.Expect(sa.Secrets).To(ContainElement(corev1.ObjectReference{Name: "preserved-token-secret"}),
					"secrets should be preserved because the rule strips it from Helm output")
			}).Should(Succeed())
		})
	})
})
