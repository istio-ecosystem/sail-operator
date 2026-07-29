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
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeriveState returns the reason of the first non-True condition,
// or healthyReason if all conditions are True.
func DeriveState(healthyReason v1.ConditionReason, conditions ...v1.StatusCondition) v1.ConditionReason {
	for _, c := range conditions {
		if c.Status != metav1.ConditionTrue {
			return c.Reason
		}
	}
	return healthyReason
}
