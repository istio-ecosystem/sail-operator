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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"istio.io/istio/pkg/ptr"
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
	istioUID       = types.UID("my-istio-uid")
	istioNamespace = "my-istio-namespace"
	appNamespace   = "my-app-namespace"
	revisionMeta   = metav1.ObjectMeta{
		Name: revisionName,
		UID:  revisionUID,
		OwnerReferences: []metav1.OwnerReference{
			{
				APIVersion:         "sailoperator.io/v1",
				Kind:               v1.IstioKind,
				Name:               istioName,
				UID:                istioUID,
				Controller:         ptr.Of(true),
				BlockOwnerDeletion: ptr.Of(true),
			},
		},
	}
)

func expectMonitoringLabels(g Gomega, labels map[string]string, monitoring string) {
	g.Expect(labels).To(HaveKeyWithValue(constants.ManagedByLabelKey, constants.ManagedByLabelValue))
	g.Expect(labels).To(HaveKeyWithValue(monitoredByLabel, kubePrometheusValue))
	g.Expect(labels).To(HaveKeyWithValue(releaseLabel, releaseLabelValue))
	g.Expect(labels).To(HaveKeyWithValue(monitoringLabel, monitoring))
}

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

// newNamespaceWithRevLabel creates a namespace with the istio.io/rev label
func newNamespaceWithRevLabel(name, rev string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				constants.IstioRevLabel: rev,
			},
		},
	}
}

// newIstioWithMonitoringEnabled creates an Istio CR with the monitoring annotation set.
func newIstioWithMonitoringEnabled(name, namespace string) *v1.Istio {
	return &v1.Istio{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			UID:  istioUID,
			Annotations: map[string]string{
				constants.MonitoringAnnotationKey: constants.MonitoringAnnotationEnabledValue,
			},
		},
		Spec: v1.IstioSpec{
			Version:   "v1.29.2",
			Namespace: namespace,
		},
	}
}

func testIstio() *v1.Istio {
	return newIstioWithMonitoringEnabled(istioName, istioNamespace)
}

func TestReconcile(t *testing.T) {
	cfg := newReconcilerTestConfig()

	tests := []struct {
		name              string
		istio             *v1.Istio
		revisions         []*v1.IstioRevision
		existingObjects   []client.Object
		expectErr         bool
		expectSMRevision  string // revision name for expected ServiceMonitor (empty = none)
		expectPMNamespace string // namespace where PodMonitor should be created (empty = none)
		expectPMRevision  string // revision name for expected PodMonitor
	}{
		{
			name:  "creates ServiceMonitor and PodMonitor for owned IstioRevision",
			istio: testIstio(),
			revisions: []*v1.IstioRevision{
				{
					ObjectMeta: revisionMeta,
					Spec: v1.IstioRevisionSpec{
						Version:   "v1.24.0",
						Namespace: istioNamespace,
					},
				},
			},
			existingObjects: []client.Object{
				newNamespaceWithRevLabel(appNamespace, revisionName),
			},
			expectSMRevision:  revisionName,
			expectPMNamespace: appNamespace,
			expectPMRevision:  revisionName,
		},
		{
			name:  "creates PodMonitor for namespace with istio-injection=enabled on default revision",
			istio: testIstio(),
			revisions: []*v1.IstioRevision{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: v1.DefaultRevision,
						UID:  revisionUID,
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "sailoperator.io/v1",
								Kind:       v1.IstioKind,
								Name:       istioName,
								UID:        istioUID,
								Controller: ptr.Of(true),
							},
						},
					},
					Spec: v1.IstioRevisionSpec{
						Version:   "v1.24.0",
						Namespace: istioNamespace,
					},
				},
			},
			existingObjects: []client.Object{
				newNamespaceWithInjection(appNamespace),
			},
			expectSMRevision:  v1.DefaultRevision,
			expectPMNamespace: appNamespace,
			expectPMRevision:  v1.DefaultRevision,
		},
		{
			name:  "does not create PodMonitor when istio.io/rev references a different revision",
			istio: testIstio(),
			revisions: []*v1.IstioRevision{
				{
					ObjectMeta: revisionMeta,
					Spec: v1.IstioRevisionSpec{
						Version:   "v1.24.0",
						Namespace: istioNamespace,
					},
				},
			},
			existingObjects: []client.Object{
				newNamespaceWithRevLabel(appNamespace, "other-revision"),
			},
			expectSMRevision: revisionName,
		},
		{
			name:  "skips deleting IstioRevision",
			istio: testIstio(),
			revisions: []*v1.IstioRevision{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:              revisionName,
						UID:               revisionUID,
						DeletionTimestamp: &metav1.Time{Time: metav1.Now().Time},
						Finalizers:        []string{"test-finalizer"},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "sailoperator.io/v1",
								Kind:       v1.IstioKind,
								Name:       istioName,
								UID:        istioUID,
								Controller: ptr.Of(true),
							},
						},
					},
					Spec: v1.IstioRevisionSpec{
						Version:   "v1.24.0",
						Namespace: istioNamespace,
					},
				},
			},
		},
		{
			name:  "no PodMonitor when no namespaces have injection enabled",
			istio: testIstio(),
			revisions: []*v1.IstioRevision{
				{
					ObjectMeta: revisionMeta,
					Spec: v1.IstioRevisionSpec{
						Version:   "v1.24.0",
						Namespace: istioNamespace,
					},
				},
			},
			expectSMRevision: revisionName,
		},
		{
			name: "skips reconciliation when monitoring annotation is absent on Istio CR",
			istio: &v1.Istio{
				ObjectMeta: metav1.ObjectMeta{
					Name: istioName,
					UID:  istioUID,
				},
				Spec: v1.IstioSpec{
					Version:   "v1.29.2",
					Namespace: istioNamespace,
				},
			},
			revisions: []*v1.IstioRevision{
				{
					ObjectMeta: revisionMeta,
					Spec: v1.IstioRevisionSpec{
						Version:   "v1.24.0",
						Namespace: istioNamespace,
					},
				},
			},
			existingObjects: []client.Object{
				newNamespaceWithInjection(appNamespace),
			},
		},
		{
			name: "skips reconciliation when Istio is being deleted",
			istio: &v1.Istio{
				ObjectMeta: metav1.ObjectMeta{
					Name:              istioName,
					UID:               istioUID,
					DeletionTimestamp: &metav1.Time{Time: metav1.Now().Time},
					Finalizers:        []string{"test-finalizer"},
					Annotations: map[string]string{
						constants.MonitoringAnnotationKey: constants.MonitoringAnnotationEnabledValue,
					},
				},
				Spec: v1.IstioSpec{
					Version:   "v1.29.2",
					Namespace: istioNamespace,
				},
			},
			revisions: []*v1.IstioRevision{
				{
					ObjectMeta: revisionMeta,
					Spec: v1.IstioRevisionSpec{
						Version:   "v1.24.0",
						Namespace: istioNamespace,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			objects := tt.existingObjects
			if tt.istio != nil {
				objects = append(objects, tt.istio)
			}
			for _, rev := range tt.revisions {
				objects = append(objects, rev)
			}

			cl := newFakeClientBuilder().
				WithObjects(objects...).
				Build()

			reconciler := NewReconciler(cfg, cl, scheme.Scheme)
			_, err := reconciler.Reconcile(ctx, tt.istio)

			if tt.expectErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}

			if tt.expectSMRevision != "" {
				sm := &monitoringv1.ServiceMonitor{}
				sm.SetGroupVersionKind(monitoringv1.SchemeGroupVersion.WithKind("ServiceMonitor"))
				err := cl.Get(ctx, types.NamespacedName{
					Name:      tt.expectSMRevision + serviceMonitorNameSuffix,
					Namespace: istioNamespace,
				}, sm)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(sm.Name).To(Equal(tt.expectSMRevision + serviceMonitorNameSuffix))
				expectMonitoringLabels(g, sm.Labels, serviceMonitorMonitoring)
			}

			if tt.expectPMNamespace != "" {
				pm := &monitoringv1.PodMonitor{}
				pm.SetGroupVersionKind(monitoringv1.SchemeGroupVersion.WithKind("PodMonitor"))
				err := cl.Get(ctx, types.NamespacedName{
					Name:      tt.expectPMRevision + podMonitorNameSuffix,
					Namespace: tt.expectPMNamespace,
				}, pm)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(pm.Name).To(Equal(tt.expectPMRevision + podMonitorNameSuffix))
				expectMonitoringLabels(g, pm.Labels, podMonitorMonitoring)
				g.Expect(pm.OwnerReferences).To(HaveLen(1))
				g.Expect(pm.OwnerReferences[0].Kind).To(Equal(v1.IstioRevisionKind))
				g.Expect(pm.OwnerReferences[0].Name).To(Equal(tt.expectPMRevision))
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
			name: "leaves existing ServiceMonitor unchanged",
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
						Labels: map[string]string{
							"custom": "user-set",
						},
					},
				}
				sm.SetGroupVersionKind(monitoringv1.SchemeGroupVersion.WithKind("ServiceMonitor"))
				return sm
			}(),
			expectErr:    false,
			expectCreate: false,
			expectUpdate: false,
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
			err := reconciler.reconcileServiceMonitor(ctx, testIstio(), tt.rev)

			if tt.expectErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())

				// Verify the ServiceMonitor exists
				result := &monitoringv1.ServiceMonitor{}
				result.SetGroupVersionKind(monitoringv1.SchemeGroupVersion.WithKind("ServiceMonitor"))
				err := cl.Get(ctx, types.NamespacedName{
					Name:      revisionName + serviceMonitorNameSuffix,
					Namespace: istioNamespace,
				}, result)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(result.Name).To(Equal(revisionName + serviceMonitorNameSuffix))
				if tt.existingSM != nil {
					// Existing resources must not be overwritten so user customizations remain.
					g.Expect(result.Labels).To(HaveKeyWithValue("custom", "user-set"))
					g.Expect(result.ResourceVersion).To(Equal("123"))
				} else {
					expectMonitoringLabels(g, result.Labels, serviceMonitorMonitoring)
				}
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
			name: "creates PodMonitor in namespace with istio.io/rev label",
			rev: &v1.IstioRevision{
				ObjectMeta: revisionMeta,
				Spec: v1.IstioRevisionSpec{
					Version:   "v1.24.0",
					Namespace: istioNamespace,
				},
			},
			existingNamespaces: []client.Object{
				newNamespaceWithRevLabel(appNamespace, revisionName),
			},
			expectErr:          false,
			expectPMNamespaces: []string{appNamespace},
		},
		{
			name: "creates PodMonitor in namespace with istio-injection=enabled for default revision",
			rev: &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: v1.DefaultRevision,
					UID:  revisionUID,
				},
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
				newNamespaceWithRevLabel(appNamespace, revisionName),
				newNamespaceWithRevLabel("another-app-namespace", revisionName),
			},
			expectErr:          false,
			expectPMNamespaces: []string{appNamespace, "another-app-namespace"},
		},
		{
			name: "leaves existing PodMonitor unchanged",
			rev: &v1.IstioRevision{
				ObjectMeta: revisionMeta,
				Spec: v1.IstioRevisionSpec{
					Version:   "v1.24.0",
					Namespace: istioNamespace,
				},
			},
			existingNamespaces: []client.Object{
				newNamespaceWithRevLabel(appNamespace, revisionName),
			},
			existingPM: func() *monitoringv1.PodMonitor {
				pm := &monitoringv1.PodMonitor{
					ObjectMeta: metav1.ObjectMeta{
						Name:            revisionName + podMonitorNameSuffix,
						Namespace:       appNamespace,
						ResourceVersion: "123",
						Labels: map[string]string{
							"custom": "user-set",
						},
					},
				}
				pm.SetGroupVersionKind(monitoringv1.SchemeGroupVersion.WithKind("PodMonitor"))
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
			err := reconciler.reconcilePodMonitors(ctx, testIstio(), tt.rev)

			if tt.expectErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())

				// Verify PodMonitors exist in expected namespaces
				revName := revisionName
				if tt.rev != nil && tt.rev.Name != "" {
					revName = tt.rev.Name
				}
				for _, ns := range tt.expectPMNamespaces {
					pm := &monitoringv1.PodMonitor{}
					pm.SetGroupVersionKind(monitoringv1.SchemeGroupVersion.WithKind("PodMonitor"))
					err := cl.Get(ctx, types.NamespacedName{
						Name:      revName + podMonitorNameSuffix,
						Namespace: ns,
					}, pm)
					g.Expect(err).ToNot(HaveOccurred(), "PodMonitor should exist in namespace %s", ns)
					g.Expect(pm.Name).To(Equal(revName + podMonitorNameSuffix))
					if tt.existingPM != nil {
						// Existing resources must not be overwritten so user customizations remain.
						g.Expect(pm.Labels).To(HaveKeyWithValue("custom", "user-set"))
						g.Expect(pm.ResourceVersion).To(Equal("123"))
					} else {
						expectMonitoringLabels(g, pm.Labels, podMonitorMonitoring)
					}
				}
			}
		})
	}
}

func TestBuildServiceMonitor(t *testing.T) {
	tests := []struct {
		name                   string
		platform               config.Platform
		rev                    *v1.IstioRevision
		expectedName           string
		expectedNS             string
		expectedTargetLabels   []string
		expectedSelectorKey    string
		expectedSelectorValues []string
	}{
		{
			name:     "default revision on kubernetes",
			platform: config.PlatformKubernetes,
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
			expectedName:           "default-istiod-metrics",
			expectedNS:             "istio-system",
			expectedTargetLabels:   []string{"app"},
			expectedSelectorKey:    "istio",
			expectedSelectorValues: []string{"pilot"},
		},
		{
			name:     "named revision",
			platform: config.PlatformKubernetes,
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
			expectedName:           "canary-istiod-metrics",
			expectedNS:             "istio-system",
			expectedTargetLabels:   []string{"app"},
			expectedSelectorKey:    "istio",
			expectedSelectorValues: []string{"pilot"},
		},
		{
			name:     "custom namespace",
			platform: config.PlatformKubernetes,
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
			expectedName:           "default-istiod-metrics",
			expectedNS:             "custom-istio-ns",
			expectedTargetLabels:   []string{"app"},
			expectedSelectorKey:    "istio",
			expectedSelectorValues: []string{"pilot"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			cfg := newReconcilerTestConfig()
			cfg.Platform = tt.platform

			cl := newFakeClientBuilder().Build()
			reconciler := NewReconciler(cfg, cl, scheme.Scheme)
			result := reconciler.buildServiceMonitor(testIstio(), tt.rev)

			g.Expect(result.GetName()).To(Equal(tt.expectedName))
			g.Expect(result.GetNamespace()).To(Equal(tt.expectedNS))

			// Check labels
			labels := result.GetLabels()
			g.Expect(labels["app"]).To(Equal("istiod"))
			expectMonitoringLabels(g, labels, serviceMonitorMonitoring)

			// Check spec.targetLabels and selector
			g.Expect(result.Spec.JobLabel).To(Equal(serviceMonitorJobLabel))
			g.Expect(result.Spec.TargetLabels).To(Equal(tt.expectedTargetLabels))
			g.Expect(result.Spec.Selector.MatchExpressions).To(HaveLen(1))
			g.Expect(result.Spec.Selector.MatchExpressions[0].Key).To(Equal(tt.expectedSelectorKey))
			g.Expect(result.Spec.Selector.MatchExpressions[0].Values).To(Equal(tt.expectedSelectorValues))

			// Check endpoints
			g.Expect(result.Spec.Endpoints).To(HaveLen(1))
			endpoint := result.Spec.Endpoints[0]
			g.Expect(endpoint.Port).To(Equal("http-monitoring"))
			g.Expect(string(endpoint.Interval)).To(Equal("15s"))
			g.Expect(endpoint.RelabelConfigs).To(BeEmpty())

			// Check owner references
			ownerRefs := result.GetOwnerReferences()
			g.Expect(ownerRefs).To(HaveLen(1))
			g.Expect(ownerRefs[0].Kind).To(Equal(v1.IstioRevisionKind))
			g.Expect(ownerRefs[0].Name).To(Equal(tt.rev.Name))
		})
	}
}

func TestBuildPodMonitor(t *testing.T) {
	tests := []struct {
		name                 string
		platform             config.Platform
		istio                *v1.Istio
		rev                  *v1.IstioRevision
		namespace            string
		expectedName         string
		expectedRelabelCount int
		expectedLastLabel    string
	}{
		{
			name:     "kubernetes platform uses upstream istio relabelings",
			platform: config.PlatformKubernetes,
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
			namespace:            "bookinfo",
			expectedName:         "default-proxies-metrics",
			expectedRelabelCount: 7,
			expectedLastLabel:    "pod",
		},
		{
			name:     "openshift platform uses service mesh relabelings",
			platform: config.PlatformOpenShift,
			istio: &v1.Istio{
				ObjectMeta: metav1.ObjectMeta{Name: "prod-mesh", UID: istioUID},
			},
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
			namespace:            "myapp",
			expectedName:         "canary-proxies-metrics",
			expectedRelabelCount: 8,
			expectedLastLabel:    "mesh_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			cfg := newReconcilerTestConfig()
			cfg.Platform = tt.platform

			cl := newFakeClientBuilder().Build()
			reconciler := NewReconciler(cfg, cl, scheme.Scheme)
			istio := tt.istio
			if istio == nil {
				istio = testIstio()
			}
			result := reconciler.buildPodMonitor(istio, tt.rev, tt.namespace)

			g.Expect(result.GetName()).To(Equal(tt.expectedName))
			g.Expect(result.GetNamespace()).To(Equal(tt.namespace))

			// Check labels
			labels := result.GetLabels()
			g.Expect(labels["app"]).To(Equal("istio-proxy"))
			expectMonitoringLabels(g, labels, podMonitorMonitoring)

			ownerRefs := result.GetOwnerReferences()
			g.Expect(ownerRefs).To(HaveLen(1))
			g.Expect(ownerRefs[0].Kind).To(Equal(v1.IstioRevisionKind))
			g.Expect(ownerRefs[0].Name).To(Equal(tt.rev.Name))
			g.Expect(ownerRefs[0].UID).To(Equal(tt.rev.UID))
			g.Expect(ownerRefs[0].Controller).ToNot(BeNil())
			g.Expect(*ownerRefs[0].Controller).To(BeTrue())

			// Check spec.selector.matchExpressions
			g.Expect(result.Spec.JobLabel).To(Equal(podMonitorJobLabel))
			g.Expect(result.Spec.Selector.MatchExpressions).To(HaveLen(1))
			expr := result.Spec.Selector.MatchExpressions[0]
			g.Expect(expr.Key).To(Equal("istio-prometheus-ignore"))
			g.Expect(expr.Operator).To(Equal(metav1.LabelSelectorOpDoesNotExist))

			// Check podMetricsEndpoints
			g.Expect(result.Spec.PodMetricsEndpoints).To(HaveLen(1))
			endpoint := result.Spec.PodMetricsEndpoints[0]
			g.Expect(endpoint.Port).To(BeNil())
			g.Expect(endpoint.Path).To(Equal("/stats/prometheus"))
			g.Expect(string(endpoint.Interval)).To(Equal("15s"))
			g.Expect(endpoint.RelabelConfigs).To(HaveLen(tt.expectedRelabelCount))
			g.Expect(endpoint.RelabelConfigs[0].Regex).To(Equal("istio-proxy"))
			g.Expect(endpoint.RelabelConfigs[len(endpoint.RelabelConfigs)-1].TargetLabel).To(Equal(tt.expectedLastLabel))

			if tt.platform == config.PlatformOpenShift {
				meshID := endpoint.RelabelConfigs[len(endpoint.RelabelConfigs)-1]
				g.Expect(*meshID.Replacement).To(Equal("prod-mesh"))
			}
		})
	}
}

func TestSidecarInjectionNamespacePredicate(t *testing.T) {
	pred := sidecarInjectionNamespacePredicate()

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
			name: "CreateFunc: namespace with istio.io/rev label",
			event: event.CreateEvent{
				Object: newNamespaceWithRevLabel("test-ns", "canary"),
			},
			expected: true,
		},
		{
			name: "UpdateFunc: istio.io/rev label added",
			event: event.UpdateEvent{
				ObjectOld: &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-ns",
					},
				},
				ObjectNew: newNamespaceWithRevLabel("test-ns", "canary"),
			},
			expected: true,
		},
		{
			name: "UpdateFunc: istio.io/rev label changed",
			event: event.UpdateEvent{
				ObjectOld: newNamespaceWithRevLabel("test-ns", "canary"),
				ObjectNew: newNamespaceWithRevLabel("test-ns", "stable"),
			},
			expected: true,
		},
		{
			name: "UpdateFunc: istio.io/rev label removed",
			event: event.UpdateEvent{
				ObjectOld: newNamespaceWithRevLabel("test-ns", "canary"),
				ObjectNew: &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-ns",
					},
				},
			},
			expected: true,
		},
		{
			name: "DeleteFunc: namespace with istio.io/rev label",
			event: event.DeleteEvent{
				Object: newNamespaceWithRevLabel("test-ns", "canary"),
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
		{
			name: "GenericFunc: namespace with injection enabled",
			event: event.GenericEvent{
				Object: newNamespaceWithInjection("test-ns"),
			},
			expected: true,
		},
		{
			name: "GenericFunc: namespace without injection",
			event: event.GenericEvent{
				Object: &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-ns",
					},
				},
			},
			expected: false,
		},
		{
			name: "CreateFunc: nil object",
			event: event.CreateEvent{
				Object: nil,
			},
			expected: false,
		},
		{
			name: "CreateFunc: namespace with nil labels map",
			event: event.CreateEvent{
				Object: &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "test-ns",
						Labels: nil,
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
			case event.GenericEvent:
				result = pred.Generic(e)
			}

			g.Expect(result).To(Equal(tt.expected))
		})
	}
}

func TestMonitoringEnabled(t *testing.T) {
	g := NewWithT(t)
	g.Expect(monitoringEnabled(testIstio())).To(BeTrue())
	g.Expect(monitoringEnabled(&v1.Istio{
		ObjectMeta: metav1.ObjectMeta{Name: istioName},
	})).To(BeFalse())
}

func TestNamespacesForRevision(t *testing.T) {
	cfg := newReconcilerTestConfig()

	t.Run("skips namespaces where istio-injection takes precedence over istio.io/rev", func(t *testing.T) {
		g := NewWithT(t)
		both := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "both-labels",
				Labels: map[string]string{
					constants.IstioInjectionLabel: constants.IstioInjectionEnabledValue,
					constants.IstioRevLabel:       revisionName,
				},
			},
		}
		cl := newFakeClientBuilder().WithObjects(
			both,
			newNamespaceWithRevLabel(appNamespace, revisionName),
		).Build()
		r := NewReconciler(cfg, cl, scheme.Scheme)
		namespaces, err := r.namespacesForRevision(ctx, &v1.IstioRevision{ObjectMeta: revisionMeta})
		g.Expect(err).ToNot(HaveOccurred())
		g.Expect(namespaces).To(HaveLen(1))
		g.Expect(namespaces[0].Name).To(Equal(appNamespace))
	})

	t.Run("default revision includes injection-labeled namespaces and skips rev-only duplicates", func(t *testing.T) {
		g := NewWithT(t)
		// Appears in both the istio-injection=enabled list and the istio.io/rev=default list.
		overlap := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "overlap-ns",
				Labels: map[string]string{
					constants.IstioInjectionLabel: constants.IstioInjectionEnabledValue,
					constants.IstioRevLabel:       v1.DefaultRevision,
				},
			},
		}
		injectionOnly := newNamespaceWithInjection("injection-only")
		cl := newFakeClientBuilder().WithObjects(overlap, injectionOnly).Build()
		r := NewReconciler(cfg, cl, scheme.Scheme)
		namespaces, err := r.namespacesForRevision(ctx, &v1.IstioRevision{
			ObjectMeta: metav1.ObjectMeta{Name: v1.DefaultRevision},
		})
		g.Expect(err).ToNot(HaveOccurred())
		names := make([]string, 0, len(namespaces))
		for _, ns := range namespaces {
			names = append(names, ns.Name)
		}
		g.Expect(names).To(ConsistOf("overlap-ns", "injection-only"))
	})
}

func TestMapNamespaceToReconcileRequest(t *testing.T) {
	cfg := newReconcilerTestConfig()
	istioA := &v1.Istio{ObjectMeta: metav1.ObjectMeta{Name: "mesh-a", UID: "uid-a"}}
	istioB := &v1.Istio{ObjectMeta: metav1.ObjectMeta{Name: "mesh-b", UID: "uid-b"}}
	defaultRev := &v1.IstioRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1.DefaultRevision,
			UID:  "default-rev-uid",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: v1.GroupVersion.String(),
					Kind:       v1.IstioKind,
					Name:       istioA.Name,
					UID:        istioA.UID,
					Controller: ptr.Of(true),
				},
			},
		},
	}
	namedRev := &v1.IstioRevision{ObjectMeta: revisionMeta}
	unownedRev := &v1.IstioRevision{
		ObjectMeta: metav1.ObjectMeta{Name: "unowned"},
	}

	tests := []struct {
		name      string
		objects   []client.Object
		namespace *corev1.Namespace
		want      []reconcile.Request
	}{
		{
			name:      "enqueues owning Istio for istio-injection=enabled",
			objects:   []client.Object{istioA, istioB, defaultRev, namedRev},
			namespace: newNamespaceWithInjection(appNamespace),
			want: []reconcile.Request{
				{NamespacedName: types.NamespacedName{Name: istioA.Name}},
			},
		},
		{
			name:      "enqueues owning Istio for istio.io/rev",
			objects:   []client.Object{istioA, istioB, defaultRev, namedRev},
			namespace: newNamespaceWithRevLabel(appNamespace, revisionName),
			want: []reconcile.Request{
				{NamespacedName: types.NamespacedName{Name: istioName}},
			},
		},
		{
			name:    "istio-injection takes precedence over istio.io/rev",
			objects: []client.Object{istioA, istioB, defaultRev, namedRev},
			namespace: &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: appNamespace,
					Labels: map[string]string{
						constants.IstioInjectionLabel: constants.IstioInjectionEnabledValue,
						constants.IstioRevLabel:       revisionName,
					},
				},
			},
			want: []reconcile.Request{
				{NamespacedName: types.NamespacedName{Name: istioA.Name}},
			},
		},
		{
			name:      "no request when namespace has no injection labels",
			objects:   []client.Object{istioA, defaultRev},
			namespace: &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: appNamespace}},
			want:      nil,
		},
		{
			name:      "no request when referenced revision does not exist",
			objects:   []client.Object{istioA, istioB},
			namespace: newNamespaceWithRevLabel(appNamespace, "missing-revision"),
			want:      nil,
		},
		{
			name:      "no request when revision has no Istio owner",
			objects:   []client.Object{unownedRev},
			namespace: newNamespaceWithRevLabel(appNamespace, "unowned"),
			want:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			cl := newFakeClientBuilder().WithObjects(tt.objects...).Build()
			r := NewReconciler(cfg, cl, scheme.Scheme)
			reqs := r.mapNamespaceToReconcileRequest(ctx, tt.namespace)
			if tt.want == nil {
				g.Expect(reqs).To(BeEmpty())
				return
			}
			g.Expect(reqs).To(ConsistOf(tt.want))
		})
	}
}

func newReconcilerTestConfig() config.ReconcilerConfig {
	return config.ReconcilerConfig{
		MaxConcurrentReconciles: 1,
		Platform:                config.PlatformKubernetes,
	}
}
