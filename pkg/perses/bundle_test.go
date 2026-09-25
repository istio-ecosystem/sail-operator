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
	"os"
	"path"
	"strings"
	"testing"

	"github.com/istio-ecosystem/sail-operator/pkg/test/project"
	persesresources "github.com/istio-ecosystem/sail-operator/resources/perses"
)

func TestBundledDashboardsHaveSpecConfig(t *testing.T) {
	fsys := os.DirFS(path.Join(project.RootDir, "resources", "perses"))
	for _, def := range ProductDashboards {
		data, err := LoadDashboardYAML(fsys, def)
		if err != nil {
			t.Fatalf("LoadDashboardYAML(%s): %v", def.Filename, err)
		}
		dashboard, err := ParseDashboard(data)
		if err != nil {
			t.Fatalf("ParseDashboard(%s): %v", def.Filename, err)
		}
		spec, ok := dashboard.Object["spec"]
		if !ok || spec == nil {
			t.Fatalf("dashboard %s missing spec", def.Name)
		}
		config, ok := spec.(map[string]interface{})["config"]
		if !ok || config == nil {
			t.Fatalf("dashboard %s missing spec.config", def.Name)
		}
	}
}

func TestEmbeddedDashboardsReadable(t *testing.T) {
	for _, def := range ProductDashboards {
		data, err := LoadDashboardYAML(persesresources.FS, def)
		if err != nil {
			t.Fatalf("LoadDashboardYAML(%s): %v", def.Filename, err)
		}
		if len(data) == 0 {
			t.Fatalf("dashboard %s is empty", def.Name)
		}
	}
}

func TestBundledDashboardsReferenceRequiredDatasource(t *testing.T) {
	for _, def := range ProductDashboards {
		data, err := LoadDashboardYAML(persesresources.FS, def)
		if err != nil {
			t.Fatalf("LoadDashboardYAML(%s): %v", def.Filename, err)
		}
		if !strings.Contains(string(data), RequiredDatasourceName) {
			t.Fatalf("dashboard %s does not reference required datasource %q", def.Name, RequiredDatasourceName)
		}
	}
}

func TestPrepareDashboardSetsNamespaceAndClearsOwners(t *testing.T) {
	fsys := os.DirFS(path.Join(project.RootDir, "resources", "perses"))
	def := ProductDashboards[0]
	data, err := LoadDashboardYAML(fsys, def)
	if err != nil {
		t.Fatalf("LoadDashboardYAML: %v", err)
	}
	dashboard, err := PrepareDashboard(data, "sail-operator", def)
	if err != nil {
		t.Fatalf("PrepareDashboard: %v", err)
	}
	if dashboard.GetNamespace() != "sail-operator" {
		t.Fatalf("namespace = %q, want sail-operator", dashboard.GetNamespace())
	}
	if dashboard.GetName() != def.Name {
		t.Fatalf("name = %q, want %q", dashboard.GetName(), def.Name)
	}
	if len(dashboard.GetOwnerReferences()) != 0 {
		t.Fatal("expected no ownerReferences")
	}
}

func TestLoadDashboardYAMLMissing(t *testing.T) {
	_, err := LoadDashboardYAML(os.DirFS(t.TempDir()), DashboardDefinition{Filename: "missing.yaml"})
	if err == nil {
		t.Fatal("expected error for missing dashboard file")
	}
}

func TestParseDashboardInvalid(t *testing.T) {
	_, err := ParseDashboard([]byte("::: not yaml"))
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestPrepareDashboardInvalid(t *testing.T) {
	_, err := PrepareDashboard([]byte("{"), "ns", ProductDashboards[0])
	if err == nil {
		t.Fatal("expected prepare error for invalid YAML")
	}
}
