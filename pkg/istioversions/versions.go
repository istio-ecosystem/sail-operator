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
	"embed"
	"fmt"

	"github.com/Masterminds/semver/v3"
	"gopkg.in/yaml.v3"

	"istio.io/istio/pkg/log"
)

var (
	//go:embed *.yaml
	versionsFiles embed.FS

	versionsFilename = "versions.yaml"
)

// Versions represents the top-level structure of versions.yaml
type Versions struct {
	Versions []VersionInfo `json:"versions"`
	Alias    []AliasInfo   `json:"alias"`
}

// AliasInfo contains information about version aliases
type AliasInfo struct {
	Name string `json:"name"`
	Ref  string `json:"ref"`
}

// VersionInfo contains information about a specific Istio version
type VersionInfo struct {
	Name    string          `json:"name"`
	Version *semver.Version `json:"version"`
	Repo    string          `json:"repo"`
	Branch  string          `json:"branch,omitempty"`
	Commit  string          `json:"commit"`
	Charts  []string        `json:"charts,omitempty"`
}

var (
	// List contains all supported versions. Does not include aliases
	List []VersionInfo
	// Map contains version info mapped by version name. Includes mappings for aliases
	Map map[string]VersionInfo
	// Default is the default version
	Default string
	// Old is the previous supported version
	Old string
	// New is the latest supported version
	New string
	// Alias is the alias for the version
	Alias []AliasInfo
)

func ResolveVersionName(version string) (string, error) {
	info, ok := Map[version]
	if !ok {
		return "", fmt.Errorf("version %q not found", version)
	}
	return info.Name, nil
}

func init() {
	log.Info("loading supported istio versions from " + versionsFilename)
	// Read the embedded versions.yaml file
	data, err := versionsFiles.ReadFile(versionsFilename)
	if err != nil {
		panic(fmt.Errorf("failed to read versions from '%s': %w", versionsFilename, err))
	}

	List, Default, Old, New, Alias = mustParseVersionsYaml(data)

	Map = mustBuildVersionMap(Alias, List)
}

func mustBuildVersionMap(alias []AliasInfo, list []VersionInfo) map[string]VersionInfo {
	versionMap := make(map[string]VersionInfo)
	for _, v := range list {
		versionMap[v.Name] = v
	}
	for _, a := range alias {
		v, ok := (versionMap)[a.Ref]
		if !ok {
			panic(fmt.Errorf("version %q not found", a.Ref))
		}
		(versionMap)[a.Name] = v
	}

	return versionMap
}

func mustParseVersionsYaml(yamlBytes []byte) (list []VersionInfo, defaultVersion string, oldVersion string, newVersion string, alias []AliasInfo) {
	versions := Versions{}
	err := yaml.Unmarshal(yamlBytes, &versions)
	if err != nil {
		panic(fmt.Errorf("failed to parse versions data: %w", err))
	}

	list = versions.Versions
	defaultVersion = list[0].Name
	if len(list) > 1 {
		oldVersion = list[1].Name
	}
	newVersion = list[0].Name
	return list, defaultVersion, oldVersion, newVersion, versions.Alias
}
