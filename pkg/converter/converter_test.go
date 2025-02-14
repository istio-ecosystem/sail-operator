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
	"github.com/istio-ecosystem/sail-operator/pkg/test/util/supportedversion"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/shell"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v2"
)

var (
	controlPlaneNamespace = "istio-system"
	istioVersion          = supportedversion.Default
	istioFile             = filepath.Join(project.RootDir, "tools", "istioConfig.yaml")
	sailFile              = filepath.Join(project.RootDir, "tools", "istioConfig-sail.yaml")
)

func TestConversion(t *testing.T) {
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
			expectedOutput: fmt.Sprintf(`apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  version: %s
  namespace: %s`, istioVersion, controlPlaneNamespace),
			converterArgs: fmt.Sprintf("%s %s -n %s -v %s", istioFile, sailFile, controlPlaneNamespace, istioVersion),
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
			expectedOutput: fmt.Sprintf(`apiVersion: sailoperator.io/v1
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
  namespace: %s
  version: %s`, controlPlaneNamespace, istioVersion),
			converterArgs: fmt.Sprintf("%s %s -v %s -n %s", istioFile, sailFile, istioVersion, controlPlaneNamespace),
		},
		{
			name: "mandatory-arguments-only",
			input: `apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  name: default
spec:`,
			expectedOutput: fmt.Sprintf(`apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  namespace: %s`, controlPlaneNamespace),
			converterArgs: istioFile,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			t.Cleanup(func() {
				os.Remove(istioFile)
				os.Remove(sailFile)
			})
			g := NewWithT(t)
			err := saveYamlToFile(tc.input, istioFile)
			g.Expect(err).To(Succeed(), "failed to write YAML file")

			g.Expect(executeConfigConverter(tc.converterArgs)).To(Succeed(), "error in execution of ./configuration-converter.sh")

			isConversionValidated, err := validateYamlContent(sailFile, tc.expectedOutput)
			g.Expect(err).To(Succeed(), fmt.Errorf("Error on validation: %w", err))
			g.Expect(isConversionValidated).To(BeTrue(), "Converted content is not as expected")
		})
	}
}

// saveYamlToFile writes the given YAML content to given file
func saveYamlToFile(input, istioFile string) error {
	if err := os.WriteFile(istioFile, []byte(input), 0o644); err != nil {
		return fmt.Errorf("failed to write YAML file: %w", err)
	}
	return nil
}

func executeConfigConverter(scriptArgs string) error {
	converter := filepath.Join(project.RootDir, "tools", "configuration-converter.sh")

	cmdArgs := []string{converter, scriptArgs}

	cmd := strings.Join(cmdArgs, " ")
	output, err := shell.ExecuteCommand(cmd)
	if err != nil {
		fmt.Printf("Script output: %s", output)
		return fmt.Errorf("error in execution of command %v: %w", cmdArgs, err)
	}
	return nil
}

// compareYamlContent checks if the provided YAML content matches the content of converted sail operator config
func validateYamlContent(sailFile, expectedOutput string) (bool, error) {
	actualOutput, err := os.ReadFile(sailFile)
	if err != nil {
		return false, fmt.Errorf("Can not read %s. %w", sailFile, err)
	}

	actualData, err := parseYaml(string(actualOutput))
	if err != nil {
		return false, fmt.Errorf("failed to parse sailFile: %w", err)
	}

	expectedData, err := parseYaml(expectedOutput)
	if err != nil {
		return false, fmt.Errorf("failed to parse expected output: %w", err)
	}

	// Compare YAML content ignoring order
	diff := cmp.Diff(expectedData, actualData)
	if diff != "" {
		fmt.Println("YAML files differ:", diff)
		return false, nil
	}
	return true, nil
}

// parseYaml takes a YAML string and unmarshals it into a map
func parseYaml(yamlContent string) (map[string]interface{}, error) {
	var config map[string]interface{}
	err := yaml.Unmarshal([]byte(yamlContent), &config)
	if err != nil {
		return nil, err
	}
	return config, nil
}
