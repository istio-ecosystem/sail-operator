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
	"github.com/istio-ecosystem/sail-operator/pkg/istiovalues"
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
					Version:   istioversion.Default,
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
					Version: istioversion.Default,
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
					Version:   istioversion.Default,
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
					Version: istioversion.Default,
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
					istioversion.Default: {
						CNIImage: "cni-test",
					},
				},
			},
			input: &v1.IstioCNI{
				Spec: v1.IstioCNISpec{
					Version: istioversion.Default,
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
					istioversion.Default: {
						CNIImage: "cni-test",
					},
				},
			},
			input: &v1.IstioCNI{
				Spec: v1.IstioCNISpec{
					Version: istioversion.Default,
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
					istioversion.Default: {
						CNIImage: "cni-test",
					},
				},
			},
			input: &v1.IstioCNI{
				Spec: v1.IstioCNISpec{
					Version: istioversion.Default,
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
			name: "user-supplied-global-hub",
			config: config.OperatorConfig{
				ImageDigests: map[string]config.IstioImageConfig{
					istioversion.Default: {
						CNIImage: "cni-test",
					},
				},
			},
			input: &v1.IstioCNI{
				Spec: v1.IstioCNISpec{
					Version: istioversion.Default,
					Values: &v1.CNIValues{
						Global: &v1.CNIGlobalConfig{
							Hub: ptr.Of("docker.io/istio"),
						},
					},
				},
			},
			expectValues: &v1.CNIValues{
				Global: &v1.CNIGlobalConfig{
					Hub: ptr.Of("docker.io/istio"),
				},
			},
		},
		{
			name: "user-supplied-global-tag",
			config: config.OperatorConfig{
				ImageDigests: map[string]config.IstioImageConfig{
					istioversion.Default: {
						CNIImage: "cni-test",
					},
				},
			},
			input: &v1.IstioCNI{
				Spec: v1.IstioCNISpec{
					Version: istioversion.Default,
					Values: &v1.CNIValues{
						Global: &v1.CNIGlobalConfig{
							Tag: ptr.Of("v1.24.0-custom-build"),
						},
					},
				},
			},
			expectValues: &v1.CNIValues{
				Global: &v1.CNIGlobalConfig{
					Tag: ptr.Of("v1.24.0-custom-build"),
				},
			},
		},
		{
			name: "version-without-defaults",
			config: config.OperatorConfig{
				ImageDigests: map[string]config.IstioImageConfig{
					istioversion.Default: {
						CNIImage: "cni-test",
					},
				},
			},
			input: &v1.IstioCNI{
				Spec: v1.IstioCNISpec{
					Version: istioversion.Default,
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
			version, err := istioversion.Resolve(tc.input.Spec.Version)
			if err != nil {
				t.Errorf("failed to resolve IstioCNI version for %q: %v", tc.input.Name, err)
			}
			result := applyImageDigests(version, tc.input.Spec.Values, tc.config)
			if diff := cmp.Diff(tc.expectValues, result); diff != "" {
				t.Errorf("unexpected merge result; diff (-expected, +actual):\n%v", diff)
			}
		})
	}
}

func TestIstioCNIvendorDefaults(t *testing.T) {
	tests := []struct {
		name               string
		version            string
		userValues         *v1.CNIValues
		expected           *v1.CNIValues
		vendorDefaultsYAML string
		expectError        bool
	}{
		{
			name:    "user values and no vendor defaults",
			version: "v1.24.2",
			userValues: &v1.CNIValues{
				Cni: &v1.CNIConfig{
					CniConfDir: StringPtr("example/path"),
				},
			},
			vendorDefaultsYAML: `
`, // No vendor defaults provided
			expected: &v1.CNIValues{
				Cni: &v1.CNIConfig{
					CniConfDir: StringPtr("example/path"),
				},
			},
			expectError: false,
		},
		{
			name:    "vendor default not override user values",
			version: "v1.24.2",
			userValues: &v1.CNIValues{
				Cni: &v1.CNIConfig{
					CniConfDir: StringPtr("example/path"),
					Image:      StringPtr("custom/cni-image"),
				},
			},
			vendorDefaultsYAML: `
v1.24.2:
  istiocni:
    cni:
      cniConfDir: example/path/vendor/path
`, // Vendor defaults provided but should not override user values
			expected: &v1.CNIValues{
				Cni: &v1.CNIConfig{
					CniConfDir: StringPtr("example/path"),
					Image:      StringPtr("custom/cni-image"),
				},
			},
			expectError: false,
		},
		{
			name:    "merge vendor defaults with user values",
			version: "v1.24.2",
			userValues: &v1.CNIValues{
				Cni: &v1.CNIConfig{
					Image: StringPtr("custom/cni-image"),
				},
			},
			vendorDefaultsYAML: `
v1.24.2:
  istiocni:
    cni:
      cniConfDir: example/path/vendor/path
      image: vendor/cni-image
`,
			expected: &v1.CNIValues{
				Cni: &v1.CNIConfig{
					CniConfDir: StringPtr("example/path/vendor/path"),
					Image:      StringPtr("custom/cni-image"),
				},
			},
			expectError: false,
		},
		{
			name:       "apply vendor defaults with no user values",
			version:    "v1.24.2",
			userValues: &v1.CNIValues{},
			vendorDefaultsYAML: `
v1.24.2:
  istiocni:
    cni:
      cniConfDir: example/path/vendor/path
      image: vendor/cni-image
`,
			expected: &v1.CNIValues{
				Cni: &v1.CNIConfig{
					CniConfDir: StringPtr("example/path/vendor/path"),
					Image:      StringPtr("vendor/cni-image"),
				},
			},
			expectError: false,
		},
		{
			name:       "non existing field",
			version:    "v1.24.2",
			userValues: &v1.CNIValues{},
			vendorDefaultsYAML: `
v1.24.2:
  istiocni:
    cni:
      nonExistingField: example/path/vendor/path
`, // A non-existing field in vendor defaults
			expected: &v1.CNIValues{
				Cni: &v1.CNIConfig{},
			}, // Should not cause an error, but the field should not be present in the result
			expectError: false,
		},
		{
			name:       "malformed vendor defaults",
			version:    "v1.24.2",
			userValues: &v1.CNIValues{},
			vendorDefaultsYAML: `
v1.24.2:
  istiocni:
    cni: ""
`, // Malformed vendor defaults (cni should be a map, not a string)
			expected:    nil, // Expect an error due to malformed vendor defaults
			expectError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			vendorDefaults := istiovalues.MustParseVendorDefaultsYAML([]byte(tt.vendorDefaultsYAML))

			// Apply vendor defaults
			istiovalues.OverrideVendorDefaults(vendorDefaults)
			result, err := istiovalues.ApplyIstioCNIVendorDefaults(tt.version, tt.userValues)
			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}

			// Check if the result matches the expected values
			if diff := cmp.Diff(tt.expected, result); diff != "" {
				t.Errorf("unexpected merge result; diff (-expected, +actual):\n%v", diff)
			}
		})
	}
}

// StringPtr returns a pointer to a string literal.
func StringPtr(s string) *string {
	return &s
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
		ResourceDirectory:       t.TempDir(),
		Platform:                config.PlatformKubernetes,
		DefaultProfile:          "",
		MaxConcurrentReconciles: 1,
	}
}
