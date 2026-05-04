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
	"crypto/tls"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestNewTLSConfigForOpenShift(t *testing.T) {
	log := zap.New(zap.UseDevMode(true))
	scheme := runtime.NewScheme()
	require.NoError(t, configv1.Install(scheme))

	tests := []struct {
		name      string
		apiServer *configv1.APIServer
		wantErr   bool
		expected  TLSConfig
	}{
		{
			name: "no TLS profile set, no adherence policy",
			apiServer: &configv1.APIServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
			},
			expected: TLSConfig{
				OpenShift: &OpenShiftTLS{
					TLSAdherencePolicy: configv1.TLSAdherencePolicyNoOpinion,
				},
			},
		},
		{
			name: "LegacyAdheringComponentsOnly does not honor profile",
			apiServer: &configv1.APIServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec: configv1.APIServerSpec{
					TLSAdherence: configv1.TLSAdherencePolicyLegacyAdheringComponentsOnly,
					TLSSecurityProfile: &configv1.TLSSecurityProfile{
						Type: configv1.TLSProfileModernType,
					},
				},
			},
			expected: TLSConfig{
				OpenShift: &OpenShiftTLS{
					TLSAdherencePolicy: configv1.TLSAdherencePolicyLegacyAdheringComponentsOnly,
				},
			},
		},
		{
			name: "StrictAllComponents honors profile and populates cipher suites",
			apiServer: &configv1.APIServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec: configv1.APIServerSpec{
					TLSAdherence: configv1.TLSAdherencePolicyStrictAllComponents,
					TLSSecurityProfile: &configv1.TLSSecurityProfile{
						Type: configv1.TLSProfileOldType,
					},
				},
			},
			expected: TLSConfig{
				OpenShift: &OpenShiftTLS{
					TLSAdherencePolicy: configv1.TLSAdherencePolicyStrictAllComponents,
				},
			},
		},
		{
			name: "StrictAllComponents with nil profile uses default (intermediate)",
			apiServer: &configv1.APIServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec: configv1.APIServerSpec{
					TLSAdherence: configv1.TLSAdherencePolicyStrictAllComponents,
				},
			},
			expected: TLSConfig{
				OpenShift: &OpenShiftTLS{
					TLSAdherencePolicy: configv1.TLSAdherencePolicyStrictAllComponents,
				},
			},
		},
		{
			name:    "APIServer not found returns error",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(scheme)
			if tt.apiServer != nil {
				builder = builder.WithObjects(tt.apiServer)
			}
			cl := builder.Build()

			tlsConfig, err := NewTLSConfigForOpenShift(t.Context(), log, cl)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, tlsConfig)
			require.NotNil(t, tlsConfig.OpenShift)

			assert.Equal(t, tt.expected.OpenShift.TLSAdherencePolicy, tlsConfig.OpenShift.TLSAdherencePolicy)

			if tt.expected.OpenShift.TLSAdherencePolicy == configv1.TLSAdherencePolicyStrictAllComponents {
				assert.NotEmpty(t, tlsConfig.CipherSuites)
				require.NotNil(t, tlsConfig.OpenShift.TLSConfigFunc)
				goTLS := &tls.Config{MinVersion: tls.VersionTLS12}
				tlsConfig.OpenShift.TLSConfigFunc(goTLS)
				assert.NotEmpty(t, goTLS.CipherSuites)
			} else {
				assert.Empty(t, tlsConfig.CipherSuites)
				assert.Nil(t, tlsConfig.OpenShift.TLSConfigFunc)
			}
		})
	}
}
