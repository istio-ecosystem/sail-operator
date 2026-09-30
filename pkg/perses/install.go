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
	"io/fs"

	"github.com/go-logr/logr"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// waitForCRDsFunc waits until the named CRDs are ready. Overridable in tests.
type waitForCRDsFunc func(ctx context.Context, crdCache cache.Cache, crdNames ...string) error

// Installer waits for the PersesDashboard CRD, then creates dashboards from DashboardFS once in the
// operator namespace. It does not watch PersesDashboard resources and does not register them as a
// required type, so the CRD may be absent at manager startup.
type Installer struct {
	Client      client.Client
	Cache       cache.Cache
	DashboardFS fs.FS
	Namespace   string

	waitForCRDs waitForCRDsFunc
	log         logr.Logger
}

// NewInstaller builds an Installer that provisions dashboards in the operator namespace.
func NewInstaller(cfg config.ReconcilerConfig, cl client.Client, crdCache cache.Cache, dashboardFS fs.FS) *Installer {
	return &Installer{
		Client:      cl,
		Cache:       crdCache,
		DashboardFS: dashboardFS,
		Namespace:   cfg.OperatorNamespace,
		waitForCRDs: kube.WaitForCRDs,
	}
}

// NeedLeaderElection ensures only the leader installs dashboards.
func (i *Installer) NeedLeaderElection() bool { return true }

// Start waits for the PersesDashboard CRD, creates any missing dashboards once, then returns.
// Missing CRDs do not fail the operator; Start blocks until the CRD is ready or the context is cancelled.
func (i *Installer) Start(ctx context.Context) error {
	log := i.log
	if log.GetSink() == nil {
		log = ctrl.Log.WithName("perses")
	}
	wait := i.waitForCRDs
	if wait == nil {
		wait = kube.WaitForCRDs
	}

	log.Info("Waiting for PersesDashboard CRD")
	if err := wait(ctx, i.Cache, PersesDashboardCRD); err != nil {
		return err
	}
	log.Info("PersesDashboard CRD is ready; installing dashboards", "namespace", i.Namespace)

	if err := reconcileDashboards(ctrl.LoggerInto(ctx, log), i.Client, i.DashboardFS, i.Namespace); err != nil {
		// Do not take down the operator for optional dashboard provisioning failures.
		log.Error(err, "Failed to install Perses dashboards")
		return nil
	}
	return nil
}

// SetupWithManager registers the installer as a leader-elected Runnable.
//
// +kubebuilder:rbac:groups=perses.dev,resources=persesdashboards,verbs=get;list;create
func (i *Installer) SetupWithManager(mgr ctrl.Manager) error {
	i.log = mgr.GetLogger().WithName("perses")
	if i.Cache == nil {
		i.Cache = mgr.GetCache()
	}
	if i.Client == nil {
		i.Client = mgr.GetClient()
	}
	return mgr.Add(manager.Runnable(i))
}
