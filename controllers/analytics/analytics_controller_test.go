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

package analytics

import (
	"context"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/analytics"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// MockEventHandler records calls delegated by MetricRecordingHandler
type MockEventHandler struct {
	CreateCalled  bool
	UpdateCalled  bool
	DeleteCalled  bool
	GenericCalled bool
}

func (m *MockEventHandler) Create(ctx context.Context, e event.CreateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	m.CreateCalled = true
}

func (m *MockEventHandler) Update(ctx context.Context, e event.UpdateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	m.UpdateCalled = true
}

func (m *MockEventHandler) Delete(ctx context.Context, e event.DeleteEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	m.DeleteCalled = true
}

func (m *MockEventHandler) Generic(ctx context.Context, e event.GenericEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	m.GenericCalled = true
}

var _ = Describe("Analytics Controller", func() {
	var (
		ctx        context.Context
		scheme     *runtime.Scheme
		cl         client.Client
		reconciler *Reconciler
		req        ctrl.Request
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()

		Expect(v1.AddToScheme(scheme)).To(Succeed())
		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		Expect(monitoringv1.AddToScheme(scheme)).To(Succeed())

		cl = fake.NewClientBuilder().WithScheme(scheme).Build()

		reconciler = NewReconciler(
			config.ReconcilerConfig{},
			cl,
			scheme,
		)

		req = ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      analytics.OperatorMonitorName,
				Namespace: namespace, // defined in the analytics_controller.go
			},
		}
	})

	Context("Reconcile", func() {
		It("should create ClusterRoleBinding if it does not exist", func() {
			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			// Verify ClusterRoleBinding was created
			binding := &rbacv1.ClusterRoleBinding{}
			err = cl.Get(ctx, types.NamespacedName{Name: "metrics-reader-rolebinding", Namespace: namespace}, binding)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should create ServiceMonitor and PrometheusRule if they do not exist", func() {
			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			// Verify ServiceMonitor was created
			sm := &monitoringv1.ServiceMonitor{}
			err = cl.Get(ctx, types.NamespacedName{Name: analytics.OperatorMonitorName, Namespace: namespace}, sm)
			Expect(err).NotTo(HaveOccurred())

			// Verify PrometheusRule was created
			pr := &monitoringv1.PrometheusRule{}
			err = cl.Get(ctx, types.NamespacedName{Name: analytics.RuleName, Namespace: namespace}, pr)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should update ServiceMonitor spec if it deviates from desired spec", func() {
			// Pre-create a ServiceMonitor with outdated spec
			sm := &monitoringv1.ServiceMonitor{
				ObjectMeta: metav1.ObjectMeta{
					Name:      analytics.OperatorMonitorName,
					Namespace: namespace,
				},
				Spec: monitoringv1.ServiceMonitorSpec{
					Endpoints: []monitoringv1.Endpoint{{}},
				},
			}
			Expect(cl.Create(ctx, sm)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			// Verify ServiceMonitor spec was mutated back to desired spec
			updatedSM := &monitoringv1.ServiceMonitor{}
			Expect(cl.Get(ctx, types.NamespacedName{Name: analytics.OperatorMonitorName, Namespace: namespace}, updatedSM)).To(Succeed())
			Expect(updatedSM.Spec).To(Equal(*analytics.NewOperatorServiceMonitorSpec()))
		})

		It("should update PrometheusRule spec if it deviates from desired spec", func() {
			// Pre-create ServiceMonitor to get past first check
			sm := analytics.NewOperatorServiceMonitor(namespace)
			Expect(cl.Create(ctx, sm)).To(Succeed())

			// Pre-create PrometheusRule with outdated spec
			pr := &monitoringv1.PrometheusRule{
				ObjectMeta: metav1.ObjectMeta{
					Name:      analytics.RuleName,
					Namespace: namespace,
				},
				Spec: monitoringv1.PrometheusRuleSpec{
					Groups: []monitoringv1.RuleGroup{{}},
				},
			}
			Expect(cl.Create(ctx, pr)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(ctrl.Result{}))

			// Verify PrometheusRule spec was mutated back to desired spec
			updatedPR := &monitoringv1.PrometheusRule{}
			Expect(cl.Get(ctx, types.NamespacedName{Name: analytics.RuleName, Namespace: namespace}, updatedPR)).To(Succeed())
			Expect(updatedPR.Spec).To(Equal(*analytics.NewPrometheusRuleSpec()))
		})
	})

	Context("MetricRecordingHandler", func() {
		var (
			mockNext *MockEventHandler
			handler  *MetricRecordingHandler
		)

		BeforeEach(func() {
			mockNext = &MockEventHandler{}
			handler = &MetricRecordingHandler{Next: mockNext}
		})

		It("should delegate Create events to Next handler", func() {
			e := event.CreateEvent{Object: &v1.Istio{Spec: v1.IstioSpec{Version: "v1.30.0"}}}
			handler.Create(ctx, e, nil)
			Expect(mockNext.CreateCalled).To(BeTrue())
		})

		It("should delegate Update events to Next handler", func() {
			e := event.UpdateEvent{ObjectNew: &v1.Istio{Spec: v1.IstioSpec{Version: "v1.30.0"}}}
			handler.Update(ctx, e, nil)
			Expect(mockNext.UpdateCalled).To(BeTrue())
		})

		It("should delegate Delete events to Next handler", func() {
			e := event.DeleteEvent{Object: &v1.Istio{Spec: v1.IstioSpec{Version: "v1.30.0"}}}
			handler.Delete(ctx, e, nil)
			Expect(mockNext.DeleteCalled).To(BeTrue())
		})

		It("should delegate Generic events to Next handler", func() {
			e := event.GenericEvent{Object: &v1.Istio{Spec: v1.IstioSpec{Version: "v1.30.0"}}}
			handler.Generic(ctx, e, nil)
			Expect(mockNext.GenericCalled).To(BeTrue())
		})
	})

	Context("Namespace Filters", func() {
		DescribeTable("sidecarNamespaceFilter",
			func(labels map[string]string, expected bool) {
				ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Labels: labels}}
				Expect(sidecarNamespaceFilter(ns)).To(Equal(expected))
			},
			Entry("nil labels", nil, false),
			Entry("enabled injection label", map[string]string{"istio-injection": "enabled"}, true),
			Entry("disabled injection label", map[string]string{"istio-injection": "disabled"}, false),
			Entry("istio revision label present", map[string]string{"istio.io/rev": "canary"}, true),
			Entry("no matching labels", map[string]string{"foo": "bar"}, false),
		)

		DescribeTable("ambientNamespaceFilter",
			func(labels map[string]string, expected bool) {
				ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Labels: labels}}
				Expect(ambientNamespaceFilter(ns)).To(Equal(expected))
			},
			Entry("nil labels", nil, false),
			Entry("dataplane mode ambient", map[string]string{"istio.io/dataplane-mode": "ambient"}, true),
			Entry("use-waypoint valid value", map[string]string{"istio.io/use-waypoint": "waypoint-name"}, true),
			Entry("use-waypoint none", map[string]string{"istio.io/use-waypoint": "none"}, false),
			Entry("ingress-use-waypoint true", map[string]string{"istio.io/ingress-use-waypoint": "true"}, true),
			Entry("ingress-use-waypoint false", map[string]string{"istio.io/ingress-use-waypoint": "false"}, false),
			Entry("no matching labels", map[string]string{"foo": "bar"}, false),
		)

		It("namespaceLabelPredicate should correctly filter namespaces", func() {
			pred := namespaceLabelPredicate()

			validNs := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"istio.io/dataplane-mode": "ambient"},
				},
			}
			invalidNs := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"foo": "bar"},
				},
			}

			Expect(pred.Create(event.CreateEvent{Object: validNs})).To(BeTrue())
			Expect(pred.Create(event.CreateEvent{Object: invalidNs})).To(BeFalse())
		})
	})
})
