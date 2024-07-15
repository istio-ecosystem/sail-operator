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
	"github.com/istio-ecosystem/sail-operator/api/v1alpha1"
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	"github.com/istio-ecosystem/sail-operator/pkg/enqueuelogger"
	"github.com/istio-ecosystem/sail-operator/pkg/errlist"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	"github.com/istio-ecosystem/sail-operator/pkg/reconciler"
	"github.com/istio-ecosystem/sail-operator/pkg/validation"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
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

	networkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	"istio.io/istio/pkg/ptr"
)

const (
	IstioInjectionLabel        = "istio-injection"
	IstioInjectionEnabledValue = "enabled"
	IstioRevLabel              = "istio.io/rev"
	IstioSidecarInjectLabel    = "sidecar.istio.io/inject"

	istiodChartName       = "istiod"
	istiodRemoteChartName = "istiod-remote"
)

// Reconciler reconciles an IstioRevision object
type Reconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	ResourceDirectory string
	ChartManager      *helm.ChartManager
}

func NewReconciler(client client.Client, scheme *runtime.Scheme, resourceDir string, chartManager *helm.ChartManager) *Reconciler {
	return &Reconciler{
		Client:            client,
		Scheme:            scheme,
		ResourceDirectory: resourceDir,
		ChartManager:      chartManager,
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
// +kubebuilder:rbac:groups="apiextensions.k8s.io",resources=customresourcedefinitions,verbs=get;list;watch
// +kubebuilder:rbac:groups="k8s.cni.cncf.io",resources=network-attachment-definitions,verbs="*"
// +kubebuilder:rbac:groups="security.openshift.io",resources=securitycontextconstraints,resourceNames=privileged,verbs=use
// +kubebuilder:rbac:groups="networking.istio.io",resources=envoyfilters,verbs="*"

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *Reconciler) Reconcile(ctx context.Context, rev *v1alpha1.IstioRevision) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	reconcileErr := r.doReconcile(ctx, rev)

	log.Info("Reconciliation done. Updating status.")
	statusErr := r.updateStatus(ctx, rev, reconcileErr)

	return ctrl.Result{}, errors.Join(reconcileErr, statusErr)
}

func (r *Reconciler) doReconcile(ctx context.Context, rev *v1alpha1.IstioRevision) error {
	log := logf.FromContext(ctx)
	if err := r.validate(ctx, rev); err != nil {
		return err
	}

	log.Info("Installing Helm chart")
	return r.installHelmCharts(ctx, rev)
}

func (r *Reconciler) Finalize(ctx context.Context, rev *v1alpha1.IstioRevision) error {
	return r.uninstallHelmCharts(ctx, rev)
}

func (r *Reconciler) validate(ctx context.Context, rev *v1alpha1.IstioRevision) error {
	if rev.Spec.Type == "" {
		return reconciler.NewValidationError("spec.type not set")
	}
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

	if rev.Name == v1alpha1.DefaultRevision && rev.Spec.Values.Revision != "" {
		return reconciler.NewValidationError(fmt.Sprintf("spec.values.revision must be \"\" when IstioRevision name is %s", v1alpha1.DefaultRevision))
	} else if rev.Name != v1alpha1.DefaultRevision && rev.Spec.Values.Revision != rev.Name {
		return reconciler.NewValidationError("spec.values.revision does not match IstioRevision name")
	}

	if rev.Spec.Values.Global == nil || rev.Spec.Values.Global.IstioNamespace != rev.Spec.Namespace {
		return reconciler.NewValidationError("spec.values.global.istioNamespace does not match spec.namespace")
	}
	return nil
}

func (r *Reconciler) installHelmCharts(ctx context.Context, rev *v1alpha1.IstioRevision) error {
	ownerReference := metav1.OwnerReference{
		APIVersion:         v1alpha1.GroupVersion.String(),
		Kind:               v1alpha1.IstioRevisionKind,
		Name:               rev.Name,
		UID:                rev.UID,
		Controller:         ptr.Of(true),
		BlockOwnerDeletion: ptr.Of(true),
	}

	values := helm.FromValues(rev.Spec.Values)
	_, err := r.ChartManager.UpgradeOrInstallChart(ctx, r.getChartDir(rev),
		values, rev.Spec.Namespace, getReleaseName(rev), ownerReference)
	if err != nil {
		return fmt.Errorf("failed to install/update Helm chart %q: %w", getChartName(rev), err)
	}
	return nil
}

func getReleaseName(rev *v1alpha1.IstioRevision) string {
	return fmt.Sprintf("%s-%s", rev.Name, getChartName(rev))
}

func (r *Reconciler) getChartDir(rev *v1alpha1.IstioRevision) string {
	return path.Join(r.ResourceDirectory, rev.Spec.Version, "charts", getChartName(rev))
}

func getChartName(rev *v1alpha1.IstioRevision) string {
	switch rev.Spec.Type {
	case v1alpha1.IstioRevisionTypeLocal:
		return istiodChartName
	case v1alpha1.IstioRevisionTypeRemote:
		return istiodRemoteChartName
	default:
		panic(badIstioRevisionType(rev))
	}
}

func (r *Reconciler) uninstallHelmCharts(ctx context.Context, rev *v1alpha1.IstioRevision) error {
	if _, err := r.ChartManager.UninstallChart(ctx, getReleaseName(rev), rev.Spec.Namespace); err != nil {
		return fmt.Errorf("failed to uninstall Helm chart %q: %w", getChartName(rev), err)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	logger := mgr.GetLogger().WithName("ctrlr").WithName("istiorev")

	// mainObjectHandler handles the IstioRevision watch events
	mainObjectHandler := enqueuelogger.WrapIfNecessary(v1alpha1.IstioRevisionKind, logger, &handler.EnqueueRequestForObject{})

	// ownedResourceHandler handles resources that are owned by the IstioRevision CR
	ownedResourceHandler := enqueuelogger.WrapIfNecessary(v1alpha1.IstioRevisionKind, logger,
		handler.EnqueueRequestForOwner(r.Scheme, r.RESTMapper(), &v1alpha1.IstioRevision{}, handler.OnlyControllerOwner()))

	// nsHandler triggers reconciliation in two cases:
	// - when a namespace that is referenced in IstioRevision.spec.namespace is
	//   created, so that the control plane is installed immediately.
	// - when a namespace that references the IstioRevision CR via the istio.io/rev
	//   or istio-injection labels is updated, so that the InUse condition of
	//   the IstioRevision CR is updated.
	nsHandler := enqueuelogger.WrapIfNecessary(v1alpha1.IstioRevisionKind, logger, handler.EnqueueRequestsFromMapFunc(r.mapNamespaceToReconcileRequest))

	// podHandler handles pods that reference the IstioRevision CR via the istio.io/rev or sidecar.istio.io/inject labels.
	// The handler triggers the reconciliation of the referenced IstioRevision CR so that its InUse condition is updated.
	podHandler := enqueuelogger.WrapIfNecessary(v1alpha1.IstioRevisionKind, logger, handler.EnqueueRequestsFromMapFunc(r.mapPodToReconcileRequest))

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{
			LogConstructor: func(req *reconcile.Request) logr.Logger {
				log := logger
				if req != nil {
					log = log.WithValues("IstioRevision", req.Name)
				}
				return log
			},
		}).
		// we use Watches() instead of For(), so that we can wrap the handler so that events that cause the object to be enqueued are logged
		Watches(&v1alpha1.IstioRevision{}, mainObjectHandler).
		Named("istiorevision").

		// namespaced resources
		Watches(&corev1.ConfigMap{}, ownedResourceHandler).
		Watches(&appsv1.Deployment{}, ownedResourceHandler). // we don't ignore the status here because we use it to calculate the IstioRevision status
		Watches(&appsv1.DaemonSet{}, ownedResourceHandler).  // we don't ignore the status here because we use it to calculate the IstioRevision status
		Watches(&corev1.Endpoints{}, ownedResourceHandler).
		Watches(&corev1.Secret{}, ownedResourceHandler).
		Watches(&corev1.Service{}, ownedResourceHandler, builder.WithPredicates(ignoreStatusChange())).
		Watches(&corev1.ServiceAccount{}, ownedResourceHandler).
		Watches(&rbacv1.Role{}, ownedResourceHandler).
		Watches(&rbacv1.RoleBinding{}, ownedResourceHandler).
		Watches(&policyv1.PodDisruptionBudget{}, ownedResourceHandler, builder.WithPredicates(ignoreStatusChange())).
		Watches(&autoscalingv2.HorizontalPodAutoscaler{}, ownedResourceHandler, builder.WithPredicates(ignoreStatusChange())).
		Watches(&networkingv1alpha3.EnvoyFilter{}, ownedResourceHandler, builder.WithPredicates(ignoreStatusChange())).
		Watches(&corev1.Namespace{}, nsHandler, builder.WithPredicates(ignoreStatusChange())).
		Watches(&corev1.Pod{}, podHandler, builder.WithPredicates(ignoreStatusChange())).

		// cluster-scoped resources
		Watches(&rbacv1.ClusterRole{}, ownedResourceHandler).
		Watches(&rbacv1.ClusterRoleBinding{}, ownedResourceHandler).
		Watches(&admissionv1.MutatingWebhookConfiguration{}, ownedResourceHandler).
		Watches(&admissionv1.ValidatingWebhookConfiguration{}, ownedResourceHandler, builder.WithPredicates(validatingWebhookConfigPredicate())).

		// +lint-watches:ignore: ValidatingAdmissionPolicy (TODO: fix this when CI supports golang 1.22 and k8s 1.30)
		// +lint-watches:ignore: ValidatingAdmissionPolicyBinding (TODO: fix this when CI supports golang 1.22 and k8s 1.30)
		// +lint-watches:ignore: CustomResourceDefinition (prevents `make lint-watches` from bugging us about CRDs)
		Complete(reconciler.NewStandardReconcilerWithFinalizer[*v1alpha1.IstioRevision](r.Client, r.Reconcile, r.Finalize, constants.FinalizerName))
}

func (r *Reconciler) determineStatus(ctx context.Context, rev *v1alpha1.IstioRevision, reconcileErr error) (v1alpha1.IstioRevisionStatus, error) {
	var errs errlist.Builder
	reconciledCondition := r.determineReconciledCondition(reconcileErr)
	readyCondition, err := r.determineReadyCondition(ctx, rev)
	errs.Add(err)

	inUseCondition, err := r.determineInUseCondition(ctx, rev)
	errs.Add(err)

	status := *rev.Status.DeepCopy()
	status.ObservedGeneration = rev.Generation
	status.SetCondition(reconciledCondition)
	status.SetCondition(readyCondition)
	status.SetCondition(inUseCondition)
	status.State = deriveState(reconciledCondition, readyCondition)
	return status, errs.Error()
}

func (r *Reconciler) updateStatus(ctx context.Context, rev *v1alpha1.IstioRevision, reconcileErr error) error {
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

func deriveState(reconciledCondition, readyCondition v1alpha1.IstioRevisionCondition) v1alpha1.IstioRevisionConditionReason {
	if reconciledCondition.Status != metav1.ConditionTrue {
		return reconciledCondition.Reason
	} else if readyCondition.Status != metav1.ConditionTrue {
		return readyCondition.Reason
	}
	return v1alpha1.IstioRevisionReasonHealthy
}

func (r *Reconciler) determineReconciledCondition(err error) v1alpha1.IstioRevisionCondition {
	c := v1alpha1.IstioRevisionCondition{Type: v1alpha1.IstioRevisionConditionReconciled}

	if err == nil {
		c.Status = metav1.ConditionTrue
	} else {
		c.Status = metav1.ConditionFalse
		c.Reason = v1alpha1.IstioRevisionReasonReconcileError
		c.Message = fmt.Sprintf("error reconciling resource: %v", err)
	}
	return c
}

func (r *Reconciler) determineReadyCondition(ctx context.Context, rev *v1alpha1.IstioRevision) (v1alpha1.IstioRevisionCondition, error) {
	c := v1alpha1.IstioRevisionCondition{
		Type:   v1alpha1.IstioRevisionConditionReady,
		Status: metav1.ConditionFalse,
	}

	switch rev.Spec.Type {
	case v1alpha1.IstioRevisionTypeLocal:
		istiod := appsv1.Deployment{}
		if err := r.Client.Get(ctx, istiodDeploymentKey(rev), &istiod); err == nil {
			if istiod.Status.Replicas == 0 {
				c.Reason = v1alpha1.IstioRevisionReasonIstiodNotReady
				c.Message = "istiod Deployment is scaled to zero replicas"
			} else if istiod.Status.ReadyReplicas < istiod.Status.Replicas {
				c.Reason = v1alpha1.IstioRevisionReasonIstiodNotReady
				c.Message = "not all istiod pods are ready"
			} else {
				c.Status = metav1.ConditionTrue
			}
		} else if apierrors.IsNotFound(err) {
			c.Reason = v1alpha1.IstioRevisionReasonIstiodNotReady
			c.Message = "istiod Deployment not found"
		} else {
			c.Status = metav1.ConditionUnknown
			c.Reason = v1alpha1.IstioRevisionReasonReadinessCheckFailed
			c.Message = fmt.Sprintf("failed to get readiness: %v", err)
			return c, fmt.Errorf("get failed: %w", err)
		}
	case v1alpha1.IstioRevisionTypeRemote:
		webhook := admissionv1.MutatingWebhookConfiguration{}
		webhookKey := injectionWebhookKey(rev)
		if err := r.Client.Get(ctx, webhookKey, &webhook); err == nil {
			switch webhook.Annotations[constants.WebhookReadinessProbeStatusAnnotationKey] {
			case "true":
				c.Status = metav1.ConditionTrue
			case "false":
				c.Reason = v1alpha1.IstioRevisionReasonRemoteIstiodNotReady
				c.Message = "readiness probe on remote istiod failed"
			default:
				c.Reason = v1alpha1.IstioRevisionReasonRemoteIstiodNotReady
				c.Message = fmt.Sprintf("invalid or missing annotation %s on MutatingWebhookConfiguration %s",
					constants.WebhookReadinessProbeStatusAnnotationKey, webhookKey.Name)
			}
		} else if apierrors.IsNotFound(err) {
			c.Reason = v1alpha1.IstioRevisionReasonRemoteIstiodNotReady
			c.Message = fmt.Sprintf("MutatingWebhookConfiguration %s not found", webhookKey.Name)
		} else {
			c.Status = metav1.ConditionUnknown
			c.Reason = v1alpha1.IstioRevisionReasonReadinessCheckFailed
			c.Message = fmt.Sprintf("failed to get readiness: %v", err)
			return c, fmt.Errorf("get failed: %w", err)
		}
	default:
		panic(badIstioRevisionType(rev))
	}
	return c, nil
}

func (r *Reconciler) determineInUseCondition(ctx context.Context, rev *v1alpha1.IstioRevision) (v1alpha1.IstioRevisionCondition, error) {
	c := v1alpha1.IstioRevisionCondition{Type: v1alpha1.IstioRevisionConditionInUse}

	isReferenced, err := r.isRevisionReferencedByWorkloads(ctx, rev)
	if err == nil {
		if isReferenced {
			c.Status = metav1.ConditionTrue
			c.Reason = v1alpha1.IstioRevisionReasonReferencedByWorkloads
			c.Message = "Referenced by at least one pod or namespace"
		} else {
			c.Status = metav1.ConditionFalse
			c.Reason = v1alpha1.IstioRevisionReasonNotReferenced
			c.Message = "Not referenced by any pod or namespace"
		}
		return c, nil
	}
	c.Status = metav1.ConditionUnknown
	c.Reason = v1alpha1.IstioRevisionReasonUsageCheckFailed
	c.Message = fmt.Sprintf("failed to determine if revision is in use: %v", err)
	return c, fmt.Errorf("failed to determine if IstioRevision is in use: %w", err)
}

func (r *Reconciler) isRevisionReferencedByWorkloads(ctx context.Context, rev *v1alpha1.IstioRevision) (bool, error) {
	log := logf.FromContext(ctx)
	nsList := corev1.NamespaceList{}
	nsMap := map[string]corev1.Namespace{}
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
		if ns, found := nsMap[pod.Namespace]; found && podReferencesRevision(pod, ns, rev) {
			log.V(2).Info("Revision is referenced by Pod", "Pod", client.ObjectKeyFromObject(&pod))
			return true, nil
		}
	}

	if rev.Name == v1alpha1.DefaultRevision && rev.Spec.Values != nil &&
		rev.Spec.Values.SidecarInjectorWebhook != nil &&
		rev.Spec.Values.SidecarInjectorWebhook.EnableNamespacesByDefault != nil &&
		*rev.Spec.Values.SidecarInjectorWebhook.EnableNamespacesByDefault {
		return true, nil
	}

	log.V(2).Info("Revision is not referenced by any Pod or Namespace")
	return false, nil
}

func namespaceReferencesRevision(ns corev1.Namespace, rev *v1alpha1.IstioRevision) bool {
	return rev.Name == getReferencedRevisionFromNamespace(ns.Labels)
}

func podReferencesRevision(pod corev1.Pod, ns corev1.Namespace, rev *v1alpha1.IstioRevision) bool {
	return rev.Name == getReferencedRevisionFromPod(pod.GetLabels(), pod.GetAnnotations(), ns.GetLabels())
}

func getReferencedRevisionFromNamespace(labels map[string]string) string {
	if labels[IstioInjectionLabel] == IstioInjectionEnabledValue {
		return v1alpha1.DefaultRevision
	}
	revision := labels[IstioRevLabel]
	if revision != "" {
		return revision
	}
	// TODO: if .Values.sidecarInjectorWebhook.enableNamespacesByDefault is true, then all namespaces except system namespaces should use the "default" revision

	return ""
}

func getReferencedRevisionFromPod(podLabels, podAnnotations, nsLabels map[string]string) string {
	// if pod was already injected, the revision that did the injection is specified in the istio.io/rev annotation
	revision := podAnnotations[IstioRevLabel]
	if revision != "" {
		return revision
	}

	// pod is marked for injection by a specific revision, but wasn't injected (e.g. because it was created before the revision was applied)
	revisionFromNamespace := getReferencedRevisionFromNamespace(nsLabels)
	if podLabels[IstioSidecarInjectLabel] != "false" {
		if revisionFromNamespace != "" {
			return revisionFromNamespace
		}
		revisionFromPod := podLabels[IstioRevLabel]
		if revisionFromPod != "" {
			return revisionFromPod
		} else if podLabels[IstioSidecarInjectLabel] == "true" {
			return v1alpha1.DefaultRevision
		}
	}
	return ""
}

func istiodDeploymentKey(rev *v1alpha1.IstioRevision) client.ObjectKey {
	name := "istiod"
	if rev.Spec.Values != nil && rev.Spec.Values.Revision != "" {
		name += "-" + rev.Spec.Values.Revision
	}

	return client.ObjectKey{
		Namespace: rev.Spec.Namespace,
		Name:      name,
	}
}

func injectionWebhookKey(rev *v1alpha1.IstioRevision) client.ObjectKey {
	name := "istio-sidecar-injector"
	if rev.Spec.Values != nil && rev.Spec.Values.Revision != "" {
		name += "-" + rev.Spec.Values.Revision
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
	revList := v1alpha1.IstioRevisionList{}
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
	revision := getReferencedRevisionFromNamespace(ns.GetLabels())
	if revision != "" {
		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: revision}})
	}
	return requests
}

func (r *Reconciler) mapPodToReconcileRequest(ctx context.Context, pod client.Object) []reconcile.Request {
	// TODO: rewrite getReferencedRevisionFromPod to use lazy loading to avoid loading the namespace if the pod references a revision directly
	ns := corev1.Namespace{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: pod.GetNamespace()}, &ns)
	if err != nil {
		return nil
	}

	revision := getReferencedRevisionFromPod(pod.GetLabels(), pod.GetAnnotations(), ns.GetLabels())
	if revision != "" {
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: revision}}}
	}
	return nil
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
		for i := 0; i < len(webhookConfig.Webhooks); i++ {
			webhookConfig.Webhooks[i].FailurePolicy = nil
		}
	}
}

func badIstioRevisionType(rev *v1alpha1.IstioRevision) string {
	return fmt.Sprintf("unknown IstioRevisionType: %s", rev.Spec.Type)
}
