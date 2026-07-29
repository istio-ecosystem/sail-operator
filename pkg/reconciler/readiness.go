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

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CheckDaemonSetReadiness checks a DaemonSet's readiness state and returns
// the appropriate status condition. If the GET operation itself fails (not
// a NotFound), the error is also returned for logging.
func CheckDaemonSetReadiness(
	ctx context.Context,
	cl client.Client,
	key client.ObjectKey,
	componentName string,
	readyConditionType v1.ConditionType,
	notReadyReason v1.ConditionReason,
	checkFailedReason v1.ConditionReason,
) (v1.StatusCondition, error) {
	c := v1.StatusCondition{
		Type:   readyConditionType,
		Status: metav1.ConditionFalse,
	}

	ds := appsv1.DaemonSet{}
	if err := cl.Get(ctx, key, &ds); err == nil {
		if ds.Status.CurrentNumberScheduled == 0 {
			c.Reason = notReadyReason
			c.Message = fmt.Sprintf("no %s pods are currently scheduled", componentName)
		} else if ds.Status.NumberReady < ds.Status.CurrentNumberScheduled {
			c.Reason = notReadyReason
			c.Message = fmt.Sprintf("not all %s pods are ready", componentName)
		} else {
			c.Status = metav1.ConditionTrue
			c.Reason = v1.ConditionReason(readyConditionType)
		}
	} else if apierrors.IsNotFound(err) {
		c.Reason = notReadyReason
		c.Message = fmt.Sprintf("%s DaemonSet not found", componentName)
	} else {
		c.Status = metav1.ConditionUnknown
		c.Reason = checkFailedReason
		c.Message = fmt.Sprintf("failed to get readiness: %v", err)
		return c, fmt.Errorf("get failed: %w", err)
	}
	return c, nil
}
