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
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
)

// ApplyDigests applies image digests from configuration, if not already set by user
func ApplyDigests(version string, values *v1.Values, config config.OperatorConfig) *v1.Values {
	imageDigests, digestsDefined := config.ImageDigests[version]
	// if we don't have default image digests defined for this version, it's a no-op
	if !digestsDefined {
		return values
	}

	// if a global hub or tag value is configured by the user, don't set image digests for any components
	if values != nil && values.Global != nil && (values.Global.Hub != nil || values.Global.Tag != nil) {
		return values
	}

	if values == nil {
		values = &v1.Values{}
	}

	// set image digests for components unless they've been configured by the user
	if values.Pilot == nil {
		values.Pilot = &v1.PilotConfig{}
	}
	if values.Pilot.Image == nil && values.Pilot.Hub == nil && values.Pilot.Tag == nil {
		values.Pilot.Image = &imageDigests.IstiodImage
	}

	if values.Global == nil {
		values.Global = &v1.GlobalConfig{}
	}

	if values.Global.Proxy == nil {
		values.Global.Proxy = &v1.ProxyConfig{}
	}
	if values.Global.Proxy.Image == nil {
		values.Global.Proxy.Image = &imageDigests.ProxyImage
	}

	if values.Global.ProxyInit == nil {
		values.Global.ProxyInit = &v1.ProxyInitConfig{}
	}
	if values.Global.ProxyInit.Image == nil {
		values.Global.ProxyInit.Image = &imageDigests.ProxyImage
	}

	// TODO: add this once the API supports ambient
	// if !hasUserDefinedImage("ztunnel", values) {
	// 	values.ZTunnel.Image = imageDigests.ZTunnelImage
	// }
	return values
}
