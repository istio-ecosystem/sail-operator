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
	"sync"
	"testing"
	"time"

	"github.com/istio-ecosystem/sail-operator/pkg/helm"
	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	"helm.sh/helm/v4/pkg/release"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

func TestStartWaitsWithoutCRDThenInstalls(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	namespace := "sail-operator"
	ready := make(chan struct{})
	var once sync.Once
	helmCalls := 0
	installer := &Installer{
		ChartManager: &mockChartReconciler{
			upgradeOrInstall: func(context.Context, fs.FS, string, helm.Values, string, string, *metav1.OwnerReference) (release.Releaser, error) {
				helmCalls++
				return nil, nil
			},
		},
		Namespace: namespace,
		waitForCRDs: func(ctx context.Context, _ cache.Cache, _ ...schema.GroupVersionResource) error {
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

	time.Sleep(50 * time.Millisecond)
	if helmCalls != 0 {
		t.Fatalf("helm calls before CRD ready = %d, want 0", helmCalls)
	}

	once.Do(func() { close(ready) })

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after CRD became ready")
	}

	if helmCalls != 1 {
		t.Fatalf("helm calls after install = %d, want 1", helmCalls)
	}
}

func TestStartIdempotentHelmUpgrade(t *testing.T) {
	namespace := "sail-operator"
	helmCalls := 0
	installer := &Installer{
		ChartManager: &mockChartReconciler{
			upgradeOrInstall: func(context.Context, fs.FS, string, helm.Values, string, string, *metav1.OwnerReference) (release.Releaser, error) {
				helmCalls++
				return nil, nil
			},
		},
		Namespace:   namespace,
		waitForCRDs: func(context.Context, cache.Cache, ...schema.GroupVersionResource) error { return nil },
	}

	if err := installer.Start(context.Background()); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	if err := installer.Start(context.Background()); err != nil {
		t.Fatalf("second Start: %v", err)
	}
	if helmCalls != 2 {
		t.Fatalf("helm calls = %d, want 2", helmCalls)
	}
}

func TestNewInstallerUsesOperatorNamespace(t *testing.T) {
	i := NewInstaller("my-operator", nil, nil)
	if i.Namespace != "my-operator" {
		t.Fatalf("Namespace = %q, want my-operator", i.Namespace)
	}
	if i.ChartFS == nil {
		t.Fatal("expected embedded chart FS")
	}
}

func TestNeedLeaderElection(t *testing.T) {
	if !(&Installer{}).NeedLeaderElection() {
		t.Fatal("NeedLeaderElection() = false, want true")
	}
}

func TestStartWaitError(t *testing.T) {
	installer := &Installer{
		waitForCRDs: func(context.Context, cache.Cache, ...schema.GroupVersionResource) error {
			return context.DeadlineExceeded
		},
	}
	if err := installer.Start(context.Background()); err == nil {
		t.Fatal("expected wait error")
	}
}

func TestStartInstallErrorDoesNotFailOperator(t *testing.T) {
	installer := &Installer{
		ChartManager: &mockChartReconciler{
			upgradeOrInstall: func(context.Context, fs.FS, string, helm.Values, string, string, *metav1.OwnerReference) (release.Releaser, error) {
				return nil, fmt.Errorf("helm failed")
			},
		},
		Namespace:   "sail-operator",
		waitForCRDs: func(context.Context, cache.Cache, ...schema.GroupVersionResource) error { return nil },
	}
	if err := installer.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v, want nil (install failures must not take down the operator)", err)
	}
}

func TestSetupWithManager(t *testing.T) {
	mgr, err := ctrl.NewManager(&rest.Config{Host: "https://127.0.0.1:1"}, ctrl.Options{
		Scheme:                 scheme.Scheme,
		Metrics:                metricsserver.Options{BindAddress: "0"},
		HealthProbeBindAddress: "0",
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	installer := &Installer{ChartManager: &mockChartReconciler{}}
	if err := installer.SetupWithManager(mgr); err != nil {
		t.Fatalf("SetupWithManager: %v", err)
	}
	if installer.Cache == nil {
		t.Fatal("expected Cache to be set from manager")
	}
	if installer.log.GetSink() == nil {
		t.Fatal("expected logger to be set")
	}
}

func TestSetupWithManagerRequiresChartManager(t *testing.T) {
	mgr, err := ctrl.NewManager(&rest.Config{Host: "https://127.0.0.1:1"}, ctrl.Options{
		Scheme:                 scheme.Scheme,
		Metrics:                metricsserver.Options{BindAddress: "0"},
		HealthProbeBindAddress: "0",
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if err := (&Installer{}).SetupWithManager(mgr); err == nil {
		t.Fatal("expected error when ChartManager is nil")
	}
}
