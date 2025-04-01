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

package istioversion

import (
	"slices"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/assert"
)

func TestInit(t *testing.T) {
	// no need to call init(), since it's called automatically
	assert.True(t, len(List) > 0, "istioversions.List should not be empty")
	assert.True(t, len(Map) > 0, "istioversions.Map should not be empty")
	assert.True(t, Default != "", "istioversions.Default should not be empty")

	assert.Equal(t, len(List)+len(aliasList), len(Map), "Map should have at least as many entries as List + Aliases")
	for _, vi := range List {
		assert.Equal(t, vi, Map[vi.Name])
	}
	for _, ai := range aliasList {
		assert.Equal(t,
			List[slices.IndexFunc(List, func(v VersionInfo) bool { return v.Name == ai.Ref })],
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
    ref: v2.0.0
  - name: "v2.0.0"
    version: 2.0.0
    repo: "repo2"
    commit: "commit2"
`)

	list, defaultVersion, versionMap, aliasList := mustParseVersionsYaml(yamlBytes)

	assert.Len(t, list, 2)
	assert.Equal(t, "v1.0.0", defaultVersion)

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
	resolved, err := Resolve("latest")
	assert.NoError(t, err)
	assert.Equal(t, "v2.0.0", resolved)

	resolved, err = Resolve("nonexistent-version")
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

	list, defaultVersion, versionMap, aliasList := mustParseVersionsYaml(yamlBytes)

	assert.Len(t, list, 1)
	assert.Equal(t, "v1.0.0", defaultVersion)
	assert.Len(t, versionMap, 1)
	assert.Len(t, aliasList, 0)
	assert.Equal(t, list[0], versionMap[list[0].Name])
}

func TestParseVersionsYaml_InvalidAlias(t *testing.T) {
	yamlBytes := []byte(`
versions:
  - name: "1.0-latest"
    ref: v1.0.0
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
    ref: v2.0.0
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

func TestGetLatestPatchVersions_Valid(t *testing.T) {
	List = []VersionInfo{
		{Name: "1.24.2", Version: semver.MustParse("1.24.2")},
		{Name: "1.24.1", Version: semver.MustParse("1.24.1")},
		{Name: "1.23", Version: semver.MustParse("1.23")},
		{Name: "1.23.2", Version: semver.MustParse("1.23.2")},
		{Name: "1.23.1", Version: semver.MustParse("1.23.1")},
		{Name: "1.22.0", Version: semver.MustParse("1.22.0")},
	}

	versions := GetLatestPatchVersions()

	expected := map[string]VersionInfo{
		"1.24": {Name: "1.24.2", Version: semver.MustParse("1.24.2")},
		"1.23": {Name: "1.23.2", Version: semver.MustParse("1.23.2")},
		"1.22": {Name: "1.22.0", Version: semver.MustParse("1.22.0")},
	}

	if diff := cmp.Diff(expected, versions); diff != "" {
		t.Errorf("unexpected result; diff (-expected, +actual):\n%v", diff)
	}
}

func TestGetLatestPatchVersions_EmptyList(t *testing.T) {
	List = []VersionInfo{}

	versions := GetLatestPatchVersions()

	assert.Len(t, versions, 0)
}

func TestGetPatchVersionsByMajorMinor_ValidVersionInfo(t *testing.T) {
	List = []VersionInfo{
		{Name: "1.24.2", Version: semver.MustParse("1.24.2")},
		{Name: "1.24.1", Version: semver.MustParse("1.24.1")},
		{Name: "1.23", Version: semver.MustParse("1.23")},
		{Name: "1.23.2", Version: semver.MustParse("1.23.2")},
		{Name: "1.23.1", Version: semver.MustParse("1.23.1")},
		{Name: "1.22.0", Version: semver.MustParse("1.22.0")},
	}

	groups := getPatchVersionsByMajorMinor()

	assert.Len(t, groups, 3)
	assert.Contains(t, groups, "1.24")
	assert.Len(t, groups["1.24"], 2)
	assert.Contains(t, groups, "1.23")
	assert.Len(t, groups["1.23"], 3)
	assert.Contains(t, groups, "1.22")
	assert.Len(t, groups["1.22"], 1)
}

func TestGetPatchVersionsByMajorMinor_EmptyList(t *testing.T) {
	List = []VersionInfo{}

	groups := getPatchVersionsByMajorMinor()

	assert.Len(t, groups, 0)
}

func TestGetBaseAndNewVersion_ValidList(t *testing.T) {
	List = []VersionInfo{
		{Name: "1.24.2", Version: semver.MustParse("1.24.2")},
		{Name: "1.24.1", Version: semver.MustParse("1.24.1")},
		{Name: "1.23", Version: semver.MustParse("1.23")},
		{Name: "1.23.2", Version: semver.MustParse("1.23.2")},
		{Name: "1.23.1", Version: semver.MustParse("1.23.1")},
		{Name: "1.22.0", Version: semver.MustParse("1.22.0")},
	}

	base, newVersion := GetBaseAndNewVersion()

	assert.Equal(t, "1.24.1", base)
	assert.Equal(t, "1.24.2", newVersion)
}

func TestGetBaseAndNewVersion_EmptyList(t *testing.T) {
	List = []VersionInfo{}

	base, newVersion := GetBaseAndNewVersion()

	assert.Equal(t, "", base)
	assert.Equal(t, "", newVersion)
}

func TestGetBaseAndNewVersion_NoValidVersions(t *testing.T) {
	List = []VersionInfo{
		{Name: "1.24.2", Version: semver.MustParse("1.24.2")},
		{Name: "1.23.2", Version: semver.MustParse("1.23.2")},
		{Name: "1.22.0", Version: semver.MustParse("1.22.0")},
	}

	base, newVersion := GetBaseAndNewVersion()

	// base and newVersion should be empty since there are no consecutive patch versions in the List for same Minor version
	assert.Equal(t, "", base)
	assert.Equal(t, "", newVersion)
}
