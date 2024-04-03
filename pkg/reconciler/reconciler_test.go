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
	"fmt"
	"testing"
	"time"

	"github.com/istio-ecosystem/sail-operator/api/v1alpha1"
	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

const testFinalizer = "test-finalizer"

type mockReconciler struct {
	reconcileInvoked bool
	finalizeInvoked  bool
	failReconcile    bool
	failFinalize     bool
}

func (t *mockReconciler) Object() client.Object {
	return &v1alpha1.Istio{}
}

func (t *mockReconciler) Reconcile(ctx context.Context, _ *v1alpha1.Istio) (ctrl.Result, error) {
	t.reconcileInvoked = true
	if t.failReconcile {
		return ctrl.Result{}, fmt.Errorf("reconcile failed")
	}
	return ctrl.Result{}, nil
}

func (t *mockReconciler) Finalize(ctx context.Context, _ *v1alpha1.Istio) error {
	t.finalizeInvoked = true
	if t.failFinalize {
		return fmt.Errorf("finalize failed")
	}
	return nil
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
				&v1alpha1.Istio{
					ObjectMeta: metav1.ObjectMeta{
						Name:              key.Name,
						DeletionTimestamp: oneMinuteAgo(),
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
				&v1alpha1.Istio{
					ObjectMeta: metav1.ObjectMeta{
						Name:              key.Name,
						DeletionTimestamp: oneMinuteAgo(),
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
				&v1alpha1.Istio{
					ObjectMeta: metav1.ObjectMeta{
						Name:              key.Name,
						DeletionTimestamp: oneMinuteAgo(),
						Finalizers:        []string{testFinalizer},
					},
				},
			},
			setup: func(g *WithT, mock *mockReconciler) {
				mock.failFinalize = true
			},
			assert: func(g *WithT, cl client.Client, result ctrl.Result, err error, mock *mockReconciler) {
				g.Expect(result).To(BeZero())
				g.Expect(err).To(HaveOccurred())
				g.Expect(mock.reconcileInvoked).To(BeFalse(), "reconcile should not be invoked when object is being deleted")
				g.Expect(mock.finalizeInvoked).To(BeTrue(), "finalize should be invoked when object is being deleted and still has the finalizer")

				obj := &v1alpha1.Istio{}
				g.Expect(cl.Get(ctx, key, obj)).To(Succeed())
				g.Expect(obj.GetFinalizers()).To(ContainElement(testFinalizer))
			},
		},
		{
			name: "adds finalizer when resource doesn't have it",
			objects: []client.Object{
				&v1alpha1.Istio{
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

				obj := &v1alpha1.Istio{}
				g.Expect(cl.Get(ctx, key, obj)).To(Succeed())
				g.Expect(obj.GetFinalizers()).To(ContainElement(testFinalizer))
			},
		},
		{
			name: "invokes reconcile if everything is fine",
			objects: []client.Object{
				&v1alpha1.Istio{
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
				&v1alpha1.Istio{
					ObjectMeta: metav1.ObjectMeta{
						Name:       key.Name,
						Finalizers: []string{testFinalizer},
					},
				},
			},
			setup: func(g *WithT, mock *mockReconciler) {
				mock.failReconcile = true
			},
			assert: func(g *WithT, cl client.Client, result ctrl.Result, err error, mock *mockReconciler) {
				g.Expect(result).To(BeZero())
				g.Expect(err).To(HaveOccurred())
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
				WithStatusSubresource(&v1alpha1.Istio{}).
				WithObjects(tt.objects...).
				WithInterceptorFuncs(tt.interceptorFuncs).
				Build()

			mock := &mockReconciler{}
			if tt.setup != nil {
				tt.setup(g, mock)
			}

			reconciler := NewStandardReconcilerWithFinalizer(cl, &v1alpha1.Istio{}, mock.Reconcile, mock.Finalize, testFinalizer)
			result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: key})

			tt.assert(g, cl, result, err, mock)
		})
	}
}

func oneMinuteAgo() *metav1.Time {
	t := metav1.NewTime(time.Now().Add(-1 * time.Minute))
	return &t
}
