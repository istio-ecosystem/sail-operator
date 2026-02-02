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
	"github.com/istio-ecosystem/sail-operator/pkg/reconciler"
	"github.com/stretchr/testify/assert"

	"istio.io/istio/pkg/ptr"
)

func TestIstiodReconciler_ValidateSpec(t *testing.T) {
	tests := []struct {
		name         string
		version      string
		namespace    string
		values       *v1.Values
		revisionName string
		wantErr      bool
		errContains  string
	}{
		{
			name:        "missing version",
			version:     "",
			namespace:   "istio-system",
			values:      &v1.Values{},
			wantErr:     true,
			errContains: "version not set",
		},
		{
			name:        "missing namespace",
			version:     "v1.24.0",
			namespace:   "",
			values:      &v1.Values{},
			wantErr:     true,
			errContains: "namespace not set",
		},
		{
			name:        "missing values",
			version:     "v1.24.0",
			namespace:   "istio-system",
			values:      nil,
			wantErr:     true,
			errContains: "values not set",
		},
		{
			name:         "default revision with non-empty revision name in values",
			version:      "v1.24.0",
			namespace:    "istio-system",
			revisionName: v1.DefaultRevision,
			values: &v1.Values{
				Revision: ptr.Of("canary"),
				Global: &v1.GlobalConfig{
					IstioNamespace: ptr.Of("istio-system"),
				},
			},
			wantErr:     true,
			errContains: "values.revision must be",
		},
		{
			name:         "non-default revision with mismatched revision name",
			version:      "v1.24.0",
			namespace:    "istio-system",
			revisionName: "canary",
			values: &v1.Values{
				Revision: ptr.Of("other"),
				Global: &v1.GlobalConfig{
					IstioNamespace: ptr.Of("istio-system"),
				},
			},
			wantErr:     true,
			errContains: "values.revision does not match revision name",
		},
		{
			name:         "namespace mismatch",
			version:      "v1.24.0",
			namespace:    "istio-system",
			revisionName: v1.DefaultRevision,
			values: &v1.Values{
				Global: &v1.GlobalConfig{
					IstioNamespace: ptr.Of("other-namespace"),
				},
			},
			wantErr:     true,
			errContains: "values.global.istioNamespace does not match namespace",
		},
		{
			name:         "valid default revision",
			version:      "v1.24.0",
			namespace:    "istio-system",
			revisionName: v1.DefaultRevision,
			values: &v1.Values{
				Revision: ptr.Of(""),
				Global: &v1.GlobalConfig{
					IstioNamespace: ptr.Of("istio-system"),
				},
			},
			wantErr: false,
		},
		{
			name:         "valid canary revision",
			version:      "v1.24.0",
			namespace:    "istio-system",
			revisionName: "canary",
			values: &v1.Values{
				Revision: ptr.Of("canary"),
				Global: &v1.GlobalConfig{
					IstioNamespace: ptr.Of("istio-system"),
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewIstiodReconciler(Config{}, nil)
			err := r.ValidateSpec(tt.version, tt.namespace, tt.values, tt.revisionName)

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

func TestGetReleaseName(t *testing.T) {
	tests := []struct {
		revisionName string
		chartName    string
		expected     string
	}{
		{
			revisionName: "default",
			chartName:    "istiod",
			expected:     "default-istiod",
		},
		{
			revisionName: "canary",
			chartName:    "istiod",
			expected:     "canary-istiod",
		},
		{
			revisionName: "default",
			chartName:    "base",
			expected:     "default-base",
		},
	}

	for _, tt := range tests {
		t.Run(tt.revisionName+"-"+tt.chartName, func(t *testing.T) {
			result := GetReleaseName(tt.revisionName, tt.chartName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetChartPath(t *testing.T) {
	tests := []struct {
		version   string
		chartName string
		expected  string
	}{
		{
			version:   "v1.24.0",
			chartName: "istiod",
			expected:  "v1.24.0/charts/istiod",
		},
		{
			version:   "v1.23.0",
			chartName: "base",
			expected:  "v1.23.0/charts/base",
		},
	}

	for _, tt := range tests {
		t.Run(tt.version+"-"+tt.chartName, func(t *testing.T) {
			result := GetChartPath(tt.version, tt.chartName)
			assert.Equal(t, tt.expected, result)
		})
	}
}
