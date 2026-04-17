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

package install

import (
	"context"
	"fmt"

	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	sharedreconcile "github.com/istio-ecosystem/sail-operator/pkg/reconcile"
	"github.com/istio-ecosystem/sail-operator/pkg/revision"
	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	"github.com/istio-ecosystem/sail-operator/pkg/watches"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	ctrlreconcile "sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"istio.io/istio/pkg/log"
	"istio.io/istio/pkg/ptr"
)

type libraryReconciler struct {
	lib *Library
}

func (r *libraryReconciler) Reconcile(ctx context.Context, _ ctrlreconcile.Request) (ctrlreconcile.Result, error) {
	r.lib.mu.Lock()
	opts := r.lib.desiredOpts
	gen := r.lib.generation
	r.lib.mu.Unlock()

	if opts == nil {
		return ctrlreconcile.Result{}, nil
	}

	inst := r.lib.newInstaller(opts.Namespace)
	status := inst.reconcile(ctx, *opts)
	status.Generation = gen

	r.lib.statusMu.Lock()
	r.lib.currentStatus = status
	r.lib.statusMu.Unlock()

	select {
	case r.lib.notifyCh <- struct{}{}:
	default:
	}

	if status.Error != nil {
		log.Errorf("reconciliation failed: %v", status.Error)
		return ctrlreconcile.Result{}, status.Error
	}
	return ctrlreconcile.Result{}, nil
}

func (l *Library) setupController(mgr ctrl.Manager) error {
	fixedKeyHandler := handler.EnqueueRequestsFromMapFunc(
		func(_ context.Context, _ client.Object) []ctrlreconcile.Request {
			return []ctrlreconcile.Request{{NamespacedName: types.NamespacedName{Name: "sail-library"}}}
		},
	)

	managedByPred, err := predicate.LabelSelectorPredicate(metav1.LabelSelector{
		MatchLabels: map[string]string{"app.kubernetes.io/managed-by": managedByValue},
	})
	if err != nil {
		return fmt.Errorf("failed to create label predicate: %w", err)
	}

	b := ctrl.NewControllerManagedBy(mgr).
		Named("sail-library").
		WithOptions(controller.Options{SkipNameValidation: ptr.Of(true)}).
		WatchesRawSource(source.Channel(l.triggerCh, fixedKeyHandler))

	watches.RegisterOwnedWatches(b, watches.IstiodWatches, fixedKeyHandler, nil, managedByPred)
	b.Watches(&apiextensionsv1.CustomResourceDefinition{}, fixedKeyHandler)

	return b.Complete(&libraryReconciler{lib: l})
}

// Start begins the reconciliation loop and drift-detection watches.
// The returned channel receives a notification each time a reconciliation
// completes. The loop runs until the context is cancelled or Stop is called.
func (l *Library) Start(ctx context.Context) (<-chan struct{}, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	mgr, err := ctrl.NewManager(l.kubeConfig, ctrl.Options{
		Scheme:                 scheme.Scheme,
		Metrics:                metricsserver.Options{BindAddress: "0"},
		HealthProbeBindAddress: "",
		LeaderElection:         false,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create manager: %w", err)
	}

	l.cl = mgr.GetClient()

	if err := l.setupController(mgr); err != nil {
		return nil, fmt.Errorf("failed to setup controller: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	l.cancel = cancel
	l.done = make(chan struct{})

	go func() {
		defer close(l.done)
		if err := mgr.Start(ctx); err != nil {
			log.Errorf("manager exited with error: %v", err)
		}
	}()

	return l.notifyCh, nil
}

func (l *Library) sendTrigger() {
	select {
	case l.triggerCh <- event.GenericEvent{
		Object: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "sail-library"}},
	}:
	default:
	}
}

func (l *Library) newInstaller(namespace string) *installer {
	cfg := sharedreconcile.Config{
		ResourceFS:        l.resourceFS,
		ChartManager:      l.chartManager,
		OperatorNamespace: namespace,
	}
	return &installer{
		istiodReconciler: sharedreconcile.NewIstiodReconciler(cfg, l.cl),
		crdManager:       &crdManager{cl: l.cl, crdFS: l.crdFS},
		cfg:              cfg,
		platform:         l.platform,
		tlsConfig:        l.tlsConfig,
	}
}

type installer struct {
	istiodReconciler *sharedreconcile.IstiodReconciler
	crdManager       *crdManager
	cfg              sharedreconcile.Config
	platform         config.Platform
	tlsConfig        *config.TLSConfig
}

func (inst *installer) reconcile(ctx context.Context, opts Options) Status {
	status := Status{Version: opts.Version}

	if err := istioversion.ValidateVersion(opts.Version); err != nil {
		status.Error = err
		return status
	}

	resolvedVersion, err := istioversion.Resolve(opts.Version)
	if err != nil {
		status.Error = fmt.Errorf("failed to resolve version: %w", err)
		return status
	}

	revisionName := opts.Revision
	if revisionName == "" {
		revisionName = "default"
	}

	defaultProfile := ""
	if inst.platform == config.PlatformOpenShift {
		defaultProfile = "openshift"
	}

	values, err := revision.ComputeValues(
		opts.Values,
		opts.Namespace,
		resolvedVersion,
		inst.platform,
		defaultProfile,
		"",
		inst.cfg.ResourceFS,
		revisionName,
		inst.tlsConfig,
	)
	if err != nil {
		status.Error = fmt.Errorf("failed to compute values: %w", err)
		return status
	}

	if opts.ManageCRDs {
		crdInfos, err := inst.crdManager.Reconcile(ctx, opts)
		if err != nil {
			status.Error = fmt.Errorf("failed to reconcile CRDs: %w", err)
			status.CRDState = CRDManagementStateError
			status.CRDMessage = err.Error()
			status.CRDs = crdInfos
			return status
		}
		status.CRDs = crdInfos
		status.CRDState, status.CRDMessage = AggregateState(crdInfos)
	}

	if err := inst.istiodReconciler.Validate(ctx, resolvedVersion, opts.Namespace, values); err != nil {
		status.Error = err
		return status
	}

	if err := inst.istiodReconciler.Install(ctx, resolvedVersion, opts.Namespace, values, revisionName, nil); err != nil {
		status.Error = fmt.Errorf("failed to install istiod: %w", err)
		return status
	}

	status.Installed = true
	return status
}

func (inst *installer) uninstall(ctx context.Context, namespace, revisionName string) error {
	return inst.istiodReconciler.Uninstall(ctx, namespace, revisionName)
}
