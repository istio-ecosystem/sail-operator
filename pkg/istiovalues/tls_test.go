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
	gotls "crypto/tls"
	"testing"

	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
)

func TestApplyTLSConfig(t *testing.T) {
	tests := []struct {
		name        string
		tlsConfig   *config.TLSConfig
		inputValues helm.Values
		checkKey    string
		checkValue  any
		shouldError bool
	}{
		{
			name:        "nil TLS config",
			tlsConfig:   nil,
			inputValues: helm.Values{},
			checkKey:    "",
			checkValue:  nil,
			shouldError: false,
		},
		{
			name: "applies cipher suites to tlsDefaults",
			tlsConfig: &config.TLSConfig{
				CipherSuites: []gotls.CipherSuite{
					{ID: gotls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, Name: "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
				},
			},
			inputValues: helm.Values{},
			checkKey:    "meshConfig.tlsDefaults.cipherSuites",
			checkValue:  []any{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
			shouldError: false,
		},
		{
			name: "does not override existing cipherSuites",
			tlsConfig: &config.TLSConfig{
				CipherSuites: []gotls.CipherSuite{
					{ID: gotls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, Name: "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
				},
			},
			inputValues: helm.Values{
				"meshConfig": map[string]any{
					"tlsDefaults": map[string]any{
						"cipherSuites": []any{"TLS_AES_128_GCM_SHA256"},
					},
				},
			},
			checkKey:    "meshConfig.tlsDefaults.cipherSuites",
			checkValue:  []any{"TLS_AES_128_GCM_SHA256"}, // Should keep original value
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ApplyTLSConfig(tt.tlsConfig, tt.inputValues)
			if (err != nil) != tt.shouldError {
				t.Errorf("ApplyTLSConfig() error = %v, shouldError = %v", err, tt.shouldError)
				return
			}

			if tt.checkKey != "" {
				val, found, err := result.GetString(tt.checkKey)
				if err == nil && found {
					if strVal, ok := tt.checkValue.(string); ok && val != strVal {
						t.Errorf("Value at %q = %q, want %q", tt.checkKey, val, strVal)
					}
				}
			}
		})
	}
}

func TestApplyTLSConfig_ExtraContainerArgs(t *testing.T) {
	tests := []struct {
		name                 string
		tlsConfig            *config.TLSConfig
		inputValues          helm.Values
		expectExtraArgsCount int
		expectArgContains    string
	}{
		{
			name: "adds tls-cipher-suites to extraContainerArgs",
			tlsConfig: &config.TLSConfig{
				CipherSuites: []gotls.CipherSuite{
					{ID: gotls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, Name: "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
					{ID: gotls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384, Name: "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384"},
				},
			},
			inputValues:          helm.Values{},
			expectExtraArgsCount: 1,
			expectArgContains:    "--tls-cipher-suites=",
		},
		{
			name: "preserves existing extraContainerArgs",
			tlsConfig: &config.TLSConfig{
				CipherSuites: []gotls.CipherSuite{
					{ID: gotls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256, Name: "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
				},
			},
			inputValues: helm.Values{
				"pilot": map[string]any{
					"extraContainerArgs": []any{"--some-arg=value"},
				},
			},
			expectExtraArgsCount: 2,
			expectArgContains:    "--tls-cipher-suites=",
		},
		{
			name: "does not add extraContainerArgs when empty settings",
			tlsConfig: &config.TLSConfig{
				CipherSuites: []gotls.CipherSuite{},
			},
			inputValues:          helm.Values{},
			expectExtraArgsCount: 0,
			expectArgContains:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ApplyTLSConfig(tt.tlsConfig, tt.inputValues)
			if err != nil {
				t.Errorf("ApplyTLSConfig() error = %v", err)
				return
			}

			args, found, _ := result.GetSlice("pilot.extraContainerArgs")
			if tt.expectExtraArgsCount == 0 {
				if found && len(args) > 0 {
					t.Errorf("Expected no extraContainerArgs, but got %v", args)
				}
				return
			}

			if !found {
				t.Errorf("Expected pilot.extraContainerArgs to be set")
				return
			}

			if len(args) != tt.expectExtraArgsCount {
				t.Errorf("Expected %d extraContainerArgs, got %d: %v", tt.expectExtraArgsCount, len(args), args)
			}

			if tt.expectArgContains != "" {
				found := false
				for _, arg := range args {
					if argStr, ok := arg.(string); ok {
						if len(argStr) >= len(tt.expectArgContains) && argStr[:len(tt.expectArgContains)] == tt.expectArgContains {
							found = true
							break
						}
					}
				}
				if !found {
					t.Errorf("Expected extraContainerArgs to contain %q, got %v", tt.expectArgContains, args)
				}
			}
		})
	}
}
