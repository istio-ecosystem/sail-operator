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

package perses

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/equality"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ReconcileResult captures the outcome of dashboard reconciliation.
type ReconcileResult struct {
	CRDsAvailable bool
	AllCreated    bool
	FailedNames   []string
}

// ReconcileDashboards creates or updates PersesDashboard resources in the target namespace.
// Dashboards managed by the operator (app.kubernetes.io/managed-by=sail-operator) are updated
// when the bundled spec differs from the cluster. User-managed dashboards (no managed-by label)
// are left unchanged. The CRD must already be available; callers that need to wait for the CRD
// should use kube.WaitForCRDs first.
func ReconcileDashboards(
	ctx context.Context,
	cl client.Client,
	fsys fs.FS,
	namespace string,
) (ReconcileResult, error) {
	result := ReconcileResult{CRDsAvailable: true, AllCreated: true}

	for _, def := range ProductDashboards {
		raw, err := LoadDashboardYAML(fsys, def)
		if err != nil {
			return result, err
		}
		dashboard, err := PrepareDashboard(raw, namespace, def)
		if err != nil {
			return result, err
		}
		if err := createOrUpdateIfChanged(ctx, cl, dashboard); err != nil {
			result.AllCreated = false
			result.FailedNames = append(result.FailedNames, def.Name)
			return result, fmt.Errorf("reconcile dashboard %s: %w", def.Name, err)
		}
	}

	return result, nil
}

func createOrUpdateIfChanged(ctx context.Context, cl client.Client, desired *unstructured.Unstructured) error {
	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(desired.GroupVersionKind())
	err := cl.Get(ctx, client.ObjectKey{Namespace: desired.GetNamespace(), Name: desired.GetName()}, existing)
	if apierrors.IsNotFound(err) {
		return cl.Create(ctx, desired)
	}
	if err != nil {
		return err
	}

	// Preserve dashboards not managed by the operator.
	if existing.GetLabels()[constants.KubernetesAppManagedByKey] != constants.ManagedByLabelValue {
		return nil
	}

	desiredSpec, _, err := unstructured.NestedMap(desired.Object, "spec")
	if err != nil {
		return fmt.Errorf("desired spec: %w", err)
	}
	existingSpec, _, err := unstructured.NestedMap(existing.Object, "spec")
	if err != nil {
		return fmt.Errorf("existing spec: %w", err)
	}
	if equality.Semantic.DeepEqual(desiredSpec, existingSpec) {
		return nil
	}

	if err := unstructured.SetNestedMap(existing.Object, desiredSpec, "spec"); err != nil {
		return err
	}
	labels := existing.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[constants.KubernetesAppManagedByKey] = constants.ManagedByLabelValue
	existing.SetLabels(labels)
	existing.SetOwnerReferences(nil)
	return cl.Update(ctx, existing)
}
