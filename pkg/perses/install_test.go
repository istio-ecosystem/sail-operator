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
	"sync"
	"testing"
	"time"

	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	"github.com/istio-ecosystem/sail-operator/pkg/test/project"
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
	fsys := os.DirFS(path.Join(project.RootDir, "pkg", "perses", "resources"))
	want := countDashboardYAMLs(t, fsys)

	ready := make(chan struct{})
	var once sync.Once
	installer := &Installer{
		Client:      cl,
		DashboardFS: fsys,
		Namespace:   namespace,
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
	assertDashboardCount(t, cl, 0)

	once.Do(func() { close(ready) })

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after CRD became ready")
	}

	assertDashboardCount(t, cl, want)
}

func TestStartIdempotentCreation(t *testing.T) {
	namespace := "sail-operator"
	cl := newPersesTestClient(t, testPersesDashboardCRD())
	fsys := os.DirFS(path.Join(project.RootDir, "pkg", "perses", "resources"))
	want := countDashboardYAMLs(t, fsys)

	installer := &Installer{
		Client:      cl,
		DashboardFS: fsys,
		Namespace:   namespace,
		waitForCRDs: func(context.Context, cache.Cache, ...string) error { return nil },
	}

	if err := installer.Start(context.Background()); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	assertDashboardCount(t, cl, want)

	if err := installer.Start(context.Background()); err != nil {
		t.Fatalf("second Start: %v", err)
	}
	assertDashboardCount(t, cl, want)
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

func TestStartInstallErrorDoesNotFailOperator(t *testing.T) {
	installer := &Installer{
		Client:      fake.NewClientBuilder().WithScheme(scheme.Scheme).Build(),
		DashboardFS: os.DirFS(t.TempDir()), // missing dashboards → install error path
		Namespace:   "sail-operator",
		waitForCRDs: func(context.Context, cache.Cache, ...string) error { return nil },
	}
	if err := installer.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v, want nil (install failures must not take down the operator)", err)
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

func assertDashboardCount(t *testing.T, cl client.Client, want int) {
	t.Helper()
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{Group: "perses.dev", Version: "v1alpha2", Kind: "PersesDashboardList"})
	if err := cl.List(context.Background(), list, client.InNamespace("sail-operator")); err != nil {
		t.Fatalf("list dashboards: %v", err)
	}
	if got := len(list.Items); got != want {
		t.Fatalf("dashboard count = %d, want %d", got, want)
	}
}
