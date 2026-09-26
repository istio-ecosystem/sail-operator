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

package kube

import (
	"context"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// CRDsReady checks whether all named CRDs are established, serving at least one version, and not terminating.
func CRDsReady(ctx context.Context, reader client.Reader, crdNames ...string) (bool, error) {
	for _, name := range crdNames {
		crd := &apiextensionsv1.CustomResourceDefinition{}
		if err := reader.Get(ctx, client.ObjectKey{Name: name}, crd); err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		if crd.DeletionTimestamp != nil {
			return false, nil
		}
		established, served := false, false
		for _, condition := range crd.Status.Conditions {
			if condition.Type == apiextensionsv1.Established && condition.Status == apiextensionsv1.ConditionTrue {
				established = true
			}
		}
		for _, version := range crd.Spec.Versions {
			if version.Served {
				served = true
			}
		}
		if !established || !served {
			return false, nil
		}
	}
	return true, nil
}

// WaitForCRDs waits until all named CRDs are established, serving at least one version, and not terminating.
// It watches CRD objects, not custom resources. The shared informer handles list/watch retries and updates its
// store before notifying us, so readiness checks use the same cache as the event source.
func WaitForCRDs(ctx context.Context, crdCache cache.Cache, crdNames ...string) error {
	if len(crdNames) == 0 {
		return nil
	}
	required := make(map[string]struct{}, len(crdNames))
	for _, name := range crdNames {
		required[name] = struct{}{}
	}
	// GetInformer waits for the informer to sync by default.
	informer, err := crdCache.GetInformer(ctx, &apiextensionsv1.CustomResourceDefinition{})
	if err != nil {
		return err
	}
	changed := make(chan struct{}, 1)
	notify := func(obj any) {
		if tombstone, ok := obj.(toolscache.DeletedFinalStateUnknown); ok {
			obj = tombstone.Obj
		}
		crd, ok := obj.(*apiextensionsv1.CustomResourceDefinition)
		if !ok {
			return
		}
		if _, wanted := required[crd.Name]; !wanted {
			return
		}
		select {
		case changed <- struct{}{}:
		default:
		}
	}
	registration, err := informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc:    notify,
		UpdateFunc: func(_, obj any) { notify(obj) },
		DeleteFunc: notify,
	})
	if err != nil {
		return err
	}
	defer func() {
		if err := informer.RemoveEventHandler(registration); err != nil {
			logf.FromContext(ctx).Error(err, "Unable to remove CRD event handler")
		}
	}()

	// Subscribe before checking to avoid missing a change between the check and
	// registration. Coalesced notifications always trigger a fresh cache read.
	for {
		ready, err := CRDsReady(ctx, crdCache, crdNames...)
		if err != nil {
			return err
		}
		if ready {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-changed:
		}
	}
}
