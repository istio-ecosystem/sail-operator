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

package revision

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetReferencedRevisionFromNamespace(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		expected string
	}{
		{
			name:     "no-labels",
			labels:   map[string]string{},
			expected: "",
		},
		{
			name: "injection-enabled-label",
			labels: map[string]string{
				"istio-injection": "enabled",
			},
			expected: "default",
		},
		{
			name: "rev-label",
			labels: map[string]string{
				"istio.io/rev": "my-revision",
			},
			expected: "my-revision",
		},
		{
			name: "injection-enabled-and-rev-label",
			labels: map[string]string{
				"istio-injection": "enabled",
				"istio.io/rev":    "my-revision",
			},
			expected: "default", // injection-enabled takes precedence; rev label is ignored
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetReferencedRevisionFromNamespace(tt.labels)
			assert.Equalf(t, tt.expected, result, "GetReferencedRevisionFromNamespace(%v)", tt.labels)
		})
	}
}

func TestGetReferencedRevisionFromPod(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		expected string
	}{
		{
			name:     "no-labels",
			labels:   map[string]string{},
			expected: "",
		},
		{
			name: "inject-false",
			labels: map[string]string{
				"sidecar.istio.io/inject": "false",
			},
			expected: "",
		},
		{
			name: "inject-true",
			labels: map[string]string{
				"sidecar.istio.io/inject": "true",
			},
			expected: "default",
		},
		{
			name: "rev-label-only",
			labels: map[string]string{
				"istio.io/rev": "my-revision",
			},
			expected: "my-revision",
		},
		{
			name: "rev-label-with-inject-false",
			labels: map[string]string{
				"sidecar.istio.io/inject": "false",
				"istio.io/rev":            "my-revision",
			},
			expected: "",
		},
		{
			name: "rev-label-with-inject-true",
			labels: map[string]string{
				"sidecar.istio.io/inject": "true",
				"istio.io/rev":            "my-revision",
			},
			expected: "my-revision",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetReferencedRevisionFromPod(tt.labels)
			assert.Equalf(t, tt.expected, result, "GetReferencedRevisionFromPod(%v)", tt.labels)
		})
	}
}

func TestGetInjectedRevisionFromPod(t *testing.T) {
	tests := []struct {
		name           string
		podAnnotations map[string]string
		expected       string
	}{
		{
			name:           "no-annotations",
			podAnnotations: map[string]string{},
			expected:       "",
		},
		{
			name: "rev-annotation",
			podAnnotations: map[string]string{
				"istio.io/rev": "my-revision",
			},
			expected: "my-revision",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetInjectedRevisionFromPod(tt.podAnnotations)
			assert.Equalf(t, tt.expected, result, "GetInjectedRevisionFromPod(%v)", tt.podAnnotations)
		})
	}
}
