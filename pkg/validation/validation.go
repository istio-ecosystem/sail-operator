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

package validation

import (
	"context"
	"fmt"
	"strings"

	"github.com/istio-ecosystem/sail-operator/pkg/reconciler"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ValidateTargetNamespace checks if the target namespace exists and is not being deleted.
func ValidateTargetNamespace(ctx context.Context, cl client.Client, namespace string) error {
	ns := &corev1.Namespace{}
	if err := cl.Get(ctx, types.NamespacedName{Name: namespace}, ns); err != nil {
		if apierrors.IsNotFound(err) {
			return reconciler.NewValidationError(fmt.Sprintf("namespace %q doesn't exist", namespace))
		}
		return fmt.Errorf("get failed: %w", err)
	}
	if ns.DeletionTimestamp != nil {
		return reconciler.NewValidationError(fmt.Sprintf("namespace %q is being deleted", namespace))
	}
	return nil
}

// ResourceTakesPrecedence returns `true` if `object1` takes precedence over `object2`. This is used in
// cases where IstioRevision and IstioRevisionTag objects have the same name, to decide which will get
// reconciled.
// The decision is based on creation date, but if those are equal, a lexicographic comparison of the
// objects' UIDs is used as tie-breaker.
func ResourceTakesPrecedence(object1 *metav1.ObjectMeta, object2 *metav1.ObjectMeta) bool {
	return object1.CreationTimestamp.Before(&object2.CreationTimestamp) ||
		(object1.CreationTimestamp.Equal(&object2.CreationTimestamp) && strings.Compare(string(object1.UID), string(object2.UID)) < 0)
}
