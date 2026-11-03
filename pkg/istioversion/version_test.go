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
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
)

func TestInit(t *testing.T) {
	// no need to call init(), since it's called automatically
	assert.True(t, len(supportedVersions) > 0, "istioversions.supportedVersions should not be empty")
	assert.True(t, len(versionsByName) > 0, "istioversions.versionsByName should not be empty")
	assert.True(t, Default != "", "istioversions.Default should not be empty")
	assert.True(t, Base != "", "istioversions.Old should not be empty")
	assert.True(t, New != "", "istioversions.New should not be empty")

	assert.Equal(t, len(supportedVersions)+len(aliasList), len(versionsByName))
	for _, vi := range supportedVersions {
		assert.Equal(t, vi, versionsByName[vi.Name])
	}
	for _, ai := range aliasList {
		assert.Equal(t,
			supportedVersions[slices.IndexFunc(supportedVersions, func(v VersionInfo) bool { return v.Name == ai.Ref })],
			versionsByName[ai.Name])
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
  - name: "v0.1"
    eol: true
`)

	list, defaultVersion, baseVersion, newVersion, versionMap, aliasList, eolVersions := mustParseVersionsYaml(yamlBytes)

	assert.Len(t, list, 2)
	assert.Equal(t, "v1.0.0", defaultVersion)
	assert.Equal(t, "v2.0.0", baseVersion)
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

	// Test eolVersions list
	assert.Len(t, eolVersions, 1)
	assert.Equal(t, "v0.1", eolVersions[0])

	versionsByName = versionMap
	eolVersionNames = eolVersions
	resolved, err := Resolve("latest")
	assert.NoError(t, err)
	assert.Equal(t, "v2.0.0", resolved)

	resolved, err = Resolve("nonexistent-version")
	assert.Error(t, err)
	assert.Equal(t, "", resolved)

	assert.True(t, IsEOLVersion("v0.1"))
	assert.False(t, IsEOLVersion("nonexistent-version"))
	assert.False(t, IsEOLVersion("v1.0.0"))
}

func TestParseVersionsYaml_SingleVersion(t *testing.T) {
	yamlBytes := []byte(`
versions:
  - name: "v1.0.0"
    version: 1.0.0
    repo: "repo1"
    commit: "commit1"
`)

	list, defaultVersion, baseVersion, newVersion, versionMap, aliasList, eolVersions := mustParseVersionsYaml(yamlBytes)

	assert.Len(t, list, 1)
	assert.Equal(t, "v1.0.0", defaultVersion)
	assert.Equal(t, "", baseVersion)
	assert.Equal(t, "v1.0.0", newVersion)
	assert.Len(t, versionMap, 1)
	assert.Len(t, aliasList, 0)
	assert.Len(t, eolVersions, 0)
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

func TestDefaultVersion(t *testing.T) {
	assert.Equal(t, Default, DefaultVersion())
	assert.NotEmpty(t, DefaultVersion())
}

func TestValidateVersion(t *testing.T) {
	savedMap := versionsByName
	savedEOL := eolVersionNames
	defer func() { versionsByName = savedMap; eolVersionNames = savedEOL }()

	versionsByName = map[string]VersionInfo{
		"v1.0.0": {Name: "v1.0.0"},
	}
	eolVersionNames = []string{"v0.9.0"}

	assert.NoError(t, ValidateVersion("v1.0.0"))
	assert.Error(t, ValidateVersion(""))
	assert.Error(t, ValidateVersion("nonexistent-version"))

	err := ValidateVersion("v0.9.0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "end-of-life")
}

func TestGetLatestPatchVersions_Valid(t *testing.T) {
	t.Run("valid versions", func(t *testing.T) {
		supportedVersions = []VersionInfo{
			{Name: "master", Version: semver.MustParse("v1.25-alpha.c2ac935c")},
			{Name: "v1.24-latest", Version: semver.MustParse("1.24.2")},
			{Name: "v1.24.1", Version: semver.MustParse("1.24.1")},
			{Name: "v1.23", Version: semver.MustParse("1.23")},
			{Name: "v1.23.2", Version: semver.MustParse("1.23.2")},
			{Name: "v1.23.1", Version: semver.MustParse("1.23.1")},
			{Name: "v1.22.0", Version: semver.MustParse("1.22.0")},
		}

		versions := GetLatestPatchVersions()

		expected := []VersionInfo{
			{Name: "master", Version: semver.MustParse("v1.25-alpha.c2ac935c")},
			{Name: "v1.24-latest", Version: semver.MustParse("1.24.2")},
			{Name: "v1.23.2", Version: semver.MustParse("1.23.2")},
			{Name: "v1.22.0", Version: semver.MustParse("1.22.0")},
		}

		if diff := cmp.Diff(expected, versions); diff != "" {
			t.Errorf("unexpected result; diff (-expected, +actual):\n%v", diff)
		}
	})

	t.Run("empty list", func(t *testing.T) {
		supportedVersions = []VersionInfo{}

		versions := GetLatestPatchVersions()

		assert.Len(t, versions, 0)
	})
}

func TestGetTwoConsecutiveMinorVersions(t *testing.T) {
	t.Run("valid consecutive versions", func(t *testing.T) {
		supportedVersions = []VersionInfo{
			{Name: "v1.25.0", Version: semver.MustParse("1.25.0")},
			{Name: "v1.24.2", Version: semver.MustParse("1.24.2")},
			{Name: "v1.24.1", Version: semver.MustParse("1.24.1")},
			{Name: "v1.23.5", Version: semver.MustParse("1.23.5")},
			{Name: "v1.23.4", Version: semver.MustParse("1.23.4")},
		}

		baseVer, newVer, err := GetTwoConsecutiveMinorVersions(Sidecar)

		assert.NoError(t, err)
		assert.Equal(t, "v1.24.2", baseVer.Name)
		assert.Equal(t, "v1.25.0", newVer.Name)
	})

	t.Run("ambient mode filters out versions below 1.24", func(t *testing.T) {
		t.Setenv("FIPS_CLUSTER", "false")
		supportedVersions = []VersionInfo{
			{Name: "v1.24.1", Version: semver.MustParse("1.24.1")},
			{Name: "v1.23.5", Version: semver.MustParse("1.23.5")},
			{Name: "v1.22.0", Version: semver.MustParse("1.22.0")},
		}

		baseVer, newVer, err := GetTwoConsecutiveMinorVersions(Sidecar)
		assert.NoError(t, err)
		assert.Equal(t, "v1.23.5", baseVer.Name)
		assert.Equal(t, "v1.24.1", newVer.Name)

		_, _, err = GetTwoConsecutiveMinorVersions(Ambient)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient versions available")
	})

	t.Run("ambient mode on a FIPS cluster filters out versions below 1.28", func(t *testing.T) {
		supportedVersions = []VersionInfo{
			{Name: "v1.28.0", Version: semver.MustParse("1.28.0")},
			{Name: "v1.27.0", Version: semver.MustParse("1.27.0")},
			{Name: "v1.26.0", Version: semver.MustParse("1.26.0")},
		}

		t.Setenv("FIPS_CLUSTER", "false")
		baseVer, newVer, err := GetTwoConsecutiveMinorVersions(Ambient)
		assert.NoError(t, err)
		assert.Equal(t, "v1.27.0", baseVer.Name)
		assert.Equal(t, "v1.28.0", newVer.Name)

		t.Setenv("FIPS_CLUSTER", "true")
		_, _, err = GetTwoConsecutiveMinorVersions(Ambient)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient versions available")
	})

	t.Run("empty list returns error", func(t *testing.T) {
		supportedVersions = []VersionInfo{}

		_, _, err := GetTwoConsecutiveMinorVersions(Sidecar)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient versions available")
	})

	t.Run("only one version returns error", func(t *testing.T) {
		supportedVersions = []VersionInfo{
			{Name: "v1.25.0", Version: semver.MustParse("1.25.0")},
			{Name: "v1.25.1", Version: semver.MustParse("1.25.1")},
		}

		_, _, err := GetTwoConsecutiveMinorVersions(Sidecar)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient versions available")
	})
}

func TestGetOldestAndNewestMinorVersions(t *testing.T) {
	t.Run("skips over the minors in between", func(t *testing.T) {
		supportedVersions = []VersionInfo{
			{Name: "v1.25.0", Version: semver.MustParse("1.25.0")},
			{Name: "v1.24.2", Version: semver.MustParse("1.24.2")},
			{Name: "v1.24.1", Version: semver.MustParse("1.24.1")},
			{Name: "v1.23.5", Version: semver.MustParse("1.23.5")},
			{Name: "v1.22.0", Version: semver.MustParse("1.22.0")},
		}

		oldestVer, newestVer, err := GetOldestAndNewestMinorVersions(Sidecar)

		assert.NoError(t, err)
		assert.Equal(t, "v1.22.0", oldestVer.Name)
		assert.Equal(t, "v1.25.0", newestVer.Name)
	})

	t.Run("ambient mode raises the oldest version to 1.24", func(t *testing.T) {
		t.Setenv("FIPS_CLUSTER", "false")
		supportedVersions = []VersionInfo{
			{Name: "v1.25.0", Version: semver.MustParse("1.25.0")},
			{Name: "v1.24.2", Version: semver.MustParse("1.24.2")},
			{Name: "v1.23.5", Version: semver.MustParse("1.23.5")},
			{Name: "v1.22.0", Version: semver.MustParse("1.22.0")},
		}

		oldestVer, newestVer, err := GetOldestAndNewestMinorVersions(Ambient)

		assert.NoError(t, err)
		assert.Equal(t, "v1.24.2", oldestVer.Name)
		assert.Equal(t, "v1.25.0", newestVer.Name)
	})

	t.Run("ambient mode on a FIPS cluster raises the oldest version to 1.28", func(t *testing.T) {
		t.Setenv("FIPS_CLUSTER", "true")
		supportedVersions = []VersionInfo{
			{Name: "v1.29.0", Version: semver.MustParse("1.29.0")},
			{Name: "v1.28.0", Version: semver.MustParse("1.28.0")},
			{Name: "v1.27.0", Version: semver.MustParse("1.27.0")},
			{Name: "v1.26.0", Version: semver.MustParse("1.26.0")},
		}

		oldestVer, newestVer, err := GetOldestAndNewestMinorVersions(Ambient)

		assert.NoError(t, err)
		assert.Equal(t, "v1.28.0", oldestVer.Name)
		assert.Equal(t, "v1.29.0", newestVer.Name)
	})

	t.Run("two minors are returned even though they are consecutive", func(t *testing.T) {
		supportedVersions = []VersionInfo{
			{Name: "v1.25.0", Version: semver.MustParse("1.25.0")},
			{Name: "v1.24.2", Version: semver.MustParse("1.24.2")},
		}

		oldestVer, newestVer, err := GetOldestAndNewestMinorVersions(Sidecar)

		assert.NoError(t, err)
		assert.Equal(t, "v1.24.2", oldestVer.Name)
		assert.Equal(t, "v1.25.0", newestVer.Name)
	})

	t.Run("single minor returns error", func(t *testing.T) {
		supportedVersions = []VersionInfo{
			{Name: "v1.25.1", Version: semver.MustParse("1.25.1")},
			{Name: "v1.25.0", Version: semver.MustParse("1.25.0")},
		}

		_, _, err := GetOldestAndNewestMinorVersions(Sidecar)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient versions available")
	})

	t.Run("empty list returns error", func(t *testing.T) {
		supportedVersions = []VersionInfo{}

		_, _, err := GetOldestAndNewestMinorVersions(Sidecar)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient versions available")
	})
}

func TestGetLatestAmbientVersion(t *testing.T) {
	t.Run("returns latest version >= 1.24", func(t *testing.T) {
		supportedVersions = []VersionInfo{
			{Name: "v1.26.0", Version: semver.MustParse("1.26.0")},
			{Name: "v1.25.0", Version: semver.MustParse("1.25.0")},
			{Name: "v1.24.2", Version: semver.MustParse("1.24.2")},
			{Name: "v1.23.5", Version: semver.MustParse("1.23.5")},
		}
		t.Setenv("FIPS_CLUSTER", "false")

		version := GetLatestAmbientVersion()

		assert.Equal(t, "v1.26.0", version.Name)
	})

	t.Run("skips versions < 1.24", func(t *testing.T) {
		supportedVersions = []VersionInfo{
			{Name: "v1.25.0", Version: semver.MustParse("1.25.0")},
			{Name: "v1.23.5", Version: semver.MustParse("1.23.5")},
			{Name: "v1.22.0", Version: semver.MustParse("1.22.0")},
		}
		t.Setenv("FIPS_CLUSTER", "false")

		version := GetLatestAmbientVersion()

		assert.Equal(t, "v1.25.0", version.Name)
	})

	t.Run("FIPS cluster requires >= 1.28", func(t *testing.T) {
		supportedVersions = []VersionInfo{
			{Name: "v1.29.0", Version: semver.MustParse("1.29.0")},
			{Name: "v1.28.2", Version: semver.MustParse("1.28.2")},
			{Name: "v1.27.5", Version: semver.MustParse("1.27.5")},
			{Name: "v1.26.0", Version: semver.MustParse("1.26.0")},
		}
		t.Setenv("FIPS_CLUSTER", "true")

		version := GetLatestAmbientVersion()

		assert.Equal(t, "v1.29.0", version.Name)
	})

	t.Run("FIPS cluster skips versions < 1.28", func(t *testing.T) {
		supportedVersions = []VersionInfo{
			{Name: "v1.28.0", Version: semver.MustParse("1.28.0")},
			{Name: "v1.27.5", Version: semver.MustParse("1.27.5")},
			{Name: "v1.26.0", Version: semver.MustParse("1.26.0")},
		}
		t.Setenv("FIPS_CLUSTER", "true")

		version := GetLatestAmbientVersion()

		assert.Equal(t, "v1.28.0", version.Name)
	})

	t.Run("returns last version when all < 1.24", func(t *testing.T) {
		supportedVersions = []VersionInfo{
			{Name: "v1.23.5", Version: semver.MustParse("1.23.5")},
			{Name: "v1.22.0", Version: semver.MustParse("1.22.0")},
			{Name: "v1.21.0", Version: semver.MustParse("1.21.0")},
		}
		t.Setenv("FIPS_CLUSTER", "false")

		version := GetLatestAmbientVersion()

		assert.Equal(t, "v1.21.0", version.Name)
	})

	t.Run("FIPS returns last version when all < 1.28", func(t *testing.T) {
		supportedVersions = []VersionInfo{
			{Name: "v1.27.5", Version: semver.MustParse("1.27.5")},
			{Name: "v1.26.0", Version: semver.MustParse("1.26.0")},
			{Name: "v1.25.0", Version: semver.MustParse("1.25.0")},
		}
		t.Setenv("FIPS_CLUSTER", "true")

		version := GetLatestAmbientVersion()

		assert.Equal(t, "v1.25.0", version.Name)
	})

	t.Run("handles multiple patch versions", func(t *testing.T) {
		supportedVersions = []VersionInfo{
			{Name: "v1.25.3", Version: semver.MustParse("1.25.3")},
			{Name: "v1.25.2", Version: semver.MustParse("1.25.2")},
			{Name: "v1.24.5", Version: semver.MustParse("1.24.5")},
			{Name: "v1.24.4", Version: semver.MustParse("1.24.4")},
		}
		t.Setenv("FIPS_CLUSTER", "false")

		version := GetLatestAmbientVersion()

		// Should return latest patch of latest minor version
		assert.Equal(t, "v1.25.3", version.Name)
	})
}

func TestVersionAccessorsReturnCopies(t *testing.T) {
	savedList := supportedVersions
	savedMap := versionsByName
	savedEOL := eolVersionNames
	t.Cleanup(func() {
		supportedVersions = savedList
		versionsByName = savedMap
		eolVersionNames = savedEOL
	})

	supportedVersions = []VersionInfo{{Name: "v1.2.3", Charts: []string{"chart-a"}}}
	versionsByName = map[string]VersionInfo{
		"v1.2.3": {Name: "v1.2.3", Charts: []string{"chart-a"}},
	}
	eolVersionNames = []string{"v0.1.0"}

	versions := SupportedVersions()
	versions[0].Name = "changed"
	versions[0].Charts[0] = "changed"
	appended := append(versions, VersionInfo{Name: "extra"})

	gotVersions := SupportedVersions()
	assert.Len(t, appended, 2)
	assert.Equal(t, []VersionInfo{{Name: "v1.2.3", Charts: []string{"chart-a"}}}, gotVersions)

	byName := VersionsByName()
	info := byName["v1.2.3"]
	info.Name = "changed"
	info.Charts[0] = "changed"
	byName["v1.2.3"] = info
	byName["extra"] = VersionInfo{Name: "extra"}

	got := VersionsByName()["v1.2.3"]
	assert.Equal(t, "v1.2.3", got.Name)
	assert.Equal(t, []string{"chart-a"}, got.Charts)
	_, exists := VersionsByName()["extra"]
	assert.False(t, exists)

	eol := EOLVersions()
	eol[0] = "changed"
	appendedEOL := append(eol, "extra")
	assert.Len(t, appendedEOL, 2)
	assert.Equal(t, []string{"v0.1.0"}, EOLVersions())
	assert.True(t, IsEOLVersion("v0.1.0"))
	assert.False(t, IsEOLVersion("changed"))
	assert.False(t, IsEOLVersion("extra"))
}
