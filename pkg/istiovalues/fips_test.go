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
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
)

func TestDetectFipsMode(t *testing.T) {
	resourceDir := t.TempDir()
	os.WriteFile(path.Join(resourceDir, "fips_enabled"), []byte(("1\n")), 0o644)
	os.WriteFile(path.Join(resourceDir, "fips_not_enabled"), []byte(("0\n")), 0o644)
	tests := []struct {
		name        string
		filepath    string
		expectValue bool
		expectErr   bool
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
			expectErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			FipsEnableFilePath = tt.filepath
			actual, err := DetectFipsMode()
			if (err != nil) != tt.expectErr {
				t.Errorf("DetectFipsMode() error = %v, expectErr %v", err, tt.expectErr)
			}

			if err == nil {
				if diff := cmp.Diff(tt.expectValue, actual); diff != "" {
					t.Errorf("FipsEnabled variable wasn't applied properly; diff (-expected, +actual):\n%v", diff)
				}
			}
		})
	}
}

func TestApplyFipsValues(t *testing.T) {
	tests := []struct {
		name         string
		fipsEnabled  bool
		expectValues helm.Values
		expectErr    bool
	}{
		{
			name:         "FIPS not enabled",
			fipsEnabled:  false,
			expectValues: helm.Values{},
		},
		{
			name:        "FIPS enabled",
			fipsEnabled: true,
			expectValues: helm.Values{
				"pilot": map[string]any{
					"env": map[string]any{"COMPLIANCE_POLICY": string("fips-140-2")},
				},
			},
		},
	}

	values := helm.Values{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			FipsEnabled = tt.fipsEnabled
			actual, err := ApplyFipsValues(values)
			if (err != nil) != tt.expectErr {
				t.Errorf("applyFipsValues() error = %v, expectErr %v", err, tt.expectErr)
			}

			if err == nil {
				if diff := cmp.Diff(tt.expectValues, actual); diff != "" {
					t.Errorf("COMPLIANCE_POLICY env wasn't applied properly; diff (-expected, +actual):\n%v", diff)
				}
			}
		})
	}
}
