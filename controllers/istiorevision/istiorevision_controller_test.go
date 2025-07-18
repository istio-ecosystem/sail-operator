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

package istiorevision

import (
	"context"
	"fmt"
	"strings"
	"testing"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"istio.io/istio/pkg/ptr"
)

func TestValidate(t *testing.T) {
	cfg := newReconcilerTestConfig(t)

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "istio-system",
		},
	}

	testCases := []struct {
		name      string
		rev       *v1.IstioRevision
		objects   []client.Object
		expectErr string
	}{
		{
			name: "success",
			rev: &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1.IstioRevisionSpec{
					Version:   istioversion.Default,
					Namespace: "istio-system",
					Values: &v1.Values{
						Global: &v1.GlobalConfig{
							IstioNamespace: ptr.Of("istio-system"),
						},
					},
				},
			},
			objects:   []client.Object{ns},
			expectErr: "",
		},
		{
			name: "no version",
			rev: &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1.IstioRevisionSpec{
					Namespace: "istio-system",
				},
			},
			objects:   []client.Object{ns},
			expectErr: "spec.version not set",
		},
		{
			name: "no namespace",
			rev: &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1.IstioRevisionSpec{
					Version: istioversion.Default,
				},
			},
			objects:   []client.Object{ns},
			expectErr: "spec.namespace not set",
		},
		{
			name: "namespace not found",
			rev: &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1.IstioRevisionSpec{
					Version:   istioversion.Default,
					Namespace: "istio-system",
				},
			},
			objects:   []client.Object{},
			expectErr: `namespace "istio-system" doesn't exist`,
		},
		{
			name: "no values",
			rev: &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1.IstioRevisionSpec{
					Version:   istioversion.Default,
					Namespace: "istio-system",
				},
			},
			objects:   []client.Object{ns},
			expectErr: "spec.values not set",
		},
		{
			name: "invalid istioNamespace",
			rev: &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1.IstioRevisionSpec{
					Version:   istioversion.Default,
					Namespace: "istio-system",
					Values: &v1.Values{
						Global: &v1.GlobalConfig{
							IstioNamespace: ptr.Of("other-namespace"),
						},
					},
				},
			},
			objects:   []client.Object{ns},
			expectErr: "spec.values.global.istioNamespace does not match spec.namespace",
		},
		{
			name: "invalid revision default",
			rev: &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1.IstioRevisionSpec{
					Version:   istioversion.Default,
					Namespace: "istio-system",
					Values: &v1.Values{
						Revision: ptr.Of("my-revision"),
						Global: &v1.GlobalConfig{
							IstioNamespace: ptr.Of("other-namespace"),
						},
					},
				},
			},
			objects:   []client.Object{ns},
			expectErr: `spec.values.revision must be "" when IstioRevision name is default`,
		},
		{
			name: "invalid revision non-default",
			rev: &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-revision",
				},
				Spec: v1.IstioRevisionSpec{
					Version:   istioversion.Default,
					Namespace: "istio-system",
					Values: &v1.Values{
						Revision: ptr.Of("other-revision"),
						Global: &v1.GlobalConfig{
							IstioNamespace: ptr.Of("other-namespace"),
						},
					},
				},
			},
			objects:   []client.Object{ns},
			expectErr: `spec.values.revision does not match IstioRevision name`,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(tc.objects...).Build()
			r := NewReconciler(cfg, cl, scheme.Scheme, nil)

			err := r.validate(context.TODO(), tc.rev)
			if tc.expectErr == "" {
				g.Expect(err).ToNot(HaveOccurred())
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectErr))
			}
		})
	}
}

func TestMapEndpointSliceToReconcileRequests(t *testing.T) {
	testCases := []struct {
		endpointSlice *discoveryv1.EndpointSlice
		objs          []client.Object
		expected      []reconcile.Request
	}{
		{
			endpointSlice: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: v1.GroupVersion.String(),
							Kind:       v1.IstioRevisionKind,
							Name:       "direct-istiorevision-owner",
							Controller: ptr.Of(true),
						},
					},
				},
			},
			expected: []reconcile.Request{
				{NamespacedName: types.NamespacedName{Name: "direct-istiorevision-owner"}},
			},
		},
		{
			endpointSlice: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "istio-system",
					Name:      "endpointslice-1",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: corev1.SchemeGroupVersion.String(),
							Kind:       "Endpoints",
							Name:       "endpoints-owner",
							Controller: ptr.Of(true),
						},
					},
				},
			},
			objs: []client.Object{
				// nolint:staticcheck
				&corev1.Endpoints{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "istio-system",
						Name:      "endpoints-owner",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: v1.GroupVersion.String(),
								Kind:       v1.IstioRevisionKind,
								Name:       "indirect-istiorevision-owner",
								Controller: ptr.Of(true),
							},
						},
					},
				},
			},
			expected: []reconcile.Request{
				{NamespacedName: types.NamespacedName{Name: "indirect-istiorevision-owner"}},
			},
		},
		{
			endpointSlice: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: v1.GroupVersion.String(),
							Kind:       "SomeOtherKind",
							Name:       "not-istiorevision-owner",
							Controller: ptr.Of(true),
						},
					},
				},
			},
			expected: nil,
		},
		{
			endpointSlice: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: v1.GroupVersion.String(),
							Kind:       v1.IstioRevisionKind,
							Name:       "not-controller-owner",
							Controller: ptr.Of(false),
						},
					},
				},
			},
			expected: nil,
		},
		{
			endpointSlice: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "istio-system",
					Name:      "endpointslice-1",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: corev1.SchemeGroupVersion.String(),
							Kind:       "Endpoints",
							Name:       "endpoints-owner",
							Controller: ptr.Of(true),
						},
					},
				},
			},
			objs: []client.Object{
				// nolint:staticcheck
				&corev1.Endpoints{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "istio-system",
						Name:      "endpoints-owner",
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "some-other-group-version",
								Kind:       "some-other-group-kind",
								Name:       "indirect-istiorevision-owner",
								Controller: ptr.Of(true),
							},
						},
					},
				},
			},
			expected: nil,
		},
		{
			endpointSlice: &discoveryv1.EndpointSlice{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "istio-system",
					Name:      "endpointslice-1",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: corev1.SchemeGroupVersion.String(),
							Kind:       "Endpoints",
							Name:       "endpoints-owner-does-not-exist",
							Controller: ptr.Of(true),
						},
					},
				},
			},
			expected: nil,
		},
	}

	for _, tc := range testCases {
		cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(tc.objs...).Build()
		r := NewReconciler(newReconcilerTestConfig(t), cl, scheme.Scheme, nil)

		got := r.mapEndpointSliceToReconcileRequests(context.Background(), tc.endpointSlice)

		g := NewWithT(t)
		g.Expect(got).To(HaveLen(len(tc.expected)))
		g.Expect(got).To(ContainElements(tc.expected))
	}
}

func TestDeriveState(t *testing.T) {
	testCases := []struct {
		name          string
		conditions    []v1.IstioRevisionCondition
		expectedState v1.IstioRevisionConditionReason
	}{
		{
			name: "healthy",
			conditions: []v1.IstioRevisionCondition{
				newCondition(v1.IstioRevisionConditionReconciled, metav1.ConditionTrue, ""),
				newCondition(v1.IstioRevisionConditionReady, metav1.ConditionTrue, ""),
				newCondition(v1.IstioRevisionConditionDependenciesHealthy, metav1.ConditionTrue, ""),
			},
			expectedState: v1.IstioRevisionReasonHealthy,
		},
		{
			name: "not reconciled",
			conditions: []v1.IstioRevisionCondition{
				newCondition(v1.IstioRevisionConditionReconciled, metav1.ConditionFalse, v1.IstioRevisionReasonReconcileError),
				newCondition(v1.IstioRevisionConditionReady, metav1.ConditionTrue, ""),
				newCondition(v1.IstioRevisionConditionDependenciesHealthy, metav1.ConditionTrue, ""),
			},
			expectedState: v1.IstioRevisionReasonReconcileError,
		},
		{
			name: "not ready",
			conditions: []v1.IstioRevisionCondition{
				newCondition(v1.IstioRevisionConditionReconciled, metav1.ConditionTrue, ""),
				newCondition(v1.IstioRevisionConditionReady, metav1.ConditionFalse, v1.IstioRevisionReasonIstiodNotReady),
				newCondition(v1.IstioRevisionConditionDependenciesHealthy, metav1.ConditionTrue, ""),
			},
			expectedState: v1.IstioRevisionReasonIstiodNotReady,
		},
		{
			name: "readiness unknown",
			conditions: []v1.IstioRevisionCondition{
				newCondition(v1.IstioRevisionConditionReconciled, metav1.ConditionTrue, ""),
				newCondition(v1.IstioRevisionConditionReady, metav1.ConditionUnknown, v1.IstioRevisionReasonReadinessCheckFailed),
				newCondition(v1.IstioRevisionConditionDependenciesHealthy, metav1.ConditionTrue, ""),
			},
			expectedState: v1.IstioRevisionReasonReadinessCheckFailed,
		},
		{
			name: "not reconciled nor ready",
			conditions: []v1.IstioRevisionCondition{
				newCondition(v1.IstioRevisionConditionReconciled, metav1.ConditionFalse, v1.IstioRevisionReasonReconcileError),
				newCondition(v1.IstioRevisionConditionReady, metav1.ConditionFalse, v1.IstioRevisionReasonIstiodNotReady),
				newCondition(v1.IstioRevisionConditionDependenciesHealthy, metav1.ConditionTrue, ""),
			},
			expectedState: v1.IstioRevisionReasonReconcileError, // reconcile reason takes precedence over ready reason
		},
		{
			name: "dependencies not ready",
			conditions: []v1.IstioRevisionCondition{
				newCondition(v1.IstioRevisionConditionReconciled, metav1.ConditionTrue, ""),
				newCondition(v1.IstioRevisionConditionReady, metav1.ConditionTrue, ""),
				newCondition(v1.IstioRevisionConditionDependenciesHealthy, metav1.ConditionFalse, v1.IstioRevisionReasonIstioCNINotHealthy),
			},
			expectedState: v1.IstioRevisionReasonIstioCNINotHealthy,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			result := deriveState(tc.conditions...)
			g.Expect(result).To(Equal(tc.expectedState))
		})
	}
}

func newCondition(
	conditionType v1.IstioRevisionConditionType, status metav1.ConditionStatus, reason v1.IstioRevisionConditionReason,
) v1.IstioRevisionCondition {
	return v1.IstioRevisionCondition{
		Type:   conditionType,
		Status: status,
		Reason: reason,
	}
}

func TestDetermineReadyCondition(t *testing.T) {
	cfg := newReconcilerTestConfig(t)

	testCases := []struct {
		name          string
		values        *v1.Values
		clientObjects []client.Object
		interceptors  interceptor.Funcs
		expected      v1.IstioRevisionCondition
		expectErr     bool
	}{
		{
			name:   "Istiod ready",
			values: nil,
			clientObjects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istiod",
						Namespace: "istio-system",
					},
					Status: appsv1.DeploymentStatus{
						Replicas:          2,
						ReadyReplicas:     2,
						AvailableReplicas: 2,
					},
				},
			},
			expected: v1.IstioRevisionCondition{
				Type:   v1.IstioRevisionConditionReady,
				Status: metav1.ConditionTrue,
			},
		},
		{
			name:   "Istiod not ready",
			values: nil,
			clientObjects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istiod",
						Namespace: "istio-system",
					},
					Status: appsv1.DeploymentStatus{
						Replicas:          2,
						ReadyReplicas:     1,
						AvailableReplicas: 1,
					},
				},
			},
			expected: v1.IstioRevisionCondition{
				Type:    v1.IstioRevisionConditionReady,
				Status:  metav1.ConditionFalse,
				Reason:  v1.IstioRevisionReasonIstiodNotReady,
				Message: "not all istiod pods are ready",
			},
		},
		{
			name:   "Istiod scaled to zero",
			values: nil,
			clientObjects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istiod",
						Namespace: "istio-system",
					},
					Status: appsv1.DeploymentStatus{
						Replicas:          0,
						ReadyReplicas:     0,
						AvailableReplicas: 0,
					},
				},
			},
			expected: v1.IstioRevisionCondition{
				Type:    v1.IstioRevisionConditionReady,
				Status:  metav1.ConditionFalse,
				Reason:  v1.IstioRevisionReasonIstiodNotReady,
				Message: "istiod Deployment is scaled to zero replicas",
			},
		},
		{
			name:          "Istiod not found",
			values:        nil,
			clientObjects: []client.Object{},
			expected: v1.IstioRevisionCondition{
				Type:    v1.IstioRevisionConditionReady,
				Status:  metav1.ConditionFalse,
				Reason:  v1.IstioRevisionReasonIstiodNotReady,
				Message: "istiod Deployment not found",
			},
		},
		{
			name: "Non-default revision",
			values: &v1.Values{
				Revision: ptr.Of("my-revision"),
			},
			clientObjects: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istiod-my-revision",
						Namespace: "istio-system",
					},
					Status: appsv1.DeploymentStatus{
						Replicas:          2,
						ReadyReplicas:     2,
						AvailableReplicas: 2,
					},
				},
			},
			expected: v1.IstioRevisionCondition{
				Type:   v1.IstioRevisionConditionReady,
				Status: metav1.ConditionTrue,
			},
		},
		{
			name:          "client error on get",
			clientObjects: []client.Object{},
			interceptors: interceptor.Funcs{
				Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					return fmt.Errorf("simulated error")
				},
			},
			expected: v1.IstioRevisionCondition{
				Type:    v1.IstioRevisionConditionReady,
				Status:  metav1.ConditionUnknown,
				Reason:  v1.IstioRevisionReasonReadinessCheckFailed,
				Message: "failed to get readiness: simulated error",
			},
			expectErr: true,
		},
		{
			name:   "Istiod-remote ready",
			values: &v1.Values{Profile: ptr.Of("remote")},
			clientObjects: []client.Object{
				&admissionv1.MutatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name: "istio-sidecar-injector",
						Annotations: map[string]string{
							constants.WebhookReadinessProbeStatusAnnotationKey: "true",
						},
					},
				},
			},
			expected: v1.IstioRevisionCondition{
				Type:   v1.IstioRevisionConditionReady,
				Status: metav1.ConditionTrue,
			},
		},
		{
			name:   "Istiod-remote not ready",
			values: &v1.Values{Profile: ptr.Of("remote")},
			clientObjects: []client.Object{
				&admissionv1.MutatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name: "istio-sidecar-injector",
						Annotations: map[string]string{
							constants.WebhookReadinessProbeStatusAnnotationKey: "false",
						},
					},
				},
			},
			expected: v1.IstioRevisionCondition{
				Type:    v1.IstioRevisionConditionReady,
				Status:  metav1.ConditionFalse,
				Reason:  v1.IstioRevisionReasonRemoteIstiodNotReady,
				Message: "readiness probe on remote istiod failed",
			},
		},
		{
			name:   "Istiod-remote no readiness probe status annotation",
			values: &v1.Values{Profile: ptr.Of("remote")},
			clientObjects: []client.Object{
				&admissionv1.MutatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "istio-sidecar-injector",
						Annotations: map[string]string{},
					},
				},
			},
			expected: v1.IstioRevisionCondition{
				Type:    v1.IstioRevisionConditionReady,
				Status:  metav1.ConditionFalse,
				Reason:  v1.IstioRevisionReasonRemoteIstiodNotReady,
				Message: "invalid or missing annotation sailoperator.io/readinessProbe.status on MutatingWebhookConfiguration istio-sidecar-injector",
			},
		},
		{
			name:          "Istiod-remote webhook config not found",
			values:        &v1.Values{Profile: ptr.Of("remote")},
			clientObjects: []client.Object{},
			expected: v1.IstioRevisionCondition{
				Type:    v1.IstioRevisionConditionReady,
				Status:  metav1.ConditionFalse,
				Reason:  v1.IstioRevisionReasonRemoteIstiodNotReady,
				Message: "MutatingWebhookConfiguration istio-sidecar-injector not found",
			},
		},
		{
			name:          "Istiod-remote client error on get",
			values:        &v1.Values{Profile: ptr.Of("remote")},
			clientObjects: []client.Object{},
			interceptors: interceptor.Funcs{
				Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					return fmt.Errorf("simulated error")
				},
			},
			expected: v1.IstioRevisionCondition{
				Type:    v1.IstioRevisionConditionReady,
				Status:  metav1.ConditionUnknown,
				Reason:  v1.IstioRevisionReasonReadinessCheckFailed,
				Message: "failed to get readiness: simulated error",
			},
			expectErr: true,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(tt.clientObjects...).WithInterceptorFuncs(tt.interceptors).Build()

			r := NewReconciler(cfg, cl, scheme.Scheme, nil)

			rev := &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-istio",
				},
				Spec: v1.IstioRevisionSpec{
					Namespace: "istio-system",
					Values:    tt.values,
				},
			}

			result, err := r.determineReadyCondition(context.TODO(), rev)
			if tt.expectErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
			g.Expect(result.Type).To(Equal(tt.expected.Type))
			g.Expect(result.Status).To(Equal(tt.expected.Status))
			g.Expect(result.Reason).To(Equal(tt.expected.Reason))
			g.Expect(result.Message).To(Equal(tt.expected.Message))
		})
	}
}

func TestDetermineInUseCondition(t *testing.T) {
	cfg := newReconcilerTestConfig(t)

	testCases := []struct {
		podLabels           map[string]string
		podAnnotations      map[string]string
		nsLabels            map[string]string
		podPhase            corev1.PodPhase
		enableAllNamespaces bool
		interceptors        interceptor.Funcs
		matchesRevision     string
		expectUnknownState  bool
	}{
		// no labels on namespace or pod
		{
			nsLabels:        map[string]string{},
			podLabels:       map[string]string{},
			matchesRevision: "",
		},

		// pod annotations only
		{
			podAnnotations:  map[string]string{"istio.io/rev": "default"},
			matchesRevision: "default",
		},

		// pod succeeded
		{
			podAnnotations:  map[string]string{"istio.io/rev": "default"},
			podPhase:        corev1.PodSucceeded,
			matchesRevision: "",
		},

		// namespace labels only
		{
			nsLabels:        map[string]string{"istio-injection": "enabled"},
			matchesRevision: "default",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "default"},
			matchesRevision: "default",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "my-rev"},
			matchesRevision: "my-rev",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "default", "istio-injection": "enabled"},
			matchesRevision: "default",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "my-rev", "istio-injection": "enabled"},
			matchesRevision: "default",
		},

		// pod labels only
		{
			podLabels:       map[string]string{"istio.io/rev": "default"},
			matchesRevision: "default",
		},
		{
			podLabels:       map[string]string{"istio.io/rev": "my-rev"},
			matchesRevision: "my-rev",
		},
		{
			podLabels:       map[string]string{"sidecar.istio.io/inject": "true"},
			matchesRevision: "default",
		},
		{
			podLabels:       map[string]string{"sidecar.istio.io/inject": "true", "istio.io/rev": "my-rev"},
			matchesRevision: "my-rev",
		},

		// ns and pod labels
		{
			nsLabels:        map[string]string{"istio.io/rev": "my-rev"},
			podLabels:       map[string]string{"sidecar.istio.io/inject": "true"},
			matchesRevision: "my-rev",
		},
		{
			nsLabels:        map[string]string{"istio-injection": "enabled"},
			podLabels:       map[string]string{"istio.io/rev": "default"},
			matchesRevision: "default",
		},
		{
			nsLabels:        map[string]string{"istio-injection": "enabled"},
			podLabels:       map[string]string{"istio.io/rev": "my-rev"},
			matchesRevision: "default",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "default"},
			podLabels:       map[string]string{"istio.io/rev": "default"},
			matchesRevision: "default",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "default"},
			podLabels:       map[string]string{"istio.io/rev": "my-rev"},
			matchesRevision: "default",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "my-rev"},
			podLabels:       map[string]string{"istio.io/rev": "default"},
			matchesRevision: "my-rev",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "my-rev"},
			podLabels:       map[string]string{"istio.io/rev": "my-rev"},
			matchesRevision: "my-rev",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "default", "istio-injection": "enabled"},
			podLabels:       map[string]string{"istio.io/rev": "default"},
			matchesRevision: "default",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "default", "istio-injection": "enabled"},
			podLabels:       map[string]string{"istio.io/rev": "my-rev"},
			matchesRevision: "default",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "my-rev", "istio-injection": "enabled"},
			podLabels:       map[string]string{"istio.io/rev": "default"},
			matchesRevision: "default",
		},
		{
			nsLabels:        map[string]string{"istio.io/rev": "my-rev", "istio-injection": "enabled"},
			podLabels:       map[string]string{"istio.io/rev": "my-rev"},
			matchesRevision: "default",
		},

		// special case: when Values.sidecarInjectorWebhook.enableNamespacesByDefault is true, all pods should match the default revision
		// unless they are in one of the system namespaces ("kube-system","kube-public","kube-node-lease","local-path-storage")
		{
			enableAllNamespaces: true,
			matchesRevision:     "default",
		},
		{
			interceptors: interceptor.Funcs{
				List: func(ctx context.Context, client client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
					return fmt.Errorf("simulated error")
				},
			},
			expectUnknownState: true,
		},
	}

	for _, revName := range []string{"default", "my-rev"} {
		for _, tc := range testCases {
			nameBuilder := strings.Builder{}
			nameBuilder.WriteString(revName + ":")
			if len(tc.nsLabels) == 0 && len(tc.podLabels) == 0 {
				nameBuilder.WriteString("no labels")
			}
			if len(tc.nsLabels) > 0 {
				nameBuilder.WriteString("NS:")
				for k, v := range tc.nsLabels {
					nameBuilder.WriteString(k + ":" + v + ",")
				}
			}
			if len(tc.podLabels) > 0 {
				nameBuilder.WriteString("POD:")
				for k, v := range tc.podLabels {
					nameBuilder.WriteString(k + ":" + v + ",")
				}
			}
			if len(tc.podPhase) > 0 {
				nameBuilder.WriteString("Phase:" + string(tc.podPhase) + ",")
			}
			name := strings.TrimSuffix(nameBuilder.String(), ",")

			t.Run(name, func(t *testing.T) {
				g := NewWithT(t)
				rev := &v1.IstioRevision{
					ObjectMeta: metav1.ObjectMeta{
						Name: revName,
					},
					Spec: v1.IstioRevisionSpec{
						Namespace: "istio-system",
						Version:   "my-version",
					},
				}
				if tc.enableAllNamespaces {
					rev.Spec.Values = &v1.Values{
						SidecarInjectorWebhook: &v1.SidecarInjectorConfig{
							EnableNamespacesByDefault: ptr.Of(true),
						},
					}
				}

				namespace := "bookinfo"
				ns := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   namespace,
						Labels: tc.nsLabels,
					},
				}

				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:        "some-pod",
						Namespace:   namespace,
						Labels:      tc.podLabels,
						Annotations: tc.podAnnotations,
					},
					Status: corev1.PodStatus{
						Phase: tc.podPhase,
					},
				}

				cl := fake.NewClientBuilder().
					WithScheme(scheme.Scheme).
					WithObjects(rev, ns, pod).
					WithInterceptorFuncs(tc.interceptors).
					Build()

				r := NewReconciler(cfg, cl, scheme.Scheme, nil)

				result, _ := r.determineInUseCondition(context.TODO(), rev)
				g.Expect(result.Type).To(Equal(v1.IstioRevisionConditionInUse))

				if tc.expectUnknownState {
					g.Expect(result.Status).To(Equal(metav1.ConditionUnknown))
					g.Expect(result.Reason).To(Equal(v1.IstioRevisionReasonUsageCheckFailed))
				} else {
					if revName == tc.matchesRevision {
						g.Expect(result.Status).To(Equal(metav1.ConditionTrue),
							fmt.Sprintf("Revision %s should be in use, but isn't\n"+
								"revision: %s\nexpected revision: %s\nnamespace labels: %+v\npod labels: %+v",
								revName, revName, tc.matchesRevision, tc.nsLabels, tc.podLabels))
					} else {
						g.Expect(result.Status).To(Equal(metav1.ConditionFalse),
							fmt.Sprintf("Revision %s should not be in use\n"+
								"revision: %s\nexpected revision: %s\nnamespace labels: %+v\npod labels: %+v",
								revName, revName, tc.matchesRevision, tc.nsLabels, tc.podLabels))
					}
				}
			})
		}
	}
}

func TestIgnoreStatusChangePredicate(t *testing.T) {
	predicate := ignoreStatusChange()

	oldObj := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			ResourceVersion: "1",
			Generation:      1,
			Finalizers:      []string{"finalizer1"},
			Labels:          map[string]string{"app": "test"},
			Annotations:     map[string]string{"annotation1": "value1"},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: "v1",
					Kind:       "IstioRevision",
					Name:       "myrev",
				},
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
		},
		Status: corev1.ServiceStatus{
			LoadBalancer: corev1.LoadBalancerStatus{
				Ingress: []corev1.LoadBalancerIngress{
					{
						IP: "1.1.1.1",
					},
				},
			},
			Conditions: nil,
		},
	}

	tests := []struct {
		name     string
		update   func(svc *corev1.Service)
		expected bool
	}{
		{
			name:     "No changes",
			update:   func(svc *corev1.Service) {},
			expected: false,
		},
		{
			name: "ResourceVersion changed",
			update: func(svc *corev1.Service) {
				svc.ResourceVersion = "2"
			},
			expected: false,
		},
		{
			name: "Spec changed",
			update: func(svc *corev1.Service) {
				svc.ResourceVersion = "2"
				svc.Generation++
				svc.Spec.Type = corev1.ServiceTypeNodePort
			},
			expected: true,
		},
		{
			name: "Status changed",
			update: func(svc *corev1.Service) {
				svc.ResourceVersion = "2"
				svc.Status.LoadBalancer.Ingress[0].IP = "2.2.2.2"
			},
			expected: false,
		},
		{
			name: "Spec and status changed",
			update: func(svc *corev1.Service) {
				svc.ResourceVersion = "2"
				svc.Generation++
				svc.Spec.Type = corev1.ServiceTypeNodePort
				svc.Status.LoadBalancer.Ingress[0].IP = "2.2.2.2"
			},
			expected: true,
		},
		{
			name: "Labels changed",
			update: func(svc *corev1.Service) {
				svc.ResourceVersion = "2"
				svc.Labels["app"] = "new-value"
			},
			expected: true,
		},
		{
			name: "Annotations changed",
			update: func(svc *corev1.Service) {
				svc.ResourceVersion = "2"
				svc.Annotations["annotation1"] = "new-value"
			},
			expected: true,
		},
		{
			name: "OwnerReferences changed",
			update: func(svc *corev1.Service) {
				svc.ResourceVersion = "2"
				svc.OwnerReferences[0].Name = "new-owner"
			},
			expected: true,
		},
		{
			name: "Finalizers changed",
			update: func(svc *corev1.Service) {
				svc.ResourceVersion = "2"
				svc.Finalizers = append(svc.Finalizers, "finalizer2")
			},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			newObj := oldObj.DeepCopy()
			tc.update(newObj)

			result := predicate.Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj})
			g.Expect(result).To(Equal(tc.expected), "unexpected result of predicate.Update()")
		})
	}
}

func newReconcilerTestConfig(t *testing.T) config.ReconcilerConfig {
	return config.ReconcilerConfig{
		ResourceDirectory: t.TempDir(),
		Platform:          config.PlatformKubernetes,
		DefaultProfile:    "",
	}
}
