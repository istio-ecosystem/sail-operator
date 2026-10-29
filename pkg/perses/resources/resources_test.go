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

package resources

import (
	"io/fs"
	"slices"
	"strings"
	"testing"
)

func TestEmbeddedDashboardsFS(t *testing.T) {
	// Dashboard metadata.name values must remain stable for Kiali and other consumers.
	want := []string{
		"istio-control-plane-dashboard.yaml",
		"istio-mesh-dashboard.yaml",
		"istio-performance-dashboard.yaml",
		"istio-service-dashboard.yaml",
		"istio-workload-dashboard.yaml",
		"istio-ztunnel-dashboard.yaml",
		"istio-wasm-extension-dashboard.yaml",
	}

	entries, err := fs.ReadDir(ChartFS, "files")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	var yamlFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
			yamlFiles = append(yamlFiles, entry.Name())
		}
	}

	slices.Sort(want)
	slices.Sort(yamlFiles)

	if !slices.Equal(yamlFiles, want) {
		t.Fatalf("embedded dashboard files = %v, want %v", yamlFiles, want)
	}
}
