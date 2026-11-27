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

import "fmt"

// GatewayClassDefaultsLabel is the label that the istiod Helm chart sets on the
// ConfigMap it renders for a GatewayClass. The label's value is the name of the
// GatewayClass the ConfigMap holds defaults for.
const GatewayClassDefaultsLabel = "gateway.istio.io/defaults-for-class"

// defaultRevision is the revision name the istiod Helm chart substitutes when
// the chart is rendered without a revision.
const defaultRevision = "default"

// GatewayClassDefaultsConfigMapName returns the name of the ConfigMap that the
// istiod Helm chart renders to hold the default Gateway API configuration for
// the given GatewayClass. revision is the Istio revision name passed to the
// chart (for example, the name of the Istio or IstioRevision resource); an
// empty revision maps to "default", matching the chart template's
// `default "default"` fallback.
//
// The name and label are owned by the bundled Istio chart. This helper mirrors
// resources/<version>/charts/istiod/templates/gateway-class-configmap.yaml so
// that consumers of this library do not have to re-derive the format, which
// could drift when the bundled Istio version changes.
func GatewayClassDefaultsConfigMapName(revision, gatewayClassName string) string {
	if revision == "" {
		revision = defaultRevision
	}
	return fmt.Sprintf("istio-%s-gatewayclass-%s", revision, gatewayClassName)
}
