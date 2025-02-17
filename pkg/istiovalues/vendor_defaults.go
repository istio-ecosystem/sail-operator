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

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
	"gopkg.in/yaml.v3"
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

func ApplyVendorDefaults(version string, values *v1.Values) (*v1.Values, error) {
	if len(vendorDefaults) == 0 {
		return values, nil
	}
	mergedValues := helm.Values(mergeOverwrite(helm.FromValues(values), vendorDefaults[version]))
	return helm.ToValues(mergedValues, &v1.Values{})
}
