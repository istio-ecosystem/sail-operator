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
	"fmt"
	"io/fs"
	"strings"

	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
)

// forEachDashboardYAML walks dashboards/ and invokes fn for each YAML file.
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

// ParseDashboard unmarshals a PersesDashboard YAML manifest.
func ParseDashboard(data []byte) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	decoder := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	if err := decoder.Decode(obj); err != nil {
		return nil, fmt.Errorf("decode dashboard: %w", err)
	}
	return obj, nil
}

// PrepareDashboard prepares a PersesDashboard for creation in the target namespace.
// The dashboard name is taken from the YAML metadata. Existing ownerReferences from the
// vendored YAML are cleared; dashboards are not owned by any Sail resource and are never
// deleted by the operator.
func PrepareDashboard(data []byte, namespace string) (*unstructured.Unstructured, error) {
	dashboard, err := ParseDashboard(data)
	if err != nil {
		return nil, err
	}
	if dashboard.GetName() == "" {
		return nil, fmt.Errorf("dashboard missing metadata.name")
	}
	dashboard.SetGroupVersionKind(DashboardGVK)
	dashboard.SetNamespace(namespace)
	dashboard.SetOwnerReferences(nil)
	dashboard.SetLabels(map[string]string{
		constants.KubernetesAppManagedByKey: constants.ManagedByLabelValue,
	})
	return dashboard, nil
}
