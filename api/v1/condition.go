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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConditionType represents the type of a status condition (e.g., "Reconciled", "Ready").
type ConditionType string

// ConditionReason represents a reason for a condition's current status.
type ConditionReason string

// StatusCondition represents a specific observation of an object's state.
type StatusCondition struct {
	// The type of this condition.
	Type ConditionType `json:"type,omitempty"`

	// The status of this condition. Can be True, False or Unknown.
	Status metav1.ConditionStatus `json:"status,omitempty"`

	// Unique, single-word, CamelCase reason for the condition's last transition.
	Reason ConditionReason `json:"reason,omitempty"`

	// Human-readable message indicating details about the last transition.
	Message string `json:"message,omitempty"`

	// Last time the condition transitioned from one status to another.
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitzero"`
}

// SetCondition sets a condition in a conditions slice, preserving LastTransitionTime
// when the status hasn't changed.
func SetCondition(conditions *[]StatusCondition, condition StatusCondition) {
	now := metav1.NewTime(time.Now().Truncate(time.Second))
	for i, existing := range *conditions {
		if existing.Type == condition.Type {
			if existing.Status != condition.Status {
				condition.LastTransitionTime = now
			} else {
				condition.LastTransitionTime = existing.LastTransitionTime
			}
			(*conditions)[i] = condition
			return
		}
	}
	condition.LastTransitionTime = now
	*conditions = append(*conditions, condition)
}

// GetCondition returns the condition of the specified type from the slice,
// or a default condition with Status=Unknown if not found.
func GetCondition(conditions []StatusCondition, conditionType ConditionType) StatusCondition {
	for _, c := range conditions {
		if c.Type == conditionType {
			return c
		}
	}
	return StatusCondition{Type: conditionType, Status: metav1.ConditionUnknown}
}
