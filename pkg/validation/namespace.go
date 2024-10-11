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

	"github.com/istio-ecosystem/sail-operator/api/v1alpha1"
	"github.com/istio-ecosystem/sail-operator/pkg/reconciler"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

func IstioRevisionTagExists(ctx context.Context, cl client.Client, name string) (bool, error) {
	tag := &v1alpha1.IstioRevisionTag{}
	if err := cl.Get(ctx, types.NamespacedName{Name: name}, tag); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
