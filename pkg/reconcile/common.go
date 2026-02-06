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

// Package reconcile provides shared reconciliation logic for Istio components.
// It is used by both the operator controllers and the install library to ensure
// consistent behavior across different deployment modes.
package reconcile

import (
	"io/fs"
	"path"

	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
)

// Config holds configuration needed for component reconciliation.
// It contains all the dependencies required by reconcilers to validate,
// compute values, and install Helm charts.
type Config struct {
	// ResourceFS is the filesystem containing Istio charts and profiles
	ResourceFS fs.FS

	// Platform is the target Kubernetes platform (e.g., OpenShift, vanilla Kubernetes)
	Platform config.Platform

	// DefaultProfile is the base profile applied before user-selected profiles
	DefaultProfile string

	// OperatorNamespace is the namespace where the operator is running
	OperatorNamespace string

	// ChartManager handles Helm chart installation and upgrades
	ChartManager *helm.ChartManager
}

// GetChartPath returns the path to a chart for a given version.
func GetChartPath(version, chartName string) string {
	return path.Join(version, "charts", chartName)
}
