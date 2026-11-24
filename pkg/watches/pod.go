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
	"github.com/istio-ecosystem/sail-operator/pkg/revision"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PodReferencesRevision returns a filter that admits an update only when the pod references a
// revision, by its injected istio.io/rev annotation or by its injection labels.
//
// On its own it admits every update to such a pod, so pair it with PodRevisionChanges or
// PodTerminationChanges to limit those to pods in the mesh. This matters most for
// PodTerminationChanges, which otherwise admits every pod on the cluster that completes.
//
// Both the old and the new pod are checked, so a pod that gains or loses its injection
// metadata still passes.
func PodReferencesRevision() ShouldReconcileFunc {
	return func(oldObj, newObj client.Object) bool {
		oldPod, newPod, arePods := asPods(oldObj, newObj)
		if !arePods {
			return true
		}
		return referencesRevision(oldPod) || referencesRevision(newPod)
	}
}

// PodRevisionChanges returns a filter that admits only the pod updates which change the mesh
// revision a pod names: the revision recorded in its injected istio.io/rev annotation, or the
// revision implied by its injection labels.
//
// The annotation and the labels are compared independently, because a pod may carry an
// injected annotation naming one revision and a label naming another, and a caller may act on
// either. Everything else the platform writes to a pod over its lifecycle - scheduler binding
// and topology labels, CNI and network-status annotations, finalizers, owner references, spec
// edits - leaves both untouched.
//
// Note that Istio acts on the injection labels at admission only, since the sidecar injector
// webhook is registered for CREATE. A label edit on a running pod does not re-inject it.
func PodRevisionChanges() ShouldReconcileFunc {
	return func(oldObj, newObj client.Object) bool {
		oldPod, newPod, arePods := asPods(oldObj, newObj)
		if !arePods {
			return true
		}
		return revision.GetInjectedRevisionFromPod(oldPod.GetAnnotations()) !=
			revision.GetInjectedRevisionFromPod(newPod.GetAnnotations()) ||
			revision.GetReferencedRevisionFromPod(oldPod.GetLabels()) !=
				revision.GetReferencedRevisionFromPod(newPod.GetLabels())
	}
}

// PodTerminationChanges returns a filter that admits only the pod updates which move a pod
// into or out of a terminal phase. A pod in Succeeded or Failed has stopped running, so it no
// longer runs whatever revision it was injected with.
//
// Unlike IgnoreStatusChanges, this filter therefore admits a status-only update. Transitions
// between non-terminal phases, such as Pending to Running, and between the two terminal
// phases are not admitted, because neither changes whether the pod is running.
func PodTerminationChanges() ShouldReconcileFunc {
	return func(oldObj, newObj client.Object) bool {
		oldPod, newPod, arePods := asPods(oldObj, newObj)
		if !arePods {
			return true
		}
		return isTerminal(oldPod) != isTerminal(newPod)
	}
}

// asPods type-asserts both objects. A filter that is handed something other than a typed Pod
// cannot tell whether the event matters, so callers reconcile rather than drop it.
func asPods(oldObj, newObj client.Object) (oldPod, newPod *corev1.Pod, arePods bool) {
	oldPod, oldIsPod := oldObj.(*corev1.Pod)
	newPod, newIsPod := newObj.(*corev1.Pod)
	return oldPod, newPod, oldIsPod && newIsPod
}

// referencesRevision reports whether the pod points at a revision by either signal.
func referencesRevision(pod *corev1.Pod) bool {
	return revision.GetInjectedRevisionFromPod(pod.GetAnnotations()) != "" ||
		revision.GetReferencedRevisionFromPod(pod.GetLabels()) != ""
}

// isTerminal reports whether the pod has stopped running. This must stay in step with the
// phases the InUse computations skip.
func isTerminal(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed
}
