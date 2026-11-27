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
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// PodReferencesRevision returns a filter that admits an update only when the pod references a
// revision, either by its injected istio.io/rev annotation or by its injection labels.
//
// The filter is a scope rather than a change detector: in isolation it admits every update to
// such a pod. It is intended to be combined with PodRevisionChanges or PodTerminationChanges
// in order to restrict those to pods that belong to a mesh. The restriction is most
// significant for PodTerminationChanges, which otherwise admits the completion of every pod
// in the cluster.
//
// Both the old and the new pod are examined, so that a pod which gains or loses its injection
// metadata is admitted.
func PodReferencesRevision() ShouldReconcileFunc {
	return func(oldObj, newObj client.Object) bool {
		oldPod, newPod, arePods := asPods(oldObj, newObj)
		if !arePods {
			return true
		}
		return referencesRevision(oldPod) || referencesRevision(newPod)
	}
}

// PodRevisionChanges returns a filter that admits only the updates which change the revision a
// pod references, as recorded in its injected istio.io/rev annotation or implied by its
// injection labels.
//
// The annotation and the labels are compared independently, because a pod may carry an
// annotation naming one revision and a label naming another, and a caller may act on either.
// The remaining writes a platform makes to a pod over its lifetime, such as node binding, CNI
// and network-status annotations, finalizers, owner references and spec edits, leave both
// unchanged and are rejected.
//
// Note that Istio itself acts on the injection labels at admission only, because the sidecar
// injector webhook is registered for CREATE. Editing the label on a running pod does not
// cause it to be re-injected.
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

// PodTerminationChanges returns a filter that admits only the updates which move a pod into or
// out of a terminal phase. A pod in Succeeded or Failed has stopped running and therefore no
// longer runs the revision it was injected with.
//
// Unlike IgnoreStatusChanges, this filter admits an update that alters nothing but the status.
// Transitions between non-terminal phases, such as Pending to Running, and transitions between
// the two terminal phases are rejected, because neither changes whether the pod is running.
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

// IgnoreTerminalPodDeletes returns a filter that rejects the deletion of a pod that had
// already reached a terminal phase.
//
// Such a pod no longer runs the revision it was injected with, so it had ceased to count
// towards that revision before the deletion and its removal cannot change the outcome.
// Terminal pods are deleted routinely, for instance when a CronJob prunes its history, so
// rejecting these deletions halves the number of events a completed pod produces.
//
// Deletions of running pods are admitted, as are deletions whose final state the informer did
// not observe, because the object reported for those may be out of date.
func IgnoreTerminalPodDeletes() predicate.Funcs {
	return predicate.Funcs{
		DeleteFunc: func(e event.DeleteEvent) bool {
			if e.DeleteStateUnknown {
				return true
			}
			pod, isPod := e.Object.(*corev1.Pod)
			if !isPod {
				return true
			}
			return !isTerminal(pod)
		},
	}
}
