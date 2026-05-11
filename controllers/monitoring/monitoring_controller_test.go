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
	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

var ctx = context.Background()

func TestReconcile(t *testing.T) {
	cfg := newReconcilerTestConfig()

	tests := []struct {
		name            string
		rev             *v1.IstioRevision
		existingObjects []client.Object
		expectErr       bool
		expectSMCreated bool
		expectPMCreated bool
	}{
		{
			name: "creates ServiceMonitor and PodMonitor for new IstioRevision",
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
			existingObjects: []client.Object{},
			expectErr:       false,
			expectSMCreated: true,
			expectPMCreated: true,
		},
		{
			name: "skips reconciliation for deleting IstioRevision",
			rev: &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "default",
					UID:               "test-uid",
					DeletionTimestamp: &metav1.Time{Time: metav1.Now().Time},
					Finalizers:        []string{"test-finalizer"},
				},
				Spec: v1.IstioRevisionSpec{
					Version:   "v1.24.0",
					Namespace: "istio-system",
				},
			},
			existingObjects: []client.Object{},
			expectErr:       false,
			expectSMCreated: false,
			expectPMCreated: false,
		},
		{
			name: "updates existing ServiceMonitor",
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
			existingObjects: []client.Object{
				newUnstructuredServiceMonitor("default"+serviceMonitorNameSuffix, "istio-system"),
			},
			expectErr:       false,
			expectSMCreated: false,
			expectPMCreated: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			objects := tt.existingObjects
			if tt.rev != nil {
				objects = append(objects, tt.rev)
			}

			cl := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
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
				sm := &unstructured.Unstructured{}
				sm.SetGroupVersionKind(serviceMonitorGVK)
				err = cl.Get(ctx, types.NamespacedName{
					Name:      tt.rev.Name + serviceMonitorNameSuffix,
					Namespace: tt.rev.Spec.Namespace,
				}, sm)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(sm.GetLabels()["app"]).To(Equal("istiod"))
				g.Expect(sm.GetLabels()["monitored-by"]).To(Equal("coo-prometheus"))
			}

			if tt.expectPMCreated {
				pm := &unstructured.Unstructured{}
				pm.SetGroupVersionKind(podMonitorGVK)
				err = cl.Get(ctx, types.NamespacedName{
					Name:      tt.rev.Name + podMonitorNameSuffix,
					Namespace: tt.rev.Spec.Namespace,
				}, pm)
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(pm.GetLabels()["app"]).To(Equal("istiod"))
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
		existing     *unstructured.Unstructured
		interceptors interceptor.Funcs
		expectErr    bool
	}{
		{
			name: "creates new ServiceMonitor",
			rev: &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
					UID:  "test-uid",
				},
				Spec: v1.IstioRevisionSpec{
					Namespace: "istio-system",
				},
			},
			existing:  nil,
			expectErr: false,
		},
		{
			name: "updates existing ServiceMonitor",
			rev: &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
					UID:  "test-uid",
				},
				Spec: v1.IstioRevisionSpec{
					Namespace: "istio-system",
				},
			},
			existing:  newUnstructuredServiceMonitor("default"+serviceMonitorNameSuffix, "istio-system"),
			expectErr: false,
		},
		{
			name: "returns error on client Get failure",
			rev: &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
					UID:  "test-uid",
				},
				Spec: v1.IstioRevisionSpec{
					Namespace: "istio-system",
				},
			},
			interceptors: interceptor.Funcs{
				Get: func(_ context.Context, _ client.WithWatch, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					if u, ok := obj.(*unstructured.Unstructured); ok && u.GetKind() == "ServiceMonitor" {
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

			cl := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
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

func TestReconcilePodMonitor(t *testing.T) {
	cfg := newReconcilerTestConfig()

	tests := []struct {
		name         string
		rev          *v1.IstioRevision
		existing     *unstructured.Unstructured
		interceptors interceptor.Funcs
		expectErr    bool
	}{
		{
			name: "creates new PodMonitor",
			rev: &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
					UID:  "test-uid",
				},
				Spec: v1.IstioRevisionSpec{
					Namespace: "istio-system",
				},
			},
			existing:  nil,
			expectErr: false,
		},
		{
			name: "updates existing PodMonitor",
			rev: &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
					UID:  "test-uid",
				},
				Spec: v1.IstioRevisionSpec{
					Namespace: "istio-system",
				},
			},
			existing:  newUnstructuredPodMonitor("default"+podMonitorNameSuffix, "istio-system"),
			expectErr: false,
		},
		{
			name: "returns error on client Get failure",
			rev: &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
					UID:  "test-uid",
				},
				Spec: v1.IstioRevisionSpec{
					Namespace: "istio-system",
				},
			},
			interceptors: interceptor.Funcs{
				Get: func(_ context.Context, _ client.WithWatch, key client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					if u, ok := obj.(*unstructured.Unstructured); ok && u.GetKind() == "PodMonitor" {
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

			cl := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithObjects(objects...).
				WithInterceptorFuncs(tt.interceptors).
				Build()
			r := NewReconciler(cfg, cl, scheme.Scheme)

			err := r.reconcilePodMonitor(ctx, tt.rev)

			if tt.expectErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).ToNot(HaveOccurred())
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
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
					UID:  "test-uid",
				},
				Spec: v1.IstioRevisionSpec{
					Version:   "v1.24.0",
					Namespace: "istio-system",
				},
			},
			expectedName:      "default" + serviceMonitorNameSuffix,
			expectedNamespace: "istio-system",
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
					Namespace: "istio-system",
				},
			},
			expectedName:      "canary" + serviceMonitorNameSuffix,
			expectedNamespace: "istio-system",
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
					Namespace: "custom-namespace",
				},
			},
			expectedName:      "default" + serviceMonitorNameSuffix,
			expectedNamespace: "custom-namespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
			r := NewReconciler(cfg, cl, scheme.Scheme)

			result := r.buildServiceMonitor(tt.rev)

			g.Expect(result.GetName()).To(Equal(tt.expectedName))
			g.Expect(result.GetNamespace()).To(Equal(tt.expectedNamespace))
			g.Expect(result.GetLabels()["app"]).To(Equal("istiod"))
			g.Expect(result.GetLabels()["monitored-by"]).To(Equal("coo-prometheus"))
			g.Expect(result.GetAPIVersion()).To(Equal("monitoring.rhobs/v1"))
			g.Expect(result.GetKind()).To(Equal("ServiceMonitor"))
			g.Expect(result.GetOwnerReferences()).To(HaveLen(1))
			g.Expect(result.GetOwnerReferences()[0].Name).To(Equal(tt.rev.Name))
			g.Expect(result.GetOwnerReferences()[0].Kind).To(Equal(v1.IstioRevisionKind))

			// Verify spec
			spec, found, err := unstructured.NestedMap(result.Object, "spec")
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(found).To(BeTrue())

			selector, _, _ := unstructured.NestedMap(spec, "selector")
			matchLabels, _, _ := unstructured.NestedStringMap(selector, "matchLabels")
			g.Expect(matchLabels["app"]).To(Equal("istiod"))

			endpoints, _, _ := unstructured.NestedSlice(spec, "endpoints")
			g.Expect(endpoints).To(HaveLen(1))
			endpoint := endpoints[0].(map[string]interface{})
			g.Expect(endpoint["path"]).To(Equal("/metrics"))
			g.Expect(endpoint["scheme"]).To(Equal("http"))
		})
	}
}

func TestBuildPodMonitor(t *testing.T) {
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
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
					UID:  "test-uid",
				},
				Spec: v1.IstioRevisionSpec{
					Version:   "v1.24.0",
					Namespace: "istio-system",
				},
			},
			expectedName:      "default" + podMonitorNameSuffix,
			expectedNamespace: "istio-system",
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
					Namespace: "istio-system",
				},
			},
			expectedName:      "stable" + podMonitorNameSuffix,
			expectedNamespace: "istio-system",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
			r := NewReconciler(cfg, cl, scheme.Scheme)

			result := r.buildPodMonitor(tt.rev)

			g.Expect(result.GetName()).To(Equal(tt.expectedName))
			g.Expect(result.GetNamespace()).To(Equal(tt.expectedNamespace))
			g.Expect(result.GetLabels()["app"]).To(Equal("istiod"))
			g.Expect(result.GetLabels()["monitored-by"]).To(Equal("coo-prometheus"))
			g.Expect(result.GetAPIVersion()).To(Equal("monitoring.rhobs/v1"))
			g.Expect(result.GetKind()).To(Equal("PodMonitor"))
			g.Expect(result.GetOwnerReferences()).To(HaveLen(1))
			g.Expect(result.GetOwnerReferences()[0].Name).To(Equal(tt.rev.Name))
			g.Expect(result.GetOwnerReferences()[0].Kind).To(Equal(v1.IstioRevisionKind))

			// Verify spec
			spec, found, err := unstructured.NestedMap(result.Object, "spec")
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(found).To(BeTrue())

			selector, _, _ := unstructured.NestedMap(spec, "selector")
			matchLabels, _, _ := unstructured.NestedStringMap(selector, "matchLabels")
			g.Expect(matchLabels["app"]).To(Equal("istiod"))

			podMetricsEndpoints, _, _ := unstructured.NestedSlice(spec, "podMetricsEndpoints")
			g.Expect(podMetricsEndpoints).To(HaveLen(1))
			endpoint := podMetricsEndpoints[0].(map[string]interface{})
			g.Expect(endpoint["path"]).To(Equal("/metrics"))
			g.Expect(endpoint["port"]).To(Equal("http-monitoring"))
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

func newUnstructuredServiceMonitor(name, namespace string) *unstructured.Unstructured {
	sm := &unstructured.Unstructured{}
	sm.SetGroupVersionKind(serviceMonitorGVK)
	sm.SetName(name)
	sm.SetNamespace(namespace)
	return sm
}

func newUnstructuredPodMonitor(name, namespace string) *unstructured.Unstructured {
	pm := &unstructured.Unstructured{}
	pm.SetGroupVersionKind(podMonitorGVK)
	pm.SetName(name)
	pm.SetNamespace(namespace)
	return pm
}
