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
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"

	"istio.io/istio/pkg/ptr"
)

func TestApplyVendorDefaults(t *testing.T) {
	testcases := []struct {
		name                  string
		vendorDefaults        string
		version               string
		istioPreValues        *v1.Values
		istoPostValues        *v1.Values
		istioCNIPreValues     *v1.CNIValues
		istioCniPostValues    *v1.CNIValues
		expectedIstioError    bool
		expectedIstioCniError bool
		expectedErrSubstring  string
	}{
		{
			name: "adding custom values for both Istio and IstioCNI",
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
					CniConfDir: ptr.Of("example/path"),
				},
			},
			expectedIstioError:    false,
			expectedIstioCniError: false,
			expectedErrSubstring:  "", // no error expected
		},
		{
			name: "adding custom values for Istio only",
			vendorDefaults: `
v1.24.2:
  istio:
    pilot:
      env:
        someEnvVar: "true"
`,
			version: "v1.24.2",
			istioPreValues: &v1.Values{
				Pilot: &v1.PilotConfig{
					Env: map[string]string{
						"someOtherEnvVar": "true",
					},
				},
				MeshConfig: &v1.MeshConfig{
					LocalityLbSetting: &v1.LocalityLoadBalancerSetting{
						Enabled: ptr.Of(true),
					},
				},
			},
			istioCNIPreValues: &v1.CNIValues{},
			istoPostValues: &v1.Values{
				Pilot: &v1.PilotConfig{
					Env: map[string]string{
						"someOtherEnvVar": "true",
						"someEnvVar":      "true",
					},
				},
				MeshConfig: &v1.MeshConfig{
					LocalityLbSetting: &v1.LocalityLoadBalancerSetting{
						Enabled: ptr.Of(true),
					},
				},
			},
			istioCniPostValues:    &v1.CNIValues{},
			expectedIstioError:    false,
			expectedIstioCniError: false,
			expectedErrSubstring:  "", // no error expected
		},
		{
			name: "adding custom values for IstioCNI only",
			vendorDefaults: `
v1.24.2:
  istiocni:
    cni:
      cniConfDir: example/path
`,
			version:           "v1.24.2",
			istioPreValues:    &v1.Values{},
			istioCNIPreValues: &v1.CNIValues{},
			istoPostValues:    &v1.Values{},
			istioCniPostValues: &v1.CNIValues{
				Cni: &v1.CNIConfig{
					CniConfDir: ptr.Of("example/path"),
				},
			},
			expectedIstioError:    false,
			expectedIstioCniError: false,
			expectedErrSubstring:  "", // no error expected
		},
		{
			name: "empty vendor defaults",
			vendorDefaults: `
`, // empty vendor defaults
			version:               "v1.24.2",
			istioPreValues:        &v1.Values{},
			istioCNIPreValues:     &v1.CNIValues{},
			istoPostValues:        &v1.Values{},
			istioCniPostValues:    &v1.CNIValues{},
			expectedIstioError:    false,
			expectedIstioCniError: false,
			expectedErrSubstring:  "", // no error expected
		},
		{
			name: "non-existent version",
			vendorDefaults: `
v1.24.2:
  istio:
    pilot:
      env:
        someEnvVar: "true"
`,
			version:               "v1.25.0", // version not in vendor defaults
			istioPreValues:        &v1.Values{},
			istioCNIPreValues:     &v1.CNIValues{},
			istoPostValues:        &v1.Values{},
			istioCniPostValues:    &v1.CNIValues{},
			expectedIstioError:    false,
			expectedIstioCniError: false,
			expectedErrSubstring:  "", // no error expected
		},
		{
			name: "invalid version format",
			vendorDefaults: `
v1.24.2:
  istio:
    pilot:
      env:
        someEnvVar: "true"
`,
			version:               "v1.24", // invalid version format
			istioPreValues:        &v1.Values{},
			istioCNIPreValues:     &v1.CNIValues{},
			istoPostValues:        &v1.Values{},
			istioCniPostValues:    &v1.CNIValues{},
			expectedIstioError:    false,
			expectedIstioCniError: false,
			expectedErrSubstring:  "", // no error expected
		},
		{
			name: "malformed vendor defaults",
			vendorDefaults: `
v1.24.2:
  istio:
    pilot: "env"
`,
			version:               "v1.24.2",
			istioPreValues:        &v1.Values{},
			istioCNIPreValues:     &v1.CNIValues{},
			istoPostValues:        nil, // expect nil due to malformed defaults
			istioCniPostValues:    &v1.CNIValues{},
			expectedIstioError:    true,
			expectedIstioCniError: false,
			expectedErrSubstring:  "cannot unmarshal string into Go struct field Values.pilot", // expect a specific error for malformed defaults
		},
		{
			name: "user values override vendor defaults",
			vendorDefaults: `
v1.24.2:
  istio:
    pilot:
      env:
        someEnvVar: "true"
`,
			version: "v1.24.2",
			istioPreValues: &v1.Values{
				Pilot: &v1.PilotConfig{
					Env: map[string]string{
						"someEnvVar": "false", // user value overrides vendor default
					},
				},
			},
			istioCNIPreValues: &v1.CNIValues{},
			istoPostValues: &v1.Values{
				Pilot: &v1.PilotConfig{
					Env: map[string]string{
						"someEnvVar": "false", // user value takes precedence
					},
				},
			},
			istioCniPostValues:    &v1.CNIValues{},
			expectedIstioError:    false,
			expectedIstioCniError: false,
			expectedErrSubstring:  "", // no error expected
		},
	}
	for _, tc := range testcases {
		vendorDefaults = MustParseVendorDefaultsYAML([]byte(tc.vendorDefaults))

		// preserve current vendor defaults
		preVendorDefaults, err := yaml.Marshal(vendorDefaults)
		if err != nil {
			t.Fatalf("failed to marshal vendor defaults: %v", err)
		}

		// Test Istio values
		istioResult, istioErr := ApplyIstioVendorDefaults(tc.version, tc.istioPreValues)
		if tc.expectedIstioError {
			if assert.Error(t, istioErr, "expected an error for Istio on %s but got none", tc.name) {
				assert.ErrorContains(t, istioErr, tc.expectedErrSubstring,
					"Istio default values on %s should unwrap JSON-unmarshal errors", tc.name)
			}
		} else {
			assert.NoError(t, istioErr, "unexpected error for Istio on %s: %v", tc.name, istioErr)
		}
		if diff := cmp.Diff(tc.istoPostValues, istioResult); diff != "" {
			t.Errorf("unexpected Istio merge result; diff (-expected, +actual):\n%v", diff)
		}

		// Test IstioCNI values
		cniResult, cniErr := ApplyIstioCNIVendorDefaults(tc.version, tc.istioCNIPreValues)
		if tc.expectedIstioCniError {
			if assert.Error(t, cniErr, "expected an error for IstioCNI on %s but got none", tc.name) {
				assert.ErrorContains(t, cniErr, tc.expectedErrSubstring,
					"IstioCNI default values on %s should unwrap JSON-unmarshal errors", tc.name)
			}
		} else {
			assert.NoError(t, cniErr, "unexpected error for IstioCNI on %s: %v", tc.name, cniErr)
		}
		if diff := cmp.Diff(tc.istioCniPostValues, cniResult); diff != "" {
			t.Errorf("unexpected IstioCNI merge result; diff (-expected, +actual):\n%v", diff)
		}

		postVendorDefaults, err := yaml.Marshal(vendorDefaults)
		if err != nil {
			t.Fatalf("failed to marshal vendor defaults: %v", err)
		}
		if diff := cmp.Diff(preVendorDefaults, postVendorDefaults); diff != "" {
			t.Errorf("vendor defaults should not be modified; diff (-expected, +actual):\n%v", diff)
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

// OverrideVendorDefaults allows tests to override the vendor defaults
func TestOverrideVendorDefaults(t *testing.T) {
	// Create a copy of the original vendor defaults
	originalDefaults := vendorDefaults

	// Define new defaults to override
	newDefaults := map[string]map[string]any{
		"v1.24.2": {
			"istio": map[string]any{
				"pilot": map[string]any{
					"env": map[string]string{
						"newEnvVar": "true",
					},
				},
			},
			"istiocni": map[string]any{
				"cni": map[string]any{
					"cniConfDir": "new/path",
				},
			},
		},
	}

	// Override the vendor defaults
	OverrideVendorDefaults(newDefaults)

	// Validate the override
	assert.Equal(t, newDefaults, vendorDefaults, "Vendor defaults should be overridden")

	// Restore the original defaults
	vendorDefaults = originalDefaults
}
