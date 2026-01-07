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
	"os"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	configv1 "github.com/openshift/api/config/v1"
)

var testImages = IstioImageConfig{
	IstiodImage:  "istiod-test",
	ProxyImage:   "proxy-test",
	CNIImage:     "cni-test",
	ZTunnelImage: "ztunnel-test",
}

func TestRead(t *testing.T) {
	testCases := []struct {
		name           string
		configFile     string
		expectedConfig OperatorConfig
		success        bool
	}{
		{
			name: "single-version",
			configFile: `
images.v1_20_0.istiod=istiod-test
images.v1_20_0.proxy=proxy-test
images.v1_20_0.cni=cni-test
images.v1_20_0.ztunnel=ztunnel-test
`,
			expectedConfig: OperatorConfig{
				ImageDigests: map[string]IstioImageConfig{
					"v1.20.0": testImages,
				},
			},
			success: true,
		},
		{
			name: "multiple-versions",
			configFile: `
images.v1_20_0.istiod=istiod-test
images.v1_20_0.proxy=proxy-test
images.v1_20_0.cni=cni-test
images.v1_20_0.ztunnel=ztunnel-test
images.v1_20_1.istiod=istiod-test
images.v1_20_1.proxy=proxy-test
images.v1_20_1.cni=cni-test
images.v1_20_1.ztunnel=ztunnel-test
images.latest.istiod=istiod-test
images.latest.proxy=proxy-test
images.latest.cni=cni-test
images.latest.ztunnel=ztunnel-test
`,
			expectedConfig: OperatorConfig{
				ImageDigests: map[string]IstioImageConfig{
					"v1.20.0": testImages,
					"v1.20.1": testImages,
					"latest":  testImages,
				},
			},
			success: true,
		},
		{
			name: "missing-proxy",
			configFile: `
images.v1_20_0.istiod=istiod-test
images.v1_20_0.cni=cni-test
images.v1_20_0.ztunnel=ztunnel-test
`,
			success: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			file, err := os.CreateTemp("", "operator-unit-")
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				err = file.Close()
				if err != nil {
					t.Fatal(err)
				}
				err = os.Remove(file.Name())
				if err != nil {
					t.Fatal(err)
				}
			}()

			_, err = file.WriteString(tc.configFile)
			if err != nil {
				t.Fatal(err)
			}
			err = Read(file.Name())
			if !tc.success {
				if err != nil {
					return
				}
				t.Fatal("expected error but got:", err)
			} else if err != nil {
				t.Fatal("expected no error but got:", err)
			}
			if diff := cmp.Diff(Config, tc.expectedConfig); diff != "" {
				t.Fatal("config did not match expectation:\n\n", diff)
			}
		})
	}
}

func TestTLSConfigFromAPIServer(t *testing.T) {
	// This list is pulled from: https://pkg.go.dev/github.com/openshift/api@v0.0.0-20260116135531-36664f770c0a/config/v1#TLSSecurityProfile
	expectedIntermediateCiphers := []uint16{
		tls.TLS_AES_128_GCM_SHA256,
		tls.TLS_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_CHACHA20_POLY1305_SHA256,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		// TODO: These are left out of the openshift crypto conversion for some reason.
		// tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
		// tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
	}
	tests := map[string]struct {
		apiServer         *configv1.APIServer
		expectedCipherIDs []uint16
	}{
		"custom TLS profile with ciphers": {
			apiServer: &configv1.APIServer{
				Spec: configv1.APIServerSpec{
					TLSSecurityProfile: &configv1.TLSSecurityProfile{
						Type: configv1.TLSProfileCustomType,
						Custom: &configv1.CustomTLSProfile{
							TLSProfileSpec: configv1.TLSProfileSpec{
								Ciphers: []string{"ECDHE-RSA-AES128-GCM-SHA256"},
							},
						},
					},
				},
			},
			expectedCipherIDs: []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256},
		},
		"default to intermediate when no profile is set": {
			apiServer: &configv1.APIServer{
				Spec: configv1.APIServerSpec{
					TLSSecurityProfile: nil,
				},
			},
			expectedCipherIDs: expectedIntermediateCiphers,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			result := TLSConfigFromAPIServer(tt.apiServer)
			var resultIDs []uint16
			for _, cs := range result.CipherSuites {
				resultIDs = append(resultIDs, cs.ID)
			}
			slices.Sort(resultIDs)
			slices.Sort(tt.expectedCipherIDs)
			if diff := cmp.Diff(resultIDs, tt.expectedCipherIDs); diff != "" {
				t.Errorf("unexpected cipher suite IDs; diff (-expected, +actual):\n%v", diff)
			}
		})
	}
}
