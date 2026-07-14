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

func TestReconcileMutating(t *testing.T) {
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
				r.recordFailure("istio-sidecar-injector")
			},
			expectRequeue: true,
			expectStatus:  "false",
			expectReason:  "apiserver reported webhook call failures",
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
				r.failureHistory["istio-sidecar-injector"] = failureEntry{
					prev: time.Now().Add(-12 * time.Minute),
					last: time.Now().Add(-10 * time.Minute),
				}
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

			result, err := r.ReconcileMutating(ctx, tt.webhook)

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

func TestReconcileValidating(t *testing.T) {
	g := NewWithT(t)

	webhook := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: "istio-validator"},
		Webhooks: []admissionv1.ValidatingWebhook{{
			ClientConfig: admissionv1.WebhookClientConfig{
				Service:  &admissionv1.ServiceReference{Name: "istiod", Namespace: "istio-system"},
				CABundle: []byte("ca-data"),
			},
		}},
	}

	cl := newFakeClientBuilder().WithObjects(webhook).Build()
	r := NewReconciler(newReconcilerTestConfig(t), cl, scheme.Scheme)

	result, err := r.ReconcileValidating(ctx, webhook)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.RequeueAfter).To(BeZero())

	g.Expect(cl.Get(ctx, kube.Key("istio-validator"), webhook)).To(Succeed())
	g.Expect(webhook.Annotations[constants.WebhookReadinessStatusAnnotationKey]).To(Equal("true"))
}

func TestEvaluateReadiness(t *testing.T) {
	svc := admissionv1.ServiceReference{Name: "istiod", Namespace: "istio-system"}

	tests := []struct {
		name           string
		webhookName    string
		webhookCount   int
		cc             admissionv1.WebhookClientConfig
		setup          func(r *Reconciler)
		expectedReady  bool
		expectedReason string
	}{
		{
			name:           "no webhooks",
			webhookName:    "test",
			webhookCount:   0,
			cc:             admissionv1.WebhookClientConfig{},
			expectedReady:  false,
			expectedReason: "webhook configuration contains no webhooks",
		},
		{
			name:           "no endpoint configured",
			webhookName:    "test",
			webhookCount:   1,
			cc:             admissionv1.WebhookClientConfig{},
			expectedReady:  false,
			expectedReason: "no endpoint configured in webhooks[].clientConfig",
		},
		{
			name:           "missing caBundle with service",
			webhookName:    "test",
			webhookCount:   1,
			cc:             admissionv1.WebhookClientConfig{Service: &svc},
			expectedReady:  false,
			expectedReason: "webhooks[].clientConfig.caBundle hasn't been set; check if the remote istiod can access this cluster",
		},
		{
			name:          "ready with service endpoint",
			webhookName:   "test",
			webhookCount:  1,
			cc:            admissionv1.WebhookClientConfig{Service: &svc, CABundle: []byte("ca")},
			expectedReady: true,
		},
		{
			name:         "ready with URL endpoint",
			webhookName:  "test",
			webhookCount: 1,
			cc: admissionv1.WebhookClientConfig{
				URL:      ptr.Of("https://remote-istiod.example.com/inject"),
				CABundle: []byte("ca"),
			},
			expectedReady: true,
		},
		{
			name:         "degraded due to recent failure event",
			webhookName:  "test",
			webhookCount: 1,
			cc:           admissionv1.WebhookClientConfig{Service: &svc, CABundle: []byte("ca")},
			setup: func(r *Reconciler) {
				r.recordFailure("test")
				r.recordFailure("test")
			},
			expectedReady:  false,
			expectedReason: "apiserver reported webhook call failures",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			r := NewReconciler(newReconcilerTestConfig(t), nil, scheme.Scheme)
			if tt.setup != nil {
				tt.setup(r)
			}
			ready, reason, _ := r.evaluateReadiness(tt.webhookName, tt.webhookCount, tt.cc)
			g.Expect(ready).To(Equal(tt.expectedReady))
			g.Expect(reason).To(Equal(tt.expectedReason))
		})
	}
}

func TestIsDegraded(t *testing.T) {
	g := NewWithT(t)
	r := NewReconciler(newReconcilerTestConfig(t), nil, scheme.Scheme)

	g.Expect(r.isDegraded("test")).To(BeZero(), "no failure recorded")

	// Single failure: uses minFailureGap (30s) as the gap
	r.recordFailure("test")
	g.Expect(r.isDegraded("test")).To(BeNumerically(">", 0), "recent single failure")

	// Two failures 1 minute apart: gap=60s, clears after 2*60s=120s
	r.mu.Lock()
	r.failureHistory["test"] = failureEntry{
		prev: time.Now().Add(-5 * time.Minute),
		last: time.Now().Add(-4 * time.Minute),
	}
	r.mu.Unlock()
	g.Expect(r.isDegraded("test")).To(BeZero(), "failure aged out (4min > 2*60s)")

	// Verify the entry was cleaned up
	r.mu.Lock()
	_, exists := r.failureHistory["test"]
	r.mu.Unlock()
	g.Expect(exists).To(BeFalse(), "expired entry removed from map")

	// Two failures 5 minutes apart: gap=300s, still degraded within 2*300s window
	r.mu.Lock()
	r.failureHistory["test"] = failureEntry{
		prev: time.Now().Add(-6 * time.Minute),
		last: time.Now().Add(-1 * time.Minute),
	}
	r.mu.Unlock()
	g.Expect(r.isDegraded("test")).To(BeNumerically(">", 0), "still within 2*gap window")
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
}

func TestMapFailureEventToMutatingWebhook(t *testing.T) {
	mutatingConfig := &admissionv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: "istio-sidecar-injector"},
		Webhooks: []admissionv1.MutatingWebhook{{
			Name: "sidecar-injector.istio.io",
			ClientConfig: admissionv1.WebhookClientConfig{
				Service: &admissionv1.ServiceReference{Name: "istiod", Namespace: "istio-system"},
			},
		}},
	}
	validatingConfig := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: "istio-validator"},
		Webhooks: []admissionv1.ValidatingWebhook{{
			Name: "validation.istio.io",
			ClientConfig: admissionv1.WebhookClientConfig{
				Service: &admissionv1.ServiceReference{Name: "istiod", Namespace: "istio-system"},
			},
		}},
	}

	tests := []struct {
		name         string
		obj          client.Object
		expectedReqs []reconcile.Request
		expectFailed bool
	}{
		{
			name: "maps event to mutating webhook config and records failure",
			obj: &corev1.Event{
				ObjectMeta: metav1.ObjectMeta{Name: "evt1", Namespace: "default"},
				Message: `Error creating: Internal error occurred: failed calling webhook "sidecar-injector.istio.io": ` +
					`Post "https://istiod.istio-system.svc:443/inject": dial tcp: connect: connection refused`,
			},
			expectedReqs: []reconcile.Request{{NamespacedName: types.NamespacedName{Name: "istio-sidecar-injector"}}},
			expectFailed: true,
		},
		{
			name: "ignores validating webhook in mutating mapper",
			obj: &corev1.Event{
				ObjectMeta: metav1.ObjectMeta{Name: "evt2", Namespace: "default"},
				Message:    `Error creating: Internal error occurred: failed calling webhook "validation.istio.io": connection refused`,
			},
		},
		{
			name: "ignores webhook name not found in any config",
			obj: &corev1.Event{
				ObjectMeta: metav1.ObjectMeta{Name: "evt3", Namespace: "default"},
				Message:    `Error creating: Internal error occurred: failed calling webhook "unknown-webhook.example.com": connection refused`,
			},
		},
		{
			name: "ignores unrelated message",
			obj: &corev1.Event{
				ObjectMeta: metav1.ObjectMeta{Name: "evt4", Namespace: "default"},
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
			cl := newFakeClientBuilder().WithObjects(mutatingConfig, validatingConfig).Build()
			r := NewReconciler(newReconcilerTestConfig(t), cl, scheme.Scheme)

			reqs := r.mapFailureEventToMutatingWebhook(ctx, tt.obj)
			g.Expect(reqs).To(Equal(tt.expectedReqs))

			if tt.expectFailed {
				g.Expect(r.isDegraded("istio-sidecar-injector")).To(BeNumerically(">", 0))
			}
		})
	}
}

func TestMapFailureEventToValidatingWebhook(t *testing.T) {
	mutatingConfig := &admissionv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: "istio-sidecar-injector"},
		Webhooks: []admissionv1.MutatingWebhook{{
			Name: "sidecar-injector.istio.io",
		}},
	}
	validatingConfig := &admissionv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: "istio-validator"},
		Webhooks: []admissionv1.ValidatingWebhook{{
			Name: "validation.istio.io",
		}},
	}

	tests := []struct {
		name         string
		obj          client.Object
		expectedReqs []reconcile.Request
		expectFailed bool
	}{
		{
			name: "maps event to validating webhook config and records failure",
			obj: &corev1.Event{
				ObjectMeta: metav1.ObjectMeta{Name: "evt1", Namespace: "default"},
				Message:    `Error creating: Internal error occurred: failed calling webhook "validation.istio.io": connection refused`,
			},
			expectedReqs: []reconcile.Request{{NamespacedName: types.NamespacedName{Name: "istio-validator"}}},
			expectFailed: true,
		},
		{
			name: "ignores mutating webhook in validating mapper",
			obj: &corev1.Event{
				ObjectMeta: metav1.ObjectMeta{Name: "evt2", Namespace: "default"},
				Message: `Error creating: Internal error occurred: failed calling webhook "sidecar-injector.istio.io": ` +
					`Post "https://istiod.istio-system.svc:443/inject": connection refused`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			cl := newFakeClientBuilder().WithObjects(mutatingConfig, validatingConfig).Build()
			r := NewReconciler(newReconcilerTestConfig(t), cl, scheme.Scheme)

			reqs := r.mapFailureEventToValidatingWebhook(ctx, tt.obj)
			g.Expect(reqs).To(Equal(tt.expectedReqs))

			if tt.expectFailed {
				g.Expect(r.isDegraded("istio-validator")).To(BeNumerically(">", 0))
			}
		})
	}
}

func TestFindWebhookConfig(t *testing.T) {
	configs := []client.Object{
		&admissionv1.MutatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{Name: "istio-sidecar-injector"},
			Webhooks: []admissionv1.MutatingWebhook{
				{Name: "sidecar-injector.istio.io"},
				{Name: "namespace.sidecar-injector.istio.io"},
			},
		},
		&admissionv1.ValidatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{Name: "istio-validator"},
			Webhooks: []admissionv1.ValidatingWebhook{
				{Name: "validation.istio.io"},
			},
		},
	}

	tests := []struct {
		name        string
		webhookName string
		expected    *webhookConfigRef
	}{
		{
			name:        "finds mutating config",
			webhookName: "sidecar-injector.istio.io",
			expected:    &webhookConfigRef{name: "istio-sidecar-injector", isMutating: true},
		},
		{
			name:        "finds second webhook in mutating config",
			webhookName: "namespace.sidecar-injector.istio.io",
			expected:    &webhookConfigRef{name: "istio-sidecar-injector", isMutating: true},
		},
		{
			name:        "finds validating config",
			webhookName: "validation.istio.io",
			expected:    &webhookConfigRef{name: "istio-validator", isMutating: false},
		},
		{
			name:        "returns nil for unknown webhook",
			webhookName: "unknown.webhook.io",
			expected:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			cl := newFakeClientBuilder().WithObjects(configs...).Build()
			r := NewReconciler(newReconcilerTestConfig(t), cl, scheme.Scheme)
			g.Expect(r.findWebhookConfig(ctx, tt.webhookName)).To(Equal(tt.expected))
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
	}
}
