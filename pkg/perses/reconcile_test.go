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
	"os"
	"path"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	"github.com/istio-ecosystem/sail-operator/pkg/test/project"
)

func TestReconcileDashboardsCreatesAll(t *testing.T) {
	ctx := context.Background()
	namespace := "sail-operator"
	cl := newPersesTestClient(t, testPersesDashboardCRD())
	fsys := os.DirFS(path.Join(project.RootDir, "resources", "perses"))

	result, err := ReconcileDashboards(ctx, cl, fsys, namespace)
	if err != nil {
		t.Fatalf("ReconcileDashboards() error = %v", err)
	}
	if !result.AllCreated {
		t.Fatal("expected all dashboards to be created")
	}

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{Group: "perses.dev", Version: "v1alpha2", Kind: "PersesDashboardList"})
	if err := cl.List(ctx, list, client.InNamespace(namespace)); err != nil {
		t.Fatalf("list dashboards: %v", err)
	}
	if len(list.Items) != len(ProductDashboards) {
		t.Fatalf("expected %d dashboards, got %d", len(ProductDashboards), len(list.Items))
	}
	for _, item := range list.Items {
		if len(item.GetOwnerReferences()) != 0 {
			t.Fatalf("dashboard %s has ownerReferences", item.GetName())
		}
	}
}

func TestReconcileDashboardsSkipsExisting(t *testing.T) {
	ctx := context.Background()
	namespace := "sail-operator"
	existing := &unstructured.Unstructured{}
	existing.SetGroupVersionKind(DashboardGVK)
	existing.SetName("istio-control-plane")
	existing.SetNamespace(namespace)
	existing.SetLabels(map[string]string{"custom": "true"})
	if err := unstructured.SetNestedMap(existing.Object, map[string]interface{}{
		"display": map[string]interface{}{"name": "user-managed"},
	}, "spec", "config"); err != nil {
		t.Fatalf("set nested map: %v", err)
	}

	cl := newPersesTestClient(t, testPersesDashboardCRD(), existing)
	fsys := os.DirFS(path.Join(project.RootDir, "resources", "perses"))

	result, err := ReconcileDashboards(ctx, cl, fsys, namespace)
	if err != nil {
		t.Fatalf("ReconcileDashboards() error = %v", err)
	}
	if !result.AllCreated {
		t.Fatal("expected reconciliation to succeed")
	}

	got := &unstructured.Unstructured{}
	got.SetGroupVersionKind(DashboardGVK)
	if err := cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: "istio-control-plane"}, got); err != nil {
		t.Fatalf("get existing dashboard: %v", err)
	}
	if got.GetLabels()["custom"] != "true" {
		t.Fatal("expected existing dashboard to remain unchanged")
	}
}

func TestReconcileDashboardsIdempotent(t *testing.T) {
	ctx := context.Background()
	namespace := "sail-operator"
	cl := newPersesTestClient(t, testPersesDashboardCRD())
	fsys := os.DirFS(path.Join(project.RootDir, "resources", "perses"))

	if _, err := ReconcileDashboards(ctx, cl, fsys, namespace); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	if _, err := ReconcileDashboards(ctx, cl, fsys, namespace); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{Group: "perses.dev", Version: "v1alpha2", Kind: "PersesDashboardList"})
	if err := cl.List(ctx, list, client.InNamespace(namespace)); err != nil {
		t.Fatalf("list dashboards: %v", err)
	}
	if len(list.Items) != len(ProductDashboards) {
		t.Fatalf("expected %d dashboards after idempotent reconcile, got %d", len(ProductDashboards), len(list.Items))
	}
}

func newPersesTestClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()
	return fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		WithObjects(objects...).
		Build()
}

func testPersesDashboardCRD() *apiextensionsv1.CustomResourceDefinition {
	preserve := true
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: PersesDashboardCRD},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "perses.dev",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:     "PersesDashboard",
				ListKind: "PersesDashboardList",
				Plural:   "persesdashboards",
				Singular: "persesdashboard",
			},
			Scope: apiextensionsv1.NamespaceScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{
				Name:    "v1alpha2",
				Served:  true,
				Storage: true,
				Schema: &apiextensionsv1.CustomResourceValidation{
					OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
						Type:                   "object",
						XPreserveUnknownFields: &preserve,
					},
				},
			}},
		},
	}
}
