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
	"fmt"
	"time"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// PruneInactive deletes IstioRevisions owned by the specified owner that are
// not in use and whose grace period has expired.
func PruneInactive(ctx context.Context, cl client.Client, ownerUID types.UID, activeRevisionName string, gracePeriod time.Duration) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	revisions, err := ListOwned(ctx, cl, ownerUID)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get revisions: %w", err)
	}

	// the following code does two things:
	// - prunes revisions whose grace period has expired
	// - finds the time when the next revision is to be pruned
	var nextPruneTimestamp *time.Time
	for _, rev := range revisions {
		if rev.Name == activeRevisionName {
			log.V(2).Info("IstioRevision is the active revision", "IstioRevision", rev.Name)
			continue
		}
		inUseCondition := rev.Status.GetCondition(v1.IstioRevisionConditionInUse)

		// Only prune revisions that are confirmed to be not in use (i.e., ConditionFalse).
		// Skip revisions that are in use (ConditionTrue) or whose usage status is unknown (ConditionUnknown).
		if inUseCondition.Status == metav1.ConditionTrue {
			log.V(2).Info("IstioRevision is in use", "IstioRevision", rev.Name)
			continue
		}
		if inUseCondition.Status != metav1.ConditionFalse {
			log.V(2).Info("IstioRevision usage status is unknown, skipping pruning", "IstioRevision", rev.Name)
			continue
		}

		pruneTimestamp := inUseCondition.LastTransitionTime.Time.Add(gracePeriod)
		expired := pruneTimestamp.Before(time.Now())
		if expired {
			log.Info("Deleting expired IstioRevision", "IstioRevision", rev.Name)
			err = cl.Delete(ctx, &rev)
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("delete failed: %w", err)
			}
		} else {
			log.V(2).Info("IstioRevision is not in use, but hasn't yet expired", "IstioRevision", rev.Name, "InUseLastTransitionTime", inUseCondition.LastTransitionTime)
			if nextPruneTimestamp == nil || nextPruneTimestamp.After(pruneTimestamp) {
				nextPruneTimestamp = &pruneTimestamp
			}
		}
	}
	if nextPruneTimestamp == nil {
		log.V(2).Info("No IstioRevisions to prune")
		return ctrl.Result{}, nil
	}

	requeueAfter := time.Until(*nextPruneTimestamp)
	log.Info("Requeueing resource for cleanup of expired IstioRevision", "RequeueAfter", requeueAfter)
	// requeue so that we prune the next revision at the right time (if we didn't, we would prune it when
	// something else triggers another reconciliation)
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}
