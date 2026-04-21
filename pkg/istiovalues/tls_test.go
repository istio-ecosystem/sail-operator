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

package istiovalues

import (
	"crypto/tls"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
)

func TestApplyTLSConfig(t *testing.T) {
	tests := []struct {
		name         string
		tlsConfig    *config.TLSConfig
		istioVersion string
		inputValues  *v1.Values
		wantValues   *v1.Values
	}{
		{
			name:         "nil TLS config does not change values",
			tlsConfig:    nil,
			istioVersion: "1.29.0",
			inputValues:  &v1.Values{},
			wantValues:   &v1.Values{},
		},
		{
			name:         "nil values is safe",
			tlsConfig:    &config.TLSConfig{CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256}},
			istioVersion: "1.29.0",
			inputValues:  nil,
			wantValues:   nil,
		},
		{
			name: "empty cipher suites does not change values",
			tlsConfig: &config.TLSConfig{
				CipherSuites: []uint16{},
			},
			istioVersion: "1.29.0",
			inputValues:  &v1.Values{},
			wantValues:   &v1.Values{},
		},
		{
			name: "applies multiple cipher suites",
			tlsConfig: &config.TLSConfig{
				CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384},
			},
			istioVersion: "1.28.0",
			inputValues:  &v1.Values{},
			wantValues: &v1.Values{
				MeshConfig: &v1.MeshConfig{
					MeshMTLS: &v1.MeshConfigTLSConfig{
						CipherSuites: []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384"},
					},
					TlsDefaults: &v1.MeshConfigTLSConfig{
						CipherSuites: []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384"},
					},
				},
				Pilot: &v1.PilotConfig{
					ExtraContainerArgs: []string{"--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384"},
				},
			},
		},
		{
			name: "does not override existing cipherSuites",
			tlsConfig: &config.TLSConfig{
				CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
			},
			istioVersion: "1.28.0",
			inputValues: &v1.Values{
				MeshConfig: &v1.MeshConfig{
					MeshMTLS: &v1.MeshConfigTLSConfig{
						CipherSuites: []string{"TLS_AES_128_GCM_SHA256"},
					},
					TlsDefaults: &v1.MeshConfigTLSConfig{
						CipherSuites: []string{"TLS_AES_128_GCM_SHA256"},
					},
				},
				Pilot: &v1.PilotConfig{
					ExtraContainerArgs: []string{"--tls-cipher-suites=TLS_AES_128_GCM_SHA256"},
				},
			},
			wantValues: &v1.Values{
				MeshConfig: &v1.MeshConfig{
					MeshMTLS: &v1.MeshConfigTLSConfig{
						CipherSuites: []string{"TLS_AES_128_GCM_SHA256"},
					},
					TlsDefaults: &v1.MeshConfigTLSConfig{
						CipherSuites: []string{"TLS_AES_128_GCM_SHA256"},
					},
				},
				Pilot: &v1.PilotConfig{
					ExtraContainerArgs: []string{"--tls-cipher-suites=TLS_AES_128_GCM_SHA256"},
				},
			},
		},
		{
			name: "preserves existing extraContainerArgs",
			tlsConfig: &config.TLSConfig{
				CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
			},
			istioVersion: "1.28.0",
			inputValues: &v1.Values{
				Pilot: &v1.PilotConfig{
					ExtraContainerArgs: []string{"--some-arg=value"},
				},
			},
			wantValues: &v1.Values{
				MeshConfig: &v1.MeshConfig{
					MeshMTLS: &v1.MeshConfigTLSConfig{
						CipherSuites: []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
					},
					TlsDefaults: &v1.MeshConfigTLSConfig{
						CipherSuites: []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
					},
				},
				Pilot: &v1.PilotConfig{
					ExtraContainerArgs: []string{"--some-arg=value", "--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
				},
			},
		},
		{
			name: "does not treat arg with shared prefix as already present",
			tlsConfig: &config.TLSConfig{
				CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
			},
			istioVersion: "1.28.0",
			inputValues: &v1.Values{
				Pilot: &v1.PilotConfig{
					ExtraContainerArgs: []string{"--tls-cipher-suites-foo=bar"},
				},
			},
			wantValues: &v1.Values{
				MeshConfig: &v1.MeshConfig{
					MeshMTLS: &v1.MeshConfigTLSConfig{
						CipherSuites: []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
					},
					TlsDefaults: &v1.MeshConfigTLSConfig{
						CipherSuites: []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
					},
				},
				Pilot: &v1.PilotConfig{
					ExtraContainerArgs: []string{"--tls-cipher-suites-foo=bar", "--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
				},
			},
		},
		{
			name: "adds tls-min-version for istio 1.29+",
			tlsConfig: &config.TLSConfig{
				CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
				MinVersion:   tls.VersionTLS12,
			},
			istioVersion: "1.29.0",
			inputValues:  &v1.Values{},
			wantValues: &v1.Values{
				MeshConfig: &v1.MeshConfig{
					MeshMTLS: &v1.MeshConfigTLSConfig{
						CipherSuites:       []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
						MinProtocolVersion: v1.MeshConfigTLSConfigTLSProtocolTlsv12,
					},
					TlsDefaults: &v1.MeshConfigTLSConfig{
						CipherSuites:       []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
						MinProtocolVersion: v1.MeshConfigTLSConfigTLSProtocolTlsv12,
					},
				},
				Pilot: &v1.PilotConfig{
					ExtraContainerArgs: []string{
						"--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
						"--tls-min-version=1.2",
					},
				},
			},
		},
		{
			name: "adds tls-min-version TLS 1.3 for istio 1.29+",
			tlsConfig: &config.TLSConfig{
				CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
				MinVersion:   tls.VersionTLS13,
			},
			istioVersion: "1.30.0",
			inputValues:  &v1.Values{},
			wantValues: &v1.Values{
				MeshConfig: &v1.MeshConfig{
					MeshMTLS: &v1.MeshConfigTLSConfig{
						CipherSuites:       []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
						MinProtocolVersion: v1.MeshConfigTLSConfigTLSProtocolTlsv13,
					},
					TlsDefaults: &v1.MeshConfigTLSConfig{
						CipherSuites:       []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
						MinProtocolVersion: v1.MeshConfigTLSConfigTLSProtocolTlsv13,
					},
				},
				Pilot: &v1.PilotConfig{
					ExtraContainerArgs: []string{
						"--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
						"--tls-min-version=1.3",
					},
				},
			},
		},
		{
			name: "does not add tls-min-version for istio < 1.29",
			tlsConfig: &config.TLSConfig{
				CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
				MinVersion:   tls.VersionTLS12,
			},
			istioVersion: "1.28.3",
			inputValues:  &v1.Values{},
			wantValues: &v1.Values{
				MeshConfig: &v1.MeshConfig{
					MeshMTLS: &v1.MeshConfigTLSConfig{
						CipherSuites:       []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
						MinProtocolVersion: v1.MeshConfigTLSConfigTLSProtocolTlsv12,
					},
					TlsDefaults: &v1.MeshConfigTLSConfig{
						CipherSuites:       []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
						MinProtocolVersion: v1.MeshConfigTLSConfigTLSProtocolTlsv12,
					},
				},
				Pilot: &v1.PilotConfig{
					ExtraContainerArgs: []string{"--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
				},
			},
		},
		{
			name: "does not add tls-min-version when MinVersion is zero",
			tlsConfig: &config.TLSConfig{
				CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
				MinVersion:   0,
			},
			istioVersion: "1.29.0",
			inputValues:  &v1.Values{},
			wantValues: &v1.Values{
				MeshConfig: &v1.MeshConfig{
					MeshMTLS: &v1.MeshConfigTLSConfig{
						CipherSuites: []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
					},
					TlsDefaults: &v1.MeshConfigTLSConfig{
						CipherSuites: []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
					},
				},
				Pilot: &v1.PilotConfig{
					ExtraContainerArgs: []string{"--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ApplyTLSConfig(tt.tlsConfig, tt.istioVersion, tt.inputValues)
			if diff := cmp.Diff(tt.wantValues, tt.inputValues); diff != "" {
				t.Errorf("ApplyTLSConfig() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
