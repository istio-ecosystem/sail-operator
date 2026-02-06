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
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
	"github.com/istio-ecosystem/sail-operator/pkg/istiovalues"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	"github.com/istio-ecosystem/sail-operator/pkg/reconciler"
	"github.com/istio-ecosystem/sail-operator/pkg/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	cniReleaseName = "istio-cni"
	cniChartName   = "cni"
)

// CNIReconciler handles reconciliation of the istio-cni component.
type CNIReconciler struct {
	cfg    Config
	client client.Client
}

// NewCNIReconciler creates a new CNIReconciler.
func NewCNIReconciler(cfg Config, client client.Client) *CNIReconciler {
	return &CNIReconciler{
		cfg:    cfg,
		client: client,
	}
}

// Validate performs general validation of the CNI specification.
// This includes basic field validation and Kubernetes API checks (namespace exists).
func (r *CNIReconciler) Validate(ctx context.Context, version, namespace string) error {
	if version == "" {
		return reconciler.NewValidationError("version not set")
	}
	if namespace == "" {
		return reconciler.NewValidationError("namespace not set")
	}

	// Validate target namespace exists
	if err := validation.ValidateTargetNamespace(ctx, r.client, namespace); err != nil {
		return err
	}

	return nil
}

// ComputeValues computes the final Helm values by applying digests, vendor defaults, and profiles.
func (r *CNIReconciler) ComputeValues(version string, userValues *v1.CNIValues, profile string) (helm.Values, error) {
	resolvedVersion, err := istioversion.Resolve(version)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve CNI version: %w", err)
	}

	// Apply image digests from configuration, if not already set by user
	userValues = ApplyCNIImageDigests(resolvedVersion, userValues, config.Config)

	// Apply vendor-specific default values
	userValues, err = istiovalues.ApplyIstioCNIVendorDefaults(resolvedVersion, userValues)
	if err != nil {
		return nil, fmt.Errorf("failed to apply vendor defaults: %w", err)
	}

	// Apply userValues on top of defaultValues from profiles
	mergedHelmValues, err := istiovalues.ApplyProfilesAndPlatform(
		r.cfg.ResourceFS, resolvedVersion, r.cfg.Platform, r.cfg.DefaultProfile, profile, helm.FromValues(userValues))
	if err != nil {
		return nil, fmt.Errorf("failed to apply profile: %w", err)
	}

	return mergedHelmValues, nil
}

// Install installs or upgrades the istio-cni Helm chart.
func (r *CNIReconciler) Install(ctx context.Context, version, namespace string, values *v1.CNIValues, profile string, ownerRef *metav1.OwnerReference) error {
	mergedHelmValues, err := r.ComputeValues(version, values, profile)
	if err != nil {
		return err
	}

	resolvedVersion, err := istioversion.Resolve(version)
	if err != nil {
		return fmt.Errorf("failed to resolve CNI version: %w", err)
	}

	chartPath := GetChartPath(resolvedVersion, cniChartName)
	_, err = r.cfg.ChartManager.UpgradeOrInstallChart(
		ctx,
		r.cfg.ResourceFS,
		chartPath,
		mergedHelmValues,
		namespace,
		cniReleaseName,
		ownerRef,
	)
	if err != nil {
		return fmt.Errorf("failed to install/update Helm chart %q: %w", cniChartName, err)
	}
	return nil
}

// Uninstall removes the istio-cni Helm chart.
func (r *CNIReconciler) Uninstall(ctx context.Context, namespace string) error {
	_, err := r.cfg.ChartManager.UninstallChart(ctx, cniReleaseName, namespace)
	if err != nil {
		return fmt.Errorf("failed to uninstall Helm chart %q: %w", cniChartName, err)
	}
	return nil
}

// ApplyCNIImageDigests applies image digests to CNI values if not already set by user.
// This function is exported for use by the controller and library.
func ApplyCNIImageDigests(version string, values *v1.CNIValues, cfg config.OperatorConfig) *v1.CNIValues {
	imageDigests, digestsDefined := cfg.ImageDigests[version]
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
