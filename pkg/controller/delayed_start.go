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

package controller

import (
	"context"
	"errors"
	"maps"
	"slices"

	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// NewDelayedStartController wraps a controller so its sources can wait for their CRDs before starting.
func NewDelayedStartController(name string, options controller.Options, crdCache cache.Cache) (*DelayedStartController, error) {
	c, err := controller.NewUnmanaged(name, options)
	if err != nil {
		return nil, err
	}
	return newDelayedStartController(c, crdCache), nil
}

func newDelayedStartController(wrappedController controller.Controller, crdCache cache.Cache) *DelayedStartController {
	return &DelayedStartController{
		requiredSources: map[string]source.Source{},
		optionalSources: map[string]source.Source{},
		Controller:      wrappedController,
		cache:           crdCache,
		waitForCRDs:     kube.WaitForCRDs,
	}
}

// DelayedStartController waits until its required CRDs are ready before starting and adds
// optional watches asynchronously when their CRDs become available.
type DelayedStartController struct {
	cache           cache.Cache
	requiredSources map[string]source.Source
	optionalSources map[string]source.Source
	waitForCRDs     func(context.Context, cache.Cache, ...string) error
	controller.Controller
}

// DelayedWatch adds a source whose CRD must be ready before the wrapped controller starts.
// If the CRD is not ready, the controller won't be started until the CRD is ready.
func (d *DelayedStartController) DelayedWatch(crdName string, src source.Source) {
	d.requiredSources[crdName] = src
}

// OptionalDelayedWatch adds a source asynchronously when its CRD becomes ready.
func (d *DelayedStartController) OptionalDelayedWatch(crdName string, src source.Source) {
	d.optionalSources[crdName] = src
}

// NeedLeaderElection ensures that the wrapper is started in the manager's leader-election group.
// Delegate to the wrapped controller when it provides an explicit preference.
func (d *DelayedStartController) NeedLeaderElection() bool {
	if runnable, ok := d.Controller.(manager.LeaderElectionRunnable); ok {
		return runnable.NeedLeaderElection()
	}
	return true
}

// Start will wait for all CRDs added through DelayedWatch to be present before
// starting the actual controller or the Watch for the source. Any CRDs added through
// OptionalDelayedWatch will have their Watch added either before or after the controller
// is started but the controller will not be blocked on these.
func (d *DelayedStartController) Start(ctx context.Context) error {
	// Fire go routines to Watch all the optionalSources once
	// their CRDs are ready. These won't block the Start of the Controller.
	// Controller.Watch is safe to call both before and after Start: the
	// source is either queued until Start or started right away.
	for crdName, src := range d.optionalSources {
		go func(ctx context.Context, crdName string, src source.Source) {
			log := d.GetLogger().WithValues("crd", crdName)
			log.Info("Checking if CRD exists. If it does not then watch will begin after it is present.")
			if err := d.waitForCRDs(ctx, d.cache, crdName); err != nil {
				if !errors.Is(err, context.Canceled) {
					log.Error(err, "Unable to wait for optional CRD; its watch was not added")
				}
				return
			}
			if err := d.Watch(src); err != nil {
				log.Error(err, "Unable to add watch for optional CRD")
				return
			}
			log.Info("CRD is ready. Watch added.")
		}(ctx, crdName, src)
	}

	requiredCRDNames := slices.Collect(maps.Keys(d.requiredSources))
	d.GetLogger().Info("Waiting for CRDs to be present to start controller", "crds", requiredCRDNames)
	if err := d.waitForCRDs(ctx, d.cache, requiredCRDNames...); err != nil {
		return err
	}
	d.GetLogger().Info("CRDs present. Starting controller.", "crds", requiredCRDNames)

	for _, src := range d.requiredSources {
		if err := d.Watch(src); err != nil {
			return err
		}
	}

	return d.Controller.Start(ctx)
}
