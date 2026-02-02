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

package reconcile

import (
	"testing"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/reconciler"
	"github.com/stretchr/testify/assert"

	"istio.io/istio/pkg/ptr"
)

func TestCNIReconciler_ValidateSpec(t *testing.T) {
	tests := []struct {
		name        string
		version     string
		namespace   string
		wantErr     bool
		errContains string
	}{
		{
			name:        "missing version",
			version:     "",
			namespace:   "istio-system",
			wantErr:     true,
			errContains: "version not set",
		},
		{
			name:        "missing namespace",
			version:     "v1.24.0",
			namespace:   "",
			wantErr:     true,
			errContains: "namespace not set",
		},
		{
			name:      "valid",
			version:   "v1.24.0",
			namespace: "istio-system",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewCNIReconciler(Config{}, nil)
			err := r.ValidateSpec(tt.version, tt.namespace)

			if tt.wantErr {
				assert.Error(t, err)
				assert.True(t, reconciler.IsValidationError(err), "expected validation error")
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestApplyCNIImageDigests(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		values   *v1.CNIValues
		config   config.OperatorConfig
		expected *v1.CNIValues
	}{
		{
			name:    "no digests defined",
			version: "v1.24.0",
			values:  nil,
			config: config.OperatorConfig{
				ImageDigests: map[string]config.IstioImageConfig{},
			},
			expected: nil,
		},
		{
			name:    "applies digest when values is nil",
			version: "v1.24.0",
			values:  nil,
			config: config.OperatorConfig{
				ImageDigests: map[string]config.IstioImageConfig{
					"v1.24.0": {CNIImage: "istio/cni@sha256:abc123"},
				},
			},
			expected: &v1.CNIValues{
				Cni: &v1.CNIConfig{
					Image: ptr.Of("istio/cni@sha256:abc123"),
				},
			},
		},
		{
			name:    "does not override user-set global hub",
			version: "v1.24.0",
			values: &v1.CNIValues{
				Global: &v1.CNIGlobalConfig{
					Hub: ptr.Of("my-registry.io"),
				},
			},
			config: config.OperatorConfig{
				ImageDigests: map[string]config.IstioImageConfig{
					"v1.24.0": {CNIImage: "istio/cni@sha256:abc123"},
				},
			},
			expected: &v1.CNIValues{
				Global: &v1.CNIGlobalConfig{
					Hub: ptr.Of("my-registry.io"),
				},
			},
		},
		{
			name:    "does not override user-set image",
			version: "v1.24.0",
			values: &v1.CNIValues{
				Cni: &v1.CNIConfig{
					Image: ptr.Of("my-custom-image"),
				},
			},
			config: config.OperatorConfig{
				ImageDigests: map[string]config.IstioImageConfig{
					"v1.24.0": {CNIImage: "istio/cni@sha256:abc123"},
				},
			},
			expected: &v1.CNIValues{
				Cni: &v1.CNIConfig{
					Image: ptr.Of("my-custom-image"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ApplyCNIImageDigests(tt.version, tt.values, tt.config)
			assert.Equal(t, tt.expected, result)
		})
	}
}
