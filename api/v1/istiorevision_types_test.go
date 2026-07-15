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

package v1

import (
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetCondition(t *testing.T) {
	testCases := []struct {
		name           string
		istioStatus    *IstioRevisionStatus
		conditionType  IstioRevisionConditionType
		expectedResult StatusCondition
	}{
		{
			name: "condition found",
			istioStatus: &IstioRevisionStatus{
				Conditions: []StatusCondition{
					{
						Type:   IstioRevisionConditionReconciled,
						Status: metav1.ConditionTrue,
						Reason: ConditionReason(IstioRevisionConditionReconciled),
					},
					{
						Type:   IstioRevisionConditionReady,
						Status: metav1.ConditionFalse,
						Reason: IstioRevisionReasonIstiodNotReady,
					},
				},
			},
			conditionType: IstioRevisionConditionReady,
			expectedResult: StatusCondition{
				Type:   IstioRevisionConditionReady,
				Status: metav1.ConditionFalse,
				Reason: IstioRevisionReasonIstiodNotReady,
			},
		},
		{
			name: "condition not found",
			istioStatus: &IstioRevisionStatus{
				Conditions: []StatusCondition{
					{
						Type:   IstioRevisionConditionReconciled,
						Status: metav1.ConditionTrue,
						Reason: ConditionReason(IstioRevisionConditionReconciled),
					},
				},
			},
			conditionType: IstioRevisionConditionReady,
			expectedResult: StatusCondition{
				Type:   IstioRevisionConditionReady,
				Status: metav1.ConditionUnknown,
			},
		},
		{
			name:          "nil IstioRevisionStatus",
			istioStatus:   (*IstioRevisionStatus)(nil),
			conditionType: IstioRevisionConditionReady,
			expectedResult: StatusCondition{
				Type:   IstioRevisionConditionReady,
				Status: metav1.ConditionUnknown,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.istioStatus.GetCondition(tc.conditionType)
			if !reflect.DeepEqual(tc.expectedResult, result) {
				t.Errorf("Expected condition:\n    %+v,\n but got:\n    %+v", tc.expectedResult, result)
			}
		})
	}
}

func TestSetCondition(t *testing.T) {
	prevTime := time.Date(2023, 9, 26, 9, 0, 0, 0, time.UTC)

	testCases := []struct {
		name                          string
		existing                      []StatusCondition
		condition                     StatusCondition
		expectTransitionTimePreserved bool // if true, LastTransitionTime should match existing; if false, it should be updated
	}{
		{
			name: "add",
			existing: []StatusCondition{
				{
					Type:   IstioRevisionConditionReconciled,
					Status: metav1.ConditionTrue,
					Reason: ConditionReason(IstioRevisionConditionReconciled),
				},
			},
			condition: StatusCondition{
				Type:   IstioRevisionConditionReady,
				Status: metav1.ConditionFalse,
				Reason: IstioRevisionReasonIstiodNotReady,
			},
			expectTransitionTimePreserved: false,
		},
		{
			name: "update with status change",
			existing: []StatusCondition{
				{
					Type:               IstioRevisionConditionReconciled,
					Status:             metav1.ConditionTrue,
					Reason:             ConditionReason(IstioRevisionConditionReconciled),
					LastTransitionTime: metav1.NewTime(prevTime),
				},
				{
					Type:               IstioRevisionConditionReady,
					Status:             metav1.ConditionFalse,
					Reason:             IstioRevisionReasonIstiodNotReady,
					LastTransitionTime: metav1.NewTime(prevTime),
				},
			},
			condition: StatusCondition{
				Type:   IstioRevisionConditionReady,
				Status: metav1.ConditionTrue,
				Reason: ConditionReason(IstioRevisionConditionReady),
			},
			expectTransitionTimePreserved: false,
		},
		{
			name: "update without status change",
			existing: []StatusCondition{
				{
					Type:               IstioRevisionConditionReconciled,
					Status:             metav1.ConditionTrue,
					Reason:             ConditionReason(IstioRevisionConditionReconciled),
					LastTransitionTime: metav1.NewTime(prevTime),
				},
				{
					Type:               IstioRevisionConditionReady,
					Status:             metav1.ConditionFalse,
					Reason:             IstioRevisionReasonIstiodNotReady,
					LastTransitionTime: metav1.NewTime(prevTime),
				},
			},
			condition: StatusCondition{
				Type:   IstioRevisionConditionReady,
				Status: metav1.ConditionFalse, // same as previous status
				Reason: IstioRevisionReasonIstiodNotReady,
			},
			expectTransitionTimePreserved: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			status := IstioRevisionStatus{
				Conditions: tc.existing,
			}

			before := time.Now()
			status.SetCondition(tc.condition)
			after := time.Now()

			// Find the condition that was set
			var found *StatusCondition
			for i := range status.Conditions {
				if status.Conditions[i].Type == tc.condition.Type {
					found = &status.Conditions[i]
					break
				}
			}
			if found == nil {
				t.Fatal("condition not found after SetCondition")
			}

			if found.Status != tc.condition.Status {
				t.Errorf("Expected status %v, got %v", tc.condition.Status, found.Status)
			}
			if found.Reason != tc.condition.Reason {
				t.Errorf("Expected reason %v, got %v", tc.condition.Reason, found.Reason)
			}

			if tc.expectTransitionTimePreserved {
				if !found.LastTransitionTime.Equal(&metav1.Time{Time: prevTime}) {
					t.Errorf("Expected LastTransitionTime to be preserved as %v, but got %v", prevTime, found.LastTransitionTime)
				}
			} else {
				transitionTime := found.LastTransitionTime.Time
				// SetCondition truncates to the second, so compare with truncated bounds
				if transitionTime.Before(before.Truncate(time.Second)) || transitionTime.After(after.Truncate(time.Second).Add(time.Second)) {
					t.Errorf("Expected LastTransitionTime to be around %v, but got %v", before.Truncate(time.Second), transitionTime)
				}
			}
		})
	}
}
