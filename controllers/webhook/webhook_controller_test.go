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

package webhook

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/go-logr/logr"
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"istio.io/istio/pkg/ptr"
)

var ctx = context.Background()

func TestReconcile(t *testing.T) {
	tests := []struct {
		name          string
		webhook       *admissionv1.MutatingWebhookConfiguration
		setup         func(r *Reconciler)
		interceptors  interceptor.Funcs
		expectRequeue bool
		expectErr     error
		expectStatus  string
		expectReason  string
	}{
		{
			name: "ready when caBundle is set",
			webhook: &admissionv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: "istio-sidecar-injector"},
				Webhooks: []admissionv1.MutatingWebhook{{
					ClientConfig: admissionv1.WebhookClientConfig{
						Service:  &admissionv1.ServiceReference{Name: "istiod", Namespace: "istio-system"},
						CABundle: []byte("ca-data"),
					},
				}},
			},
			expectStatus: "true",
		},
		{
			name: "not ready when caBundle is empty",
			webhook: &admissionv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: "istio-sidecar-injector"},
				Webhooks: []admissionv1.MutatingWebhook{{
					ClientConfig: admissionv1.WebhookClientConfig{
						Service: &admissionv1.ServiceReference{Name: "istiod", Namespace: "istio-system"},
					},
				}},
			},
			expectStatus: "false",
		},
		{
			name: "not ready when webhook configuration contains no webhooks",
			webhook: &admissionv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: "istio-sidecar-injector"},
			},
			expectStatus: "false",
			expectReason: "webhook configuration contains no webhooks",
		},
		{
			name: "update error",
			webhook: &admissionv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: "istio-sidecar-injector"},
				Webhooks: []admissionv1.MutatingWebhook{{
					ClientConfig: admissionv1.WebhookClientConfig{
						Service:  &admissionv1.ServiceReference{Name: "istiod", Namespace: "istio-system"},
						CABundle: []byte("ca-data"),
					},
				}},
			},
			interceptors: interceptor.Funcs{
				Update: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
					return errors.New("some error")
				},
			},
			expectErr:    errors.New("some error"),
			expectStatus: "",
		},
		{
			name: "not ready when recent failure event recorded",
			webhook: &admissionv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: "istio-sidecar-injector"},
				Webhooks: []admissionv1.MutatingWebhook{{
					ClientConfig: admissionv1.WebhookClientConfig{
						Service:  &admissionv1.ServiceReference{Name: "istiod", Namespace: "istio-system"},
						CABundle: []byte("ca-data"),
					},
				}},
			},
			setup: func(r *Reconciler) {
				r.recordFailure("istio-sidecar-injector")
			},
			expectRequeue: true,
			expectStatus:  "false",
			expectReason:  "webhook call failures reported in cluster events",
		},
		{
			name: "recovers after failure ages out",
			webhook: &admissionv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: "istio-sidecar-injector"},
				Webhooks: []admissionv1.MutatingWebhook{{
					ClientConfig: admissionv1.WebhookClientConfig{
						Service:  &admissionv1.ServiceReference{Name: "istiod", Namespace: "istio-system"},
						CABundle: []byte("ca-data"),
					},
				}},
			},
			setup: func(r *Reconciler) {
				r.mu.Lock()
				r.failureHistory["istio-sidecar-injector"] = time.Now().Add(-10 * time.Minute)
				r.mu.Unlock()
			},
			expectStatus: "true",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			cl := newFakeClientBuilder().
				WithObjects(tt.webhook).
				WithInterceptorFuncs(tt.interceptors).
				Build()
			r := NewReconciler(newReconcilerTestConfig(t), cl, scheme.Scheme)
			if tt.setup != nil {
				tt.setup(r)
			}

			result, err := r.Reconcile(ctx, tt.webhook)

			if tt.expectRequeue {
				g.Expect(result.RequeueAfter).To(BeNumerically(">", 0))
			} else {
				g.Expect(result.RequeueAfter).To(BeZero())
			}
			if tt.expectErr != nil {
				g.Expect(err).To(Equal(tt.expectErr))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}

			if tt.expectStatus != "" {
				g.Expect(cl.Get(ctx, kube.Key("istio-sidecar-injector"), tt.webhook)).To(Succeed())
				g.Expect(tt.webhook.Annotations[constants.WebhookReadinessStatusAnnotationKey]).To(Equal(tt.expectStatus))
			}
			if tt.expectReason != "" {
				g.Expect(tt.webhook.Annotations[constants.WebhookReadinessReasonAnnotationKey]).To(Equal(tt.expectReason))
			}
		})
	}
}

func TestReconcileDoesNotUpdateWhenAnnotationsAreCurrent(t *testing.T) {
	g := NewWithT(t)

	webhook := &admissionv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "istio-sidecar-injector",
			Annotations: map[string]string{
				constants.WebhookReadinessStatusAnnotationKey: "true",
				constants.WebhookReadinessReasonAnnotationKey: "",
			},
		},
		Webhooks: []admissionv1.MutatingWebhook{{
			ClientConfig: admissionv1.WebhookClientConfig{
				Service:  &admissionv1.ServiceReference{Name: "istiod", Namespace: "istio-system"},
				CABundle: []byte("ca-data"),
			},
		}},
	}

	cl := newFakeClientBuilder().WithObjects(webhook).Build()
	r := NewReconciler(newReconcilerTestConfig(t), cl, scheme.Scheme)

	before := &admissionv1.MutatingWebhookConfiguration{}
	g.Expect(cl.Get(ctx, kube.Key("istio-sidecar-injector"), before)).To(Succeed())

	_, err := r.Reconcile(ctx, webhook)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(cl.Get(ctx, kube.Key("istio-sidecar-injector"), webhook)).To(Succeed())
	g.Expect(webhook.ResourceVersion).To(Equal(before.ResourceVersion),
		"no update expected when the readiness annotations are already current")
}

func TestReconcileRemovesLegacyReadinessAnnotations(t *testing.T) {
	g := NewWithT(t)

	webhook := &admissionv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "istio-sidecar-injector",
			Annotations: map[string]string{
				"sailoperator.io/readinessProbe.status":         "false",
				"sailoperator.io/readinessProbe.reason":         "readiness probe failed",
				"sailoperator.io/readinessProbe.periodSeconds":  "10",
				"sailoperator.io/readinessProbe.timeoutSeconds": "5",
			},
		},
		Webhooks: []admissionv1.MutatingWebhook{{
			ClientConfig: admissionv1.WebhookClientConfig{
				Service:  &admissionv1.ServiceReference{Name: "istiod", Namespace: "istio-system"},
				CABundle: []byte("ca-data"),
			},
		}},
	}

	cl := newFakeClientBuilder().WithObjects(webhook).Build()
	r := NewReconciler(newReconcilerTestConfig(t), cl, scheme.Scheme)

	_, err := r.Reconcile(ctx, webhook)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(cl.Get(ctx, kube.Key("istio-sidecar-injector"), webhook)).To(Succeed())
	g.Expect(webhook.Annotations).ToNot(HaveKey("sailoperator.io/readinessProbe.status"))
	g.Expect(webhook.Annotations).ToNot(HaveKey("sailoperator.io/readinessProbe.reason"))
	g.Expect(webhook.Annotations).ToNot(HaveKey("sailoperator.io/readinessProbe.periodSeconds"))
	g.Expect(webhook.Annotations).ToNot(HaveKey("sailoperator.io/readinessProbe.timeoutSeconds"))
	g.Expect(webhook.Annotations[constants.WebhookReadinessStatusAnnotationKey]).To(Equal("true"))
}

func TestEvaluateReadiness(t *testing.T) {
	svc := admissionv1.ServiceReference{Name: "istiod", Namespace: "istio-system"}

	tests := []struct {
		name           string
		webhookName    string
		clientConfigs  []admissionv1.WebhookClientConfig
		setup          func(r *Reconciler)
		expectedReady  bool
		expectedReason string
	}{
		{
			name:           "no webhooks",
			webhookName:    "test",
			clientConfigs:  []admissionv1.WebhookClientConfig{},
			expectedReady:  false,
			expectedReason: "webhook configuration contains no webhooks",
		},
		{
			name:           "no endpoint configured",
			webhookName:    "test",
			clientConfigs:  []admissionv1.WebhookClientConfig{{}},
			expectedReady:  false,
			expectedReason: "no endpoint configured in webhooks[].clientConfig",
		},
		{
			name:           "missing caBundle with service",
			webhookName:    "test",
			clientConfigs:  []admissionv1.WebhookClientConfig{{Service: &svc}},
			expectedReady:  false,
			expectedReason: "webhooks[].clientConfig.caBundle hasn't been set; check if the remote istiod can access this cluster",
		},
		{
			name:          "ready with service endpoint",
			webhookName:   "test",
			clientConfigs: []admissionv1.WebhookClientConfig{{Service: &svc, CABundle: []byte("ca")}},
			expectedReady: true,
		},
		{
			name:        "ready with URL endpoint",
			webhookName: "test",
			clientConfigs: []admissionv1.WebhookClientConfig{{
				URL:      ptr.Of("https://remote-istiod.example.com/inject"),
				CABundle: []byte("ca-data"),
			}},
			expectedReady: true,
		},
		{
			name:          "degraded due to recent failure event",
			webhookName:   "test",
			clientConfigs: []admissionv1.WebhookClientConfig{{Service: &svc, CABundle: []byte("ca")}},
			setup: func(r *Reconciler) {
				r.recordFailure("test")
			},
			expectedReady:  false,
			expectedReason: "webhook call failures reported in cluster events",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			r := NewReconciler(newReconcilerTestConfig(t), nil, scheme.Scheme)
			if tt.setup != nil {
				tt.setup(r)
			}
			result := r.evaluateReadiness(tt.webhookName, tt.clientConfigs)
			g.Expect(result.ready).To(Equal(tt.expectedReady))
			g.Expect(result.reason).To(Equal(tt.expectedReason))
		})
	}
}

func TestIsDegraded(t *testing.T) {
	g := NewWithT(t)
	r := NewReconciler(newReconcilerTestConfig(t), nil, scheme.Scheme)

	g.Expect(r.isDegraded("test")).To(BeZero(), "no failure recorded")

	// Recent failure: still within the degraded window
	r.recordFailure("test")
	g.Expect(r.isDegraded("test")).To(BeNumerically(">", 0), "recent single failure")

	// Failure older than the degraded window has aged out
	r.mu.Lock()
	r.failureHistory["test"] = time.Now().Add(-(DefaultDegradedWindow + time.Minute))
	r.mu.Unlock()
	g.Expect(r.isDegraded("test")).To(BeZero(), "failure aged out")

	// Verify the entry was cleaned up
	r.mu.Lock()
	_, exists := r.failureHistory["test"]
	r.mu.Unlock()
	g.Expect(exists).To(BeFalse(), "expired entry removed from map")

	// Failure within the degraded window is still degraded
	r.mu.Lock()
	r.failureHistory["test"] = time.Now().Add(-(DefaultDegradedWindow - time.Minute))
	r.mu.Unlock()
	g.Expect(r.isDegraded("test")).To(BeNumerically(">", 0), "still within the degraded window")
}

func TestPruneStaleFailures(t *testing.T) {
	tests := []struct {
		name       string
		seed       map[string]time.Duration // name -> age of the entry (how far in the past it was recorded)
		wantPruned int
		wantKept   []string
	}{
		{
			name:       "empty map is a no-op",
			seed:       map[string]time.Duration{},
			wantPruned: 0,
			wantKept:   nil,
		},
		{
			name:       "all fresh entries are kept",
			seed:       map[string]time.Duration{"a": 0, "b": time.Minute},
			wantPruned: 0,
			wantKept:   []string{"a", "b"},
		},
		{
			name:       "all stale entries are pruned",
			seed:       map[string]time.Duration{"a": DefaultDegradedWindow + time.Minute, "b": DefaultDegradedWindow + 2*time.Minute},
			wantPruned: 2,
			wantKept:   nil,
		},
		{
			name:       "entries at the window boundary are pruned",
			seed:       map[string]time.Duration{"a": DefaultDegradedWindow},
			wantPruned: 1,
			wantKept:   nil,
		},
		{
			name:       "mixed entries: only those within the window survive",
			seed:       map[string]time.Duration{"stale": DefaultDegradedWindow + time.Minute, "boundary": DefaultDegradedWindow, "fresh": time.Minute},
			wantPruned: 2,
			wantKept:   []string{"fresh"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			r := NewReconciler(newReconcilerTestConfig(t), nil, scheme.Scheme)

			now := time.Now()
			r.mu.Lock()
			for name, age := range tt.seed {
				r.failureHistory[name] = now.Add(-age)
			}
			r.mu.Unlock()

			g.Expect(r.pruneStaleFailures()).To(Equal(tt.wantPruned))

			r.mu.Lock()
			defer r.mu.Unlock()
			for name := range r.failureHistory {
				g.Expect(tt.wantKept).To(ContainElement(name), "unexpected entry survived: %s", name)
			}
			for _, name := range tt.wantKept {
				g.Expect(r.failureHistory).To(HaveKey(name), "expected entry to survive: %s", name)
			}
		})
	}
}

func TestFailureHistoryJanitor(t *testing.T) {
	tests := []struct {
		name     string
		seed     map[string]time.Duration // name -> age of the entry
		wantKept []string
	}{
		{
			name:     "prunes stale entries and keeps fresh ones",
			seed:     map[string]time.Duration{"stale": DefaultDegradedWindow + time.Minute, "fresh": 0},
			wantKept: []string{"fresh"},
		},
		{
			name:     "no-op when all entries are fresh",
			seed:     map[string]time.Duration{"a": 0, "b": time.Minute},
			wantKept: []string{"a", "b"},
		},
		{
			name:     "no-op on empty map",
			seed:     map[string]time.Duration{},
			wantKept: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			r := NewReconciler(newReconcilerTestConfig(t), nil, scheme.Scheme)

			now := time.Now()
			r.mu.Lock()
			for name, age := range tt.seed {
				r.failureHistory[name] = now.Add(-age)
			}
			r.mu.Unlock()

			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan struct{})
			go func() {
				defer close(done)
				_ = r.failureHistoryJanitor(logr.Discard(), 10*time.Millisecond).Start(ctx)
			}()

			g.Eventually(func() bool {
				r.mu.Lock()
				defer r.mu.Unlock()
				if len(r.failureHistory) != len(tt.wantKept) {
					return false
				}
				for _, name := range tt.wantKept {
					if _, ok := r.failureHistory[name]; !ok {
						return false
					}
				}
				return true
			}).Should(BeTrue(), "janitor should leave exactly the entries within the window")

			cancel()
			<-done
		})
	}
}

func TestExtractWebhookName(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected string
	}{
		{
			name: "replicaset controller event with connection refused",
			message: `Error creating: Internal error occurred: failed calling webhook "sidecar-injector.istio.io": ` +
				`Post "https://istiod.istio-system.svc:443/inject": dial tcp: connect: connection refused`,
			expected: "sidecar-injector.istio.io",
		},
		{
			name: "replicaset controller event with no endpoints",
			message: `Error creating: Internal error occurred: failed calling webhook "sidecar-injector.istio.io": ` +
				`Post "https://istiod.istio-system.svc:443/inject": no endpoints available for service "istiod"`,
			expected: "sidecar-injector.istio.io",
		},
		{
			name: "webhook with no further details",
			message: `Error creating: Internal error occurred: failed calling webhook "sidecar-injector.istio.io"; ` +
				`no further details available`,
			expected: "sidecar-injector.istio.io",
		},
		{
			name:     "no match",
			message:  "some other event message",
			expected: "",
		},
		{
			name:     "empty message",
			message:  "",
			expected: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(ExtractWebhookName(tt.message)).To(Equal(tt.expected))
		})
	}
}

func TestWebhookFailureEventPredicate(t *testing.T) {
	pred := webhookFailureEventPredicate()
	tests := []struct {
		name     string
		obj      client.Object
		expected bool
	}{
		{
			name: "matching event with webhook failure message",
			obj: &corev1.Event{
				Type:    corev1.EventTypeWarning,
				Reason:  "FailedCreate",
				Message: `Error creating: Internal error occurred: failed calling webhook "sidecar-injector.istio.io": connection refused`,
			},
			expected: true,
		},
		{
			name: "warning event without webhook failure",
			obj: &corev1.Event{
				Type:    corev1.EventTypeWarning,
				Reason:  "FailedCreate",
				Message: "Error creating: some other error",
			},
			expected: false,
		},
		{
			name: "normal event with webhook text",
			obj: &corev1.Event{
				Type:    corev1.EventTypeNormal,
				Message: `failed calling webhook "sidecar-injector.istio.io"`,
			},
			expected: false,
		},
		{
			name:     "not an event",
			obj:      &admissionv1.MutatingWebhookConfiguration{},
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(pred.Create(event.CreateEvent{Object: tt.obj})).To(Equal(tt.expected))
		})
	}

	failureEvent := &corev1.Event{
		Type:    corev1.EventTypeWarning,
		Reason:  "FailedCreate",
		Message: `Error creating: Internal error occurred: failed calling webhook "sidecar-injector.istio.io": connection refused`,
	}
	t.Run("update matches on the new object", func(t *testing.T) {
		g := NewWithT(t)
		g.Expect(pred.Update(event.UpdateEvent{ObjectNew: failureEvent})).To(BeTrue())
		g.Expect(pred.Update(event.UpdateEvent{
			ObjectNew: &corev1.Event{Type: corev1.EventTypeWarning, Message: "Error creating: some other error"},
		})).To(BeFalse())
	})
	t.Run("delete is ignored", func(t *testing.T) {
		g := NewWithT(t)
		g.Expect(pred.Delete(event.DeleteEvent{Object: failureEvent})).To(BeFalse())
	})
	t.Run("generic is ignored", func(t *testing.T) {
		g := NewWithT(t)
		g.Expect(pred.Generic(event.GenericEvent{Object: failureEvent})).To(BeFalse())
	})
}

func testOwnedByRevisionRef(revisionName string) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: v1.GroupVersion.String(),
		Kind:       v1.IstioRevisionKind,
		Name:       revisionName,
		UID:        types.UID(revisionName + "-uid"),
	}
}

func testRemoteRevision(name string) *v1.IstioRevision {
	return &v1.IstioRevision{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1.IstioRevisionSpec{
			Values: &v1.Values{Profile: ptr.Of("remote")},
		},
	}
}

func TestMapFailureEventToWebhook(t *testing.T) {
	mutatingConfig := &admissionv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "istio-sidecar-injector",
			OwnerReferences: []metav1.OwnerReference{testOwnedByRevisionRef("test-revision")},
		},
		Webhooks: []admissionv1.MutatingWebhook{{
			Name: "sidecar-injector.istio.io",
			ClientConfig: admissionv1.WebhookClientConfig{
				Service: &admissionv1.ServiceReference{Name: "istiod", Namespace: "istio-system"},
			},
		}},
	}
	unownedConfig := &admissionv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: "unowned-sidecar-injector"},
		Webhooks: []admissionv1.MutatingWebhook{{
			Name: "unowned.istio.io",
		}},
	}
	localConfig := &admissionv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "local-sidecar-injector",
			OwnerReferences: []metav1.OwnerReference{testOwnedByRevisionRef("local-revision")},
		},
		Webhooks: []admissionv1.MutatingWebhook{{
			Name: "local.istio.io",
		}},
	}

	tests := []struct {
		name         string
		obj          client.Object
		expectedReqs []reconcile.Request
		expectFailed bool
	}{
		{
			name: "maps event to webhook config and records failure",
			obj: &corev1.Event{
				ObjectMeta: metav1.ObjectMeta{Name: "evt1", Namespace: "default"},
				Message: `Error creating: Internal error occurred: failed calling webhook "sidecar-injector.istio.io": ` +
					`Post "https://istiod.istio-system.svc:443/inject": dial tcp: connect: connection refused`,
			},
			expectedReqs: []reconcile.Request{{NamespacedName: types.NamespacedName{Name: "istio-sidecar-injector"}}},
			expectFailed: true,
		},
		{
			name: "ignores webhook in config that is not owned by an IstioRevision",
			obj: &corev1.Event{
				ObjectMeta: metav1.ObjectMeta{Name: "evt2", Namespace: "default"},
				Message:    `Error creating: Internal error occurred: failed calling webhook "unowned.istio.io": connection refused`,
			},
		},
		{
			name: "ignores webhook in config owned by a non-remote IstioRevision",
			obj: &corev1.Event{
				ObjectMeta: metav1.ObjectMeta{Name: "evt3", Namespace: "default"},
				Message:    `Error creating: Internal error occurred: failed calling webhook "local.istio.io": connection refused`,
			},
		},
		{
			name: "ignores webhook name not found in any config",
			obj: &corev1.Event{
				ObjectMeta: metav1.ObjectMeta{Name: "evt4", Namespace: "default"},
				Message:    `Error creating: Internal error occurred: failed calling webhook "unknown-webhook.example.com": connection refused`,
			},
		},
		{
			name: "ignores unrelated message",
			obj: &corev1.Event{
				ObjectMeta: metav1.ObjectMeta{Name: "evt5", Namespace: "default"},
				Message:    "some unrelated message",
			},
		},
		{
			name: "ignores non-event",
			obj:  &admissionv1.MutatingWebhookConfiguration{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			cl := newFakeClientBuilder().WithObjects(
				mutatingConfig, unownedConfig, localConfig,
				testRemoteRevision("test-revision"),
				&v1.IstioRevision{ObjectMeta: metav1.ObjectMeta{Name: "local-revision"}},
			).Build()
			r := NewReconciler(newReconcilerTestConfig(t), cl, scheme.Scheme)

			reqs := r.mapFailureEventToWebhook(ctx, tt.obj)
			g.Expect(reqs).To(Equal(tt.expectedReqs))

			if tt.expectFailed {
				g.Expect(r.isDegraded("istio-sidecar-injector")).To(BeNumerically(">", 0))
			} else {
				g.Expect(r.isDegraded("unowned-sidecar-injector")).To(BeZero())
				g.Expect(r.isDegraded("local-sidecar-injector")).To(BeZero())
			}
		})
	}
}

func TestFindOwnedWebhookConfig(t *testing.T) {
	owned := testOwnedByRevisionRef("test-revision")
	configs := []client.Object{
		&admissionv1.MutatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "istio-sidecar-injector",
				OwnerReferences: []metav1.OwnerReference{owned},
			},
			Webhooks: []admissionv1.MutatingWebhook{
				{Name: "sidecar-injector.istio.io"},
				{Name: "namespace.sidecar-injector.istio.io"},
			},
		},
		&admissionv1.MutatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{Name: "third-party-injector"},
			Webhooks: []admissionv1.MutatingWebhook{
				{Name: "third-party.istio.io"},
			},
		},
	}

	tests := []struct {
		name        string
		webhookName string
		expected    string
	}{
		{
			name:        "finds owned config",
			webhookName: "sidecar-injector.istio.io",
			expected:    "istio-sidecar-injector",
		},
		{
			name:        "finds second webhook in owned config",
			webhookName: "namespace.sidecar-injector.istio.io",
			expected:    "istio-sidecar-injector",
		},
		{
			name:        "ignores webhook in config that is not owned by an IstioRevision",
			webhookName: "third-party.istio.io",
			expected:    "",
		},
		{
			name:        "returns empty for unknown webhook",
			webhookName: "unknown.webhook.io",
			expected:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			cl := newFakeClientBuilder().WithObjects(configs...).WithObjects(testRemoteRevision("test-revision")).Build()
			r := NewReconciler(newReconcilerTestConfig(t), cl, scheme.Scheme)
			g.Expect(r.findOwnedWebhookConfig(ctx, tt.webhookName)).To(Equal(tt.expected))
		})
	}
}

func TestIsOwnedByRevisionWithRemoteControlPlane(t *testing.T) {
	tests := []struct {
		name         string
		ownerRefs    []metav1.OwnerReference
		objects      []client.Object
		interceptors interceptor.Funcs
		expected     bool
	}{
		{
			name:      "No owner references",
			ownerRefs: []metav1.OwnerReference{},
			expected:  false,
		},
		{
			name: "Owner reference not IstioRevision",
			ownerRefs: []metav1.OwnerReference{{
				APIVersion: "someothergroup/v1",
				Kind:       "SomeKind",
			}},
			expected: false,
		},
		{
			name: "IstioRevision not found",
			ownerRefs: []metav1.OwnerReference{{
				APIVersion: v1.GroupVersion.String(),
				Kind:       v1.IstioRevisionKind,
				Name:       "revision1",
			}},
			expected: false,
		},
		{
			name: "IstioRevision fetch error",
			ownerRefs: []metav1.OwnerReference{{
				APIVersion: v1.GroupVersion.String(),
				Kind:       v1.IstioRevisionKind,
				Name:       "revision1",
			}},
			interceptors: interceptor.Funcs{
				Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					return errors.New("some error")
				},
			},
			expected: false,
		},
		{
			name: "IstioRevision not using remote profile",
			ownerRefs: []metav1.OwnerReference{{
				APIVersion: v1.GroupVersion.String(),
				Kind:       v1.IstioRevisionKind,
				Name:       "revision1",
			}},
			objects: []client.Object{
				&v1.IstioRevision{
					ObjectMeta: metav1.ObjectMeta{Name: "revision1"},
					Spec:       v1.IstioRevisionSpec{},
				},
			},
			expected: false,
		},
		{
			name: "IstioRevision uses remote profile",
			ownerRefs: []metav1.OwnerReference{{
				APIVersion: v1.GroupVersion.String(),
				Kind:       v1.IstioRevisionKind,
				Name:       "revision1",
			}},
			objects: []client.Object{
				&v1.IstioRevision{
					ObjectMeta: metav1.ObjectMeta{Name: "revision1"},
					Spec: v1.IstioRevisionSpec{
						Values: &v1.Values{Profile: ptr.Of("remote")},
					},
				},
			},
			expected: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			cl := newFakeClientBuilder().
				WithObjects(tt.objects...).
				WithInterceptorFuncs(tt.interceptors).
				Build()
			obj := &admissionv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{OwnerReferences: tt.ownerRefs},
			}
			g.Expect(IsOwnedByRevisionWithRemoteControlPlane(cl, obj)).To(Equal(tt.expected))
		})
	}
}

func newFakeClientBuilder() *fake.ClientBuilder {
	return fake.NewClientBuilder().WithScheme(scheme.Scheme)
}

func newReconcilerTestConfig(t *testing.T) config.ReconcilerConfig {
	return config.ReconcilerConfig{
		ResourceFS:              os.DirFS(t.TempDir()),
		Platform:                config.PlatformKubernetes,
		DefaultProfile:          "",
		MaxConcurrentReconciles: 1,
		WebhookDegradedWindow:   DefaultDegradedWindow,
	}
}
