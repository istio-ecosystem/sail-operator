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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func CreateOrUpdate(
	ctx context.Context, cl client.Client, revName string, version string, namespace string,
	values *v1.Values, ownerRef metav1.OwnerReference,
) error {
	log := logf.FromContext(ctx)
	log = log.WithValues("IstioRevision", revName)

	rev, found, err := getRevision(ctx, cl, revName)
	if err != nil {
		return fmt.Errorf("failed to get active IstioRevision: %w", err)
	}

	if found {
		// update
		rev.Spec.Version = version
		rev.Spec.Values = values
		log.Info("Updating IstioRevision")
		if err = cl.Update(ctx, &rev); err != nil {
			return fmt.Errorf("failed to update IstioRevision %q: %w", rev.Name, err)
		}
	} else {
		// create new
		rev = v1.IstioRevision{
			ObjectMeta: metav1.ObjectMeta{
				Name:            revName,
				OwnerReferences: []metav1.OwnerReference{ownerRef},
			},
			Spec: v1.IstioRevisionSpec{
				Version:   version,
				Namespace: namespace,
				Values:    values,
			},
		}
		log.Info("Creating IstioRevision")
		if err = cl.Create(ctx, &rev); err != nil {
			return fmt.Errorf("failed to create IstioRevision %q: %w", rev.Name, err)
		}
	}
	return nil
}

func getRevision(ctx context.Context, cl client.Client, name string) (rev v1.IstioRevision, found bool, err error) {
	key := types.NamespacedName{Name: name}
	rev = v1.IstioRevision{}
	err = cl.Get(ctx, key, &rev)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return rev, false, nil
		}
		return rev, false, fmt.Errorf("get failed: %w", err)
	}
	return rev, true, nil
}
