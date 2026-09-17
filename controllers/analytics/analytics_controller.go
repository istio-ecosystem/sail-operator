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
	"fmt"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/analytics"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const namespace = "sail-operator"

// MetricRecordingHandler wraps a standard handler to record metrics on incoming events
type MetricRecordingHandler struct {
	Next handler.EventHandler
}

// Reconciler reconciles operator analytics metrics.
type Reconciler struct {
	client.Client
	Config config.ReconcilerConfig
	Scheme *runtime.Scheme
}

func NewReconciler(cfg config.ReconcilerConfig, client client.Client, scheme *runtime.Scheme) *Reconciler {
	return &Reconciler{
		Config: cfg,
		Client: client,
		Scheme: scheme,
	}
}

func (m *MetricRecordingHandler) Create(ctx context.Context, e event.CreateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	m.recordMetric("create", e.Object)
	m.Next.Create(ctx, e, q)
}

func (m *MetricRecordingHandler) Update(ctx context.Context, e event.UpdateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	m.recordMetric("update", e.ObjectNew)
	m.Next.Update(ctx, e, q)
}

func (m *MetricRecordingHandler) Delete(ctx context.Context, e event.DeleteEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	m.recordMetric("delete", e.Object)
	m.Next.Delete(ctx, e, q)
}

func (m *MetricRecordingHandler) Generic(ctx context.Context, e event.GenericEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	m.recordMetric("generic", e.Object)
	m.Next.Generic(ctx, e, q)
}

func (m *MetricRecordingHandler) recordMetric(eventType string, obj client.Object) {
	if obj == nil {
		return
	}

	switch eventType {
	case "create":
		switch resource := obj.(type) {
		case *v1.Istio:
			analytics.IstioVersionTotal.WithLabelValues(resource.Spec.Version).Inc()
		case *v1.IstioRevision:
			analytics.IstioVersionTotal.WithLabelValues(resource.Spec.Version).Inc()
		case *v1.ZTunnel:
			analytics.ZTunnelVersionTotal.WithLabelValues(resource.Spec.Version).Inc()
		case *corev1.Namespace:
			if sidecarNamespaceFilter(obj) {
				analytics.SidecarNamespaceTotal.Inc()
			}
			if ambientNamespaceFilter(obj) {
				analytics.AmbientNamespaceTotal.Inc()
			}
		default:
			return
		}

	case "update":
		switch resource := obj.(type) {
		case *v1.Istio:
			analytics.IstioVersionTotal.WithLabelValues(resource.Spec.Version).Inc()
		case *v1.IstioRevision:
			analytics.IstioVersionTotal.WithLabelValues(resource.Spec.Version).Inc()
		case *v1.ZTunnel:
			analytics.ZTunnelVersionTotal.WithLabelValues(resource.Spec.Version).Inc()
		case *corev1.Namespace:
			if sidecarNamespaceFilter(obj) {
				analytics.SidecarNamespaceTotal.Inc()
			}
			if ambientNamespaceFilter(obj) {
				analytics.AmbientNamespaceTotal.Inc()
			}
		default:
			return
		}

	case "delete":
		switch resource := obj.(type) {
		case *v1.Istio:
			analytics.IstioVersionTotal.WithLabelValues(resource.Spec.Version).Dec()
		case *v1.IstioRevision:
			analytics.IstioVersionTotal.WithLabelValues(resource.Spec.Version).Dec()
		case *v1.ZTunnel:
			analytics.ZTunnelVersionTotal.WithLabelValues(resource.Spec.Version).Dec()
		case *corev1.Namespace:
			if sidecarNamespaceFilter(obj) {
				analytics.SidecarNamespaceTotal.Dec()
			}
			if ambientNamespaceFilter(obj) {
				analytics.AmbientNamespaceTotal.Dec()
			}
		default:
			return
		}

	default:
		return
	}
}

// +kubebuilder:rbac:groups=sailoperator.io,resources=istios,verbs=get;list;watch
// +kubebuilder:rbac:groups=sailoperator.io,resources=istiorevisions,verbs=get;list;watch
// +kubebuilder:rbac:groups=sailoperator.io,resources=istiorevisiontags,verbs=get;list;watch
// +kubebuilder:rbac:groups=sailoperator.io,resources=ztunnels,verbs=get;list;watch
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;create;update;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules,verbs=get;list;create;update;delete

// Reconcile implements the reconcile loop for ServiceMonitor and PrometheusRule resources
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Check if metrics reader ClusterRole exists
	binding, err := r.discoverMetricsReaderRbac(ctx)
	if err != nil {
		log.Error(err, "Failed to discover metrics reader ClusterRole")
		return ctrl.Result{}, nil
	}
	if err := r.Client.Create(ctx, binding); err != nil {
		log.Error(err, "Failed to create metrics reader ClusterRoleBinding")
		return ctrl.Result{}, nil
	}

	// Check if a ServiceMonitor already exists, if not create a new one
	foundMonitor := &monitoringv1.ServiceMonitor{}
	if err := r.Get(ctx, types.NamespacedName{Name: analytics.OperatorMonitorName, Namespace: namespace}, foundMonitor); err != nil {
		if apierrors.IsNotFound(err) {
			serviceMonitor := analytics.NewOperatorServiceMonitor(namespace)
			if err := r.Create(ctx, serviceMonitor); err != nil {
				log.Error(err, "Failed to create operator ServiceMonitor")
				return ctrl.Result{}, nil
			}
		}
		log.Error(err, "Failed to get ServiceMonitor")
		return ctrl.Result{}, nil
	}

	// Check if ServiceMonitor spec was changed, if so set as desired
	desiredMonitorSpec := analytics.NewOperatorServiceMonitorSpec()
	if !reflect.DeepEqual(foundMonitor.Spec.DeepCopy(), desiredMonitorSpec) {
		desiredMonitorSpec.DeepCopyInto(&foundMonitor.Spec)
		if err := r.Update(ctx, foundMonitor); err != nil {
			log.Error(err, "Failed to update operator ServiceMonitor")
			return ctrl.Result{}, nil
		}
	}

	// Check if a PrometheusRule already exists, if not create a new one
	foundRule := &monitoringv1.PrometheusRule{}
	if err := r.Get(ctx, types.NamespacedName{Name: analytics.RuleName, Namespace: namespace}, foundRule); err != nil {
		if apierrors.IsNotFound(err) {
			prometheusRule := analytics.NewPrometheusRule(namespace)
			if err := r.Create(ctx, prometheusRule); err != nil {
				log.Error(err, "Failed to create PrometheusRule")
				return ctrl.Result{}, nil
			}
		}
		log.Error(err, "Failed to get PrometheusRule")
		return ctrl.Result{}, nil
	}

	// Check if PrometheusRule spec was changed, if so set as desired
	desiredRuleSpec := analytics.NewPrometheusRuleSpec()
	if !reflect.DeepEqual(foundRule.Spec.DeepCopy(), desiredRuleSpec) {
		desiredRuleSpec.DeepCopyInto(&foundRule.Spec)
		if err := r.Update(ctx, foundRule); err != nil {
			log.Error(err, "Failed to update PrometheusRule")
			return ctrl.Result{}, nil
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	logger := mgr.GetLogger().WithName("ctrlr").WithName("analytics")

	// baseHandler creates the base enqueue handler
	baseHandler := &handler.EnqueueRequestForObject{}

	// metricHandler wraps the baseHandler
	metricHandler := &MetricRecordingHandler{
		Next: baseHandler,
	}

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{
			LogConstructor: func(req *reconcile.Request) logr.Logger {
				log := logger
				if req != nil {
					log = log.WithValues("analytics", req.Name)
				}
				return log
			},
			MaxConcurrentReconciles: r.Config.MaxConcurrentReconciles,
		}).
		Watches(&v1.Istio{}, metricHandler).
		Watches(&v1.IstioRevision{}, metricHandler).
		Watches(&v1.ZTunnel{}, metricHandler).
		Watches(&corev1.Namespace{}, metricHandler,
			builder.WithPredicates(namespaceLabelPredicate())).
		Owns(&monitoringv1.ServiceMonitor{}).
		Owns(&monitoringv1.PrometheusRule{}).
		Complete(r)
}

// discoverMetricsReaderRbac lists ClusterRoles with the kube-rbac-proxy component label
// (same as chart/bundle), then picks the one whose name ends with "-metrics-reader".
func (r *Reconciler) discoverMetricsReaderRbac(ctx context.Context) (*rbacv1.ClusterRoleBinding, error) {
	// discover metrics reader ClusterRole
	clusterRoleList := &rbacv1.ClusterRoleBindingList{}
	err := r.Client.List(ctx, clusterRoleList, client.MatchingLabels{"app.kubernetes.io/component": "kube-rbac-proxy"})
	if err != nil {
		return nil, fmt.Errorf("failed to list ClusterRoles: %w", err)
	}

	var matches []string
	for _, role := range clusterRoleList.Items {
		if strings.HasSuffix(role.Name, "-metrics-reader") {
			matches = append(matches, role.Name)
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no *-metrics-reader ClusterRole under label app.kubernetes.io/component=kube-rbac-proxy")
	}

	// return a ClusterRoleBinding for the service account to allow access to metrics
	return analytics.NewMetricsReaderClusterRoleBinding(matches[0]), nil
}

// sidecarNamespaceFilter filters objects where the namespace has Istio sidecar injection label and value
func sidecarNamespaceFilter(obj client.Object) bool {
	if obj == nil {
		return false
	}
	labels := obj.GetLabels()
	if labels == nil {
		return false
	}
	if labels["istio-injection"] == "" && labels["istio.io/rev"] == "" {
		return false
	}
	// istio-injection label takes precedence over istio.io/rev
	if labels["istio-injection"] == "disabled" {
		return false
	}
	if labels["istio-injection"] == "enabled" {
		return true
	}
	return true
}

// ambientNamespaceFilter filters objects where the namespace has Istio Ambient mode label and value:
// istio.io/dataplane-mode, istio.io/use-waypoint or istio.io/ingress-use-waypoint
func ambientNamespaceFilter(obj client.Object) bool {
	if obj == nil {
		return false
	}
	labels := obj.GetLabels()
	if labels == nil {
		return false
	}
	if labels["istio.io/dataplane-mode"] == "ambient" {
		return true
	}
	if labels["istio.io/use-waypoint"] != "" && labels["istio.io/use-waypoint"] != "none" {
		return true
	}
	if labels["istio.io/ingress-use-waypoint"] == "true" {
		return true
	}
	return false
}

// namespaceLabelPredicate filters objects where the namespace has Istio sidecar injection or Ambient mode label and value
func namespaceLabelPredicate() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return sidecarNamespaceFilter(obj) || ambientNamespaceFilter(obj)
	})
}
