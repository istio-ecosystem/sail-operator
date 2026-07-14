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

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/controllers/webhook"
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
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
						Path:      ptr.To("/inject"),
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

var _ = Describe("Remote istiod webhook (DNS-based URL) failure detection", Label("operator", "webhook-remote-url"), Ordered, func() {
	SetDefaultEventuallyTimeout(120 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)

	const (
		testNS  = "webhook-remote-url-test"
		revName = "remote-url-test-rev"
		// Must equal injectionWebhookKey(rev), or the revision's Ready check won't find this config.
		webhookCfgName    = "istio-sidecar-injector-" + testNS
		webhookName       = "sidecar-injector.istio.io"
		triggerDeployment = "test-trigger-url"
	)
	// Points at a service that does not exist, so every webhook call fails.
	webhookURL := "https://istiod-remote." + testNS + ".svc:15017/mutate-istio.io"

	BeforeAll(func(ctx SpecContext) {
		// Use a short webhook degraded window so the recovery step is fast.
		opDeploy := &appsv1.Deployment{}
		Expect(cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: deploymentName}, opDeploy)).To(Succeed())
		for i := range opDeploy.Spec.Template.Spec.Containers {
			c := &opDeploy.Spec.Template.Spec.Containers[i]
			if c.Name == "sail-operator" {
				c.Env = append(c.Env, corev1.EnvVar{Name: "WEBHOOK_DEGRADED_WINDOW", Value: "10s"})
			}
		}
		Expect(cl.Update(ctx, opDeploy)).To(Succeed())
		Eventually(func(g Gomega) {
			g.Expect(cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: deploymentName}, opDeploy)).To(Succeed())
			g.Expect(opDeploy.Status.ObservedGeneration).To(Equal(opDeploy.Generation), "operator rollout not observed yet")
			g.Expect(opDeploy.Status.AvailableReplicas).To(BeNumerically(">=", *opDeploy.Spec.Replicas), "operator not available yet")
		}).Should(Succeed())
		Log("Set a short webhook degraded window on the operator")

		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNS, Labels: map[string]string{testNS: "true"}}}
		Expect(cl.Create(ctx, ns)).To(Succeed())
		Log("Created test namespace", testNS)

		istio := &v1.Istio{
			ObjectMeta: metav1.ObjectMeta{Name: revName},
			Spec: v1.IstioSpec{
				Version:   istioversion.Default,
				Namespace: testNS,
				Profile:   "remote",
				Values: &v1.Values{
					Global: &v1.GlobalConfig{
						// So the istio chart does not create a conflicting MutatingWebhookConfiguration.
						OperatorManageWebhooks: ptr.To(true),
					},
				},
			},
		}
		Expect(cl.Create(ctx, istio)).To(Succeed())
		Log("Created remote-profile Istio", revName)

		// The Istio's InPlace update strategy names the IstioRevision it creates after the Istio itself.
		rev := &v1.IstioRevision{}
		Eventually(func(g Gomega) {
			g.Expect(cl.Get(ctx, client.ObjectKey{Name: revName}, rev)).To(Succeed())
		}).Should(Succeed())
		Log("IstioRevision created for remote-profile Istio", revName)

		sideEffects := admissionv1.SideEffectClassNone
		failPolicy := admissionv1.Fail
		whCfg := &admissionv1.MutatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name: webhookCfgName,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion:         v1.GroupVersion.String(),
					Kind:               v1.IstioRevisionKind,
					Name:               revName,
					UID:                rev.UID,
					Controller:         ptr.To(true),
					BlockOwnerDeletion: ptr.To(true),
				}},
			},
			Webhooks: []admissionv1.MutatingWebhook{{
				Name:                    webhookName,
				AdmissionReviewVersions: []string{"v1"},
				SideEffects:             &sideEffects,
				FailurePolicy:           &failPolicy,
				ClientConfig: admissionv1.WebhookClientConfig{
					URL:      ptr.To(webhookURL),
					CABundle: []byte("test-ca-bundle"),
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
					MatchLabels: map[string]string{testNS: "true"},
				},
			}},
		}
		Expect(cl.Create(ctx, whCfg)).To(Succeed())
		Success("Created MutatingWebhookConfiguration with a DNS-based URL, owned by the remote IstioRevision")

		DeferCleanup(func(ctx SpecContext) {
			whCfg := &admissionv1.MutatingWebhookConfiguration{ObjectMeta: metav1.ObjectMeta{Name: webhookCfgName}}
			_ = cl.Delete(ctx, whCfg)
			istio := &v1.Istio{ObjectMeta: metav1.ObjectMeta{Name: revName}}
			_ = cl.Delete(ctx, istio)
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNS}}
			_ = cl.Delete(ctx, ns)
		})
	})

	It("marks the webhook ready when a DNS-based URL and caBundle are configured", func(ctx SpecContext) {
		Eventually(func(g Gomega) {
			whCfg := &admissionv1.MutatingWebhookConfiguration{}
			g.Expect(cl.Get(ctx, client.ObjectKey{Name: webhookCfgName}, whCfg)).To(Succeed())
			g.Expect(whCfg.Annotations[constants.WebhookReadinessStatusAnnotationKey]).To(Equal("true"),
				"webhook with a DNS-based URL and caBundle should be ready")
		}).Should(Succeed())
		Success("Webhook with DNS-based URL marked ready")
	})

	It("detects a webhook call failure from a cluster event and marks it not ready", func(ctx SpecContext) {
		replicas := int32(1)
		deploy := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      triggerDeployment,
				Namespace: testNS,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: &replicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": triggerDeployment},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{"app": triggerDeployment},
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
		Success("Created Deployment to trigger webhook call failures")

		Eventually(func(g Gomega) {
			whCfg := &admissionv1.MutatingWebhookConfiguration{}
			g.Expect(cl.Get(ctx, client.ObjectKey{Name: webhookCfgName}, whCfg)).To(Succeed())
			g.Expect(whCfg.Annotations[constants.WebhookReadinessStatusAnnotationKey]).To(Equal("false"),
				"webhook should be marked not ready after a call failure")
			g.Expect(whCfg.Annotations[constants.WebhookReadinessReasonAnnotationKey]).To(
				Equal("webhook call failures reported in cluster events"))
		}).Should(Succeed())
		Success("Webhook marked not ready after a call failure detected from a cluster event")

		Eventually(func(g Gomega) {
			rev := &v1.IstioRevision{}
			g.Expect(cl.Get(ctx, client.ObjectKey{Name: revName}, rev)).To(Succeed())
			cond := rev.Status.GetCondition(v1.IstioRevisionConditionReady)
			g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(cond.Reason).To(Equal(v1.IstioRevisionReasonRemoteIstiodNotReady))
		}).Should(Succeed())
		Success("Remote IstioRevision reports Ready=False (RemoteIstiodNotReady)")
	})

	It("recovers to ready after the degraded window expires", func(ctx SpecContext) {
		// Delete the Deployment first: while it is stuck, each re-attempted pod creation
		// re-records the failure (via the event count-increment update), resetting the window.
		deploy := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: triggerDeployment, Namespace: testNS}}
		Expect(cl.Delete(ctx, deploy)).To(Succeed())
		Success("Deleted the trigger Deployment to stop new failure events")

		Eventually(func(g Gomega) {
			whCfg := &admissionv1.MutatingWebhookConfiguration{}
			g.Expect(cl.Get(ctx, client.ObjectKey{Name: webhookCfgName}, whCfg)).To(Succeed())
			g.Expect(whCfg.Annotations[constants.WebhookReadinessStatusAnnotationKey]).To(Equal("true"),
				"webhook should recover to ready after the degraded window expires")
		}).WithTimeout(60 * time.Second).Should(Succeed())
		Success("Webhook recovered to ready after the degraded window expired")
	})
})
