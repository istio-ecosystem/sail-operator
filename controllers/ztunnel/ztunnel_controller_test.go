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

package ztunnel

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
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

const (
	ztunnelNamespace = "ztunnel"
)

func TestValidate(t *testing.T) {
	cfg := newReconcilerTestConfig(t)

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ztunnelNamespace,
		},
	}

	testCases := []struct {
		name      string
		ztunnel   *v1.ZTunnel
		objects   []client.Object
		expectErr string
	}{
		{
			name: "success",
			ztunnel: &v1.ZTunnel{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1.ZTunnelSpec{
					Version:   istioversion.Default,
					Namespace: ztunnelNamespace,
				},
			},
			objects:   []client.Object{ns},
			expectErr: "",
		},
		{
			name: "no version",
			ztunnel: &v1.ZTunnel{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1.ZTunnelSpec{
					Namespace: ztunnelNamespace,
				},
			},
			objects:   []client.Object{ns},
			expectErr: "spec.version not set",
		},
		{
			name: "no namespace",
			ztunnel: &v1.ZTunnel{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1.ZTunnelSpec{
					Version: istioversion.Default,
				},
			},
			objects:   []client.Object{ns},
			expectErr: "spec.namespace not set",
		},
		{
			name: "namespace not found",
			ztunnel: &v1.ZTunnel{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1.ZTunnelSpec{
					Version:   istioversion.Default,
					Namespace: ztunnelNamespace,
				},
			},
			objects:   []client.Object{},
			expectErr: fmt.Sprintf(`namespace %q doesn't exist`, ztunnelNamespace),
		},
		{
			name: "namespace is being deleted",
			ztunnel: &v1.ZTunnel{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1.ZTunnelSpec{
					Version:   istioversion.Default,
					Namespace: ztunnelNamespace,
				},
			},
			objects: []client.Object{&corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: ztunnelNamespace,
					DeletionTimestamp: &metav1.Time{
						Time: time.Now(),
					},
					Finalizers: []string{
						"sail-operator",
					},
				},
			}},
			expectErr: fmt.Sprintf(`namespace %q is being deleted`, ztunnelNamespace),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(tc.objects...).Build()
			r := NewReconciler(cfg, cl, scheme.Scheme, nil)

			err := r.validate(context.TODO(), tc.ztunnel)
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
		reconciledCondition v1.ZTunnelCondition
		readyCondition      v1.ZTunnelCondition
		expectedState       v1.ZTunnelConditionReason
	}{
		{
			name:                "healthy",
			reconciledCondition: newCondition(v1.ZTunnelConditionReconciled, metav1.ConditionTrue, ""),
			readyCondition:      newCondition(v1.ZTunnelConditionReady, metav1.ConditionTrue, ""),
			expectedState:       v1.ZTunnelReasonHealthy,
		},
		{
			name:                "not reconciled",
			reconciledCondition: newCondition(v1.ZTunnelConditionReconciled, metav1.ConditionFalse, v1.ZTunnelReasonReconcileError),
			readyCondition:      newCondition(v1.ZTunnelConditionReady, metav1.ConditionTrue, ""),
			expectedState:       v1.ZTunnelReasonReconcileError,
		},
		{
			name:                "not ready",
			reconciledCondition: newCondition(v1.ZTunnelConditionReconciled, metav1.ConditionTrue, ""),
			readyCondition:      newCondition(v1.ZTunnelConditionReady, metav1.ConditionFalse, v1.ZTunnelDaemonSetNotReady),
			expectedState:       v1.ZTunnelDaemonSetNotReady,
		},
		{
			name:                "readiness unknown",
			reconciledCondition: newCondition(v1.ZTunnelConditionReconciled, metav1.ConditionTrue, ""),
			readyCondition:      newCondition(v1.ZTunnelConditionReady, metav1.ConditionUnknown, v1.ZTunnelReasonReadinessCheckFailed),
			expectedState:       v1.ZTunnelReasonReadinessCheckFailed,
		},
		{
			name:                "not reconciled nor ready",
			reconciledCondition: newCondition(v1.ZTunnelConditionReconciled, metav1.ConditionFalse, v1.ZTunnelReasonReconcileError),
			readyCondition:      newCondition(v1.ZTunnelConditionReady, metav1.ConditionFalse, v1.ZTunnelDaemonSetNotReady),
			expectedState:       v1.ZTunnelReasonReconcileError, // reconcile reason takes precedence over ready reason
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

func newCondition(condType v1.ZTunnelConditionType, status metav1.ConditionStatus, reason v1.ZTunnelConditionReason) v1.ZTunnelCondition {
	return v1.ZTunnelCondition{
		Type:   condType,
		Status: status,
		Reason: reason,
	}
}

func TestDetermineReadyCondition(t *testing.T) {
	cfg := newReconcilerTestConfig(t)

	testCases := []struct {
		name          string
		clientObjects []client.Object
		interceptors  interceptor.Funcs
		expected      v1.ZTunnelCondition
		expectErr     bool
	}{
		{
			name: "ZTunnel ready",
			clientObjects: []client.Object{
				&appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "ztunnel",
						Namespace: ztunnelNamespace,
					},
					Status: appsv1.DaemonSetStatus{
						CurrentNumberScheduled: 1,
						NumberReady:            1,
					},
				},
			},
			expected: v1.ZTunnelCondition{
				Type:   v1.ZTunnelConditionReady,
				Status: metav1.ConditionTrue,
			},
		},
		{
			name: "ZTunnel not ready",
			clientObjects: []client.Object{
				&appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "ztunnel",
						Namespace: ztunnelNamespace,
					},
					Status: appsv1.DaemonSetStatus{
						CurrentNumberScheduled: 1,
						NumberReady:            0,
					},
				},
			},
			expected: v1.ZTunnelCondition{
				Type:    v1.ZTunnelConditionReady,
				Status:  metav1.ConditionFalse,
				Reason:  v1.ZTunnelDaemonSetNotReady,
				Message: "not all ztunnel pods are ready",
			},
		},
		{
			name: "ZTunnel pods not scheduled",
			clientObjects: []client.Object{
				&appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "ztunnel",
						Namespace: ztunnelNamespace,
					},
					Status: appsv1.DaemonSetStatus{
						CurrentNumberScheduled: 0,
						NumberReady:            0,
					},
				},
			},
			expected: v1.ZTunnelCondition{
				Type:    v1.ZTunnelConditionReady,
				Status:  metav1.ConditionFalse,
				Reason:  v1.ZTunnelDaemonSetNotReady,
				Message: "no ztunnel pods are currently scheduled",
			},
		},
		{
			name:          "ZTunnel daemonSet not found",
			clientObjects: []client.Object{},
			expected: v1.ZTunnelCondition{
				Type:    v1.ZTunnelConditionReady,
				Status:  metav1.ConditionFalse,
				Reason:  v1.ZTunnelDaemonSetNotReady,
				Message: "ztunnel DaemonSet not found",
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
			expected: v1.ZTunnelCondition{
				Type:    v1.ZTunnelConditionReady,
				Status:  metav1.ConditionUnknown,
				Reason:  v1.ZTunnelReasonReadinessCheckFailed,
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

			ztunnel := &v1.ZTunnel{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ztunnel",
				},
				Spec: v1.ZTunnelSpec{
					Namespace: ztunnelNamespace,
				},
			}

			result, err := r.determineReadyCondition(context.TODO(), ztunnel)
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
		input        *v1.ZTunnel
		expectValues *v1.ZTunnelValues
	}{
		{
			name: "no-config",
			config: config.OperatorConfig{
				ImageDigests: map[string]config.IstioImageConfig{},
			},
			input: &v1.ZTunnel{
				Spec: v1.ZTunnelSpec{
					Version: "v1.24.0",
					Values: &v1.ZTunnelValues{
						ZTunnel: &v1.ZTunnelConfig{
							Image: ptr.Of("ztunnel-test"),
						},
					},
				},
			},
			expectValues: &v1.ZTunnelValues{
				ZTunnel: &v1.ZTunnelConfig{
					Image: ptr.Of("ztunnel-test"),
				},
			},
		},
		{
			name: "no-user-values",
			config: config.OperatorConfig{
				ImageDigests: map[string]config.IstioImageConfig{
					"v1.24.0": {
						ZTunnelImage: "ztunnel-test",
					},
				},
			},
			input: &v1.ZTunnel{
				Spec: v1.ZTunnelSpec{
					Version: "v1.24.0",
					Values:  &v1.ZTunnelValues{},
				},
			},
			expectValues: &v1.ZTunnelValues{
				ZTunnel: &v1.ZTunnelConfig{
					Image: ptr.Of("ztunnel-test"),
				},
			},
		},
		{
			name: "user-supplied-image",
			config: config.OperatorConfig{
				ImageDigests: map[string]config.IstioImageConfig{
					"v1.24.0": {
						ZTunnelImage: "ztunnel-test",
					},
				},
			},
			input: &v1.ZTunnel{
				Spec: v1.ZTunnelSpec{
					Version: "v1.24.0",
					Values: &v1.ZTunnelValues{
						ZTunnel: &v1.ZTunnelConfig{
							Image: ptr.Of("ztunnel-custom"),
						},
					},
				},
			},
			expectValues: &v1.ZTunnelValues{
				ZTunnel: &v1.ZTunnelConfig{
					Image: ptr.Of("ztunnel-custom"),
				},
			},
		},
		{
			name: "user-supplied-hub-tag",
			config: config.OperatorConfig{
				ImageDigests: map[string]config.IstioImageConfig{
					"v1.24.0": {
						ZTunnelImage: "ztunnel-test",
					},
				},
			},
			input: &v1.ZTunnel{
				Spec: v1.ZTunnelSpec{
					Version: "v1.24.0",
					Values: &v1.ZTunnelValues{
						ZTunnel: &v1.ZTunnelConfig{
							Hub: ptr.Of("docker.io/istio"),
							Tag: ptr.Of("1.24.0"),
						},
					},
				},
			},
			expectValues: &v1.ZTunnelValues{
				ZTunnel: &v1.ZTunnelConfig{
					Hub: ptr.Of("docker.io/istio"),
					Tag: ptr.Of("1.24.0"),
				},
			},
		},
		{
			name: "user-supplied-global-hub",
			config: config.OperatorConfig{
				ImageDigests: map[string]config.IstioImageConfig{
					"v1.24.0": {
						ZTunnelImage: "ztunnel-test",
					},
				},
			},
			input: &v1.ZTunnel{
				Spec: v1.ZTunnelSpec{
					Version: "v1.24.0",
					Values: &v1.ZTunnelValues{
						Global: &v1.ZTunnelGlobalConfig{
							Hub: ptr.Of("docker.io/istio"),
						},
					},
				},
			},
			expectValues: &v1.ZTunnelValues{
				Global: &v1.ZTunnelGlobalConfig{
					Hub: ptr.Of("docker.io/istio"),
				},
			},
		},
		{
			name: "user-supplied-global-tag",
			config: config.OperatorConfig{
				ImageDigests: map[string]config.IstioImageConfig{
					"v1.24.0": {
						ZTunnelImage: "ztunnel-test",
					},
				},
			},
			input: &v1.ZTunnel{
				Spec: v1.ZTunnelSpec{
					Version: "v1.24.0",
					Values: &v1.ZTunnelValues{
						Global: &v1.ZTunnelGlobalConfig{
							Tag: ptr.Of("v1.24.0-custom-build"),
						},
					},
				},
			},
			expectValues: &v1.ZTunnelValues{
				Global: &v1.ZTunnelGlobalConfig{
					Tag: ptr.Of("v1.24.0-custom-build"),
				},
			},
		},
		{
			name: "version-without-defaults",
			config: config.OperatorConfig{
				ImageDigests: map[string]config.IstioImageConfig{
					"v1.24.0": {
						ZTunnelImage: "ztunnel-test",
					},
				},
			},
			input: &v1.ZTunnel{
				Spec: v1.ZTunnelSpec{
					Version: "v1.24.1",
					Values: &v1.ZTunnelValues{
						ZTunnel: &v1.ZTunnelConfig{
							Hub: ptr.Of("docker.io/istio"),
							Tag: ptr.Of("1.24.1"),
						},
					},
				},
			},
			expectValues: &v1.ZTunnelValues{
				ZTunnel: &v1.ZTunnelConfig{
					Hub: ptr.Of("docker.io/istio"),
					Tag: ptr.Of("1.24.1"),
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := applyImageDigests(tc.input.Spec.Version, tc.input.Spec.Values, tc.config)
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

			ztunnel := &v1.ZTunnel{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "ztunnel",
					Generation: 123,
				},
			}

			status, err := r.determineStatus(ctx, ztunnel, tt.reconcileErr)
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(status.ObservedGeneration).To(Equal(ztunnel.Generation))

			reconciledCondition := r.determineReconciledCondition(tt.reconcileErr)
			readyCondition, err := r.determineReadyCondition(ctx, ztunnel)
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(status.State).To(Equal(deriveState(reconciledCondition, readyCondition)))
			g.Expect(normalize(status.GetCondition(v1.ZTunnelConditionReconciled))).To(Equal(normalize(reconciledCondition)))
			g.Expect(normalize(status.GetCondition(v1.ZTunnelConditionReady))).To(Equal(normalize(readyCondition)))
		})
	}
}

func normalize(condition v1.ZTunnelCondition) v1.ZTunnelCondition {
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
