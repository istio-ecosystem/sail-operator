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

package enqueuelogger

import (
	"context"
	"reflect"

	"github.com/go-logr/logr"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var LogEnqueueEvents = false

// EnqueueEventLogger is a handler.EventHandler that wraps another handler.EventHandler and logs enqueued items (i.e.
// if the wrapped handler enqueues items from the event that is being handled, the EnqueueEventLogger logs them).
// The main purpose of this wrapper is to help with debugging which watch events are causing an object to be enqueued
// for reconciliation.
type EnqueueEventLogger struct {
	kind     string
	logger   logr.Logger
	delegate handler.EventHandler
}

var _ handler.EventHandler = &EnqueueEventLogger{}

func (h *EnqueueEventLogger) Create(ctx context.Context, e event.TypedCreateEvent[client.Object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	h.delegate.Create(ctx, e, h.wrapQueue(q, "Create", e.Object))
}

func (h *EnqueueEventLogger) Update(ctx context.Context, e event.TypedUpdateEvent[client.Object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	h.delegate.Update(ctx, e, h.wrapQueue(q, "Update", e.ObjectNew))
}

func (h *EnqueueEventLogger) Delete(ctx context.Context, e event.TypedDeleteEvent[client.Object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	h.delegate.Delete(ctx, e, h.wrapQueue(q, "Delete", e.Object))
}

func (h *EnqueueEventLogger) Generic(ctx context.Context, e event.TypedGenericEvent[client.Object], q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	h.delegate.Generic(ctx, e, h.wrapQueue(q, "Generic", e.Object))
}

func (h *EnqueueEventLogger) wrapQueue(
	q workqueue.TypedRateLimitingInterface[reconcile.Request], eventType string, obj client.Object,
) workqueue.TypedRateLimitingInterface[reconcile.Request] {
	return &AdditionNotifierQueue{
		TypedRateLimitingInterface: q,
		onAdd: func(request reconcile.Request) {
			requestSummary := ObjectSummary{
				Kind:      h.kind,
				Namespace: request.Namespace,
				Name:      request.Name,
			}

			eventSummary := EventSummary{
				Type: eventType,
				Object: ObjectSummary{
					Kind:      determineKind(obj),
					Name:      obj.GetName(),
					Namespace: obj.GetNamespace(),
				},
			}

			h.logger.Info("Object queued for reconciliation due to event", "object", requestSummary, "event", eventSummary)
		},
	}
}

func determineKind(obj client.Object) string {
	if obj == nil {
		return ""
	}
	kind := obj.GetObjectKind().GroupVersionKind().Kind
	if kind == "" {
		kind = reflect.TypeOf(obj).Elem().Name()
	}
	return kind
}

func WrapIfNecessary(kind string, logger logr.Logger, handler handler.EventHandler) handler.EventHandler {
	if LogEnqueueEvents {
		return &EnqueueEventLogger{
			kind:     kind,
			logger:   logger,
			delegate: handler,
		}
	}
	return handler
}

type EventSummary struct {
	Type   string        `json:"type"`
	Object ObjectSummary `json:"object"`
}

type ObjectSummary struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
}
