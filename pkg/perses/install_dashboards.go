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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// installDashboards creates PersesDashboard resources in the target namespace when they do not exist.
// Existing dashboards are left unchanged. Dashboard YAML is discovered by walking dashboards/ under
// fsys. The CRD must already be available; callers that need to wait for the CRD should use
// kube.WaitForCRDs first.
func installDashboards(
	ctx context.Context,
	cl client.Client,
	fsys fs.FS,
	namespace string,
) error {
	log := ctrl.LoggerFrom(ctx)
	created, existing := 0, 0

	err := forEachDashboardYAML(fsys, func(path string, data []byte) error {
		dashboard, err := PrepareDashboard(data, namespace)
		if err != nil {
			return fmt.Errorf("prepare dashboard %s: %w", path, err)
		}
		wasCreated, err := createIfNotExists(ctx, cl, dashboard)
		if err != nil {
			return fmt.Errorf("create dashboard %s: %w", dashboard.GetName(), err)
		}
		if wasCreated {
			created++
		} else {
			existing++
		}
		return nil
	})
	if err != nil {
		return err
	}
	if created == 0 && existing == 0 {
		return fmt.Errorf("no Perses dashboard YAML found under %s/", dashboardsDir)
	}

	log.Info("Perses dashboards installed", "namespace", namespace, "created", created, "existing", existing)
	return nil
}

func createIfNotExists(ctx context.Context, cl client.Client, desired *unstructured.Unstructured) (bool, error) {
	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(desired.GroupVersionKind())
	err := cl.Get(ctx, client.ObjectKey{Namespace: desired.GetNamespace(), Name: desired.GetName()}, existing)
	if err == nil {
		return false, nil
	}
	if !apierrors.IsNotFound(err) {
		return false, err
	}
	if err := cl.Create(ctx, desired); err != nil {
		return false, err
	}
	return true, nil
}
