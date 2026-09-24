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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ReconcileResult captures the outcome of dashboard reconciliation.
type ReconcileResult struct {
	CRDsAvailable bool
	AllCreated    bool
	FailedNames   []string
}

// ReconcileDashboards creates PersesDashboard resources in the target namespace when they do not exist.
// Existing dashboards are left unchanged. The CRD must already be available; callers that need to wait
// for the CRD should use kube.WaitForCRDs first.
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
		if err := createIfNotExists(ctx, cl, dashboard); err != nil {
			result.AllCreated = false
			result.FailedNames = append(result.FailedNames, def.Name)
			return result, fmt.Errorf("create dashboard %s: %w", def.Name, err)
		}
	}

	return result, nil
}

func createIfNotExists(ctx context.Context, cl client.Client, desired *unstructured.Unstructured) error {
	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(desired.GroupVersionKind())
	err := cl.Get(ctx, client.ObjectKey{Namespace: desired.GetNamespace(), Name: desired.GetName()}, existing)
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}
	return cl.Create(ctx, desired)
}
