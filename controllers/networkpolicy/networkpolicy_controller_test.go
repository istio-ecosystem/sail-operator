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

package networkpolicy

import (
	"context"
	"fmt"
	"testing"

	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	. "github.com/onsi/gomega"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

var (
	ctx               = context.Background()
	operatorNamespace = "sail-operator"
)

func TestReconcileNetworkPolicy(t *testing.T) {
	tests := []struct {
		name                  string
		networkPolicy         *networkingv1.NetworkPolicy
		existingObjects       []client.Object
		interceptors          interceptor.Funcs
		expectedResult        ctrl.Result
		expectError           bool
		validateNetworkPolicy func(*testing.T, client.Client)
	}{
		{
			name: "ignores network policy with different name",
			networkPolicy: &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "some-other-policy",
					Namespace: operatorNamespace,
				},
			},
			expectedResult: ctrl.Result{},
			expectError:    false,
		},
		{
			name: "updates network policy when spec differs",
			networkPolicy: &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      constants.NetworkPolicyName,
					Namespace: operatorNamespace,
				},
				Spec: networkingv1.NetworkPolicySpec{
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"really-wrong": "selector",
						},
					},
				},
			},
			expectedResult: ctrl.Result{},
			expectError:    false,
			validateNetworkPolicy: func(t *testing.T, cl client.Client) {
				g := NewWithT(t)
				np := &networkingv1.NetworkPolicy{}
				err := cl.Get(ctx, types.NamespacedName{
					Name:      constants.NetworkPolicyName,
					Namespace: operatorNamespace,
				}, np)
				g.Expect(err).ToNot(HaveOccurred())

				g.Expect(np.Spec.PodSelector.MatchLabels).To(HaveKeyWithValue("control-plane", "sail-operator"))
				g.Expect(np.Spec.PolicyTypes).To(ContainElements(
					networkingv1.PolicyTypeIngress,
					networkingv1.PolicyTypeEgress,
				))
			},
		},
		{
			name: "returns error when update fails",
			networkPolicy: &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      constants.NetworkPolicyName,
					Namespace: operatorNamespace,
				},
				Spec: networkingv1.NetworkPolicySpec{
					PodSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"wrong": "selector",
						},
					},
				},
			},
			interceptors: interceptor.Funcs{
				Update: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
					return fmt.Errorf("update failed")
				},
			},
			expectedResult: ctrl.Result{},
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			cfg := newReconcilerTestConfig(t)

			objects := tt.existingObjects
			if tt.networkPolicy != nil {
				objects = append(objects, tt.networkPolicy)
			}

			cl := newFakeClientBuilder().
				WithObjects(objects...).
				WithInterceptorFuncs(tt.interceptors).
				Build()

			reconciler := NewReconciler(cfg, cl, scheme.Scheme, operatorNamespace)

			result, err := reconciler.reconcileNetworkPolicy(ctx, tt.networkPolicy)

			g.Expect(result).To(Equal(tt.expectedResult))
			if tt.expectError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}

			if tt.validateNetworkPolicy != nil {
				tt.validateNetworkPolicy(t, cl)
			}
		})
	}
}

func TestBuildNetworkPolicy(t *testing.T) {
	tests := []struct {
		name        string
		platform    config.Platform
		expectedDNS struct {
			namespace string
			podLabel  string
			ports     []string
		}
	}{
		{
			name:     "kubernetes platform",
			platform: config.PlatformKubernetes,
			expectedDNS: struct {
				namespace string
				podLabel  string
				ports     []string
			}{
				namespace: "kube-system",
				podLabel:  "k8s-app=kube-dns",
				ports:     []string{"53", "53"},
			},
		},
		{
			name:     "openshift platform",
			platform: config.PlatformOpenShift,
			expectedDNS: struct {
				namespace string
				podLabel  string
				ports     []string
			}{
				namespace: "openshift-dns",
				podLabel:  "dns.operator.openshift.io/daemonset-dns=default",
				ports:     []string{"dns-tcp", "dns"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			reconciler := &Reconciler{
				config:            config.ReconcilerConfig{Platform: tt.platform},
				operatorNamespace: operatorNamespace,
			}

			result := reconciler.buildNetworkPolicy()

			g.Expect(result.Name).To(Equal(constants.NetworkPolicyName))
			g.Expect(result.Namespace).To(Equal(operatorNamespace))
			g.Expect(result.Labels).To(HaveKeyWithValue("control-plane", "sail-operator"))

			// Test the platform-specific DNS egress rule
			g.Expect(result.Spec.Egress).To(HaveLen(2))
			dnsRule := result.Spec.Egress[1]
			g.Expect(dnsRule.To[0].NamespaceSelector.MatchLabels).To(HaveKeyWithValue("kubernetes.io/metadata.name", tt.expectedDNS.namespace))
		})
	}
}

func TestNetworkPolicyEnsurer(t *testing.T) {
	tests := []struct {
		name                  string
		existingObjects       []client.Object
		interceptors          interceptor.Funcs
		expectError           bool
		expectCreation        bool
		validateNetworkPolicy func(*testing.T, client.Client)
	}{
		{
			name:            "creates network policy when it doesn't exist",
			existingObjects: []client.Object{},
			expectError:     false,
			expectCreation:  true,
			validateNetworkPolicy: func(t *testing.T, cl client.Client) {
				g := NewWithT(t)
				np := &networkingv1.NetworkPolicy{}
				err := cl.Get(ctx, types.NamespacedName{
					Name:      constants.NetworkPolicyName,
					Namespace: operatorNamespace,
				}, np)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(np.Spec.PodSelector.MatchLabels).To(HaveKeyWithValue("control-plane", "sail-operator"))
			},
		},
		{
			name:            "returns error when non-NotFound error",
			existingObjects: []client.Object{},
			interceptors: interceptor.Funcs{
				Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					if key.Name == constants.NetworkPolicyName {
						return fmt.Errorf("some error")
					}
					return nil
				},
			},
			expectError: true,
		},
		{
			name:            "returns error when create fails",
			existingObjects: []client.Object{},
			interceptors: interceptor.Funcs{
				Create: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
					return fmt.Errorf("create failed")
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			cfg := newReconcilerTestConfig(t)

			cl := newFakeClientBuilder().
				WithObjects(tt.existingObjects...).
				WithInterceptorFuncs(tt.interceptors).
				Build()

			reconciler := NewReconciler(cfg, cl, scheme.Scheme, operatorNamespace)

			// This simulates what happens when the ensurer starts
			np := reconciler.buildNetworkPolicy()

			existing := &networkingv1.NetworkPolicy{}
			err := reconciler.client.Get(ctx, client.ObjectKeyFromObject(np), existing)
			if err != nil {
				if !errors.IsNotFound(err) {
					if tt.expectError {
						g.Expect(err).To(HaveOccurred())
						return
					}
					t.Errorf("unexpected error: %v", err)
					return
				}

				//  doesn't exist, try to create it
				err = reconciler.client.Create(ctx, np)
				if tt.expectError {
					g.Expect(err).To(HaveOccurred())
					return
				}
				g.Expect(err).ToNot(HaveOccurred())
			}

			if tt.validateNetworkPolicy != nil {
				tt.validateNetworkPolicy(t, cl)
			}
		})
	}
}

func newFakeClientBuilder() *fake.ClientBuilder {
	return fake.NewClientBuilder().
		WithScheme(scheme.Scheme)
}

func newReconcilerTestConfig(t *testing.T) config.ReconcilerConfig {
	return config.ReconcilerConfig{
		ResourceDirectory: t.TempDir(),
		Platform:          config.PlatformKubernetes,
		DefaultProfile:    "",
	}
}
