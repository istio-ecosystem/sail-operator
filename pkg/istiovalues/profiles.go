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

package istiovalues

import (
	"fmt"
	"io/fs"
	"path"

	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
	"github.com/istio-ecosystem/sail-operator/pkg/reconciler"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"istio.io/istio/pkg/util/sets"
)

// ApplyProfilesAndPlatform loads profiles from an fs.FS and applies them with platform settings.
// Works with embed.FS, os.DirFS, or any other fs.FS implementation.
func ApplyProfilesAndPlatform(
	resourceFS fs.FS, version string, platform config.Platform, defaultProfile, userProfile string, userValues helm.Values,
) (helm.Values, error) {
	profile := resolve(defaultProfile, userProfile)
	profilesPath := path.Join(version, "profiles")
	defaultValues, err := getValuesFromProfiles(resourceFS, profilesPath, profile)
	if err != nil {
		return nil, fmt.Errorf("failed to get values from profile %q: %w", profile, err)
	}
	values := helm.Values(mergeOverwrite(defaultValues, userValues))

	if platform != config.PlatformKubernetes && platform != config.PlatformUndefined {
		if err = values.SetIfAbsent("global.platform", string(platform)); err != nil {
			return nil, fmt.Errorf("failed to set global.platform: %w", err)
		}
	}
	return values, nil
}

func ApplyUserValues(mergedValues, userValues helm.Values,
) (helm.Values, error) {
	values := helm.Values(mergeOverwrite(mergedValues, userValues))
	return values, nil
}

func resolve(defaultProfile, userProfile string) []string {
	switch {
	case userProfile != "" && userProfile != "default":
		return []string{"default", userProfile}
	case defaultProfile != "" && defaultProfile != "default":
		return []string{"default", defaultProfile}
	default:
		return []string{"default"}
	}
}

func getValuesFromProfiles(resourceFS fs.FS, profilesDir string, profiles []string) (helm.Values, error) {
	// start with an empty values map
	values := helm.Values{}

	// apply profiles in order, overwriting values from previous profiles
	alreadyApplied := sets.New[string]()
	for _, profile := range profiles {
		if profile == "" {
			return nil, reconciler.NewValidationError("profile name cannot be empty")
		}
		if alreadyApplied.Contains(profile) {
			continue
		}
		alreadyApplied.Insert(profile)

		file := path.Join(profilesDir, profile+".yaml")
		// prevent path traversal attacks
		if path.Dir(file) != profilesDir {
			return nil, reconciler.NewValidationError(fmt.Sprintf("invalid profile name %s", profile))
		}

		profileValues, err := getProfileValues(resourceFS, file)
		if err != nil {
			return nil, err
		}
		values = mergeOverwrite(values, profileValues)
	}

	return values, nil
}

func getProfileValues(resourceFS fs.FS, file string) (helm.Values, error) {
	fileContents, err := fs.ReadFile(resourceFS, file)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile file %v: %w", file, err)
	}

	return parseProfileYAML(fileContents, file)
}

// parseProfileYAML parses the profile YAML content and extracts spec.values
func parseProfileYAML(fileContents []byte, filename string) (helm.Values, error) {
	var profile map[string]any
	err := yaml.Unmarshal(fileContents, &profile)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal profile YAML %s: %w", filename, err)
	}

	val, found, err := unstructured.NestedFieldNoCopy(profile, "spec", "values")
	if !found || err != nil {
		return nil, err
	}
	m, ok := val.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("spec.values is not a map[string]any")
	}
	return m, nil
}

func mergeOverwrite(base map[string]any, overrides map[string]any) map[string]any {
	if base == nil {
		base = make(map[string]any, 1)
	}

	for key, value := range overrides {
		// if the key doesn't already exist, add it
		if _, exists := base[key]; !exists {
			base[key] = value
			continue
		}

		// At this point, key exists in both base and overrides.
		// If both are maps, recurse so that we override only specific values in the map.
		// If only override value is a map, overwrite base value completely.
		// If both are values, overwrite base.
		childOverrides, overrideValueIsMap := value.(map[string]any)
		childBase, baseValueIsMap := base[key].(map[string]any)
		if baseValueIsMap && overrideValueIsMap {
			base[key] = mergeOverwrite(childBase, childOverrides)
		} else {
			base[key] = value
		}
	}
	return base
}
