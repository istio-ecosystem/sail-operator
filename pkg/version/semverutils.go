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

import "github.com/Masterminds/semver/v3"

// VersionConstraint returns a semver constraint for the given string or panics
// if the string is not a valid semver constraint.
func Constraint(constraint string) semver.Constraints {
	c, err := semver.NewConstraint(constraint)
	if err == nil {
		return *c
	}
	panic(err)
}
