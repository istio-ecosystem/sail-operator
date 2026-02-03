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

package revision

import (
	"fmt"
	"io/fs"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
	"github.com/istio-ecosystem/sail-operator/pkg/istiovalues"
)

// ComputeValues computes the Istio Helm values for an IstioRevision as follows:
// - applies image digests from the operator configuration
// - applies vendor-specific default values
// - applies the user-provided values on top of the default values from the default and user-selected profiles
// - applies overrides that are not configurable by the user
//
// The resourceFS parameter accepts any fs.FS implementation (embed.FS, os.DirFS, etc.).
func ComputeValues(
	userValues *v1.Values, namespace string, version string,
	platform config.Platform, defaultProfile, userProfile string, resourceFS fs.FS,
	activeRevisionName string,
) (*v1.Values, error) {
	// apply image digests from configuration, if not already set by user
	userValues = istiovalues.ApplyDigests(version, userValues, config.Config)

	// apply vendor-specific default values
	userValues, err := istiovalues.ApplyIstioVendorDefaults(version, userValues)
	if err != nil {
		return nil, fmt.Errorf("failed to apply vendor defaults: %w", err)
	}

	// apply userValues on top of defaultValues from profiles
	mergedHelmValues, err := istiovalues.ApplyProfilesAndPlatform(resourceFS, version, platform, defaultProfile, userProfile, helm.FromValues(userValues))
	if err != nil {
		return nil, fmt.Errorf("failed to apply profile: %w", err)
	}

	// apply FipsValues on top of mergedHelmValues from profile
	mergedHelmValues, err = istiovalues.ApplyFipsValues(mergedHelmValues)
	if err != nil {
		return nil, fmt.Errorf("failed to apply FIPS values: %w", err)
	}

	values, err := helm.ToValues(mergedHelmValues, &v1.Values{})
	if err != nil {
		return nil, fmt.Errorf("conversion to Helm values failed: %w", err)
	}

	// override values that are not configurable by the user
	istiovalues.ApplyOverrides(activeRevisionName, namespace, values)
	return values, nil
}
