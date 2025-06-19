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
	"embed"
	"fmt"
	"maps"
	"slices"
	"sort"

	"github.com/Masterminds/semver/v3"
	"github.com/istio-ecosystem/sail-operator/pkg/env"
	"gopkg.in/yaml.v3"

	"istio.io/istio/pkg/log"
)

var (
	//go:embed *.yaml
	versionsFiles embed.FS

	// versionsFilename is set via ldflags when building the binary and via an environment variable when running tests
	versionsFilename = ""
)

// Versions represents the top-level structure of versions.yaml
type Versions struct {
	Versions []VersionInfo `json:"versions"`
}

// AliasInfo contains information about version aliases
type AliasInfo struct {
	Name string `json:"name"`
	Ref  string `json:"ref"`
}

// VersionInfo contains information about a specific Istio version
type VersionInfo struct {
	Name    string          `json:"name"`
	Ref     string          `json:"ref"`
	Version *semver.Version `json:"version"`
	Repo    string          `json:"repo"`
	Branch  string          `json:"branch,omitempty"`
	Commit  string          `json:"commit"`
	Charts  []string        `json:"charts,omitempty"`
	EOL     bool            `json:"eol"`
}

var (
	// List contains all supported versions. Does not include aliases
	List []VersionInfo
	// Map contains version info mapped by version name. Includes mappings for aliases
	Map map[string]VersionInfo
	// Default is the default version
	Default string
	// Base is the previous supported version
	Base string
	// New is the latest version
	New string
	// aliasList is the alias for the version
	aliasList []AliasInfo
	// EOL is a list of version names that have gone out of support
	EOL []string
)

func Resolve(version string) (string, error) {
	info, ok := Map[version]
	if !ok {
		return "", fmt.Errorf("version %q not found", version)
	}
	return info.Name, nil
}

func init() {
	// we can't use ldflags when running tests, use an env variable instead
	// tmp workaround for https://github.com/golang/go/issues/64246
	if versionsFilename == "" {
		versionsFilename = env.Get("VERSIONS_YAML_FILE", "versions.yaml")
	}
	log.Info("loading supported istio versions from " + versionsFilename)
	// Read the embedded versions.yaml file
	data, err := versionsFiles.ReadFile(versionsFilename)
	if err != nil {
		panic(fmt.Errorf("failed to read versions from '%s': %w", versionsFilename, err))
	}

	List, Default, Base, New, Map, aliasList, EOL = mustParseVersionsYaml(data)
}

func mustParseVersionsYaml(yamlBytes []byte) (
	list []VersionInfo,
	defaultVersion string,
	baseVersion string,
	newVersion string,
	versionMap map[string]VersionInfo,
	aliasList []AliasInfo,
	eolVersions []string,
) {
	versions := Versions{}
	err := yaml.Unmarshal(yamlBytes, &versions)
	if err != nil {
		panic(fmt.Errorf("failed to parse versions data: %w", err))
	}

	versionMap = make(map[string]VersionInfo)

	for _, v := range versions.Versions {
		if v.EOL {
			eolVersions = append(eolVersions, v.Name)
			continue
		}
		if v.Ref == "" {
			list = append(list, v)
			versionMap[v.Name] = v
		} else {
			if v.Version != nil || v.Repo != "" || v.Commit != "" || v.Branch != "" || len(v.Charts) > 0 {
				panic(fmt.Errorf("version %q has aliasFor set but the other fields cannot be specified", v.Name))
			}
			aliasList = append(aliasList, AliasInfo{
				Name: v.Name,
				Ref:  v.Ref,
			})
		}
	}

	// Process aliases after all versions are in the lookup map
	for _, a := range aliasList {
		v, ok := versionMap[a.Ref]
		if !ok {
			panic(fmt.Errorf("version %q not found", a.Ref))
		}
		versionMap[a.Name] = v
	}

	if len(list) > 0 {
		newVersion = list[0].Name
		defaultVersion = newVersion
		if len(list) > 1 {
			baseVersion = list[1].Name
		}
	}

	return list, defaultVersion, baseVersion, newVersion, versionMap, aliasList, eolVersions
}

// GetLatestPatchVersions returns the latest patch versions for all the Major.Minor versions
func GetLatestPatchVersions() []VersionInfo {
	latestPatchVersions := make(map[string]VersionInfo)
	for _, version := range List {
		majorMinorVersion := fmt.Sprintf("%d.%d", version.Version.Major(), version.Version.Minor())
		latestPatchVersion, ok := latestPatchVersions[majorMinorVersion]
		if !ok || version.Version.GreaterThan(latestPatchVersion.Version) {
			latestPatchVersions[majorMinorVersion] = version
		}
	}

	latestSlice := slices.Collect(maps.Values(latestPatchVersions))

	// Sort the slice in descending order based on the version.
	sort.Slice(latestSlice, func(i, j int) bool {
		return latestSlice[i].Version.GreaterThan(latestSlice[j].Version)
	})

	return latestSlice
}
