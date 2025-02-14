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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/istio-ecosystem/sail-operator/pkg/test/project"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/shell"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

var (
	istioFile = filepath.Join(project.RootDir, "tools", "istioConfig.yaml")
	sailFile  = filepath.Join(project.RootDir, "tools", "istioConfig-sail.yaml")
)

func TestConversion(t *testing.T) {
	RegisterTestingT(t)
	g := NewWithT(t)
	testcases := []struct {
		name           string
		input          string
		expectedOutput string
		converterArgs  string
	}{
		{
			name: "simple",
			input: `apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  name: default
spec:`,
			converterArgs: fmt.Sprintf("%s %s -n istio-system -v v1.24.3", istioFile, sailFile),
			expectedOutput: `apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  version: v1.24.3
  namespace: istio-system`,
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
			converterArgs: fmt.Sprintf("%s %s -v v1.24.3 -n istio-system", istioFile, sailFile),
			expectedOutput: `apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  values:
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
			converterArgs: istioFile,
			expectedOutput: `apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  namespace: istio-system`,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(func() {
				os.Remove(istioFile)
				os.Remove(sailFile)
			})
			g.Expect(os.WriteFile(istioFile, []byte(tc.input), 0o644)).To(Succeed(), "failed to write YAML file")

			g.Expect(executeConfigConverter(tc.converterArgs)).To(Succeed(), "error in execution of ./configuration-converter.sh")

			actualOutput, err := os.ReadFile(sailFile)
			Expect(err).To(Succeed(), fmt.Sprintf("Cannot read %s", sailFile))

			actualData, err := parseYaml(actualOutput)
			Expect(err).To(Succeed(), "Failed to parse sailFile")

			expectedData, err := parseYaml([]byte(tc.expectedOutput))
			Expect(err).To(Succeed(), "Failed to parse expected output")

			Expect(cmp.Diff(actualData, expectedData)).To(Equal(""), "Conversion is not as expected")
		})
	}
}

func executeConfigConverter(scriptArgs string) error {
	converter := filepath.Join(project.RootDir, "tools", "configuration-converter.sh")

	cmdArgs := []string{converter, scriptArgs}

	cmd := strings.Join(cmdArgs, " ")
	_, err := shell.ExecuteCommand(cmd)
	if err != nil {
		return fmt.Errorf("error in execution of command %v: %w", cmdArgs, err)
	}
	return nil
}

// parseYaml takes a YAML string and unmarshals it into a map
func parseYaml(yamlContent []byte) (map[string]interface{}, error) {
	var config map[string]interface{}
	err := yaml.Unmarshal(yamlContent, &config)
	return config, err
}
