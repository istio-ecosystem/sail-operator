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

package istio

import (
	"context"
	"fmt"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	"github.com/istio-ecosystem/sail-operator/pkg/test/testtime"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"istio.io/istio/pkg/ptr"
)

var (
	ctx            = context.Background()
	istioNamespace = "my-istio-namespace"
	istioName      = "my-istio"
	istioKey       = types.NamespacedName{
		Name: istioName,
	}
	istioUID   = types.UID("my-istio-uid")
	objectMeta = metav1.ObjectMeta{
		Name: istioKey.Name,
	}
)

func TestReconcile(t *testing.T) {
	cfg := newReconcilerTestConfig(t)

	t.Run("returns error when Istio version not set", func(t *testing.T) {
		istio := &v1.Istio{
			ObjectMeta: objectMeta,
		}

		cl := newFakeClientBuilder().
			WithObjects(istio).
			Build()
		reconciler := NewReconciler(cfg, cl, scheme.Scheme)

		_, err := reconciler.Reconcile(ctx, istio)
		if err == nil {
			t.Errorf("Expected an error, but got nil")
		}

		Must(t, cl.Get(ctx, istioKey, istio))

		if istio.Status.State != v1.IstioReasonReconcileError {
			t.Errorf("Expected status.state to be %q, but got %q", v1.IstioReasonReconcileError, istio.Status.State)
		}

		reconciledCond := istio.Status.GetCondition(v1.IstioConditionReconciled)
		if reconciledCond.Status != metav1.ConditionFalse {
			t.Errorf("Expected Reconciled condition status to be %q, but got %q", metav1.ConditionFalse, reconciledCond.Status)
		}

		readyCond := istio.Status.GetCondition(v1.IstioConditionReady)
		if readyCond.Status != metav1.ConditionUnknown {
			t.Errorf("Expected Reconciled condition status to be %q, but got %q", metav1.ConditionUnknown, readyCond.Status)
		}
	})

	t.Run("returns error when computeIstioRevisionValues fails", func(t *testing.T) {
		istio := &v1.Istio{
			ObjectMeta: objectMeta,
			Spec: v1.IstioSpec{
				Version: "my-version",
			},
		}

		cl := newFakeClientBuilder().
			WithStatusSubresource(&v1.Istio{}).
			WithObjects(istio).
			Build()
		cfg := newReconcilerTestConfig(t)
		cfg.DefaultProfile = "invalid-profile"
		reconciler := NewReconciler(cfg, cl, scheme.Scheme)

		_, err := reconciler.Reconcile(ctx, istio)
		if err == nil {
			t.Errorf("Expected an error, but got nil")
		}

		Must(t, cl.Get(ctx, istioKey, istio))

		if istio.Status.State != v1.IstioReasonReconcileError {
			t.Errorf("Expected status.state to be %q, but got %q", v1.IstioReasonReconcileError, istio.Status.State)
		}

		reconciledCond := istio.Status.GetCondition(v1.IstioConditionReconciled)
		if reconciledCond.Status != metav1.ConditionFalse {
			t.Errorf("Expected Reconciled condition status to be %q, but got %q", metav1.ConditionFalse, reconciledCond.Status)
		}

		readyCond := istio.Status.GetCondition(v1.IstioConditionReady)
		if readyCond.Status != metav1.ConditionUnknown {
			t.Errorf("Expected Reconciled condition status to be %q, but got %q", metav1.ConditionUnknown, readyCond.Status)
		}
	})

	t.Run("returns error when reconcileActiveRevision fails", func(t *testing.T) {
		istio := &v1.Istio{
			ObjectMeta: objectMeta,
			Spec: v1.IstioSpec{
				Version:   "my-version",
				Namespace: "istio-system",
			},
		}

		cl := newFakeClientBuilder().
			WithObjects(istio).
			WithInterceptorFuncs(interceptor.Funcs{
				Create: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.CreateOption) error {
					return fmt.Errorf("internal error")
				},
			}).
			Build()
		reconciler := NewReconciler(cfg, cl, scheme.Scheme)

		_, err := reconciler.Reconcile(ctx, istio)
		if err == nil {
			t.Errorf("Expected an error, but got nil")
		}

		Must(t, cl.Get(ctx, istioKey, istio))

		if istio.Status.State != v1.IstioReasonReconcileError {
			t.Errorf("Expected status.state to be %q, but got %q", v1.IstioReasonReconcileError, istio.Status.State)
		}

		reconciledCond := istio.Status.GetCondition(v1.IstioConditionReconciled)
		if reconciledCond.Status != metav1.ConditionFalse {
			t.Errorf("Expected Reconciled condition status to be %q, but got %q", metav1.ConditionFalse, reconciledCond.Status)
		}

		if !strings.Contains(reconciledCond.Message, "version \"my-version\" not found") {
			t.Errorf("Expected Reconciled condition message to contain %q, but got %q", "version \"my-version\" not found", reconciledCond.Message)
		}

		readyCond := istio.Status.GetCondition(v1.IstioConditionReady)
		if readyCond.Status != metav1.ConditionUnknown {
			t.Errorf("Expected Reconciled condition status to be %q, but got %q", metav1.ConditionUnknown, readyCond.Status)
		}
	})
}

func TestValidate(t *testing.T) {
	testCases := []struct {
		name      string
		istio     *v1.Istio
		expectErr string
	}{
		{
			name: "success",
			istio: &v1.Istio{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1.IstioSpec{
					Version:   istioversion.Default,
					Namespace: "istio-system",
				},
			},
			expectErr: "",
		},
		{
			name: "no version",
			istio: &v1.Istio{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1.IstioSpec{
					Namespace: "istio-system",
				},
			},
			expectErr: "spec.version not set",
		},
		{
			name: "no namespace",
			istio: &v1.Istio{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
				},
				Spec: v1.IstioSpec{
					Version: istioversion.Default,
				},
			},
			expectErr: "spec.namespace not set",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			err := validate(tc.istio)
			if tc.expectErr == "" {
				g.Expect(err).ToNot(HaveOccurred())
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectErr))
			}
		})
	}
}

func TestDetermineStatus(t *testing.T) {
	cfg := newReconcilerTestConfig(t)

	generation := int64(100)

	ownedByIstio := metav1.OwnerReference{
		APIVersion:         v1.GroupVersion.String(),
		Kind:               v1.IstioKind,
		Name:               istioName,
		UID:                istioUID,
		Controller:         ptr.Of(true),
		BlockOwnerDeletion: ptr.Of(true),
	}

	ownedByAnotherIstio := metav1.OwnerReference{
		APIVersion:         v1.GroupVersion.String(),
		Kind:               v1.IstioKind,
		Name:               "some-other-Istio",
		UID:                "some-other-uid",
		Controller:         ptr.Of(true),
		BlockOwnerDeletion: ptr.Of(true),
	}

	revision := func(name string, ownerRef metav1.OwnerReference, reconciled, ready, inUse bool) v1.IstioRevision {
		return v1.IstioRevision{
			ObjectMeta: metav1.ObjectMeta{
				Name:            name,
				OwnerReferences: []metav1.OwnerReference{ownerRef},
			},
			Spec: v1.IstioRevisionSpec{Namespace: istioNamespace},
			Status: v1.IstioRevisionStatus{
				State: v1.IstioRevisionReasonHealthy,
				Conditions: []v1.IstioRevisionCondition{
					{Type: v1.IstioRevisionConditionReconciled, Status: toConditionStatus(reconciled)},
					{Type: v1.IstioRevisionConditionReady, Status: toConditionStatus(ready)},
					{Type: v1.IstioRevisionConditionDependenciesHealthy, Status: toConditionStatus(true)},
					{Type: v1.IstioRevisionConditionInUse, Status: toConditionStatus(inUse)},
				},
			},
		}
	}

	testCases := []struct {
		name              string
		reconciliationErr error
		istio             *v1.Istio
		revisions         []v1.IstioRevision
		interceptorFuncs  *interceptor.Funcs
		wantErr           bool
		expectedStatus    v1.IstioStatus
	}{
		{
			name:              "reconciliation error",
			reconciliationErr: fmt.Errorf("reconciliation error"),
			wantErr:           false,
			expectedStatus: v1.IstioStatus{
				State:              v1.IstioReasonReconcileError,
				ObservedGeneration: generation,
				Conditions: []v1.IstioCondition{
					{
						Type:    v1.IstioConditionReconciled,
						Status:  metav1.ConditionFalse,
						Reason:  v1.IstioReasonReconcileError,
						Message: "reconciliation error",
					},
					{
						Type:    v1.IstioConditionReady,
						Status:  metav1.ConditionUnknown,
						Reason:  v1.IstioReasonReconcileError,
						Message: "cannot determine readiness due to reconciliation error",
					},
				},
			},
		},
		{
			name:    "mirrors status of active revision",
			wantErr: false,
			revisions: []v1.IstioRevision{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            istioKey.Name,
						OwnerReferences: []metav1.OwnerReference{ownedByIstio},
					},
					Spec: v1.IstioRevisionSpec{
						Namespace: istioNamespace,
					},
					Status: v1.IstioRevisionStatus{
						State: v1.IstioRevisionReasonHealthy,
						Conditions: []v1.IstioRevisionCondition{
							{
								Type:    v1.IstioRevisionConditionReconciled,
								Status:  metav1.ConditionTrue,
								Reason:  v1.IstioRevisionReasonHealthy,
								Message: "reconciled message",
							},
							{
								Type:    v1.IstioRevisionConditionReady,
								Status:  metav1.ConditionTrue,
								Reason:  v1.IstioRevisionReasonHealthy,
								Message: "ready message",
							},
							{
								Type:    v1.IstioRevisionConditionDependenciesHealthy,
								Status:  metav1.ConditionTrue,
								Reason:  v1.IstioRevisionReasonHealthy,
								Message: "dependencies healthy message",
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            istioKey.Name + "-not-active",
						OwnerReferences: []metav1.OwnerReference{ownedByIstio},
					},
					Spec: v1.IstioRevisionSpec{
						Namespace: istioNamespace,
					},
					Status: v1.IstioRevisionStatus{
						State: v1.IstioRevisionReasonHealthy,
						Conditions: []v1.IstioRevisionCondition{
							{
								Type:    v1.IstioRevisionConditionReconciled,
								Status:  metav1.ConditionFalse,
								Reason:  v1.IstioRevisionReasonHealthy,
								Message: "shouldn't mirror this revision",
							},
							{
								Type:    v1.IstioRevisionConditionReady,
								Status:  metav1.ConditionFalse,
								Reason:  v1.IstioRevisionReasonHealthy,
								Message: "shouldn't mirror this revision",
							},
						},
					},
				},
			},
			expectedStatus: v1.IstioStatus{
				State:              v1.IstioReasonHealthy,
				ObservedGeneration: generation,
				Conditions: []v1.IstioCondition{
					{
						Type:    v1.IstioConditionReconciled,
						Status:  metav1.ConditionTrue,
						Reason:  v1.IstioReasonHealthy,
						Message: "reconciled message",
					},
					{
						Type:    v1.IstioConditionReady,
						Status:  metav1.ConditionTrue,
						Reason:  v1.IstioReasonHealthy,
						Message: "ready message",
					},
					{
						Type:    v1.IstioConditionDependenciesHealthy,
						Status:  metav1.ConditionTrue,
						Reason:  v1.IstioReasonHealthy,
						Message: "dependencies healthy message",
					},
				},
				ActiveRevisionName: istioKey.Name,
				Revisions: v1.RevisionSummary{
					Total: 2,
					Ready: 1,
					InUse: 0,
				},
			},
		},
		{
			name:    "shows correct revision counts",
			wantErr: false,
			revisions: []v1.IstioRevision{
				// owned by the Istio under test; 3 total, 2 ready, 1 in use
				revision(istioKey.Name, ownedByIstio, true, true, true),
				revision(istioKey.Name+"-old1", ownedByIstio, true, true, false),
				revision(istioKey.Name+"-old2", ownedByIstio, true, false, false),
				// not owned by the Istio being tested; shouldn't affect counts
				revision("some-other-istio", ownedByAnotherIstio, true, true, true),
			},
			expectedStatus: v1.IstioStatus{
				State:              v1.IstioReasonHealthy,
				ObservedGeneration: generation,
				Conditions: []v1.IstioCondition{
					{
						Type:   v1.IstioConditionReconciled,
						Status: metav1.ConditionTrue,
					},
					{
						Type:   v1.IstioConditionReady,
						Status: metav1.ConditionTrue,
					},
					{
						Type:   v1.IstioConditionDependenciesHealthy,
						Status: metav1.ConditionTrue,
					},
				},
				ActiveRevisionName: istioKey.Name,
				Revisions: v1.RevisionSummary{
					Total: 3,
					Ready: 2,
					InUse: 1,
				},
			},
		},
		{
			name:    "active revision not found",
			wantErr: false,
			expectedStatus: v1.IstioStatus{
				State:              v1.IstioReasonRevisionNotFound,
				ObservedGeneration: generation,
				Conditions: []v1.IstioCondition{
					{
						Type:    v1.IstioConditionReconciled,
						Status:  metav1.ConditionFalse,
						Reason:  v1.IstioReasonRevisionNotFound,
						Message: "active IstioRevision not found",
					},
					{
						Type:    v1.IstioConditionReady,
						Status:  metav1.ConditionFalse,
						Reason:  v1.IstioReasonRevisionNotFound,
						Message: "active IstioRevision not found",
					},
				},
				ActiveRevisionName: istioKey.Name,
			},
		},
		{
			name: "get active revision error",
			interceptorFuncs: &interceptor.Funcs{
				Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					if _, ok := obj.(*v1.IstioRevision); ok {
						return fmt.Errorf("simulated error")
					}
					return nil
				},
			},
			wantErr: true,
			expectedStatus: v1.IstioStatus{
				State:              v1.IstioReasonFailedToGetActiveRevision,
				ObservedGeneration: generation,
				Conditions: []v1.IstioCondition{
					{
						Type:    v1.IstioConditionReconciled,
						Status:  metav1.ConditionUnknown,
						Reason:  v1.IstioReasonFailedToGetActiveRevision,
						Message: "failed to get active IstioRevision: get failed: simulated error",
					},
					{
						Type:    v1.IstioConditionReady,
						Status:  metav1.ConditionUnknown,
						Reason:  v1.IstioReasonFailedToGetActiveRevision,
						Message: "failed to get active IstioRevision: get failed: simulated error",
					},
				},
				ActiveRevisionName: istioKey.Name,
				Revisions:          v1.RevisionSummary{},
			},
		},
		{
			name: "list revisions error",
			interceptorFuncs: &interceptor.Funcs{
				List: func(_ context.Context, _ client.WithWatch, list client.ObjectList, _ ...client.ListOption) error {
					if _, ok := list.(*v1.IstioRevisionList); ok {
						return fmt.Errorf("simulated error")
					}
					return nil
				},
			},
			wantErr: true,
			expectedStatus: v1.IstioStatus{
				State:              v1.IstioReasonRevisionNotFound,
				ObservedGeneration: generation,
				Conditions: []v1.IstioCondition{
					{
						Type:    v1.IstioConditionReconciled,
						Status:  metav1.ConditionFalse,
						Reason:  v1.IstioReasonRevisionNotFound,
						Message: "active IstioRevision not found",
					},
					{
						Type:    v1.IstioConditionReady,
						Status:  metav1.ConditionFalse,
						Reason:  v1.IstioReasonRevisionNotFound,
						Message: "active IstioRevision not found",
					},
				},
				ActiveRevisionName: istioKey.Name,
				Revisions: v1.RevisionSummary{
					Total: -1,
					Ready: -1,
					InUse: -1,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var interceptorFuncs interceptor.Funcs
			if tc.interceptorFuncs != nil {
				interceptorFuncs = *tc.interceptorFuncs
			}

			istio := tc.istio
			if istio == nil {
				istio = &v1.Istio{
					ObjectMeta: metav1.ObjectMeta{
						Name:       istioKey.Name,
						UID:        istioUID,
						Generation: 100,
					},
					Spec: v1.IstioSpec{
						Version:   "my-version",
						Namespace: istioNamespace,
					},
				}
			}

			initObjs := []client.Object{istio}
			for _, rev := range tc.revisions {
				initObjs = append(initObjs, &rev)
			}

			cl := newFakeClientBuilder().
				WithObjects(initObjs...).
				WithInterceptorFuncs(interceptorFuncs).
				Build()
			reconciler := NewReconciler(cfg, cl, scheme.Scheme)

			status, err := reconciler.determineStatus(ctx, istio, tc.reconciliationErr)
			if (err != nil) != tc.wantErr {
				t.Errorf("determineStatus() error = %v, wantErr %v", err, tc.wantErr)
			}

			if diff := cmp.Diff(tc.expectedStatus, clearTimestamps(status)); diff != "" {
				t.Errorf("returned status wasn't as expected; diff (-expected, +actual):\n%v", diff)
			}
		})
	}
}

func TestUpdateStatus(t *testing.T) {
	cfg := newReconcilerTestConfig(t)

	generation := int64(100)
	oneMinuteAgo := testtime.OneMinuteAgo()

	testCases := []struct {
		name              string
		reconciliationErr error
		istio             *v1.Istio
		revisions         []v1.IstioRevision
		interceptorFuncs  *interceptor.Funcs
		disallowWrites    bool
		wantErr           bool
		expectedStatus    v1.IstioStatus

		skipInterceptors bool // used internally by test implementation when it wants to get around the interceptor
	}{
		{
			name: "updates status even when determineStatus returns error",
			interceptorFuncs: &interceptor.Funcs{
				List: func(_ context.Context, _ client.WithWatch, list client.ObjectList, _ ...client.ListOption) error {
					if _, ok := list.(*v1.IstioRevisionList); ok {
						return fmt.Errorf("simulated error")
					}
					return nil
				},
			},
			wantErr: true,
			expectedStatus: v1.IstioStatus{
				State:              v1.IstioReasonRevisionNotFound,
				ObservedGeneration: generation,
				Conditions: []v1.IstioCondition{
					{
						Type:    v1.IstioConditionReconciled,
						Status:  metav1.ConditionFalse,
						Reason:  v1.IstioReasonRevisionNotFound,
						Message: "active IstioRevision not found",
					},
					{
						Type:    v1.IstioConditionReady,
						Status:  metav1.ConditionFalse,
						Reason:  v1.IstioReasonRevisionNotFound,
						Message: "active IstioRevision not found",
					},
				},
				ActiveRevisionName: istioKey.Name,
				Revisions: v1.RevisionSummary{
					Total: -1,
					Ready: -1,
					InUse: -1,
				},
			},
		},
		{
			name: "skips update when status unchanged",
			istio: &v1.Istio{
				ObjectMeta: metav1.ObjectMeta{
					Name:       istioKey.Name,
					UID:        istioUID,
					Generation: 100,
				},
				Spec: v1.IstioSpec{
					Version:   "my-version",
					Namespace: istioNamespace,
				},
				Status: v1.IstioStatus{
					ObservedGeneration: 100,
					State:              v1.IstioReasonHealthy,
					Conditions: []v1.IstioCondition{
						{
							Type:               v1.IstioConditionReconciled,
							Status:             metav1.ConditionTrue,
							Reason:             v1.IstioReasonHealthy,
							Message:            "reconciled message",
							LastTransitionTime: *oneMinuteAgo,
						},
						{
							Type:               v1.IstioConditionReady,
							Status:             metav1.ConditionTrue,
							Reason:             v1.IstioReasonHealthy,
							Message:            "ready message",
							LastTransitionTime: *oneMinuteAgo,
						},
						{
							Type:               v1.IstioConditionDependenciesHealthy,
							Status:             metav1.ConditionTrue,
							LastTransitionTime: *oneMinuteAgo,
						},
					},
					ActiveRevisionName: istioKey.Name,
				},
			},
			revisions: []v1.IstioRevision{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: istioKey.Name,
					},
					Spec: v1.IstioRevisionSpec{
						Namespace: istioNamespace,
					},
					Status: v1.IstioRevisionStatus{
						State: v1.IstioRevisionReasonHealthy,
						Conditions: []v1.IstioRevisionCondition{
							{
								Type:               v1.IstioRevisionConditionReconciled,
								Status:             metav1.ConditionTrue,
								Reason:             v1.IstioRevisionReasonHealthy,
								Message:            "reconciled message",
								LastTransitionTime: *oneMinuteAgo,
							},
							{
								Type:               v1.IstioRevisionConditionReady,
								Status:             metav1.ConditionTrue,
								Reason:             v1.IstioRevisionReasonHealthy,
								Message:            "ready message",
								LastTransitionTime: *oneMinuteAgo,
							},
							{
								Type:               v1.IstioRevisionConditionDependenciesHealthy,
								Status:             metav1.ConditionTrue,
								LastTransitionTime: *oneMinuteAgo,
							},
						},
					},
				},
			},
			expectedStatus: v1.IstioStatus{
				State:              v1.IstioReasonHealthy,
				ObservedGeneration: generation,
				Conditions: []v1.IstioCondition{
					{
						Type:    v1.IstioConditionReconciled,
						Status:  metav1.ConditionTrue,
						Reason:  v1.IstioReasonHealthy,
						Message: "reconciled message",
					},
					{
						Type:    v1.IstioConditionReady,
						Status:  metav1.ConditionTrue,
						Reason:  v1.IstioReasonHealthy,
						Message: "ready message",
					},
					{
						Type:   v1.IstioConditionDependenciesHealthy,
						Status: metav1.ConditionTrue,
					},
				},
				ActiveRevisionName: istioKey.Name,
			},
			disallowWrites: true,
			wantErr:        false,
		},
		{
			name: "returns status update error",
			interceptorFuncs: &interceptor.Funcs{
				SubResourcePatch: func(_ context.Context, _ client.Client, _ string, _ client.Object, _ client.Patch, _ ...client.SubResourcePatchOption) error {
					return fmt.Errorf("patch status error")
				},
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var interceptorFuncs interceptor.Funcs
			if tc.disallowWrites {
				if tc.interceptorFuncs != nil {
					panic("can't use disallowWrites and interceptorFuncs at the same time")
				}
				interceptorFuncs = noWrites(t)
			} else if tc.interceptorFuncs != nil {
				interceptorFuncs = *tc.interceptorFuncs
			}

			istio := tc.istio
			if istio == nil {
				istio = &v1.Istio{
					ObjectMeta: metav1.ObjectMeta{
						Name:       istioKey.Name,
						UID:        istioUID,
						Generation: 100,
					},
					Spec: v1.IstioSpec{
						Version:   "my-version",
						Namespace: istioNamespace,
					},
				}
			}

			initObjs := []client.Object{istio}
			for _, rev := range tc.revisions {
				initObjs = append(initObjs, &rev)
			}

			cl := newFakeClientBuilder().
				WithObjects(initObjs...).
				WithInterceptorFuncs(interceptorFuncs).
				Build()
			reconciler := NewReconciler(cfg, cl, scheme.Scheme)

			err := reconciler.updateStatus(ctx, istio, tc.reconciliationErr)
			if (err != nil) != tc.wantErr {
				t.Errorf("updateStatus() error = %v, wantErr %v", err, tc.wantErr)
			}

			Must(t, cl.Get(ctx, istioKey, istio))
			if diff := cmp.Diff(tc.expectedStatus, clearTimestamps(istio.Status)); diff != "" {
				t.Errorf("returned status wasn't as expected; diff (-expected, +actual):\n%v", diff)
			}
		})
	}
}

func clearTimestamps(status v1.IstioStatus) v1.IstioStatus {
	for i := range status.Conditions {
		status.Conditions[i].LastTransitionTime = metav1.Time{}
	}
	return status
}

func toConditionStatus(b bool) metav1.ConditionStatus {
	if b {
		return metav1.ConditionTrue
	}
	return metav1.ConditionFalse
}

func TestGetActiveRevisionName(t *testing.T) {
	tests := []struct {
		name                 string
		version              string
		updateStrategyType   *v1.UpdateStrategyType
		expectedRevisionName string
	}{
		{
			name:                 "No update strategy specified",
			version:              "1.0.0",
			updateStrategyType:   nil,
			expectedRevisionName: "test-istio",
		},
		{
			name:                 "InPlace",
			version:              "1.0.0",
			updateStrategyType:   ptr.Of(v1.UpdateStrategyTypeInPlace),
			expectedRevisionName: "test-istio",
		},
		{
			name:                 "RevisionBased v1.0.0",
			version:              "1.0.0",
			updateStrategyType:   ptr.Of(v1.UpdateStrategyTypeRevisionBased),
			expectedRevisionName: "test-istio-1-0-0",
		},
		{
			name:                 "RevisionBased v2.0.0",
			version:              "2.0.0",
			updateStrategyType:   ptr.Of(v1.UpdateStrategyTypeRevisionBased),
			expectedRevisionName: "test-istio-2-0-0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			istio := &v1.Istio{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-istio",
				},
				Spec: v1.IstioSpec{
					Version: tt.version,
				},
			}
			if tt.updateStrategyType != nil {
				istio.Spec.UpdateStrategy = &v1.IstioUpdateStrategy{
					Type: *tt.updateStrategyType,
				}
			}
			actual := getActiveRevisionName(istio)
			if actual != tt.expectedRevisionName {
				t.Errorf("getActiveRevisionName() = %v, want %v", actual, tt.expectedRevisionName)
			}
		})
	}
}

func newFakeClientBuilder() *fake.ClientBuilder {
	return fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithStatusSubresource(&v1.Istio{})
}

func TestGetPruningGracePeriod(t *testing.T) {
	tests := []struct {
		name           string
		updateStrategy *v1.IstioUpdateStrategy
		expected       time.Duration
	}{
		{
			name:           "Nil update strategy",
			updateStrategy: nil,
			expected:       v1.DefaultRevisionDeletionGracePeriodSeconds * time.Second,
		},
		{
			name:           "Nil grace period",
			updateStrategy: &v1.IstioUpdateStrategy{},
			expected:       v1.DefaultRevisionDeletionGracePeriodSeconds * time.Second,
		},
		{
			name: "Grace period less than minimum",
			updateStrategy: &v1.IstioUpdateStrategy{
				InactiveRevisionDeletionGracePeriodSeconds: ptr.Of(int64(v1.MinRevisionDeletionGracePeriodSeconds - 10)),
			},
			expected: v1.MinRevisionDeletionGracePeriodSeconds * time.Second,
		},
		{
			name: "Grace period more than minimum",
			updateStrategy: &v1.IstioUpdateStrategy{
				InactiveRevisionDeletionGracePeriodSeconds: ptr.Of(int64(v1.MinRevisionDeletionGracePeriodSeconds + 10)),
			},
			expected: (v1.MinRevisionDeletionGracePeriodSeconds + 10) * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			istio := &v1.Istio{
				Spec: v1.IstioSpec{
					UpdateStrategy: tt.updateStrategy,
				},
			}
			got := getPruningGracePeriod(istio)
			if got != tt.expected {
				t.Errorf("getPruningGracePeriod() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestManagesExternalRevision(t *testing.T) {
	tests := []struct {
		name     string
		spec     v1.IstioSpec
		expected bool
	}{
		{
			name:     "Empty spec.values does not manage",
			spec:     v1.IstioSpec{},
			expected: false,
		},
		{
			name:     "Empty spec.values.pilot does not manage",
			spec:     v1.IstioSpec{Values: &v1.Values{}},
			expected: false,
		},
		{
			name:     "Empty spec.values.pilot.env does not manage",
			spec:     v1.IstioSpec{Values: &v1.Values{Pilot: &v1.PilotConfig{}}},
			expected: false,
		},
		{
			name:     "env with EXTERNAL_ISTIOD=true does manage",
			spec:     v1.IstioSpec{Values: &v1.Values{Pilot: &v1.PilotConfig{Env: map[string]string{"EXTERNAL_ISTIOD": "true"}}}},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			istio := &v1.Istio{
				Spec: tt.spec,
			}
			got := managesExternalRevision(istio)
			if got != tt.expected {
				t.Errorf("managesExternalRevision() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConvertCondition(t *testing.T) {
	testCases := []struct {
		conditionType    v1.IstioRevisionConditionType
		expectedType     v1.IstioConditionType
		conditionReasons []v1.IstioRevisionConditionReason
		expectedReasons  []v1.IstioConditionReason
	}{
		{
			conditionType: v1.IstioRevisionConditionReconciled,
			expectedType:  v1.IstioConditionReconciled,
			conditionReasons: []v1.IstioRevisionConditionReason{
				v1.IstioRevisionReasonReconcileError,
			},
			expectedReasons: []v1.IstioConditionReason{
				v1.IstioReasonReconcileError,
			},
		},
		{
			conditionType: v1.IstioRevisionConditionReady,
			expectedType:  v1.IstioConditionReady,
			conditionReasons: []v1.IstioRevisionConditionReason{
				v1.IstioRevisionReasonIstiodNotReady,
				v1.IstioRevisionReasonReadinessCheckFailed,
				v1.IstioRevisionReasonRemoteIstiodNotReady,
				v1.IstioRevisionReasonReadinessCheckFailed,
			},
			expectedReasons: []v1.IstioConditionReason{
				v1.IstioReasonIstiodNotReady,
				v1.IstioReasonReadinessCheckFailed,
				v1.IstioReasonRemoteIstiodNotReady,
				v1.IstioReasonReadinessCheckFailed,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(string(tc.conditionType), func(t *testing.T) {
			got := convertCondition(
				v1.IstioRevisionCondition{
					Type:   tc.conditionType,
					Status: metav1.ConditionTrue,
				})
			expected := v1.IstioCondition{
				Type:   tc.expectedType,
				Status: metav1.ConditionTrue,
			}
			if diff := cmp.Diff(expected, got); diff != "" {
				t.Errorf("convertCondition() mismatch (-expected, +actual):\n%s", diff)
			}
		})

		for i, reason := range tc.conditionReasons {
			t.Run(fmt.Sprintf("%s %s", tc.conditionType, reason), func(t *testing.T) {
				got := convertCondition(
					v1.IstioRevisionCondition{
						Type:    tc.conditionType,
						Status:  metav1.ConditionFalse,
						Reason:  reason,
						Message: "some message",
					})
				expected := v1.IstioCondition{
					Type:    tc.expectedType,
					Status:  metav1.ConditionFalse,
					Reason:  tc.expectedReasons[i],
					Message: "some message",
				}
				if diff := cmp.Diff(expected, got); diff != "" {
					t.Errorf("convertCondition() mismatch (-expected, +actual):\n%s", diff)
				}
			})
		}
	}
}

func TestConvertState(t *testing.T) {
	testCases := []struct {
		revisionState v1.IstioRevisionConditionReason
		expected      v1.IstioConditionReason
	}{
		{revisionState: v1.IstioRevisionReasonHealthy, expected: v1.IstioReasonHealthy},
		{revisionState: v1.IstioRevisionReasonReconcileError, expected: v1.IstioReasonReconcileError},
		{revisionState: v1.IstioRevisionReasonIstiodNotReady, expected: v1.IstioReasonIstiodNotReady},
		{revisionState: v1.IstioRevisionReasonRemoteIstiodNotReady, expected: v1.IstioReasonRemoteIstiodNotReady},
		{revisionState: v1.IstioRevisionReasonReadinessCheckFailed, expected: v1.IstioReasonReadinessCheckFailed},
	}

	for _, tc := range testCases {
		t.Run(string(tc.revisionState), func(t *testing.T) {
			got := convertState(tc.revisionState)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func Must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func noWrites(t *testing.T) interceptor.Funcs {
	return interceptor.Funcs{
		Create: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.CreateOption) error {
			t.Fatal("unexpected call to Create in", string(debug.Stack()))
			return nil
		},
		Update: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.UpdateOption) error {
			t.Fatal("unexpected call to Update in", string(debug.Stack()))
			return nil
		},
		Delete: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.DeleteOption) error {
			t.Fatal("unexpected call to Delete in", string(debug.Stack()))
			return nil
		},
		Patch: func(_ context.Context, _ client.WithWatch, _ client.Object, _ client.Patch, _ ...client.PatchOption) error {
			t.Fatal("unexpected call to Patch in", string(debug.Stack()))
			return nil
		},
		DeleteAllOf: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.DeleteAllOfOption) error {
			t.Fatal("unexpected call to DeleteAllOf in", string(debug.Stack()))
			return nil
		},
		SubResourceCreate: func(_ context.Context, _ client.Client, _ string, _ client.Object, _ client.Object, _ ...client.SubResourceCreateOption) error {
			t.Fatal("unexpected call to SubResourceCreate in", string(debug.Stack()))
			return nil
		},
		SubResourceUpdate: func(_ context.Context, _ client.Client, _ string, _ client.Object, _ ...client.SubResourceUpdateOption) error {
			t.Fatal("unexpected call to SubResourceUpdate in", string(debug.Stack()))
			return nil
		},
		SubResourcePatch: func(_ context.Context, _ client.Client, _ string, obj client.Object, _ client.Patch, _ ...client.SubResourcePatchOption) error {
			t.Fatalf("unexpected call to SubResourcePatch with the object %+v: %v", obj, string(debug.Stack()))
			return nil
		},
	}
}

func newReconcilerTestConfig(t *testing.T) config.ReconcilerConfig {
	return config.ReconcilerConfig{
		ResourceDirectory:       t.TempDir(),
		Platform:                config.PlatformKubernetes,
		DefaultProfile:          "",
		MaxConcurrentReconciles: 1,
	}
}
