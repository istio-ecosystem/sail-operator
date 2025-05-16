// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR Condition OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package matchers

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HaveConditionMatcher checks for a specific condition and status in a Kubernetes object.
type HaveConditionMatcher struct {
	conditionType      string
	conditionStatus    metav1.ConditionStatus
	conditionMessage   string
	lastSeenConditions []string // To store the last seen conditions for error reporting
}

// HaveConditionStatus creates a new HaveConditionMatcher that matches a condition status.
func HaveConditionStatus[T ~string](conditionType T, conditionStatus metav1.ConditionStatus) types.GomegaMatcher {
	return &HaveConditionMatcher{
		conditionType:   string(conditionType),
		conditionStatus: conditionStatus,
	}
}

// HaveConditionMessage creates a new HaveConditionMatcher that matches a condition message.
func HaveConditionMessage[T ~string](conditionType T, conditionMessage string) types.GomegaMatcher {
	return &HaveConditionMatcher{
		conditionType:    string(conditionType),
		conditionMessage: conditionMessage,
	}
}

// Match checks if the actual object has the specified condition and status.
func (matcher *HaveConditionMatcher) Match(actual any) (success bool, err error) {
	matcher.lastSeenConditions = []string{}

	val := reflect.ValueOf(actual)
	if val.Kind() == reflect.Ptr && !val.IsNil() {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return false, fmt.Errorf("expected a struct but got a %s; the object might be empty or not correctly passed", val.Kind())
	}

	status := val.FieldByName("Status")
	if !status.IsValid() {
		return false, fmt.Errorf("'Status' field not found in the object")
	}

	conditions := status.FieldByName("Conditions")
	if conditions.Kind() != reflect.Slice {
		return false, fmt.Errorf("'Conditions' is not a slice")
	}

	for i := range conditions.Len() {
		condition := conditions.Index(i).Interface()

		// Assuming the condition is of a type that has Type and Status fields
		// Adjust this part if your condition items are of a different type
		conditionVal := reflect.ValueOf(condition)
		if conditionVal.Kind() != reflect.Struct {
			continue // Skip if it's not a struct; this shouldn't happen
		}

		typeField := conditionVal.FieldByName("Type")
		statusField := conditionVal.FieldByName("Status")
		messageField := conditionVal.FieldByName("Message")

		// Record the condition's current state for error reporting
		if typeField.IsValid() && statusField.IsValid() && messageField.IsValid() {
			matcher.lastSeenConditions = append(matcher.lastSeenConditions, fmt.Sprintf("%s: %s (%s)", typeField, statusField, messageField))
		}
		if typeField.IsValid() && statusField.IsValid() && messageField.IsValid() &&
			typeField.String() == matcher.conditionType {
			if matcher.conditionStatus != "" && statusField.String() == string(matcher.conditionStatus) {
				return true, nil
			} else if matcher.conditionMessage != "" && strings.Contains(messageField.String(), matcher.conditionMessage) {
				return true, nil
			}
		}
	}

	// If we get here, no matching condition was found
	return false, nil
}

// FailureMessage is the message returned on matcher failure.
func (matcher *HaveConditionMatcher) FailureMessage(_ any) (message string) {
	return fmt.Sprintf("Expected object to have condition %s with status %s but last seen conditions were: %v",
		matcher.conditionType, matcher.conditionStatus, matcher.lastSeenConditions)
}

// NegatedFailureMessage is the message returned on negated matcher failure.
func (matcher *HaveConditionMatcher) NegatedFailureMessage(_ any) (message string) {
	return fmt.Sprintf("Expected object not to have condition %s with status %s", matcher.conditionType, matcher.conditionStatus)
}
