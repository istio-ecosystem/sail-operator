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

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ListOwned returns all IstioRevisions owned by the given owner.
func ListOwned(ctx context.Context, cl client.Client, ownerUID types.UID) ([]v1.IstioRevision, error) {
	revList := v1.IstioRevisionList{}
	if err := cl.List(ctx, &revList); err != nil {
		return nil, fmt.Errorf("list failed: %w", err)
	}

	var revisions []v1.IstioRevision
	for _, rev := range revList.Items {
		if isOwnedRevision(rev, ownerUID) {
			revisions = append(revisions, rev)
		}
	}
	return revisions, nil
}

func isOwnedRevision(rev v1.IstioRevision, ownerUID types.UID) bool {
	if ownerUID == "" {
		panic("resource has no UID; did you forget to set it in your test?")
	}
	for _, owner := range rev.OwnerReferences {
		if owner.UID == ownerUID {
			return true
		}
	}
	return false
}
