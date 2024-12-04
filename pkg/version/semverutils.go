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

package version

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
)

// VersionConstraint returns a semver constraint for the given string or panics
// if the string is not a valid semver constraint.
func Constraint(constraint string) semver.Constraints {
	c, err := semver.NewConstraint(constraint)
	if err == nil {
		return *c
	}
	panic(err)
}

func IsSupported(version string) error {
	if version == "" {
		return fmt.Errorf("spec.version not set")
	}
	if version == "latest" {
		version = "v999.999.999"
	}
	semanticVersion, err := semver.NewVersion(version)
	if err != nil {
		return fmt.Errorf("spec.version is not a valid semver: %s", err.Error())
	}
	if config.Config.MaximumIstioVersion != nil && semanticVersion.GreaterThan(config.Config.MaximumIstioVersion) {
		return fmt.Errorf("spec.version is not supported")
	}
	return nil
}
