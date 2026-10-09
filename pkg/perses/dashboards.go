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

	persesDashboardResource = "persesdashboards"

	// RequiredDatasourceName is the PersesDatasource metadata.name expected by community-mixins
	// dashboards. Users must create a datasource with this name in the operator namespace;
	// see pkg/perses/resources/files/README.md.
	RequiredDatasourceName = "prometheus-datasource"

	dashboardsDir     = "files"
	persesReleaseName = "sail-perses-dashboards"

	persesGroup   = "perses.dev"
	persesVersion = "v1alpha2"
)

// DashboardGVK is the GroupVersionKind for PersesDashboard resources.
var DashboardGVK = schema.GroupVersionKind{Group: persesGroup, Version: persesVersion, Kind: "PersesDashboard"}

// PersesDashboardGVR is the API version bundled manifests use when waiting for the CRD.
var PersesDashboardGVR = schema.GroupVersionResource{
	Group:    persesGroup,
	Version:  persesVersion,
	Resource: persesDashboardResource,
}
