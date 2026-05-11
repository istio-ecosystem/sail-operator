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
	"github.com/istio-ecosystem/sail-operator/pkg/enqueuelogger"
	"github.com/istio-ecosystem/sail-operator/pkg/reconciler"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	serviceMonitorNameSuffix = "-istiod"
	podMonitorNameSuffix     = "-proxies"

	// COO (Cluster Observability Operator) API group
	rhobsAPIGroup   = "monitoring.rhobs"
	rhobsAPIVersion = "v1"
)

var (
	serviceMonitorGVK = schema.GroupVersionKind{
		Group:   rhobsAPIGroup,
		Version: rhobsAPIVersion,
		Kind:    "ServiceMonitor",
	}
	podMonitorGVK = schema.GroupVersionKind{
		Group:   rhobsAPIGroup,
		Version: rhobsAPIVersion,
		Kind:    "PodMonitor",
	}
)

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

// Reconcile creates or updates ServiceMonitor and PodMonitor resources for each IstioRevision
func (r *Reconciler) Reconcile(ctx context.Context, rev *v1.IstioRevision) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Skip if the IstioRevision is being deleted
	if rev.DeletionTimestamp != nil {
		log.V(2).Info("IstioRevision is being deleted, skipping monitoring reconciliation")
		return ctrl.Result{}, nil
	}

	// Reconcile ServiceMonitor for istiod
	if err := r.reconcileServiceMonitor(ctx, rev); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile ServiceMonitor: %w", err)
	}

	// Reconcile PodMonitor for istio-proxy sidecars
	if err := r.reconcilePodMonitor(ctx, rev); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile PodMonitor: %w", err)
	}

	log.Info("Monitoring resources reconciled successfully")
	return ctrl.Result{}, nil
}

// reconcileServiceMonitor creates or updates the ServiceMonitor for istiod
func (r *Reconciler) reconcileServiceMonitor(ctx context.Context, rev *v1.IstioRevision) error {
	log := logf.FromContext(ctx)
	desired := r.buildServiceMonitor(rev)

	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(serviceMonitorGVK)

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

// reconcilePodMonitor creates or updates the PodMonitor for istio-proxy sidecars
func (r *Reconciler) reconcilePodMonitor(ctx context.Context, rev *v1.IstioRevision) error {
	log := logf.FromContext(ctx)
	desired := r.buildPodMonitor(rev)

	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(podMonitorGVK)

	err := r.Client.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Creating PodMonitor", "name", desired.GetName(), "namespace", desired.GetNamespace())
			return r.Client.Create(ctx, desired)
		}
		return fmt.Errorf("failed to get PodMonitor: %w", err)
	}

	log.V(2).Info("Updating PodMonitor", "name", desired.GetName(), "namespace", desired.GetNamespace())
	desired.SetResourceVersion(existing.GetResourceVersion())
	return r.Client.Update(ctx, desired)
}

// buildServiceMonitor constructs the ServiceMonitor for monitoring istiod
func (r *Reconciler) buildServiceMonitor(rev *v1.IstioRevision) *unstructured.Unstructured {
	name := rev.Name + serviceMonitorNameSuffix
	namespace := rev.Spec.Namespace

	sm := &unstructured.Unstructured{}
	sm.SetGroupVersionKind(serviceMonitorGVK)
	sm.SetName(name)
	sm.SetNamespace(namespace)
	sm.SetLabels(map[string]string{
		"app":          "istiod",
		"monitored-by": "coo-prometheus",
	})
	sm.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion:         v1.GroupVersion.String(),
			Kind:               v1.IstioRevisionKind,
			Name:               rev.Name,
			UID:                rev.UID,
			Controller:         ptrBool(true),
			BlockOwnerDeletion: ptrBool(true),
		},
	})

	// Set the spec
	spec := map[string]interface{}{
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
	}
	unstructured.SetNestedMap(sm.Object, spec, "spec")

	return sm
}

// buildPodMonitor constructs the PodMonitor for monitoring istiod pods
func (r *Reconciler) buildPodMonitor(rev *v1.IstioRevision) *unstructured.Unstructured {
	name := rev.Name + podMonitorNameSuffix
	namespace := rev.Spec.Namespace

	pm := &unstructured.Unstructured{}
	pm.SetGroupVersionKind(podMonitorGVK)
	pm.SetName(name)
	pm.SetNamespace(namespace)
	pm.SetLabels(map[string]string{
		"app":          "istiod",
		"monitored-by": "coo-prometheus",
	})
	pm.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion:         v1.GroupVersion.String(),
			Kind:               v1.IstioRevisionKind,
			Name:               rev.Name,
			UID:                rev.UID,
			Controller:         ptrBool(true),
			BlockOwnerDeletion: ptrBool(true),
		},
	})

	// Set the spec
	spec := map[string]interface{}{
		"selector": map[string]interface{}{
			"matchLabels": map[string]interface{}{
				"app": "istiod",
			},
		},
		"podMetricsEndpoints": []interface{}{
			map[string]interface{}{
				"port":     "http-monitoring",
				"path":     "/metrics",
				"interval": "30s",
			},
		},
	}
	unstructured.SetNestedMap(pm.Object, spec, "spec")

	return pm
}

// SetupWithManager sets up the controller with the Manager
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	logger := mgr.GetLogger().WithName("ctrlr").WithName("monitoring")

	// Create unstructured objects for watching
	serviceMonitorObj := &unstructured.Unstructured{}
	serviceMonitorObj.SetGroupVersionKind(serviceMonitorGVK)

	podMonitorObj := &unstructured.Unstructured{}
	podMonitorObj.SetGroupVersionKind(podMonitorGVK)

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
		Watches(serviceMonitorObj,
			wrapEventHandler(logger, handler.EnqueueRequestForOwner(r.Scheme, mgr.GetRESTMapper(), &v1.IstioRevision{}, handler.OnlyControllerOwner()))).
		Watches(podMonitorObj,
			wrapEventHandler(logger, handler.EnqueueRequestForOwner(r.Scheme, mgr.GetRESTMapper(), &v1.IstioRevision{}, handler.OnlyControllerOwner()))).
		Complete(reconciler.NewStandardReconciler[*v1.IstioRevision](r.Client, r.Reconcile))
}

func wrapEventHandler(logger logr.Logger, h handler.EventHandler) handler.EventHandler {
	return enqueuelogger.WrapIfNecessary(v1.IstioRevisionKind, logger, h)
}

func ptrBool(b bool) *bool {
	return &b
}
