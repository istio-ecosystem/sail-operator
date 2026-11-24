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
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// TestPodFilter covers the update filter on the Pod watch of the IstioRevision controller.
// The InUse computation reads exactly three things from a pod: the revision in its injected
// istio.io/rev annotation, the revision implied by its injection labels, and whether it has
// reached a terminal phase. Only changes to those may wake the controller.
func TestPodFilter(t *testing.T) {
	pred := AsPredicate(PodFilter())

	// an injected, running workload pod carrying the kind of finalizer a Job pod has
	basePod := func() *corev1.Pod {
		return &corev1.Pod{
			Name:            "workload",
			Namespace:       "bookinfo",
			ResourceVersion: "1",
			Finalizers:      []string{"batch.kubernetes.io/job-tracking"},
			Labels: map[string]string{
				"app":          "workload",
				"istio.io/rev": "my-rev",
			},
			Annotations: map[string]string{
				"istio.io/rev": "my-rev",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "app", Image: "app:v1"}},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		}
	}

	tests := []struct {
		name      string
		updateOld func(pod *corev1.Pod)
		update    func(pod *corev1.Pod)
		expected  bool
	}{
		{
			name:     "no changes",
			update:   func(pod *corev1.Pod) {},
			expected: false,
		},
		{
			name: "resourceVersion changed",
			update: func(pod *corev1.Pod) {
				pod.ResourceVersion = "2"
			},
			expected: false,
		},

		// platform-driven metadata writes: these are the events that currently wake the
		// controller several times per pod lifecycle and cannot change the InUse answer
		{
			name: "scheduler binding and topology labels added",
			update: func(pod *corev1.Pod) {
				pod.Labels["kubernetes.io/hostname"] = "node-1"
				pod.Labels["topology.kubernetes.io/zone"] = "us-east-1a"
			},
			expected: false,
		},
		{
			name: "CNI pod-network annotation added",
			update: func(pod *corev1.Pod) {
				pod.Annotations["k8s.ovn.org/pod-networks"] = `{"default":{"ip_addresses":["10.128.2.17/23"]}}`
			},
			expected: false,
		},
		{
			name: "Multus network-status annotation added",
			update: func(pod *corev1.Pod) {
				pod.Annotations["k8s.v1.cni.cncf.io/network-status"] = `[{"name":"ovn-kubernetes","interface":"eth0"}]`
			},
			expected: false,
		},
		{
			name: "job-tracking finalizer removed",
			update: func(pod *corev1.Pod) {
				pod.Finalizers = nil
			},
			expected: false,
		},
		{
			name: "deletionTimestamp set",
			update: func(pod *corev1.Pod) {
				pod.DeletionTimestamp = new(metav1.Now())
			},
			expected: false,
		},
		{
			name: "ownerReferences changed",
			update: func(pod *corev1.Pod) {
				pod.OwnerReferences = []metav1.OwnerReference{{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       "workload-123",
				}}
			},
			expected: false,
		},
		{
			name: "spec changed",
			update: func(pod *corev1.Pod) {
				pod.Spec.Containers[0].Image = "app:v2"
			},
			expected: false,
		},
		{
			name: "unrelated label changed",
			update: func(pod *corev1.Pod) {
				pod.Labels["app"] = "other"
			},
			expected: false,
		},
		{
			name: "unrelated annotation changed",
			update: func(pod *corev1.Pod) {
				pod.Annotations["prometheus.io/scrape"] = "true"
			},
			expected: false,
		},

		// phase transitions: only moving in or out of a terminal phase changes the answer,
		// because the InUse computation skips Succeeded and Failed pods
		{
			name: "pending to running",
			updateOld: func(pod *corev1.Pod) {
				pod.Status.Phase = corev1.PodPending
			},
			update: func(pod *corev1.Pod) {
				pod.Status.Phase = corev1.PodRunning
			},
			expected: false,
		},
		{
			name: "running to succeeded",
			update: func(pod *corev1.Pod) {
				pod.Status.Phase = corev1.PodSucceeded
			},
			expected: true,
		},
		{
			name: "running to failed",
			update: func(pod *corev1.Pod) {
				pod.Status.Phase = corev1.PodFailed
			},
			expected: true,
		},
		{
			name: "succeeded to failed stays terminal",
			updateOld: func(pod *corev1.Pod) {
				pod.Status.Phase = corev1.PodSucceeded
			},
			update: func(pod *corev1.Pod) {
				pod.Status.Phase = corev1.PodFailed
			},
			expected: false,
		},
		{
			name: "container statuses changed but phase unchanged",
			update: func(pod *corev1.Pod) {
				pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "app", RestartCount: 3}}
			},
			expected: false,
		},

		// changes to the properties the InUse computation actually reads
		{
			name: "injected istio.io/rev annotation changed",
			update: func(pod *corev1.Pod) {
				pod.Annotations["istio.io/rev"] = "other-rev"
			},
			expected: true,
		},
		{
			name: "injected istio.io/rev annotation removed",
			update: func(pod *corev1.Pod) {
				delete(pod.Annotations, "istio.io/rev")
			},
			expected: true,
		},
		{
			// podReferencesRevision treats the annotation and the labels as an OR rather than a
			// fallback: the label branch applies whenever the pod's namespace carries no
			// injection label, so one pod can keep two revisions in use at once. A label change
			// therefore has to wake the controller even when the annotation is untouched.
			//
			// Istio itself only acts on this label at admission, since the injector webhook is
			// registered for CREATE only, so in practice this is a hand-edit on a running pod.
			// The case exists to keep the filter a superset of what the computation reads.
			name: "istio.io/rev label changed while annotation unchanged",
			update: func(pod *corev1.Pod) {
				pod.Labels["istio.io/rev"] = "other-rev"
			},
			expected: true,
		},
		{
			name: "istio.io/rev label removed while annotation unchanged",
			update: func(pod *corev1.Pod) {
				delete(pod.Labels, "istio.io/rev")
			},
			expected: true,
		},
		{
			name: "sidecar.istio.io/inject label set to false",
			update: func(pod *corev1.Pod) {
				pod.Labels["sidecar.istio.io/inject"] = "false"
			},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			oldObj := basePod()
			if tc.updateOld != nil {
				tc.updateOld(oldObj)
			}
			newObj := oldObj.DeepCopy()
			newObj.ResourceVersion = "2"
			tc.update(newObj)

			result := pred.Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj})
			g.Expect(result).To(Equal(tc.expected), "unexpected result of predicate.Update()")
		})
	}
}

// TestPodFilterOnNonInjectedPod covers pods that reference no revision at all, which are
// the majority of pods on a cluster. No change to such a pod can affect any IstioRevision, so
// even its terminal transitions must be filtered out; admitting them would make this predicate
// noisier than the IgnoreStatusChanges filter it replaces.
func TestPodFilterOnNonInjectedPod(t *testing.T) {
	shouldReconcile := PodFilter()

	oldObj := &corev1.Pod{
		Name:      "plain",
		Namespace: "default",
		Labels:    map[string]string{"app": "plain"},
		Status:    corev1.PodStatus{Phase: corev1.PodRunning},
	}

	t.Run("metadata churn is ignored", func(t *testing.T) {
		g := NewWithT(t)
		newObj := oldObj.DeepCopy()
		newObj.Labels["kubernetes.io/hostname"] = "node-1"
		newObj.Annotations = map[string]string{"k8s.ovn.org/pod-networks": "{}"}
		g.Expect(shouldReconcile(oldObj, newObj)).To(BeFalse())
	})

	t.Run("terminal transition is ignored", func(t *testing.T) {
		g := NewWithT(t)
		newObj := oldObj.DeepCopy()
		newObj.Status.Phase = corev1.PodSucceeded
		g.Expect(shouldReconcile(oldObj, newObj)).To(BeFalse())
	})

	t.Run("becoming injected is not ignored", func(t *testing.T) {
		g := NewWithT(t)
		newObj := oldObj.DeepCopy()
		newObj.Labels["istio.io/rev"] = "my-rev"
		g.Expect(shouldReconcile(oldObj, newObj)).To(BeTrue())
	})
}

// TestPodFilterOnIstiodPod asserts the istiod pod itself never wakes the controller.
// Its pod template carries sidecar.istio.io/inject=false and no injected istio.io/rev
// annotation, so it references no revision. Readiness is tracked through the istiod
// Deployment watch, not through its pods.
func TestPodFilterOnIstiodPod(t *testing.T) {
	g := NewWithT(t)
	shouldReconcile := PodFilter()

	oldObj := &corev1.Pod{
		Name:      "istiod-my-rev-7d4b8c9f5-abcde",
		Namespace: "istio-system",
		Labels: map[string]string{
			"app":                     "istiod",
			"istio.io/rev":            "my-rev",
			"sidecar.istio.io/inject": "false",
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}

	newObj := oldObj.DeepCopy()
	newObj.Status.Phase = corev1.PodSucceeded
	g.Expect(shouldReconcile(oldObj, newObj)).To(BeFalse())
}

// TestPodFilterPassesCreateAndDelete asserts create and delete events are never filtered:
// a pod appearing or disappearing always has to be reflected in the InUse condition.
func TestPodFilterPassesCreateAndDelete(t *testing.T) {
	g := NewWithT(t)
	pred := AsPredicate(PodFilter())

	pod := &corev1.Pod{Name: "workload", Namespace: "bookinfo"}

	g.Expect(pred.Create(event.CreateEvent{Object: pod})).To(BeTrue(), "create events must not be filtered")
	g.Expect(pred.Delete(event.DeleteEvent{Object: pod})).To(BeTrue(), "delete events must not be filtered")
	g.Expect(pred.Generic(event.GenericEvent{Object: pod})).To(BeTrue(), "generic events must not be filtered")
}

// TestPodFilterOnNonPod asserts the filter fails open if it is ever handed something that
// is not a typed Pod, rather than silently dropping events.
func TestPodFilterOnNonPod(t *testing.T) {
	g := NewWithT(t)
	shouldReconcile := PodFilter()

	oldObj := &corev1.ConfigMap{Name: "cm", ResourceVersion: "1"}
	newObj := &corev1.ConfigMap{Name: "cm", ResourceVersion: "2"}

	g.Expect(shouldReconcile(oldObj, newObj)).To(BeTrue())
}
