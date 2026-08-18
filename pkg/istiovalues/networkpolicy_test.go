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

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

func TestApplyIstioNetworkPolicyDefaults(t *testing.T) {
	tests := []struct {
		name        string
		ocpVersion  *config.OCPVersion
		values      *v1.Values
		wantEnabled *bool
	}{
		{
			name:        "nil OCPVersion (non-OpenShift) - no change",
			ocpVersion:  nil,
			values:      &v1.Values{},
			wantEnabled: nil,
		},
		{
			name:        "OCP 4.16 - no change",
			ocpVersion:  &config.OCPVersion{Major: 4, Minor: 16},
			values:      &v1.Values{},
			wantEnabled: nil,
		},
		{
			name:        "OCP 5.0 - sets enabled true",
			ocpVersion:  &config.OCPVersion{Major: 5, Minor: 0},
			values:      &v1.Values{},
			wantEnabled: ptr.To(true),
		},
		{
			name:        "OCP 5.1 - sets enabled true",
			ocpVersion:  &config.OCPVersion{Major: 5, Minor: 1},
			values:      &v1.Values{},
			wantEnabled: ptr.To(true),
		},
		{
			name:        "OCP 6.0 - sets enabled true",
			ocpVersion:  &config.OCPVersion{Major: 6, Minor: 0},
			values:      &v1.Values{},
			wantEnabled: ptr.To(true),
		},
		{
			name:       "OCP 5+ with user-explicit false - remains false",
			ocpVersion: &config.OCPVersion{Major: 5, Minor: 0},
			values: &v1.Values{
				Global: &v1.GlobalConfig{
					NetworkPolicy: &v1.NetworkPolicyConfig{
						Enabled: ptr.To(false),
					},
				},
			},
			wantEnabled: ptr.To(false),
		},
		{
			name:       "OCP 4 with user-explicit true - remains true",
			ocpVersion: &config.OCPVersion{Major: 4, Minor: 16},
			values: &v1.Values{
				Global: &v1.GlobalConfig{
					NetworkPolicy: &v1.NetworkPolicyConfig{
						Enabled: ptr.To(true),
					},
				},
			},
			wantEnabled: ptr.To(true),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ApplyIstioNetworkPolicyDefaults(tt.ocpVersion, tt.values)
			if tt.wantEnabled == nil {
				if result.Global != nil && result.Global.NetworkPolicy != nil {
					assert.Nil(t, result.Global.NetworkPolicy.Enabled)
				}
			} else {
				assert.NotNil(t, result.Global)
				assert.NotNil(t, result.Global.NetworkPolicy)
				assert.NotNil(t, result.Global.NetworkPolicy.Enabled)
				assert.Equal(t, *tt.wantEnabled, *result.Global.NetworkPolicy.Enabled)
			}
		})
	}
}

func TestApplyCNINetworkPolicyDefaults(t *testing.T) {
	tests := []struct {
		name        string
		ocpVersion  *config.OCPVersion
		values      *v1.CNIValues
		wantEnabled *bool
	}{
		{
			name:        "nil OCPVersion - no change",
			ocpVersion:  nil,
			values:      &v1.CNIValues{},
			wantEnabled: nil,
		},
		{
			name:        "OCP 4.16 - no change",
			ocpVersion:  &config.OCPVersion{Major: 4, Minor: 16},
			values:      &v1.CNIValues{},
			wantEnabled: nil,
		},
		{
			name:        "OCP 5.0 - sets enabled true",
			ocpVersion:  &config.OCPVersion{Major: 5, Minor: 0},
			values:      &v1.CNIValues{},
			wantEnabled: ptr.To(true),
		},
		{
			name:       "OCP 5+ with user-explicit false - remains false",
			ocpVersion: &config.OCPVersion{Major: 5, Minor: 0},
			values: &v1.CNIValues{
				Global: &v1.CNIGlobalConfig{
					NetworkPolicy: &v1.NetworkPolicyConfig{
						Enabled: ptr.To(false),
					},
				},
			},
			wantEnabled: ptr.To(false),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ApplyCNINetworkPolicyDefaults(tt.ocpVersion, tt.values)
			if tt.wantEnabled == nil {
				if result.Global != nil && result.Global.NetworkPolicy != nil {
					assert.Nil(t, result.Global.NetworkPolicy.Enabled)
				}
			} else {
				assert.NotNil(t, result.Global)
				assert.NotNil(t, result.Global.NetworkPolicy)
				assert.NotNil(t, result.Global.NetworkPolicy.Enabled)
				assert.Equal(t, *tt.wantEnabled, *result.Global.NetworkPolicy.Enabled)
			}
		})
	}
}

func TestApplyZTunnelNetworkPolicyDefaults(t *testing.T) {
	tests := []struct {
		name        string
		ocpVersion  *config.OCPVersion
		values      *v1.ZTunnelValues
		wantEnabled *bool
	}{
		{
			name:        "nil OCPVersion - no change",
			ocpVersion:  nil,
			values:      &v1.ZTunnelValues{},
			wantEnabled: nil,
		},
		{
			name:        "OCP 4.16 - no change",
			ocpVersion:  &config.OCPVersion{Major: 4, Minor: 16},
			values:      &v1.ZTunnelValues{},
			wantEnabled: nil,
		},
		{
			name:        "OCP 5.0 - sets enabled true",
			ocpVersion:  &config.OCPVersion{Major: 5, Minor: 0},
			values:      &v1.ZTunnelValues{},
			wantEnabled: ptr.To(true),
		},
		{
			name:       "OCP 5+ with user-explicit false - remains false",
			ocpVersion: &config.OCPVersion{Major: 5, Minor: 0},
			values: &v1.ZTunnelValues{
				Global: &v1.ZTunnelGlobalConfig{
					NetworkPolicy: &v1.NetworkPolicyConfig{
						Enabled: ptr.To(false),
					},
				},
			},
			wantEnabled: ptr.To(false),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ApplyZTunnelNetworkPolicyDefaults(tt.ocpVersion, tt.values)
			if tt.wantEnabled == nil {
				if result.Global != nil && result.Global.NetworkPolicy != nil {
					assert.Nil(t, result.Global.NetworkPolicy.Enabled)
				}
			} else {
				assert.NotNil(t, result.Global)
				assert.NotNil(t, result.Global.NetworkPolicy)
				assert.NotNil(t, result.Global.NetworkPolicy.Enabled)
				assert.Equal(t, *tt.wantEnabled, *result.Global.NetworkPolicy.Enabled)
			}
		})
	}
}
