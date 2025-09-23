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

package istiorevisiontag

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
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	"github.com/istio-ecosystem/sail-operator/pkg/reconciler"
	"github.com/istio-ecosystem/sail-operator/pkg/revision"
	"github.com/istio-ecosystem/sail-operator/pkg/validation"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
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
	revisionTagsChartName = "revisiontags"
)

// Reconciler reconciles an IstioRevisionTag object
type Reconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	Config       config.ReconcilerConfig
	ChartManager *helm.ChartManager
}

func NewReconciler(reconcilerCfg config.ReconcilerConfig, client client.Client, scheme *runtime.Scheme, chartManager *helm.ChartManager) *Reconciler {
	return &Reconciler{
		Client:       client,
		Scheme:       scheme,
		Config:       reconcilerCfg,
		ChartManager: chartManager,
	}
}

// +kubebuilder:rbac:groups=sailoperator.io,resources=istiorevisiontags,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sailoperator.io,resources=istiorevisiontags/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sailoperator.io,resources=istiorevisiontags/finalizers,verbs=update
// +kubebuilder:rbac:groups="admissionregistration.k8s.io",resources=mutatingwebhookconfigurations,verbs="*"
// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *Reconciler) Reconcile(ctx context.Context, tag *v1.IstioRevisionTag) (ctrl.Result, error) {
	log := logf.FromContext(ctx).WithValues("IstioRevisionTag", tag.Name)

	rev, reconcileErr := r.doReconcile(ctx, tag)

	log.Info("Reconciliation done. Updating status.")
	statusErr := r.updateStatus(ctx, tag, rev, reconcileErr)

	reconcileErr = errors.Unwrap(reconcileErr)

	return ctrl.Result{}, errors.Join(reconcileErr, statusErr)
}

func (r *Reconciler) doReconcile(ctx context.Context, tag *v1.IstioRevisionTag) (*v1.IstioRevision, error) {
	log := logf.FromContext(ctx).WithValues("IstioRevisionTag", tag.Name)
	if err := r.validate(ctx, tag); err != nil {
		return nil, err
	}

	log.Info("Retrieving referenced IstioRevision for IstioRevisionTag")
	rev, err := r.getIstioRevision(ctx, tag.Spec.TargetRef)
	if rev == nil || err != nil {
		return nil, err
	}

	if revision.IsUsingRemoteControlPlane(rev) {
		return nil, reconciler.NewValidationError("IstioRevisionTag cannot reference a remote IstioRevision")
	}

	// if the IstioRevision's namespace changes, we need to completely reinstall the tag
	if tag.Status.IstiodNamespace != "" && tag.Status.IstiodNamespace != rev.Spec.Namespace {
		if err := r.uninstallHelmCharts(ctx, tag); err != nil {
			return nil, err
		}
	}

	log.Info("Installing Helm chart")
	return rev, r.installHelmCharts(ctx, tag, rev)
}

func (r *Reconciler) Finalize(ctx context.Context, tag *v1.IstioRevisionTag) error {
	return r.uninstallHelmCharts(ctx, tag)
}

func (r *Reconciler) validate(ctx context.Context, tag *v1.IstioRevisionTag) error {
	if tag.Spec.TargetRef.Kind == "" || tag.Spec.TargetRef.Name == "" {
		return reconciler.NewValidationError("spec.targetRef not set")
	}
	rev := v1.IstioRevision{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: tag.Name}, &rev); err == nil {
		if validation.ResourceTakesPrecedence(&rev.ObjectMeta, &tag.ObjectMeta) {
			return reconciler.NewNameAlreadyExistsError("an IstioRevision exists with this name", nil)
		}
	} else if !apierrors.IsNotFound(err) {
		return err
	}
	if tag.Spec.TargetRef.Kind == v1.IstioKind {
		i := v1.Istio{}
		if err := r.Client.Get(ctx, types.NamespacedName{Name: tag.Spec.TargetRef.Name}, &i); err != nil {
			if apierrors.IsNotFound(err) {
				return NewReferenceNotFoundError("referenced Istio resource does not exist", err)
			}
			return reconciler.NewValidationError("failed to get referenced Istio resource: " + err.Error())
		}
	} else if tag.Spec.TargetRef.Kind == v1.IstioRevisionKind {
		if err := r.Client.Get(ctx, types.NamespacedName{Name: tag.Spec.TargetRef.Name}, &rev); err != nil {
			if apierrors.IsNotFound(err) {
				return NewReferenceNotFoundError("referenced IstioRevision resource does not exist", err)
			}
			return reconciler.NewValidationError("failed to get referenced IstioRevision resource: " + err.Error())
		}
	}
	return nil
}

func (r *Reconciler) getIstioRevision(ctx context.Context, ref v1.IstioRevisionTagTargetReference) (*v1.IstioRevision, error) {
	var revisionName string
	if ref.Kind == v1.IstioRevisionKind {
		revisionName = ref.Name
	} else if ref.Kind == v1.IstioKind {
		i := v1.Istio{}
		err := r.Client.Get(ctx, types.NamespacedName{Name: ref.Name}, &i)
		if err != nil {
			return nil, err
		}
		if i.Status.ActiveRevisionName == "" {
			return nil, reconciler.NewTransientError("referenced Istio has no active revision")
		}
		revisionName = i.Status.ActiveRevisionName
	} else {
		return nil, reconciler.NewValidationError("unknown targetRef.kind")
	}

	rev := v1.IstioRevision{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: revisionName}, &rev)
	if err != nil {
		return nil, err
	}
	return &rev, nil
}

func (r *Reconciler) installHelmCharts(ctx context.Context, tag *v1.IstioRevisionTag, rev *v1.IstioRevision) error {
	ownerReference := metav1.OwnerReference{
		APIVersion:         v1.GroupVersion.String(),
		Kind:               v1.IstioRevisionTagKind,
		Name:               tag.Name,
		UID:                tag.UID,
		Controller:         ptr.Of(true),
		BlockOwnerDeletion: ptr.Of(true),
	}

	values := helm.FromValues(rev.Spec.Values)
	if err := values.SetStringSlice("revisionTags", []string{tag.Name}); err != nil {
		return err
	}

	_, err := r.ChartManager.UpgradeOrInstallChart(ctx, r.getChartDir(rev, revisionTagsChartName),
		values, rev.Spec.Namespace, getReleaseName(tag, revisionTagsChartName), &ownerReference)
	if err != nil {
		return fmt.Errorf("failed to install/update Helm chart %q: %w", revisionTagsChartName, err)
	}
	if tag.Name == v1.DefaultRevision {
		_, err := r.ChartManager.UpgradeOrInstallChart(ctx, r.getChartDir(rev, constants.BaseChartName),
			values, r.Config.OperatorNamespace, getReleaseName(tag, constants.BaseChartName), &ownerReference)
		if err != nil {
			return fmt.Errorf("failed to install/update Helm chart %q: %w", constants.BaseChartName, err)
		}
	}
	return nil
}

func getReleaseName(tag *v1.IstioRevisionTag, chartName string) string {
	return fmt.Sprintf("%s-%s", tag.Name, chartName)
}

func (r *Reconciler) getChartDir(tag *v1.IstioRevision, chartName string) string {
	return path.Join(r.Config.ResourceDirectory, tag.Spec.Version, "charts", chartName)
}

func (r *Reconciler) uninstallHelmCharts(ctx context.Context, tag *v1.IstioRevisionTag) error {
	if _, err := r.ChartManager.UninstallChart(ctx, getReleaseName(tag, revisionTagsChartName), tag.Status.IstiodNamespace); err != nil {
		return fmt.Errorf("failed to uninstall Helm chart %q: %w", revisionTagsChartName, err)
	}
	if tag.Name == v1.DefaultRevisionTag {
		_, err := r.ChartManager.UninstallChart(ctx, getReleaseName(tag, constants.BaseChartName), r.Config.OperatorNamespace)
		if err != nil {
			return fmt.Errorf("failed to uninstall Helm chart %q: %w", constants.BaseChartName, err)
		}
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	logger := mgr.GetLogger().WithName("ctrlr").WithName("revtag")

	// mainObjectHandler handles the IstioRevisionTag watch events
	mainObjectHandler := wrapEventHandler(logger, &handler.EnqueueRequestForObject{})

	// ownedResourceHandler handles resources that are owned by the IstioRevisionTag CR
	ownedResourceHandler := wrapEventHandler(
		logger, handler.EnqueueRequestForOwner(r.Scheme, r.RESTMapper(), &v1.IstioRevisionTag{}, handler.OnlyControllerOwner()))

	// operatorResourcesHandler handles watch events from operator CRDs Istio and IstioRevision
	operatorResourcesHandler := wrapEventHandler(logger, handler.EnqueueRequestsFromMapFunc(r.mapOperatorResourceToReconcileRequest))
	// nsHandler triggers reconciliation in two cases:
	// - when a namespace that references the IstioRevisionTag CR via the istio.io/rev
	//   or istio-injection labels is updated, so that the InUse condition of
	//   the IstioRevisionTag CR is updated.
	nsHandler := wrapEventHandler(logger, handler.EnqueueRequestsFromMapFunc(r.mapNamespaceToReconcileRequest))

	// podHandler handles pods that reference the IstioRevisionTag CR via the istio.io/rev or sidecar.istio.io/inject labels.
	// The handler triggers the reconciliation of the referenced IstioRevision CR so that its InUse condition is updated.
	podHandler := wrapEventHandler(logger, handler.EnqueueRequestsFromMapFunc(r.mapPodToReconcileRequest))

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{
			LogConstructor: func(req *reconcile.Request) logr.Logger {
				log := logger
				if req != nil {
					log = log.WithValues("IstioRevisionTag", req.Name)
				}
				return log
			},
			MaxConcurrentReconciles: r.Config.MaxConcurrentReconciles,
		}).
		// we use the Watches function instead of For(), so that we can wrap the handler so that events that cause the object to be enqueued are logged
		Watches(&v1.IstioRevisionTag{}, mainObjectHandler).
		Named("istiorevisiontag").
		// watches related to in-use detection
		Watches(&corev1.Namespace{}, nsHandler, builder.WithPredicates(ignoreStatusChange())).
		Watches(&corev1.Pod{}, podHandler, builder.WithPredicates(ignoreStatusChange())).

		// cluster-scoped resources
		Watches(&v1.Istio{}, operatorResourcesHandler).
		Watches(&v1.IstioRevision{}, operatorResourcesHandler).
		Watches(&admissionv1.MutatingWebhookConfiguration{}, ownedResourceHandler).
		Complete(reconciler.NewStandardReconcilerWithFinalizer[*v1.IstioRevisionTag](r.Client, r.Reconcile, r.Finalize, constants.FinalizerName))
}

func (r *Reconciler) determineStatus(ctx context.Context, tag *v1.IstioRevisionTag,
	rev *v1.IstioRevision, reconcileErr error,
) (v1.IstioRevisionTagStatus, error) {
	var errs errlist.Builder
	reconciledCondition := r.determineReconciledCondition(reconcileErr)

	inUseCondition, err := r.determineInUseCondition(ctx, tag)
	errs.Add(err)

	status := *tag.Status.DeepCopy()
	status.ObservedGeneration = tag.Generation
	if reconciledCondition.Status == metav1.ConditionTrue && rev != nil {
		status.IstiodNamespace = rev.Spec.Namespace
		status.IstioRevision = rev.Name
	}
	status.SetCondition(reconciledCondition)
	status.SetCondition(inUseCondition)
	status.State = deriveState(reconciledCondition, inUseCondition)
	return status, errs.Error()
}

func (r *Reconciler) updateStatus(ctx context.Context, tag *v1.IstioRevisionTag, rev *v1.IstioRevision, reconcileErr error) error {
	var errs errlist.Builder

	status, err := r.determineStatus(ctx, tag, rev, reconcileErr)
	if err != nil {
		errs.Add(fmt.Errorf("failed to determine status: %w", err))
	}

	if !reflect.DeepEqual(tag.Status, status) {
		if err := r.Client.Status().Patch(ctx, tag, kube.NewStatusPatch(status)); err != nil {
			errs.Add(fmt.Errorf("failed to patch status: %w", err))
		}
	}
	return errs.Error()
}

func deriveState(reconciledCondition, inUseCondition v1.IstioRevisionTagCondition) v1.IstioRevisionTagConditionReason {
	if reconciledCondition.Status != metav1.ConditionTrue {
		return reconciledCondition.Reason
	}
	if inUseCondition.Status != metav1.ConditionTrue {
		return inUseCondition.Reason
	}
	return v1.IstioRevisionTagReasonHealthy
}

func (r *Reconciler) determineReconciledCondition(err error) v1.IstioRevisionTagCondition {
	c := v1.IstioRevisionTagCondition{Type: v1.IstioRevisionTagConditionReconciled}

	if err == nil {
		c.Status = metav1.ConditionTrue
	} else if reconciler.IsNameAlreadyExistsError(err) {
		c.Status = metav1.ConditionFalse
		c.Reason = v1.IstioRevisionTagReasonNameAlreadyExists
		c.Message = err.Error()
	} else if IsReferenceNotFoundError(err) {
		c.Status = metav1.ConditionFalse
		c.Reason = v1.IstioRevisionTagReasonReferenceNotFound
		c.Message = err.Error()
	} else {
		c.Status = metav1.ConditionFalse
		c.Reason = v1.IstioRevisionTagReasonReconcileError
		c.Message = fmt.Sprintf("error reconciling resource: %v", err)
	}
	return c
}

func (r *Reconciler) determineInUseCondition(ctx context.Context, tag *v1.IstioRevisionTag) (v1.IstioRevisionTagCondition, error) {
	c := v1.IstioRevisionTagCondition{Type: v1.IstioRevisionTagConditionInUse}

	isReferenced, err := r.isRevisionTagReferencedByWorkloads(ctx, tag)
	if err == nil {
		if isReferenced {
			c.Status = metav1.ConditionTrue
			c.Reason = v1.IstioRevisionTagReasonReferencedByWorkloads
			c.Message = "Referenced by at least one pod or namespace"
		} else {
			c.Status = metav1.ConditionFalse
			c.Reason = v1.IstioRevisionTagReasonNotReferenced
			c.Message = "Not referenced by any pod or namespace"
		}
		return c, nil
	}
	c.Status = metav1.ConditionUnknown
	c.Reason = v1.IstioRevisionTagReasonUsageCheckFailed
	c.Message = fmt.Sprintf("failed to determine if revision tag is in use: %v", err)
	return c, fmt.Errorf("failed to determine if IstioRevisionTag is in use: %w", err)
}

func (r *Reconciler) isRevisionTagReferencedByWorkloads(ctx context.Context, tag *v1.IstioRevisionTag) (bool, error) {
	log := logf.FromContext(ctx)
	nsList := corev1.NamespaceList{}
	nsMap := map[string]corev1.Namespace{}
	if err := r.Client.List(ctx, &nsList); err != nil { // TODO: can we optimize this by specifying a label selector
		return false, fmt.Errorf("failed to list namespaces: %w", err)
	}
	for _, ns := range nsList.Items {
		if namespaceReferencesRevisionTag(ns, tag) {
			log.V(2).Info("RevisionTag is referenced by Namespace", "Namespace", ns.Name)
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
		if ns, found := nsMap[pod.Namespace]; found && podReferencesRevisionTag(pod, tag, ns) {
			log.V(2).Info("RevisionTag is referenced by Pod", "Pod", client.ObjectKeyFromObject(&pod))
			return true, nil
		}
	}

	rev, err := r.getIstioRevision(ctx, tag.Spec.TargetRef)
	if err != nil {
		return false, err
	}

	if tag.Name == v1.DefaultRevision && rev.Spec.Values != nil &&
		rev.Spec.Values.SidecarInjectorWebhook != nil &&
		rev.Spec.Values.SidecarInjectorWebhook.EnableNamespacesByDefault != nil &&
		*rev.Spec.Values.SidecarInjectorWebhook.EnableNamespacesByDefault {
		return true, nil
	}

	log.V(2).Info("RevisionTag is not referenced by any Pod or Namespace")
	return false, nil
}

func namespaceReferencesRevisionTag(ns corev1.Namespace, tag *v1.IstioRevisionTag) bool {
	return tag.Name == revision.GetReferencedRevisionFromNamespace(ns.Labels)
}

func podReferencesRevisionTag(pod corev1.Pod, tag *v1.IstioRevisionTag, ns corev1.Namespace) bool {
	return revision.GetReferencedRevisionFromNamespace(ns.Labels) == "" &&
		tag.Name == revision.GetReferencedRevisionFromPod(pod.GetLabels())
}

func (r *Reconciler) mapNamespaceToReconcileRequest(ctx context.Context, ns client.Object) []reconcile.Request {
	var requests []reconcile.Request

	// Check if the namespace references an IstioRevisionTag in its labels
	tag := revision.GetReferencedRevisionFromNamespace(ns.GetLabels())
	if tag != "" {
		requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: tag}})
	}
	return requests
}

func (r *Reconciler) mapPodToReconcileRequest(ctx context.Context, pod client.Object) []reconcile.Request {
	tag := revision.GetReferencedRevisionFromPod(pod.GetLabels())
	if tag != "" {
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: tag}}}
	}
	return nil
}

func (r *Reconciler) mapOperatorResourceToReconcileRequest(ctx context.Context, obj client.Object) []reconcile.Request {
	var revisionName string
	if i, ok := obj.(*v1.Istio); ok && i.Status.ActiveRevisionName != "" {
		revisionName = i.Status.ActiveRevisionName
	} else if rev, ok := obj.(*v1.IstioRevision); ok {
		revisionName = rev.Name
	} else {
		return nil
	}
	tags := v1.IstioRevisionTagList{}
	err := r.Client.List(ctx, &tags, &client.ListOptions{})
	if err != nil {
		return nil
	}
	requests := []reconcile.Request{}
	for _, tag := range tags.Items {
		if tag.Status.IstioRevision == revisionName {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: tag.Name}})
		}
	}
	return requests
}

// ignoreStatusChange returns a predicate that ignores watch events where only the resource status changes; if
// there are any other changes to the resource, the event is not ignored.
// This ensures that the controller doesn't reconcile the entire IstioRevisionTag every time the status of an owned
// resource is updated. Without this predicate, the controller would continuously reconcile the IstioRevisionTag
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

func wrapEventHandler(logger logr.Logger, handler handler.EventHandler) handler.EventHandler {
	return enqueuelogger.WrapIfNecessary(v1.IstioRevisionTagKind, logger, handler)
}

type ReferenceNotFoundError struct {
	Message       string
	originalError error
}

func (err ReferenceNotFoundError) Error() string {
	return err.Message
}

func (err ReferenceNotFoundError) Unwrap() error {
	return err.originalError
}

func NewReferenceNotFoundError(message string, originalError error) ReferenceNotFoundError {
	return ReferenceNotFoundError{
		Message:       message,
		originalError: originalError,
	}
}

func IsReferenceNotFoundError(err error) bool {
	if _, ok := err.(ReferenceNotFoundError); ok {
		return true
	}
	return false
}
