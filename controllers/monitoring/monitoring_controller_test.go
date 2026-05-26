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
)

func newFakeClientBuilder() *fake.ClientBuilder {
	return fake.NewClientBuilder().
		WithScheme(scheme.Scheme)
}

var (
	ctx             = context.Background()
	revisionName    = "my-revision"
	revisionUID     = types.UID("my-revision-uid")
	istioNamespace  = "my-istio-namespace"
	appNamespace    = "my-app-namespace"
	revisionKey     = types.NamespacedName{Name: revisionName}
	revisionMeta    = metav1.ObjectMeta{
		Name: revisionName,
		UID:  revisionUID,
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
			existingObjects:   []client.Object{},
			expectErr:         false,
			expectSMCreated:   true,
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
			r := NewReconciler(cfg, cl, scheme.Scheme)

			_, err := r.Reconcile(ctx, tt.rev)

			if tt.expectErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}

			if tt.expectSMCreated {
				sm := &monitoringv1.ServiceMonitor{}
				sm.SetGroupVersionKind(testRhobsGV.WithKind("ServiceMonitor"))
				err = cl.Get(ctx, types.NamespacedName{
					Name:      tt.rev.Name + serviceMonitorNameSuffix,
					Namespace: tt.rev.Spec.Namespace,
				}, sm)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(sm.GetLabels()["app"]).To(Equal("istiod"))
				g.Expect(sm.GetLabels()["monitored-by"]).To(Equal("coo-prometheus"))
			}

			if tt.expectPMNamespace != "" {
				pm := &monitoringv1.PodMonitor{}
				pm.SetGroupVersionKind(testRhobsGV.WithKind("PodMonitor"))
				err = cl.Get(ctx, types.NamespacedName{
					Name:      tt.rev.Name + podMonitorNameSuffix,
					Namespace: tt.expectPMNamespace,
				}, pm)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(pm.GetLabels()["app"]).To(Equal("istio-proxy"))
				g.Expect(pm.GetLabels()["monitored-by"]).To(Equal("coo-prometheus"))
			}
		})
	}
}

func TestReconcileServiceMonitor(t *testing.T) {
	cfg := newReconcilerTestConfig()

	tests := []struct {
		name         string
		rev          *v1.IstioRevision
		existing     *monitoringv1.ServiceMonitor
		interceptors interceptor.Funcs
		expectErr    bool
	}{
		{
			name: "creates new ServiceMonitor",
			rev: &v1.IstioRevision{
				ObjectMeta: revisionMeta,
				Spec: v1.IstioRevisionSpec{
					Namespace: istioNamespace,
				},
			},
			existing:  nil,
			expectErr: false,
		},
		{
			name: "updates existing ServiceMonitor",
			rev: &v1.IstioRevision{
				ObjectMeta: revisionMeta,
				Spec: v1.IstioRevisionSpec{
					Namespace: istioNamespace,
				},
			},
			existing:  newServiceMonitor(revisionName+serviceMonitorNameSuffix, istioNamespace),
			expectErr: false,
		},
		{
			name: "returns error on client Get failure",
			rev: &v1.IstioRevision{
				ObjectMeta: revisionMeta,
				Spec: v1.IstioRevisionSpec{
					Namespace: istioNamespace,
				},
			},
			interceptors: interceptor.Funcs{
				Get: func(_ context.Context, _ client.WithWatch, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					if _, ok := obj.(*monitoringv1.ServiceMonitor); ok {
						return fmt.Errorf("simulated error")
					}
					return nil
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			objects := []client.Object{tt.rev}
			if tt.existing != nil {
				objects = append(objects, tt.existing)
			}

			cl := newFakeClientBuilder().
				WithObjects(objects...).
				WithInterceptorFuncs(tt.interceptors).
				Build()
			r := NewReconciler(cfg, cl, scheme.Scheme)

			err := r.reconcileServiceMonitor(ctx, tt.rev)

			if tt.expectErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}

func TestReconcilePodMonitors(t *testing.T) {
	cfg := newReconcilerTestConfig()

	tests := []struct {
		name                    string
		rev                     *v1.IstioRevision
		namespaces              []*corev1.Namespace
		existingPodMonitor      *monitoringv1.PodMonitor
		interceptors            interceptor.Funcs
		expectErr               bool
		expectPodMonitorCreated []string // list of namespaces where PodMonitor should be created
	}{
		{
			name: "creates PodMonitor in namespace with injection enabled",
			rev: &v1.IstioRevision{
				ObjectMeta: revisionMeta,
				Spec: v1.IstioRevisionSpec{
					Namespace: istioNamespace,
				},
			},
			namespaces: []*corev1.Namespace{
				newNamespaceWithInjection(appNamespace),
			},
			expectErr:               false,
			expectPodMonitorCreated: []string{appNamespace},
		},
		{
			name: "creates PodMonitor in multiple namespaces",
			rev: &v1.IstioRevision{
				ObjectMeta: revisionMeta,
				Spec: v1.IstioRevisionSpec{
					Namespace: istioNamespace,
				},
			},
			namespaces: []*corev1.Namespace{
				newNamespaceWithInjection(appNamespace),
				newNamespaceWithInjection("another-app-namespace"),
			},
			expectErr:               false,
			expectPodMonitorCreated: []string{appNamespace, "another-app-namespace"},
		},
		{
			name: "skips control plane namespace",
			rev: &v1.IstioRevision{
				ObjectMeta: revisionMeta,
				Spec: v1.IstioRevisionSpec{
					Namespace: istioNamespace,
				},
			},
			namespaces: []*corev1.Namespace{
				newNamespaceWithInjection(istioNamespace), // should be skipped
				newNamespaceWithInjection(appNamespace),
			},
			expectErr:               false,
			expectPodMonitorCreated: []string{appNamespace}, // only app namespace
		},
		{
			name: "updates existing PodMonitor",
			rev: &v1.IstioRevision{
				ObjectMeta: revisionMeta,
				Spec: v1.IstioRevisionSpec{
					Namespace: istioNamespace,
				},
			},
			namespaces: []*corev1.Namespace{
				newNamespaceWithInjection(appNamespace),
			},
			existingPodMonitor:      newPodMonitor(revisionName+podMonitorNameSuffix, appNamespace),
			expectErr:               false,
			expectPodMonitorCreated: []string{appNamespace},
		},
		{
			name: "no PodMonitor when no namespaces with injection",
			rev: &v1.IstioRevision{
				ObjectMeta: revisionMeta,
				Spec: v1.IstioRevisionSpec{
					Namespace: istioNamespace,
				},
			},
			namespaces:              []*corev1.Namespace{},
			expectErr:               false,
			expectPodMonitorCreated: []string{},
		},
		{
			name: "returns error on client List failure",
			rev: &v1.IstioRevision{
				ObjectMeta: revisionMeta,
				Spec: v1.IstioRevisionSpec{
					Namespace: istioNamespace,
				},
			},
			interceptors: interceptor.Funcs{
				List: func(_ context.Context, _ client.WithWatch, list client.ObjectList, _ ...client.ListOption) error {
					if _, ok := list.(*corev1.NamespaceList); ok {
						return fmt.Errorf("simulated error")
					}
					return nil
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			objects := []client.Object{tt.rev}
			for _, ns := range tt.namespaces {
				objects = append(objects, ns)
			}
			if tt.existingPodMonitor != nil {
				objects = append(objects, tt.existingPodMonitor)
			}

			cl := newFakeClientBuilder().
				WithObjects(objects...).
				WithInterceptorFuncs(tt.interceptors).
				Build()
			r := NewReconciler(cfg, cl, scheme.Scheme)

			err := r.reconcilePodMonitors(ctx, tt.rev)

			if tt.expectErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())

				// Verify PodMonitors were created in expected namespaces
				for _, ns := range tt.expectPodMonitorCreated {
					pm := &monitoringv1.PodMonitor{}
					pm.SetGroupVersionKind(testRhobsGV.WithKind("PodMonitor"))
					err = cl.Get(ctx, types.NamespacedName{
						Name:      revisionName + podMonitorNameSuffix,
						Namespace: ns,
					}, pm)
					g.Expect(err).ToNot(HaveOccurred(), "PodMonitor should exist in namespace %s", ns)
				}
			}
		})
	}
}

func TestBuildServiceMonitor(t *testing.T) {
	cfg := newReconcilerTestConfig()

	tests := []struct {
		name              string
		rev               *v1.IstioRevision
		expectedName      string
		expectedNamespace string
	}{
		{
			name: "default revision",
			rev: &v1.IstioRevision{
				ObjectMeta: revisionMeta,
				Spec: v1.IstioRevisionSpec{
					Version:   "v1.24.0",
					Namespace: istioNamespace,
				},
			},
			expectedName:      revisionName + serviceMonitorNameSuffix,
			expectedNamespace: istioNamespace,
		},
		{
			name: "named revision",
			rev: &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "canary",
					UID:  "test-uid-canary",
				},
				Spec: v1.IstioRevisionSpec{
					Version:   "v1.25.0",
					Namespace: istioNamespace,
				},
			},
			expectedName:      "canary" + serviceMonitorNameSuffix,
			expectedNamespace: istioNamespace,
		},
		{
			name: "custom namespace",
			rev: &v1.IstioRevision{
				ObjectMeta: revisionMeta,
				Spec: v1.IstioRevisionSpec{
					Version:   "v1.24.0",
					Namespace: "custom-namespace",
				},
			},
			expectedName:      revisionName + serviceMonitorNameSuffix,
			expectedNamespace: "custom-namespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			cl := newFakeClientBuilder().Build()
			r := NewReconciler(cfg, cl, scheme.Scheme)

			result := r.buildServiceMonitor(tt.rev)

			g.Expect(result.GetName()).To(Equal(tt.expectedName))
			g.Expect(result.GetNamespace()).To(Equal(tt.expectedNamespace))
			g.Expect(result.GetLabels()["app"]).To(Equal("istiod"))
			g.Expect(result.GetLabels()["monitored-by"]).To(Equal("coo-prometheus"))
			gvk := result.GetObjectKind().GroupVersionKind()
			g.Expect(gvk.Group).To(Equal("monitoring.rhobs"))
			g.Expect(gvk.Version).To(Equal("v1"))
			g.Expect(gvk.Kind).To(Equal("ServiceMonitor"))
			g.Expect(result.GetOwnerReferences()).To(HaveLen(1))
			g.Expect(result.GetOwnerReferences()[0].Name).To(Equal(tt.rev.Name))
			g.Expect(result.GetOwnerReferences()[0].Kind).To(Equal(v1.IstioRevisionKind))

			// Verify spec using typed fields
			g.Expect(result.Spec.Selector.MatchLabels["app"]).To(Equal("istiod"))
			g.Expect(result.Spec.Endpoints).To(HaveLen(1))
			g.Expect(result.Spec.Endpoints[0].Path).To(Equal("/metrics"))
			g.Expect(result.Spec.Endpoints[0].Scheme).ToNot(BeNil())
			g.Expect(string(*result.Spec.Endpoints[0].Scheme)).To(Equal("HTTP"))
		})
	}
}

func TestBuildPodMonitor(t *testing.T) {
	cfg := newReconcilerTestConfig()

	tests := []struct {
		name              string
		rev               *v1.IstioRevision
		targetNamespace   string
		expectedName      string
		expectedNamespace string
	}{
		{
			name: "builds PodMonitor for application namespace",
			rev: &v1.IstioRevision{
				ObjectMeta: revisionMeta,
				Spec: v1.IstioRevisionSpec{
					Version:   "v1.24.0",
					Namespace: istioNamespace,
				},
			},
			targetNamespace:   appNamespace,
			expectedName:      revisionName + podMonitorNameSuffix,
			expectedNamespace: appNamespace,
		},
		{
			name: "named revision",
			rev: &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "stable",
					UID:  "test-uid-stable",
				},
				Spec: v1.IstioRevisionSpec{
					Version:   "v1.23.0",
					Namespace: istioNamespace,
				},
			},
			targetNamespace:   "another-app-ns",
			expectedName:      "stable" + podMonitorNameSuffix,
			expectedNamespace: "another-app-ns",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			cl := newFakeClientBuilder().Build()
			r := NewReconciler(cfg, cl, scheme.Scheme)

			result := r.buildPodMonitor(tt.rev, tt.targetNamespace)

			g.Expect(result.GetName()).To(Equal(tt.expectedName))
			g.Expect(result.GetNamespace()).To(Equal(tt.expectedNamespace))
			g.Expect(result.GetLabels()["app"]).To(Equal("istio-proxy"))
			g.Expect(result.GetLabels()["monitored-by"]).To(Equal("coo-prometheus"))
			gvk := result.GetObjectKind().GroupVersionKind()
			g.Expect(gvk.Group).To(Equal("monitoring.rhobs"))
			g.Expect(gvk.Version).To(Equal("v1"))
			g.Expect(gvk.Kind).To(Equal("PodMonitor"))
			// PodMonitors in application namespaces don't have owner references
			// (cross-namespace owner references are not supported)
			g.Expect(result.GetOwnerReferences()).To(BeEmpty())

			// Verify selector uses matchExpressions for security.istio.io/tlsMode
			g.Expect(result.Spec.Selector.MatchExpressions).To(HaveLen(1))
			g.Expect(result.Spec.Selector.MatchExpressions[0].Key).To(Equal("security.istio.io/tlsMode"))
			g.Expect(result.Spec.Selector.MatchExpressions[0].Operator).To(Equal(metav1.LabelSelectorOpExists))

			// Verify podMetricsEndpoints
			g.Expect(result.Spec.PodMetricsEndpoints).To(HaveLen(1))
			endpoint := result.Spec.PodMetricsEndpoints[0]
			g.Expect(endpoint.Port).ToNot(BeNil())
			g.Expect(*endpoint.Port).To(Equal("http-envoy-prom"))
			g.Expect(endpoint.Path).To(Equal("/stats/prometheus"))
			g.Expect(endpoint.Scheme).ToNot(BeNil())
			g.Expect(string(*endpoint.Scheme)).To(Equal("HTTP"))
			g.Expect(string(endpoint.Interval)).To(Equal("30s"))
			g.Expect(string(endpoint.ScrapeTimeout)).To(Equal("10s"))
			g.Expect(endpoint.HonorLabels).To(BeTrue())
			g.Expect(endpoint.FilterRunning).ToNot(BeNil())
			g.Expect(*endpoint.FilterRunning).To(BeTrue())
		})
	}
}

func newReconcilerTestConfig() config.ReconcilerConfig {
	return config.ReconcilerConfig{
		Platform:                config.PlatformKubernetes,
		DefaultProfile:          "",
		MaxConcurrentReconciles: 1,
	}
}

func newServiceMonitor(name, namespace string) *monitoringv1.ServiceMonitor {
	sm := &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	sm.SetGroupVersionKind(testRhobsGV.WithKind("ServiceMonitor"))
	return sm
}

func newPodMonitor(name, namespace string) *monitoringv1.PodMonitor {
	pm := &monitoringv1.PodMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	pm.SetGroupVersionKind(testRhobsGV.WithKind("PodMonitor"))
	return pm
}
