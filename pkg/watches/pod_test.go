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
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// injectedPod is an injected, running workload pod carrying the kind of finalizer a Job pod
// has. It names revision "my-rev" both by annotation and by label.
func injectedPod() *corev1.Pod {
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

// platformWrites are the metadata writes the platform makes to a pod over its lifecycle. None
// of them change the revision the pod names, nor whether it is running, so neither filter may
// admit them. These are the events that wake the controller several times per pod today.
var platformWrites = []struct {
	name   string
	update func(pod *corev1.Pod)
}{
	{
		name: "scheduler binding and topology labels added",
		update: func(pod *corev1.Pod) {
			pod.Labels["kubernetes.io/hostname"] = "node-1"
			pod.Labels["topology.kubernetes.io/zone"] = "us-east-1a"
		},
	},
	{
		name: "CNI pod-network annotation added",
		update: func(pod *corev1.Pod) {
			pod.Annotations["k8s.ovn.org/pod-networks"] = `{"default":{"ip_addresses":["10.128.2.17/23"]}}`
		},
	},
	{
		name: "Multus network-status annotation added",
		update: func(pod *corev1.Pod) {
			pod.Annotations["k8s.v1.cni.cncf.io/network-status"] = `[{"name":"ovn-kubernetes","interface":"eth0"}]`
		},
	},
	{
		name: "job-tracking finalizer removed",
		update: func(pod *corev1.Pod) {
			pod.Finalizers = nil
		},
	},
	{
		name: "deletionTimestamp set",
		update: func(pod *corev1.Pod) {
			pod.DeletionTimestamp = new(metav1.Now())
		},
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
	},
	{
		name: "spec changed",
		update: func(pod *corev1.Pod) {
			pod.Spec.Containers[0].Image = "app:v2"
		},
	},
	{
		name: "app label changed",
		update: func(pod *corev1.Pod) {
			pod.Labels["app"] = "other"
		},
	},
	{
		name: "prometheus.io/scrape annotation added",
		update: func(pod *corev1.Pod) {
			pod.Annotations["prometheus.io/scrape"] = "true"
		},
	},
}

// TestPodReferencesRevision covers the scope filter used to restrict the other two to pods a mesh
// revision can be affected by.
func TestPodReferencesRevision(t *testing.T) {
	pred := AsPredicate(PodReferencesRevision())

	plain := &corev1.Pod{
		Name:      "plain",
		Namespace: "default",
		Labels:    map[string]string{"app": "plain"},
		Status:    corev1.PodStatus{Phase: corev1.PodRunning},
	}

	// istiod's own pods carry an istio.io/rev label but disable injection, so they reference no
	// revision in the sense that matters here
	istiod := &corev1.Pod{
		Name:      "istiod-my-rev-7d4b8c9f5-abcde",
		Namespace: "istio-system",
		Labels: map[string]string{
			"app":                     "istiod",
			"istio.io/rev":            "my-rev",
			"sidecar.istio.io/inject": "false",
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}

	// a pod named by annotation only, as the injector leaves it
	annotatedOnly := &corev1.Pod{
		Name:        "annotated",
		Namespace:   "bookinfo",
		Labels:      map[string]string{"app": "annotated"},
		Annotations: map[string]string{"istio.io/rev": "my-rev"},
		Status:      corev1.PodStatus{Phase: corev1.PodRunning},
	}

	tests := []struct {
		name     string
		oldPod   *corev1.Pod
		newPod   *corev1.Pod
		expected bool
	}{
		{
			name:     "pod references a revision by annotation and label",
			oldPod:   injectedPod(),
			newPod:   injectedPod(),
			expected: true,
		},
		{
			name:     "pod references a revision by annotation only",
			oldPod:   annotatedOnly,
			newPod:   annotatedOnly,
			expected: true,
		},
		{
			name:     "pod names no revision",
			oldPod:   plain,
			newPod:   plain,
			expected: false,
		},
		{
			name:     "istiod pod disables injection, so references no revision",
			oldPod:   istiod,
			newPod:   istiod,
			expected: false,
		},
		{
			name:     "pod gains injection metadata",
			oldPod:   plain,
			newPod:   injectedPod(),
			expected: true,
		},
		{
			name:     "pod loses injection metadata",
			oldPod:   injectedPod(),
			newPod:   plain,
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			result := pred.Update(event.UpdateEvent{ObjectOld: tc.oldPod, ObjectNew: tc.newPod})
			g.Expect(result).To(Equal(tc.expected), "unexpected result of predicate.Update()")
		})
	}
}

// TestPodRevisionChanges covers the filter on the revision a pod names, by its injected
// istio.io/rev annotation or by its injection labels. Phase is none of its concern.
func TestPodRevisionChanges(t *testing.T) {
	pred := AsPredicate(PodRevisionChanges())

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
			name:     "resourceVersion changed",
			update:   func(pod *corev1.Pod) {},
			expected: false,
		},

		// phase belongs to PodTerminationChanges, not here
		{
			name: "running to succeeded",
			update: func(pod *corev1.Pod) {
				pod.Status.Phase = corev1.PodSucceeded
			},
			expected: false,
		},
		{
			name: "running to failed",
			update: func(pod *corev1.Pod) {
				pod.Status.Phase = corev1.PodFailed
			},
			expected: false,
		},

		// the revision the pod names
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
			// the annotation and the labels are independent signals, not a fallback chain: a
			// pod can name one revision by annotation and another by label, and a caller may
			// act on either, so a label change must pass even when the annotation is untouched
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
		{
			name: "pod starts naming a revision",
			updateOld: func(pod *corev1.Pod) {
				pod.Annotations = map[string]string{}
				pod.Labels = map[string]string{"app": "workload"}
			},
			update: func(pod *corev1.Pod) {
				pod.Labels["istio.io/rev"] = "my-rev"
			},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			oldObj := injectedPod()
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

	for _, tc := range platformWrites {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			oldObj := injectedPod()
			newObj := oldObj.DeepCopy()
			newObj.ResourceVersion = "2"
			tc.update(newObj)

			result := pred.Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj})
			g.Expect(result).To(BeFalse(), "platform metadata writes must not be admitted")
		})
	}
}

// TestPodTerminationChanges covers the filter on whether a pod is still running. Which
// revision the pod names is none of its concern.
func TestPodTerminationChanges(t *testing.T) {
	pred := AsPredicate(PodTerminationChanges())

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
			name: "container statuses changed but phase unchanged",
			update: func(pod *corev1.Pod) {
				pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "app", RestartCount: 3}}
			},
			expected: false,
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
			// the filter is about whether the pod runs, not about which revision it names, so
			// it admits this for any pod. Scoping that to pods which name a revision is the
			// caller's business.
			name: "running to succeeded on a pod naming no revision",
			updateOld: func(pod *corev1.Pod) {
				pod.Annotations = map[string]string{}
				pod.Labels = map[string]string{"app": "workload"}
			},
			update: func(pod *corev1.Pod) {
				pod.Status.Phase = corev1.PodSucceeded
			},
			expected: true,
		},
		{
			name: "istio.io/rev label changed",
			update: func(pod *corev1.Pod) {
				pod.Labels["istio.io/rev"] = "other-rev"
			},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			oldObj := injectedPod()
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

	for _, tc := range platformWrites {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			oldObj := injectedPod()
			newObj := oldObj.DeepCopy()
			newObj.ResourceVersion = "2"
			tc.update(newObj)

			result := pred.Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj})
			g.Expect(result).To(BeFalse(), "platform metadata writes must not be admitted")
		})
	}
}

// TestPodFiltersPassCreateAndDelete asserts none of the filters drops a create or delete
// event: a pod appearing or disappearing always has to reach the caller.
func TestPodFiltersPassCreateAndDelete(t *testing.T) {
	pod := &corev1.Pod{Name: "workload", Namespace: "bookinfo"}

	for name, pred := range map[string]predicate.Funcs{
		"PodReferencesRevision": AsPredicate(PodReferencesRevision()),
		"PodRevisionChanges":    AsPredicate(PodRevisionChanges()),
		"PodTerminationChanges": AsPredicate(PodTerminationChanges()),
	} {
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(pred.Create(event.CreateEvent{Object: pod})).To(BeTrue(), "create events must not be filtered")
			g.Expect(pred.Delete(event.DeleteEvent{Object: pod})).To(BeTrue(), "delete events must not be filtered")
			g.Expect(pred.Generic(event.GenericEvent{Object: pod})).To(BeTrue(), "generic events must not be filtered")
		})
	}
}

// TestPodFiltersOnNonPod asserts the filters fail open if handed something that is not a
// typed Pod, rather than silently dropping events.
func TestPodFiltersOnNonPod(t *testing.T) {
	oldObj := &corev1.ConfigMap{Name: "cm", ResourceVersion: "1"}
	newObj := &corev1.ConfigMap{Name: "cm", ResourceVersion: "2"}

	for name, shouldReconcile := range map[string]ShouldReconcileFunc{
		"PodReferencesRevision": PodReferencesRevision(),
		"PodRevisionChanges":    PodRevisionChanges(),
		"PodTerminationChanges": PodTerminationChanges(),
	} {
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(shouldReconcile(oldObj, newObj)).To(BeTrue())
		})
	}
}
