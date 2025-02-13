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

	"github.com/istio-ecosystem/sail-operator/pkg/test/project"
	"github.com/istio-ecosystem/sail-operator/pkg/test/util/supportedversion"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/shell"
	. "github.com/onsi/gomega"
)

var (
	controlPlaneNamespace = "istio-system"
	istioVersion          = supportedversion.Default
	istioFile             = filepath.Join(project.RootDir, "tools", "istioConfig.yaml")
	sailFile              = filepath.Join(project.RootDir, "tools", "istioConfig-sail.yaml")
)

func TestConversion(t *testing.T) {
	t.Cleanup(func() {
		os.Remove(istioFile)
		os.Remove(sailFile)
	})
	testcases := []struct {
		name           string
		input          string
		expectedOutput string
	}{
		{
			name: "simple",
			input: `apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  name: default
spec:`,
			expectedOutput: `apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  version: %s
  namespace: %s`,
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
  version: %s
  namespace: %s`,
		},
		{
			name: "mandatory-arguments-only",
			input: `apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  name: default
spec:`,
			expectedOutput: `apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  version: %s
  namespace: %s`,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			err := saveYamlToFile(tc.input, istioFile)
			g.Expect(err).To(Succeed(), "failed to write YAML file")

			if tc.name == "mandatory-arguments-only" {
				g.Expect(executeConfigConverter(istioFile, "", "", istioVersion)).To(Succeed(), "error in execution of ./configuration-converter.sh")
			} else {
				g.Expect(executeConfigConverter(istioFile, sailFile, controlPlaneNamespace,
					istioVersion)).To(Succeed(), "error in execution of ./configuration-converter.sh")
			}
			tc.expectedOutput = fmt.Sprintf(tc.expectedOutput, istioVersion, controlPlaneNamespace)
			isConversionValidated, err := validateYamlContent(tc.expectedOutput, sailFile)
			g.Expect(err).To(Succeed(), "Can not open file to compare")
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

func executeConfigConverter(input, output, controlPlaneNamespace, istioVersion string) error {
	converter := filepath.Join(project.RootDir, "tools", "configuration-converter.sh")
	args := []string{
		converter,
		"-i", input,
		"-v", istioVersion,
	}

	if output != "" {
		args = append(args, "-o", output)
	}

	if controlPlaneNamespace != "" {
		args = append(args, "-n", controlPlaneNamespace)
	}

	cmd := strings.Join(args, " ")
	_, err := shell.ExecuteCommand(cmd)
	if err != nil {
		return fmt.Errorf("error in execution of %s -i %s -o %s -n %s -v %s: %w", converter, input, output, controlPlaneNamespace, istioVersion, err)
	}
	return nil
}

// compareYamlContent checks if the provided YAML content matches the content of converted sail operator config
func validateYamlContent(expectedOutput, sailFile string) (bool, error) {
	// Write the input YAML string to a temporary file
	tmpFile := filepath.Join(project.RootDir, "tools", "temp.yaml")
	err := os.WriteFile(tmpFile, []byte(expectedOutput), 0o644)
	if err != nil {
		return false, fmt.Errorf("failed to write temporary YAML file: %w", err)
	}
	defer os.Remove(tmpFile)

	// The command will check if the files are equal, ignoring order
	cmd := fmt.Sprintf("diff <(yq -P 'sort_keys(..)' -o=props %s) <(yq -P 'sort_keys(..)' -o=props %s)", sailFile, tmpFile)
	output, err := shell.ExecuteCommand(cmd)
	if err != nil {
		return false, fmt.Errorf("error executing yq comparison: %w", err)
	}
	if output != "" {
		return false, nil
	}
	// If no output from yq, the files are considered equal
	return true, nil
}
