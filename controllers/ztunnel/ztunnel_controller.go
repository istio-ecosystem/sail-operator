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
	"path"
	"reflect"

	"github.com/go-logr/logr"
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/api/v1alpha1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	"github.com/istio-ecosystem/sail-operator/pkg/enqueuelogger"
	"github.com/istio-ecosystem/sail-operator/pkg/errlist"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
	"github.com/istio-ecosystem/sail-operator/pkg/istiovalues"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	"github.com/istio-ecosystem/sail-operator/pkg/predicate"
	"github.com/istio-ecosystem/sail-operator/pkg/reconciler"
	"github.com/istio-ecosystem/sail-operator/pkg/validation"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

const (
	ztunnelChart   = "ztunnel"
	defaultProfile = "ambient"
)

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

	reconcileErr := r.doReconcile(ctx, ztunnel)

	log.Info("Reconciliation done. Updating status.")
	statusErr := r.updateStatus(ctx, ztunnel, reconcileErr)

	return ctrl.Result{}, errors.Join(reconcileErr, statusErr)
}

func (r *Reconciler) Finalize(ctx context.Context, ztunnel *v1.ZTunnel) error {
	return r.uninstallHelmChart(ctx, ztunnel)
}

func (r *Reconciler) doReconcile(ctx context.Context, ztunnel *v1.ZTunnel) error {
	log := logf.FromContext(ctx)
	if err := r.validate(ctx, ztunnel); err != nil {
		return err
	}

	log.Info("Installing ztunnel Helm chart")
	return r.installHelmChart(ctx, ztunnel)
}

func (r *Reconciler) validate(ctx context.Context, ztunnel *v1.ZTunnel) error {
	if ztunnel.Spec.Version == "" {
		return reconciler.NewValidationError("spec.version not set")
	}
	if ztunnel.Spec.Namespace == "" {
		return reconciler.NewValidationError("spec.namespace not set")
	}
	if err := validation.ValidateTargetNamespace(ctx, r.Client, ztunnel.Spec.Namespace); err != nil {
		return err
	}
	return nil
}

func (r *Reconciler) installHelmChart(ctx context.Context, ztunnel *v1.ZTunnel) error {
	ownerReference := metav1.OwnerReference{
		APIVersion:         v1.GroupVersion.String(),
		Kind:               v1.ZTunnelKind,
		Name:               ztunnel.Name,
		UID:                ztunnel.UID,
		Controller:         ptr.Of(true),
		BlockOwnerDeletion: ptr.Of(true),
	}

	version, err := istioversion.Resolve(ztunnel.Spec.Version)
	if err != nil {
		return fmt.Errorf("failed to resolve Ztunnel version for %q: %w", ztunnel.Name, err)
	}
	// get userValues from ztunnel.spec.values
	userValues := ztunnel.Spec.Values
	if userValues == nil {
		userValues = &v1.ZTunnelValues{}
	}

	// apply image digests from configuration, if not already set by user
	userValues = applyImageDigests(version, userValues, config.Config)

	// apply userValues on top of defaultValues from profiles
	mergedHelmValues, err := istiovalues.ApplyProfilesAndPlatform(
		r.Config.ResourceDirectory, version, r.Config.Platform, r.Config.DefaultProfile, defaultProfile, helm.FromValues(userValues))
	if err != nil {
		return fmt.Errorf("failed to apply profile: %w", err)
	}

	// Apply any user Overrides configured as part of values.ztunnel
	// This step was not required for the IstioCNI resource because the Helm templates[*] automatically override values.cni
	// [*]https://github.com/istio/istio/blob/0200fd0d4c3963a72f36987c2e8c2887df172abf/manifests/charts/istio-cni/templates/zzy_descope_legacy.yaml#L3
	// However, ztunnel charts do not have such a file, hence we are manually applying the mergeOperation here.
	finalHelmValues, err := istiovalues.ApplyUserValues(helm.FromValues(mergedHelmValues), helm.FromValues(userValues.ZTunnel))
	if err != nil {
		return fmt.Errorf("failed to apply user overrides: %w", err)
	}

	_, err = r.ChartManager.UpgradeOrInstallChart(ctx, r.getChartDir(version), finalHelmValues, ztunnel.Spec.Namespace, ztunnelChart, &ownerReference)
	if err != nil {
		return fmt.Errorf("failed to install/update Helm chart %q: %w", ztunnelChart, err)
	}
	return nil
}

func (r *Reconciler) getChartDir(version string) string {
	return path.Join(r.Config.ResourceDirectory, version, "charts", ztunnelChart)
}

func applyImageDigests(version string, values *v1.ZTunnelValues, config config.OperatorConfig) *v1.ZTunnelValues {
	imageDigests, digestsDefined := config.ImageDigests[version]
	// if we don't have default image digests defined for this version, it's a no-op
	if !digestsDefined {
		return values
	}

	// if a global hub or tag value is configured by the user, don't set image digests
	if values != nil && values.Global != nil && (values.Global.Hub != nil || values.Global.Tag != nil) {
		return values
	}

	if values == nil {
		values = &v1.ZTunnelValues{}
	}

	// set image digest unless any part of the image has been configured by the user
	if values.ZTunnel == nil {
		values.ZTunnel = &v1.ZTunnelConfig{}
	}
	if values.ZTunnel.Image == nil && values.ZTunnel.Hub == nil && values.ZTunnel.Tag == nil {
		values.ZTunnel.Image = &imageDigests.ZTunnelImage
	}
	return values
}

func (r *Reconciler) uninstallHelmChart(ctx context.Context, ztunnel *v1.ZTunnel) error {
	_, err := r.ChartManager.UninstallChart(ctx, ztunnelChart, ztunnel.Spec.Namespace)
	if err != nil {
		return fmt.Errorf("failed to uninstall Helm chart %q: %w", ztunnelChart, err)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	logger := mgr.GetLogger().WithName("ctrlr").WithName("ztunnel")

	// mainObjectHandler handles the ZTunnel watch events
	mainObjectHandler := wrapEventHandler(logger, &handler.EnqueueRequestForObject{})

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
		Complete(reconciler.NewStandardReconcilerWithFinalizer[*v1.ZTunnel](r.Client, r.Reconcile, r.Finalize, constants.FinalizerName))
}

func (r *Reconciler) determineStatus(ctx context.Context, ztunnel *v1.ZTunnel, reconcileErr error) (v1.ZTunnelStatus, error) {
	var errs errlist.Builder
	reconciledCondition := r.determineReconciledCondition(reconcileErr)
	readyCondition, err := r.determineReadyCondition(ctx, ztunnel)
	errs.Add(err)

	status := *ztunnel.Status.DeepCopy()
	status.ObservedGeneration = ztunnel.Generation
	status.SetCondition(reconciledCondition)
	status.SetCondition(readyCondition)
	status.State = deriveState(reconciledCondition, readyCondition)
	return status, errs.Error()
}

func (r *Reconciler) updateStatus(ctx context.Context, ztunnel *v1.ZTunnel, reconcileErr error) error {
	var errs errlist.Builder

	status, err := r.determineStatus(ctx, ztunnel, reconcileErr)
	if err != nil {
		errs.Add(fmt.Errorf("failed to determine status: %w", err))
	}

	if !reflect.DeepEqual(ztunnel.Status, status) {
		if err := r.Client.Status().Patch(ctx, ztunnel, kube.NewStatusPatch(status)); err != nil {
			errs.Add(fmt.Errorf("failed to patch status: %w", err))
		}
	}
	return errs.Error()
}

func deriveState(reconciledCondition, readyCondition v1.ZTunnelCondition) v1.ZTunnelConditionReason {
	if reconciledCondition.Status != metav1.ConditionTrue {
		return reconciledCondition.Reason
	} else if readyCondition.Status != metav1.ConditionTrue {
		return readyCondition.Reason
	}
	return v1.ZTunnelReasonHealthy
}

func (r *Reconciler) determineReconciledCondition(err error) v1.ZTunnelCondition {
	c := v1.ZTunnelCondition{Type: v1.ZTunnelConditionReconciled}

	if err == nil {
		c.Status = metav1.ConditionTrue
	} else {
		c.Status = metav1.ConditionFalse
		c.Reason = v1.ZTunnelReasonReconcileError
		c.Message = fmt.Sprintf("error reconciling resource: %v", err)
	}
	return c
}

func (r *Reconciler) determineReadyCondition(ctx context.Context, ztunnel *v1.ZTunnel) (v1.ZTunnelCondition, error) {
	c := v1.ZTunnelCondition{
		Type:   v1.ZTunnelConditionReady,
		Status: metav1.ConditionFalse,
	}

	ds := appsv1.DaemonSet{}
	if err := r.Client.Get(ctx, r.getDaemonSetKey(ztunnel), &ds); err == nil {
		if ds.Status.CurrentNumberScheduled == 0 {
			c.Reason = v1.ZTunnelDaemonSetNotReady
			c.Message = "no ztunnel pods are currently scheduled"
		} else if ds.Status.NumberReady < ds.Status.CurrentNumberScheduled {
			c.Reason = v1.ZTunnelDaemonSetNotReady
			c.Message = "not all ztunnel pods are ready"
		} else {
			c.Status = metav1.ConditionTrue
		}
	} else if apierrors.IsNotFound(err) {
		c.Reason = v1.ZTunnelDaemonSetNotReady
		c.Message = "ztunnel DaemonSet not found"
	} else {
		c.Status = metav1.ConditionUnknown
		c.Reason = v1.ZTunnelReasonReadinessCheckFailed
		c.Message = fmt.Sprintf("failed to get readiness: %v", err)
		return c, fmt.Errorf("get failed: %w", err)
	}
	return c, nil
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

func wrapEventHandler(logger logr.Logger, handler handler.EventHandler) handler.EventHandler {
	return enqueuelogger.WrapIfNecessary(v1.ZTunnelKind, logger, handler)
}
