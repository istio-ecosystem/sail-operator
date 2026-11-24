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

// Package podwatch provides the pod events a Pod watch has to make a decision about, so that
// every controller watching pods is held to the same set of them.
//
// It deliberately carries no expectations. What a given watch should do with an event depends
// on which signals its controller reads - the IstioRevision controller reads a pod's injected
// istio.io/rev annotation as well as its injection labels, while the IstioRevisionTag
// controller reads the labels only - so the expectations belong in each controller's own test,
// where they serve as that watch's specification. Only the setup is shared.
package podwatch

import (
	"sort"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	istioRevLabel      = "istio.io/rev"
	sidecarInjectLabel = "sidecar.istio.io/inject"
	revision           = "my-rev"
)

// Scenario is a pod update: a pod to start from, and the change applied to it.
type Scenario struct {
	Pod    func() *corev1.Pod
	Update func(pod *corev1.Pod)
}

// InjectedPod is a running workload pod that references a revision both by injected annotation
// by label, and carries the kind of finalizer a Job pod has.
func InjectedPod() *corev1.Pod {
	return &corev1.Pod{
		Name:       "workload",
		Namespace:  "bookinfo",
		Finalizers: []string{"batch.kubernetes.io/job-tracking"},
		Labels: map[string]string{
			"app":         "workload",
			istioRevLabel: revision,
		},
		Annotations: map[string]string{istioRevLabel: revision},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "app", Image: "app:v1"}},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
}

// AnnotatedPod references a revision by injected annotation only, which is how the sidecar
// injector leaves a pod whose namespace opted it in.
func AnnotatedPod() *corev1.Pod {
	pod := InjectedPod()
	delete(pod.Labels, istioRevLabel)
	return pod
}

// LabeledPod references a revision by injection label only, with no injected annotation: a pod the
// injector never saw, or an ambient workload.
func LabeledPod() *corev1.Pod {
	pod := InjectedPod()
	pod.Annotations = map[string]string{}
	return pod
}

// PlainPod carries no injection metadata at all. Most pods on a cluster look like this.
func PlainPod() *corev1.Pod {
	return &corev1.Pod{
		Name:      "ci-job",
		Namespace: "ci",
		Labels:    map[string]string{"app": "ci-job"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "job", Image: "job:v1"}},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
}

// IstiodPod is a control plane pod. It carries an istio.io/rev label like a mesh workload,
// but disables injection, so it references no revision. Any pod can be marked Failed when its
// node is lost or shut down.
func IstiodPod() *corev1.Pod {
	return &corev1.Pod{
		Name:      "istiod-my-rev-7d4b8c9f5-abcde",
		Namespace: "istio-system",
		Labels: map[string]string{
			"app":              "istiod",
			istioRevLabel:      revision,
			sidecarInjectLabel: "false",
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
}

// Scenarios returns every pod event a Pod watch has to make a decision about, keyed by name.
//
// The first group is the metadata the platform writes over a normal pod lifecycle, which is
// what makes an unfiltered Pod watch expensive. The rest are the events that may genuinely
// matter: phase transitions, edits to injection metadata, and pods outside the mesh.
func Scenarios() map[string]Scenario {
	return map[string]Scenario{
		// platform-driven metadata writes
		"the pod is bound to a node": {
			Pod:    InjectedPod,
			Update: func(pod *corev1.Pod) { pod.Spec.NodeName = "node-1" },
		},
		"CNI writes the pod-network annotation": {
			Pod: InjectedPod,
			Update: func(pod *corev1.Pod) {
				pod.Annotations["k8s.ovn.org/pod-networks"] = `{"default":{"ip_addresses":["10.128.2.17/23"]}}`
			},
		},
		"Multus writes the network-status annotation": {
			Pod: InjectedPod,
			Update: func(pod *corev1.Pod) {
				pod.Annotations["k8s.v1.cni.cncf.io/network-status"] = `[{"name":"ovn-kubernetes","interface":"eth0"}]`
			},
		},
		"job controller removes its tracking finalizer": {
			Pod:    InjectedPod,
			Update: func(pod *corev1.Pod) { pod.Finalizers = nil },
		},
		"pod is marked for deletion": {
			Pod:    InjectedPod,
			Update: func(pod *corev1.Pod) { pod.DeletionTimestamp = new(metav1.Now()) },
		},
		"pod is adopted by a ReplicaSet": {
			Pod: InjectedPod,
			Update: func(pod *corev1.Pod) {
				pod.OwnerReferences = []metav1.OwnerReference{{
					APIVersion: "apps/v1",
					Kind:       "ReplicaSet",
					Name:       "workload-123",
				}}
			},
		},
		"the app label changes": {
			Pod:    InjectedPod,
			Update: func(pod *corev1.Pod) { pod.Labels["app"] = "other" },
		},
		"the prometheus.io/scrape annotation is added": {
			Pod:    InjectedPod,
			Update: func(pod *corev1.Pod) { pod.Annotations["prometheus.io/scrape"] = "true" },
		},
		"a container restarts": {
			Pod: InjectedPod,
			Update: func(pod *corev1.Pod) {
				pod.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "app", RestartCount: 3}}
			},
		},

		// phase transitions
		"pod starts running": {
			Pod: func() *corev1.Pod {
				pod := InjectedPod()
				pod.Status.Phase = corev1.PodPending
				return pod
			},
			Update: func(pod *corev1.Pod) { pod.Status.Phase = corev1.PodRunning },
		},
		"injected pod completes": {
			Pod:    InjectedPod,
			Update: func(pod *corev1.Pod) { pod.Status.Phase = corev1.PodSucceeded },
		},
		"injected pod fails": {
			Pod:    InjectedPod,
			Update: func(pod *corev1.Pod) { pod.Status.Phase = corev1.PodFailed },
		},
		"completed pod is recorded as failed (terminal state to terminal state)": {
			Pod: func() *corev1.Pod {
				pod := InjectedPod()
				pod.Status.Phase = corev1.PodSucceeded
				return pod
			},
			Update: func(pod *corev1.Pod) { pod.Status.Phase = corev1.PodFailed },
		},
		"pod with injection label only completes": {
			Pod:    LabeledPod,
			Update: func(pod *corev1.Pod) { pod.Status.Phase = corev1.PodSucceeded },
		},
		"pod with injected annotation only completes": {
			Pod:    AnnotatedPod,
			Update: func(pod *corev1.Pod) { pod.Status.Phase = corev1.PodSucceeded },
		},

		// edits to injection metadata
		"pod is moved to another revision": {
			Pod:    InjectedPod,
			Update: func(pod *corev1.Pod) { pod.Labels[istioRevLabel] = "other-rev" },
		},
		"pod loses its injection label": {
			Pod:    InjectedPod,
			Update: func(pod *corev1.Pod) { delete(pod.Labels, istioRevLabel) },
		},
		"pod loses its injected annotation": {
			Pod:    InjectedPod,
			Update: func(pod *corev1.Pod) { delete(pod.Annotations, istioRevLabel) },
		},
		"pod's injected annotation changes": {
			Pod:    InjectedPod,
			Update: func(pod *corev1.Pod) { pod.Annotations[istioRevLabel] = "other-rev" },
		},
		"pod opts out of injection": {
			Pod:    InjectedPod,
			Update: func(pod *corev1.Pod) { pod.Labels[sidecarInjectLabel] = "false" },
		},
		"pod joins a revision": {
			Pod:    PlainPod,
			Update: func(pod *corev1.Pod) { pod.Labels[istioRevLabel] = revision },
		},

		// pods outside the mesh
		"pod with no injection metadata completes": {
			Pod:    PlainPod,
			Update: func(pod *corev1.Pod) { pod.Status.Phase = corev1.PodSucceeded },
		},
		"pod with no injection metadata is scheduled": {
			Pod:    PlainPod,
			Update: func(pod *corev1.Pod) { pod.Labels["kubernetes.io/hostname"] = "node-1" },
		},

		// control plane
		"istiod pod fails": {
			Pod:    IstiodPod,
			Update: func(pod *corev1.Pod) { pod.Status.Phase = corev1.PodFailed },
		},
	}
}

// Run asserts pred against every scenario, using the outcome expected declares for it.
//
// It fails if expected omits a scenario or names one that does not exist, so a scenario added
// here forces every watch using it to state what it does about that event, rather than
// silently inheriting someone else's answer.
func Run(t *testing.T, pred predicate.Predicate, expected map[string]bool) {
	t.Helper()
	scenarios := Scenarios()

	for name := range expected {
		if _, ok := scenarios[name]; !ok {
			t.Errorf("expectation declared for unknown scenario %q", name)
		}
	}

	names := make([]string, 0, len(scenarios))
	for name := range scenarios {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		want, declared := expected[name]
		if !declared {
			t.Errorf("no expectation declared for scenario %q", name)
			continue
		}

		t.Run(name, func(t *testing.T) {
			scenario := scenarios[name]

			oldPod := scenario.Pod()
			oldPod.ResourceVersion = "1"
			newPod := oldPod.DeepCopy()
			newPod.ResourceVersion = "2"
			scenario.Update(newPod)

			if got := pred.Update(event.UpdateEvent{ObjectOld: oldPod, ObjectNew: newPod}); got != want {
				t.Errorf("expected the watch to be woken: %t, but it was: %t", want, got)
			}
		})
	}
}

// AssertCreateAndDeletePass checks that pred never filters out a pod appearing or
// disappearing, which always has to reach the controller.
func AssertCreateAndDeletePass(t *testing.T, pred predicate.Predicate) {
	t.Helper()
	pod := InjectedPod()

	if !pred.Create(event.CreateEvent{Object: pod}) {
		t.Error("create events must not be filtered out")
	}
	if !pred.Delete(event.DeleteEvent{Object: pod}) {
		t.Error("delete events must not be filtered out")
	}
}
