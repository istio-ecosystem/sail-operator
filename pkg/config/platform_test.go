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

package config

import (
	"context"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDetectOCPVersion(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, configv1.Install(scheme))

	tests := []struct {
		name        string
		cv          *configv1.ClusterVersion
		wantMajor   int
		wantMinor   int
		wantErr     bool
		errContains string
	}{
		{
			name: "OCP 4.16.3",
			cv: &configv1.ClusterVersion{
				ObjectMeta: metav1.ObjectMeta{Name: "version"},
				Status:     configv1.ClusterVersionStatus{Desired: configv1.Release{Version: "4.16.3"}},
			},
			wantMajor: 4,
			wantMinor: 16,
		},
		{
			name: "OCP 5.0.0",
			cv: &configv1.ClusterVersion{
				ObjectMeta: metav1.ObjectMeta{Name: "version"},
				Status:     configv1.ClusterVersionStatus{Desired: configv1.Release{Version: "5.0.0"}},
			},
			wantMajor: 5,
			wantMinor: 0,
		},
		{
			name: "OCP 5.1.2",
			cv: &configv1.ClusterVersion{
				ObjectMeta: metav1.ObjectMeta{Name: "version"},
				Status:     configv1.ClusterVersionStatus{Desired: configv1.Release{Version: "5.1.2"}},
			},
			wantMajor: 5,
			wantMinor: 1,
		},
		{
			name:        "ClusterVersion not found",
			cv:          nil,
			wantErr:     true,
			errContains: "failed to get ClusterVersion",
		},
		{
			name: "malformed version string",
			cv: &configv1.ClusterVersion{
				ObjectMeta: metav1.ObjectMeta{Name: "version"},
				Status:     configv1.ClusterVersionStatus{Desired: configv1.Release{Version: "invalid"}},
			},
			wantErr:     true,
			errContains: "failed to parse ClusterVersion",
		},
		{
			name: "empty version string",
			cv: &configv1.ClusterVersion{
				ObjectMeta: metav1.ObjectMeta{Name: "version"},
				Status:     configv1.ClusterVersionStatus{Desired: configv1.Release{Version: ""}},
			},
			wantErr:     true,
			errContains: "failed to parse ClusterVersion",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(scheme)
			if tt.cv != nil {
				builder = builder.WithObjects(tt.cv)
			}
			cl := builder.Build()

			result, err := DetectOCPVersion(context.Background(), cl)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.Equal(t, tt.wantMajor, result.Major)
				assert.Equal(t, tt.wantMinor, result.Minor)
			}
		})
	}
}
