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

// Package resources provides embedded Perses dashboard manifests vendored from community-mixins.
//
// Dashboards are packaged as a Helm chart:
//   - Chart.yaml
//   - files/*.yaml — PersesDashboard manifests
//   - templates/persesdashboards.yaml — renders dashboards into the release namespace
package resources

import "embed"

// ChartFS contains the embedded Perses dashboards Helm chart.
//
//go:embed Chart.yaml files templates
var ChartFS embed.FS

// ChartPath is the path to the chart root within ChartFS.
const ChartPath = "."
