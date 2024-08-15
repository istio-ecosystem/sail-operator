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
	"github.com/istio-ecosystem/sail-operator/api/v1alpha1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	"github.com/istio-ecosystem/sail-operator/pkg/errlist"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
	"github.com/istio-ecosystem/sail-operator/pkg/istiovalues"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
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
	ResourceDirectory string
	DefaultProfile    string
	client.Client
	Scheme       *runtime.Scheme
	ChartManager *helm.ChartManager
}

func NewReconciler(
	client client.Client, scheme *runtime.Scheme, resourceDir string, chartManager *helm.ChartManager, defaultProfile string,
) *Reconciler {
	return &Reconciler{
		ResourceDirectory: resourceDir,
		DefaultProfile:    defaultProfile,
		Client:            client,
		Scheme:            scheme,
		ChartManager:      chartManager,
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
func (r *Reconciler) Reconcile(ctx context.Context, cni *v1alpha1.IstioCNI) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	reconcileErr := r.doReconcile(ctx, cni)

	log.Info("Reconciliation done. Updating status.")
	statusErr := r.updateStatus(ctx, cni, reconcileErr)

	return ctrl.Result{}, errors.Join(reconcileErr, statusErr)
}

func (r *Reconciler) Finalize(ctx context.Context, cni *v1alpha1.IstioCNI) error {
	return r.uninstallHelmChart(ctx, cni)
}

func (r *Reconciler) doReconcile(ctx context.Context, cni *v1alpha1.IstioCNI) error {
	log := logf.FromContext(ctx)
	if err := r.validate(ctx, cni); err != nil {
		return err
	}

	log.Info("Installing Helm chart")
	return r.installHelmChart(ctx, cni)
}

func (r *Reconciler) validate(ctx context.Context, cni *v1alpha1.IstioCNI) error {
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

func (r *Reconciler) installHelmChart(ctx context.Context, cni *v1alpha1.IstioCNI) error {
	ownerReference := metav1.OwnerReference{
		APIVersion:         v1alpha1.GroupVersion.String(),
		Kind:               v1alpha1.IstioCNIKind,
		Name:               cni.Name,
		UID:                cni.UID,
		Controller:         ptr.Of(true),
		BlockOwnerDeletion: ptr.Of(true),
	}

	// get userValues from Istio.spec.values
	userValues := cni.Spec.Values

	// apply image digests from configuration, if not already set by user
	userValues = applyImageDigests(cni, userValues, config.Config)

	// apply userValues on top of defaultValues from profiles
	mergedHelmValues, err := istiovalues.ApplyProfiles(r.ResourceDirectory, cni.Spec.Version, r.DefaultProfile, cni.Spec.Profile, helm.FromValues(userValues))
	if err != nil {
		return fmt.Errorf("failed to apply profile: %w", err)
	}

	_, err = r.ChartManager.UpgradeOrInstallChart(ctx, r.getChartDir(cni), mergedHelmValues, cni.Spec.Namespace, cniReleaseName, ownerReference)
	if err != nil {
		return fmt.Errorf("failed to install/update Helm chart %q: %w", cniChartName, err)
	}
	return nil
}

func (r *Reconciler) getChartDir(cni *v1alpha1.IstioCNI) string {
	return path.Join(r.ResourceDirectory, cni.Spec.Version, "charts", cniChartName)
}

func applyImageDigests(cni *v1alpha1.IstioCNI, values *v1alpha1.CNIValues, config config.OperatorConfig) *v1alpha1.CNIValues {
	imageDigests, digestsDefined := config.ImageDigests[cni.Spec.Version]
	// if we don't have default image digests defined for this version, it's a no-op
	if !digestsDefined {
		return values
	}

	if values == nil {
		values = &v1alpha1.CNIValues{}
	}

	// set image digest unless any part of the image has been configured by the user
	if values.Cni == nil {
		values.Cni = &v1alpha1.CNIConfig{}
	}
	if values.Cni.Image == "" && values.Cni.Hub == "" && values.Cni.Tag == "" {
		values.Cni.Image = imageDigests.CNIImage
	}
	return values
}

func (r *Reconciler) uninstallHelmChart(ctx context.Context, cni *v1alpha1.IstioCNI) error {
	_, err := r.ChartManager.UninstallChart(ctx, cniReleaseName, cni.Spec.Namespace)
	if err != nil {
		return fmt.Errorf("failed to uninstall Helm chart %q: %w", cniChartName, err)
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	// ownedResourceHandler handles resources that are owned by the IstioCNI CR
	ownedResourceHandler := handler.EnqueueRequestForOwner(r.Scheme, r.RESTMapper(), &v1alpha1.IstioCNI{}, handler.OnlyControllerOwner())

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{
			LogConstructor: func(req *reconcile.Request) logr.Logger {
				log := mgr.GetLogger().WithName("ctrlr").WithName("istiocni")
				if req != nil {
					log = log.WithValues("IstioCNI", req.Name)
				}
				return log
			},
		}).
		For(&v1alpha1.IstioCNI{}).

		// namespaced resources
		Watches(&corev1.ConfigMap{}, ownedResourceHandler).
		Watches(&appsv1.DaemonSet{}, ownedResourceHandler).
		Watches(&corev1.ResourceQuota{}, ownedResourceHandler).
		Watches(&corev1.ServiceAccount{}, ownedResourceHandler).
		Watches(&rbacv1.RoleBinding{}, ownedResourceHandler).

		// TODO: only register NetAttachDef if the CRD is installed (may also need to watch for CRD creation)
		// Owns(&multusv1.NetworkAttachmentDefinition{}).

		// cluster-scoped resources
		Watches(&corev1.Namespace{}, handler.EnqueueRequestsFromMapFunc(r.mapNamespaceToReconcileRequest)).
		Watches(&rbacv1.ClusterRole{}, ownedResourceHandler).
		Watches(&rbacv1.ClusterRoleBinding{}, ownedResourceHandler).
		Complete(reconciler.NewStandardReconcilerWithFinalizer[*v1alpha1.IstioCNI](r.Client, r.Reconcile, r.Finalize, constants.FinalizerName))
}

func (r *Reconciler) determineStatus(ctx context.Context, cni *v1alpha1.IstioCNI, reconcileErr error) (v1alpha1.IstioCNIStatus, error) {
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

func (r *Reconciler) updateStatus(ctx context.Context, cni *v1alpha1.IstioCNI, reconcileErr error) error {
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

func deriveState(reconciledCondition, readyCondition v1alpha1.IstioCNICondition) v1alpha1.IstioCNIConditionReason {
	if reconciledCondition.Status != metav1.ConditionTrue {
		return reconciledCondition.Reason
	} else if readyCondition.Status != metav1.ConditionTrue {
		return readyCondition.Reason
	}
	return v1alpha1.IstioCNIReasonHealthy
}

func (r *Reconciler) determineReconciledCondition(err error) v1alpha1.IstioCNICondition {
	c := v1alpha1.IstioCNICondition{Type: v1alpha1.IstioCNIConditionReconciled}

	if err == nil {
		c.Status = metav1.ConditionTrue
	} else {
		c.Status = metav1.ConditionFalse
		c.Reason = v1alpha1.IstioCNIReasonReconcileError
		c.Message = fmt.Sprintf("error reconciling resource: %v", err)
	}
	return c
}

func (r *Reconciler) determineReadyCondition(ctx context.Context, cni *v1alpha1.IstioCNI) (v1alpha1.IstioCNICondition, error) {
	c := v1alpha1.IstioCNICondition{
		Type:   v1alpha1.IstioCNIConditionReady,
		Status: metav1.ConditionFalse,
	}

	ds := appsv1.DaemonSet{}
	if err := r.Client.Get(ctx, r.cniDaemonSetKey(cni), &ds); err == nil {
		if ds.Status.CurrentNumberScheduled == 0 {
			c.Reason = v1alpha1.IstioCNIDaemonSetNotReady
			c.Message = "no istio-cni-node pods are currently scheduled"
		} else if ds.Status.NumberReady < ds.Status.CurrentNumberScheduled {
			c.Reason = v1alpha1.IstioCNIDaemonSetNotReady
			c.Message = "not all istio-cni-node pods are ready"
		} else {
			c.Status = metav1.ConditionTrue
		}
	} else if apierrors.IsNotFound(err) {
		c.Reason = v1alpha1.IstioCNIDaemonSetNotReady
		c.Message = "istio-cni-node DaemonSet not found"
	} else {
		c.Status = metav1.ConditionUnknown
		c.Reason = v1alpha1.IstioCNIReasonReadinessCheckFailed
		c.Message = fmt.Sprintf("failed to get readiness: %v", err)
		return c, fmt.Errorf("get failed: %w", err)
	}
	return c, nil
}

func (r *Reconciler) cniDaemonSetKey(cni *v1alpha1.IstioCNI) client.ObjectKey {
	return client.ObjectKey{
		Namespace: cni.Spec.Namespace,
		Name:      "istio-cni-node",
	}
}

func (r *Reconciler) mapNamespaceToReconcileRequest(ctx context.Context, ns client.Object) []reconcile.Request {
	log := logf.FromContext(ctx)

	// Check if any IstioCNI references this namespace in .spec.namespace
	cniList := v1alpha1.IstioCNIList{}
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
