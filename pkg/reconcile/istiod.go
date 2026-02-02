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

package reconcile

import (
	"context"
	"fmt"
	"path"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
	"github.com/istio-ecosystem/sail-operator/pkg/reconciler"
	"github.com/istio-ecosystem/sail-operator/pkg/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IstiodReconciler handles reconciliation of the istiod component.
type IstiodReconciler struct {
	cfg    Config
	client client.Client
}

// NewIstiodReconciler creates a new IstiodReconciler.
// The client parameter is optional - pass nil when using from the library
// where Kubernetes client operations are not needed.
func NewIstiodReconciler(cfg Config, client client.Client) *IstiodReconciler {
	return &IstiodReconciler{
		cfg:    cfg,
		client: client,
	}
}

// ValidateSpec validates the istiod specification.
// It performs basic validation that doesn't require Kubernetes API access.
func (r *IstiodReconciler) ValidateSpec(version, namespace string, values *v1.Values, revisionName string) error {
	if version == "" {
		return reconciler.NewValidationError("version not set")
	}
	if namespace == "" {
		return reconciler.NewValidationError("namespace not set")
	}
	if values == nil {
		return reconciler.NewValidationError("values not set")
	}

	// Validate revision name consistency
	revName := values.Revision
	if revisionName == v1.DefaultRevision && (revName != nil && *revName != "") {
		return reconciler.NewValidationError(fmt.Sprintf("values.revision must be \"\" when revision name is %s", v1.DefaultRevision))
	} else if revisionName != v1.DefaultRevision && (revName == nil || *revName != revisionName) {
		return reconciler.NewValidationError("values.revision does not match revision name")
	}

	// Validate namespace consistency
	if values.Global == nil || values.Global.IstioNamespace == nil || *values.Global.IstioNamespace != namespace {
		return reconciler.NewValidationError("values.global.istioNamespace does not match namespace")
	}

	return nil
}

// Validate performs full validation including Kubernetes API checks.
// This requires a non-nil client to be set.
func (r *IstiodReconciler) Validate(
	ctx context.Context,
	version, namespace string,
	values *v1.Values,
	revisionName string,
	revisionMeta *metav1.ObjectMeta,
) error {
	// First perform basic validation
	if err := r.ValidateSpec(version, namespace, values, revisionName); err != nil {
		return err
	}

	// Skip Kubernetes checks if no client is available
	if r.client == nil {
		return nil
	}

	// Validate target namespace exists
	if err := validation.ValidateTargetNamespace(ctx, r.client, namespace); err != nil {
		return err
	}

	// Check for name conflicts with IstioRevisionTag (only when revisionMeta is provided)
	if revisionMeta != nil {
		tag := v1.IstioRevisionTag{}
		if err := r.client.Get(ctx, types.NamespacedName{Name: revisionName}, &tag); err == nil {
			if validation.ResourceTakesPrecedence(&tag.ObjectMeta, revisionMeta) {
				return reconciler.NewNameAlreadyExistsError("an IstioRevisionTag exists with this name", nil)
			}
		}
	}

	return nil
}

// Install installs or upgrades the istiod Helm charts.
func (r *IstiodReconciler) Install(
	ctx context.Context,
	version, namespace string,
	values *v1.Values,
	revisionName string,
	ownerRef *metav1.OwnerReference,
) error {
	helmValues := helm.FromValues(values)

	// Install istiod chart
	istiodChartPath := path.Join(version, "charts", constants.IstiodChartName)
	istiodReleaseName := GetReleaseName(revisionName, constants.IstiodChartName)

	_, err := r.cfg.ChartManager.UpgradeOrInstallChart(
		ctx,
		r.cfg.ResourceFS,
		istiodChartPath,
		helmValues,
		namespace,
		istiodReleaseName,
		ownerRef,
	)
	if err != nil {
		return fmt.Errorf("failed to install/update Helm chart %q: %w", constants.IstiodChartName, err)
	}

	// Install base chart for default revision
	if revisionName == v1.DefaultRevision {
		baseChartPath := path.Join(version, "charts", constants.BaseChartName)
		baseReleaseName := GetReleaseName(revisionName, constants.BaseChartName)

		_, err := r.cfg.ChartManager.UpgradeOrInstallChart(
			ctx,
			r.cfg.ResourceFS,
			baseChartPath,
			helmValues,
			r.cfg.OperatorNamespace,
			baseReleaseName,
			ownerRef,
		)
		if err != nil {
			return fmt.Errorf("failed to install/update Helm chart %q: %w", constants.BaseChartName, err)
		}
	}

	return nil
}

// Uninstall removes the istiod Helm charts.
func (r *IstiodReconciler) Uninstall(ctx context.Context, namespace, revisionName string) error {
	// Uninstall istiod chart
	istiodReleaseName := GetReleaseName(revisionName, constants.IstiodChartName)
	if _, err := r.cfg.ChartManager.UninstallChart(ctx, istiodReleaseName, namespace); err != nil {
		return fmt.Errorf("failed to uninstall Helm chart %q: %w", constants.IstiodChartName, err)
	}

	// Uninstall base chart for default revision
	if revisionName == v1.DefaultRevision {
		baseReleaseName := GetReleaseName(revisionName, constants.BaseChartName)
		if _, err := r.cfg.ChartManager.UninstallChart(ctx, baseReleaseName, r.cfg.OperatorNamespace); err != nil {
			return fmt.Errorf("failed to uninstall Helm chart %q: %w", constants.BaseChartName, err)
		}
	}

	return nil
}

// GetReleaseName returns the Helm release name for a given revision and chart.
func GetReleaseName(revisionName, chartName string) string {
	return fmt.Sprintf("%s-%s", revisionName, chartName)
}

// GetChartPath returns the path to a chart for a given version.
func GetChartPath(version, chartName string) string {
	return path.Join(version, "charts", chartName)
}
