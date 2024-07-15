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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/istio-ecosystem/sail-operator/api/v1alpha1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
)

func TestApplyImageDigests(t *testing.T) {
	testCases := []struct {
		name         string
		config       config.OperatorConfig
		version      string
		inputValues  *v1alpha1.Values
		expectValues *v1alpha1.Values
	}{
		{
			name: "no-config",
			config: config.OperatorConfig{
				ImageDigests: map[string]config.IstioImageConfig{},
			},
			version: "v1.20.0",
			inputValues: &v1alpha1.Values{
				Pilot: &v1alpha1.PilotConfig{
					Image: "istiod-test",
				},
			},
			expectValues: &v1alpha1.Values{
				Pilot: &v1alpha1.PilotConfig{
					Image: "istiod-test",
				},
			},
		},
		{
			name: "no-user-values",
			config: config.OperatorConfig{
				ImageDigests: map[string]config.IstioImageConfig{
					"v1.20.0": {
						IstiodImage:  "istiod-test",
						ProxyImage:   "proxy-test",
						ZTunnelImage: "ztunnel-test",
					},
				},
			},
			version:     "v1.20.0",
			inputValues: &v1alpha1.Values{},
			expectValues: &v1alpha1.Values{
				Pilot: &v1alpha1.PilotConfig{
					Image: "istiod-test",
				},
				Global: &v1alpha1.GlobalConfig{
					Proxy: &v1alpha1.ProxyConfig{
						Image: "proxy-test",
					},
					ProxyInit: &v1alpha1.ProxyInitConfig{
						Image: "proxy-test",
					},
				},
				// ZTunnel: &v1alpha1.ZTunnelConfig{
				// 	Image: "ztunnel-test",
				// },
			},
		},
		{
			name: "user-supplied-image",
			config: config.OperatorConfig{
				ImageDigests: map[string]config.IstioImageConfig{
					"v1.20.0": {
						IstiodImage:  "istiod-test",
						ProxyImage:   "proxy-test",
						ZTunnelImage: "ztunnel-test",
					},
				},
			},
			version: "v1.20.0",
			inputValues: &v1alpha1.Values{
				Pilot: &v1alpha1.PilotConfig{
					Image: "istiod-custom",
				},
			},
			expectValues: &v1alpha1.Values{
				Pilot: &v1alpha1.PilotConfig{
					Image: "istiod-custom",
				},
				Global: &v1alpha1.GlobalConfig{
					Proxy: &v1alpha1.ProxyConfig{
						Image: "proxy-test",
					},
					ProxyInit: &v1alpha1.ProxyInitConfig{
						Image: "proxy-test",
					},
				},
				// ZTunnel: &v1alpha1.ZTunnelConfig{
				// 	Image: "ztunnel-test",
				// },
			},
		},
		{
			name: "user-supplied-hub-tag",
			config: config.OperatorConfig{
				ImageDigests: map[string]config.IstioImageConfig{
					"v1.20.0": {
						IstiodImage:  "istiod-test",
						ProxyImage:   "proxy-test",
						ZTunnelImage: "ztunnel-test",
					},
				},
			},
			version: "v1.20.0",
			inputValues: &v1alpha1.Values{
				Pilot: &v1alpha1.PilotConfig{
					Hub: "docker.io/istio",
					Tag: "1.20.1",
				},
			},
			expectValues: &v1alpha1.Values{
				Pilot: &v1alpha1.PilotConfig{
					Hub: "docker.io/istio",
					Tag: "1.20.1",
				},
				Global: &v1alpha1.GlobalConfig{
					Proxy: &v1alpha1.ProxyConfig{
						Image: "proxy-test",
					},
					ProxyInit: &v1alpha1.ProxyInitConfig{
						Image: "proxy-test",
					},
				},
				// ZTunnel: &v1alpha1.ZTunnelConfig{
				// 	Image: "ztunnel-test",
				// },
			},
		},
		{
			name: "version-without-defaults",
			config: config.OperatorConfig{
				ImageDigests: map[string]config.IstioImageConfig{
					"v1.20.0": {
						IstiodImage:  "istiod-test",
						ProxyImage:   "proxy-test",
						ZTunnelImage: "ztunnel-test",
					},
				},
			},
			version: "v1.20.1",
			inputValues: &v1alpha1.Values{
				Pilot: &v1alpha1.PilotConfig{
					Hub: "docker.io/istio",
					Tag: "1.20.2",
				},
			},
			expectValues: &v1alpha1.Values{
				Pilot: &v1alpha1.PilotConfig{
					Hub: "docker.io/istio",
					Tag: "1.20.2",
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ApplyDigests(tc.version, tc.inputValues, tc.config)
			if diff := cmp.Diff(tc.expectValues, result); diff != "" {
				t.Errorf("unexpected merge result; diff (-expected, +actual):\n%v", diff)
			}
		})
	}
}
