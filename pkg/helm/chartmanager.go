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

	"helm.sh/helm/v3/pkg/action"
	chartLoader "helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/storage/driver"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/rest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type ChartManager struct {
	restClientGetter genericclioptions.RESTClientGetter
	driver           string
}

// NewChartManager creates a new Helm chart manager using cfg as the configuration
// that Helm will use to connect to the cluster when installing or uninstalling
// charts, and using the specified driver to store information about releases
// (one of: memory, secret, configmap, sql, or "" (same as "secret")).
func NewChartManager(cfg *rest.Config, driver string) *ChartManager {
	return &ChartManager{
		restClientGetter: NewRESTClientGetter(cfg),
		driver:           driver,
	}
}

// newActionConfig Create a new Helm action config from in-cluster service account
func (h *ChartManager) newActionConfig(ctx context.Context, namespace string) (*action.Configuration, error) {
	logAdapter := func(format string, v ...any) {
		log := logf.FromContext(ctx)
		logv2 := log.V(2)
		if logv2.Enabled() {
			logv2.Info(fmt.Sprintf(format, v...))
		}
	}

	actionConfig := new(action.Configuration)
	err := actionConfig.Init(h.restClientGetter, namespace, h.driver, logAdapter)
	return actionConfig, err
}

// UpgradeOrInstallChart upgrades a chart in cluster or installs it new if it does not already exist
func (h *ChartManager) UpgradeOrInstallChart(
	ctx context.Context, chartDir string, values Values,
	namespace, releaseName string, ownerReference *metav1.OwnerReference,
) (*release.Release, error) {
	log := logf.FromContext(ctx)

	cfg, err := h.newActionConfig(ctx, namespace)
	if err != nil {
		return nil, err
	}

	chart, err := chartLoader.Load(chartDir)
	if err != nil {
		return nil, err
	}

	rel, err := getRelease(cfg, releaseName)
	if err != nil {
		return rel, err
	}

	releaseExists := rel != nil

	// A helm release can be stuck in pending state when:
	// - operator exit/crashes during helm install/upgrade/uninstall
	// - lost connection to apiserver
	// - helm release timeouts (context timeouts, cancellation)
	// - etc
	// let's try to brutally unlock it and then remediate later by either rollback or uninstall
	if releaseExists && rel.Info.Status.IsPending() {
		log.V(2).Info("Unlocking helm release", "status", rel.Info.Status, "release", releaseName)
		rel.SetStatus(release.StatusFailed, fmt.Sprintf("Release unlocked from %q state", rel.Info.Status))

		if err := cfg.Releases.Update(rel); err != nil {
			return nil, fmt.Errorf("failed to unlock helm release %s: %w", releaseName, err)
		}
	}

	switch {
	case !releaseExists:
		break
	case rel.Info.Status == release.StatusDeployed:
		break
	case rel.Info.Status == release.StatusFailed && rel.Version > 1:
		log.V(2).Info("Performing helm rollback", "release", releaseName)
		if err := action.NewRollback(cfg).Run(releaseName); err != nil {
			return nil, fmt.Errorf("failed to roll back helm release %s: %w", releaseName, err)
		}
	case rel.Info.Status == release.StatusUninstalling,
		rel.Info.Status == release.StatusFailed && rel.Version <= 1:
		log.V(2).Info("Performing helm uninstall", "release", releaseName, "status", rel.Info.Status)
		if _, err := action.NewUninstall(cfg).Run(releaseName); err != nil {
			return nil, fmt.Errorf("failed to uninstall failed helm release %s: %w", releaseName, err)
		}
		releaseExists = false
	default:
		return nil, fmt.Errorf("unexpected helm release status %s", rel.Info.Status)
	}

	if releaseExists {
		log.V(2).Info("Performing helm upgrade", "chartName", chart.Name())

		updateAction := action.NewUpgrade(cfg)
		updateAction.PostRenderer = NewHelmPostRenderer(ownerReference, "", true)
		updateAction.MaxHistory = 1
		updateAction.SkipCRDs = true
		updateAction.DisableOpenAPIValidation = true

		rel, err = updateAction.RunWithContext(ctx, releaseName, chart, values)
		if err != nil {
			return nil, fmt.Errorf("failed to update helm chart %s: %w", chart.Name(), err)
		}
	} else {
		log.V(2).Info("Performing helm install", "chartName", chart.Name())

		installAction := action.NewInstall(cfg)
		installAction.PostRenderer = NewHelmPostRenderer(ownerReference, "", false)
		installAction.Namespace = namespace
		installAction.ReleaseName = releaseName
		installAction.SkipCRDs = true
		installAction.DisableOpenAPIValidation = true

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

	return action.NewUninstall(cfg).Run(releaseName)
}

func getRelease(cfg *action.Configuration, releaseName string) (*release.Release, error) {
	getAction := action.NewGet(cfg)
	rel, err := getAction.Run(releaseName)
	if err != nil && !errors.Is(err, driver.ErrReleaseNotFound) {
		return nil, fmt.Errorf("failed to get helm release %s: %w", releaseName, err)
	}
	return rel, nil
}

func (h *ChartManager) GetRelease(ctx context.Context, namespace, releaseName string) (*release.Release, error) {
	cfg, err := h.newActionConfig(ctx, namespace)
	if err != nil {
		return nil, err
	}

	return getRelease(cfg, releaseName)
}

func (h *ChartManager) UpdateRelease(ctx context.Context, namespace string, rel *release.Release) error {
	cfg, err := h.newActionConfig(ctx, namespace)
	if err != nil {
		return err
	}
	return cfg.Releases.Update(rel)
}

func (h *ChartManager) ListReleases(ctx context.Context) ([]*release.Release, error) {
	cfg, err := h.newActionConfig(ctx, "")
	if err != nil {
		return nil, err
	}

	listAction := action.NewList(cfg)
	listAction.AllNamespaces = true
	return listAction.Run()
}
