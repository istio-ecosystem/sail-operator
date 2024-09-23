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
	"regexp"
	"strconv"

	"github.com/istio-ecosystem/sail-operator/pkg/test/project"
	"gopkg.in/yaml.v3"
)

var (
	List    []VersionInfo
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

	versions := Versions{}
	err = yaml.Unmarshal(versionsBytes, &versions)
	if err != nil {
		panic(err)
	}

	// Major, Minor and Patch needs to be set from parsing the version string
	for i := range versions.Versions {
		v := &versions.Versions[i]
		v.Major, v.Minor, v.Patch = parseVersion(v.Version)
	}

	List = versions.Versions
	Default = List[0].Name
	if len(List) > 1 {
		Old = List[1].Name
	}
	New = List[0].Name
}

func parseVersion(version string) (int, int, int) {
	// The version can have this formats: "1.22.2", "1.23.0-rc.1", "1.24-alpha"
	re := regexp.MustCompile(`^(\d+)\.(\d+)\.?(\d*)`)

	matches := re.FindStringSubmatch(version)
	if len(matches) < 4 {
		return 0, 0, 0
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])

	return major, minor, patch
}

type Versions struct {
	Versions []VersionInfo `json:"versions"`
}

type VersionInfo struct {
	Name    string   `json:"name"`
	Version string   `json:"version"`
	Major   int      `json:"major"`
	Minor   int      `json:"minor"`
	Patch   int      `json:"patch"`
	Repo    string   `json:"repo"`
	Branch  string   `json:"branch,omitempty"`
	Commit  string   `json:"commit"`
	Charts  []string `json:"charts,omitempty"`
}
