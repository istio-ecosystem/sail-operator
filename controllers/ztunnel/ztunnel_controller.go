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

package ztunnel

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/api/v1alpha1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	"github.com/istio-ecosystem/sail-operator/pkg/enqueuelogger"
	"github.com/istio-ecosystem/sail-operator/pkg/errlist"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
	"github.com/istio-ecosystem/sail-operator/pkg/predicate"
	sharedreconcile "github.com/istio-ecosystem/sail-operator/pkg/reconcile"
	"github.com/istio-ecosystem/sail-operator/pkg/reconciler"
	"github.com/istio-ecosystem/sail-operator/pkg/revision"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"istio.io/istio/pkg/ptr"
)

// Reconciler reconciles the ZTunnel object
type Reconciler struct {
	client.Client
	Config       config.ReconcilerConfig
	Scheme       *runtime.Scheme
	ChartManager *helm.ChartManager
}

func NewReconciler(cfg config.ReconcilerConfig, client client.Client, scheme *runtime.Scheme, chartManager *helm.ChartManager) *Reconciler {
	return &Reconciler{
		Config:       cfg,
		Client:       client,
		Scheme:       scheme,
		ChartManager: chartManager,
	}
}

// +kubebuilder:rbac:groups=sailoperator.io,resources=ztunnels,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sailoperator.io,resources=ztunnels/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sailoperator.io,resources=ztunnels/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources="*",verbs="*"
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterroles;clusterrolebindings;roles;rolebindings,verbs="*"
// +kubebuilder:rbac:groups="apps",resources=deployments;daemonsets,verbs="*"
// +kubebuilder:rbac:groups="apiextensions.k8s.io",resources=customresourcedefinitions,verbs=get;list;watch
// +kubebuilder:rbac:groups="security.openshift.io",resources=securitycontextconstraints,resourceNames=privileged,verbs=use

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *Reconciler) Reconcile(ctx context.Context, ztunnel *v1.ZTunnel) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	rev, reconcileErr := r.doReconcile(ctx, ztunnel)

	log.Info("Reconciliation done. Updating status.")
	statusErr := r.updateStatus(ctx, ztunnel, rev, reconcileErr)

	return ctrl.Result{}, errors.Join(reconcileErr, statusErr)
}

func (r *Reconciler) Finalize(ctx context.Context, ztunnel *v1.ZTunnel) error {
	ztunnelReconciler := r.newZTunnelReconciler()
	return ztunnelReconciler.Uninstall(ctx, ztunnel.Spec.Namespace)
}

func (r *Reconciler) doReconcile(ctx context.Context, ztunnel *v1.ZTunnel) (rev *v1.IstioRevision, err error) {
	log := logf.FromContext(ctx)
	ztunnelReconciler := r.newZTunnelReconciler()

	if err := ztunnelReconciler.Validate(ctx, ztunnel.Spec.Version, ztunnel.Spec.Namespace); err != nil {
		return nil, err
	}

	if ztunnel.Spec.TargetRef != nil {
		log.Info("Retrieving referenced IstioRevision")
		rev, err = revision.GetIstioRevisionFromTargetReference(ctx, r.Client, *ztunnel.Spec.TargetRef)
		if err != nil {
			return nil, err
		}
	}

	log.Info("Installing ztunnel Helm chart")
	return rev, r.installHelmChart(ctx, ztunnel, ztunnelReconciler, rev)
}

func (r *Reconciler) installHelmChart(ctx context.Context, ztunnel *v1.ZTunnel,
	ztunnelReconciler *sharedreconcile.ZTunnelReconciler, rev *v1.IstioRevision,
) error {
	ownerReference := metav1.OwnerReference{
		APIVersion:         v1.GroupVersion.String(),
		Kind:               v1.ZTunnelKind,
		Name:               ztunnel.Name,
		UID:                ztunnel.UID,
		Controller:         ptr.Of(true),
		BlockOwnerDeletion: ptr.Of(true),
	}

	if rev != nil && rev.Spec.Values != nil {
		revisionValues := helm.FromValues(v1.Values{
			MeshConfig: rev.Spec.Values.MeshConfig,
			Revision:   rev.Spec.Values.Revision,
			Global:     rev.Spec.Values.Global,
		})
		return ztunnelReconciler.Install(
			ctx, ztunnel.Spec.Version, ztunnel.Spec.Namespace, ztunnel.Spec.Values, &ownerReference, revisionValues)
	}

	return ztunnelReconciler.Install(ctx, ztunnel.Spec.Version, ztunnel.Spec.Namespace, ztunnel.Spec.Values, &ownerReference)
}

func (r *Reconciler) newZTunnelReconciler() *sharedreconcile.ZTunnelReconciler {
	return sharedreconcile.NewZTunnelReconciler(sharedreconcile.Config{
		ResourceFS:        r.Config.ResourceFS,
		Platform:          r.Config.Platform,
		DefaultProfile:    r.Config.DefaultProfile,
		OperatorNamespace: r.Config.OperatorNamespace,
		ChartManager:      r.ChartManager,
	}, r.Client)
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	logger := mgr.GetLogger().WithName("ctrlr").WithName("ztunnel")

	// mainObjectHandler handles the ZTunnel watch events
	mainObjectHandler := wrapEventHandler(logger, &handler.EnqueueRequestForObject{})

	// operatorResourcesHandler handles watch events from operator CRDs Istio and IstioRevision
	operatorResourcesHandler := wrapEventHandler(logger, handler.EnqueueRequestsFromMapFunc(r.mapOperatorResourceToReconcileRequest))

	// ownedResourceHandler handles resources that are owned by the ZTunnel CR
	ownedResourceHandler := wrapEventHandler(logger,
		handler.EnqueueRequestForOwner(r.Scheme, r.RESTMapper(), &v1.ZTunnel{}, handler.OnlyControllerOwner()))

	namespaceHandler := wrapEventHandler(logger, handler.EnqueueRequestsFromMapFunc(r.mapNamespaceToReconcileRequest))

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{
			LogConstructor: func(req *reconcile.Request) logr.Logger {
				log := logger
				if req != nil {
					log = log.WithValues("ztunnel", req.Name)
				}
				return log
			},
			MaxConcurrentReconciles: r.Config.MaxConcurrentReconciles,
		}).

		// we use the Watches function instead of For(), so that we can wrap the handler so that events that cause the object to be enqueued are logged
		Watches(&v1alpha1.ZTunnel{}, mainObjectHandler).
		Watches(&v1.ZTunnel{}, mainObjectHandler).
		Named("ztunnel").

		// namespaced resources
		Watches(&corev1.ConfigMap{}, ownedResourceHandler).
		Watches(&appsv1.DaemonSet{}, ownedResourceHandler).
		Watches(&corev1.ResourceQuota{}, ownedResourceHandler).

		// We use predicate.IgnoreUpdate() so that we skip the reconciliation when a pull secret is added to the ServiceAccount.
		// This is necessary so that we don't remove the newly-added secret.
		// TODO: this is a temporary hack until we implement the correct solution on the Helm-render side
		Watches(&corev1.ServiceAccount{}, ownedResourceHandler, builder.WithPredicates(predicate.IgnoreUpdate())).

		// cluster-scoped resources
		// +lint-watches:ignore: Namespace (not present in charts, but must be watched to reconcile ZTunnel when its namespace is created)
		Watches(&corev1.Namespace{}, namespaceHandler).
		Watches(&rbacv1.ClusterRole{}, ownedResourceHandler).
		Watches(&rbacv1.ClusterRoleBinding{}, ownedResourceHandler).
		Watches(&v1.Istio{}, operatorResourcesHandler).
		Watches(&v1.IstioRevision{}, operatorResourcesHandler).
		Complete(reconciler.NewStandardReconcilerWithFinalizer[*v1.ZTunnel](r.Client, r.Reconcile, r.Finalize, constants.FinalizerName))
}

func (r *Reconciler) determineStatus(ctx context.Context, ztunnel *v1.ZTunnel, rev *v1.IstioRevision, reconcileErr error) (v1.ZTunnelStatus, error) {
	var errs errlist.Builder
	reconciledCondition := r.determineReconciledCondition(reconcileErr)
	readyCondition, err := r.determineReadyCondition(ctx, ztunnel)
	errs.Add(err)

	status := *ztunnel.Status.DeepCopy()
	status.ObservedGeneration = ztunnel.Generation
	status.SetCondition(reconciledCondition)
	status.SetCondition(readyCondition)
	status.State = reconciler.DeriveState(v1.ZTunnelReasonHealthy, reconciledCondition, readyCondition)
	status.IstioRevision = ""
	if rev != nil {
		status.IstioRevision = rev.Name
	}
	return status, errs.Error()
}

func (r *Reconciler) updateStatus(ctx context.Context, ztunnel *v1.ZTunnel, rev *v1.IstioRevision, reconcileErr error) error {
	status, err := r.determineStatus(ctx, ztunnel, rev, reconcileErr)
	return reconciler.UpdateStatus(ctx, r.Client, ztunnel, ztunnel.Status, status, err)
}

func (r *Reconciler) determineReconciledCondition(err error) v1.StatusCondition {
	c := v1.StatusCondition{Type: v1.ZTunnelConditionReconciled}
	if err == nil {
		c.Status = metav1.ConditionTrue
		c.Reason = v1.ConditionReason(v1.ZTunnelConditionReconciled)
	} else {
		c.Status = metav1.ConditionFalse
		c.Reason = v1.ZTunnelReasonReconcileError
		c.Message = fmt.Sprintf("error reconciling resource: %v", err)
	}
	return c
}

func (r *Reconciler) determineReadyCondition(ctx context.Context, ztunnel *v1.ZTunnel) (v1.StatusCondition, error) {
	return reconciler.CheckDaemonSetReadiness(ctx, r.Client, r.getDaemonSetKey(ztunnel),
		"ztunnel", v1.ZTunnelConditionReady, v1.ZTunnelDaemonSetNotReady, v1.ZTunnelReasonReadinessCheckFailed)
}

func (r *Reconciler) getDaemonSetKey(ztunnel *v1.ZTunnel) client.ObjectKey {
	return client.ObjectKey{
		Namespace: ztunnel.Spec.Namespace,
		Name:      "ztunnel",
	}
}

func (r *Reconciler) mapNamespaceToReconcileRequest(ctx context.Context, ns client.Object) []reconcile.Request {
	log := logf.FromContext(ctx)

	// Check if any ZTunnel references this namespace in .spec.namespace
	ztunnelList := v1.ZTunnelList{}
	if err := r.Client.List(ctx, &ztunnelList); err != nil {
		log.Error(err, "failed to list ZTunnels")
		return nil
	}

	var requests []reconcile.Request
	for _, ztunnel := range ztunnelList.Items {
		if ztunnel.Spec.Namespace == ns.GetName() {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: ztunnel.Name}})
		}
	}
	return requests
}

func (r *Reconciler) mapOperatorResourceToReconcileRequest(ctx context.Context, obj client.Object) []reconcile.Request {
	log := logf.FromContext(ctx)
	var revisionName string
	if i, ok := obj.(*v1.Istio); ok && i.Status.ActiveRevisionName != "" {
		revisionName = i.Status.ActiveRevisionName
	} else if rev, ok := obj.(*v1.IstioRevision); ok {
		revisionName = rev.Name
	} else {
		return nil
	}
	ztunnels := v1.ZTunnelList{}
	err := r.Client.List(ctx, &ztunnels)
	if err != nil {
		log.Error(err, "failed to list ZTunnels")
		return nil
	}
	requests := []reconcile.Request{}
	for _, ztunnel := range ztunnels.Items {
		if ztunnel.Status.IstioRevision == revisionName {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: ztunnel.Name}})
		}
	}
	return requests
}

func wrapEventHandler(logger logr.Logger, handler handler.EventHandler) handler.EventHandler {
	return enqueuelogger.WrapIfNecessary(v1.ZTunnelKind, logger, handler)
}
