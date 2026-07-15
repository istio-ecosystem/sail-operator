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
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"

	"istio.io/istio/pkg/ptr"
)

func TestApplyImageDigests(t *testing.T) {
	testCases := []struct {
		name         string
		config       config.OperatorConfig
		version      string
		inputValues  *v1.Values
		expectValues *v1.Values
	}{
		{
			name: "no-config",
			config: config.OperatorConfig{
				ImageDigests: map[string]config.IstioImageConfig{},
			},
			version: "v1.20.0",
			inputValues: &v1.Values{
				Pilot: &v1.PilotConfig{
					Image: ptr.Of("istiod-test"),
				},
			},
			expectValues: &v1.Values{
				Pilot: &v1.PilotConfig{
					Image: ptr.Of("istiod-test"),
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
			inputValues: &v1.Values{},
			expectValues: &v1.Values{
				Pilot: &v1.PilotConfig{
					Image: ptr.Of("istiod-test"),
				},
				Global: &v1.GlobalConfig{
					Proxy: &v1.ProxyConfig{
						Image: ptr.Of("proxy-test"),
					},
					ProxyInit: &v1.ProxyInitConfig{
						Image: ptr.Of("proxy-test"),
					},
				},
				// ZTunnel: &v1.ZTunnelConfig{
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
			inputValues: &v1.Values{
				Pilot: &v1.PilotConfig{
					Image: ptr.Of("istiod-custom"),
				},
			},
			expectValues: &v1.Values{
				Pilot: &v1.PilotConfig{
					Image: ptr.Of("istiod-custom"),
				},
				Global: &v1.GlobalConfig{
					Proxy: &v1.ProxyConfig{
						Image: ptr.Of("proxy-test"),
					},
					ProxyInit: &v1.ProxyInitConfig{
						Image: ptr.Of("proxy-test"),
					},
				},
				// ZTunnel: &v1.ZTunnelConfig{
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
			inputValues: &v1.Values{
				Pilot: &v1.PilotConfig{
					Hub: ptr.Of("docker.io/istio"),
					Tag: ptr.Of("1.20.1"),
				},
			},
			expectValues: &v1.Values{
				Pilot: &v1.PilotConfig{
					Hub: ptr.Of("docker.io/istio"),
					Tag: ptr.Of("1.20.1"),
				},
				Global: &v1.GlobalConfig{
					Proxy: &v1.ProxyConfig{
						Image: ptr.Of("proxy-test"),
					},
					ProxyInit: &v1.ProxyInitConfig{
						Image: ptr.Of("proxy-test"),
					},
				},
				// ZTunnel: &v1.ZTunnelConfig{
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
			inputValues: &v1.Values{
				Pilot: &v1.PilotConfig{
					Hub: ptr.Of("docker.io/istio"),
					Tag: ptr.Of("1.20.2"),
				},
			},
			expectValues: &v1.Values{
				Pilot: &v1.PilotConfig{
					Hub: ptr.Of("docker.io/istio"),
					Tag: ptr.Of("1.20.2"),
				},
			},
		},
		{
			name: "user-supplied-global-hub",
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
			inputValues: &v1.Values{
				Global: &v1.GlobalConfig{
					Hub: ptr.Of("docker.io/istio"),
				},
			},
			expectValues: &v1.Values{
				Global: &v1.GlobalConfig{
					Hub: ptr.Of("docker.io/istio"),
				},
			},
		},
		{
			name: "user-supplied-global-tag",
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
			inputValues: &v1.Values{
				Global: &v1.GlobalConfig{
					Tag: ptr.Of("1.20.0-custom-build"),
				},
			},
			expectValues: &v1.Values{
				Global: &v1.GlobalConfig{
					Tag: ptr.Of("1.20.0-custom-build"),
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
