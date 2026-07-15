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
	"path"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
)

func TestDetectFipsMode(t *testing.T) {
	resourceDir := t.TempDir()
	os.WriteFile(path.Join(resourceDir, "fips_enabled"), []byte("1\n"), 0o644)
	os.WriteFile(path.Join(resourceDir, "fips_not_enabled"), []byte("0\n"), 0o644)
	tests := []struct {
		name        string
		filepath    string
		expectValue bool
	}{
		{
			name:        "FIPS not enabled",
			filepath:    path.Join(resourceDir, "fips_not_enabled"),
			expectValue: false,
		},
		{
			name:        "FIPS enabled",
			filepath:    path.Join(resourceDir, "fips_enabled"),
			expectValue: true,
		},
		{
			name:        "file not found",
			filepath:    path.Join(resourceDir, "fips_not_found"),
			expectValue: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detectFipsMode(tt.filepath)
			actual := FipsEnabled

			if diff := cmp.Diff(tt.expectValue, actual); diff != "" {
				t.Errorf("FipsEnabled variable wasn't applied properly; diff (-expected, +actual):\n%v", diff)
			}
		})
	}
}

func TestApplyFipsValues(t *testing.T) {
	tests := []struct {
		name         string
		fipsEnabled  bool
		inputValues  *v1.Values
		expectValues *v1.Values
	}{
		{
			name:         "FIPS not enabled",
			fipsEnabled:  false,
			inputValues:  &v1.Values{},
			expectValues: &v1.Values{},
		},
		{
			name:        "FIPS enabled",
			fipsEnabled: true,
			inputValues: &v1.Values{},
			expectValues: &v1.Values{
				Pilot: &v1.PilotConfig{
					Env: map[string]string{"COMPLIANCE_POLICY": "fips-140-2"},
				},
			},
		},
		{
			name:        "FIPS enabled with existing env",
			fipsEnabled: true,
			inputValues: &v1.Values{
				Pilot: &v1.PilotConfig{
					Env: map[string]string{"OTHER_VAR": "value"},
				},
			},
			expectValues: &v1.Values{
				Pilot: &v1.PilotConfig{
					Env: map[string]string{
						"OTHER_VAR":         "value",
						"COMPLIANCE_POLICY": "fips-140-2",
					},
				},
			},
		},
		{
			name:        "FIPS enabled but COMPLIANCE_POLICY already set",
			fipsEnabled: true,
			inputValues: &v1.Values{
				Pilot: &v1.PilotConfig{
					Env: map[string]string{"COMPLIANCE_POLICY": "custom-policy"},
				},
			},
			expectValues: &v1.Values{
				Pilot: &v1.PilotConfig{
					Env: map[string]string{"COMPLIANCE_POLICY": "custom-policy"},
				},
			},
		},
		{
			name:         "nil values",
			fipsEnabled:  false,
			inputValues:  nil,
			expectValues: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalFipsEnabled := FipsEnabled
			t.Cleanup(func() { FipsEnabled = originalFipsEnabled })
			FipsEnabled = tt.fipsEnabled
			ApplyFipsValues(tt.inputValues)

			if diff := cmp.Diff(tt.expectValues, tt.inputValues); diff != "" {
				t.Errorf("COMPLIANCE_POLICY env wasn't applied properly; diff (-expected, +actual):\n%v", diff)
			}
		})
	}
}

func TestApplyZTunnelFipsValues(t *testing.T) {
	tests := []struct {
		name         string
		fipsEnabled  bool
		version      string
		inputValues  *v1.ZTunnelValues
		expectValues *v1.ZTunnelValues
	}{
		{
			name:         "FIPS not enabled",
			fipsEnabled:  false,
			version:      "1.29.0",
			inputValues:  &v1.ZTunnelValues{},
			expectValues: &v1.ZTunnelValues{},
		},
		{
			name:        "FIPS enabled",
			fipsEnabled: true,
			version:     "1.29.0",
			inputValues: &v1.ZTunnelValues{},
			expectValues: &v1.ZTunnelValues{
				ZTunnel: &v1.ZTunnelConfig{
					Env: map[string]string{"TLS12_ENABLED": "true"},
				},
			},
		},
		{
			name:        "FIPS enabled with existing env",
			fipsEnabled: true,
			version:     "1.29.0",
			inputValues: &v1.ZTunnelValues{
				ZTunnel: &v1.ZTunnelConfig{
					Env: map[string]string{"OTHER_VAR": "value"},
				},
			},
			expectValues: &v1.ZTunnelValues{
				ZTunnel: &v1.ZTunnelConfig{
					Env: map[string]string{
						"OTHER_VAR":     "value",
						"TLS12_ENABLED": "true",
					},
				},
			},
		},
		{
			name:        "FIPS enabled but TLS12_ENABLED already set",
			fipsEnabled: true,
			version:     "1.29.0",
			inputValues: &v1.ZTunnelValues{
				ZTunnel: &v1.ZTunnelConfig{
					Env: map[string]string{"TLS12_ENABLED": "false"},
				},
			},
			expectValues: &v1.ZTunnelValues{
				ZTunnel: &v1.ZTunnelConfig{
					Env: map[string]string{"TLS12_ENABLED": "false"},
				},
			},
		},
		{
			name:         "nil values",
			fipsEnabled:  false,
			version:      "1.29.0",
			inputValues:  nil,
			expectValues: nil,
		},
		{
			name:        "version 1.30 still sets TLS12_ENABLED",
			fipsEnabled: true,
			version:     "1.30.0",
			inputValues: &v1.ZTunnelValues{},
			expectValues: &v1.ZTunnelValues{
				ZTunnel: &v1.ZTunnelConfig{
					Env: map[string]string{"TLS12_ENABLED": "true"},
				},
			},
		},
		{
			name:        "version > 1.30 removes TLS12_ENABLED",
			fipsEnabled: true,
			version:     "1.31.0",
			inputValues: &v1.ZTunnelValues{
				ZTunnel: &v1.ZTunnelConfig{
					Env: map[string]string{
						"TLS12_ENABLED": "true",
						"OTHER_VAR":     "keep",
					},
				},
			},
			expectValues: &v1.ZTunnelValues{
				ZTunnel: &v1.ZTunnelConfig{
					Env: map[string]string{
						"OTHER_VAR": "keep",
					},
				},
			},
		},
		{
			name:         "version > 1.30 does not set TLS12_ENABLED even with FIPS",
			fipsEnabled:  true,
			version:      "1.31.0",
			inputValues:  &v1.ZTunnelValues{},
			expectValues: &v1.ZTunnelValues{},
		},
		{
			name:         "version 1.31-alpha does not set TLS12_ENABLED",
			fipsEnabled:  true,
			version:      "1.31.0-alpha.0",
			inputValues:  &v1.ZTunnelValues{},
			expectValues: &v1.ZTunnelValues{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			originalFipsEnabled := FipsEnabled
			t.Cleanup(func() { FipsEnabled = originalFipsEnabled })
			FipsEnabled = tt.fipsEnabled
			ApplyZTunnelFipsValues(tt.inputValues, tt.version)

			if diff := cmp.Diff(tt.expectValues, tt.inputValues); diff != "" {
				t.Errorf("TLS12_ENABLED env wasn't applied properly; diff (-expected, +actual):\n%v", diff)
			}
		})
	}
}
