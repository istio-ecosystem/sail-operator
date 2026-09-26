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

package kube

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	collectorCRDName = "collectors.example.com"
	telemetryCRDName = "telemetries.example.com"
)

func testCRDs() []*apiextensionsv1.CustomResourceDefinition {
	return []*apiextensionsv1.CustomResourceDefinition{
		{
			ObjectMeta: metav1.ObjectMeta{Name: collectorCRDName},
			Spec: apiextensionsv1.CustomResourceDefinitionSpec{
				Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{Name: "v1beta1", Served: true}},
			},
			Status: apiextensionsv1.CustomResourceDefinitionStatus{
				Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{{
					Type: apiextensionsv1.Established, Status: apiextensionsv1.ConditionTrue,
				}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: telemetryCRDName},
			Spec: apiextensionsv1.CustomResourceDefinitionSpec{
				Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{Name: "v1", Served: true}},
			},
			Status: apiextensionsv1.CustomResourceDefinitionStatus{
				Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{{
					Type: apiextensionsv1.Established, Status: apiextensionsv1.ConditionTrue,
				}},
			},
		},
	}
}

func TestCRDsReady(t *testing.T) {
	for name, tc := range map[string]struct {
		existing []*apiextensionsv1.CustomResourceDefinition
		want     bool
	}{
		"all present - ready":     {existing: testCRDs(), want: true},
		"all missing - not ready": {},
		"one missing - not ready": {existing: testCRDs()[:1]},
		"not established": {
			existing: func() []*apiextensionsv1.CustomResourceDefinition {
				crds := testCRDs()
				crds[0].Status.Conditions = nil
				return crds
			}(),
		},
		"not served": {
			existing: func() []*apiextensionsv1.CustomResourceDefinition {
				crds := testCRDs()
				crds[0].Spec.Versions[0].Served = false
				return crds
			}(),
		},
		"any served version": {
			existing: func() []*apiextensionsv1.CustomResourceDefinition {
				crds := testCRDs()
				crds[0].Spec.Versions[0].Name = "v99"
				return crds
			}(),
			want: true,
		},
		"terminating - not ready": {
			existing: func() []*apiextensionsv1.CustomResourceDefinition {
				crds := testCRDs()
				crds[0].SetFinalizers([]string{"test"})
				crds[0].SetDeletionTimestamp(new(metav1.Now()))
				return crds
			}(),
		},
	} {
		t.Run(name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(scheme.Scheme)
			for _, crd := range tc.existing {
				builder.WithObjects(crd)
			}
			ready, err := CRDsReady(t.Context(), builder.Build(), collectorCRDName, telemetryCRDName)
			if err != nil || ready != tc.want {
				t.Fatalf("ready = %v, err = %v; want %v", ready, err, tc.want)
			}
		})
	}
}

func TestWaitForCRDsEmpty(t *testing.T) {
	if err := WaitForCRDs(t.Context(), nil); err != nil {
		t.Fatal(err)
	}
}

func TestWaitForCRDsCustomNames(t *testing.T) {
	obj := testCRDs()[0]
	obj.SetName("examples.example.com")
	cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(obj).Build()
	informer := &crdTestInformer{handlers: make(chan toolscache.ResourceEventHandler, 1)}
	crdCache := &crdTestCache{Reader: cl, informer: informer}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	if err := WaitForCRDs(ctx, crdCache, obj.GetName()); err != nil {
		t.Fatal(err)
	}
	if !informer.removed {
		t.Error("handler was not removed")
	}
}

func TestWaitForCRDs(t *testing.T) {
	readyCRDs := testCRDs()

	for name, tc := range map[string]struct {
		initial         []*apiextensionsv1.CustomResourceDefinition
		addedDuringWait []*apiextensionsv1.CustomResourceDefinition
		names           []string
		wantReady       bool
	}{
		"CRDs already ready": {
			initial: readyCRDs, names: []string{collectorCRDName, telemetryCRDName}, wantReady: true,
		},
		"CRDs never ready": {
			names: []string{collectorCRDName, telemetryCRDName},
		},
		"missing CRD is added": {
			initial:         readyCRDs[:1],
			names:           []string{collectorCRDName, telemetryCRDName},
			addedDuringWait: readyCRDs[1:],
			wantReady:       true,
		},
		"waits for all missing CRDs": {
			names:           []string{collectorCRDName, telemetryCRDName},
			addedDuringWait: readyCRDs,
			wantReady:       true,
		},
		"unrelated CRD event does not complete wait": {
			addedDuringWait: []*apiextensionsv1.CustomResourceDefinition{{
				Name: "unrelated.example.com",
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{Name: "v1beta1", Served: true}},
				},
				Status: apiextensionsv1.CustomResourceDefinitionStatus{
					Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{{
						Type: apiextensionsv1.Established, Status: apiextensionsv1.ConditionTrue,
					}},
				},
			}},
			names: []string{collectorCRDName},
		},
	} {
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				builder := fake.NewClientBuilder().WithScheme(scheme.Scheme).
					WithStatusSubresource(&apiextensionsv1.CustomResourceDefinition{})
				for _, crd := range tc.initial {
					builder.WithObjects(crd.DeepCopy())
				}
				cl := builder.Build()
				informer := &crdTestInformer{handlers: make(chan toolscache.ResourceEventHandler, 1)}
				crdCache := &crdTestCache{Reader: cl, informer: informer}
				var (
					err      error
					returned bool
				)
				go func() {
					err = WaitForCRDs(ctx, crdCache, tc.names...)
					returned = true
				}()
				// Wait until the initial cache check finishes or WaitForCRDs blocks.
				// In cases with events below, assert it has NOT returned before adding CRDs.
				synctest.Wait()
				if !informer.added {
					t.Fatal("handler was not added to CRD informer")
				}
				handler := <-informer.handlers
				check := func(wantReturned bool) {
					t.Helper()
					if returned != wantReturned {
						t.Fatalf("WaitForCRDs returned = %t; want %t", returned, wantReturned)
					}
					if returned && err != nil {
						t.Errorf("WaitForCRDs error = %v; want nil", err)
					}
					if informer.removed != wantReturned {
						t.Errorf("handler removed = %t; want %t", informer.removed, wantReturned)
					}
				}
				check(tc.wantReady && len(tc.addedDuringWait) == 0)

				for i, crd := range tc.addedDuringWait {
					if err := cl.Create(ctx, crd.DeepCopy()); err != nil {
						t.Fatal(err)
					}
					handler.OnAdd(crd, false)
					synctest.Wait()
					check(tc.wantReady && i == len(tc.addedDuringWait)-1)
				}

				cancel()
				synctest.Wait()
				if !tc.wantReady && !errors.Is(err, context.Canceled) {
					t.Errorf("WaitForCRDs error = %v; want context.Canceled", err)
				}
				if !informer.removed {
					t.Error("handler was not removed after cancellation")
				}
			})
		})
	}
}

type crdTestInformer struct {
	cache.Informer
	handlers chan toolscache.ResourceEventHandler
	added    bool
	removed  bool
}

func (i *crdTestInformer) AddEventHandler(handler toolscache.ResourceEventHandler) (toolscache.ResourceEventHandlerRegistration, error) {
	i.handlers <- handler
	i.added = true
	return nil, nil
}

func (i *crdTestInformer) RemoveEventHandler(toolscache.ResourceEventHandlerRegistration) error {
	i.removed = true
	return nil
}

type crdTestCache struct {
	cache.Cache
	client.Reader
	informer *crdTestInformer
}

func (c *crdTestCache) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return c.Reader.Get(ctx, key, obj, opts...)
}

func (c *crdTestCache) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return c.Reader.List(ctx, list, opts...)
}

func (c *crdTestCache) GetInformer(context.Context, client.Object, ...cache.InformerGetOption) (cache.Informer, error) {
	return c.informer, nil
}
