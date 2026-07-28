//go:build e2e

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
// WITHOUT WARRANTIES OR Condition OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package update

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
)

// GetTwoConsecutiveAmbientVersions returns consecutive minor versions for ambient mode.
// Ambient requires:
//   - Istio >= 1.24.0
//   - Istio >= 1.28.0 on FIPS clusters
func GetTwoConsecutiveAmbientVersions(fipsCluster bool) (baseVer, newVer istioversion.VersionInfo, err error) {
	minVersion := "1.24.0"
	if fipsCluster {
		minVersion = "1.28.0"
	}
	minVer, err := semver.NewVersion(minVersion)
	if err != nil {
		return baseVer, newVer, fmt.Errorf("failed to parse minimum version %q: %w", minVersion, err)
	}
	return istioversion.GetTwoConsecutiveMinorVersions(minVer)
}

// GetTwoConsecutiveSidecarVersions returns consecutive minor versions for sidecar mode.
// No special version constraints for sidecar mode.
func GetTwoConsecutiveSidecarVersions() (baseVer, newVer istioversion.VersionInfo, err error) {
	minVer, err := semver.NewVersion("1.0.0")
	if err != nil {
		return baseVer, newVer, fmt.Errorf("failed to parse minimum version: %w", err)
	}
	return istioversion.GetTwoConsecutiveMinorVersions(minVer)
}
