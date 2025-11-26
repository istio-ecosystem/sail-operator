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

package istiorevision

import (
	"context"
	"errors"
	"fmt"
	"path"
	"reflect"
	"regexp"

	"github.com/go-logr/logr"
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	"github.com/istio-ecosystem/sail-operator/pkg/enqueuelogger"
	"github.com/istio-ecosystem/sail-operator/pkg/errlist"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	predicate2 "github.com/istio-ecosystem/sail-operator/pkg/predicate"
	"github.com/istio-ecosystem/sail-operator/pkg/reconciler"
	"github.com/istio-ecosystem/sail-operator/pkg/revision"
	"github.com/istio-ecosystem/sail-operator/pkg/validation"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
	istioCniName = "default"
	ztunnelName  = "default"
)

// Reconciler reconciles an IstioRevision object
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

// +kubebuilder:rbac:groups=sailoperator.io,resources=istiorevisions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sailoperator.io,resources=istiorevisions/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sailoperator.io,resources=istiorevisions/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources="*",verbs="*"
// +kubebuilder:rbac:groups="networking.k8s.io",resources="networkpolicies",verbs="*"
// +kubebuilder:rbac:groups="policy",resources="poddisruptionbudgets",verbs="*"
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterroles;clusterrolebindings;roles;rolebindings,verbs="*"
// +kubebuilder:rbac:groups="apps",resources=deployments;daemonsets,verbs="*"
// +kubebuilder:rbac:groups="admissionregistration.k8s.io",resources=validatingwebhookconfigurations;mutatingwebhookconfigurations,verbs="*"
// +kubebuilder:rbac:groups="autoscaling",resources=horizontalpodautoscalers,verbs="*"
// +kubebuilder:rbac:groups="discovery.k8s.io",resources=endpointslices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="apiextensions.k8s.io",resources=customresourcedefinitions,verbs=get;list;watch
// +kubebuilder:rbac:groups="k8s.cni.cncf.io",resources=network-attachment-definitions,verbs="*"
// +kubebuilder:rbac:groups="security.openshift.io",resources=securitycontextconstraints,resourceNames=privileged,verbs=use
// +kubebuilder:rbac:groups="networking.istio.io",resources=envoyfilters,verbs="*"

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *Reconciler) Reconcile(ctx context.Context, rev *v1.IstioRevision) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	reconcileErr := r.doReconcile(ctx, rev)

	log.Info("Reconciliation done. Updating status.")
	statusErr := r.updateStatus(ctx, rev, reconcileErr)

	return ctrl.Result{}, errors.Join(reconcileErr, statusErr)
}

func (r *Reconciler) doReconcile(ctx context.Context, rev *v1.IstioRevision) error {
	log := logf.FromContext(ctx)
	if err := r.validate(ctx, rev); err != nil {
		return err
	}

	log.Info("Installing Helm chart")
	return r.installHelmCharts(ctx, rev)
}

func (r *Reconciler) Finalize(ctx context.Context, rev *v1.IstioRevision) error {
	return r.uninstallHelmCharts(ctx, rev)
}

func (r *Reconciler) validate(ctx context.Context, rev *v1.IstioRevision) error {
	if rev.Spec.Version == "" {
		return reconciler.NewValidationError("spec.version not set")
	}
	if rev.Spec.Namespace == "" {
		return reconciler.NewValidationError("spec.namespace not set")
	}
	if err := validation.ValidateTargetNamespace(ctx, r.Client, rev.Spec.Namespace); err != nil {
		return err
	}

	if rev.Spec.Values == nil {
		return reconciler.NewValidationError("spec.values not set")
	}

	revName := rev.Spec.Values.Revision
	if rev.Name == v1.DefaultRevision && (revName != nil && *revName != "") {
		return reconciler.NewValidationError(fmt.Sprintf("spec.values.revision must be \"\" when IstioRevision name is %s", v1.DefaultRevision))
	} else if rev.Name != v1.DefaultRevision && (revName == nil || *revName != rev.Name) {
		return reconciler.NewValidationError("spec.values.revision does not match IstioRevision name")
	}

	if rev.Spec.Values.Global == nil || rev.Spec.Values.Global.IstioNamespace == nil || *rev.Spec.Values.Global.IstioNamespace != rev.Spec.Namespace {
		return reconciler.NewValidationError("spec.values.global.istioNamespace does not match spec.namespace")
	}

	tag := v1.IstioRevisionTag{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: rev.Name}, &tag); err == nil {
		if validation.ResourceTakesPrecedence(&tag.ObjectMeta, &rev.ObjectMeta) {
			return reconciler.NewNameAlreadyExistsError("an IstioRevisionTag exists with this name", nil)
		}
	} else if !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}

func (r *Reconciler) installHelmCharts(ctx context.Context, rev *v1.IstioRevision) error {
	ownerReference := metav1.OwnerReference{
		APIVersion:         v1.GroupVersion.String(),
		Kind:               v1.IstioRevisionKind,
		Name:               rev.Name,
		UID:                rev.UID,
		Controller:         ptr.Of(true),
		BlockOwnerDeletion: ptr.Of(true),
	}

	values := helm.FromValues(rev.Spec.Values)
	_, err := r.ChartManager.UpgradeOrInstallChart(ctx, r.getChartDir(rev, constants.IstiodChartName),
		values, rev.Spec.Namespace, getReleaseName(rev, constants.IstiodChartName), &ownerReference)
	if err != nil {
		return fmt.Errorf("failed to install/update Helm chart %q: %w", constants.IstiodChartName, err)
	}
	if rev.Name == v1.DefaultRevision {
		_, err := r.ChartManager.UpgradeOrInstallChart(ctx, r.getChartDir(rev, constants.BaseChartName),
			values, r.Config.OperatorNamespace, getReleaseName(rev, constants.BaseChartName), &ownerReference)
		if err != nil {
			return fmt.Errorf("failed to install/update Helm chart %q: %w", constants.BaseChartName, err)
		}
	}
	return nil
}

func getReleaseName(rev *v1.IstioRevision, chartName string) string {
	return fmt.Sprintf("%s-%s", rev.Name, chartName)
}

func (r *Reconciler) getChartDir(rev *v1.IstioRevision, chartName string) string {
	return path.Join(r.Config.ResourceDirectory, rev.Spec.Version, "charts", chartName)
}

func (r *Reconciler) uninstallHelmCharts(ctx context.Context, rev *v1.IstioRevision) error {
	if _, err := r.ChartManager.UninstallChart(ctx, getReleaseName(rev, constants.IstiodChartName), rev.Spec.Namespace); err != nil {
		return fmt.Errorf("failed to uninstall Helm chart %q: %w", constants.IstiodChartName, err)
	}
	if rev.Name == v1.DefaultRevision {
		_, err := r.ChartManager.UninstallChart(ctx, getReleaseName(rev, constants.BaseChartName), r.Config.OperatorNamespace)
		if err != nil {
			return fmt.Errorf("failed to uninstall Helm chart %q: %w", constants.BaseChartName, err)
		}
	}
	return nil
}

func (r *Reconciler) mapEndpointSliceToReconcileRequests(ctx context.Context, obj client.Object) []reconcile.Request {
	// EndpointSlices may be owned by an Endpoints resource (if mirrored) which is in turn owned
	// by an IstioRevision, or in the future, EndpointSlices may be owned directly by an IstioRevision.

	controller := metav1.GetControllerOf(obj)
	if controller == nil {
		return nil
	}
	if controller.APIVersion == v1.GroupVersion.String() && controller.Kind == v1.IstioRevisionKind {
		return []reconcile.Request{
			{NamespacedName: types.NamespacedName{Name: controller.Name}},
		}
	}
	if controller.APIVersion == corev1.SchemeGroupVersion.String() && controller.Kind == "Endpoints" {
		// nolint:staticcheck
		ep := &corev1.Endpoints{}
		if err := r.Client.Get(ctx, types.NamespacedName{Namespace: obj.GetNamespace(), Name: controller.Name}, ep); err != nil {
			return nil
		}

		controller := metav1.GetControllerOf(ep)
		if controller != nil && controller.APIVersion == v1.GroupVersion.String() && controller.Kind == v1.IstioRevisionKind {
			return []reconcile.Request{
				{NamespacedName: types.NamespacedName{Name: controller.Name}},
			}
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	logger := mgr.GetLogger().WithName("ctrlr").WithName("istiorev")

	// mainObjectHandler handles the IstioRevision watch events
	mainObjectHandler := wrapEventHandler(logger, &handler.EnqueueRequestForObject{})

	// ownedResourceHandler handles resources that are owned by the IstioRevision CR
	ownedResourceHandler := wrapEventHandler(logger,
		handler.EnqueueRequestForOwner(r.Scheme, r.RESTMapper(), &v1.IstioRevision{}, handler.OnlyControllerOwner()))

	// nsHandler triggers reconciliation in two cases:
	// - when a namespace that is referenced in IstioRevision.spec.namespace is
	//   created, so that the control plane is installed immediately.
	// - when a namespace that references the IstioRevision CR via the istio.io/rev
	//   or istio-injection labels is updated, so that the InUse condition of
	//   the IstioRevision CR is updated.
	nsHandler := wrapEventHandler(logger, handler.EnqueueRequestsFromMapFunc(r.mapNamespaceToReconcileRequest))

	// podHandler handles pods that reference the IstioRevision CR via the istio.io/rev or sidecar.istio.io/inject labels.
	// The handler triggers the reconciliation of the referenced IstioRevision CR so that its InUse condition is updated.
	podHandler := wrapEventHandler(logger, handler.EnqueueRequestsFromMapFunc(r.mapPodToReconcileRequest))

	// revisionTagHandler handles IstioRevisionTags that reference the IstioRevision CR via their targetRef.
	// The handler triggers the reconciliation of the referenced IstioRevision CR so that its InUse condition is updated.
	revisionTagHandler := wrapEventHandler(logger, handler.EnqueueRequestsFromMapFunc(r.mapRevisionTagToReconcileRequest))

	istioCniHandler := wrapEventHandler(logger, handler.EnqueueRequestsFromMapFunc(r.mapIstioCniToReconcileRequests))

	ztunnelHandler := wrapEventHandler(logger, handler.EnqueueRequestsFromMapFunc(r.mapZTunnelToReconcileRequests))

	// endpointSliceHandler triggers reconciliation if the EndpointSlice is owned directly by an IstioRevision,
	// or if it's owned by an Endpoints object which in turn is owned by an IstioRevision.
	endpointSliceHandler := wrapEventHandler(logger, handler.EnqueueRequestsFromMapFunc(r.mapEndpointSliceToReconcileRequests))

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
		// we use the Watches function instead of For(), so that we can wrap the handler so that events that cause the object to be enqueued are logged
		// +lint-watches:ignore: IstioRevision (not found in charts, but this is the main resource watched by this controller)
		Watches(&v1.IstioRevision{}, mainObjectHandler).
		Named("istiorevision").

		// namespaced resources
		Watches(&corev1.ConfigMap{}, ownedResourceHandler, builder.WithPredicates(predicate2.IgnoreUpdateWhenAnnotation())).
		// We don't ignore the status for Deployments because we use it to calculate the IstioRevision status
		Watches(&appsv1.Deployment{}, ownedResourceHandler, builder.WithPredicates(predicate2.IgnoreUpdateWhenAnnotation())).
		// +lint-watches:ignore: Endpoints (older versions of istiod chart create Endpoints for remote installs, but this controller watches EndpointSlices)
		// +lint-watches:ignore: EndpointSlice (istiod chart creates Endpoints for remote installs, but this controller watches EndpointSlices)
		Watches(&discoveryv1.EndpointSlice{}, endpointSliceHandler, builder.WithPredicates(predicate2.IgnoreUpdateWhenAnnotation())).
		Watches(&corev1.Service{}, ownedResourceHandler,
			builder.WithPredicates(ignoreStatusChange(), predicate2.IgnoreUpdateWhenAnnotation())).

		// +lint-watches:ignore: NetworkPolicy (FIXME: NetworkPolicy has not yet been added upstream, but is WIP)
		Watches(&networkingv1.NetworkPolicy{}, ownedResourceHandler,
			builder.WithPredicates(ignoreStatusChange(), predicate2.IgnoreUpdateWhenAnnotation())).

		// We use predicate.IgnoreUpdate() so that we skip the reconciliation when a pull secret is added to the ServiceAccount.
		// This is necessary so that we don't remove the newly-added secret.
		// TODO: this is a temporary hack until we implement the correct solution on the Helm-render side
		Watches(&corev1.ServiceAccount{}, ownedResourceHandler, builder.WithPredicates(predicate2.IgnoreUpdate())).
		Watches(&rbacv1.Role{}, ownedResourceHandler, builder.WithPredicates(predicate2.IgnoreUpdateWhenAnnotation())).
		Watches(&rbacv1.RoleBinding{}, ownedResourceHandler, builder.WithPredicates(predicate2.IgnoreUpdateWhenAnnotation())).
		Watches(&policyv1.PodDisruptionBudget{}, ownedResourceHandler,
			builder.WithPredicates(ignoreStatusChange(), predicate2.IgnoreUpdateWhenAnnotation())).
		Watches(&autoscalingv2.HorizontalPodAutoscaler{}, ownedResourceHandler,
			builder.WithPredicates(ignoreStatusChange(), predicate2.IgnoreUpdateWhenAnnotation())).

		// +lint-watches:ignore: Namespace (not found in charts, but must be watched to reconcile IstioRevision when its namespace is created)
		Watches(&corev1.Namespace{}, nsHandler, builder.WithPredicates(ignoreStatusChange()), builder.WithPredicates(predicate2.IgnoreUpdateWhenAnnotation())).

		// +lint-watches:ignore: Pod (not found in charts, but must be watched to reconcile IstioRevision when a pod references it)
		Watches(&corev1.Pod{}, podHandler, builder.WithPredicates(ignoreStatusChange(), predicate2.IgnoreUpdateWhenAnnotation())).

		// +lint-watches:ignore: IstioRevisionTag (not found in charts, but must be watched to reconcile IstioRevision when a pod references it)
		Watches(&v1.IstioRevisionTag{}, revisionTagHandler).

		// cluster-scoped resources
		Watches(&rbacv1.ClusterRole{}, ownedResourceHandler, builder.WithPredicates(predicate2.IgnoreUpdateWhenAnnotation())).
		Watches(&rbacv1.ClusterRoleBinding{}, ownedResourceHandler, builder.WithPredicates(predicate2.IgnoreUpdateWhenAnnotation())).
		Watches(&admissionv1.MutatingWebhookConfiguration{}, ownedResourceHandler, builder.WithPredicates(predicate2.IgnoreUpdateWhenAnnotation())).
		Watches(&admissionv1.ValidatingWebhookConfiguration{}, ownedResourceHandler,
			builder.WithPredicates(validatingWebhookConfigPredicate(), predicate2.IgnoreUpdateWhenAnnotation())).

		// +lint-watches:ignore: IstioCNI (not found in charts, but this controller needs to watch it to update the IstioRevision status)
		Watches(&v1.IstioCNI{}, istioCniHandler).

		// +lint-watches:ignore: ZTunnel (not found in charts, but this controller needs to watch it to update the IstioRevision status)
		Watches(&v1.ZTunnel{}, ztunnelHandler).

		// +lint-watches:ignore: ValidatingAdmissionPolicy (TODO: fix this when CI supports golang 1.22 and k8s 1.30)
		// +lint-watches:ignore: ValidatingAdmissionPolicyBinding (TODO: fix this when CI supports golang 1.22 and k8s 1.30)
		// +lint-watches:ignore: CustomResourceDefinition (prevents `make lint-watches` from bugging us about CRDs)
		Complete(reconciler.NewStandardReconcilerWithFinalizer[*v1.IstioRevision](r.Client, r.Reconcile, r.Finalize, constants.FinalizerName))
}

func (r *Reconciler) determineStatus(ctx context.Context, rev *v1.IstioRevision, reconcileErr error) (v1.IstioRevisionStatus, error) {
	var errs errlist.Builder
	reconciledCondition := r.determineReconciledCondition(reconcileErr)
	readyCondition, err := r.determineReadyCondition(ctx, rev)
	errs.Add(err)
	dependenciesHealthyCondition, err := r.determineDependenciesHealthyCondition(ctx, rev)
	errs.Add(err)

	inUseCondition, err := r.determineInUseCondition(ctx, rev)
	errs.Add(err)

	status := *rev.Status.DeepCopy()
	status.ObservedGeneration = rev.Generation
	status.SetCondition(reconciledCondition)
	status.SetCondition(readyCondition)
	status.SetCondition(dependenciesHealthyCondition)
	status.SetCondition(inUseCondition)
	status.State = deriveState(reconciledCondition, readyCondition, dependenciesHealthyCondition)
	return status, errs.Error()
}

func (r *Reconciler) updateStatus(ctx context.Context, rev *v1.IstioRevision, reconcileErr error) error {
	var errs errlist.Builder

	status, err := r.determineStatus(ctx, rev, reconcileErr)
	if err != nil {
		errs.Add(fmt.Errorf("failed to determine status: %w", err))
	}

	if !reflect.DeepEqual(rev.Status, status) {
		if err := r.Client.Status().Patch(ctx, rev, kube.NewStatusPatch(status)); err != nil {
			errs.Add(fmt.Errorf("failed to patch status: %w", err))
		}
	}
	return errs.Error()
}

func deriveState(conditions ...v1.IstioRevisionCondition) v1.IstioRevisionConditionReason {
	for _, c := range conditions {
		if c.Status != metav1.ConditionTrue {
			return c.Reason
		}
	}
	return v1.IstioRevisionReasonHealthy
}

func (r *Reconciler) determineReconciledCondition(err error) v1.IstioRevisionCondition {
	c := v1.IstioRevisionCondition{Type: v1.IstioRevisionConditionReconciled}

	if err == nil {
		c.Status = metav1.ConditionTrue
	} else if reconciler.IsNameAlreadyExistsError(err) {
		c.Status = metav1.ConditionFalse
		c.Reason = v1.IstioRevisionReasonNameAlreadyExists
		c.Message = err.Error()
	} else {
		c.Status = metav1.ConditionFalse
		c.Reason = v1.IstioRevisionReasonReconcileError
		c.Message = fmt.Sprintf("error reconciling resource: %v", err)
	}
	return c
}

func (r *Reconciler) determineReadyCondition(ctx context.Context, rev *v1.IstioRevision) (v1.IstioRevisionCondition, error) {
	c := v1.IstioRevisionCondition{
		Type:   v1.IstioRevisionConditionReady,
		Status: metav1.ConditionFalse,
	}

	if !revision.IsUsingRemoteControlPlane(rev) {
		istiod := appsv1.Deployment{}
		if err := r.Client.Get(ctx, istiodDeploymentKey(rev), &istiod); err == nil {
			if istiod.Status.Replicas == 0 {
				c.Reason = v1.IstioRevisionReasonIstiodNotReady
				c.Message = "istiod Deployment is scaled to zero replicas"
			} else if istiod.Status.ReadyReplicas < istiod.Status.Replicas {
				c.Reason = v1.IstioRevisionReasonIstiodNotReady
				c.Message = "not all istiod pods are ready"
			} else {
				c.Status = metav1.ConditionTrue
			}
		} else if apierrors.IsNotFound(err) {
			c.Reason = v1.IstioRevisionReasonIstiodNotReady
			c.Message = "istiod Deployment not found"
		} else {
			c.Status = metav1.ConditionUnknown
			c.Reason = v1.IstioRevisionReasonReadinessCheckFailed
			c.Message = fmt.Sprintf("failed to get readiness: %v", err)
			return c, fmt.Errorf("get failed: %w", err)
		}
	} else {
		webhook := admissionv1.MutatingWebhookConfiguration{}
		webhookKey := injectionWebhookKey(rev)
		if err := r.Client.Get(ctx, webhookKey, &webhook); err == nil {
			switch webhook.Annotations[constants.WebhookReadinessProbeStatusAnnotationKey] {
			case "true":
				c.Status = metav1.ConditionTrue
			case "false":
				c.Reason = v1.IstioRevisionReasonRemoteIstiodNotReady
				c.Message = "readiness probe on remote istiod failed"
			default:
				c.Reason = v1.IstioRevisionReasonRemoteIstiodNotReady
				c.Message = fmt.Sprintf("invalid or missing annotation %s on MutatingWebhookConfiguration %s",
					constants.WebhookReadinessProbeStatusAnnotationKey, webhookKey.Name)
			}
		} else if apierrors.IsNotFound(err) {
			c.Reason = v1.IstioRevisionReasonRemoteIstiodNotReady
			c.Message = fmt.Sprintf("MutatingWebhookConfiguration %s not found", webhookKey.Name)
		} else {
			c.Status = metav1.ConditionUnknown
			c.Reason = v1.IstioRevisionReasonReadinessCheckFailed
			c.Message = fmt.Sprintf("failed to get readiness: %v", err)
			return c, fmt.Errorf("get failed: %w", err)
		}
	}
	return c, nil
}

func (r *Reconciler) determineDependenciesHealthyCondition(ctx context.Context, rev *v1.IstioRevision) (v1.IstioRevisionCondition, error) {
	if revision.DependsOnIstioCNI(rev, r.Config) {
		cni := v1.IstioCNI{}
		if err := r.Client.Get(ctx, client.ObjectKey{Name: istioCniName}, &cni); err != nil {
			if apierrors.IsNotFound(err) {
				return v1.IstioRevisionCondition{
					Type:    v1.IstioRevisionConditionDependenciesHealthy,
					Status:  metav1.ConditionFalse,
					Reason:  v1.IstioRevisionReasonIstioCNINotFound,
					Message: "IstioCNI resource does not exist",
				}, nil
			}

			return v1.IstioRevisionCondition{
				Type:    v1.IstioRevisionConditionDependenciesHealthy,
				Status:  metav1.ConditionUnknown,
				Reason:  v1.IstioRevisionDependencyCheckFailed,
				Message: fmt.Sprintf("failed to get IstioCNI status: %v", err),
			}, fmt.Errorf("get failed: %w", err)
		}

		if cni.Status.State != v1.IstioCNIReasonHealthy {
			return v1.IstioRevisionCondition{
				Type:    v1.IstioRevisionConditionDependenciesHealthy,
				Status:  metav1.ConditionFalse,
				Reason:  v1.IstioRevisionReasonIstioCNINotHealthy,
				Message: "IstioCNI resource status indicates that the component is not healthy",
			}, nil
		}
	}

	if revision.DependsOnZTunnel(rev, r.Config) {
		ztunnel := v1.ZTunnel{}
		if err := r.Client.Get(ctx, client.ObjectKey{Name: ztunnelName}, &ztunnel); err != nil {
			if apierrors.IsNotFound(err) {
				return v1.IstioRevisionCondition{
					Type:    v1.IstioRevisionConditionDependenciesHealthy,
					Status:  metav1.ConditionFalse,
					Reason:  v1.IstioRevisionReasonZTunnelNotFound,
					Message: "ZTunnel resource does not exist",
				}, nil
			}

			return v1.IstioRevisionCondition{
				Type:    v1.IstioRevisionConditionDependenciesHealthy,
				Status:  metav1.ConditionUnknown,
				Reason:  v1.IstioRevisionDependencyCheckFailed,
				Message: fmt.Sprintf("failed to get ZTunnel status: %v", err),
			}, fmt.Errorf("get failed: %w", err)
		}

		if ztunnel.Status.State != v1.ZTunnelReasonHealthy {
			return v1.IstioRevisionCondition{
				Type:    v1.IstioRevisionConditionDependenciesHealthy,
				Status:  metav1.ConditionFalse,
				Reason:  v1.IstioRevisionReasonZTunnelNotHealthy,
				Message: "ZTunnel resource status indicates that the component is not healthy",
			}, nil
		}
	}

	return v1.IstioRevisionCondition{
		Type:   v1.IstioRevisionConditionDependenciesHealthy,
		Status: metav1.ConditionTrue,
	}, nil
}

func (r *Reconciler) determineInUseCondition(ctx context.Context, rev *v1.IstioRevision) (v1.IstioRevisionCondition, error) {
	c := v1.IstioRevisionCondition{Type: v1.IstioRevisionConditionInUse}

	isReferenced, err := r.isRevisionReferenced(ctx, rev)
	if err == nil {
		if isReferenced {
			c.Status = metav1.ConditionTrue
			c.Reason = v1.IstioRevisionReasonReferencedByWorkloads
			c.Message = "Referenced by at least one pod or namespace"
		} else {
			c.Status = metav1.ConditionFalse
			c.Reason = v1.IstioRevisionReasonNotReferenced
			c.Message = "Not referenced by any pod or namespace"
		}
		return c, nil
	}
	c.Status = metav1.ConditionUnknown
	c.Reason = v1.IstioRevisionReasonUsageCheckFailed
	c.Message = fmt.Sprintf("failed to determine if revision is in use: %v", err)
	return c, fmt.Errorf("failed to determine if IstioRevision is in use: %w", err)
}

func (r *Reconciler) isRevisionReferenced(ctx context.Context, rev *v1.IstioRevision) (bool, error) {
	log := logf.FromContext(ctx)
	nsList := corev1.NamespaceList{}
	nsMap := map[string]corev1.Namespace{}
	// if an IstioRevision is referenced by a revisionTag, it's considered as InUse
	revisionTagList := v1.IstioRevisionTagList{}
	if err := r.Client.List(ctx, &revisionTagList); err != nil {
		return false, fmt.Errorf("failed to list IstioRevisionTags: %w", err)
	}
	for _, tag := range revisionTagList.Items {
		if tag.Status.IstioRevision == rev.Name {
			log.V(2).Info("Revision is referenced by IstioRevisionTag", "IstioRevisionTag", tag.Name)
			return true, nil
		}
	}

	if err := r.Client.List(ctx, &nsList); err != nil { // TODO: can we optimize this by specifying a label selector
		return false, fmt.Errorf("failed to list namespaces: %w", err)
	}
	for _, ns := range nsList.Items {
		if namespaceReferencesRevision(ns, rev) {
			log.V(2).Info("Revision is referenced by Namespace", "Namespace", ns.Name)
			return true, nil
		}
		nsMap[ns.Name] = ns
	}

	podList := corev1.PodList{}
	if err := r.Client.List(ctx, &podList); err != nil { // TODO: can we optimize this by specifying a label selector
		return false, fmt.Errorf("failed to list pods: %w", err)
	}
	for _, pod := range podList.Items {
		if pod.Status.Phase == corev1.PodSucceeded {
			continue
		}
		if ns, found := nsMap[pod.Namespace]; found && podReferencesRevision(pod, ns, rev) {
			log.V(2).Info("Revision is referenced by Pod", "Pod", client.ObjectKeyFromObject(&pod))
			return true, nil
		}
	}

	if rev.Name == v1.DefaultRevision && rev.Spec.Values != nil &&
		rev.Spec.Values.SidecarInjectorWebhook != nil &&
		rev.Spec.Values.SidecarInjectorWebhook.EnableNamespacesByDefault != nil &&
		*rev.Spec.Values.SidecarInjectorWebhook.EnableNamespacesByDefault {
		return true, nil
	}

	log.V(2).Info("Revision is not referenced by any Pod or Namespace")
	return false, nil
}

func namespaceReferencesRevision(ns corev1.Namespace, rev *v1.IstioRevision) bool {
	return rev.Name == revision.GetReferencedRevisionFromNamespace(ns.Labels)
}

func podReferencesRevision(pod corev1.Pod, ns corev1.Namespace, rev *v1.IstioRevision) bool {
	if rev.Name == revision.GetInjectedRevisionFromPod(pod.GetAnnotations()) {
		return true
	}
	if revision.GetReferencedRevisionFromNamespace(ns.Labels) == "" &&
		rev.Name == revision.GetReferencedRevisionFromPod(pod.GetLabels()) {
		return true
	}
	return false
}

func istiodDeploymentKey(rev *v1.IstioRevision) client.ObjectKey {
	name := "istiod"
	if rev.Spec.Values != nil && rev.Spec.Values.Revision != nil && *rev.Spec.Values.Revision != "" {
		name += "-" + *rev.Spec.Values.Revision
	}

	return client.ObjectKey{
		Namespace: rev.Spec.Namespace,
		Name:      name,
	}
}

func injectionWebhookKey(rev *v1.IstioRevision) client.ObjectKey {
	name := "istio-sidecar-injector"
	if rev.Spec.Values != nil && rev.Spec.Values.Revision != nil && *rev.Spec.Values.Revision != "" {
		name += "-" + *rev.Spec.Values.Revision
	}
	if rev.Spec.Namespace != "istio-system" {
		name += "-" + rev.Spec.Namespace
	}
	return client.ObjectKey{
		Name: name,
	}
}

func (r *Reconciler) mapNamespaceToReconcileRequest(ctx context.Context, ns client.Object) []reconcile.Request {
	log := logf.FromContext(ctx)
	var requests []reconcile.Request

	// Check if any IstioRevision references this namespace in .spec.namespace
	revList := v1.IstioRevisionList{}
	if err := r.Client.List(ctx, &revList); err != nil {
		log.Error(err, "failed to list IstioRevisions")
		return nil
	}
	for _, rev := range revList.Items {
		if rev.Spec.Namespace == ns.GetName() {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: rev.Name}})
		}
	}

	// Check if the namespace references an IstioRevision in its labels
	revisionName := revision.GetReferencedRevisionFromNamespace(ns.GetLabels())
	if revisionName != "" {
		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: revisionName}})
	}
	return requests
}

// mapPodToReconcileRequest will collect all referenced revisions from a pod and trigger reconciliation if there are any
func (r *Reconciler) mapPodToReconcileRequest(ctx context.Context, pod client.Object) []reconcile.Request {
	revisionNames := []string{}
	revisionName := revision.GetInjectedRevisionFromPod(pod.GetAnnotations())
	if revisionName != "" {
		revisionNames = append(revisionNames, revisionName)
	} else {
		revisionName = revision.GetReferencedRevisionFromPod(pod.GetLabels())
		if revisionName != "" {
			revisionNames = append(revisionNames, revisionName)
		}
	}

	if len(revisionNames) > 0 {
		reqs := []reconcile.Request{}
		for _, revName := range revisionNames {
			reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Name: revName}})
		}
		return reqs
	}
	return nil
}

func (r *Reconciler) mapRevisionTagToReconcileRequest(ctx context.Context, revisionTag client.Object) []reconcile.Request {
	tag, ok := revisionTag.(*v1.IstioRevisionTag)
	if ok && tag.Status.IstioRevision != "" {
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: tag.Status.IstioRevision}}}
	}
	return nil
}

// mapIstioCniToReconcileRequests returns reconcile requests for all IstioRevisions that depend on IstioCNI
func (r *Reconciler) mapIstioCniToReconcileRequests(ctx context.Context, _ client.Object) []reconcile.Request {
	list := v1.IstioRevisionList{}
	if err := r.Client.List(ctx, &list); err != nil {
		return nil
	}
	var reqs []reconcile.Request
	for _, rev := range list.Items {
		if revision.DependsOnIstioCNI(&rev, r.Config) {
			reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Name: rev.Name}})
		}
	}
	return reqs
}

// mapZTunnelToReconcileRequests returns reconcile requests for all IstioRevisions that depend on ZTunnel
func (r *Reconciler) mapZTunnelToReconcileRequests(ctx context.Context, _ client.Object) []reconcile.Request {
	list := v1.IstioRevisionList{}
	if err := r.Client.List(ctx, &list); err != nil {
		return nil
	}
	var reqs []reconcile.Request
	for _, rev := range list.Items {
		if revision.DependsOnZTunnel(&rev, r.Config) {
			reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Name: rev.Name}})
		}
	}
	return reqs
}

// ignoreStatusChange returns a predicate that ignores watch events where only the resource status changes; if
// there are any other changes to the resource, the event is not ignored.
// This ensures that the controller doesn't reconcile the entire IstioRevision every time the status of an owned
// resource is updated. Without this predicate, the controller would continuously reconcile the IstioRevision
// because the status.currentMetrics of the HorizontalPodAutoscaler object was updated.
func ignoreStatusChange() predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return specWasUpdated(e.ObjectOld, e.ObjectNew) ||
				!reflect.DeepEqual(e.ObjectNew.GetLabels(), e.ObjectOld.GetLabels()) ||
				!reflect.DeepEqual(e.ObjectNew.GetAnnotations(), e.ObjectOld.GetAnnotations()) ||
				!reflect.DeepEqual(e.ObjectNew.GetOwnerReferences(), e.ObjectOld.GetOwnerReferences()) ||
				!reflect.DeepEqual(e.ObjectNew.GetFinalizers(), e.ObjectOld.GetFinalizers())
		},
	}
}

func specWasUpdated(oldObject client.Object, newObject client.Object) bool {
	// for HPAs, k8s doesn't set metadata.generation, so we actually have to check whether the spec was updated
	if oldHpa, ok := oldObject.(*autoscalingv2.HorizontalPodAutoscaler); ok {
		if newHpa, ok := newObject.(*autoscalingv2.HorizontalPodAutoscaler); ok {
			return !reflect.DeepEqual(oldHpa.Spec, newHpa.Spec)
		}
	}

	// for other resources, comparing the metadata.generation suffices
	return oldObject.GetGeneration() != newObject.GetGeneration()
}

func validatingWebhookConfigPredicate() predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.TypedUpdateEvent[client.Object]) bool {
			if e.ObjectOld == nil || e.ObjectNew == nil {
				return false
			}

			if matched, _ := regexp.MatchString("istiod-.*-validator|istio-validator.*", e.ObjectNew.GetName()); matched {
				// Istiod updates the caBundle and failurePolicy fields in istiod-<ns>-validator and istio-validator[-<rev>]-<ns>
				// webhook configs. We must ignore changes to these fields to prevent an endless update loop.
				clearIgnoredFields(e.ObjectOld)
				clearIgnoredFields(e.ObjectNew)
				return !reflect.DeepEqual(e.ObjectNew, e.ObjectOld)
			}
			return true
		},
	}
}

func clearIgnoredFields(obj client.Object) {
	obj.SetResourceVersion("")
	obj.SetGeneration(0)
	obj.SetManagedFields(nil)
	if webhookConfig, ok := obj.(*admissionv1.ValidatingWebhookConfiguration); ok {
		for i := range len(webhookConfig.Webhooks) {
			webhookConfig.Webhooks[i].FailurePolicy = nil
		}
	}
}

func wrapEventHandler(logger logr.Logger, handler handler.EventHandler) handler.EventHandler {
	return enqueuelogger.WrapIfNecessary(v1.IstioRevisionKind, logger, handler)
}
