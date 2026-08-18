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
	"k8s.io/utils/ptr"
)

func shouldEnableNetworkPolicy(ocpVersion *config.OCPVersion) bool {
	return ocpVersion != nil && ocpVersion.Major >= 5
}

// ApplyIstioNetworkPolicyDefaults sets global.networkPolicy.enabled to true
// for OCP 5+ clusters, unless the user has explicitly set the value.
func ApplyIstioNetworkPolicyDefaults(ocpVersion *config.OCPVersion, values *v1.Values) *v1.Values {
	if !shouldEnableNetworkPolicy(ocpVersion) {
		return values
	}
	if values.Global == nil {
		values.Global = &v1.GlobalConfig{}
	}
	if values.Global.NetworkPolicy == nil {
		values.Global.NetworkPolicy = &v1.NetworkPolicyConfig{}
	}
	if values.Global.NetworkPolicy.Enabled == nil {
		values.Global.NetworkPolicy.Enabled = ptr.To(true)
	}
	return values
}

// ApplyCNINetworkPolicyDefaults sets global.networkPolicy.enabled to true
// for OCP 5+ clusters, unless the user has explicitly set the value.
func ApplyCNINetworkPolicyDefaults(ocpVersion *config.OCPVersion, values *v1.CNIValues) *v1.CNIValues {
	if !shouldEnableNetworkPolicy(ocpVersion) {
		return values
	}
	if values.Global == nil {
		values.Global = &v1.CNIGlobalConfig{}
	}
	if values.Global.NetworkPolicy == nil {
		values.Global.NetworkPolicy = &v1.NetworkPolicyConfig{}
	}
	if values.Global.NetworkPolicy.Enabled == nil {
		values.Global.NetworkPolicy.Enabled = ptr.To(true)
	}
	return values
}

// ApplyZTunnelNetworkPolicyDefaults sets global.networkPolicy.enabled to true
// for OCP 5+ clusters, unless the user has explicitly set the value.
func ApplyZTunnelNetworkPolicyDefaults(ocpVersion *config.OCPVersion, values *v1.ZTunnelValues) *v1.ZTunnelValues {
	if !shouldEnableNetworkPolicy(ocpVersion) {
		return values
	}
	if values.Global == nil {
		values.Global = &v1.ZTunnelGlobalConfig{}
	}
	if values.Global.NetworkPolicy == nil {
		values.Global.NetworkPolicy = &v1.NetworkPolicyConfig{}
	}
	if values.Global.NetworkPolicy.Enabled == nil {
		values.Global.NetworkPolicy.Enabled = ptr.To(true)
	}
	return values
}
