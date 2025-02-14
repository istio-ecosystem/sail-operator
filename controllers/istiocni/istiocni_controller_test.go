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

package istiocni

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversions"
	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"istio.io/istio/pkg/ptr"
)

func TestValidate(t *testing.T) {
	cfg := newReconcilerTestConfig(t)

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "istio-cni",
		},
	}

	testCases := []struct {
		name      string
		cni       *v1.IstioCNI
		objects   []client.Object
		expectErr string
	}{
		{
			name: "success",
			cni: &v1.IstioCNI{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1.IstioCNISpec{
					Version:   istioversions.Default,
					Namespace: "istio-cni",
				},
			},
			objects:   []client.Object{ns},
			expectErr: "",
		},
		{
			name: "no version",
			cni: &v1.IstioCNI{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1.IstioCNISpec{
					Namespace: "istio-cni",
				},
			},
			objects:   []client.Object{ns},
			expectErr: "spec.version not set",
		},
		{
			name: "no namespace",
			cni: &v1.IstioCNI{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1.IstioCNISpec{
					Version: istioversions.Default,
				},
			},
			objects:   []client.Object{ns},
			expectErr: "spec.namespace not set",
		},
		{
			name: "namespace not found",
			cni: &v1.IstioCNI{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1.IstioCNISpec{
					Version:   istioversions.Default,
					Namespace: "istio-cni",
				},
			},
			objects:   []client.Object{},
			expectErr: `namespace "istio-cni" doesn't exist`,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(tc.objects...).Build()
			r := NewReconciler(cfg, cl, scheme.Scheme, nil)

			err := r.validate(context.TODO(), tc.cni)
			if tc.expectErr == "" {
				g.Expect(err).ToNot(HaveOccurred())
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectErr))
			}
		})
	}
}

func TestDeriveState(t *testing.T) {
	testCases := []struct {
		name                string
		reconciledCondition v1.IstioCNICondition
		readyCondition      v1.IstioCNICondition
		expectedState       v1.IstioCNIConditionReason
	}{
		{
			name:                "healthy",
			reconciledCondition: newCondition(v1.IstioCNIConditionReconciled, metav1.ConditionTrue, ""),
			readyCondition:      newCondition(v1.IstioCNIConditionReady, metav1.ConditionTrue, ""),
			expectedState:       v1.IstioCNIReasonHealthy,
		},
		{
			name:                "not reconciled",
			reconciledCondition: newCondition(v1.IstioCNIConditionReconciled, metav1.ConditionFalse, v1.IstioCNIReasonReconcileError),
			readyCondition:      newCondition(v1.IstioCNIConditionReady, metav1.ConditionTrue, ""),
			expectedState:       v1.IstioCNIReasonReconcileError,
		},
		{
			name:                "not ready",
			reconciledCondition: newCondition(v1.IstioCNIConditionReconciled, metav1.ConditionTrue, ""),
			readyCondition:      newCondition(v1.IstioCNIConditionReady, metav1.ConditionFalse, v1.IstioCNIDaemonSetNotReady),
			expectedState:       v1.IstioCNIDaemonSetNotReady,
		},
		{
			name:                "readiness unknown",
			reconciledCondition: newCondition(v1.IstioCNIConditionReconciled, metav1.ConditionTrue, ""),
			readyCondition:      newCondition(v1.IstioCNIConditionReady, metav1.ConditionUnknown, v1.IstioCNIReasonReadinessCheckFailed),
			expectedState:       v1.IstioCNIReasonReadinessCheckFailed,
		},
		{
			name:                "not reconciled nor ready",
			reconciledCondition: newCondition(v1.IstioCNIConditionReconciled, metav1.ConditionFalse, v1.IstioCNIReasonReconcileError),
			readyCondition:      newCondition(v1.IstioCNIConditionReady, metav1.ConditionFalse, v1.IstioCNIDaemonSetNotReady),
			expectedState:       v1.IstioCNIReasonReconcileError, // reconcile reason takes precedence over ready reason
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			result := deriveState(tc.reconciledCondition, tc.readyCondition)
			g.Expect(result).To(Equal(tc.expectedState))
		})
	}
}

func newCondition(condType v1.IstioCNIConditionType, status metav1.ConditionStatus, reason v1.IstioCNIConditionReason) v1.IstioCNICondition {
	return v1.IstioCNICondition{
		Type:   condType,
		Status: status,
		Reason: reason,
	}
}

func TestDetermineReadyCondition(t *testing.T) {
	cfg := newReconcilerTestConfig(t)

	testCases := []struct {
		name          string
		cniEnabled    bool
		clientObjects []client.Object
		interceptors  interceptor.Funcs
		expected      v1.IstioCNICondition
		expectErr     bool
	}{
		{
			name: "CNI ready",
			clientObjects: []client.Object{
				&appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istio-cni-node",
						Namespace: "istio-cni",
					},
					Status: appsv1.DaemonSetStatus{
						CurrentNumberScheduled: 1,
						NumberReady:            1,
					},
				},
			},
			expected: v1.IstioCNICondition{
				Type:   v1.IstioCNIConditionReady,
				Status: metav1.ConditionTrue,
			},
		},
		{
			name: "CNI not ready",
			clientObjects: []client.Object{
				&appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istio-cni-node",
						Namespace: "istio-cni",
					},
					Status: appsv1.DaemonSetStatus{
						CurrentNumberScheduled: 1,
						NumberReady:            0,
					},
				},
			},
			expected: v1.IstioCNICondition{
				Type:    v1.IstioCNIConditionReady,
				Status:  metav1.ConditionFalse,
				Reason:  v1.IstioCNIDaemonSetNotReady,
				Message: "not all istio-cni-node pods are ready",
			},
		},
		{
			name: "CNI pods not scheduled",
			clientObjects: []client.Object{
				&appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "istio-cni-node",
						Namespace: "istio-cni",
					},
					Status: appsv1.DaemonSetStatus{
						CurrentNumberScheduled: 0,
						NumberReady:            0,
					},
				},
			},
			expected: v1.IstioCNICondition{
				Type:    v1.IstioCNIConditionReady,
				Status:  metav1.ConditionFalse,
				Reason:  v1.IstioCNIDaemonSetNotReady,
				Message: "no istio-cni-node pods are currently scheduled",
			},
		},
		{
			name:          "CNI not found",
			clientObjects: []client.Object{},
			expected: v1.IstioCNICondition{
				Type:    v1.IstioCNIConditionReady,
				Status:  metav1.ConditionFalse,
				Reason:  v1.IstioCNIDaemonSetNotReady,
				Message: "istio-cni-node DaemonSet not found",
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
			expected: v1.IstioCNICondition{
				Type:    v1.IstioCNIConditionReady,
				Status:  metav1.ConditionUnknown,
				Reason:  v1.IstioCNIReasonReadinessCheckFailed,
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

			cni := &v1.IstioCNI{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-istio",
				},
				Spec: v1.IstioCNISpec{
					Namespace: "istio-cni",
				},
			}

			result, err := r.determineReadyCondition(context.TODO(), cni)
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

func TestApplyImageDigests(t *testing.T) {
	testCases := []struct {
		name         string
		config       config.OperatorConfig
		input        *v1.IstioCNI
		expectValues *v1.CNIValues
	}{
		{
			name: "no-config",
			config: config.OperatorConfig{
				ImageDigests: map[string]config.IstioImageConfig{},
			},
			input: &v1.IstioCNI{
				Spec: v1.IstioCNISpec{
					Version: istioversions.Default,
					Values: &v1.CNIValues{
						Cni: &v1.CNIConfig{
							Image: ptr.Of("istiocni-test"),
						},
					},
				},
			},
			expectValues: &v1.CNIValues{
				Cni: &v1.CNIConfig{
					Image: ptr.Of("istiocni-test"),
				},
			},
		},
		{
			name: "no-user-values",
			config: config.OperatorConfig{
				ImageDigests: map[string]config.IstioImageConfig{
					istioversions.Default: {
						CNIImage: "cni-test",
					},
				},
			},
			input: &v1.IstioCNI{
				Spec: v1.IstioCNISpec{
					Version: istioversions.Default,
					Values:  &v1.CNIValues{},
				},
			},
			expectValues: &v1.CNIValues{
				Cni: &v1.CNIConfig{
					Image: ptr.Of("cni-test"),
				},
			},
		},
		{
			name: "user-supplied-image",
			config: config.OperatorConfig{
				ImageDigests: map[string]config.IstioImageConfig{
					istioversions.Default: {
						CNIImage: "cni-test",
					},
				},
			},
			input: &v1.IstioCNI{
				Spec: v1.IstioCNISpec{
					Version: istioversions.Default,
					Values: &v1.CNIValues{
						Cni: &v1.CNIConfig{
							Image: ptr.Of("cni-custom"),
						},
					},
				},
			},
			expectValues: &v1.CNIValues{
				Cni: &v1.CNIConfig{
					Image: ptr.Of("cni-custom"),
				},
			},
		},
		{
			name: "user-supplied-hub-tag",
			config: config.OperatorConfig{
				ImageDigests: map[string]config.IstioImageConfig{
					istioversions.Default: {
						CNIImage: "cni-test",
					},
				},
			},
			input: &v1.IstioCNI{
				Spec: v1.IstioCNISpec{
					Version: istioversions.Default,
					Values: &v1.CNIValues{
						Cni: &v1.CNIConfig{
							Hub: ptr.Of("docker.io/istio"),
							Tag: ptr.Of("1.20.1"),
						},
					},
				},
			},
			expectValues: &v1.CNIValues{
				Cni: &v1.CNIConfig{
					Hub: ptr.Of("docker.io/istio"),
					Tag: ptr.Of("1.20.1"),
				},
			},
		},
		{
			name: "version-without-defaults",
			config: config.OperatorConfig{
				ImageDigests: map[string]config.IstioImageConfig{
					istioversions.Default: {
						CNIImage: "cni-test",
					},
				},
			},
			input: &v1.IstioCNI{
				Spec: v1.IstioCNISpec{
					Version: istioversions.Default,
					Values: &v1.CNIValues{
						Cni: &v1.CNIConfig{
							Hub: ptr.Of("docker.io/istio"),
							Tag: ptr.Of("1.20.2"),
						},
					},
				},
			},
			expectValues: &v1.CNIValues{
				Cni: &v1.CNIConfig{
					Hub: ptr.Of("docker.io/istio"),
					Tag: ptr.Of("1.20.2"),
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			version, err := istioversions.ResolveVersion(tc.input.Spec.Version)
			if err != nil {
				t.Errorf("failed to resolve version: %v", err)
			}
			result := applyImageDigests(version, tc.input.Spec.Values, tc.config)
			if diff := cmp.Diff(tc.expectValues, result); diff != "" {
				t.Errorf("unexpected merge result; diff (-expected, +actual):\n%v", diff)
			}
		})
	}
}

func TestDetermineStatus(t *testing.T) {
	cfg := newReconcilerTestConfig(t)

	tests := []struct {
		name         string
		reconcileErr error
	}{
		{
			name:         "no error",
			reconcileErr: nil,
		},
		{
			name:         "reconcile error",
			reconcileErr: fmt.Errorf("some reconcile error"),
		},
	}

	ctx := context.TODO()
	cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
	r := NewReconciler(cfg, cl, scheme.Scheme, nil)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			cni := &v1.IstioCNI{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "my-cni",
					Generation: 123,
				},
			}

			status, err := r.determineStatus(ctx, cni, tt.reconcileErr)
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(status.ObservedGeneration).To(Equal(cni.Generation))

			reconciledCondition := r.determineReconciledCondition(tt.reconcileErr)
			readyCondition, err := r.determineReadyCondition(ctx, cni)
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(status.State).To(Equal(deriveState(reconciledCondition, readyCondition)))
			g.Expect(normalize(status.GetCondition(v1.IstioCNIConditionReconciled))).To(Equal(normalize(reconciledCondition)))
			g.Expect(normalize(status.GetCondition(v1.IstioCNIConditionReady))).To(Equal(normalize(readyCondition)))
		})
	}
}

func normalize(condition v1.IstioCNICondition) v1.IstioCNICondition {
	condition.LastTransitionTime = metav1.Time{}
	return condition
}

func newReconcilerTestConfig(t *testing.T) config.ReconcilerConfig {
	return config.ReconcilerConfig{
		ResourceDirectory: t.TempDir(),
		Platform:          config.PlatformKubernetes,
		DefaultProfile:    "",
	}
}
