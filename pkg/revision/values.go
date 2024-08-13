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

	"github.com/istio-ecosystem/sail-operator/api/v1alpha1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
	"github.com/istio-ecosystem/sail-operator/pkg/istiovalues"
)

// ComputeValues computes the Istio Helm values for an IstioRevision as follows:
// - applies image digests from the operator configuration
// - applies the user-provided values on top of the default values from the default and user-selected profiles
// - applies overrides that are not configurable by the user
func ComputeValues(
	userValues *v1alpha1.Values, namespace string, version string,
	defaultProfile, userProfile string, resourceDir string,
	activeRevisionName string,
) (*v1alpha1.Values, error) {
	// apply image digests from configuration, if not already set by user
	userValues = istiovalues.ApplyDigests(version, userValues, config.Config)

	// apply userValues on top of defaultValues from profiles
	mergedHelmValues, err := istiovalues.ApplyProfiles(resourceDir, version, defaultProfile, userProfile, helm.FromValues(userValues))
	if err != nil {
		return nil, fmt.Errorf("failed to apply profile: %w", err)
	}

	values, err := helm.ToValues(mergedHelmValues, &v1alpha1.Values{})
	if err != nil {
		return nil, fmt.Errorf("conversion to Helm values failed: %w", err)
	}

	// override values that are not configurable by the user
	istiovalues.ApplyOverrides(activeRevisionName, namespace, values)
	return values, nil
}
