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
		vendorDefaults string
		version        string
		preValues      *v1.Values
		postValues     *v1.Values
		err            error
	}{
		{
			vendorDefaults: `
v1.24.2:
  pilot:
    env:
      someEnvVar: "true"
`,
			version:   "v1.24.2",
			preValues: &v1.Values{},
			postValues: &v1.Values{
				Pilot: &v1.PilotConfig{
					Env: map[string]string{
						"someEnvVar": "true",
					},
				},
			},
		},
	}
	for _, tc := range testcases {
		vendorDefaults = MustParseVendorDefaultsYAML([]byte(tc.vendorDefaults))
		result, err := ApplyVendorDefaults(tc.version, tc.preValues)
		if err != tc.err {
			t.Errorf("unexpected error: %v, expected %v", err, tc.err)
		}
		if diff := cmp.Diff(tc.postValues, result); diff != "" {
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
		_, err := ApplyVendorDefaults(version, &v1.Values{})
		if err != nil {
			t.Errorf("failed to parse vendor_defaults.yaml at version %s: %v", version, err)
		}
	}
}
