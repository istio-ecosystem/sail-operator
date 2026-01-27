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
	ztunnelReleaseName = "ztunnel"
	ztunnelChartName   = "ztunnel"
	ztunnelProfile     = "ambient"
)

// ZTunnelReconciler handles reconciliation of the ztunnel component.
type ZTunnelReconciler struct {
	cfg    Config
	client client.Client
}

// NewZTunnelReconciler creates a new ZTunnelReconciler.
func NewZTunnelReconciler(cfg Config, client client.Client) *ZTunnelReconciler {
	return &ZTunnelReconciler{
		cfg:    cfg,
		client: client,
	}
}

// Validate performs general validation of the ZTunnel specification.
// This includes basic field validation and Kubernetes API checks (namespace exists).
func (r *ZTunnelReconciler) Validate(ctx context.Context, version, namespace string) error {
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

// ComputeValues computes the final Helm values by applying digests, profiles, and user overrides.
func (r *ZTunnelReconciler) ComputeValues(version string, userValues *v1.ZTunnelValues) (helm.Values, error) {
	resolvedVersion, err := istioversion.Resolve(version)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve ZTunnel version: %w", err)
	}

	if userValues == nil {
		userValues = &v1.ZTunnelValues{}
	}

	// Apply image digests from configuration, if not already set by user
	userValues = ApplyZTunnelImageDigests(resolvedVersion, userValues, config.Config)

	// Apply userValues on top of defaultValues from profiles
	mergedHelmValues, err := istiovalues.ApplyProfilesAndPlatform(
		r.cfg.ResourceFS, resolvedVersion, r.cfg.Platform, r.cfg.DefaultProfile, ztunnelProfile, helm.FromValues(userValues))
	if err != nil {
		return nil, fmt.Errorf("failed to apply profile: %w", err)
	}

	// apply fips values
	mergedHelmValues, err = istiovalues.ApplyZTunnelFipsValues(mergedHelmValues)
	if err != nil {
		return nil, fmt.Errorf("failed to apply FIPS values: %w", err)
	}

	// Apply any user Overrides configured as part of values.ztunnel
	// This step was not required for the IstioCNI resource because the Helm templates[*] automatically override values.cni
	// [*]https://github.com/istio/istio/blob/0200fd0d4c3963a72f36987c2e8c2887df172abf/manifests/charts/istio-cni/templates/zzy_descope_legacy.yaml#L3
	// However, ztunnel charts do not have such a file, hence we are manually applying the mergeOperation here.
	finalHelmValues, err := istiovalues.ApplyUserValues(helm.FromValues(mergedHelmValues), helm.FromValues(userValues.ZTunnel))
	if err != nil {
		return nil, fmt.Errorf("failed to apply user overrides: %w", err)
	}

	return finalHelmValues, nil
}

// Install installs or upgrades the ztunnel Helm chart.
func (r *ZTunnelReconciler) Install(ctx context.Context, version, namespace string, values *v1.ZTunnelValues, ownerRef *metav1.OwnerReference) error {
	finalHelmValues, err := r.ComputeValues(version, values)
	if err != nil {
		return err
	}

	resolvedVersion, err := istioversion.Resolve(version)
	if err != nil {
		return fmt.Errorf("failed to resolve ZTunnel version: %w", err)
	}

	chartPath := GetChartPath(resolvedVersion, ztunnelChartName)
	_, err = r.cfg.ChartManager.UpgradeOrInstallChart(
		ctx,
		r.cfg.ResourceFS,
		chartPath,
		finalHelmValues,
		namespace,
		ztunnelReleaseName,
		ownerRef,
	)
	if err != nil {
		return fmt.Errorf("failed to install/update Helm chart %q: %w", ztunnelChartName, err)
	}
	return nil
}

// Uninstall removes the ztunnel Helm chart.
func (r *ZTunnelReconciler) Uninstall(ctx context.Context, namespace string) error {
	_, err := r.cfg.ChartManager.UninstallChart(ctx, ztunnelReleaseName, namespace)
	if err != nil {
		return fmt.Errorf("failed to uninstall Helm chart %q: %w", ztunnelChartName, err)
	}
	return nil
}

// ApplyZTunnelImageDigests applies image digests to ZTunnel values if not already set by user.
// This function is exported for use by the controller and library.
func ApplyZTunnelImageDigests(version string, values *v1.ZTunnelValues, cfg config.OperatorConfig) *v1.ZTunnelValues {
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
		values = &v1.ZTunnelValues{}
	}

	// set image digest unless any part of the image has been configured by the user
	if values.ZTunnel == nil {
		values.ZTunnel = &v1.ZTunnelConfig{}
	}
	if values.ZTunnel.Image == nil && values.ZTunnel.Hub == nil && values.ZTunnel.Tag == nil {
		values.ZTunnel.Image = &imageDigests.ZTunnelImage
	}
	return values
}
