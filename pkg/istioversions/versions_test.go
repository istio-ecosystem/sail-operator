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
	"fmt"
	"slices"
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

	assert.Equal(t, len(List)+len(AliasList), len(Map), "Map should have at least as many entries as List + Aliases")
	for _, vi := range List {
		assert.Equal(t, vi, Map[vi.Name])
	}
	for _, ai := range AliasList {
		assert.Equal(t,
			List[slices.IndexFunc(List, func(v VersionInfo) bool { return fmt.Sprintf("v%s", v.Version.String()) == ai.Ref })],
			Map[ai.Name])
	}
}

func TestParseVersionsYaml_ValidYaml(t *testing.T) {
	yamlBytes := []byte(`
versions:
  - name: "v1.0.0"
    version: 1.0.0
    repo: "repo1"
    commit: "commit1"
  - name: "latest"
    ref: 2.0.0
  - name: "v2.0.0"
    version: 2.0.0
    repo: "repo2"
    commit: "commit2"
`)

	list, defaultVersion, oldVersion, newVersion, versionMap, aliasList := mustParseVersionsYaml(yamlBytes)

	assert.Len(t, list, 2)
	assert.Equal(t, "v1.0.0", defaultVersion)
	assert.Equal(t, "v2.0.0", oldVersion)
	assert.Equal(t, "v1.0.0", newVersion)

	// Test version map
	assert.Len(t, versionMap, 3) // 2 versions + 1 alias
	assert.Equal(t, "v2.0.0", versionMap["latest"].Name)
	assert.Equal(t, list[0], versionMap[list[0].Name])
	assert.Equal(t, list[1], versionMap[list[1].Name])

	// Test alias list
	assert.Len(t, aliasList, 1)
	assert.Equal(t, "latest", aliasList[0].Name)
	assert.Equal(t, "v2.0.0", aliasList[0].Ref)

	Map = versionMap
	resolved, err := ResolveVersion("latest")
	assert.NoError(t, err)
	assert.Equal(t, "v2.0.0", resolved)

	resolved, err = ResolveVersion("nonexistent-version")
	assert.Error(t, err)
	assert.Equal(t, "", resolved)
}

func TestParseVersionsYaml_SingleVersion(t *testing.T) {
	yamlBytes := []byte(`
versions:
  - name: "v1.0.0"
    version: 1.0.0
    repo: "repo1"
    commit: "commit1"
`)

	list, defaultVersion, oldVersion, newVersion, versionMap, aliasList := mustParseVersionsYaml(yamlBytes)

	assert.Len(t, list, 1)
	assert.Equal(t, "v1.0.0", defaultVersion)
	assert.Equal(t, "", oldVersion)
	assert.Equal(t, "v1.0.0", newVersion)
	assert.Len(t, versionMap, 1)
	assert.Len(t, aliasList, 0)
	assert.Equal(t, list[0], versionMap[list[0].Name])
}

func TestParseVersionsYaml_InvalidAlias(t *testing.T) {
	yamlBytes := []byte(`
versions:
  - name: "1.0-latest"
    ref: 1.0.0
  - name: "v2.0.0"
    version: 2.0.0
    repo: "repo1"
    commit: "commit1"
`)

	assert.Panics(t, func() {
		mustParseVersionsYaml(yamlBytes)
	})
}

func TestParseVersionsYaml_InvalidAliasWithOtherFields(t *testing.T) {
	yamlBytes := []byte(`
versions:
  - name: "2.0-latest"
    ref: 2.0.0
    repo: "should-not-have-this"
  - name: "v2.0.0"
    version: 2.0.0
    repo: "repo1"
    commit: "commit1"
`)

	assert.Panics(t, func() {
		mustParseVersionsYaml(yamlBytes)
	})
}

func TestParseVersionsYaml_InvalidYaml(t *testing.T) {
	yamlBytes := []byte(`invalid yaml`)

	assert.Panics(t, func() {
		mustParseVersionsYaml(yamlBytes)
	})
}
