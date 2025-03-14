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

import v1 "github.com/istio-ecosystem/sail-operator/api/v1"

// DependsOnIstioCNI returns true if CNI is enabled in the revision
func DependsOnIstioCNI(rev *v1.IstioRevision) bool {
	// TODO: get actual final values and inspect pilot.cni.enabled
	values := rev.Spec.Values
	if values == nil {
		return false
	}
	global := values.Global
	pilot := values.Pilot
	return (global != nil && global.Platform != nil && *global.Platform == "openshift") ||
		(pilot != nil && pilot.Cni != nil && pilot.Cni.Enabled != nil && *pilot.Cni.Enabled)
}
