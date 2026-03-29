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

// Package watches defines the static lists of resource types produced by each
// Istio Helm chart and the update-filtering functions for each resource.
// The lists are validated against chart templates by hack/lint-watches.sh.
package watches

import (
	"reflect"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const ignoreAnnotation = "sailoperator.io/ignore"

// ShouldReconcileFunc returns true if an update should trigger reconciliation.
type ShouldReconcileFunc func(oldObj, newObj client.Object) bool

// WatchedResource pairs a typed object prototype with its update filter.
type WatchedResource struct {
	// Object is a typed prototype (e.g. &corev1.ConfigMap{}) used to set up a typed informer.
	Object client.Object
	// ShouldReconcile filters update events. Nil means all updates trigger reconciliation.
	ShouldReconcile ShouldReconcileFunc
	// Skipped means the chart produces this kind but it's intentionally not watched.
	Skipped bool
}

// IgnoreAnnotation returns a predicate that skips update events when the new
// object has the sailoperator.io/ignore annotation set to "true".
func IgnoreAnnotation() predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectNew == nil {
				return false
			}
			return e.ObjectNew.GetAnnotations()[ignoreAnnotation] != "true"
		},
	}
}

// IgnoreAllUpdates skips all update events.
func IgnoreAllUpdates() ShouldReconcileFunc {
	return func(_, _ client.Object) bool { return false }
}

// IgnoreStatusChanges skips updates where only the status changed.
func IgnoreStatusChanges() ShouldReconcileFunc {
	return func(oldObj, newObj client.Object) bool {
		return specWasUpdated(oldObj, newObj) ||
			!reflect.DeepEqual(newObj.GetLabels(), oldObj.GetLabels()) ||
			!reflect.DeepEqual(newObj.GetAnnotations(), oldObj.GetAnnotations()) ||
			!reflect.DeepEqual(newObj.GetOwnerReferences(), oldObj.GetOwnerReferences()) ||
			!reflect.DeepEqual(newObj.GetFinalizers(), oldObj.GetFinalizers())
	}
}

// WebhookFilter ignores changes to fields managed by istiod (caBundle, failurePolicy).
func WebhookFilter() ShouldReconcileFunc {
	return func(oldObj, newObj client.Object) bool {
		oldCopy := oldObj.DeepCopyObject().(client.Object)
		newCopy := newObj.DeepCopyObject().(client.Object)
		clearWebhookFields(oldCopy)
		clearWebhookFields(newCopy)
		return !reflect.DeepEqual(newCopy, oldCopy)
	}
}

// specWasUpdated checks if the spec changed. Handles the HPA special case
// where k8s doesn't increment metadata.generation.
func specWasUpdated(oldObj, newObj client.Object) bool {
	if oldHpa, ok := oldObj.(*autoscalingv2.HorizontalPodAutoscaler); ok {
		if newHpa, ok := newObj.(*autoscalingv2.HorizontalPodAutoscaler); ok {
			return !reflect.DeepEqual(oldHpa.Spec, newHpa.Spec)
		}
	}
	return oldObj.GetGeneration() != newObj.GetGeneration()
}

// RegisterOwnedWatches registers watches for all non-skipped resources.
// The ignore annotation predicate is always applied.
func RegisterOwnedWatches(
	b *builder.Builder,
	watchList []WatchedResource,
	defaultHandler handler.EventHandler,
	handlerOverrides map[string]handler.EventHandler,
) {
	for _, wr := range watchList {
		if wr.Skipped {
			continue
		}

		h := defaultHandler
		if handlerOverrides != nil {
			kind := reflect.TypeOf(wr.Object).Elem().Name()
			if override, ok := handlerOverrides[kind]; ok {
				h = override
			}
		}

		predicates := []predicate.Predicate{IgnoreAnnotation()}
		if wr.ShouldReconcile != nil {
			predicates = append(predicates, AsPredicate(wr.ShouldReconcile))
		}
		b.Watches(wr.Object, h, builder.WithPredicates(predicates...))
	}
}

// AsPredicate wraps a ShouldReconcileFunc as a controller-runtime predicate.
func AsPredicate(fn ShouldReconcileFunc) predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectOld == nil || e.ObjectNew == nil {
				return false
			}
			return fn(e.ObjectOld, e.ObjectNew)
		},
	}
}

// clearWebhookFields clears fields managed by istiod on webhook configurations.
func clearWebhookFields(obj client.Object) {
	obj.SetResourceVersion("")
	obj.SetGeneration(0)
	obj.SetManagedFields(nil)

	switch wc := obj.(type) {
	case *admissionv1.ValidatingWebhookConfiguration:
		for i := range wc.Webhooks {
			wc.Webhooks[i].FailurePolicy = nil
			wc.Webhooks[i].ClientConfig.CABundle = nil
		}
	case *admissionv1.MutatingWebhookConfiguration:
		for i := range wc.Webhooks {
			wc.Webhooks[i].ClientConfig.CABundle = nil
		}
	}
}
