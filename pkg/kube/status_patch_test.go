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

package kube

import (
	"testing"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
)

func TestNewStatusPatch(t *testing.T) {
	tests := []struct {
		name         string
		status       any
		expectedData []byte
		expectError  bool
	}{
		{
			name:         "happy-path",
			status:       v1.IstioRevisionStatus{State: v1.IstioRevisionReasonHealthy},
			expectedData: []byte(`[{"op":"replace","path":"/status","value":{"state":"Healthy"}}]`),
		},
		{
			name:        "marshaling-error",
			status:      make(chan int), // causes json marshaling to fail
			expectError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patch := NewStatusPatch(tt.status)
			assert.Equal(t, types.JSONPatchType, patch.Type())

			data, err := patch.Data(&v1.IstioRevision{})
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedData, data)
			}
		})
	}
}
