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
	"context"
	"testing"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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

func TestGetIstioRevisionFromTargetReference(t *testing.T) {
	rev := &v1.IstioRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-revision",
		},
	}
	istioWithActiveRevision := &v1.Istio{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-istio",
		},
		Status: v1.IstioStatus{
			ActiveRevisionName: "my-revision",
		},
	}
	istioWithoutActiveRevision := &v1.Istio{
		ObjectMeta: metav1.ObjectMeta{
			Name: "no-active",
		},
	}

	tests := []struct {
		name        string
		ref         v1.TargetReference
		objects     []client.Object
		expectedRev string
		expectErr   string
	}{
		{
			name:        "IstioRevision reference",
			ref:         v1.TargetReference{Kind: v1.IstioRevisionKind, Name: "my-revision"},
			objects:     []client.Object{rev},
			expectedRev: "my-revision",
		},
		{
			name:        "Istio reference with active revision",
			ref:         v1.TargetReference{Kind: v1.IstioKind, Name: "my-istio"},
			objects:     []client.Object{istioWithActiveRevision, rev},
			expectedRev: "my-revision",
		},
		{
			name:      "Istio reference without active revision",
			ref:       v1.TargetReference{Kind: v1.IstioKind, Name: "no-active"},
			objects:   []client.Object{istioWithoutActiveRevision},
			expectErr: "referenced Istio has no active revision",
		},
		{
			name:      "Istio reference not found",
			ref:       v1.TargetReference{Kind: v1.IstioKind, Name: "nonexistent"},
			objects:   []client.Object{},
			expectErr: "not found",
		},
		{
			name:      "IstioRevision reference not found",
			ref:       v1.TargetReference{Kind: v1.IstioRevisionKind, Name: "nonexistent"},
			objects:   []client.Object{},
			expectErr: "not found",
		},
		{
			name:      "unknown kind",
			ref:       v1.TargetReference{Kind: "UnknownKind", Name: "test"},
			objects:   []client.Object{},
			expectErr: "unknown targetRef.kind",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(tt.objects...).WithStatusSubresource(&v1.Istio{}).Build()
			result, err := GetIstioRevisionFromTargetReference(context.TODO(), cl, tt.ref)
			if tt.expectErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedRev, result.Name)
			}
		})
	}
}
