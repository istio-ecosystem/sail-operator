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
	configv1 "github.com/openshift/api/config/v1"
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
			name: "adds tls-min-version TLS 1.3 for istio 1.29+ and remove cipherSuites from meshConfig",
			tlsConfig: &config.TLSConfig{
				CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
				MinVersion:   tls.VersionTLS13,
			},
			istioVersion: "1.30.0",
			inputValues:  &v1.Values{},
			wantValues: &v1.Values{
				MeshConfig: &v1.MeshConfig{
					MeshMTLS: &v1.MeshConfigTLSConfig{
						CipherSuites:       nil,
						MinProtocolVersion: v1.MeshConfigTLSConfigTLSProtocolTlsv13,
					},
					TlsDefaults: &v1.MeshConfigTLSConfig{
						CipherSuites:       nil,
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
		{
			name: "adds tls-min-version TLS 1.3 and the right ecdh curves only to TLSDefaults",
			tlsConfig: &config.TLSConfig{
				CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
				MinVersion:   tls.VersionTLS13,
				OpenShift: &config.OpenShiftTLS{
					TLSAdherencePolicy: configv1.TLSAdherencePolicyStrictAllComponents,
					// Matches the group list of every predefined OpenShift profile
					// (Old, Intermediate and Modern all specify these four).
					TLSProfileSpec: configv1.TLSProfileSpec{
						Groups: []configv1.TLSGroup{
							configv1.TLSGroupX25519MLKEM768,
							configv1.TLSGroupX25519,
							configv1.TLSGroupSecP256r1,
							configv1.TLSGroupSecP384r1,
						},
					},
				},
			},
			istioVersion: "1.30.0",
			inputValues:  &v1.Values{},
			wantValues: &v1.Values{
				MeshConfig: &v1.MeshConfig{
					MeshMTLS: &v1.MeshConfigTLSConfig{
						CipherSuites:       nil,
						MinProtocolVersion: v1.MeshConfigTLSConfigTLSProtocolTlsv13,
						EcdhCurves:         nil,
					},
					TlsDefaults: &v1.MeshConfigTLSConfig{
						CipherSuites:       nil,
						MinProtocolVersion: v1.MeshConfigTLSConfigTLSProtocolTlsv13,
						EcdhCurves:         []string{"X25519MLKEM768", "X25519", "P-256", "P-384"},
					},
					DefaultConfig: &v1.MeshConfigProxyConfig{
						ProxyMetadata: map[string]string{
							"OPENSSL_TLS1_3_CIPHERSUITES": "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
						},
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
			name: "does not override existing OPENSSL_TLS1_3_CIPHERSUITES in proxyMetadata",
			tlsConfig: &config.TLSConfig{
				CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
				MinVersion:   tls.VersionTLS13,
				OpenShift:    &config.OpenShiftTLS{},
			},
			istioVersion: "1.30.0",
			inputValues: &v1.Values{
				MeshConfig: &v1.MeshConfig{
					DefaultConfig: &v1.MeshConfigProxyConfig{
						ProxyMetadata: map[string]string{
							"OPENSSL_TLS1_3_CIPHERSUITES": "TLS_AES_256_GCM_SHA384",
						},
					},
				},
			},
			wantValues: &v1.Values{
				MeshConfig: &v1.MeshConfig{
					MeshMTLS: &v1.MeshConfigTLSConfig{
						CipherSuites:       nil,
						MinProtocolVersion: v1.MeshConfigTLSConfigTLSProtocolTlsv13,
					},
					TlsDefaults: &v1.MeshConfigTLSConfig{
						CipherSuites:       nil,
						MinProtocolVersion: v1.MeshConfigTLSConfigTLSProtocolTlsv13,
						EcdhCurves:         defaultECDHCurves,
					},
					DefaultConfig: &v1.MeshConfigProxyConfig{
						ProxyMetadata: map[string]string{
							"OPENSSL_TLS1_3_CIPHERSUITES": "TLS_AES_256_GCM_SHA384",
						},
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
			name: "preserves existing proxyMetadata keys when adding OPENSSL_TLS1_3_CIPHERSUITES",
			tlsConfig: &config.TLSConfig{
				CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
				MinVersion:   tls.VersionTLS13,
				OpenShift:    &config.OpenShiftTLS{},
			},
			istioVersion: "1.30.0",
			inputValues: &v1.Values{
				MeshConfig: &v1.MeshConfig{
					DefaultConfig: &v1.MeshConfigProxyConfig{
						ProxyMetadata: map[string]string{
							"ISTIO_DUAL_STACK": "true",
						},
					},
				},
			},
			wantValues: &v1.Values{
				MeshConfig: &v1.MeshConfig{
					MeshMTLS: &v1.MeshConfigTLSConfig{
						CipherSuites:       nil,
						MinProtocolVersion: v1.MeshConfigTLSConfigTLSProtocolTlsv13,
					},
					TlsDefaults: &v1.MeshConfigTLSConfig{
						CipherSuites:       nil,
						MinProtocolVersion: v1.MeshConfigTLSConfigTLSProtocolTlsv13,
						EcdhCurves:         defaultECDHCurves,
					},
					DefaultConfig: &v1.MeshConfigProxyConfig{
						ProxyMetadata: map[string]string{
							"ISTIO_DUAL_STACK":            "true",
							"OPENSSL_TLS1_3_CIPHERSUITES": "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
						},
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
			name: "preserves existing defaultConfig fields when adding proxyMetadata",
			tlsConfig: &config.TLSConfig{
				CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
				MinVersion:   tls.VersionTLS13,
				OpenShift:    &config.OpenShiftTLS{},
			},
			istioVersion: "1.30.0",
			inputValues: &v1.Values{
				MeshConfig: &v1.MeshConfig{
					DefaultConfig: &v1.MeshConfigProxyConfig{
						DiscoveryAddress: new("istiod.custom-ns.svc:15012"),
					},
				},
			},
			wantValues: &v1.Values{
				MeshConfig: &v1.MeshConfig{
					MeshMTLS: &v1.MeshConfigTLSConfig{
						CipherSuites:       nil,
						MinProtocolVersion: v1.MeshConfigTLSConfigTLSProtocolTlsv13,
					},
					TlsDefaults: &v1.MeshConfigTLSConfig{
						CipherSuites:       nil,
						MinProtocolVersion: v1.MeshConfigTLSConfigTLSProtocolTlsv13,
						EcdhCurves:         defaultECDHCurves,
					},
					DefaultConfig: &v1.MeshConfigProxyConfig{
						DiscoveryAddress: new("istiod.custom-ns.svc:15012"),
						ProxyMetadata: map[string]string{
							"OPENSSL_TLS1_3_CIPHERSUITES": "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
						},
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
			name: "sets OPENSSL_TLS1_3_CIPHERSUITES on OpenShift even when MinVersion is not 1.3",
			tlsConfig: &config.TLSConfig{
				CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
				MinVersion:   tls.VersionTLS12,
				OpenShift:    &config.OpenShiftTLS{},
			},
			istioVersion: "1.30.0",
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
						EcdhCurves:         defaultECDHCurves,
					},
					DefaultConfig: &v1.MeshConfigProxyConfig{
						ProxyMetadata: map[string]string{
							"OPENSSL_TLS1_3_CIPHERSUITES": "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
						},
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
			// tlsAdherence NoOpinion leaves CipherSuites empty, so the cipher-driven
			// configuration is skipped, but the hybrid group must still be offered.
			name: "applies default ECDH curves on OpenShift when the TLS profile is not honored",
			tlsConfig: &config.TLSConfig{
				OpenShift: &config.OpenShiftTLS{},
			},
			inputValues: &v1.Values{},
			wantValues: &v1.Values{
				MeshConfig: &v1.MeshConfig{
					TlsDefaults: &v1.MeshConfigTLSConfig{
						EcdhCurves: defaultECDHCurves,
					},
				},
			},
		},
		{
			// tlsAdherence must be honored: under NoOpinion the profile's groups are ignored
			// just like its ciphers, and the defaults apply instead.
			name: "ignores profile groups when tlsAdherence is NoOpinion",
			tlsConfig: &config.TLSConfig{
				OpenShift: &config.OpenShiftTLS{
					TLSAdherencePolicy: configv1.TLSAdherencePolicyNoOpinion,
					TLSProfileSpec: configv1.TLSProfileSpec{
						Groups: []configv1.TLSGroup{configv1.TLSGroupSecP256r1},
					},
				},
			},
			inputValues: &v1.Values{},
			wantValues: &v1.Values{
				MeshConfig: &v1.MeshConfig{
					TlsDefaults: &v1.MeshConfigTLSConfig{
						EcdhCurves: defaultECDHCurves,
					},
				},
			},
		},
		{
			name: "ignores profile groups when tlsAdherence is LegacyAdheringComponentsOnly",
			tlsConfig: &config.TLSConfig{
				OpenShift: &config.OpenShiftTLS{
					TLSAdherencePolicy: configv1.TLSAdherencePolicyLegacyAdheringComponentsOnly,
					TLSProfileSpec: configv1.TLSProfileSpec{
						Groups: []configv1.TLSGroup{configv1.TLSGroupSecP256r1},
					},
				},
			},
			inputValues: &v1.Values{},
			wantValues: &v1.Values{
				MeshConfig: &v1.MeshConfig{
					TlsDefaults: &v1.MeshConfigTLSConfig{
						EcdhCurves: defaultECDHCurves,
					},
				},
			},
		},
		{
			name: "uses profile groups when tlsAdherence is StrictAllComponents",
			tlsConfig: &config.TLSConfig{
				OpenShift: &config.OpenShiftTLS{
					TLSAdherencePolicy: configv1.TLSAdherencePolicyStrictAllComponents,
					TLSProfileSpec: configv1.TLSProfileSpec{
						Groups: []configv1.TLSGroup{configv1.TLSGroupSecP256r1, configv1.TLSGroupX25519},
					},
				},
			},
			inputValues: &v1.Values{},
			wantValues: &v1.Values{
				MeshConfig: &v1.MeshConfig{
					TlsDefaults: &v1.MeshConfigTLSConfig{
						EcdhCurves: []string{"P-256", "X25519"},
					},
				},
			},
		},
		{
			name: "applies default ECDH curves when tlsAdherence is honored but the profile has no groups",
			tlsConfig: &config.TLSConfig{
				OpenShift: &config.OpenShiftTLS{
					TLSAdherencePolicy: configv1.TLSAdherencePolicyStrictAllComponents,
				},
			},
			inputValues: &v1.Values{},
			wantValues: &v1.Values{
				MeshConfig: &v1.MeshConfig{
					TlsDefaults: &v1.MeshConfigTLSConfig{
						EcdhCurves: defaultECDHCurves,
					},
				},
			},
		},
		{
			name: "does not override user-specified ecdhCurves on OpenShift",
			tlsConfig: &config.TLSConfig{
				OpenShift: &config.OpenShiftTLS{
					TLSAdherencePolicy: configv1.TLSAdherencePolicyStrictAllComponents,
					TLSProfileSpec: configv1.TLSProfileSpec{
						Groups: []configv1.TLSGroup{configv1.TLSGroupSecP256r1},
					},
				},
			},
			inputValues: &v1.Values{
				MeshConfig: &v1.MeshConfig{
					TlsDefaults: &v1.MeshConfigTLSConfig{
						EcdhCurves: []string{"P-384"},
					},
				},
			},
			wantValues: &v1.Values{
				MeshConfig: &v1.MeshConfig{
					TlsDefaults: &v1.MeshConfigTLSConfig{
						EcdhCurves: []string{"P-384"},
					},
				},
			},
		},
		{
			name: "does not apply default ECDH curves off OpenShift",
			tlsConfig: &config.TLSConfig{
				CipherSuites: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
			},
			inputValues: &v1.Values{},
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
