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

package install

import (
	"fmt"
	"io/fs"
	"maps"
	"path"
	"strings"
	"testing"

	"github.com/istio-ecosystem/sail-operator/pkg/helm"
	"github.com/istio-ecosystem/sail-operator/resources"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

func TestGatewayClassDefaultsConfigMapName(t *testing.T) {
	tests := []struct {
		name         string
		revision     string
		gatewayClass string
		expected     string
	}{
		{
			name:         "revision is used verbatim",
			revision:     "openshift-gateway",
			gatewayClass: "openshift-clusterip",
			expected:     "istio-openshift-gateway-gatewayclass-openshift-clusterip",
		},
		{
			name:         "empty revision falls back to default",
			revision:     "",
			gatewayClass: "openshift-clusterip",
			expected:     "istio-default-gatewayclass-openshift-clusterip",
		},
		{
			name:         "gateway class name containing hyphens is preserved",
			revision:     "my-revision",
			gatewayClass: "my-class-with-hyphens",
			expected:     "istio-my-revision-gatewayclass-my-class-with-hyphens",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(GatewayClassDefaultsConfigMapName(tc.revision, tc.gatewayClass)).To(Equal(tc.expected))
		})
	}
}

// TestGatewayClassDefaultsConfigMapMatchesRenderedChart renders the bundled
// istiod chart with the same values Sail uses and asserts that the ConfigMap
// name and label produced by the chart match the helper. This guards against
// the chart template changing in a future Istio version bump.
func TestGatewayClassDefaultsConfigMapMatchesRenderedChart(t *testing.T) {
	const gatewayClass = "openshift-clusterip"

	entries, err := fs.ReadDir(resources.FS, ".")
	if err != nil {
		t.Fatalf("failed to list bundled Istio versions: %v", err)
	}

	renderedVersions := 0
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "v") {
			continue
		}
		chartPath := path.Join(entry.Name(), "charts", "istiod")
		if _, err := fs.Stat(resources.FS, path.Join(chartPath, "Chart.yaml")); err != nil {
			continue
		}
		renderedVersions++

		t.Run(entry.Name(), func(t *testing.T) {
			scenarios := []struct {
				name     string
				revision string
			}{
				{name: "revision", revision: "openshift-gateway"},
				{name: "empty revision", revision: ""},
			}
			for _, scenario := range scenarios {
				t.Run(scenario.name, func(t *testing.T) {
					g := NewWithT(t)
					cm, err := renderGatewayClassConfigMap(chartPath, scenario.revision, gatewayClass)
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(cm.Name).To(Equal(GatewayClassDefaultsConfigMapName(scenario.revision, gatewayClass)))
					g.Expect(cm.Labels).To(HaveKeyWithValue(GatewayClassDefaultsLabel, gatewayClass))
				})
			}
		})
	}

	if renderedVersions == 0 {
		t.Fatal("no bundled istiod charts found to render")
	}
}

func renderGatewayClassConfigMap(chartPath, revision, gatewayClass string) (*corev1.ConfigMap, error) {
	values := helm.Values{
		"revision": revision,
		"gatewayClasses": map[string]any{
			gatewayClass: map[string]any{
				"service": map[string]any{
					"spec": map[string]any{"type": "ClusterIP"},
				},
			},
		},
	}

	rendered, err := helm.RenderChart(resources.FS, chartPath, values, "istio-system", "istiod")
	if err != nil {
		return nil, err
	}

	for name, content := range rendered {
		if !strings.HasSuffix(name, "gateway-class-configmap.yaml") || strings.TrimSpace(content) == "" {
			continue
		}
		cm := &corev1.ConfigMap{}
		if err := yaml.Unmarshal([]byte(content), cm); err != nil {
			return nil, fmt.Errorf("failed to parse rendered template %s: %w", name, err)
		}
		return cm, nil
	}

	return nil, fmt.Errorf("gateway-class ConfigMap not found in rendered chart; rendered templates: %v", maps.Keys(rendered))
}
