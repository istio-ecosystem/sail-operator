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
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	persesresources "github.com/istio-ecosystem/sail-operator/pkg/perses/resources"
	"github.com/istio-ecosystem/sail-operator/pkg/test/project"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
)

func forEachDashboardYAML(fsys fs.FS, fn func(path string, data []byte) error) error {
	return fs.WalkDir(fsys, dashboardsDir, func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		lower := strings.ToLower(name)
		if !strings.HasSuffix(lower, ".yaml") && !strings.HasSuffix(lower, ".yml") {
			return nil
		}
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return fmt.Errorf("read dashboard %s: %w", name, err)
		}
		return fn(name, data)
	})
}

func parseDashboard(data []byte) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	decoder := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	if err := decoder.Decode(obj); err != nil {
		return nil, fmt.Errorf("decode dashboard: %w", err)
	}
	return obj, nil
}

func prepareDashboard(data []byte, namespace string) (*unstructured.Unstructured, error) {
	dashboard, err := parseDashboard(data)
	if err != nil {
		return nil, err
	}
	if dashboard.GroupVersionKind().Group != persesGroup ||
		dashboard.GroupVersionKind().Version != persesVersion ||
		dashboard.GetKind() != "PersesDashboard" {
		return nil, fmt.Errorf("unexpected object kind %q apiVersion %q; want PersesDashboard %s/%s",
			dashboard.GetKind(), dashboard.GetAPIVersion(), persesGroup, persesVersion)
	}
	if dashboard.GetName() == "" {
		return nil, fmt.Errorf("dashboard missing metadata.name")
	}
	dashboard.SetGroupVersionKind(DashboardGVK)
	dashboard.SetNamespace(namespace)
	dashboard.SetOwnerReferences(nil)
	labels := dashboard.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[constants.KubernetesAppManagedByKey] = constants.ManagedByLabelValue
	dashboard.SetLabels(labels)
	return dashboard, nil
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

func TestBundledDashboardsHaveSpecConfig(t *testing.T) {
	fsys := os.DirFS(path.Join(project.RootDir, "pkg", "perses", "resources"))
	err := forEachDashboardYAML(fsys, func(filePath string, data []byte) error {
		dashboard, err := parseDashboard(data)
		if err != nil {
			t.Fatalf("parseDashboard(%s): %v", filePath, err)
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
	err := forEachDashboardYAML(persesresources.ChartFS, func(_ string, data []byte) error {
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
	err := forEachDashboardYAML(persesresources.ChartFS, func(filePath string, data []byte) error {
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
	dashboard, err := prepareDashboard(data, "sail-operator")
	if err != nil {
		t.Fatalf("prepareDashboard: %v", err)
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
	_, err := parseDashboard([]byte("::: not yaml"))
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestPrepareDashboardInvalid(t *testing.T) {
	_, err := prepareDashboard([]byte("{"), "ns")
	if err == nil {
		t.Fatal("expected prepare error for invalid YAML")
	}
}

func TestPrepareDashboardMissingName(t *testing.T) {
	_, err := prepareDashboard([]byte("apiVersion: perses.dev/v1alpha2\nkind: PersesDashboard\nmetadata: {}\n"), "ns")
	if err == nil {
		t.Fatal("expected error for missing metadata.name")
	}
}

func TestPrepareDashboardWrongKind(t *testing.T) {
	_, err := prepareDashboard([]byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: cm\n"), "ns")
	if err == nil {
		t.Fatal("expected error for wrong kind")
	}
}

func TestPrepareDashboardSetsManagedByLabel(t *testing.T) {
	const manifest = `apiVersion: perses.dev/v1alpha2
kind: PersesDashboard
metadata:
  name: test-dashboard
spec:
  config: {}
`
	dashboard, err := prepareDashboard([]byte(manifest), "sail-operator")
	if err != nil {
		t.Fatalf("prepareDashboard: %v", err)
	}
	if dashboard.GetLabels()[constants.KubernetesAppManagedByKey] != constants.ManagedByLabelValue {
		t.Fatalf("managed-by label = %q, want %q", dashboard.GetLabels()[constants.KubernetesAppManagedByKey], constants.ManagedByLabelValue)
	}
}

func TestForEachDashboardYAMLSkipsNonYAML(t *testing.T) {
	const manifest = `apiVersion: perses.dev/v1alpha2
kind: PersesDashboard
metadata:
  name: dash
spec:
  config: {}
`
	fsys := fstest.MapFS{
		"files/README.md": &fstest.MapFile{Data: []byte("# readme")},
		"files/dash.yaml": &fstest.MapFile{Data: []byte(manifest)},
	}
	count := 0
	if err := forEachDashboardYAML(fsys, func(string, []byte) error {
		count++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 YAML dashboard, got %d", count)
	}
}

func TestForEachDashboardYAMLPropagatesCallbackError(t *testing.T) {
	fsys := fstest.MapFS{
		"files/dash.yaml": &fstest.MapFile{Data: []byte("apiVersion: perses.dev/v1alpha2\nkind: PersesDashboard\nmetadata:\n  name: dash\n")},
	}
	err := forEachDashboardYAML(fsys, func(string, []byte) error {
		return fmt.Errorf("callback failed")
	})
	if err == nil {
		t.Fatal("expected callback error")
	}
}

func TestForEachDashboardYAMLMissingDir(t *testing.T) {
	err := forEachDashboardYAML(fstest.MapFS{}, func(string, []byte) error { return nil })
	if err == nil {
		t.Fatal("expected error when files/ is missing")
	}
}
