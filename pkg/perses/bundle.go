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
	"path"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/istio-ecosystem/sail-operator/pkg/constants"
)

// LoadDashboardYAML returns vendored PersesDashboard manifest bytes.
func LoadDashboardYAML(fsys fs.FS, def DashboardDefinition) ([]byte, error) {
	filePath := path.Join("dashboards", def.Filename)
	data, err := fs.ReadFile(fsys, filePath)
	if err != nil {
		return nil, fmt.Errorf("read dashboard %s: %w", def.Filename, err)
	}
	return data, nil
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
// Existing ownerReferences from the vendored YAML are cleared; dashboards are not owned
// by any Sail resource and are never deleted by the operator.
func PrepareDashboard(data []byte, namespace string, def DashboardDefinition) (*unstructured.Unstructured, error) {
	dashboard, err := ParseDashboard(data)
	if err != nil {
		return nil, err
	}
	dashboard.SetGroupVersionKind(DashboardGVK)
	dashboard.SetName(def.Name)
	dashboard.SetNamespace(namespace)
	dashboard.SetOwnerReferences(nil)
	dashboard.SetLabels(map[string]string{
		constants.KubernetesAppManagedByKey: constants.ManagedByLabelValue,
	})
	return dashboard, nil
}
