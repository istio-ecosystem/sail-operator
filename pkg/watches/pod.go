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

package watches

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PodFilter returns a filter for the Pod watches of the IstioRevision and
// IstioRevisionTag controllers, admitting only updates that can change whether a pod keeps a
// revision or a revision tag in use.
//
// The InUse computation reads exactly three things from a pod: the revision recorded in its
// injected istio.io/rev annotation, the revision implied by its injection labels, and whether
// the pod has reached a terminal phase. Everything else the platform writes to a pod during
// its lifecycle - scheduler binding and topology labels, CNI and network-status annotations,
// controller finalizers, owner references, spec edits - cannot change the answer.
//
// Unlike IgnoreStatusChanges, this filter admits a status-only update when it moves the pod
// into or out of a terminal phase, because the InUse computation skips Succeeded and Failed
// pods. Create and delete events are never filtered.
func PodFilter() ShouldReconcileFunc {
	// TODO(OSSM-16613): not implemented yet, so that the tests covering it fail.
	return func(_, _ client.Object) bool {
		return false
	}
}
