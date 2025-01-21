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
)

//go:embed versions.yaml
var versionsFile embed.FS

// Versions represents the top-level structure of versions.yaml
type Versions struct {
	Versions []VersionInfo `json:"versions"`
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
	// List contains all supported versions
	List []VersionInfo
	// Map contains version info mapped by version name
	Map map[string]VersionInfo
	// Default is the default version
	Default string
	// Old is the previous supported version
	Old string
	// New is the latest supported version
	New string
)

func init() {
	// Read the embedded versions.yaml file
	data, err := versionsFile.ReadFile("versions.yaml")
	if err != nil {
		panic(fmt.Errorf("failed to read versions.yaml: %w", err))
	}

	List, Default, Old, New = mustParseVersionsYaml(data)

	Map = make(map[string]VersionInfo)
	for _, v := range List {
		Map[v.Name] = v
	}
}

func mustParseVersionsYaml(yamlBytes []byte) (list []VersionInfo, defaultVersion string, oldVersion string, newVersion string) {
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
	return list, defaultVersion, oldVersion, newVersion
}
