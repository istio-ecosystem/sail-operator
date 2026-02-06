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

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
	"github.com/istio-ecosystem/sail-operator/pkg/reconciler"
	"github.com/istio-ecosystem/sail-operator/pkg/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IstiodReconciler handles reconciliation of the istiod component.
type IstiodReconciler struct {
	cfg    Config
	client client.Client
}

// NewIstiodReconciler creates a new IstiodReconciler.
func NewIstiodReconciler(cfg Config, client client.Client) *IstiodReconciler {
	return &IstiodReconciler{
		cfg:    cfg,
		client: client,
	}
}

// Validate performs general validation of the istiod specification.
// This includes basic field validation and Kubernetes API checks (namespace exists).
// CRD-specific validations (revision name consistency, IstioRevisionTag conflicts)
// should be performed by the controller before calling this method.
func (r *IstiodReconciler) Validate(ctx context.Context, version, namespace string, values *v1.Values) error {
	if version == "" {
		return reconciler.NewValidationError("version not set")
	}
	if namespace == "" {
		return reconciler.NewValidationError("namespace not set")
	}
	if values == nil {
		return reconciler.NewValidationError("values not set")
	}

	// Validate target namespace exists
	if err := validation.ValidateTargetNamespace(ctx, r.client, namespace); err != nil {
		return err
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
	istiodChartPath := GetChartPath(version, constants.IstiodChartName)
	istiodReleaseName := getReleaseName(revisionName, constants.IstiodChartName)

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
		baseChartPath := GetChartPath(version, constants.BaseChartName)
		baseReleaseName := getReleaseName(revisionName, constants.BaseChartName)

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
	istiodReleaseName := getReleaseName(revisionName, constants.IstiodChartName)
	if _, err := r.cfg.ChartManager.UninstallChart(ctx, istiodReleaseName, namespace); err != nil {
		return fmt.Errorf("failed to uninstall Helm chart %q: %w", constants.IstiodChartName, err)
	}

	// Uninstall base chart for default revision
	if revisionName == v1.DefaultRevision {
		baseReleaseName := getReleaseName(revisionName, constants.BaseChartName)
		if _, err := r.cfg.ChartManager.UninstallChart(ctx, baseReleaseName, r.cfg.OperatorNamespace); err != nil {
			return fmt.Errorf("failed to uninstall Helm chart %q: %w", constants.BaseChartName, err)
		}
	}

	return nil
}

// getReleaseName returns the Helm release name for a given revision and chart.
func getReleaseName(revisionName, chartName string) string {
	return fmt.Sprintf("%s-%s", revisionName, chartName)
}
