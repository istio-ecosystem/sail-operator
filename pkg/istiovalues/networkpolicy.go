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
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
)

func shouldEnableNetworkPolicy(ocpVersion *config.OCPVersion) bool {
	return ocpVersion != nil && ocpVersion.Major >= 5
}

// ApplyNetworkPolicyDefaults sets global.networkPolicy.enabled to true on a
// fresh OCP 5+ installation. For existing releases it restores the value
// previously supplied to Helm, so an OCP 4 to 5 upgrade cannot introduce a
// NetworkPolicy. Values explicitly set by the user always take precedence.
func ApplyNetworkPolicyDefaults(ocpVersion *config.OCPVersion, values *helm.Values, existingReleaseValues helm.Values) error {
	if _, found, err := values.GetBool("global.networkPolicy.enabled"); err != nil || found {
		return err
	}

	if existingReleaseValues != nil {
		enabled, found, err := existingReleaseValues.GetBool("global.networkPolicy.enabled")
		if err != nil || !found {
			return err
		}
		return values.Set("global.networkPolicy.enabled", enabled)
	}

	if shouldEnableNetworkPolicy(ocpVersion) {
		return values.Set("global.networkPolicy.enabled", true)
	}
	return nil
}
