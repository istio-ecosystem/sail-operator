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

package reconciler

import (
	"context"
	"reflect"
	"strings"

	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// ReconcileFunc is a function that reconciles an object.
type ReconcileFunc[T client.Object] func(ctx context.Context, obj T) (ctrl.Result, error)

// FinalizeFunc is a function that finalizes an object. It does not remove the finalizer.
type FinalizeFunc[T client.Object] func(ctx context.Context, obj T) error

// StandardReconciler encapsulates common reconciler behavior, allowing you to
// implement a reconciler simply by providing a ReconcileFunc and an optional
// FinalizeFunc. These functions are invoked at the appropriate time and are
// passed the object being reconciled.
type StandardReconciler[T client.Object] struct {
	client    client.Client
	reconcile ReconcileFunc[T]
	finalizer string
	finalize  FinalizeFunc[T]
}

// NewStandardReconciler creates a new StandardReconciler for objects of the specified type.
func NewStandardReconciler[T client.Object](cl client.Client, reconcileFunc ReconcileFunc[T]) *StandardReconciler[T] {
	return NewStandardReconcilerWithFinalizer[T](cl, reconcileFunc, nil, "")
}

// NewStandardReconcilerWithFinalizer is similar to NewStandardReconciler, but also accepts a finalizer and a
// FinalizerFunc.
func NewStandardReconcilerWithFinalizer[T client.Object](
	cl client.Client, reconcileFunc ReconcileFunc[T], finalizeFunc FinalizeFunc[T], finalizer string,
) *StandardReconciler[T] {
	return &StandardReconciler[T]{
		client:    cl,
		reconcile: reconcileFunc,
		finalizer: finalizer,
		finalize:  finalizeFunc,
	}
}

// Reconcile reconciles the object. It first fetches the object from the client, then invokes the
// configured ReconcileFunc. If a finalizer is configured in the reconciler, and the object is new,
// this function adds the finalizer to the object. When the object is being deleted, this function
// invokes the configured FinalizerFunc and removes the finalizer afterward.
func (r *StandardReconciler[T]) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	obj := reflect.New(reflect.TypeOf(*new(T)).Elem()).Interface().(T)
	if err := r.client.Get(ctx, req.NamespacedName, obj); err != nil {
		if errors.IsNotFound(err) {
			log.V(2).Info("Resource not found. Skipping reconciliation")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !obj.GetDeletionTimestamp().IsZero() {
		if r.finalizationEnabled() && kube.HasFinalizer(obj, r.finalizer) {
			if err := r.finalize(ctx, obj); err != nil {
				return ctrl.Result{}, err
			}
			return kube.RemoveFinalizer(ctx, r.client, obj, r.finalizer)
		}
		return ctrl.Result{}, nil
	}

	if r.finalizationEnabled() && !kube.HasFinalizer(obj, r.finalizer) {
		return kube.AddFinalizer(ctx, r.client, obj, r.finalizer)
	}

	result, err := r.reconcile(ctx, obj)
	switch {
	case errors.IsForbidden(err) && strings.Contains(err.Error(), "RESTMapping"):
		log.Info("APIServer seems to be not ready - RESTMapper of gc admission plugin is not up to date. Retrying...", "error", err)
		return ctrl.Result{Requeue: true}, nil
	case errors.IsConflict(err):
		log.Info("Conflict detected. Retrying...")
		return ctrl.Result{Requeue: true}, nil
	case errors.IsNotFound(err):
		log.Info("Resource not found. Retrying...", "error", err)
		return ctrl.Result{Requeue: true}, nil
	case IsTransientError(err):
		log.Info("Reconciliation failed. Retrying...", "error", err)
		return ctrl.Result{Requeue: true}, nil
	case IsValidationError(err):
		log.Info("Validation failed", "error", err)
		return ctrl.Result{}, nil
	default:
		return result, err
	}
}

func (r *StandardReconciler[T]) finalizationEnabled() bool {
	return r.finalizer != ""
}
