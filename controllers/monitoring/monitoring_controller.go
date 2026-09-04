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
	"github.com/istio-ecosystem/sail-operator/pkg/monitoring/relabeling"
	"github.com/istio-ecosystem/sail-operator/pkg/reconciler"
	"github.com/istio-ecosystem/sail-operator/pkg/revision"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"istio.io/istio/pkg/ptr"
)

const (
	serviceMonitorNameSuffix = "-istiod-metrics"
	podMonitorNameSuffix     = "-proxies-metrics"

	// Labels
	monitoredByLabel    = "monitored-by"
	kubePrometheusValue = "kube-prometheus"
	releaseLabel        = "release"
	releaseLabelValue   = "istio"
	monitoringLabel     = "monitoring"

	// Scrape settings from upstream Istio prometheus-operator sample.
	scrapeInterval           = monitoringv1.Duration("15s")
	serviceMonitorJobLabel   = "istio"
	podMonitorJobLabel       = "envoy-stats"
	serviceMonitorMonitoring = "istio-components"
	podMonitorMonitoring     = "istio-proxies"
)

func (r *Reconciler) monitorLabels(app, monitoring string) map[string]string {
	return map[string]string{
		"app":                       app,
		constants.ManagedByLabelKey: constants.ManagedByLabelValue,
		monitoredByLabel:            kubePrometheusValue,
		releaseLabel:                releaseLabelValue,
		monitoringLabel:             monitoring,
	}
}

func monitoringEnabled(istio *v1.Istio) bool {
	return istio.Annotations[constants.MonitoringAnnotationKey] == constants.MonitoringAnnotationEnabledValue
}

// TODO: map tuningEnabled from an Integration API spec field in a follow-up enhancement.

// Reconciler reconciles monitoring resources (ServiceMonitor, PodMonitor) for Istio objects.
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

// +kubebuilder:rbac:groups=sailoperator.io,resources=istios;istiorevisions,verbs=get;list;watch
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors;podmonitors,verbs=get;list;create;delete
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch

// Reconcile creates ServiceMonitor and PodMonitor resources for each IstioRevision owned by the Istio
// when monitoring is enabled. Existing monitor resources are left unchanged so users can customize
// labels or specs without being overwritten. Orphaned PodMonitors are not deleted when injection
// labels are removed from a namespace.
func (r *Reconciler) Reconcile(ctx context.Context, istio *v1.Istio) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	log.Info("Reconciling")
	err := r.doReconcile(ctx, istio)
	log.Info("Reconciliation done")
	return ctrl.Result{}, err
}

// doReconcile reconciles monitoring resources for all IstioRevisions owned by the Istio.
func (r *Reconciler) doReconcile(ctx context.Context, istio *v1.Istio) error {
	log := logf.FromContext(ctx)

	if istio.DeletionTimestamp != nil {
		log.V(2).Info("Istio is being deleted, skipping monitoring reconciliation")
		return nil
	}

	if !monitoringEnabled(istio) {
		log.V(2).Info("Monitoring is not enabled on Istio CR, skipping reconciliation")
		return nil
	}

	revisions, err := revision.ListOwned(ctx, r.Client, istio.UID)
	if err != nil {
		return fmt.Errorf("failed to list IstioRevisions for Istio: %w", err)
	}

	for i := range revisions {
		rev := &revisions[i]
		if rev.DeletionTimestamp != nil {
			log.V(2).Info("IstioRevision is being deleted, skipping", "IstioRevision", rev.Name)
			continue
		}

		if err := r.reconcileServiceMonitor(ctx, istio, rev); err != nil {
			return fmt.Errorf("failed to reconcile ServiceMonitor for revision %s: %w", rev.Name, err)
		}

		if err := r.reconcilePodMonitors(ctx, istio, rev); err != nil {
			return fmt.Errorf("failed to reconcile PodMonitors for revision %s: %w", rev.Name, err)
		}
	}

	log.Info("Monitoring resources reconciled successfully", "revisionCount", len(revisions))
	return nil
}

// reconcileServiceMonitor creates the ServiceMonitor for istiod if it does not already exist.
func (r *Reconciler) reconcileServiceMonitor(ctx context.Context, istio *v1.Istio, rev *v1.IstioRevision) error {
	log := logf.FromContext(ctx)
	desired := r.buildServiceMonitor(istio, rev)

	existing := &monitoringv1.ServiceMonitor{}
	err := r.Client.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Creating ServiceMonitor", "name", desired.GetName(), "namespace", desired.GetNamespace())
			return r.Client.Create(ctx, desired)
		}
		return fmt.Errorf("failed to get ServiceMonitor: %w", err)
	}

	log.V(2).Info("ServiceMonitor already exists, leaving unchanged")
	return nil
}

// reconcilePodMonitors creates PodMonitors in namespaces selected for this revision.
// Matching namespaces are those with istio.io/rev=<revision>, plus istio-injection=enabled
// when this is the default revision. Existing PodMonitors are left unchanged.
func (r *Reconciler) reconcilePodMonitors(ctx context.Context, istio *v1.Istio, rev *v1.IstioRevision) error {
	log := logf.FromContext(ctx)

	namespaces, err := r.namespacesForRevision(ctx, rev)
	if err != nil {
		return err
	}

	for _, ns := range namespaces {
		if err := r.reconcilePodMonitorInNamespace(ctx, istio, rev, ns.Name); err != nil {
			return fmt.Errorf("failed to reconcile PodMonitor in namespace %s: %w", ns.Name, err)
		}
		log.V(2).Info("Reconciled PodMonitor for injection namespace", "namespace", ns.Name, "revision", rev.Name)
	}

	return nil
}

// namespacesForRevision returns namespaces where sidecar injection is enabled for the given revision.
// A namespace matches when istio-injection=enabled (default revision) or istio.io/rev=<revision>.
// istio-injection takes precedence over istio.io/rev, matching Istio's injection selection rules.
func (r *Reconciler) namespacesForRevision(ctx context.Context, rev *v1.IstioRevision) ([]corev1.Namespace, error) {
	seen := map[string]struct{}{}
	var namespaces []corev1.Namespace

	appendUnique := func(nsList *corev1.NamespaceList) {
		for _, ns := range nsList.Items {
			if _, ok := seen[ns.Name]; ok {
				continue
			}
			seen[ns.Name] = struct{}{}
			namespaces = append(namespaces, ns)
		}
	}

	// Default revision owns namespaces labeled istio-injection=enabled.
	if rev.Name == v1.DefaultRevision {
		injectionList := &corev1.NamespaceList{}
		if err := r.Client.List(ctx, injectionList, client.MatchingLabels{
			constants.IstioInjectionLabel: constants.IstioInjectionEnabledValue,
		}); err != nil {
			return nil, fmt.Errorf("failed to list namespaces with istio-injection enabled: %w", err)
		}
		appendUnique(injectionList)
	}

	// Named (and default) revisions also own namespaces labeled istio.io/rev=<revision>.
	// Skip namespaces that also have istio-injection=enabled, since that label takes precedence
	// and belongs to the default revision.
	revList := &corev1.NamespaceList{}
	if err := r.Client.List(ctx, revList, client.MatchingLabels{
		constants.IstioRevLabel: rev.Name,
	}); err != nil {
		return nil, fmt.Errorf("failed to list namespaces with istio.io/rev=%s: %w", rev.Name, err)
	}
	for _, ns := range revList.Items {
		if _, ok := seen[ns.Name]; ok {
			continue
		}
		if ns.Labels[constants.IstioInjectionLabel] == constants.IstioInjectionEnabledValue {
			continue
		}
		seen[ns.Name] = struct{}{}
		namespaces = append(namespaces, ns)
	}

	return namespaces, nil
}

// reconcilePodMonitorInNamespace creates a PodMonitor in the specified namespace if it does not already exist.
func (r *Reconciler) reconcilePodMonitorInNamespace(ctx context.Context, istio *v1.Istio, rev *v1.IstioRevision, namespace string) error {
	log := logf.FromContext(ctx)
	desired := r.buildPodMonitor(istio, rev, namespace)

	existing := &monitoringv1.PodMonitor{}
	err := r.Client.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Creating PodMonitor", "name", desired.GetName(), "namespace", namespace)
			return r.Client.Create(ctx, desired)
		}
		return fmt.Errorf("failed to get PodMonitor: %w", err)
	}

	log.V(2).Info("PodMonitor already exists, leaving unchanged", "name", desired.GetName(), "namespace", namespace)
	return nil
}

// buildServiceMonitor constructs the ServiceMonitor for monitoring istiod
func (r *Reconciler) buildServiceMonitor(istio *v1.Istio, rev *v1.IstioRevision) *monitoringv1.ServiceMonitor {
	name := rev.Name + serviceMonitorNameSuffix
	namespace := rev.Spec.Namespace
	// TODO: map tuningEnabled from an Integration API spec field in a follow-up enhancement.
	relabelCfg := relabeling.ForPlatform(r.Config.Platform, istio.Name, false)

	sm := &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    r.monitorLabels("istiod", serviceMonitorMonitoring),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: v1.GroupVersion.String(),
					Kind:       v1.IstioRevisionKind,
					Name:       rev.Name,
					UID:        rev.UID,
					Controller: ptr.Of(true),
				},
			},
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			JobLabel:     serviceMonitorJobLabel,
			TargetLabels: []string{"app"},
			Selector: metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "istio",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"pilot"},
					},
				},
			},
			Endpoints: []monitoringv1.Endpoint{
				{
					Port:           "http-monitoring",
					Interval:       scrapeInterval,
					RelabelConfigs: relabelCfg.ServiceMonitorRelabelings,
				},
			},
		},
	}

	return sm
}

// buildPodMonitor constructs the PodMonitor for monitoring istio-proxy sidecars
func (r *Reconciler) buildPodMonitor(istio *v1.Istio, rev *v1.IstioRevision, namespace string) *monitoringv1.PodMonitor {
	name := rev.Name + podMonitorNameSuffix
	// TODO: map tuningEnabled from an Integration API spec field in a follow-up enhancement.
	relabelCfg := relabeling.ForPlatform(r.Config.Platform, istio.Name, false)

	pm := &monitoringv1.PodMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    r.monitorLabels("istio-proxy", podMonitorMonitoring),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: v1.GroupVersion.String(),
					Kind:       v1.IstioRevisionKind,
					Name:       rev.Name,
					UID:        rev.UID,
					Controller: ptr.Of(true),
				},
			},
		},
		Spec: monitoringv1.PodMonitorSpec{
			JobLabel: podMonitorJobLabel,
			Selector: metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      "istio-prometheus-ignore",
						Operator: metav1.LabelSelectorOpDoesNotExist,
					},
				},
			},
			PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{
				{
					Path:           "/stats/prometheus",
					Interval:       scrapeInterval,
					RelabelConfigs: relabelCfg.PodMonitorRelabelings,
				},
			},
		},
	}

	return pm
}

// SetupWithManager sets up the controller with the Manager
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	logger := mgr.GetLogger().WithName("ctrlr").WithName("monitoring")

	// mainObjectHandler handles Istio watch events
	mainObjectHandler := wrapEventHandler(logger, &handler.EnqueueRequestForObject{})

	// ownedRevisionHandler enqueues the parent Istio when an owned IstioRevision changes
	ownedRevisionHandler := wrapEventHandler(logger,
		handler.EnqueueRequestForOwner(r.Scheme, r.RESTMapper(), &v1.Istio{}, handler.OnlyControllerOwner()))

	// namespaceHandler enqueues the Istio that owns the revision referenced by the
	// namespace's sidecar injection labels. On update, the mapper runs for both the
	// old and new namespace so the previous and current Istio are both requeued.
	namespaceHandler := wrapEventHandler(logger, handler.EnqueueRequestsFromMapFunc(r.mapNamespaceToReconcileRequest))

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{
			LogConstructor: func(req *reconcile.Request) logr.Logger {
				log := logger
				if req != nil {
					log = log.WithValues("Istio", req.Name)
				}
				return log
			},
			MaxConcurrentReconciles: r.Config.MaxConcurrentReconciles,
		}).
		Named("monitoring").
		Watches(&v1.Istio{}, mainObjectHandler).
		// Watch IstioRevisions so create/update/delete requeues the parent Istio.
		// ServiceMonitor and PodMonitor owner references on IstioRevision handle GC on revision deletion.
		Watches(&v1.IstioRevision{}, ownedRevisionHandler).
		// Watch namespaces so sidecar injection label changes requeue the referenced Istio.
		Watches(&corev1.Namespace{}, namespaceHandler, builder.WithPredicates(sidecarInjectionNamespacePredicate())).
		Complete(reconciler.NewStandardReconciler[*v1.Istio](r.Client, r.Reconcile))
}

// mapNamespaceToReconcileRequest returns a reconcile request for the Istio that owns
// the IstioRevision referenced by the namespace's sidecar injection labels.
// EnqueueRequestsFromMapFunc invokes this for both the old and new object on updates,
// so a label change requeues the previous Istio and the newly referenced Istio.
func (r *Reconciler) mapNamespaceToReconcileRequest(ctx context.Context, obj client.Object) []reconcile.Request {
	log := logf.FromContext(ctx)
	ns, ok := obj.(*corev1.Namespace)
	if !ok {
		log.Error(nil, "unexpected object type", "type", fmt.Sprintf("%T", obj))
		return nil
	}

	revisionName := revision.GetReferencedRevisionFromNamespace(ns.GetLabels())
	if revisionName == "" {
		return nil
	}

	rev := &v1.IstioRevision{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: revisionName}, rev); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Error(err, "failed to get IstioRevision referenced by namespace",
				"namespace", ns.Name, "IstioRevision", revisionName)
		}
		return nil
	}

	istioName := ownerIstioName(rev)
	if istioName == "" {
		return nil
	}

	log.V(2).Info("Namespace sidecar injection labels changed, queuing Istio for reconciliation",
		"namespace", ns.Name, "Istio", istioName, "IstioRevision", revisionName)
	return []reconcile.Request{{
		NamespacedName: client.ObjectKey{Name: istioName},
	}}
}

// ownerIstioName returns the name of the Istio CR that owns the given IstioRevision.
func ownerIstioName(rev *v1.IstioRevision) string {
	owner := metav1.GetControllerOf(rev)
	if owner == nil {
		return ""
	}
	if owner.APIVersion != v1.GroupVersion.String() || owner.Kind != v1.IstioKind {
		return ""
	}
	return owner.Name
}

// sidecarInjectionNamespacePredicate returns a predicate that filters namespace events
// to those where istio-injection or istio.io/rev labels are added, removed, or changed.
func sidecarInjectionNamespacePredicate() predicate.Funcs {
	injectionLabelState := func(obj client.Object) string {
		if obj == nil {
			return ""
		}
		labels := obj.GetLabels()
		if labels == nil {
			return ""
		}
		injection := labels[constants.IstioInjectionLabel]
		rev := labels[constants.IstioRevLabel]
		if injection == "" && rev == "" {
			return ""
		}
		return injection + "|" + rev
	}

	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return injectionLabelState(e.Object) != ""
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return injectionLabelState(e.ObjectOld) != injectionLabelState(e.ObjectNew)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return injectionLabelState(e.Object) != ""
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return injectionLabelState(e.Object) != ""
		},
	}
}

func wrapEventHandler(logger logr.Logger, h handler.EventHandler) handler.EventHandler {
	return enqueuelogger.WrapIfNecessary(v1.IstioKind, logger, h)
}
