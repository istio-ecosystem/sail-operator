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

package install

import (
	"fmt"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
	"github.com/istio-ecosystem/sail-operator/pkg/istiovalues"

	"istio.io/istio/pkg/ptr"
)

// GatewayAPIDefaults returns Values pre-configured for Gateway API support.
// If namespace is empty, it defaults to "istio-system".
func GatewayAPIDefaults(namespace string) *v1.Values {
	if namespace == "" {
		namespace = "istio-system"
	}
	return &v1.Values{
		Pilot: &v1.PilotConfig{
			Env: map[string]string{
				"PILOT_ENABLE_GATEWAY_API":                       "true",
				"PILOT_ENABLE_GATEWAY_API_STATUS":                "true",
				"PILOT_ENABLE_GATEWAY_API_DEPLOYMENT_CONTROLLER": "true",
			},
		},
		Global: &v1.GlobalConfig{
			IstioNamespace:  ptr.Of(namespace),
			TrustBundleName: ptr.Of("openshift-gateway-ca-cert"),
		},
	}
}

// MergeValues merges overlay values on top of base values.
// Values in overlay take precedence over base.
func MergeValues(base, overlay *v1.Values) (*v1.Values, error) {
	if base == nil {
		return overlay, nil
	}
	if overlay == nil {
		return base, nil
	}
	baseMap := helm.FromValues(base)
	overlayMap := helm.FromValues(overlay)
	merged := istiovalues.MergeOverwrite(baseMap, overlayMap)
	result, err := helm.ToValues(merged, &v1.Values{})
	if err != nil {
		return nil, fmt.Errorf("failed to merge values: %w", err)
	}
	return result, nil
}
