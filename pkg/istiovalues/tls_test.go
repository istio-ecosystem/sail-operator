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
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
)

func TestApplyTLSConfig(t *testing.T) {
	tests := []struct {
		name         string
		tlsConfig    *config.TLSConfig
		inputValues  helm.Values
		outputValues helm.Values
		expectErr    bool
	}{
		{
			name:         "nil TLS config does not change values",
			tlsConfig:    nil,
			inputValues:  helm.Values{},
			outputValues: helm.Values{},
		},
		{
			name: "applies multiple cipher suites",
			tlsConfig: &config.TLSConfig{
				CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384},
			},
			inputValues: helm.Values{},
			outputValues: helm.Values{
				"meshConfig": map[string]any{
					"meshMTLS": map[string]any{
						"cipherSuites": []any{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384"},
					},
					"tlsDefaults": map[string]any{
						"cipherSuites": []any{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384"},
					},
				},
				"pilot": map[string]any{
					"extraContainerArgs": []any{"--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384"},
				},
			},
		},
		{
			name: "does not override existing cipherSuites",
			tlsConfig: &config.TLSConfig{
				CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
			},
			inputValues: helm.Values{
				"meshConfig": map[string]any{
					"meshMTLS": map[string]any{
						"cipherSuites": []any{"TLS_AES_128_GCM_SHA256"},
					},
					"tlsDefaults": map[string]any{
						"cipherSuites": []any{"TLS_AES_128_GCM_SHA256"},
					},
				},
				"pilot": map[string]any{
					"extraContainerArgs": []any{"--tls-cipher-suites=TLS_AES_128_GCM_SHA256"},
				},
			},
			outputValues: helm.Values{
				"meshConfig": map[string]any{
					"meshMTLS": map[string]any{
						"cipherSuites": []any{"TLS_AES_128_GCM_SHA256"},
					},
					"tlsDefaults": map[string]any{
						"cipherSuites": []any{"TLS_AES_128_GCM_SHA256"},
					},
				},
				"pilot": map[string]any{
					"extraContainerArgs": []any{"--tls-cipher-suites=TLS_AES_128_GCM_SHA256"},
				},
			},
		},
		{
			name: "error when meshConfig is not a map",
			tlsConfig: &config.TLSConfig{
				CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
			},
			inputValues: helm.Values{
				"meshConfig": "not a map",
			},
			expectErr: true,
		},
		{
			name: "error when pilot is not a map",
			tlsConfig: &config.TLSConfig{
				CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
			},
			inputValues: helm.Values{
				"pilot": "not a map",
			},
			expectErr: true,
		},
		{
			name: "preserves existing extraContainerArgs",
			tlsConfig: &config.TLSConfig{
				CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
			},
			inputValues: helm.Values{
				"pilot": map[string]any{
					"extraContainerArgs": []any{"--some-arg=value"},
				},
			},
			outputValues: helm.Values{
				"meshConfig": map[string]any{
					"meshMTLS": map[string]any{
						"cipherSuites": []any{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
					},
					"tlsDefaults": map[string]any{
						"cipherSuites": []any{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
					},
				},
				"pilot": map[string]any{
					"extraContainerArgs": []any{"--some-arg=value", "--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
				},
			},
		},
		{
			name: "error when extraContainerArgs is not a slice",
			tlsConfig: &config.TLSConfig{
				CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
			},
			inputValues: helm.Values{
				"pilot": map[string]any{
					"extraContainerArgs": "not a slice",
				},
			},
			outputValues: helm.Values{
				"meshConfig": map[string]any{
					"meshMTLS": map[string]any{
						"cipherSuites": []any{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
					},
					"tlsDefaults": map[string]any{
						"cipherSuites": []any{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
					},
				},
				"pilot": map[string]any{
					"extraContainerArgs": []any{"--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ApplyTLSConfig(tt.tlsConfig, tt.inputValues)
			checkError(t, err, tt.expectErr)
			if tt.expectErr {
				return
			}

			if diff := cmp.Diff(tt.outputValues, result); diff != "" {
				t.Errorf("ApplyTLSConfig() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func checkError(t *testing.T, err error, expectErr bool) {
	t.Helper()
	if expectErr {
		if err == nil {
			t.Fatal("Expected an error, but got nil")
		}
	} else {
		if err != nil {
			t.Fatalf("Expected no error, but got an error: %v", err)
		}
	}
}
