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

package istiobase

import (
	"context"
	"fmt"
	"path"
	"reflect"

	"github.com/go-logr/logr"
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	"github.com/istio-ecosystem/sail-operator/pkg/enqueuelogger"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	predicate2 "github.com/istio-ecosystem/sail-operator/pkg/predicate"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
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
)

const (
	baseReleaseName = "istio-base"
	baseChartName   = "base"

	webhookName = "istiod-default-validator"
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

// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingadmissionpolicies,verbs="*"
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingadmissionpolicybindings,verbs="*"

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *Reconciler) Reconcile(ctx context.Context, _ reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	reconcileErr := r.doReconcile(ctx)
	log.Info("Reconciliation done.")

	return ctrl.Result{}, reconcileErr
}

func (r *Reconciler) doReconcile(ctx context.Context) error {
	log := logf.FromContext(ctx)

	rev, err := r.getDefaultRevision(ctx)
	if err != nil {
		if errors.IsNotFound(err) {
			if err := r.uninstallHelmChart(ctx); err != nil {
				return fmt.Errorf("failed to uninstall base chart: %w", err)
			}
			return nil
		}
		return fmt.Errorf("failed to reconcile: %w", err)
	}
	log.Info("Installing Helm chart")
	return r.installHelmChart(ctx, rev)
}

func (r *Reconciler) getDefaultRevision(ctx context.Context) (*v1.IstioRevision, error) {
	// 1. get the IstioRevision referenced in the default IstioRevisionTag
	tag := v1.IstioRevisionTag{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: "default"}, &tag); err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get default IstioRevisionTag: %w", err)
		}
	}

	revName := tag.Status.IstioRevision
	if revName == "" {
		revName = "default"
	}

	// 2. get the IstioRevision
	rev := &v1.IstioRevision{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: revName}, rev); err != nil {
		return rev, fmt.Errorf("failed to get IstioRevision %q: %w", revName, err)
	}
	return rev, nil
}

func (r *Reconciler) installHelmChart(ctx context.Context, rev *v1.IstioRevision) error {
	version, err := istioversion.Resolve(rev.Spec.Version)
	if err != nil {
		return fmt.Errorf("failed to resolve IstioRevision version for %q: %w", rev.Name, err)
	}

	values := helm.FromValues(rev.Spec.Values)
	if err := values.Set("defaultRevision", rev.Name); err != nil {
		return fmt.Errorf("failed to set defaultRevision in Helm values: %w", err)
	}

	_, err = r.ChartManager.UpgradeOrInstallChart(ctx, r.getChartDir(version), values, rev.Spec.Namespace, baseReleaseName, nil)
	if err != nil {
		return fmt.Errorf("failed to install/update Helm chart %q: %w", baseChartName, err)
	}
	return nil
}

func (r *Reconciler) getChartDir(version string) string {
	return path.Join(r.Config.ResourceDirectory, version, "charts", baseChartName)
}

func (r *Reconciler) uninstallHelmChart(ctx context.Context) error {
	log := logf.FromContext(ctx)
	releases, err := r.ChartManager.ListReleases(ctx)
	if err != nil {
		return fmt.Errorf("failed to check whether Helm chart %q is installed: %w", baseChartName, err)
	}

	namespace := ""
	for _, release := range releases {
		if release.Name == baseReleaseName {
			namespace = release.Namespace
			break
		}
	}

	if namespace == "" {
		log.V(3).Info(fmt.Sprintf("Helm release %q not found", baseReleaseName))
		return nil
	}

	log.V(3).Info(fmt.Sprintf("Helm release %q found in namespace %q", baseReleaseName, namespace))
	log.Info("Uninstalling Helm chart")
	_, err = r.ChartManager.UninstallChart(ctx, baseReleaseName, namespace)
	if err != nil {
		return fmt.Errorf("failed to uninstall Helm chart %q: %w", baseChartName, err)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	logger := mgr.GetLogger().WithName("ctrlr").WithName("istiobase")

	enqueueSingleton := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []reconcile.Request {
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: "default"}}}
	})

	eventHandler := wrapEventHandler(logger, enqueueSingleton)

	managedByPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetLabels()[constants.ManagedByLabelKey] == constants.ManagedByLabelValue
	})

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{
			LogConstructor: func(_ *reconcile.Request) logr.Logger {
				log := logger
				return log
			},
		}).
		Named("istiobase").

		// We use predicate.IgnoreUpdate() so that we skip the reconciliation when a pull secret is added to the ServiceAccount.
		// This is necessary so that we don't remove the newly-added secret.
		// TODO: this is a temporary hack until we implement the correct solution on the Helm-render side
		Watches(&corev1.ServiceAccount{}, eventHandler, builder.WithPredicates(predicate2.IgnoreUpdate(), managedByPredicate)).
		Watches(&admissionv1.ValidatingWebhookConfiguration{}, eventHandler, builder.WithPredicates(validatingWebhookConfigPredicate(), managedByPredicate)).
		Watches(&admissionv1.ValidatingAdmissionPolicy{}, eventHandler, builder.WithPredicates(managedByPredicate)).
		Watches(&admissionv1.ValidatingAdmissionPolicyBinding{}, eventHandler, builder.WithPredicates(managedByPredicate)).
		Watches(&v1.IstioRevision{}, eventHandler).
		Watches(&v1.IstioRevisionTag{}, eventHandler).
		Complete(reconcile.Func(r.Reconcile))
}

func wrapEventHandler(logger logr.Logger, handler handler.EventHandler) handler.EventHandler {
	return enqueuelogger.WrapIfNecessary("IstioBase", logger, handler)
}

func validatingWebhookConfigPredicate() predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.TypedUpdateEvent[client.Object]) bool {
			if e.ObjectOld == nil || e.ObjectNew == nil {
				return false
			}

			if e.ObjectNew.GetName() == webhookName {
				// Istiod updates the caBundle and failurePolicy fields. We must ignore changes to these fields to prevent an endless update loop.
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
