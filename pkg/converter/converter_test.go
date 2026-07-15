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

package converter

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/istio-ecosystem/sail-operator/pkg/test/project"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/shell"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

var (
	converter = filepath.Join(project.RootDir, "tools", "configuration-converter.sh")
	istioFile = filepath.Join(project.RootDir, "tools", "istioConfig.yaml")
	sailFile  = filepath.Join(project.RootDir, "tools", "istioConfig-sail.yaml")
)

func TestConversion(t *testing.T) {
	testcases := []struct {
		name           string
		input          string
		args           string
		expectedOutput string
	}{
		{
			name: "simple",
			input: `apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  name: default
spec:`,
			args: fmt.Sprintf("%s %s -n istio-system -v v1.24.3", istioFile, sailFile),
			expectedOutput: `apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  values:
    meshConfig:
      accessLogFile: /dev/stdout
  namespace: istio-system
  version: v1.24.3`,
		},
		{
			name: "complex",
			input: `apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  name: default
spec:
  components:
    base:
      enabled: true
    pilot:
      enabled: false
  values:
    global:
      externalIstiod: true
      operatorManageWebhooks: true
      configValidation: false
    base:
      enableCRDTemplates: true
    pilot:
      env:
        PILOT_ENABLE_STATUS: true`,
			args: fmt.Sprintf("-v v1.24.3 %s -n istio-system %s", istioFile, sailFile),
			expectedOutput: `apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  values:
    meshConfig:
        accessLogFile: /dev/stdout
    global:
      externalIstiod: true
      operatorManageWebhooks: true
      configValidation: false
    base:
      enableCRDTemplates: true
      enabled: true
    pilot:
      env:
        PILOT_ENABLE_STATUS: "true"
      enabled: false
  namespace: istio-system
  version: v1.24.3`,
		},
		{
			name: "mandatory-arguments-only",
			input: `apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  name: default
spec:`,
			args: istioFile,
			expectedOutput: `apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  values:
    meshConfig:
      accessLogFile: /dev/stdout
  namespace: istio-system`,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Cleanup(func() {
				g.Expect(os.Remove(istioFile)).To(Succeed())
				g.Expect(os.Remove(sailFile)).To(Succeed())
			})

			g.Expect(os.WriteFile(istioFile, []byte(tc.input), 0o644)).To(Succeed(), "failed to write YAML file")

			std, err := shell.ExecuteCommand(converter + " " + tc.args)
			g.Expect(err).NotTo(HaveOccurred(), "error in execution of ./configuration-converter.sh %s", std)

			actualOutput, err := os.ReadFile(sailFile)
			g.Expect(err).NotTo(HaveOccurred(), "Cannot read %s", sailFile)

			actualData, err := parseYaml(actualOutput)
			g.Expect(err).NotTo(HaveOccurred(), "Failed to parse sailFile")

			expectedData, err := parseYaml([]byte(tc.expectedOutput))
			g.Expect(err).NotTo(HaveOccurred(), "Failed to parse expected output")

			g.Expect(cmp.Diff(actualData, expectedData)).To(Equal(""), "Conversion is not as expected")
		})
	}
}

// parseYaml takes a YAML string and unmarshals it into a map
func parseYaml(yamlContent []byte) (map[string]any, error) {
	var config map[string]any
	err := yaml.Unmarshal(yamlContent, &config)
	return config, err
}
