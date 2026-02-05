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
	"context"
	"testing"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/reconciler"
	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"istio.io/istio/pkg/ptr"
)

func TestIstiodReconciler_Validate(t *testing.T) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "istio-system",
		},
	}

	tests := []struct {
		name        string
		version     string
		namespace   string
		values      *v1.Values
		nsExists    bool
		wantErr     bool
		errContains string
	}{
		{
			name:        "missing version",
			version:     "",
			namespace:   "istio-system",
			values:      &v1.Values{},
			nsExists:    true,
			wantErr:     true,
			errContains: "version not set",
		},
		{
			name:        "missing namespace",
			version:     "v1.24.0",
			namespace:   "",
			values:      &v1.Values{},
			nsExists:    true,
			wantErr:     true,
			errContains: "namespace not set",
		},
		{
			name:        "missing values",
			version:     "v1.24.0",
			namespace:   "istio-system",
			values:      nil,
			nsExists:    true,
			wantErr:     true,
			errContains: "values not set",
		},
		{
			name:      "namespace not found",
			version:   "v1.24.0",
			namespace: "istio-system",
			values: &v1.Values{
				Global: &v1.GlobalConfig{
					IstioNamespace: ptr.Of("istio-system"),
				},
			},
			nsExists:    false,
			wantErr:     true,
			errContains: `namespace "istio-system" doesn't exist`,
		},
		{
			name:      "valid",
			version:   "v1.24.0",
			namespace: "istio-system",
			values: &v1.Values{
				Global: &v1.GlobalConfig{
					IstioNamespace: ptr.Of("istio-system"),
				},
			},
			nsExists: true,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientBuilder := fake.NewClientBuilder().WithScheme(scheme.Scheme)
			if tt.nsExists {
				clientBuilder = clientBuilder.WithObjects(ns)
			}
			cl := clientBuilder.Build()

			r := NewIstiodReconciler(Config{}, cl)
			err := r.Validate(context.Background(), tt.version, tt.namespace, tt.values)

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
