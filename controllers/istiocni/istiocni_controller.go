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

package istiocni

import (
	"context"
	"errors"
	"fmt"
	"path"
	"reflect"

	"github.com/go-logr/logr"
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
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
	networkingv1 "k8s.io/api/networking/v1"
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

const (
	cniReleaseName = "istio-cni"
	cniChartName   = "cni"
)

// Reconciler reconciles an IstioCNI object
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

// +kubebuilder:rbac:groups=sailoperator.io,resources=istiocnis,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sailoperator.io,resources=istiocnis/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sailoperator.io,resources=istiocnis/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources="*",verbs="*"
// +kubebuilder:rbac:groups="networking.k8s.io",resources="networkpolicies",verbs="*"
// +kubebuilder:rbac:groups="policy",resources="poddisruptionbudgets",verbs="*"
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterroles;clusterrolebindings;roles;rolebindings,verbs="*"
// +kubebuilder:rbac:groups="apps",resources=deployments;daemonsets,verbs="*"
// +kubebuilder:rbac:groups="admissionregistration.k8s.io",resources=validatingwebhookconfigurations;mutatingwebhookconfigurations,verbs="*"
// +kubebuilder:rbac:groups="autoscaling",resources=horizontalpodautoscalers,verbs="*"
// +kubebuilder:rbac:groups="apiextensions.k8s.io",resources=customresourcedefinitions,verbs=get;list;watch
// +kubebuilder:rbac:groups="k8s.cni.cncf.io",resources=network-attachment-definitions,verbs="*"
// +kubebuilder:rbac:groups="security.openshift.io",resources=securitycontextconstraints,resourceNames=privileged,verbs=use
// +kubebuilder:rbac:groups="networking.istio.io",resources=envoyfilters,verbs="*"

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *Reconciler) Reconcile(ctx context.Context, cni *v1.IstioCNI) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	reconcileErr := r.doReconcile(ctx, cni)

	log.Info("Reconciliation done. Updating status.")
	statusErr := r.updateStatus(ctx, cni, reconcileErr)

	return ctrl.Result{}, errors.Join(reconcileErr, statusErr)
}

func (r *Reconciler) Finalize(ctx context.Context, cni *v1.IstioCNI) error {
	return r.uninstallHelmChart(ctx, cni)
}

func (r *Reconciler) doReconcile(ctx context.Context, cni *v1.IstioCNI) error {
	log := logf.FromContext(ctx)
	if err := r.validate(ctx, cni); err != nil {
		return err
	}

	log.Info("Installing Helm chart")
	return r.installHelmChart(ctx, cni)
}

func (r *Reconciler) validate(ctx context.Context, cni *v1.IstioCNI) error {
	if cni.Spec.Version == "" {
		return reconciler.NewValidationError("spec.version not set")
	}
	if cni.Spec.Namespace == "" {
		return reconciler.NewValidationError("spec.namespace not set")
	}
	if err := validation.ValidateTargetNamespace(ctx, r.Client, cni.Spec.Namespace); err != nil {
		return err
	}
	return nil
}

func (r *Reconciler) installHelmChart(ctx context.Context, cni *v1.IstioCNI) error {
	ownerReference := metav1.OwnerReference{
		APIVersion:         v1.GroupVersion.String(),
		Kind:               v1.IstioCNIKind,
		Name:               cni.Name,
		UID:                cni.UID,
		Controller:         ptr.Of(true),
		BlockOwnerDeletion: ptr.Of(true),
	}

	version, err := istioversion.Resolve(cni.Spec.Version)
	if err != nil {
		return fmt.Errorf("failed to resolve IstioCNI version for %q: %w", cni.Name, err)
	}

	// get userValues from Istio.spec.values
	userValues := cni.Spec.Values

	// apply image digests from configuration, if not already set by user
	userValues = applyImageDigests(version, userValues, config.Config)

	// apply vendor-specific default values
	userValues, err = istiovalues.ApplyIstioCNIVendorDefaults(version, userValues)
	if err != nil {
		return fmt.Errorf("failed to apply vendor defaults: %w", err)
	}

	// apply userValues on top of defaultValues from profiles
	mergedHelmValues, err := istiovalues.ApplyProfilesAndPlatform(
		r.Config.ResourceDirectory, version, r.Config.Platform, r.Config.DefaultProfile, cni.Spec.Profile, helm.FromValues(userValues))
	if err != nil {
		return fmt.Errorf("failed to apply profile: %w", err)
	}

	_, err = r.ChartManager.UpgradeOrInstallChart(ctx, r.getChartDir(version), mergedHelmValues, cni.Spec.Namespace, cniReleaseName, &ownerReference)
	if err != nil {
		return fmt.Errorf("failed to install/update Helm chart %q: %w", cniChartName, err)
	}
	return nil
}

func (r *Reconciler) getChartDir(version string) string {
	return path.Join(r.Config.ResourceDirectory, version, "charts", cniChartName)
}

func applyImageDigests(version string, values *v1.CNIValues, config config.OperatorConfig) *v1.CNIValues {
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
		values = &v1.CNIValues{}
	}

	// set image digest unless any part of the image has been configured by the user
	if values.Cni == nil {
		values.Cni = &v1.CNIConfig{}
	}
	if values.Cni.Image == nil && values.Cni.Hub == nil && values.Cni.Tag == nil {
		values.Cni.Image = &imageDigests.CNIImage
	}
	return values
}

func (r *Reconciler) uninstallHelmChart(ctx context.Context, cni *v1.IstioCNI) error {
	_, err := r.ChartManager.UninstallChart(ctx, cniReleaseName, cni.Spec.Namespace)
	if err != nil {
		return fmt.Errorf("failed to uninstall Helm chart %q: %w", cniChartName, err)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	logger := mgr.GetLogger().WithName("ctrlr").WithName("istiocni")

	// mainObjectHandler handles the IstioCNI watch events
	mainObjectHandler := wrapEventHandler(logger, &handler.EnqueueRequestForObject{})

	// ownedResourceHandler handles resources that are owned by the IstioCNI CR
	ownedResourceHandler := wrapEventHandler(logger,
		handler.EnqueueRequestForOwner(r.Scheme, r.RESTMapper(), &v1.IstioCNI{}, handler.OnlyControllerOwner()))

	namespaceHandler := wrapEventHandler(logger, handler.EnqueueRequestsFromMapFunc(r.mapNamespaceToReconcileRequest))

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{
			LogConstructor: func(req *reconcile.Request) logr.Logger {
				log := logger
				if req != nil {
					log = log.WithValues("IstioCNI", req.Name)
				}
				return log
			},
			MaxConcurrentReconciles: r.Config.MaxConcurrentReconciles,
		}).

		// we use the Watches function instead of For(), so that we can wrap the handler so that events that cause the object to be enqueued are logged
		// +lint-watches:ignore: IstioCNI (not found in charts, but this is the main resource watched by this controller)
		Watches(&v1.IstioCNI{}, mainObjectHandler).
		Named("istiocni").

		// namespaced resources
		Watches(&corev1.ConfigMap{}, ownedResourceHandler).
		Watches(&appsv1.DaemonSet{}, ownedResourceHandler).
		Watches(&corev1.ResourceQuota{}, ownedResourceHandler).

		// +lint-watches:ignore: NetworkPolicy (FIXME: NetworkPolicy has not yet been added upstream, but is WIP)
		Watches(&networkingv1.NetworkPolicy{}, ownedResourceHandler, builder.WithPredicates(predicate.IgnoreUpdate())).

		// We use predicate.IgnoreUpdate() so that we skip the reconciliation when a pull secret is added to the ServiceAccount.
		// This is necessary so that we don't remove the newly-added secret.
		// TODO: this is a temporary hack until we implement the correct solution on the Helm-render side
		Watches(&corev1.ServiceAccount{}, ownedResourceHandler, builder.WithPredicates(predicate.IgnoreUpdate())).

		// TODO: only register NetAttachDef if the CRD is installed (may also need to watch for CRD creation)
		// Owns(&multusv1.NetworkAttachmentDefinition{}).

		// cluster-scoped resources
		// +lint-watches:ignore: Namespace (not present in charts, but must be watched to reconcile IstioCni when its namespace is created)
		Watches(&corev1.Namespace{}, namespaceHandler).
		Watches(&rbacv1.ClusterRole{}, ownedResourceHandler).
		Watches(&rbacv1.ClusterRoleBinding{}, ownedResourceHandler).
		Complete(reconciler.NewStandardReconcilerWithFinalizer[*v1.IstioCNI](r.Client, r.Reconcile, r.Finalize, constants.FinalizerName))
}

func (r *Reconciler) determineStatus(ctx context.Context, cni *v1.IstioCNI, reconcileErr error) (v1.IstioCNIStatus, error) {
	var errs errlist.Builder
	reconciledCondition := r.determineReconciledCondition(reconcileErr)
	readyCondition, err := r.determineReadyCondition(ctx, cni)
	errs.Add(err)

	status := *cni.Status.DeepCopy()
	status.ObservedGeneration = cni.Generation
	status.SetCondition(reconciledCondition)
	status.SetCondition(readyCondition)
	status.State = deriveState(reconciledCondition, readyCondition)
	return status, errs.Error()
}

func (r *Reconciler) updateStatus(ctx context.Context, cni *v1.IstioCNI, reconcileErr error) error {
	var errs errlist.Builder

	status, err := r.determineStatus(ctx, cni, reconcileErr)
	if err != nil {
		errs.Add(fmt.Errorf("failed to determine status: %w", err))
	}

	if !reflect.DeepEqual(cni.Status, status) {
		if err := r.Client.Status().Patch(ctx, cni, kube.NewStatusPatch(status)); err != nil {
			errs.Add(fmt.Errorf("failed to patch status: %w", err))
		}
	}
	return errs.Error()
}

func deriveState(reconciledCondition, readyCondition v1.IstioCNICondition) v1.IstioCNIConditionReason {
	if reconciledCondition.Status != metav1.ConditionTrue {
		return reconciledCondition.Reason
	} else if readyCondition.Status != metav1.ConditionTrue {
		return readyCondition.Reason
	}
	return v1.IstioCNIReasonHealthy
}

func (r *Reconciler) determineReconciledCondition(err error) v1.IstioCNICondition {
	c := v1.IstioCNICondition{Type: v1.IstioCNIConditionReconciled}

	if err == nil {
		c.Status = metav1.ConditionTrue
	} else {
		c.Status = metav1.ConditionFalse
		c.Reason = v1.IstioCNIReasonReconcileError
		c.Message = fmt.Sprintf("error reconciling resource: %v", err)
	}
	return c
}

func (r *Reconciler) determineReadyCondition(ctx context.Context, cni *v1.IstioCNI) (v1.IstioCNICondition, error) {
	c := v1.IstioCNICondition{
		Type:   v1.IstioCNIConditionReady,
		Status: metav1.ConditionFalse,
	}

	ds := appsv1.DaemonSet{}
	if err := r.Client.Get(ctx, r.cniDaemonSetKey(cni), &ds); err == nil {
		if ds.Status.CurrentNumberScheduled == 0 {
			c.Reason = v1.IstioCNIDaemonSetNotReady
			c.Message = "no istio-cni-node pods are currently scheduled"
		} else if ds.Status.NumberReady < ds.Status.CurrentNumberScheduled {
			c.Reason = v1.IstioCNIDaemonSetNotReady
			c.Message = "not all istio-cni-node pods are ready"
		} else {
			c.Status = metav1.ConditionTrue
		}
	} else if apierrors.IsNotFound(err) {
		c.Reason = v1.IstioCNIDaemonSetNotReady
		c.Message = "istio-cni-node DaemonSet not found"
	} else {
		c.Status = metav1.ConditionUnknown
		c.Reason = v1.IstioCNIReasonReadinessCheckFailed
		c.Message = fmt.Sprintf("failed to get readiness: %v", err)
		return c, fmt.Errorf("get failed: %w", err)
	}
	return c, nil
}

func (r *Reconciler) cniDaemonSetKey(cni *v1.IstioCNI) client.ObjectKey {
	return client.ObjectKey{
		Namespace: cni.Spec.Namespace,
		Name:      "istio-cni-node",
	}
}

func (r *Reconciler) mapNamespaceToReconcileRequest(ctx context.Context, ns client.Object) []reconcile.Request {
	log := logf.FromContext(ctx)

	// Check if any IstioCNI references this namespace in .spec.namespace
	cniList := v1.IstioCNIList{}
	if err := r.Client.List(ctx, &cniList); err != nil {
		log.Error(err, "failed to list IstioCNIs")
		return nil
	}

	var requests []reconcile.Request
	for _, cni := range cniList.Items {
		if cni.Spec.Namespace == ns.GetName() {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: cni.Name}})
		}
	}
	return requests
}

func wrapEventHandler(logger logr.Logger, handler handler.EventHandler) handler.EventHandler {
	return enqueuelogger.WrapIfNecessary(v1.IstioCNIKind, logger, handler)
}
