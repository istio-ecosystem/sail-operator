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

package istioversions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInit(t *testing.T) {
	// no need to call init(), since it's called automatically
	assert.True(t, len(List) > 0, "istioversions.List should not be empty")
	assert.True(t, len(Map) > 0, "istioversions.Map should not be empty")
	assert.True(t, Default != "", "istioversions.Default should not be empty")
	assert.True(t, Old != "", "istioversions.Old should not be empty")
	assert.True(t, New != "", "istioversions.New should not be empty")

	assert.Equal(t, len(List), len(Map), "Map should be same size as List")
	for _, vi := range List {
		assert.Equal(t, vi, Map[vi.Name])
	}
}

func TestParseVersionsYaml_ValidYaml(t *testing.T) {
	yamlBytes := []byte(`
alias:
  - name: "latest"
    ref: "2.0.0"
versions:
  - name: "1.0.0"
    repo: "repo1"
    commit: "commit1"
  - name: "2.0.0"
    repo: "repo2"
    commit: "commit2"
`)

	list, defaultVersion, oldVersion, newVersion, alias := mustParseVersionsYaml(yamlBytes)

	assert.Len(t, list, 2)
	assert.Equal(t, "1.0.0", defaultVersion)
	assert.Equal(t, "2.0.0", oldVersion)
	assert.Equal(t, "1.0.0", newVersion)
	assert.Len(t, alias, 1)
	assert.Equal(t, "latest", alias[0].Name)
	assert.Equal(t, "2.0.0", alias[0].Ref)
}

func TestParseVersionsYaml_SingleVersion(t *testing.T) {
	yamlBytes := []byte(`
versions:
  - name: "1.0.0"
    repo: "repo1"
    commit: "commit1"
`)

	list, defaultVersion, oldVersion, newVersion, alias := mustParseVersionsYaml(yamlBytes)

	assert.Len(t, list, 1)
	assert.Equal(t, "1.0.0", defaultVersion)
	assert.Equal(t, "", oldVersion)
	assert.Equal(t, "1.0.0", newVersion)
	assert.Len(t, alias, 0)
}

func TestParseVersionsYaml_InvalidAlias(t *testing.T) {
	yamlBytes := []byte(`
alias:
  - name: "1.0-latest"
    ref: "nonexistent-version"
versions:
  - name: "1.0.0"
    repo: "repo1"
    commit: "commit1"
`)

	assert.Panics(t, func() {
		list, _, _, _, alias := mustParseVersionsYaml(yamlBytes)
		mustBuildVersionMap(alias, list)
	})
}

func TestParseVersionsYaml_InvalidYaml(t *testing.T) {
	yamlBytes := []byte(`invalid yaml`)

	assert.Panics(t, func() {
		mustParseVersionsYaml(yamlBytes)
	})
}
