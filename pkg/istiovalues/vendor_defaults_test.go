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
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
)

func TestApplyVendorDefaults(t *testing.T) {
	testcases := []struct {
		vendorDefaults     string
		version            string
		istioPreValues     *v1.Values
		istoPostValues     *v1.Values
		istioCNIPreValues  *v1.CNIValues
		istioCniPostValues *v1.CNIValues
		err                error
	}{
		{
			vendorDefaults: `
v1.24.2:
  istio:
    pilot:
      env:
        someEnvVar: "true"
  istiocni:
    cni:
      cniConfDir: example/path
`,
			version:           "v1.24.2",
			istioPreValues:    &v1.Values{},
			istioCNIPreValues: &v1.CNIValues{},
			istoPostValues: &v1.Values{
				Pilot: &v1.PilotConfig{
					Env: map[string]string{
						"someEnvVar": "true",
					},
				},
			},
			istioCniPostValues: &v1.CNIValues{
				Cni: &v1.CNIConfig{
					CniConfDir: StringPtr("example/path"),
				},
			},
		},
	}
	for _, tc := range testcases {
		vendorDefaults = MustParseVendorDefaultsYAML([]byte(tc.vendorDefaults))
		result, err := ApplyIstioVendorDefaults(tc.version, tc.istioPreValues)
		if err != tc.err {
			t.Errorf("unexpected error: %v, expected %v", err, tc.err)
		}
		if diff := cmp.Diff(tc.istoPostValues, result); diff != "" {
			t.Errorf("unexpected merge result; diff (-expected, +actual):\n%v", diff)
		}

		resultCni, err := ApplyIstioCNIVendorDefaults(tc.version, tc.istioCNIPreValues)
		if err != tc.err {
			t.Errorf("unexpected error: %v, expected %v", err, tc.err)
		}
		if diff := cmp.Diff(tc.istioCniPostValues, resultCni); diff != "" {
			t.Errorf("unexpected merge result; diff (-expected, +actual):\n%v", diff)
		}
	}
}

func TestValidateVendorDefaultsFile(t *testing.T) {
	defaultsYAML, err := os.ReadFile("vendor_defaults.yaml")
	if err != nil {
		t.Errorf("failed to read vendor_defaults.yaml: %v", err)
	}
	vendorDefaults = MustParseVendorDefaultsYAML(defaultsYAML)
	if len(vendorDefaults) == 0 {
		return
	}

	for version := range vendorDefaults {
		_, err := ApplyIstioVendorDefaults(version, &v1.Values{})
		if err != nil {
			t.Errorf("failed to parse vendor_defaults.yaml at version %s: %v", version, err)
		}
	}
}

// StringPtr returns a pointer to a string literal.
func StringPtr(s string) *string {
	return &s
}
