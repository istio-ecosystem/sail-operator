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

package perses

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/istio-ecosystem/sail-operator/pkg/helm"
	persesresources "github.com/istio-ecosystem/sail-operator/pkg/perses/resources"
	ctrl "sigs.k8s.io/controller-runtime"
)

// installDashboards installs or upgrades bundled PersesDashboard resources using the embedded
// Helm chart under pkg/perses/resources. Dashboard content is upgraded on each operator start
// when the chart changes.
func installDashboards(
	ctx context.Context,
	chartManager helm.ChartReconciler,
	chartFS fs.FS,
	chartPath string,
	namespace string,
) error {
	log := ctrl.LoggerFrom(ctx)

	if chartFS == nil {
		chartFS = persesresources.ChartFS
	}
	if chartPath == "" {
		chartPath = persesresources.ChartPath
	}

	_, err := chartManager.UpgradeOrInstallChart(ctx, chartFS, chartPath, helm.Values{}, namespace, persesReleaseName, nil)
	if err != nil {
		return fmt.Errorf("helm upgrade or install perses dashboards: %w", err)
	}

	log.Info("Perses dashboards installed or upgraded", "namespace", namespace, "release", persesReleaseName)
	return nil
}
