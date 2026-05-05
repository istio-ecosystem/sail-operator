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
	"sort"

	"github.com/Masterminds/semver/v3"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
)

// GetTwoConsecutiveMinorVersions returns the latest patch of two consecutive minor versions
// that satisfy the given constraints.
//
// Parameters:
//   - minVersion: minimum version to consider (e.g., "1.24.0")
//   - skipPrerelease: whether to skip alpha/beta/rc versions
//
// Returns:
//   - old: the second-to-last minor version (latest patch)
//   - new: the last minor version (latest patch)
//   - err: error if fewer than 2 consecutive minor versions are found
func GetTwoConsecutiveMinorVersions(minVersion string, skipPrerelease bool) (baseVer, newVer istioversion.VersionInfo, err error) {
	minVer, err := semver.NewVersion(minVersion)
	if err != nil {
		return istioversion.VersionInfo{}, istioversion.VersionInfo{}, fmt.Errorf("invalid minVersion %q: %w", minVersion, err)
	}

	// Group versions by major.minor
	versionsByMinor := make(map[string][]istioversion.VersionInfo)

	for _, v := range istioversion.List {
		// Skip EOL versions
		if v.EOL {
			continue
		}

		// Skip versions without parsed version
		if v.Version == nil {
			continue
		}

		// Skip versions below minimum
		if v.Version.LessThan(minVer) {
			continue
		}

		// Skip prerelease versions if requested
		if skipPrerelease && v.Version.Prerelease() != "" {
			continue
		}

		majorMinor := fmt.Sprintf("%d.%d", v.Version.Major(), v.Version.Minor())
		versionsByMinor[majorMinor] = append(versionsByMinor[majorMinor], v)
	}

	if len(versionsByMinor) < 2 {
		return istioversion.VersionInfo{}, istioversion.VersionInfo{}, fmt.Errorf("need at least 2 minor versions, found %d", len(versionsByMinor))
	}

	// Get sorted list of minor version keys
	var minorVersionKeys []string
	for key := range versionsByMinor {
		minorVersionKeys = append(minorVersionKeys, key)
	}
	sort.Slice(minorVersionKeys, func(i, j int) bool {
		vi, _ := semver.NewVersion(minorVersionKeys[i] + ".0")
		vj, _ := semver.NewVersion(minorVersionKeys[j] + ".0")
		return vi.LessThan(vj)
	})

	// Get the second-to-last and last minor versions
	oldMinorKey := minorVersionKeys[len(minorVersionKeys)-2]
	newMinorKey := minorVersionKeys[len(minorVersionKeys)-1]

	// Find the latest patch version for each minor version
	baseVersion := findLatestPatch(versionsByMinor[oldMinorKey])
	newVersion := findLatestPatch(versionsByMinor[newMinorKey])

	return baseVersion, newVersion, nil
}

// GetTwoConsecutiveAmbientVersions returns consecutive minor versions for ambient mode.
// Ambient requires:
//   - Istio >= 1.24.0
//   - Istio >= 1.28.0 on FIPS clusters
func GetTwoConsecutiveAmbientVersions(fipsCluster bool) (baseVer, newVer istioversion.VersionInfo, err error) {
	minVersion := "1.24.0"
	if fipsCluster {
		minVersion = "1.28.0"
	}
	return GetTwoConsecutiveMinorVersions(minVersion, true)
}

// GetTwoConsecutiveSidecarVersions returns consecutive minor versions for sidecar mode.
// No special version constraints for sidecar mode.
func GetTwoConsecutiveSidecarVersions() (baseVer, newVer istioversion.VersionInfo, err error) {
	return GetTwoConsecutiveMinorVersions("1.0.0", true)
}

// findLatestPatch returns the version with the highest patch number from a list
func findLatestPatch(versions []istioversion.VersionInfo) istioversion.VersionInfo {
	if len(versions) == 0 {
		return istioversion.VersionInfo{}
	}

	latest := versions[0]
	for _, v := range versions[1:] {
		if v.Version.GreaterThan(latest.Version) {
			latest = v
		}
	}
	return latest
}
