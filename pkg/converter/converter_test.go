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

	"github.com/istio-ecosystem/sail-operator/pkg/test/project"
	"github.com/istio-ecosystem/sail-operator/pkg/test/util/supportedversion"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/shell"
	. "github.com/onsi/gomega"
)

var (
	controlPlaneNamespace = "istio-system"
	// Test converter with latest istio version
	istioVersion = supportedversion.List[len(supportedversion.List)-1].Name
)

func TestSimpleYamlConversion(t *testing.T) {
	g := NewWithT(t)

	istioYamlText := `apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  name: default
spec:`

	sailYamlText := `apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  version: %s
  namespace: %s`
	sailYamlText = fmt.Sprintf(sailYamlText, istioVersion, controlPlaneNamespace)

	istioYamlFileWithPath, err := saveYamlToFile(istioYamlText)
	g.Expect(err).To(Succeed(), "failed to write YAML file")

	g.Expect(executeConfigConverter(istioYamlFileWithPath, controlPlaneNamespace, istioVersion)).To(Succeed(),
		"error in execution of ./configuration-converter.sh")
	isConversionValidated, err := validateYamlContent(sailYamlText)
	g.Expect(err).To(Succeed(), "Can not open file to compare")
	g.Expect(isConversionValidated).To(BeTrue(), "Converted content is not as expected")

	err = os.Remove(filepath.Join(project.RootDir, "tools", "istioConfig.yaml"))
	g.Expect(err).To(Succeed(), "Unable to delete istioConfig.yaml")
	err = os.Remove(filepath.Join(project.RootDir, "tools", "sail-operator-config.yaml"))
	g.Expect(err).To(Succeed(), "Unable to delete sail-operator-config.yaml")
}

func TestComplexYamlConversion(t *testing.T) {
	g := NewWithT(t)

	istioYamlText := `apiVersion: install.istio.io/v1alpha1
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
        PILOT_ENABLE_STATUS: true`

	sailYamlText := `apiVersion: sailoperator.io/v1
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
  namespace: %s`
	sailYamlText = fmt.Sprintf(sailYamlText, istioVersion, controlPlaneNamespace)

	istioYamlFileWithPath, err := saveYamlToFile(istioYamlText)
	g.Expect(err).To(Succeed(), "failed to write YAML file")

	g.Expect(executeConfigConverter(istioYamlFileWithPath, controlPlaneNamespace, istioVersion)).To(Succeed(),
		"error in execution of ./configuration-converter.sh")
	isConversionValidated, err := validateYamlContent(sailYamlText)
	g.Expect(err).To(Succeed(), "Can not open file to compare")
	g.Expect(isConversionValidated).To(BeTrue(), "Converted content is not as expected")

	err = os.Remove(filepath.Join(project.RootDir, "tools", "istioConfig.yaml"))
	g.Expect(err).To(Succeed(), "Unable to delete istioConfig.yaml")
	err = os.Remove(filepath.Join(project.RootDir, "tools", "sail-operator-config.yaml"))
	g.Expect(err).To(Succeed(), "Unable to delete sail-operator-config.yaml")
}

// saveYamlToFile writes the given YAML content to IstioConfig.yaml
func saveYamlToFile(yamlText string) (string, error) {
	// Create file in the same directory with the converter
	istioYamlFile := "istioConfig.yaml"
	istioYamlFileWithPath := filepath.Join(project.RootDir, "tools", istioYamlFile)

	// Write to file
	if err := os.WriteFile(istioYamlFileWithPath, []byte(yamlText), 0o644); err != nil {
		return "", fmt.Errorf("failed to write YAML file: %w", err)
	}

	return istioYamlFileWithPath, nil
}

func executeConfigConverter(istioYamlFilePath, istioVersion, controlPlaneNamespace string) error {
	// Define file path
	configConverterPath := filepath.Join(project.RootDir, "tools", "configuration-converter.sh")

	_, err := shell.ExecuteBashScript(configConverterPath, istioYamlFilePath, controlPlaneNamespace, istioVersion)
	if err != nil {
		return fmt.Errorf("error in execution of %s %s %s %s: %w", configConverterPath, istioYamlFilePath, controlPlaneNamespace, istioVersion, err)
	}
	return nil
}

// compareYamlContent checks if the provided YAML string matches the content of converted config
func validateYamlContent(yamlString string) (bool, error) {
	sailYamlWithPath := filepath.Join(project.RootDir, "tools", "sail-operator-config.yaml")

	// Write the input YAML string to a temporary file
	tmpFile := filepath.Join(project.RootDir, "tools", "temp.yaml")
	err := os.WriteFile(tmpFile, []byte(yamlString), 0o644)
	if err != nil {
		return false, fmt.Errorf("failed to write temporary YAML file: %w", err)
	}
	defer os.Remove(tmpFile)

	// The command will check if the files are equal, ignoring order
	cmd := fmt.Sprintf("diff <(yq -P 'sort_keys(..)' -o=props %s) <(yq -P 'sort_keys(..)' -o=props %s)", sailYamlWithPath, tmpFile)
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
