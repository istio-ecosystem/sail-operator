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

package persesdashboard

import (
	"context"
	"os"
	"path"
	"sync"
	"testing"
	"time"

	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/perses"
	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	"github.com/istio-ecosystem/sail-operator/pkg/test/project"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

func TestStartWaitsWithoutCRDThenInstalls(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	namespace := "sail-operator"
	cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
	fsys := os.DirFS(path.Join(project.RootDir, "resources", "perses"))

	ready := make(chan struct{})
	var once sync.Once
	installer := &Installer{
		Client:      cl,
		DashboardFS: fsys,
		Namespace:   namespace,
		Interval:    20 * time.Millisecond,
		waitForCRDs: func(ctx context.Context, _ cache.Cache, _ ...string) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ready:
				return nil
			}
		},
	}

	done := make(chan error, 1)
	go func() { done <- installer.Start(ctx) }()

	// While CRD is unavailable, no dashboards should exist.
	time.Sleep(50 * time.Millisecond)
	assertDashboardCount(t, cl, namespace, 0)

	once.Do(func() { close(ready) })

	Eventually(t, 2*time.Second, func() bool {
		return dashboardCount(t, cl, namespace) == len(perses.ProductDashboards)
	})

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after cancel")
	}
}

func TestStartIdempotentCreation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	namespace := "sail-operator"
	cl := newPersesTestClient(t, testPersesDashboardCRD())
	fsys := os.DirFS(path.Join(project.RootDir, "resources", "perses"))

	installer := &Installer{
		Client:      cl,
		DashboardFS: fsys,
		Namespace:   namespace,
		Interval:    20 * time.Millisecond,
		waitForCRDs: func(context.Context, cache.Cache, ...string) error { return nil },
	}

	done := make(chan error, 1)
	go func() { done <- installer.Start(ctx) }()

	Eventually(t, 2*time.Second, func() bool {
		return dashboardCount(t, cl, namespace) == len(perses.ProductDashboards)
	})
	// Second reconcile cycle should preserve the same set.
	time.Sleep(50 * time.Millisecond)
	assertDashboardCount(t, cl, namespace, len(perses.ProductDashboards))

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after cancel")
	}
}

func TestNewInstallerUsesOperatorNamespace(t *testing.T) {
	i := NewInstaller(config.ReconcilerConfig{OperatorNamespace: "my-operator"}, nil, nil, nil)
	if i.Namespace != "my-operator" {
		t.Fatalf("Namespace = %q, want my-operator", i.Namespace)
	}
}

func TestNeedLeaderElection(t *testing.T) {
	if !(&Installer{}).NeedLeaderElection() {
		t.Fatal("NeedLeaderElection() = false, want true")
	}
}

func TestStartWaitError(t *testing.T) {
	installer := &Installer{
		waitForCRDs: func(context.Context, cache.Cache, ...string) error {
			return context.DeadlineExceeded
		},
	}
	if err := installer.Start(context.Background()); err == nil {
		t.Fatal("expected wait error")
	}
}

func TestStartReconcileErrorUsesDefaultInterval(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	installer := &Installer{
		Client:      fake.NewClientBuilder().WithScheme(scheme.Scheme).Build(),
		DashboardFS: os.DirFS(t.TempDir()), // missing dashboards → reconcile error path
		Namespace:   "sail-operator",
		Interval:    0, // exercise default interval
		waitForCRDs: func(context.Context, cache.Cache, ...string) error { return nil },
	}

	done := make(chan error, 1)
	go func() { done <- installer.Start(ctx) }()
	time.Sleep(30 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after cancel")
	}
}

func TestSetupWithManager(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
	mgr, err := ctrl.NewManager(&rest.Config{Host: "https://127.0.0.1:1"}, ctrl.Options{
		Scheme:                 scheme.Scheme,
		Metrics:                metricsserver.Options{BindAddress: "0"},
		HealthProbeBindAddress: "0",
		NewClient: func(*rest.Config, client.Options) (client.Client, error) {
			return cl, nil
		},
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	installer := &Installer{} // nil Client/Cache → filled from manager
	if err := installer.SetupWithManager(mgr); err != nil {
		t.Fatalf("SetupWithManager: %v", err)
	}
	if installer.Client == nil || installer.Cache == nil {
		t.Fatal("expected Client and Cache to be set from manager")
	}
	if installer.log.GetSink() == nil {
		t.Fatal("expected logger to be set")
	}
}

func Eventually(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

func dashboardCount(t *testing.T, cl client.Client, namespace string) int {
	t.Helper()
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{Group: "perses.dev", Version: "v1alpha2", Kind: "PersesDashboardList"})
	if err := cl.List(context.Background(), list, client.InNamespace(namespace)); err != nil {
		t.Fatalf("list dashboards: %v", err)
	}
	return len(list.Items)
}

func assertDashboardCount(t *testing.T, cl client.Client, namespace string, want int) {
	t.Helper()
	if got := dashboardCount(t, cl, namespace); got != want {
		t.Fatalf("dashboard count = %d, want %d", got, want)
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
		ObjectMeta: metav1.ObjectMeta{Name: perses.PersesDashboardCRD},
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
		Status: apiextensionsv1.CustomResourceDefinitionStatus{
			Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{{
				Type: apiextensionsv1.Established, Status: apiextensionsv1.ConditionTrue,
			}},
		},
	}
}
