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
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"

	"istio.io/istio/pkg/ptr"
)

func ApplyOverrides(revisionName string, namespace string, values *v1.Values) {
	// Set revision name to "" if revision name is "default". This is a temporary fix until we fix the injection
	// mutatingwebhook manifest; the webhook performs injection on namespaces labeled with "istio-injection: enabled"
	// only when revision is "", but not also for "default", which it should, since elsewhere in the same manifest,
	// the "" revision is mapped to "default".
	if revisionName == v1.DefaultRevision {
		revisionName = ""
	}
	values.Revision = &revisionName

	if values.Global == nil {
		values.Global = &v1.GlobalConfig{}
	}
	values.Global.IstioNamespace = &namespace

	// Force defaultRevision to be empty to prevent creation of the default validator webhook.
	// This field is deprecated and should be ignored by the operator.
	values.DefaultRevision = ptr.Of("")
}
