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

func testMonitoringGV(platform config.Platform) schema.GroupVersion {
	if platform == config.PlatformOpenShift {
		return testRhobsGV
	}
	return monitoringv1.SchemeGroupVersion
}

func expectMonitoringLabels(g Gomega, platform config.Platform, labels map[string]string, helmRelease string) {
	g.Expect(labels).To(HaveKeyWithValue(constants.ManagedByLabelKey, constants.ManagedByLabelValue))

	if platform == config.PlatformOpenShift {
		value := cooPrometheusValue
		if helmRelease != "" {
			value = helmRelease
		}
		g.Expect(labels).To(HaveKeyWithValue(monitoredByLabel, value))
		g.Expect(labels).NotTo(HaveKey(helmReleaseLabel))
		return
	}

	if helmRelease != "" {
		g.Expect(labels).To(HaveKeyWithValue(helmReleaseLabel, helmRelease))
		g.Expect(labels).To(HaveKeyWithValue(monitoredByLabel, helmRelease))
	} else {
		g.Expect(labels).To(HaveKeyWithValue(monitoredByLabel, kubePrometheusValue))
		g.Expect(labels).NotTo(HaveKey(helmReleaseLabel))
	}
}

func testMonitoringConfig() *v1.MonitoringConfig {
	return &v1.MonitoringConfig{
		Enabled:     true,
		MonitoredBy: "test-release",
	}
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
				Enabled:     true,
				MonitoredBy: "test-release",
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
				newNamespaceWithRevLabel(appNamespace, revisionName),
			},
			expectErr:         false,
			expectSMCreated:   true,
			expectPMNamespace: appNamespace,
		},
		{
			name: "creates PodMonitor for namespace with istio-injection=enabled on default revision",
			rev: &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: v1.DefaultRevision,
					UID:  revisionUID,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "sailoperator.io/v1",
							Kind:       v1.IstioKind,
							Name:       istioName,
						},
					},
				},
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
			name: "does not create PodMonitor when istio.io/rev references a different revision",
			rev: &v1.IstioRevision{
				ObjectMeta: revisionMeta,
				Spec: v1.IstioRevisionSpec{
					Version:   "v1.24.0",
					Namespace: istioNamespace,
				},
			},
			existingObjects: []client.Object{
				newIstioWithMonitoringEnabled(istioName, istioNamespace),
				newNamespaceWithRevLabel(appNamespace, "other-revision"),
			},
			expectErr:         false,
			expectSMCreated:   true,
			expectPMNamespace: "",
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
				sm.SetGroupVersionKind(testMonitoringGV(cfg.Platform).WithKind("ServiceMonitor"))
				revName := revisionName
				if tt.rev != nil && tt.rev.Name != "" {
					revName = tt.rev.Name
				}
				err := cl.Get(ctx, types.NamespacedName{
					Name:      revName + serviceMonitorNameSuffix,
					Namespace: istioNamespace,
				}, sm)
				g.Expect(err).ToNot(HaveOccurred())
				// Verify the ServiceMonitor has expected content
				g.Expect(sm.Name).To(Equal(revName + serviceMonitorNameSuffix))
				expectMonitoringLabels(g, cfg.Platform, sm.Labels, "test-release")
			}

			// Check PodMonitor creation
			if tt.expectPMNamespace != "" {
				pm := &monitoringv1.PodMonitor{}
				pm.SetGroupVersionKind(testMonitoringGV(cfg.Platform).WithKind("PodMonitor"))
				revName := revisionName
				if tt.rev != nil && tt.rev.Name != "" {
					revName = tt.rev.Name
				}
				err := cl.Get(ctx, types.NamespacedName{
					Name:      revName + podMonitorNameSuffix,
					Namespace: tt.expectPMNamespace,
				}, pm)
				g.Expect(err).ToNot(HaveOccurred())
				// Verify the PodMonitor has expected content
				g.Expect(pm.Name).To(Equal(revName + podMonitorNameSuffix))
				expectMonitoringLabels(g, cfg.Platform, pm.Labels, "test-release")
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
				sm.SetGroupVersionKind(testMonitoringGV(cfg.Platform).WithKind("ServiceMonitor"))
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
			err := reconciler.reconcileServiceMonitor(ctx, tt.rev, testMonitoringConfig())

			if tt.expectErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())

				// Verify the ServiceMonitor exists
				result := &monitoringv1.ServiceMonitor{}
				result.SetGroupVersionKind(testMonitoringGV(cfg.Platform).WithKind("ServiceMonitor"))
				err := cl.Get(ctx, types.NamespacedName{
					Name:      revisionName + serviceMonitorNameSuffix,
					Namespace: istioNamespace,
				}, result)
				g.Expect(err).ToNot(HaveOccurred())
				// Verify expected content
				g.Expect(result.Name).To(Equal(revisionName + serviceMonitorNameSuffix))
				expectMonitoringLabels(g, cfg.Platform, result.Labels, "test-release")
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
			name: "skips control plane namespace",
			rev: &v1.IstioRevision{
				ObjectMeta: revisionMeta,
				Spec: v1.IstioRevisionSpec{
					Version:   "v1.24.0",
					Namespace: istioNamespace,
				},
			},
			existingNamespaces: []client.Object{
				newNamespaceWithRevLabel(istioNamespace, revisionName), // should be skipped
				newNamespaceWithRevLabel(appNamespace, revisionName),
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
				newNamespaceWithRevLabel(appNamespace, revisionName),
			},
			existingPM: func() *monitoringv1.PodMonitor {
				pm := &monitoringv1.PodMonitor{
					ObjectMeta: metav1.ObjectMeta{
						Name:            revisionName + podMonitorNameSuffix,
						Namespace:       appNamespace,
						ResourceVersion: "123",
					},
				}
				pm.SetGroupVersionKind(testMonitoringGV(config.PlatformKubernetes).WithKind("PodMonitor"))
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
			err := reconciler.reconcilePodMonitors(ctx, tt.rev, testMonitoringConfig())

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
					pm.SetGroupVersionKind(testMonitoringGV(cfg.Platform).WithKind("PodMonitor"))
					err := cl.Get(ctx, types.NamespacedName{
						Name:      revName + podMonitorNameSuffix,
						Namespace: ns,
					}, pm)
					g.Expect(err).ToNot(HaveOccurred(), "PodMonitor should exist in namespace %s", ns)
					// Verify expected content
					g.Expect(pm.Name).To(Equal(revName + podMonitorNameSuffix))
					expectMonitoringLabels(g, cfg.Platform, pm.Labels, "test-release")
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
			expectedName:           "default-istiod",
			expectedNS:             "istio-system",
			expectedTargetLabels:   []string{"app"},
			expectedSelectorKey:    "istio",
			expectedSelectorValues: []string{"pilot"},
		},
		{
			name:     "named revision on openshift",
			platform: config.PlatformOpenShift,
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
			expectedName:           "default-istiod",
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
			result := reconciler.buildServiceMonitor(tt.rev, testMonitoringConfig())

			g.Expect(result.GetObjectKind().GroupVersionKind().Group).To(Equal(testMonitoringGV(tt.platform).Group))
			g.Expect(result.GetObjectKind().GroupVersionKind().Version).To(Equal("v1"))
			g.Expect(result.GetObjectKind().GroupVersionKind().Kind).To(Equal("ServiceMonitor"))
			g.Expect(result.GetName()).To(Equal(tt.expectedName))
			g.Expect(result.GetNamespace()).To(Equal(tt.expectedNS))

			// Check labels
			labels := result.GetLabels()
			g.Expect(labels["app"]).To(Equal("istiod"))
			expectMonitoringLabels(g, tt.platform, labels, "test-release")

			// Check spec.targetLabels and selector
			g.Expect(result.Spec.TargetLabels).To(Equal(tt.expectedTargetLabels))
			g.Expect(result.Spec.Selector.MatchExpressions).To(HaveLen(1))
			g.Expect(result.Spec.Selector.MatchExpressions[0].Key).To(Equal(tt.expectedSelectorKey))
			g.Expect(result.Spec.Selector.MatchExpressions[0].Values).To(Equal(tt.expectedSelectorValues))

			// Check endpoints
			g.Expect(result.Spec.Endpoints).To(HaveLen(1))
			endpoint := result.Spec.Endpoints[0]
			g.Expect(endpoint.Port).To(Equal("http-monitoring"))
			g.Expect(endpoint.Path).To(Equal("/metrics"))
			g.Expect(string(*endpoint.Scheme)).To(Equal("http"))
			g.Expect(string(endpoint.Interval)).To(Equal("30s"))
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
					OwnerReferences: []metav1.OwnerReference{
						{Kind: v1.IstioKind, Name: "my-istio"},
					},
				},
				Spec: v1.IstioRevisionSpec{
					Version:   "v1.24.0",
					Namespace: "istio-system",
				},
			},
			namespace:            "bookinfo",
			expectedName:         "default-proxies",
			expectedRelabelCount: 7,
			expectedLastLabel:    "pod",
		},
		{
			name:     "openshift platform uses service mesh relabelings",
			platform: config.PlatformOpenShift,
			rev: &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "canary",
					UID:  "test-uid",
					OwnerReferences: []metav1.OwnerReference{
						{Kind: v1.IstioKind, Name: "prod-mesh"},
					},
				},
				Spec: v1.IstioRevisionSpec{
					Version:   "v1.25.0",
					Namespace: "istio-system",
				},
			},
			namespace:            "myapp",
			expectedName:         "canary-proxies",
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
			monitoring := testMonitoringConfig()
			if tt.platform == config.PlatformOpenShift {
				monitoring = &v1.MonitoringConfig{Enabled: true}
			}
			result := reconciler.buildPodMonitor(tt.rev, tt.namespace, monitoring)

			g.Expect(result.GetObjectKind().GroupVersionKind().Group).To(Equal(testMonitoringGV(tt.platform).Group))
			g.Expect(result.GetObjectKind().GroupVersionKind().Version).To(Equal("v1"))
			g.Expect(result.GetObjectKind().GroupVersionKind().Kind).To(Equal("PodMonitor"))
			g.Expect(result.GetName()).To(Equal(tt.expectedName))
			g.Expect(result.GetNamespace()).To(Equal(tt.namespace))

			// Check labels
			labels := result.GetLabels()
			g.Expect(labels["app"]).To(Equal("istio-proxy"))
			monitoredBy := "test-release"
			if tt.platform == config.PlatformOpenShift {
				monitoredBy = ""
			}
			expectMonitoringLabels(g, tt.platform, labels, monitoredBy)

			// PodMonitor should NOT have owner references (cross-namespace)
			ownerRefs := result.GetOwnerReferences()
			g.Expect(ownerRefs).To(BeEmpty())

			// Check spec.selector.matchExpressions
			g.Expect(result.Spec.Selector.MatchExpressions).To(HaveLen(1))
			expr := result.Spec.Selector.MatchExpressions[0]
			g.Expect(expr.Key).To(Equal("istio-prometheus-ignore"))
			g.Expect(expr.Operator).To(Equal(metav1.LabelSelectorOpDoesNotExist))

			// Check podMetricsEndpoints
			g.Expect(result.Spec.PodMetricsEndpoints).To(HaveLen(1))
			endpoint := result.Spec.PodMetricsEndpoints[0]
			g.Expect(endpoint.Port).To(BeNil())
			g.Expect(endpoint.Path).To(Equal("/stats/prometheus"))
			g.Expect(string(*endpoint.Scheme)).To(Equal("http"))
			g.Expect(string(endpoint.Interval)).To(Equal("30s"))
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
		Platform:                config.PlatformKubernetes,
	}
}
