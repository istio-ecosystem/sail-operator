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
	"errors"
	"io/fs"
	"os"
	"path"
	"strings"
	"testing"

	persesresources "github.com/istio-ecosystem/sail-operator/pkg/perses/resources"
	"github.com/istio-ecosystem/sail-operator/pkg/test/project"
)

func TestBundledDashboardsHaveSpecConfig(t *testing.T) {
	fsys := os.DirFS(path.Join(project.RootDir, "pkg", "perses", "resources"))
	err := forEachDashboardYAML(fsys, func(filePath string, data []byte) error {
		dashboard, err := ParseDashboard(data)
		if err != nil {
			t.Fatalf("ParseDashboard(%s): %v", filePath, err)
		}
		if dashboard.GetName() == "" {
			t.Fatalf("dashboard %s missing metadata.name", filePath)
		}
		spec, ok := dashboard.Object["spec"]
		if !ok || spec == nil {
			t.Fatalf("dashboard %s missing spec", filePath)
		}
		config, ok := spec.(map[string]interface{})["config"]
		if !ok || config == nil {
			t.Fatalf("dashboard %s missing spec.config", filePath)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestEmbeddedDashboardsReadable(t *testing.T) {
	count := 0
	err := forEachDashboardYAML(persesresources.FS, func(_ string, data []byte) error {
		if len(data) == 0 {
			t.Fatal("found empty dashboard")
		}
		count++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("expected embedded dashboards")
	}
}

func TestBundledDashboardsReferenceRequiredDatasource(t *testing.T) {
	err := forEachDashboardYAML(persesresources.FS, func(filePath string, data []byte) error {
		if !strings.Contains(string(data), RequiredDatasourceName) {
			t.Fatalf("dashboard %s does not reference required datasource %q", filePath, RequiredDatasourceName)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPrepareDashboardSetsNamespaceAndClearsOwners(t *testing.T) {
	fsys := os.DirFS(path.Join(project.RootDir, "pkg", "perses", "resources"))
	var data []byte
	err := forEachDashboardYAML(fsys, func(_ string, d []byte) error {
		data = d
		return fs.SkipAll
	})
	if err != nil && !errors.Is(err, fs.SkipAll) {
		t.Fatalf("load dashboard: %v", err)
	}
	if data == nil {
		t.Fatal("no dashboard YAML found")
	}
	dashboard, err := PrepareDashboard(data, "sail-operator")
	if err != nil {
		t.Fatalf("PrepareDashboard: %v", err)
	}
	if dashboard.GetNamespace() != "sail-operator" {
		t.Fatalf("namespace = %q, want sail-operator", dashboard.GetNamespace())
	}
	if dashboard.GetName() == "" {
		t.Fatal("expected name from YAML metadata")
	}
	if len(dashboard.GetOwnerReferences()) != 0 {
		t.Fatal("expected no ownerReferences")
	}
}

func TestParseDashboardInvalid(t *testing.T) {
	_, err := ParseDashboard([]byte("::: not yaml"))
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestPrepareDashboardInvalid(t *testing.T) {
	_, err := PrepareDashboard([]byte("{"), "ns")
	if err == nil {
		t.Fatal("expected prepare error for invalid YAML")
	}
}

func TestPrepareDashboardMissingName(t *testing.T) {
	_, err := PrepareDashboard([]byte("apiVersion: perses.dev/v1alpha2\nkind: PersesDashboard\nmetadata: {}\n"), "ns")
	if err == nil {
		t.Fatal("expected error for missing metadata.name")
	}
}

func countDashboardYAMLs(t *testing.T, fsys fs.FS) int {
	t.Helper()
	count := 0
	err := forEachDashboardYAML(fsys, func(string, []byte) error {
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("count dashboards: %v", err)
	}
	return count
}
