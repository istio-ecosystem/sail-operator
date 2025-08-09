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

package istiovalues

import (
	_ "embed"
	"fmt"
	"strings"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	//go:embed vendor_defaults.yaml
	vendorDefaultsYAML []byte
	vendorDefaults     map[string]map[string]any
)

func init() {
	vendorDefaults = MustParseVendorDefaultsYAML(vendorDefaultsYAML)
}

func MustParseVendorDefaultsYAML(defaultsYAML []byte) map[string]map[string]any {
	var parsedDefaults map[string]map[string]any
	err := yaml.Unmarshal(defaultsYAML, &parsedDefaults)
	if err != nil {
		panic("failed to read vendor_defaults.yaml: " + err.Error())
	}
	return parsedDefaults
}

// OverrideVendorDefaults allows tests to override the vendor defaults
func OverrideVendorDefaults(defaults map[string]map[string]any) {
	// Set the vendorDefaults to the provided defaults for testing purposes.
	// This allows tests to override the defaults without modifying the original file.
	vendorDefaults = defaults
}

// ApplyIstioVendorDefaults applies vendor-specific default values to the provided
// Istio configuration (*v1.Values) for a given Istio version.
func ApplyIstioVendorDefaults(version string, values *v1.Values) (*v1.Values, error) {
	// Convert the specific *v1.Values struct to a generic map[string]any
	userHelmValues := helm.FromValues(values)

	mergedHelmValues, err := applyVendorDefaultsForResourceType(version, v1.IstioKind, userHelmValues)
	if err != nil {
		return nil, fmt.Errorf("failed to apply vendor defaults for istio: %w", err)
	}

	finalValues, err := helm.ToValues(mergedHelmValues, &v1.Values{})
	if err != nil {
		return nil, fmt.Errorf("failed to convert merged values back to *v1.Values: %w", err)
	}

	return finalValues, nil
}

// ApplyIstioCNIVendorDefaults applies vendor-specific default values to the provided
// Istio CNI configuration (*v1.CNIValues) for a given Istio version.
func ApplyIstioCNIVendorDefaults(version string, values *v1.CNIValues) (*v1.CNIValues, error) {
	// Convert the specific *v1.CNIValues struct to a generic map[string]any
	userHelmValues := helm.FromValues(values)

	mergedHelmValues, err := applyVendorDefaultsForResourceType(version, v1.IstioCNIKind, userHelmValues)
	if err != nil {
		return nil, fmt.Errorf("failed to apply vendor defaults for istiocni: %w", err)
	}

	finalValues, err := helm.ToValues(mergedHelmValues, &v1.CNIValues{})
	if err != nil {
		return nil, fmt.Errorf("failed to convert merged values back to *v1.CNIValues: %w", err)
	}

	return finalValues, nil
}

// applyVendorDefaultsForResourceType retrieves defaults for
// a given version and resourceType, current valid resources are: "istio" and "istiocni"
// It returns the merged map and an error if the defaults are not found or malformed.
// The userValuesMap is the 'base', and resource-specific defaults are 'overrides'.
func applyVendorDefaultsForResourceType(version, resourceType string, userValuesMap map[string]any) (map[string]any, error) {
	if len(vendorDefaults) == 0 {
		return userValuesMap, nil // No vendor defaults defined
	}

	versionSpecificDefaults, versionExists := vendorDefaults[version]
	if !versionExists {
		return userValuesMap, nil // No vendor defaults defined for this version
	}

	resourceSpecificDefaults, resourceExists := versionSpecificDefaults[strings.ToLower(resourceType)]
	if !resourceExists {
		return userValuesMap, nil // No vendor defaults defined for this resource type in this version
	}

	// Check if the resource-specific defaults are a map[string]any
	resourceSpecificDefaultsMap, ok := resourceSpecificDefaults.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("vendor defaults for resource '%s' (version '%s') are not a map[string]any", resourceType, version)
	}

	valsMap := runtime.DeepCopyJSON(resourceSpecificDefaultsMap)
	return mergeOverwrite(valsMap, userValuesMap), nil
}
