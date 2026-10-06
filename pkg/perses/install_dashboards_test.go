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
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/istio-ecosystem/sail-operator/pkg/helm"
	persesresources "github.com/istio-ecosystem/sail-operator/pkg/perses/resources"
	"github.com/istio-ecosystem/sail-operator/pkg/test/project"
	"helm.sh/helm/v4/pkg/release"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestInstallDashboardsCallsHelm(t *testing.T) {
	calls := 0
	mock := &mockChartReconciler{
		upgradeOrInstall: func(
			_ context.Context, resourceFS fs.FS, chartPath string, _ helm.Values,
			namespace, releaseName string, _ *metav1.OwnerReference,
		) (release.Releaser, error) {
			calls++
			if namespace != "sail-operator" {
				return nil, fmt.Errorf("namespace = %q", namespace)
			}
			if releaseName != persesReleaseName {
				return nil, fmt.Errorf("release = %q", releaseName)
			}
			if chartPath != persesresources.ChartPath {
				return nil, fmt.Errorf("chartPath = %q", chartPath)
			}
			if resourceFS == nil {
				return nil, errors.New("nil chart FS")
			}
			return nil, nil
		},
	}

	if err := installDashboards(context.Background(), mock, persesresources.ChartFS, persesresources.ChartPath, "sail-operator"); err != nil {
		t.Fatalf("installDashboards() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("UpgradeOrInstallChart calls = %d, want 1", calls)
	}
}

func TestInstallDashboardsHelmError(t *testing.T) {
	mock := &mockChartReconciler{
		upgradeOrInstall: func(context.Context, fs.FS, string, helm.Values, string, string, *metav1.OwnerReference) (release.Releaser, error) {
			return nil, fmt.Errorf("helm failed")
		},
	}
	err := installDashboards(context.Background(), mock, persesresources.ChartFS, persesresources.ChartPath, "sail-operator")
	if err == nil {
		t.Fatal("expected helm error")
	}
}

func TestRenderPersesDashboardsChart(t *testing.T) {
	rendered, err := helm.RenderChart(persesresources.ChartFS, persesresources.ChartPath, helm.Values{}, "sail-operator", persesReleaseName)
	if err != nil {
		t.Fatalf("RenderChart: %v", err)
	}
	if len(rendered) == 0 {
		t.Fatal("expected rendered templates")
	}

	fsys := os.DirFS(path.Join(project.RootDir, "pkg", "perses", "resources"))
	want := countDashboardYAMLs(t, fsys)

	count := 0
	for _, manifest := range rendered {
		for _, part := range strings.Split(manifest, "\n---") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			count++
			dashboard, err := PrepareDashboard([]byte(part), "sail-operator")
			if err != nil {
				t.Fatalf("PrepareDashboard: %v", err)
			}
			if dashboard.GetNamespace() != "sail-operator" {
				t.Fatalf("namespace = %q, want sail-operator", dashboard.GetNamespace())
			}
		}
	}
	if count != want {
		t.Fatalf("rendered %d dashboards, want %d", count, want)
	}
}

type mockChartReconciler struct {
	upgradeOrInstall func(context.Context, fs.FS, string, helm.Values, string, string, *metav1.OwnerReference) (release.Releaser, error)
	uninstall        func(context.Context, string, string) (*release.UninstallReleaseResponse, error)
	getRelease       func(context.Context, string, string) (release.Releaser, error)
}

func (m *mockChartReconciler) UpgradeOrInstallChart(
	ctx context.Context, resourceFS fs.FS, chartPath string, values helm.Values,
	namespace, releaseName string, ownerReference *metav1.OwnerReference,
) (release.Releaser, error) {
	if m.upgradeOrInstall != nil {
		return m.upgradeOrInstall(ctx, resourceFS, chartPath, values, namespace, releaseName, ownerReference)
	}
	return nil, nil
}

func (m *mockChartReconciler) UninstallChart(ctx context.Context, releaseName, namespace string) (*release.UninstallReleaseResponse, error) {
	if m.uninstall != nil {
		return m.uninstall(ctx, releaseName, namespace)
	}
	return nil, nil
}

func (m *mockChartReconciler) GetRelease(ctx context.Context, namespace, releaseName string) (release.Releaser, error) {
	if m.getRelease != nil {
		return m.getRelease(ctx, namespace, releaseName)
	}
	return nil, nil
}
