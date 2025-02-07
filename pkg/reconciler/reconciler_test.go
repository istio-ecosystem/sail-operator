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
	"errors"
	"fmt"
	"testing"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	"github.com/istio-ecosystem/sail-operator/pkg/test/testtime"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const testFinalizer = "test-finalizer"

type mockReconciler struct {
	reconcileInvoked bool
	finalizeInvoked  bool
	reconcileError   error
	finalizeError    error
}

func (t *mockReconciler) Object() client.Object {
	return &v1.Istio{}
}

func (t *mockReconciler) Reconcile(ctx context.Context, _ *v1.Istio) (ctrl.Result, error) {
	t.reconcileInvoked = true
	return ctrl.Result{}, t.reconcileError
}

func (t *mockReconciler) Finalize(ctx context.Context, _ *v1.Istio) error {
	t.finalizeInvoked = true
	return t.finalizeError
}

var ctx = context.TODO()

func TestReconcile(t *testing.T) {
	key := types.NamespacedName{
		Name: "my-resource",
	}

	type testCase struct {
		name             string
		objects          []client.Object
		interceptorFuncs interceptor.Funcs
		setup            func(g *WithT, mock *mockReconciler)
		assert           func(g *WithT, cl client.Client, result ctrl.Result, err error, mock *mockReconciler)
	}
	tests := []testCase{
		{
			name: "skips reconciliation when resource not found",
			assert: func(g *WithT, cl client.Client, result ctrl.Result, err error, mock *mockReconciler) {
				g.Expect(result).To(BeZero())
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(mock.reconcileInvoked).To(BeFalse())
				g.Expect(mock.finalizeInvoked).To(BeFalse())
			},
		},
		{
			name: "returns error when it fails to get resource",
			interceptorFuncs: interceptor.Funcs{
				Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
					return fmt.Errorf("internal error")
				},
			},
			assert: func(g *WithT, cl client.Client, result ctrl.Result, err error, mock *mockReconciler) {
				g.Expect(result).To(BeZero())
				g.Expect(err).To(HaveOccurred())
				g.Expect(mock.reconcileInvoked).To(BeFalse())
				g.Expect(mock.finalizeInvoked).To(BeFalse())
			},
		},
		{
			name: "skips reconciliation when resource deleted",
			objects: []client.Object{
				&v1.Istio{
					ObjectMeta: metav1.ObjectMeta{
						Name:              key.Name,
						DeletionTimestamp: testtime.OneMinuteAgo(),
						Finalizers:        []string{"dummy"}, // the fake client doesn't allow you to add a deleted object unless it has a finalizer
					},
				},
			},
			assert: func(g *WithT, cl client.Client, result ctrl.Result, err error, mock *mockReconciler) {
				g.Expect(result).To(BeZero())
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(mock.reconcileInvoked).To(BeFalse(), "reconcile should not be invoked when object is being deleted")
				g.Expect(mock.finalizeInvoked).To(BeFalse(), "finalize should not be invoked when object doesn't have the finalizer in question")
			},
		},
		{
			name: "finalizes resource when resource deleted",
			objects: []client.Object{
				&v1.Istio{
					ObjectMeta: metav1.ObjectMeta{
						Name:              key.Name,
						DeletionTimestamp: testtime.OneMinuteAgo(),
						Finalizers:        []string{testFinalizer},
					},
				},
			},
			assert: func(g *WithT, cl client.Client, result ctrl.Result, err error, mock *mockReconciler) {
				g.Expect(result).To(BeZero())
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(mock.reconcileInvoked).To(BeFalse(), "reconcile should not be invoked when object is being deleted")
				g.Expect(mock.finalizeInvoked).To(BeTrue(), "finalize should be invoked when object is being deleted and still has the finalizer")
			},
		},
		{
			name: "preserves finalizer and returns error when finalization fails",
			objects: []client.Object{
				&v1.Istio{
					ObjectMeta: metav1.ObjectMeta{
						Name:              key.Name,
						DeletionTimestamp: testtime.OneMinuteAgo(),
						Finalizers:        []string{testFinalizer},
					},
				},
			},
			setup: func(g *WithT, mock *mockReconciler) {
				mock.finalizeError = errors.New("simulated error")
			},
			assert: func(g *WithT, cl client.Client, result ctrl.Result, err error, mock *mockReconciler) {
				g.Expect(result).To(BeZero())
				g.Expect(err).To(HaveOccurred())
				g.Expect(mock.reconcileInvoked).To(BeFalse(), "reconcile should not be invoked when object is being deleted")
				g.Expect(mock.finalizeInvoked).To(BeTrue(), "finalize should be invoked when object is being deleted and still has the finalizer")

				obj := &v1.Istio{}
				g.Expect(cl.Get(ctx, key, obj)).To(Succeed())
				g.Expect(obj.GetFinalizers()).To(ContainElement(testFinalizer))
			},
		},
		{
			name: "adds finalizer when resource doesn't have it",
			objects: []client.Object{
				&v1.Istio{
					ObjectMeta: metav1.ObjectMeta{
						Name: key.Name,
					},
				},
			},
			assert: func(g *WithT, cl client.Client, result ctrl.Result, err error, mock *mockReconciler) {
				g.Expect(result).To(BeZero())
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(mock.reconcileInvoked).To(BeFalse())
				g.Expect(mock.finalizeInvoked).To(BeFalse())

				obj := &v1.Istio{}
				g.Expect(cl.Get(ctx, key, obj)).To(Succeed())
				g.Expect(obj.GetFinalizers()).To(ContainElement(testFinalizer))
			},
		},
		{
			name: "invokes reconcile if everything is fine",
			objects: []client.Object{
				&v1.Istio{
					ObjectMeta: metav1.ObjectMeta{
						Name:       key.Name,
						Finalizers: []string{testFinalizer},
					},
				},
			},
			assert: func(g *WithT, cl client.Client, result ctrl.Result, err error, mock *mockReconciler) {
				g.Expect(result).To(BeZero())
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(mock.reconcileInvoked).To(BeTrue())
				g.Expect(mock.finalizeInvoked).To(BeFalse())
			},
		},
		{
			name: "returns error when reconcile fails",
			objects: []client.Object{
				&v1.Istio{
					ObjectMeta: metav1.ObjectMeta{
						Name:       key.Name,
						Finalizers: []string{testFinalizer},
					},
				},
			},
			setup: func(g *WithT, mock *mockReconciler) {
				mock.reconcileError = errors.New("simulated error")
			},
			assert: func(g *WithT, cl client.Client, result ctrl.Result, err error, mock *mockReconciler) {
				g.Expect(result).To(BeZero())
				g.Expect(err).To(HaveOccurred())
				g.Expect(mock.reconcileInvoked).To(BeTrue())
				g.Expect(mock.finalizeInvoked).To(BeFalse())
			},
		},
		{
			name: "requeues on conflict",
			objects: []client.Object{
				&v1.Istio{
					ObjectMeta: metav1.ObjectMeta{
						Name:       key.Name,
						Finalizers: []string{testFinalizer},
					},
				},
			},
			setup: func(g *WithT, mock *mockReconciler) {
				mock.reconcileError = apierrors.NewConflict(schema.GroupResource{}, "foo", fmt.Errorf("simulated conflict"))
			},
			assert: func(g *WithT, cl client.Client, result ctrl.Result, err error, mock *mockReconciler) {
				g.Expect(result).To(Equal(reconcile.Result{Requeue: true}))
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(mock.reconcileInvoked).To(BeTrue())
				g.Expect(mock.finalizeInvoked).To(BeFalse())
			},
		},
		{
			name: "handles ValidationErrors",
			objects: []client.Object{
				&v1.Istio{
					ObjectMeta: metav1.ObjectMeta{
						Name:       key.Name,
						Finalizers: []string{testFinalizer},
					},
				},
			},
			setup: func(g *WithT, mock *mockReconciler) {
				mock.reconcileError = NewValidationError("simulated validation error")
			},
			assert: func(g *WithT, cl client.Client, result ctrl.Result, err error, mock *mockReconciler) {
				g.Expect(result).To(Equal(reconcile.Result{}))
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(mock.reconcileInvoked).To(BeTrue())
				g.Expect(mock.finalizeInvoked).To(BeFalse())
			},
		},
		{
			name: "requeues when gc admission plugin does not yet know about our resources",
			objects: []client.Object{
				&v1.Istio{
					ObjectMeta: metav1.ObjectMeta{
						Name:       key.Name,
						Finalizers: []string{testFinalizer},
					},
				},
			},
			setup: func(g *WithT, mock *mockReconciler) {
				mock.reconcileError = apierrors.NewForbidden(schema.GroupResource{}, "foo",
					fmt.Errorf("cannot set blockOwnerDeletion in this case because cannot find RESTMapping for APIVersion xyz"))
			},
			assert: func(g *WithT, cl client.Client, result ctrl.Result, err error, mock *mockReconciler) {
				g.Expect(result).To(Equal(reconcile.Result{Requeue: true}))
				g.Expect(err).ToNot(HaveOccurred())
				g.Expect(mock.reconcileInvoked).To(BeTrue())
				g.Expect(mock.finalizeInvoked).To(BeFalse())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			cl := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithStatusSubresource(&v1.Istio{}).
				WithObjects(tt.objects...).
				WithInterceptorFuncs(tt.interceptorFuncs).
				Build()

			mock := &mockReconciler{}
			if tt.setup != nil {
				tt.setup(g, mock)
			}

			reconciler := NewStandardReconcilerWithFinalizer[*v1.Istio](cl, mock.Reconcile, mock.Finalize, testFinalizer)
			result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: key})

			tt.assert(g, cl, result, err, mock)
		})
	}
}
