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
	"strings"
	"testing"
)

func TestEmbeddedDashboardsFS(t *testing.T) {
	entries, err := fs.ReadDir(ChartFS, "files")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	yamlCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
			yamlCount++
		}
	}
	if yamlCount == 0 {
		t.Fatal("expected embedded dashboard YAML files")
	}
}
