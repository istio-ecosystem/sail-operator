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
	"fmt"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	openshifttls "github.com/openshift/controller-runtime-common/pkg/tls"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	defaultTLSCiphers, _ = cipherCodes(openshifttls.DefaultTLSCiphers)
	modernTLSCiphers, _  = cipherCodes(configv1.TLSProfiles[configv1.TLSProfileModernType].Ciphers)
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
				MinVersion: tls.VersionTLS12,
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
				MinVersion: tls.VersionTLS12,
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
						Type: configv1.TLSProfileModernType,
					},
				},
			},
			expected: TLSConfig{
				MinVersion:   tls.VersionTLS13,
				CipherSuites: modernTLSCiphers,
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
				MinVersion:   tls.VersionTLS12,
				CipherSuites: defaultTLSCiphers,
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
			require := require.New(t)
			assert := assert.New(t)
			builder := fake.NewClientBuilder().WithScheme(scheme)
			if tt.apiServer != nil {
				builder = builder.WithObjects(tt.apiServer)
			}
			cl := builder.Build()

			tlsConfig, err := NewTLSConfigForOpenShift(t.Context(), log, cl)
			if tt.wantErr {
				require.Error(err)
				return
			}
			require.NoError(err)
			require.NotNil(tlsConfig)
			require.NotNil(tlsConfig.OpenShift)

			assert.Equal(tt.expected.OpenShift.TLSAdherencePolicy, tlsConfig.OpenShift.TLSAdherencePolicy)

			if tt.expected.OpenShift.TLSAdherencePolicy == configv1.TLSAdherencePolicyStrictAllComponents {
				assert.Equal(tt.expected.CipherSuites, tlsConfig.CipherSuites)
				assert.Equal(tt.expected.MinVersion, tlsConfig.MinVersion,
					fmt.Sprintf("TLS MinVersion mismatch: expected %s, got %s", tls.VersionName(tt.expected.MinVersion), tls.VersionName(tlsConfig.MinVersion)))
				require.NotNil(tlsConfig.OpenShift.TLSConfigFunc)
			} else {
				assert.Empty(tlsConfig.CipherSuites)
				assert.Nil(tlsConfig.OpenShift.TLSConfigFunc)
			}
		})
	}
}
