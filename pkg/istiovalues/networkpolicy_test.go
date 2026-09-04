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
	"testing"

	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyNetworkPolicyDefaults(t *testing.T) {
	tests := []struct {
		name                  string
		ocpVersion            *config.OCPVersion
		values                helm.Values
		existingReleaseValues helm.Values
		wantEnabled           *bool
	}{
		{
			name:        "fresh OCP 5 release enables NetworkPolicy",
			ocpVersion:  &config.OCPVersion{Major: 5},
			values:      helm.Values{},
			wantEnabled: boolPtr(true),
		},
		{
			name:        "fresh OCP 4 release remains opt in",
			ocpVersion:  &config.OCPVersion{Major: 4},
			values:      helm.Values{},
			wantEnabled: nil,
		},
		{
			name:        "fresh non OpenShift release remains opt in",
			values:      helm.Values{},
			wantEnabled: nil,
		},
		{
			name:       "explicit false overrides fresh OCP 5 default",
			ocpVersion: &config.OCPVersion{Major: 5},
			values: helm.Values{"global": map[string]any{"networkPolicy": map[string]any{
				"enabled": false,
			}}},
			wantEnabled: boolPtr(false),
		},
		{
			name:                  "legacy release without setting remains unchanged",
			ocpVersion:            &config.OCPVersion{Major: 5},
			values:                helm.Values{},
			existingReleaseValues: helm.Values{},
			wantEnabled:           nil,
		},
		{
			name:       "existing default enabled release remains enabled",
			ocpVersion: &config.OCPVersion{Major: 5},
			values:     helm.Values{},
			existingReleaseValues: helm.Values{"global": map[string]any{"networkPolicy": map[string]any{
				"enabled": true,
			}}},
			wantEnabled: boolPtr(true),
		},
		{
			name:   "existing disabled release remains disabled without OCP version",
			values: helm.Values{},
			existingReleaseValues: helm.Values{"global": map[string]any{"networkPolicy": map[string]any{
				"enabled": false,
			}}},
			wantEnabled: boolPtr(false),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ApplyNetworkPolicyDefaults(tt.ocpVersion, &tt.values, tt.existingReleaseValues)
			require.NoError(t, err)

			enabled, found, err := tt.values.GetBool("global.networkPolicy.enabled")
			require.NoError(t, err)
			if tt.wantEnabled == nil {
				assert.False(t, found)
				return
			}
			assert.True(t, found)
			assert.Equal(t, *tt.wantEnabled, enabled)
		})
	}
}

func boolPtr(v bool) *bool { return &v }
