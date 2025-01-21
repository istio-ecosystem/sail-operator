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

package istio

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/go-logr/logr"
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/enqueuelogger"
	"github.com/istio-ecosystem/sail-operator/pkg/errlist"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversions"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	"github.com/istio-ecosystem/sail-operator/pkg/reconciler"
	"github.com/istio-ecosystem/sail-operator/pkg/revision"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"istio.io/istio/pkg/ptr"
)

// Reconciler reconciles an Istio object
type Reconciler struct {
	Config config.ReconcilerConfig
	client.Client
	Scheme *runtime.Scheme
}

func NewReconciler(cfg config.ReconcilerConfig, client client.Client, scheme *runtime.Scheme) *Reconciler {
	return &Reconciler{
		Config: cfg,
		Client: client,
		Scheme: scheme,
	}
}

// +kubebuilder:rbac:groups=sailoperator.io,resources=istios,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sailoperator.io,resources=istios/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sailoperator.io,resources=istios/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *Reconciler) Reconcile(ctx context.Context, istio *v1.Istio) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	log.Info("Reconciling")
	result, reconcileErr := r.doReconcile(ctx, istio)

	log.Info("Reconciliation done. Updating status.")
	statusErr := r.updateStatus(ctx, istio, reconcileErr)

	return result, errors.Join(reconcileErr, statusErr)
}

// doReconcile is the function that actually reconciles the Istio object. Any error reported by this
// function should get reported in the status of the Istio object by the caller.
func (r *Reconciler) doReconcile(ctx context.Context, istio *v1.Istio) (result ctrl.Result, err error) {
	if err := validate(istio); err != nil {
		return ctrl.Result{}, err
	}

	if err = r.reconcileActiveRevision(ctx, istio); err != nil {
		return ctrl.Result{}, err
	}

	return revision.PruneInactive(ctx, r.Client, istio.UID, getActiveRevisionName(istio), getPruningGracePeriod(istio))
}

func validate(istio *v1.Istio) error {
	if istio.Spec.Version == "" {
		return reconciler.NewValidationError("spec.version not set")
	}
	if istio.Spec.Namespace == "" {
		return reconciler.NewValidationError("spec.namespace not set")
	}
	return nil
}

func (r *Reconciler) reconcileActiveRevision(ctx context.Context, istio *v1.Istio) error {
	version, ok := istioversions.Map[istio.Spec.Version]
	if !ok {
		return fmt.Errorf("version %q not found", istio.Spec.Version)
	}
	versionName := version.Name
	values, err := revision.ComputeValues(
		istio.Spec.Values, istio.Spec.Namespace, versionName,
		r.Config.Platform, r.Config.DefaultProfile, istio.Spec.Profile,
		r.Config.ResourceDirectory, getActiveRevisionName(istio))
	if err != nil {
		return err
	}

	return revision.CreateOrUpdate(ctx, r.Client,
		getActiveRevisionName(istio),
		version.Version.String(), istio.Spec.Namespace, values,
		metav1.OwnerReference{
			APIVersion:         v1.GroupVersion.String(),
			Kind:               v1.IstioKind,
			Name:               istio.Name,
			UID:                istio.UID,
			Controller:         ptr.Of(true),
			BlockOwnerDeletion: ptr.Of(true),
		})
}

func getPruningGracePeriod(istio *v1.Istio) time.Duration {
	strategy := istio.Spec.UpdateStrategy
	period := int64(v1.DefaultRevisionDeletionGracePeriodSeconds)
	if strategy != nil && strategy.InactiveRevisionDeletionGracePeriodSeconds != nil {
		period = *strategy.InactiveRevisionDeletionGracePeriodSeconds
	}
	if period < v1.MinRevisionDeletionGracePeriodSeconds {
		period = v1.MinRevisionDeletionGracePeriodSeconds
	}
	return time.Duration(period) * time.Second
}

func (r *Reconciler) getActiveRevision(ctx context.Context, istio *v1.Istio) (v1.IstioRevision, error) {
	rev := v1.IstioRevision{}
	err := r.Client.Get(ctx, GetActiveRevisionKey(istio), &rev)
	if err != nil {
		return rev, fmt.Errorf("get failed: %w", err)
	}
	return rev, nil
}

func GetActiveRevisionKey(istio *v1.Istio) types.NamespacedName {
	return types.NamespacedName{
		Name: getActiveRevisionName(istio),
	}
}

func getActiveRevisionName(istio *v1.Istio) string {
	var strategy v1.UpdateStrategyType
	if istio.Spec.UpdateStrategy != nil {
		strategy = istio.Spec.UpdateStrategy.Type
	}

	switch strategy {
	default:
		fallthrough
	case v1.UpdateStrategyTypeInPlace:
		return istio.Name
	case v1.UpdateStrategyTypeRevisionBased:
		return istio.Name + "-" + strings.ReplaceAll(istio.Spec.Version, ".", "-")
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	logger := mgr.GetLogger().WithName("ctrlr").WithName("istio")

	// mainObjectHandler handles the IstioRevision watch events
	mainObjectHandler := wrapEventHandler(logger, &handler.EnqueueRequestForObject{})

	// ownedResourceHandler handles resources that are owned by the IstioRevision CR
	ownedResourceHandler := wrapEventHandler(logger,
		handler.EnqueueRequestForOwner(r.Scheme, r.RESTMapper(), &v1.Istio{}, handler.OnlyControllerOwner()))

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{
			LogConstructor: func(req *reconcile.Request) logr.Logger {
				log := logger
				if req != nil {
					log = log.WithValues("Istio", req.Name)
				}
				return log
			},
		}).
		// we use the Watches function instead of For(), so that we can wrap the handler so that events that cause the object to be enqueued are logged
		// +lint-watches:ignore: Istio (not found in charts, but this is the main resource watched by this controller)
		Watches(&v1.Istio{}, mainObjectHandler).
		Named("istio").
		Watches(&v1.IstioRevision{}, ownedResourceHandler).
		Complete(reconciler.NewStandardReconciler[*v1.Istio](r.Client, r.Reconcile))
}

func (r *Reconciler) determineStatus(ctx context.Context, istio *v1.Istio, reconcileErr error) (v1.IstioStatus, error) {
	var errs errlist.Builder
	status := *istio.Status.DeepCopy()
	status.ObservedGeneration = istio.Generation

	// set Reconciled and Ready conditions
	if reconcileErr != nil {
		status.SetCondition(v1.IstioCondition{
			Type:    v1.IstioConditionReconciled,
			Status:  metav1.ConditionFalse,
			Reason:  v1.IstioReasonReconcileError,
			Message: reconcileErr.Error(),
		})
		status.SetCondition(v1.IstioCondition{
			Type:    v1.IstioConditionReady,
			Status:  metav1.ConditionUnknown,
			Reason:  v1.IstioReasonReconcileError,
			Message: "cannot determine readiness due to reconciliation error",
		})
		status.State = v1.IstioReasonReconcileError
	} else {
		status.ActiveRevisionName = getActiveRevisionName(istio)
		rev, err := r.getActiveRevision(ctx, istio)
		if apierrors.IsNotFound(err) {
			revisionNotFound := func(conditionType v1.IstioConditionType) v1.IstioCondition {
				return v1.IstioCondition{
					Type:    conditionType,
					Status:  metav1.ConditionFalse,
					Reason:  v1.IstioReasonRevisionNotFound,
					Message: "active IstioRevision not found",
				}
			}

			status.SetCondition(revisionNotFound(v1.IstioConditionReconciled))
			status.SetCondition(revisionNotFound(v1.IstioConditionReady))
			status.State = v1.IstioReasonRevisionNotFound
		} else if err == nil {
			status.SetCondition(convertCondition(rev.Status.GetCondition(v1.IstioRevisionConditionReconciled)))
			status.SetCondition(convertCondition(rev.Status.GetCondition(v1.IstioRevisionConditionReady)))
			status.State = convertConditionReason(rev.Status.State)
		} else {
			activeRevisionGetFailed := func(conditionType v1.IstioConditionType) v1.IstioCondition {
				return v1.IstioCondition{
					Type:    conditionType,
					Status:  metav1.ConditionUnknown,
					Reason:  v1.IstioReasonFailedToGetActiveRevision,
					Message: fmt.Sprintf("failed to get active IstioRevision: %s", err),
				}
			}
			status.SetCondition(activeRevisionGetFailed(v1.IstioConditionReconciled))
			status.SetCondition(activeRevisionGetFailed(v1.IstioConditionReady))
			status.State = v1.IstioReasonFailedToGetActiveRevision
			errs.Add(fmt.Errorf("failed to get active IstioRevision: %w", err))
		}
	}

	// count the ready, in-use, and total revisions
	if revs, err := revision.ListOwned(ctx, r.Client, istio.UID); err == nil {
		status.Revisions.Total = int32(len(revs))
		status.Revisions.Ready = 0
		status.Revisions.InUse = 0
		for _, rev := range revs {
			if rev.Status.GetCondition(v1.IstioRevisionConditionReady).Status == metav1.ConditionTrue {
				status.Revisions.Ready++
			}
			if rev.Status.GetCondition(v1.IstioRevisionConditionInUse).Status == metav1.ConditionTrue {
				status.Revisions.InUse++
			}
		}
	} else {
		status.Revisions.Total = -1
		status.Revisions.Ready = -1
		status.Revisions.InUse = -1
		errs.Add(err)
	}
	return status, errs.Error()
}

func (r *Reconciler) updateStatus(ctx context.Context, istio *v1.Istio, reconcileErr error) error {
	var errs errlist.Builder
	status, err := r.determineStatus(ctx, istio, reconcileErr)
	if err != nil {
		errs.Add(fmt.Errorf("failed to determine status: %w", err))
	}

	if !reflect.DeepEqual(istio.Status, status) {
		if err := r.Client.Status().Patch(ctx, istio, kube.NewStatusPatch(status)); err != nil {
			errs.Add(fmt.Errorf("failed to patch status: %w", err))
		}
	}
	return errs.Error()
}

func convertCondition(condition v1.IstioRevisionCondition) v1.IstioCondition {
	return v1.IstioCondition{
		Type:    convertConditionType(condition),
		Status:  condition.Status,
		Reason:  convertConditionReason(condition.Reason),
		Message: condition.Message,
	}
}

func convertConditionType(condition v1.IstioRevisionCondition) v1.IstioConditionType {
	switch condition.Type {
	case v1.IstioRevisionConditionReconciled:
		return v1.IstioConditionReconciled
	case v1.IstioRevisionConditionReady:
		return v1.IstioConditionReady
	default:
		panic(fmt.Sprintf("can't convert IstioRevisionConditionType: %s", condition.Type))
	}
}

func convertConditionReason(reason v1.IstioRevisionConditionReason) v1.IstioConditionReason {
	switch reason {
	case "":
		return ""
	case v1.IstioRevisionReasonIstiodNotReady:
		return v1.IstioReasonIstiodNotReady
	case v1.IstioRevisionReasonHealthy:
		return v1.IstioReasonHealthy
	case v1.IstioRevisionReasonReadinessCheckFailed:
		return v1.IstioReasonReadinessCheckFailed
	case v1.IstioRevisionReasonReconcileError:
		return v1.IstioReasonReconcileError
	case v1.IstioRevisionReasonRemoteIstiodNotReady:
		return v1.IstioReasonRemoteIstiodNotReady
	default:
		panic(fmt.Sprintf("can't convert IstioRevisionConditionReason: %s", reason))
	}
}

func wrapEventHandler(logger logr.Logger, handler handler.EventHandler) handler.EventHandler {
	return enqueuelogger.WrapIfNecessary(v1.IstioKind, logger, handler)
}
