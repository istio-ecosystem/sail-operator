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

package perses

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// PersesDashboardCRD is the name of the PersesDashboard CustomResourceDefinition.
	PersesDashboardCRD = "persesdashboards.perses.dev"

	// RequiredDatasourceName is the PersesDatasource metadata.name expected by community-mixins
	// dashboards. Users must create a datasource with this name in the operator namespace;
	// see resources/perses/dashboards/README.md.
	RequiredDatasourceName = "prometheus-datasource"

	persesGroup   = "perses.dev"
	persesVersion = "v1alpha2"
)

// DashboardGVK is the GroupVersionKind for PersesDashboard resources.
var DashboardGVK = schema.GroupVersionKind{Group: persesGroup, Version: persesVersion, Kind: "PersesDashboard"}

// DashboardDefinition maps a product dashboard to its PersesDashboard CR metadata.name.
type DashboardDefinition struct {
	Name     string
	Filename string
}

// ProductDashboards is the supported GA dashboard set (community-mixins operator YAML).
// Dashboard IDs must remain stable for Kiali and other consumers.
var ProductDashboards = []DashboardDefinition{
	{Name: "istio-control-plane", Filename: "istio-control-plane.yaml"},
	{Name: "istio-mesh-dashboard", Filename: "istio-mesh-dashboard.yaml"},
	{Name: "istio-performance", Filename: "istio-performance.yaml"},
	{Name: "istio-service-dashboard", Filename: "istio-service-dashboard.yaml"},
	{Name: "istio-workload-dashboard", Filename: "istio-workload-dashboard.yaml"},
	{Name: "istio-ztunnel-dashboard", Filename: "istio-ztunnel-dashboard.yaml"},
}
