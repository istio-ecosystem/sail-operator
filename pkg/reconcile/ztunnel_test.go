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

	"github.com/istio-ecosystem/sail-operator/pkg/reconciler"
	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestZTunnelReconciler_Validate(t *testing.T) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "istio-system",
		},
	}

	tests := []struct {
		name        string
		version     string
		namespace   string
		nsExists    bool
		wantErr     bool
		errContains string
	}{
		{
			name:        "missing version",
			version:     "",
			namespace:   "istio-system",
			nsExists:    true,
			wantErr:     true,
			errContains: "version not set",
		},
		{
			name:        "missing namespace",
			version:     "v1.24.0",
			namespace:   "",
			nsExists:    true,
			wantErr:     true,
			errContains: "namespace not set",
		},
		{
			name:        "namespace not found",
			version:     "v1.24.0",
			namespace:   "istio-system",
			nsExists:    false,
			wantErr:     true,
			errContains: `namespace "istio-system" doesn't exist`,
		},
		{
			name:      "valid",
			version:   "v1.24.0",
			namespace: "istio-system",
			nsExists:  true,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientBuilder := fake.NewClientBuilder().WithScheme(scheme.Scheme)
			if tt.nsExists {
				clientBuilder = clientBuilder.WithObjects(ns)
			}
			cl := clientBuilder.Build()

			r := NewZTunnelReconciler(Config{}, cl)
			err := r.Validate(context.Background(), tt.version, tt.namespace)

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
