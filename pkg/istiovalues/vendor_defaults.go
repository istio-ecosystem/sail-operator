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
	"errors"

	"dario.cat/mergo"
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// VendorDefaults maps Istio version strings to their version-specific defaults.
type VendorDefaults map[string]VersionDefaults

// VersionDefaults contains the vendor defaults for a specific Istio version.
type VersionDefaults struct {
	Istio    *v1.Values    `json:"istio,omitempty"`
	IstioCNI *v1.CNIValues `json:"istiocni,omitempty"`
}

var (
	//go:embed vendor_defaults.yaml
	vendorDefaultsYAML []byte
	vendorDefaults     VendorDefaults
)

func init() {
	vendorDefaults = MustParseVendorDefaultsYAML(vendorDefaultsYAML)
}

func ParseVendorDefaultsYAML(defaultsYAML []byte) (VendorDefaults, error) {
	var parsedDefaults VendorDefaults
	if err := yaml.Unmarshal(defaultsYAML, &parsedDefaults); err != nil {
		return nil, errors.New("failed to read vendor_defaults.yaml: " + err.Error())
	}
	return parsedDefaults, nil
}

func MustParseVendorDefaultsYAML(defaultsYAML []byte) VendorDefaults {
	parsedDefaults, err := ParseVendorDefaultsYAML(defaultsYAML)
	if err != nil {
		panic(err)
	}
	return parsedDefaults
}

// OverrideVendorDefaults allows tests to override the vendor defaults.
func OverrideVendorDefaults(defaults VendorDefaults) {
	vendorDefaults = defaults
}

// ApplyIstioVendorDefaults applies vendor-specific default values to the provided
// Istio configuration (*v1.Values) for a given Istio version.
func ApplyIstioVendorDefaults(version string, values *v1.Values) (*v1.Values, error) {
	versionDefaults, exists := vendorDefaults[version]
	if !exists || versionDefaults.Istio == nil {
		return values, nil
	}

	// Start with a copy of defaults, then merge user values on top
	result := versionDefaults.Istio.DeepCopy()
	if err := mergo.Merge(result, values, mergo.WithOverride); err != nil {
		return nil, err
	}
	return result, nil
}

// ApplyIstioCNIVendorDefaults applies vendor-specific default values to the provided
// Istio CNI configuration (*v1.CNIValues) for a given Istio version.
func ApplyIstioCNIVendorDefaults(version string, values *v1.CNIValues) (*v1.CNIValues, error) {
	versionDefaults, exists := vendorDefaults[version]
	if !exists || versionDefaults.IstioCNI == nil {
		return values, nil
	}

	// Start with a copy of defaults, then merge user values on top
	result := versionDefaults.IstioCNI.DeepCopy()
	if err := mergo.Merge(result, values, mergo.WithOverride); err != nil {
		return nil, err
	}
	return result, nil
}
