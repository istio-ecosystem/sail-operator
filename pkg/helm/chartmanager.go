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

package helm

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/go-logr/logr"
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	"helm.sh/helm/v4/pkg/action"
	chartv2 "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/kube"
	"helm.sh/helm/v4/pkg/release"
	releasecommon "helm.sh/helm/v4/pkg/release/common"
	releasev1 "helm.sh/helm/v4/pkg/release/v1"
	"helm.sh/helm/v4/pkg/storage/driver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type ChartManager struct {
	restClientGetter genericclioptions.RESTClientGetter
	driver           string
	managedByValue   string
}

// ChartManagerOption is a functional option for configuring a ChartManager.
type ChartManagerOption func(*ChartManager)

// WithManagedByValue overrides the value of the "managed-by" label that is
// applied to every resource created by Helm install/upgrade. The default
// value is constants.ManagedByLabelValue ("sail-operator").
func WithManagedByValue(v string) ChartManagerOption {
	return func(cm *ChartManager) {
		cm.managedByValue = v
	}
}

// NewChartManager creates a new Helm chart manager using cfg as the configuration
// that Helm will use to connect to the cluster when installing or uninstalling
// charts, and using the specified driver to store information about releases
// (one of: memory, secret, configmap, sql, or "" (same as "secret")).
func NewChartManager(cfg *rest.Config, driver string, opts ...ChartManagerOption) *ChartManager {
	cm := &ChartManager{
		restClientGetter: NewRESTClientGetter(cfg),
		driver:           driver,
		managedByValue:   constants.ManagedByLabelValue,
	}
	for _, o := range opts {
		o(cm)
	}
	return cm
}

// newActionConfig Create a new Helm action config from in-cluster service account
func (h *ChartManager) newActionConfig(ctx context.Context, namespace string) (*action.Configuration, error) {
	log := logf.FromContext(ctx)
	actionConfig := action.NewConfiguration(action.ConfigurationSetLogger(logr.ToSlogHandler(log.V(2))))
	err := actionConfig.Init(h.restClientGetter, namespace, h.driver)
	return actionConfig, err
}

// UpgradeOrInstallChart upgrades a chart in cluster or installs it new if it does not already exist.
// It loads the chart from an fs.FS (e.g., embed.FS or os.DirFS).
func (h *ChartManager) UpgradeOrInstallChart(
	ctx context.Context, resourceFS fs.FS, chartPath string, values Values,
	namespace, releaseName string, ownerReference *metav1.OwnerReference,
) (release.Releaser, error) {
	loadedChart, err := LoadChart(resourceFS, chartPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load chart from fs: %w", err)
	}

	return h.upgradeOrInstallChart(ctx, loadedChart, values, namespace, releaseName, ownerReference)
}

// upgradeOrInstallChart is the internal implementation that works with an already-loaded chart
func (h *ChartManager) upgradeOrInstallChart(
	ctx context.Context, chart *chartv2.Chart, values Values,
	namespace, releaseName string, ownerReference *metav1.OwnerReference,
) (release.Releaser, error) {
	log := logf.FromContext(ctx)

	cfg, err := h.newActionConfig(ctx, namespace)
	if err != nil {
		return nil, err
	}

	rel, err := getRelease(cfg, releaseName)
	if err != nil {
		return rel, err
	}

	releaseExists := rel != nil
	var relV1 *releasev1.Release
	if releaseExists {
		var ok bool
		relV1, ok = rel.(*releasev1.Release)
		if !ok {
			return nil, fmt.Errorf("unexpected release type %T for helm release %s", rel, releaseName)
		}
	}

	// A helm release can be stuck in pending state when:
	// - operator exit/crashes during helm install/upgrade/uninstall
	// - lost connection to apiserver
	// - helm release timeouts (context timeouts, cancellation)
	// - etc
	// let's try to brutally unlock it and then remediate later by either rollback or uninstall
	if releaseExists && relV1.Info.Status.IsPending() {
		log.V(2).Info("Unlocking helm release", "status", relV1.Info.Status, "release", releaseName)
		relV1.SetStatus(releasecommon.StatusFailed, fmt.Sprintf("Release unlocked from %q state", relV1.Info.Status))

		if err := cfg.Releases.Update(rel); err != nil {
			return nil, fmt.Errorf("failed to unlock helm release %s: %w", releaseName, err)
		}
	}

	switch {
	case !releaseExists:
		break
	case relV1.Info.Status == releasecommon.StatusDeployed:
		break
	case relV1.Info.Status == releasecommon.StatusFailed && relV1.Version > 1:
		log.V(2).Info("Performing helm rollback", "release", releaseName)
		rollbackAction := action.NewRollback(cfg)
		rollbackAction.WaitStrategy = kube.HookOnlyStrategy
		rollbackAction.WaitForJobs = false
		rollbackAction.ServerSideApply = "false"
		if err := rollbackAction.Run(releaseName); err != nil {
			return nil, fmt.Errorf("failed to roll back helm release %s: %w", releaseName, err)
		}
	case relV1.Info.Status == releasecommon.StatusUninstalling,
		relV1.Info.Status == releasecommon.StatusFailed && relV1.Version <= 1:
		log.V(2).Info("Performing helm uninstall", "release", releaseName, "status", relV1.Info.Status)
		uninstallAction := action.NewUninstall(cfg)
		uninstallAction.WaitStrategy = kube.HookOnlyStrategy
		if _, err := uninstallAction.Run(releaseName); err != nil {
			return nil, fmt.Errorf("failed to uninstall failed helm release %s: %w", releaseName, err)
		}
		releaseExists = false
	default:
		return nil, fmt.Errorf("unexpected helm release status %s", relV1.Info.Status)
	}

	if releaseExists {
		log.V(2).Info("Performing helm upgrade", "chartName", chart.Name())

		updateAction := action.NewUpgrade(cfg)
		updateAction.PostRenderer = NewHelmPostRenderer(ownerReference, "", true, h.managedByValue)
		updateAction.MaxHistory = 1
		updateAction.SkipCRDs = true
		updateAction.DisableOpenAPIValidation = true
		updateAction.WaitStrategy = kube.HookOnlyStrategy
		updateAction.ServerSideApply = "false"
		updateAction.WaitForJobs = false
		rel, err = updateAction.RunWithContext(ctx, releaseName, chart, values)
		if err != nil {
			return nil, fmt.Errorf("failed to update helm chart %s: %w", chart.Name(), err)
		}
	} else {
		log.V(2).Info("Performing helm install", "chartName", chart.Name())

		installAction := action.NewInstall(cfg)
		installAction.PostRenderer = NewHelmPostRenderer(ownerReference, "", false, h.managedByValue)
		installAction.Namespace = namespace
		installAction.ReleaseName = releaseName
		installAction.SkipCRDs = true
		installAction.DisableOpenAPIValidation = true
		installAction.WaitStrategy = kube.HookOnlyStrategy
		installAction.ServerSideApply = false
		installAction.WaitForJobs = false
		rel, err = installAction.RunWithContext(ctx, chart, values)
		if err != nil {
			return nil, fmt.Errorf("failed to install helm chart %s: %w", chart.Name(), err)
		}
	}
	return rel, nil
}

// UninstallChart removes a chart from the cluster
func (h *ChartManager) UninstallChart(ctx context.Context, releaseName, namespace string) (*release.UninstallReleaseResponse, error) {
	cfg, err := h.newActionConfig(ctx, namespace)
	if err != nil {
		return nil, err
	}

	if rel, err := getRelease(cfg, releaseName); err != nil {
		return nil, err
	} else if rel == nil {
		// release does not exist; no need for uninstall
		return &release.UninstallReleaseResponse{Info: "release not found"}, nil
	}

	uninstallAction := action.NewUninstall(cfg)
	uninstallAction.WaitStrategy = kube.HookOnlyStrategy
	return uninstallAction.Run(releaseName)
}

func getRelease(cfg *action.Configuration, releaseName string) (release.Releaser, error) {
	getAction := action.NewGet(cfg)
	rel, err := getAction.Run(releaseName)
	if err != nil && !errors.Is(err, driver.ErrReleaseNotFound) {
		return nil, fmt.Errorf("failed to get helm release %s: %w", releaseName, err)
	}
	return rel, nil
}

func (h *ChartManager) GetRelease(ctx context.Context, namespace, releaseName string) (release.Releaser, error) {
	cfg, err := h.newActionConfig(ctx, namespace)
	if err != nil {
		return nil, err
	}

	return getRelease(cfg, releaseName)
}

func (h *ChartManager) UpdateRelease(ctx context.Context, namespace string, rel release.Releaser) error {
	cfg, err := h.newActionConfig(ctx, namespace)
	if err != nil {
		return err
	}
	return cfg.Releases.Update(rel)
}

func (h *ChartManager) ListReleases(ctx context.Context) ([]release.Releaser, error) {
	cfg, err := h.newActionConfig(ctx, "")
	if err != nil {
		return nil, err
	}

	listAction := action.NewList(cfg)
	listAction.AllNamespaces = true
	return listAction.Run()
}
