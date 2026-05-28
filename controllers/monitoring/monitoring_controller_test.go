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

package monitoring

import (
	"context"
	"fmt"
	"testing"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func newFakeClientBuilder() *fake.ClientBuilder {
	return fake.NewClientBuilder().
		WithScheme(scheme.Scheme)
}

var (
	ctx            = context.Background()
	revisionName   = "my-revision"
	revisionUID    = types.UID("my-revision-uid")
	istioName      = "my-istio"
	istioNamespace = "my-istio-namespace"
	appNamespace   = "my-app-namespace"
	revisionKey    = types.NamespacedName{Name: revisionName}
	revisionMeta   = metav1.ObjectMeta{
		Name: revisionName,
		UID:  revisionUID,
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion: "sailoperator.io/v1",
				Kind:       v1.IstioKind,
				Name:       istioName,
			},
		},
	}

	// testRhobsGV is the GroupVersion for COO monitoring resources used in tests
	testRhobsGV = schema.GroupVersion{Group: "monitoring.rhobs", Version: "v1"}
)

// newNamespaceWithInjection creates a namespace with the istio-injection=enabled label
func newNamespaceWithInjection(name string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				constants.IstioInjectionLabel: constants.IstioInjectionEnabledValue,
			},
		},
	}
}

// newIstioWithMonitoringEnabled creates an Istio CR with monitoring enabled
func newIstioWithMonitoringEnabled(name, namespace string) *v1.Istio {
	return &v1.Istio{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: v1.IstioSpec{
			Version:   "v1.29.2",
			Namespace: namespace,
			Monitoring: &v1.MonitoringConfig{
				Enabled: true,
			},
		},
	}
}

func TestReconcile(t *testing.T) {
	cfg := newReconcilerTestConfig()

	tests := []struct {
		name              string
		rev               *v1.IstioRevision
		existingObjects   []client.Object
		expectErr         bool
		expectSMCreated   bool
		expectPMNamespace string // namespace where PodMonitor should be created (empty = none expected)
	}{
		{
			name: "creates ServiceMonitor and PodMonitor for new IstioRevision",
			rev: &v1.IstioRevision{
				ObjectMeta: revisionMeta,
				Spec: v1.IstioRevisionSpec{
					Version:   "v1.24.0",
					Namespace: istioNamespace,
				},
			},
			existingObjects: []client.Object{
				newIstioWithMonitoringEnabled(istioName, istioNamespace),
				newNamespaceWithInjection(appNamespace),
			},
			expectErr:         false,
			expectSMCreated:   true,
			expectPMNamespace: appNamespace,
		},
		{
			name: "skips reconciliation for deleting IstioRevision",
			rev: &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name:              revisionName,
					UID:               revisionUID,
					DeletionTimestamp: &metav1.Time{Time: metav1.Now().Time},
					Finalizers:        []string{"test-finalizer"},
				},
				Spec: v1.IstioRevisionSpec{
					Version:   "v1.24.0",
					Namespace: istioNamespace,
				},
			},
			existingObjects:   []client.Object{},
			expectErr:         false,
			expectSMCreated:   false,
			expectPMNamespace: "",
		},
		{
			name: "does not create PodMonitor in istio-system namespace",
			rev: &v1.IstioRevision{
				ObjectMeta: revisionMeta,
				Spec: v1.IstioRevisionSpec{
					Version:   "v1.24.0",
					Namespace: istioNamespace,
				},
			},
			existingObjects: []client.Object{
				newIstioWithMonitoringEnabled(istioName, istioNamespace),
				newNamespaceWithInjection(istioNamespace), // control plane namespace with injection label
			},
			expectErr:         false,
			expectSMCreated:   true,
			expectPMNamespace: "", // no PodMonitor should be created
		},
		{
			name: "no PodMonitor when no namespaces have injection enabled",
			rev: &v1.IstioRevision{
				ObjectMeta: revisionMeta,
				Spec: v1.IstioRevisionSpec{
					Version:   "v1.24.0",
					Namespace: istioNamespace,
				},
			},
			existingObjects: []client.Object{
				newIstioWithMonitoringEnabled(istioName, istioNamespace),
			},
			expectErr:         false,
			expectSMCreated:   true,
			expectPMNamespace: "",
		},
		{
			name: "skips reconciliation when monitoring is disabled in Istio CR",
			rev: &v1.IstioRevision{
				ObjectMeta: revisionMeta,
				Spec: v1.IstioRevisionSpec{
					Version:   "v1.24.0",
					Namespace: istioNamespace,
				},
			},
			existingObjects: []client.Object{
				// Istio CR with monitoring disabled (nil)
				&v1.Istio{
					ObjectMeta: metav1.ObjectMeta{
						Name: istioName,
					},
					Spec: v1.IstioSpec{
						Version:   "v1.29.2",
						Namespace: istioNamespace,
					},
				},
				newNamespaceWithInjection(appNamespace),
			},
			expectErr:         false,
			expectSMCreated:   false,
			expectPMNamespace: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			objects := tt.existingObjects
			if tt.rev != nil {
				objects = append(objects, tt.rev)
			}

			cl := newFakeClientBuilder().
				WithObjects(objects...).
				Build()

			reconciler := NewReconciler(cfg, cl, scheme.Scheme)
			_, err := reconciler.Reconcile(ctx, tt.rev)

			if tt.expectErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}

			// Check ServiceMonitor creation
			if tt.expectSMCreated {
				sm := &monitoringv1.ServiceMonitor{}
				sm.SetGroupVersionKind(testRhobsGV.WithKind("ServiceMonitor"))
				err := cl.Get(ctx, types.NamespacedName{
					Name:      revisionName + serviceMonitorNameSuffix,
					Namespace: istioNamespace,
				}, sm)
				g.Expect(err).ToNot(HaveOccurred())
				// Verify the ServiceMonitor has expected content
				g.Expect(sm.Name).To(Equal(revisionName + serviceMonitorNameSuffix))
				g.Expect(sm.Labels["monitored-by"]).To(Equal("coo-prometheus"))
			}

			// Check PodMonitor creation
			if tt.expectPMNamespace != "" {
				pm := &monitoringv1.PodMonitor{}
				pm.SetGroupVersionKind(testRhobsGV.WithKind("PodMonitor"))
				err := cl.Get(ctx, types.NamespacedName{
					Name:      revisionName + podMonitorNameSuffix,
					Namespace: tt.expectPMNamespace,
				}, pm)
				g.Expect(err).ToNot(HaveOccurred())
				// Verify the PodMonitor has expected content
				g.Expect(pm.Name).To(Equal(revisionName + podMonitorNameSuffix))
				g.Expect(pm.Labels["monitored-by"]).To(Equal("coo-prometheus"))
			}
		})
	}
}

func TestReconcileServiceMonitor(t *testing.T) {
	cfg := newReconcilerTestConfig()

	tests := []struct {
		name           string
		rev            *v1.IstioRevision
		existingSM     *monitoringv1.ServiceMonitor
		clientGetError error
		expectErr      bool
		expectCreate   bool
		expectUpdate   bool
	}{
		{
			name: "creates new ServiceMonitor",
			rev: &v1.IstioRevision{
				ObjectMeta: revisionMeta,
				Spec: v1.IstioRevisionSpec{
					Version:   "v1.24.0",
					Namespace: istioNamespace,
				},
			},
			existingSM:   nil,
			expectErr:    false,
			expectCreate: true,
			expectUpdate: false,
		},
		{
			name: "updates existing ServiceMonitor",
			rev: &v1.IstioRevision{
				ObjectMeta: revisionMeta,
				Spec: v1.IstioRevisionSpec{
					Version:   "v1.24.0",
					Namespace: istioNamespace,
				},
			},
			existingSM: func() *monitoringv1.ServiceMonitor {
				sm := &monitoringv1.ServiceMonitor{
					ObjectMeta: metav1.ObjectMeta{
						Name:            revisionName + serviceMonitorNameSuffix,
						Namespace:       istioNamespace,
						ResourceVersion: "123",
					},
				}
				sm.SetGroupVersionKind(testRhobsGV.WithKind("ServiceMonitor"))
				return sm
			}(),
			expectErr:    false,
			expectCreate: false,
			expectUpdate: true,
		},
		{
			name: "returns error on client Get failure",
			rev: &v1.IstioRevision{
				ObjectMeta: revisionMeta,
				Spec: v1.IstioRevisionSpec{
					Version:   "v1.24.0",
					Namespace: istioNamespace,
				},
			},
			clientGetError: fmt.Errorf("test error"),
			expectErr:      true,
			expectCreate:   false,
			expectUpdate:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			var objects []client.Object
			objects = append(objects, tt.rev)
			if tt.existingSM != nil {
				objects = append(objects, tt.existingSM)
			}

			builder := newFakeClientBuilder().WithObjects(objects...)

			if tt.clientGetError != nil {
				builder = builder.WithInterceptorFuncs(interceptor.Funcs{
					Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
						if _, ok := obj.(*monitoringv1.ServiceMonitor); ok {
							return tt.clientGetError
						}
						return c.Get(ctx, key, obj, opts...)
					},
				})
			}

			cl := builder.Build()
			reconciler := NewReconciler(cfg, cl, scheme.Scheme)
			err := reconciler.reconcileServiceMonitor(ctx, tt.rev)

			if tt.expectErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())

				// Verify the ServiceMonitor exists
				result := &monitoringv1.ServiceMonitor{}
				result.SetGroupVersionKind(testRhobsGV.WithKind("ServiceMonitor"))
				err := cl.Get(ctx, types.NamespacedName{
					Name:      revisionName + serviceMonitorNameSuffix,
					Namespace: istioNamespace,
				}, result)
				g.Expect(err).ToNot(HaveOccurred())
				// Verify expected content
				g.Expect(result.Name).To(Equal(revisionName + serviceMonitorNameSuffix))
				g.Expect(result.Labels["monitored-by"]).To(Equal("coo-prometheus"))
			}
		})
	}
}

func TestReconcilePodMonitors(t *testing.T) {
	cfg := newReconcilerTestConfig()

	tests := []struct {
		name               string
		rev                *v1.IstioRevision
		existingNamespaces []client.Object
		existingPM         *monitoringv1.PodMonitor
		clientListError    error
		expectErr          bool
		expectPMNamespaces []string // namespaces where PodMonitors should exist
	}{
		{
			name: "creates PodMonitor in namespace with injection enabled",
			rev: &v1.IstioRevision{
				ObjectMeta: revisionMeta,
				Spec: v1.IstioRevisionSpec{
					Version:   "v1.24.0",
					Namespace: istioNamespace,
				},
			},
			existingNamespaces: []client.Object{
				newNamespaceWithInjection(appNamespace),
			},
			expectErr:          false,
			expectPMNamespaces: []string{appNamespace},
		},
		{
			name: "creates PodMonitor in multiple namespaces",
			rev: &v1.IstioRevision{
				ObjectMeta: revisionMeta,
				Spec: v1.IstioRevisionSpec{
					Version:   "v1.24.0",
					Namespace: istioNamespace,
				},
			},
			existingNamespaces: []client.Object{
				newNamespaceWithInjection(appNamespace),
				newNamespaceWithInjection("another-app-namespace"),
			},
			expectErr:          false,
			expectPMNamespaces: []string{appNamespace, "another-app-namespace"},
		},
		{
			name: "skips control plane namespace",
			rev: &v1.IstioRevision{
				ObjectMeta: revisionMeta,
				Spec: v1.IstioRevisionSpec{
					Version:   "v1.24.0",
					Namespace: istioNamespace,
				},
			},
			existingNamespaces: []client.Object{
				newNamespaceWithInjection(istioNamespace), // should be skipped
				newNamespaceWithInjection(appNamespace),
			},
			expectErr:          false,
			expectPMNamespaces: []string{appNamespace}, // only app namespace
		},
		{
			name: "updates existing PodMonitor",
			rev: &v1.IstioRevision{
				ObjectMeta: revisionMeta,
				Spec: v1.IstioRevisionSpec{
					Version:   "v1.24.0",
					Namespace: istioNamespace,
				},
			},
			existingNamespaces: []client.Object{
				newNamespaceWithInjection(appNamespace),
			},
			existingPM: func() *monitoringv1.PodMonitor {
				pm := &monitoringv1.PodMonitor{
					ObjectMeta: metav1.ObjectMeta{
						Name:            revisionName + podMonitorNameSuffix,
						Namespace:       appNamespace,
						ResourceVersion: "123",
					},
				}
				pm.SetGroupVersionKind(testRhobsGV.WithKind("PodMonitor"))
				return pm
			}(),
			expectErr:          false,
			expectPMNamespaces: []string{appNamespace},
		},
		{
			name: "no PodMonitor when no namespaces with injection",
			rev: &v1.IstioRevision{
				ObjectMeta: revisionMeta,
				Spec: v1.IstioRevisionSpec{
					Version:   "v1.24.0",
					Namespace: istioNamespace,
				},
			},
			existingNamespaces: []client.Object{
				// Namespace without injection label
				&corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: appNamespace,
					},
				},
			},
			expectErr:          false,
			expectPMNamespaces: []string{},
		},
		{
			name: "returns error on client List failure",
			rev: &v1.IstioRevision{
				ObjectMeta: revisionMeta,
				Spec: v1.IstioRevisionSpec{
					Version:   "v1.24.0",
					Namespace: istioNamespace,
				},
			},
			clientListError: fmt.Errorf("test error"),
			expectErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			var objects []client.Object
			objects = append(objects, tt.rev)
			objects = append(objects, tt.existingNamespaces...)
			if tt.existingPM != nil {
				objects = append(objects, tt.existingPM)
			}

			builder := newFakeClientBuilder().WithObjects(objects...)

			if tt.clientListError != nil {
				builder = builder.WithInterceptorFuncs(interceptor.Funcs{
					List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
						if _, ok := list.(*corev1.NamespaceList); ok {
							return tt.clientListError
						}
						return c.List(ctx, list, opts...)
					},
				})
			}

			cl := builder.Build()
			reconciler := NewReconciler(cfg, cl, scheme.Scheme)
			err := reconciler.reconcilePodMonitors(ctx, tt.rev)

			if tt.expectErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())

				// Verify PodMonitors exist in expected namespaces
				for _, ns := range tt.expectPMNamespaces {
					pm := &monitoringv1.PodMonitor{}
					pm.SetGroupVersionKind(testRhobsGV.WithKind("PodMonitor"))
					err := cl.Get(ctx, types.NamespacedName{
						Name:      revisionName + podMonitorNameSuffix,
						Namespace: ns,
					}, pm)
					g.Expect(err).ToNot(HaveOccurred(), "PodMonitor should exist in namespace %s", ns)
					// Verify expected content
					g.Expect(pm.Name).To(Equal(revisionName + podMonitorNameSuffix))
					g.Expect(pm.Labels["monitored-by"]).To(Equal("coo-prometheus"))
				}
			}
		})
	}
}

func TestBuildServiceMonitor(t *testing.T) {
	cfg := newReconcilerTestConfig()

	tests := []struct {
		name                   string
		rev                    *v1.IstioRevision
		expectedName           string
		expectedNS             string
		expectedSelectorLabels map[string]string
	}{
		{
			name: "default revision",
			rev: &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
					UID:  "test-uid",
				},
				Spec: v1.IstioRevisionSpec{
					Version:   "v1.24.0",
					Namespace: "istio-system",
				},
			},
			expectedName:           "default-istiod",
			expectedNS:             "istio-system",
			expectedSelectorLabels: map[string]string{"app": "istiod"},
		},
		{
			name: "named revision",
			rev: &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "canary",
					UID:  "test-uid",
				},
				Spec: v1.IstioRevisionSpec{
					Version:   "v1.25.0",
					Namespace: "istio-system",
				},
			},
			expectedName:           "canary-istiod",
			expectedNS:             "istio-system",
			expectedSelectorLabels: map[string]string{"app": "istiod"},
		},
		{
			name: "custom namespace",
			rev: &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
					UID:  "test-uid",
				},
				Spec: v1.IstioRevisionSpec{
					Version:   "v1.24.0",
					Namespace: "custom-istio-ns",
				},
			},
			expectedName:           "default-istiod",
			expectedNS:             "custom-istio-ns",
			expectedSelectorLabels: map[string]string{"app": "istiod"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			cl := newFakeClientBuilder().Build()
			reconciler := NewReconciler(cfg, cl, scheme.Scheme)
			result := reconciler.buildServiceMonitor(tt.rev)

			g.Expect(result.GetObjectKind().GroupVersionKind().Group).To(Equal("monitoring.rhobs"))
			g.Expect(result.GetObjectKind().GroupVersionKind().Version).To(Equal("v1"))
			g.Expect(result.GetObjectKind().GroupVersionKind().Kind).To(Equal("ServiceMonitor"))
			g.Expect(result.GetName()).To(Equal(tt.expectedName))
			g.Expect(result.GetNamespace()).To(Equal(tt.expectedNS))

			// Check labels
			labels := result.GetLabels()
			g.Expect(labels["app"]).To(Equal("istiod"))
			g.Expect(labels["monitored-by"]).To(Equal("coo-prometheus"))

			// Check spec.selector
			g.Expect(result.Spec.Selector.MatchLabels).To(Equal(tt.expectedSelectorLabels))

			// Check endpoints
			g.Expect(len(result.Spec.Endpoints)).To(Equal(1))
			endpoint := result.Spec.Endpoints[0]
			g.Expect(endpoint.Port).To(Equal("http-monitoring"))
			g.Expect(endpoint.Path).To(Equal("/metrics"))
			g.Expect(string(*endpoint.Scheme)).To(Equal("http"))
			g.Expect(string(endpoint.Interval)).To(Equal("30s"))

			// Check owner references
			ownerRefs := result.GetOwnerReferences()
			g.Expect(len(ownerRefs)).To(Equal(1))
			g.Expect(ownerRefs[0].Kind).To(Equal(v1.IstioRevisionKind))
			g.Expect(ownerRefs[0].Name).To(Equal(tt.rev.Name))
		})
	}
}

func TestBuildPodMonitor(t *testing.T) {
	cfg := newReconcilerTestConfig()

	tests := []struct {
		name         string
		rev          *v1.IstioRevision
		namespace    string
		expectedName string
	}{
		{
			name: "builds PodMonitor for application namespace",
			rev: &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
					UID:  "test-uid",
				},
				Spec: v1.IstioRevisionSpec{
					Version:   "v1.24.0",
					Namespace: "istio-system",
				},
			},
			namespace:    "bookinfo",
			expectedName: "default-proxies",
		},
		{
			name: "named revision",
			rev: &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "canary",
					UID:  "test-uid",
				},
				Spec: v1.IstioRevisionSpec{
					Version:   "v1.25.0",
					Namespace: "istio-system",
				},
			},
			namespace:    "myapp",
			expectedName: "canary-proxies",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			cl := newFakeClientBuilder().Build()
			reconciler := NewReconciler(cfg, cl, scheme.Scheme)
			result := reconciler.buildPodMonitor(tt.rev, tt.namespace)

			g.Expect(result.GetObjectKind().GroupVersionKind().Group).To(Equal("monitoring.rhobs"))
			g.Expect(result.GetObjectKind().GroupVersionKind().Version).To(Equal("v1"))
			g.Expect(result.GetObjectKind().GroupVersionKind().Kind).To(Equal("PodMonitor"))
			g.Expect(result.GetName()).To(Equal(tt.expectedName))
			g.Expect(result.GetNamespace()).To(Equal(tt.namespace))

			// Check labels
			labels := result.GetLabels()
			g.Expect(labels["app"]).To(Equal("istio-proxy"))
			g.Expect(labels["monitored-by"]).To(Equal("coo-prometheus"))

			// PodMonitor should NOT have owner references (cross-namespace)
			ownerRefs := result.GetOwnerReferences()
			g.Expect(len(ownerRefs)).To(Equal(0))

			// Check spec.selector.matchExpressions
			g.Expect(len(result.Spec.Selector.MatchExpressions)).To(Equal(1))
			expr := result.Spec.Selector.MatchExpressions[0]
			g.Expect(expr.Key).To(Equal("security.istio.io/tlsMode"))
			g.Expect(expr.Operator).To(Equal(metav1.LabelSelectorOpExists))

			// Check podMetricsEndpoints
			g.Expect(len(result.Spec.PodMetricsEndpoints)).To(Equal(1))
			endpoint := result.Spec.PodMetricsEndpoints[0]
			g.Expect(*endpoint.Port).To(Equal("http-envoy-prom"))
			g.Expect(endpoint.Path).To(Equal("/stats/prometheus"))
			g.Expect(string(*endpoint.Scheme)).To(Equal("http"))
			g.Expect(string(endpoint.Interval)).To(Equal("30s"))
			g.Expect(string(endpoint.ScrapeTimeout)).To(Equal("10s"))
			g.Expect(endpoint.HonorLabels).To(BeTrue())
			g.Expect(*endpoint.FilterRunning).To(BeTrue())
		})
	}
}

func TestInjectionEnabledPredicate(t *testing.T) {
	pred := injectionEnabledPredicate()

	tests := []struct {
		name     string
		event    interface{}
		expected bool
	}{
		{
			name: "CreateFunc: namespace with injection enabled",
			event: event.CreateEvent{
				Object: newNamespaceWithInjection("test-ns"),
			},
			expected: true,
		},
		{
			name: "CreateFunc: namespace without injection label",
			event: event.CreateEvent{
				Object: &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-ns",
					},
				},
			},
			expected: false,
		},
		{
			name: "UpdateFunc: injection label added",
			event: event.UpdateEvent{
				ObjectOld: &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-ns",
					},
				},
				ObjectNew: newNamespaceWithInjection("test-ns"),
			},
			expected: true,
		},
		{
			name: "UpdateFunc: injection label removed",
			event: event.UpdateEvent{
				ObjectOld: newNamespaceWithInjection("test-ns"),
				ObjectNew: &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-ns",
					},
				},
			},
			expected: true,
		},
		{
			name: "UpdateFunc: no change (both have injection)",
			event: event.UpdateEvent{
				ObjectOld: newNamespaceWithInjection("test-ns"),
				ObjectNew: newNamespaceWithInjection("test-ns"),
			},
			expected: false,
		},
		{
			name: "UpdateFunc: no change (neither has injection)",
			event: event.UpdateEvent{
				ObjectOld: &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-ns",
					},
				},
				ObjectNew: &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-ns",
					},
				},
			},
			expected: false,
		},
		{
			name: "UpdateFunc: injection changed from enabled to disabled",
			event: event.UpdateEvent{
				ObjectOld: newNamespaceWithInjection("test-ns"),
				ObjectNew: &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-ns",
						Labels: map[string]string{
							constants.IstioInjectionLabel: "disabled",
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "DeleteFunc: namespace with injection enabled",
			event: event.DeleteEvent{
				Object: newNamespaceWithInjection("test-ns"),
			},
			expected: true,
		},
		{
			name: "DeleteFunc: namespace without injection",
			event: event.DeleteEvent{
				Object: &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-ns",
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			var result bool
			switch e := tt.event.(type) {
			case event.CreateEvent:
				result = pred.Create(e)
			case event.UpdateEvent:
				result = pred.Update(e)
			case event.DeleteEvent:
				result = pred.Delete(e)
			}

			g.Expect(result).To(Equal(tt.expected))
		})
	}
}

func newReconcilerTestConfig() config.ReconcilerConfig {
	return config.ReconcilerConfig{
		MaxConcurrentReconciles: 1,
	}
}
