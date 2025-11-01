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

package revision

import (
	"context"
	"testing"
	"time"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"istio.io/istio/pkg/ptr"
)

func TestPruneInactive(t *testing.T) {
	const (
		istioName = "my-istio"
		istioUID  = "my-uid"
		version   = "my-version"
	)

	ctx := context.Background()

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

	tenSecondsAgo := metav1.Time{Time: time.Now().Add(-10 * time.Second)}
	oneMinuteAgo := metav1.Time{Time: time.Now().Add(-1 * time.Minute)}
	testCases := []struct {
		name                string
		revName             string
		ownerReference      metav1.OwnerReference
		inUseCondition      *v1.IstioRevisionCondition
		rev                 *v1.IstioRevision
		expectDeletion      bool
		expectRequeueAfter  *time.Duration
		additionalRevisions []*v1.IstioRevision
	}{
		{
			name:           "preserves active IstioRevision even if not in use",
			revName:        istioName,
			ownerReference: ownedByIstio,
			inUseCondition: &v1.IstioRevisionCondition{
				Type:               v1.IstioRevisionConditionInUse,
				Status:             metav1.ConditionFalse,
				LastTransitionTime: oneMinuteAgo,
			},
			expectDeletion:     false,
			expectRequeueAfter: nil,
		},
		{
			name:           "preserves non-active IstioRevision that's in use",
			revName:        istioName + "-non-active",
			ownerReference: ownedByIstio,
			inUseCondition: &v1.IstioRevisionCondition{
				Type:               v1.IstioRevisionConditionInUse,
				Status:             metav1.ConditionTrue,
				LastTransitionTime: tenSecondsAgo,
			},
			expectDeletion:     false,
			expectRequeueAfter: nil,
		},
		{
			name:           "preserves unused non-active IstioRevision during grace period",
			revName:        istioName + "-non-active",
			ownerReference: ownedByIstio,
			inUseCondition: &v1.IstioRevisionCondition{
				Type:               v1.IstioRevisionConditionInUse,
				Status:             metav1.ConditionFalse,
				LastTransitionTime: tenSecondsAgo,
			},
			expectDeletion:     false,
			expectRequeueAfter: ptr.Of((v1.DefaultRevisionDeletionGracePeriodSeconds - 10) * time.Second),
		},
		{
			name:           "preserves IstioRevision owned by a different Istio",
			revName:        "other-istio-non-active",
			ownerReference: ownedByAnotherIstio,
			inUseCondition: &v1.IstioRevisionCondition{
				Type:               v1.IstioRevisionConditionInUse,
				Status:             metav1.ConditionFalse,
				LastTransitionTime: oneMinuteAgo,
			},
			expectDeletion:     false,
			expectRequeueAfter: nil,
		},
		{
			name:           "deletes non-active IstioRevision that's not in use",
			revName:        istioName + "-non-active",
			ownerReference: ownedByIstio,
			inUseCondition: &v1.IstioRevisionCondition{
				Type:               v1.IstioRevisionConditionInUse,
				Status:             metav1.ConditionFalse,
				LastTransitionTime: oneMinuteAgo,
			},
			expectDeletion:     true,
			expectRequeueAfter: nil,
		},
		{
			name:           "returns requeueAfter of earliest IstioRevision requiring pruning",
			revName:        istioName + "-non-active",
			ownerReference: ownedByIstio,
			inUseCondition: &v1.IstioRevisionCondition{
				Type:               v1.IstioRevisionConditionInUse,
				Status:             metav1.ConditionFalse,
				LastTransitionTime: oneMinuteAgo,
			},
			additionalRevisions: []*v1.IstioRevision{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            istioName + "-non-active2",
						OwnerReferences: []metav1.OwnerReference{ownedByIstio},
					},
					Status: v1.IstioRevisionStatus{
						Conditions: []v1.IstioRevisionCondition{
							{
								Type:               v1.IstioRevisionConditionInUse,
								Status:             metav1.ConditionFalse,
								LastTransitionTime: metav1.Time{Time: time.Now().Add(-25 * time.Second)},
							},
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:            istioName + "-non-active3",
						OwnerReferences: []metav1.OwnerReference{ownedByIstio},
					},
					Status: v1.IstioRevisionStatus{
						Conditions: []v1.IstioRevisionCondition{
							{
								Type:               v1.IstioRevisionConditionInUse,
								Status:             metav1.ConditionFalse,
								LastTransitionTime: metav1.Time{Time: time.Now().Add(-20 * time.Second)},
							},
						},
					},
				},
			},
			expectDeletion:     true,
			expectRequeueAfter: ptr.Of((v1.DefaultRevisionDeletionGracePeriodSeconds - 25) * time.Second),
		},
		{
			name:           "preserves non-active IstioRevision with unknown usage status",
			revName:        istioName + "-non-active",
			ownerReference: ownedByIstio,
			inUseCondition: &v1.IstioRevisionCondition{
				Type:               v1.IstioRevisionConditionInUse,
				Status:             metav1.ConditionUnknown,
				Reason:             v1.IstioRevisionReasonUsageCheckFailed,
				Message:            "failed to determine if revision is in use",
				LastTransitionTime: oneMinuteAgo,
			},
			expectDeletion:     false,
			expectRequeueAfter: nil,
		},
		{
			name:           "preserves non-active IstioRevision with missing InUse condition",
			revName:        istioName + "-non-active",
			ownerReference: ownedByIstio,
			// No InUse condition is set to simulate GetCondition returning ConditionUnknown
			inUseCondition:     nil,
			expectDeletion:     false,
			expectRequeueAfter: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			istio := &v1.Istio{
				ObjectMeta: metav1.ObjectMeta{
					Name: istioName,
					UID:  istioUID,
				},
				Spec: v1.IstioSpec{
					Version: version,
				},
			}

			rev := &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name:            tc.revName,
					OwnerReferences: []metav1.OwnerReference{tc.ownerReference},
				},
			}

			if tc.inUseCondition != nil {
				rev.Status = v1.IstioRevisionStatus{
					Conditions: []v1.IstioRevisionCondition{*tc.inUseCondition},
				}
			}

			initObjs := []client.Object{istio, rev}
			for _, additionalRev := range tc.additionalRevisions {
				initObjs = append(initObjs, additionalRev)
			}

			cl := newFakeClientBuilder().WithObjects(initObjs...).Build()

			activeRevisionName := istioName
			gracePeriod := v1.DefaultRevisionDeletionGracePeriodSeconds * time.Second

			result, err := PruneInactive(ctx, cl, istio.UID, activeRevisionName, gracePeriod)
			if err != nil {
				t.Errorf("Expected no error, but got: %v", err)
			}

			revisionWasDeleted := errors.IsNotFound(cl.Get(ctx, client.ObjectKeyFromObject(rev), rev))
			if tc.expectDeletion && !revisionWasDeleted {
				t.Error("Expected IstioRevision to be deleted, but it wasn't")
			} else if revisionWasDeleted && !tc.expectDeletion {
				t.Error("Expected IstioRevision to be preserved, but it was deleted")
			}

			if tc.expectRequeueAfter == nil {
				if result.RequeueAfter != 0 {
					t.Errorf("Didn't expect Istio to be requeued, but it was; requeueAfter: %v", result.RequeueAfter)
				}
			} else {
				if result.RequeueAfter == 0 {
					t.Error("Expected Istio to be requeued, but it wasn't")
				} else {
					diff := abs(result.RequeueAfter - *tc.expectRequeueAfter)
					if diff > time.Second {
						t.Errorf("Expected result.RequeueAfter to be around %v, but got %v", *tc.expectRequeueAfter, result.RequeueAfter)
					}
				}
			}
		})
	}
}

func abs(duration time.Duration) time.Duration {
	if duration < 0 {
		return -duration
	}
	return duration
}

func newFakeClientBuilder() *fake.ClientBuilder {
	return fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithStatusSubresource(&v1.Istio{})
}
