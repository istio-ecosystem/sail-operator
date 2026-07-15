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
	"reflect"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// ShouldReconcileFunc determines whether an update event should trigger reconciliation.
// Works with both typed objects (controller-runtime) and unstructured objects (dynamic informers).
type ShouldReconcileFunc func(oldObj, newObj client.Object) bool

// WatchedResource describes a Kubernetes resource type produced by a Helm chart.
type WatchedResource struct {
	Object          client.Object
	ShouldReconcile ShouldReconcileFunc
	Skipped         bool
}

// IgnoreAllUpdates returns a filter that suppresses all update events.
func IgnoreAllUpdates() ShouldReconcileFunc {
	return func(_, _ client.Object) bool {
		return false
	}
}

// IgnoreStatusChanges returns a filter that ignores updates where only the resource status changed.
func IgnoreStatusChanges() ShouldReconcileFunc {
	return func(oldObj, newObj client.Object) bool {
		return specWasUpdated(oldObj, newObj) ||
			!reflect.DeepEqual(newObj.GetLabels(), oldObj.GetLabels()) ||
			!reflect.DeepEqual(newObj.GetAnnotations(), oldObj.GetAnnotations()) ||
			!reflect.DeepEqual(newObj.GetOwnerReferences(), oldObj.GetOwnerReferences()) ||
			!reflect.DeepEqual(newObj.GetFinalizers(), oldObj.GetFinalizers())
	}
}

// WebhookFilter returns a filter that ignores webhook config updates caused by istiod (caBundle, failurePolicy).
func WebhookFilter() ShouldReconcileFunc {
	return func(oldObj, newObj client.Object) bool {
		if oldObj == nil || newObj == nil {
			return false
		}
		oldCopy := oldObj.DeepCopyObject().(client.Object)
		newCopy := newObj.DeepCopyObject().(client.Object)
		clearWebhookFields(oldCopy)
		clearWebhookFields(newCopy)
		return !reflect.DeepEqual(newCopy, oldCopy)
	}
}

// AsPredicate wraps a ShouldReconcileFunc as a controller-runtime predicate.
func AsPredicate(fn ShouldReconcileFunc) predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return fn(e.ObjectOld, e.ObjectNew)
		},
	}
}

// RegisterOwnedWatches registers watches for all non-skipped resources in the watch list.
// handlerOverrides is keyed by reflect.Type (e.g. reflect.TypeOf(&discoveryv1.EndpointSlice{})).
func RegisterOwnedWatches(
	b *builder.Builder,
	watchList []WatchedResource,
	defaultHandler handler.EventHandler,
	handlerOverrides map[reflect.Type]handler.EventHandler,
	defaultPredicates ...predicate.Predicate,
) *builder.Builder {
	for _, wr := range watchList {
		if wr.Skipped {
			continue
		}

		h := defaultHandler
		if override, ok := handlerOverrides[reflect.TypeOf(wr.Object)]; ok {
			h = override
		}

		var preds []predicate.Predicate
		if wr.ShouldReconcile != nil {
			preds = append(preds, AsPredicate(wr.ShouldReconcile))
		}
		preds = append(preds, defaultPredicates...)

		if len(preds) > 0 {
			b = b.Watches(wr.Object, h, builder.WithPredicates(preds...))
		} else {
			b = b.Watches(wr.Object, h)
		}
	}
	return b
}

func specWasUpdated(oldObj, newObj client.Object) bool {
	if oldHpa, ok := oldObj.(*autoscalingv2.HorizontalPodAutoscaler); ok {
		if newHpa, ok := newObj.(*autoscalingv2.HorizontalPodAutoscaler); ok {
			return !reflect.DeepEqual(oldHpa.Spec, newHpa.Spec)
		}
	}
	return oldObj.GetGeneration() != newObj.GetGeneration()
}

func clearWebhookFields(obj client.Object) {
	obj.SetResourceVersion("")
	obj.SetGeneration(0)
	obj.SetManagedFields(nil)
	switch webhookConfig := obj.(type) {
	case *admissionv1.ValidatingWebhookConfiguration:
		for i := range len(webhookConfig.Webhooks) {
			webhookConfig.Webhooks[i].FailurePolicy = nil
			webhookConfig.Webhooks[i].ClientConfig.CABundle = nil
		}
	case *admissionv1.MutatingWebhookConfiguration:
		for i := range len(webhookConfig.Webhooks) {
			webhookConfig.Webhooks[i].ClientConfig.CABundle = nil
		}
	}
}
