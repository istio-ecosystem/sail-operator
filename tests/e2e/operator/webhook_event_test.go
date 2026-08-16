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

package operator

import (
	"time"

	"github.com/istio-ecosystem/sail-operator/controllers/webhook"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Webhook failure event detection", Label("operator", "webhook-event"), Ordered, func() {
	SetDefaultEventuallyTimeout(120 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)

	const (
		testNS            = "webhook-event-test"
		webhookName       = "test-webhook.sail-operator.io"
		webhookCfgName    = "test-webhook-cfg"
		triggerDeployment = "test-trigger"
	)

	BeforeAll(func(ctx SpecContext) {
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNS}}
		Expect(cl.Create(ctx, ns)).To(Succeed())
		Log("Created test namespace", testNS)

		DeferCleanup(func(ctx SpecContext) {
			// Delete webhook config first to avoid blocking namespace deletion
			whCfg := &admissionv1.MutatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Name: webhookCfgName}}
			_ = cl.Delete(ctx, whCfg)

			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNS}}
			_ = cl.Delete(ctx, ns)
		})
	})

	It("generates a real webhook failure event that matches our extraction code", func(ctx SpecContext) {
		sideEffects := admissionv1.SideEffectClassNone
		failPolicy := admissionv1.Fail

		whCfg := &admissionv1.MutatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{Name: webhookCfgName},
			Webhooks: []admissionv1.MutatingWebhook{{
				Name:                    webhookName,
				AdmissionReviewVersions: []string{"v1"},
				SideEffects:             &sideEffects,
				FailurePolicy:           &failPolicy,
				ClientConfig: admissionv1.WebhookClientConfig{
					Service: &admissionv1.ServiceReference{
						Name:      "nonexistent-service",
						Namespace: testNS,
						Path:      strPtr("/inject"),
					},
				},
				Rules: []admissionv1.RuleWithOperations{{
					Operations: []admissionv1.OperationType{admissionv1.Create},
					Rule: admissionv1.Rule{
						APIGroups:   []string{""},
						APIVersions: []string{"v1"},
						Resources:   []string{"pods"},
					},
				}},
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"webhook-event-test": "true"},
				},
			}},
		}

		Expect(cl.Create(ctx, whCfg)).To(Succeed())
		Success("Created MutatingWebhookConfiguration pointing to non-existent service")

		// Label the namespace to match the webhook's namespaceSelector
		ns := &corev1.Namespace{}
		Expect(cl.Get(ctx, client.ObjectKey{Name: testNS}, ns)).To(Succeed())
		if ns.Labels == nil {
			ns.Labels = make(map[string]string)
		}
		ns.Labels["webhook-event-test"] = "true"
		Expect(cl.Update(ctx, ns)).To(Succeed())

		// Create a Deployment to trigger the webhook failure
		replicas := int32(1)
		deploy := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      triggerDeployment,
				Namespace: testNS,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "test-trigger"},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": "test-trigger"},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:    "busybox",
							Image:   "busybox:latest",
							Command: []string{"sleep", "3600"},
						}},
					},
				},
			},
		}

		Expect(cl.Create(ctx, deploy)).To(Succeed())
		Success("Created Deployment to trigger webhook failure events")

		// Wait for a Warning event containing "failed calling webhook" to appear.
		// The ReplicaSet controller generates this when pod creation fails due to an unreachable webhook.
		var matchedEvent *corev1.Event
		Eventually(func(g Gomega) {
			events := &corev1.EventList{}
			g.Expect(cl.List(ctx, events, client.InNamespace(testNS))).To(Succeed())

			for i := range events.Items {
				evt := &events.Items[i]
				if evt.Type == corev1.EventTypeWarning && webhook.ExtractWebhookName(evt.Message) != "" {
					matchedEvent = evt
					return
				}
			}
			g.Expect(matchedEvent).NotTo(BeNil(), "no webhook failure event found yet")
		}).Should(Succeed())

		Log("Found webhook failure event:", matchedEvent.Message)

		// Verify our extraction code correctly parses the real event
		extractedName := webhook.ExtractWebhookName(matchedEvent.Message)
		Expect(extractedName).To(Equal(webhookName),
			"ExtractWebhookName should extract the webhook name from the real Kubernetes event")
		Success("Extraction code correctly matched real Kubernetes webhook failure event")
	})
})

func strPtr(s string) *string {
	return &s
}
