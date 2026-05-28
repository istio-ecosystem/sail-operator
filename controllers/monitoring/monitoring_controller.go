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

	"github.com/go-logr/logr"
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	"github.com/istio-ecosystem/sail-operator/pkg/enqueuelogger"
	"github.com/istio-ecosystem/sail-operator/pkg/reconciler"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

const (
	serviceMonitorNameSuffix = "-istiod"
	podMonitorNameSuffix     = "-proxies"

	// COO (Cluster Observability Operator) API group
	rhobsAPIGroup   = "monitoring.rhobs"
	rhobsAPIVersion = "v1"

	// Labels
	cooMonitoredByLabel = "monitored-by"
	cooMonitoredByValue = "coo-prometheus"
)

// rhobsGV is the GroupVersion for COO monitoring resources
var rhobsGV = schema.GroupVersion{Group: rhobsAPIGroup, Version: rhobsAPIVersion}

// Reconciler reconciles monitoring resources (ServiceMonitor, PodMonitor) for IstioRevision objects
type Reconciler struct {
	client.Client
	Config config.ReconcilerConfig
	Scheme *runtime.Scheme
}

// NewReconciler creates a new monitoring Reconciler
func NewReconciler(cfg config.ReconcilerConfig, client client.Client, scheme *runtime.Scheme) *Reconciler {
	return &Reconciler{
		Config: cfg,
		Client: client,
		Scheme: scheme,
	}
}

// +kubebuilder:rbac:groups=monitoring.rhobs,resources=servicemonitors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.rhobs,resources=podmonitors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

// Reconcile creates or updates ServiceMonitor and PodMonitor resources for each IstioRevision
func (r *Reconciler) Reconcile(ctx context.Context, rev *v1.IstioRevision) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Skip if the IstioRevision is being deleted
	if rev.DeletionTimestamp != nil {
		log.V(2).Info("IstioRevision is being deleted, skipping monitoring reconciliation")
		return ctrl.Result{}, nil
	}

	// Check if monitoring is enabled in the parent Istio CR
	enabled, err := r.isMonitoringEnabled(ctx, rev)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to check if monitoring is enabled: %w", err)
	}
	if !enabled {
		log.V(2).Info("Monitoring is not enabled in Istio CR, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// Reconcile ServiceMonitor for istiod (in the istio control plane namespace)
	if err := r.reconcileServiceMonitor(ctx, rev); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile ServiceMonitor: %w", err)
	}

	// Reconcile PodMonitors for istio-proxy sidecars in namespaces with injection enabled
	if err := r.reconcilePodMonitors(ctx, rev); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile PodMonitors: %w", err)
	}

	log.Info("Monitoring resources reconciled successfully")
	return ctrl.Result{}, nil
}

// isMonitoringEnabled checks if monitoring is enabled in the parent Istio CR
func (r *Reconciler) isMonitoringEnabled(ctx context.Context, rev *v1.IstioRevision) (bool, error) {
	// Find the parent Istio CR from owner references
	for _, ownerRef := range rev.GetOwnerReferences() {
		if ownerRef.Kind == v1.IstioKind {
			istio := &v1.Istio{}
			if err := r.Client.Get(ctx, client.ObjectKey{Name: ownerRef.Name}, istio); err != nil {
				if apierrors.IsNotFound(err) {
					// Istio CR not found, monitoring not enabled
					return false, nil
				}
				return false, fmt.Errorf("failed to get Istio CR: %w", err)
			}
			// Check if monitoring is enabled (defaults to false if not set)
			return istio.Spec.Monitoring != nil && istio.Spec.Monitoring.Enabled, nil
		}
	}
	// No Istio owner found, monitoring not enabled
	return false, nil
}

// reconcileServiceMonitor creates or updates the ServiceMonitor for istiod
func (r *Reconciler) reconcileServiceMonitor(ctx context.Context, rev *v1.IstioRevision) error {
	log := logf.FromContext(ctx)
	desired := r.buildServiceMonitor(rev)

	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(rhobsGV.WithKind("ServiceMonitor"))

	err := r.Client.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Creating ServiceMonitor", "name", desired.GetName(), "namespace", desired.GetNamespace())
			return r.Client.Create(ctx, desired)
		}
		return fmt.Errorf("failed to get ServiceMonitor: %w", err)
	}

	log.V(2).Info("Updating ServiceMonitor", "name", desired.GetName(), "namespace", desired.GetNamespace())
	desired.SetResourceVersion(existing.GetResourceVersion())
	return r.Client.Update(ctx, desired)
}

// reconcilePodMonitors creates or updates PodMonitors for istio-proxy sidecars
// in namespaces with istio-injection=enabled label (excluding istio control plane namespace)
func (r *Reconciler) reconcilePodMonitors(ctx context.Context, rev *v1.IstioRevision) error {
	log := logf.FromContext(ctx)

	// List namespaces with istio-injection=enabled label
	nsList := &corev1.NamespaceList{}
	if err := r.Client.List(ctx, nsList, client.MatchingLabels{
		constants.IstioInjectionLabel: constants.IstioInjectionEnabledValue,
	}); err != nil {
		return fmt.Errorf("failed to list namespaces: %w", err)
	}

	// Create/update PodMonitor in each namespace with injection enabled
	for _, ns := range nsList.Items {
		// Skip the istio control plane namespace - we don't want to monitor sidecars there
		if ns.Name == rev.Spec.Namespace {
			log.V(2).Info("Skipping PodMonitor for control plane namespace", "namespace", ns.Name)
			continue
		}

		if err := r.reconcilePodMonitorInNamespace(ctx, rev, ns.Name); err != nil {
			return fmt.Errorf("failed to reconcile PodMonitor in namespace %s: %w", ns.Name, err)
		}
	}

	return nil
}

// reconcilePodMonitorInNamespace creates or updates a PodMonitor in the specified namespace
func (r *Reconciler) reconcilePodMonitorInNamespace(ctx context.Context, rev *v1.IstioRevision, namespace string) error {
	log := logf.FromContext(ctx)
	desired := r.buildPodMonitor(rev, namespace)

	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(rhobsGV.WithKind("PodMonitor"))

	err := r.Client.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Creating PodMonitor", "name", desired.GetName(), "namespace", namespace)
			return r.Client.Create(ctx, desired)
		}
		return fmt.Errorf("failed to get PodMonitor: %w", err)
	}

	log.V(2).Info("Updating PodMonitor", "name", desired.GetName(), "namespace", namespace)
	desired.SetResourceVersion(existing.GetResourceVersion())
	return r.Client.Update(ctx, desired)
}

// buildServiceMonitor constructs the ServiceMonitor for monitoring istiod using unstructured
func (r *Reconciler) buildServiceMonitor(rev *v1.IstioRevision) *unstructured.Unstructured {
	name := rev.Name + serviceMonitorNameSuffix
	namespace := rev.Spec.Namespace

	sm := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": rhobsGV.String(),
			"kind":       "ServiceMonitor",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"app":               "istiod",
					cooMonitoredByLabel: cooMonitoredByValue,
				},
				"ownerReferences": []interface{}{
					map[string]interface{}{
						"apiVersion":         v1.GroupVersion.String(),
						"kind":               v1.IstioRevisionKind,
						"name":               rev.Name,
						"uid":                string(rev.UID),
						"controller":         true,
						"blockOwnerDeletion": true,
					},
				},
			},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app": "istiod",
					},
				},
				"endpoints": []interface{}{
					map[string]interface{}{
						"port":     "http-monitoring",
						"path":     "/metrics",
						"scheme":   "http",
						"interval": "30s",
					},
				},
			},
		},
	}

	return sm
}

// buildPodMonitor constructs the PodMonitor for monitoring istio-proxy sidecars using unstructured
func (r *Reconciler) buildPodMonitor(rev *v1.IstioRevision, namespace string) *unstructured.Unstructured {
	name := rev.Name + podMonitorNameSuffix

	pm := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": rhobsGV.String(),
			"kind":       "PodMonitor",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
				"labels": map[string]interface{}{
					"app":               "istio-proxy",
					cooMonitoredByLabel: cooMonitoredByValue,
				},
				// Note: We don't set owner references here because the PodMonitor is in a different
				// namespace than the IstioRevision (which is cluster-scoped). Cross-namespace owner
				// references are not supported by Kubernetes.
			},
			"spec": map[string]interface{}{
				"selector": map[string]interface{}{
					"matchExpressions": []interface{}{
						map[string]interface{}{
							"key":      "security.istio.io/tlsMode",
							"operator": "Exists",
						},
					},
				},
				"podMetricsEndpoints": []interface{}{
					map[string]interface{}{
						"port":          "http-envoy-prom",
						"path":          "/stats/prometheus",
						"scheme":        "http",
						"interval":      "30s",
						"scrapeTimeout": "10s",
						"honorLabels":   true,
						"filterRunning": true,
					},
				},
			},
		},
	}

	return pm
}

// SetupWithManager sets up the controller with the Manager
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	logger := mgr.GetLogger().WithName("ctrlr").WithName("monitoring")

	// namespaceHandler triggers reconciliation of all IstioRevisions when a namespace
	// with istio-injection=enabled label is created/updated/deleted
	namespaceHandler := wrapEventHandler(logger, handler.EnqueueRequestsFromMapFunc(r.mapNamespaceToReconcileRequest))

	// istioHandler triggers reconciliation of owned IstioRevisions when an Istio CR's
	// monitoring configuration changes
	istioHandler := wrapEventHandler(logger, handler.EnqueueRequestsFromMapFunc(r.mapIstioToReconcileRequest))

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{
			LogConstructor: func(req *reconcile.Request) logr.Logger {
				log := logger
				if req != nil {
					log = log.WithValues("IstioRevision", req.Name)
				}
				return log
			},
			MaxConcurrentReconciles: r.Config.MaxConcurrentReconciles,
		}).
		Named("monitoring").
		Watches(&v1.IstioRevision{}, wrapEventHandler(logger, &handler.EnqueueRequestForObject{})).
		// Note: We don't watch ServiceMonitor/PodMonitor directly because they use the rhobs API group
		// which requires COO CRDs. Owner references ensure cleanup on IstioRevision deletion.
		// Watch Istio CR to react to monitoring configuration changes
		Watches(&v1.Istio{}, istioHandler).
		// Watch namespaces with istio-injection label to create PodMonitors in them
		// Use predicate to filter only namespaces with injection enabled
		Watches(&corev1.Namespace{}, namespaceHandler, builder.WithPredicates(injectionEnabledPredicate())).
		Complete(reconciler.NewStandardReconciler[*v1.IstioRevision](r.Client, r.Reconcile))
}

// mapNamespaceToReconcileRequest returns reconcile requests for all IstioRevisions
// when a namespace event passes the injectionEnabledPredicate
func (r *Reconciler) mapNamespaceToReconcileRequest(ctx context.Context, obj client.Object) []reconcile.Request {
	log := logf.FromContext(ctx)
	ns, ok := obj.(*corev1.Namespace)
	if !ok {
		log.Error(nil, "unexpected object type", "type", fmt.Sprintf("%T", obj))
		return nil
	}

	// List all IstioRevisions and queue them for reconciliation
	revList := &v1.IstioRevisionList{}
	if err := r.Client.List(ctx, revList); err != nil {
		log.Error(err, "failed to list IstioRevisions")
		return nil
	}

	requests := make([]reconcile.Request, 0, len(revList.Items))
	for _, rev := range revList.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(&rev),
		})
	}

	log.V(2).Info("Namespace with injection label changed, queuing IstioRevisions for reconciliation",
		"namespace", ns.Name, "revisionCount", len(requests))
	return requests
}

// mapIstioToReconcileRequest returns reconcile requests for all IstioRevisions
// owned by the given Istio CR when the Istio CR's monitoring configuration changes
func (r *Reconciler) mapIstioToReconcileRequest(ctx context.Context, obj client.Object) []reconcile.Request {
	log := logf.FromContext(ctx)
	istio, ok := obj.(*v1.Istio)
	if !ok {
		log.Error(nil, "unexpected object type", "type", fmt.Sprintf("%T", obj))
		return nil
	}

	// List IstioRevisions owned by this Istio CR
	revList := &v1.IstioRevisionList{}
	if err := r.Client.List(ctx, revList); err != nil {
		log.Error(err, "failed to list IstioRevisions")
		return nil
	}

	// Find revisions that are owned by this Istio CR
	requests := make([]reconcile.Request, 0)
	for _, rev := range revList.Items {
		for _, ownerRef := range rev.GetOwnerReferences() {
			if ownerRef.Kind == v1.IstioKind && ownerRef.Name == istio.Name {
				requests = append(requests, reconcile.Request{
					NamespacedName: client.ObjectKeyFromObject(&rev),
				})
				break
			}
		}
	}

	log.V(2).Info("Istio CR changed, queuing owned IstioRevisions for reconciliation",
		"istio", istio.Name, "revisionCount", len(requests))
	return requests
}

// injectionEnabledPredicate returns a predicate that filters namespace events
// to only those where the istio-injection label is added, removed, or changed
func injectionEnabledPredicate() predicate.Funcs {
	hasInjectionEnabled := func(obj client.Object) bool {
		if obj == nil {
			return false
		}
		labels := obj.GetLabels()
		return labels != nil && labels[constants.IstioInjectionLabel] == constants.IstioInjectionEnabledValue
	}

	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			// Trigger when a namespace is created with injection enabled
			return hasInjectionEnabled(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Trigger when injection label is added, removed, or changed
			oldHasLabel := hasInjectionEnabled(e.ObjectOld)
			newHasLabel := hasInjectionEnabled(e.ObjectNew)
			return oldHasLabel != newHasLabel
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// Trigger when a namespace with injection enabled is deleted
			return hasInjectionEnabled(e.Object)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return hasInjectionEnabled(e.Object)
		},
	}
}

func wrapEventHandler(logger logr.Logger, h handler.EventHandler) handler.EventHandler {
	return enqueuelogger.WrapIfNecessary(v1.IstioRevisionKind, logger, h)
}
