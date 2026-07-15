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

// IsUsingRemoteControlPlane returns true if the IstioRevision is configured to
// connect to a remote rather than deploy a local control plane.
func IsUsingRemoteControlPlane(rev *v1.IstioRevision) bool {
	// TODO: we should use values.istiodRemote.enabled instead of the profile, but we can't get the final set of values because of new profiles implementation
	values := rev.Spec.Values
	return values != nil && values.Profile != nil && *values.Profile == "remote"
}
