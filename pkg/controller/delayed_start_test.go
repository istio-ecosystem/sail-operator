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
	"slices"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache/informertest"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type fakeSource struct {
	source.Source
	watchErr error
}

type fakeController struct {
	started   bool
	watchedMu sync.Mutex
	watched   []source.Source
}

func newFakeController() *fakeController {
	return &fakeController{}
}

func (f *fakeController) Reconcile(context.Context, reconcile.Request) (reconcile.Result, error) {
	return reconcile.Result{}, nil
}

func (f *fakeController) Watch(src source.Source) error {
	f.watchedMu.Lock()
	defer f.watchedMu.Unlock()
	f.watched = append(f.watched, src)
	if fake, ok := src.(*fakeSource); ok {
		return fake.watchErr
	}
	return nil
}

func (f *fakeController) wasWatched(src source.Source) bool {
	f.watchedMu.Lock()
	defer f.watchedMu.Unlock()
	return slices.Contains(f.watched, src)
}

func (f *fakeController) Start(context.Context) error {
	f.started = true
	return nil
}

func (f *fakeController) GetLogger() logr.Logger {
	return logr.Discard()
}

func TestDelayedStartController(t *testing.T) {
	blockOnRequired := func(ctx context.Context, _ cache.Cache, crdNames ...string) error {
		// Simulate blocking on required and returning immediately for optional
		for _, name := range crdNames {
			if name == "required.example.io" {
				<-ctx.Done()
			}
		}
		return ctx.Err()
	}
	blockOnOptional := func(ctx context.Context, _ cache.Cache, crdNames ...string) error {
		// Simulate blocking on optional and returning immediately for required
		for _, name := range crdNames {
			if name == "optional.example.io" {
				<-ctx.Done()
			}
		}
		return ctx.Err()
	}
	requiredWaitErr := errors.New("required CRD wait failed")
	requiredWatchErr := errors.New("required source watch failed")
	optionalWatchErr := errors.New("optional source watch failed")
	testCases := map[string]struct {
		expectStart         bool
		expectRequiredWatch bool
		expectOptionalWatch bool
		expectedError       error
		requiredWatchError  error
		optionalWatchError  error
		waitFunc            func(context.Context, cache.Cache, ...string) error
	}{
		"CRDs exist. Should start controller": {
			expectStart:         true,
			expectRequiredWatch: true,
			expectOptionalWatch: true,
			waitFunc:            func(context.Context, cache.Cache, ...string) error { return nil },
		},
		"Required CRDs don't exist. Controller never calls Start": {
			expectStart:         false,
			expectRequiredWatch: false,
			expectOptionalWatch: true,
			expectedError:       context.Canceled,
			waitFunc:            blockOnRequired,
		},
		"Waiting for required CRDs fails. Controller never calls Start": {
			expectStart:         false,
			expectRequiredWatch: false,
			expectOptionalWatch: true,
			expectedError:       requiredWaitErr,
			waitFunc: func(_ context.Context, _ cache.Cache, crdNames ...string) error {
				if slices.Contains(crdNames, "required.example.io") {
					return requiredWaitErr
				}
				return nil
			},
		},
		"Optional CRDs don't exist. Controller is started but optional is never watched": {
			expectStart:         true,
			expectRequiredWatch: true,
			expectOptionalWatch: false,
			waitFunc:            blockOnOptional,
		},
		"Watching the required source fails. Controller never calls Start": {
			expectStart:         false,
			expectRequiredWatch: true,
			expectOptionalWatch: true,
			expectedError:       requiredWatchErr,
			requiredWatchError:  requiredWatchErr,
			waitFunc:            func(context.Context, cache.Cache, ...string) error { return nil },
		},
		"Watching the optional source fails. Controller is still started": {
			expectStart:         true,
			expectRequiredWatch: true,
			expectOptionalWatch: true,
			optionalWatchError:  optionalWatchErr,
			waitFunc:            func(context.Context, cache.Cache, ...string) error { return nil },
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()

				fakeController := newFakeController()
				controller := newDelayedStartController(fakeController, &informertest.FakeInformers{})
				var requiredSource source.Source = &fakeSource{watchErr: tc.requiredWatchError}
				controller.DelayedWatch("required.example.io", requiredSource)
				var optionalSource source.Source = &fakeSource{watchErr: tc.optionalWatchError}
				controller.OptionalDelayedWatch("optional.example.io", optionalSource)
				controller.waitForCRDs = tc.waitFunc

				var startErr error
				go func() {
					startErr = controller.Start(ctx)
				}()
				synctest.Wait()

				if watched := fakeController.wasWatched(requiredSource); watched != tc.expectRequiredWatch {
					t.Errorf("required source watched = %t; want %t", watched, tc.expectRequiredWatch)
				}
				if watched := fakeController.wasWatched(optionalSource); watched != tc.expectOptionalWatch {
					t.Errorf("optional source watched = %t; want %t", watched, tc.expectOptionalWatch)
				}
				if fakeController.started != tc.expectStart {
					t.Fatalf("underlying controller started = %t; want %t", fakeController.started, tc.expectStart)
				}

				cancel()
				synctest.Wait()
				if !errors.Is(startErr, tc.expectedError) {
					t.Fatalf("Start() error = %v; want %v", startErr, tc.expectedError)
				}
			})
		})
	}
}

type nonLeaderController struct {
	*fakeController
}

func (*nonLeaderController) NeedLeaderElection() bool {
	return false
}

func TestDelayedStartControllerPreservesLeaderElectionPreference(t *testing.T) {
	defaultController := newDelayedStartController(newFakeController(), nil)
	if !defaultController.NeedLeaderElection() {
		t.Error("controllers without an explicit preference should require leader election")
	}

	controller := newDelayedStartController(&nonLeaderController{fakeController: newFakeController()}, nil)
	if controller.NeedLeaderElection() {
		t.Error("wrapped controller's leader-election preference was not preserved")
	}
}
