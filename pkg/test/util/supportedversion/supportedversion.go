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

package supportedversion

import (
	"os"
	"path/filepath"

	"github.com/Masterminds/semver/v3"
	"github.com/istio-ecosystem/sail-operator/pkg/test/project"
	"gopkg.in/yaml.v3"
)

var (
	List    []VersionInfo
	Map     map[string]VersionInfo
	Default string
	Old     string
	New     string
)

func init() {
	versionsFile := os.Getenv("VERSIONS_YAML_FILE")
	if len(versionsFile) == 0 {
		versionsFile = "versions.yaml"
	}
	versionsFile = filepath.Join(project.RootDir, versionsFile)

	versionsBytes, err := os.ReadFile(versionsFile)
	if err != nil {
		panic(err)
	}

	List, Default, Old, New = mustParseVersionsYaml(versionsBytes)

	Map = make(map[string]VersionInfo)
	for _, v := range List {
		Map[v.Name] = v
	}
}

func mustParseVersionsYaml(yamlBytes []byte) (list []VersionInfo, defaultVersion string, oldVersion string, newVersion string) {
	versions := Versions{}
	err := yaml.Unmarshal(yamlBytes, &versions)
	if err != nil {
		panic(err)
	}

	list = versions.Versions
	defaultVersion = list[0].Name
	if len(list) > 1 {
		oldVersion = list[1].Name
	}
	newVersion = list[0].Name
	return list, defaultVersion, oldVersion, newVersion
}

type Versions struct {
	Versions []VersionInfo `json:"versions"`
}

type VersionInfo struct {
	Name    string          `json:"name"`
	Version *semver.Version `json:"version"`
	Repo    string          `json:"repo"`
	Branch  string          `json:"branch,omitempty"`
	Commit  string          `json:"commit"`
	Charts  []string        `json:"charts,omitempty"`
}
