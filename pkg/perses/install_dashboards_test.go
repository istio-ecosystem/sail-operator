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
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"testing"
	"testing/fstest"

	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	"github.com/istio-ecosystem/sail-operator/pkg/test/project"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestInstallDashboardsCreatesAll(t *testing.T) {
	ctx := context.Background()
	namespace := "sail-operator"
	cl := newPersesTestClient(t, testPersesDashboardCRD())
	fsys := os.DirFS(path.Join(project.RootDir, "pkg", "perses", "resources"))
	want := countDashboardYAMLs(t, fsys)

	if err := installDashboards(ctx, cl, fsys, namespace); err != nil {
		t.Fatalf("installDashboards() error = %v", err)
	}

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{Group: "perses.dev", Version: "v1alpha2", Kind: "PersesDashboardList"})
	if err := cl.List(ctx, list, client.InNamespace(namespace)); err != nil {
		t.Fatalf("list dashboards: %v", err)
	}
	if len(list.Items) != want {
		t.Fatalf("expected %d dashboards, got %d", want, len(list.Items))
	}
	for _, item := range list.Items {
		if len(item.GetOwnerReferences()) != 0 {
			t.Fatalf("dashboard %s has ownerReferences", item.GetName())
		}
	}
}

func TestInstallDashboardsSkipsExisting(t *testing.T) {
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
	fsys := os.DirFS(path.Join(project.RootDir, "pkg", "perses", "resources"))

	if err := installDashboards(ctx, cl, fsys, namespace); err != nil {
		t.Fatalf("installDashboards() error = %v", err)
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

func TestInstallDashboardsIdempotent(t *testing.T) {
	ctx := context.Background()
	namespace := "sail-operator"
	cl := newPersesTestClient(t, testPersesDashboardCRD())
	fsys := os.DirFS(path.Join(project.RootDir, "pkg", "perses", "resources"))
	want := countDashboardYAMLs(t, fsys)

	if err := installDashboards(ctx, cl, fsys, namespace); err != nil {
		t.Fatalf("first install: %v", err)
	}
	if err := installDashboards(ctx, cl, fsys, namespace); err != nil {
		t.Fatalf("second install: %v", err)
	}

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{Group: "perses.dev", Version: "v1alpha2", Kind: "PersesDashboardList"})
	if err := cl.List(ctx, list, client.InNamespace(namespace)); err != nil {
		t.Fatalf("list dashboards: %v", err)
	}
	if len(list.Items) != want {
		t.Fatalf("expected %d dashboards after idempotent install, got %d", want, len(list.Items))
	}
}

func TestInstallDashboardsLoadError(t *testing.T) {
	cl := newPersesTestClient(t, testPersesDashboardCRD())
	err := installDashboards(context.Background(), cl, os.DirFS(t.TempDir()), "sail-operator")
	if err == nil {
		t.Fatal("expected load error")
	}
}

func TestInstallDashboardsEmptyDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(path.Join(dir, "dashboards"), 0o755); err != nil {
		t.Fatal(err)
	}
	cl := newPersesTestClient(t, testPersesDashboardCRD())
	err := installDashboards(context.Background(), cl, os.DirFS(dir), "sail-operator")
	if err == nil {
		t.Fatal("expected error when no YAML found")
	}
}

func TestInstallDashboardsPrepareError(t *testing.T) {
	fsys := fstest.MapFS{
		"dashboards/broken.yaml": &fstest.MapFile{Data: []byte(":::")},
	}
	cl := newPersesTestClient(t, testPersesDashboardCRD())
	err := installDashboards(context.Background(), cl, fs.FS(fsys), "sail-operator")
	if err == nil {
		t.Fatal("expected prepare error")
	}
}

func TestInstallDashboardsCreateError(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(testPersesDashboardCRD()).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(context.Context, client.WithWatch, client.Object, ...client.CreateOption) error {
				return fmt.Errorf("create failed")
			},
		}).Build()
	fsys := os.DirFS(path.Join(project.RootDir, "pkg", "perses", "resources"))
	err := installDashboards(context.Background(), cl, fsys, "sail-operator")
	if err == nil {
		t.Fatal("expected create error")
	}
}

func TestCreateIfNotExistsGetError(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithInterceptorFuncs(interceptor.Funcs{
		Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
			return apierrors.NewInternalError(errors.New("get failed"))
		},
	}).Build()
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(DashboardGVK)
	obj.SetName("istio-control-plane")
	obj.SetNamespace("sail-operator")
	if _, err := createIfNotExists(context.Background(), cl, obj); err == nil {
		t.Fatal("expected get error")
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
