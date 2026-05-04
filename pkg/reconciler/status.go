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
	"reflect"

	"github.com/istio-ecosystem/sail-operator/pkg/errlist"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// UpdateStatus compares the current and new status, and patches the object
// if they differ. It wraps any errors from status determination or patching.
func UpdateStatus[S any](ctx context.Context, cl client.Client, obj client.Object, currentStatus S, newStatus S, determineErr error) error {
	var errs errlist.Builder
	if determineErr != nil {
		errs.Add(fmt.Errorf("failed to determine status: %w", determineErr))
	}

	if !reflect.DeepEqual(currentStatus, newStatus) {
		if err := cl.Status().Patch(ctx, obj, kube.NewStatusPatch(newStatus)); err != nil {
			errs.Add(fmt.Errorf("failed to patch status: %w", err))
		}
	}
	return errs.Error()
}
