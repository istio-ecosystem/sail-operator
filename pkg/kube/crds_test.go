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
	"time"

	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

const (
	testCRDNameA = "examples.example.com"
	testCRDNameB = "widgets.example.com"
)

func testCRDs() []*apiextensionsv1.CustomResourceDefinition {
	return []*apiextensionsv1.CustomResourceDefinition{
		{
			ObjectMeta: metav1.ObjectMeta{Name: testCRDNameA},
			Spec: apiextensionsv1.CustomResourceDefinitionSpec{
				Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{Name: "v1", Served: true}},
			},
			Status: apiextensionsv1.CustomResourceDefinitionStatus{
				Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{{
					Type: apiextensionsv1.Established, Status: apiextensionsv1.ConditionTrue,
				}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: testCRDNameB},
			Spec: apiextensionsv1.CustomResourceDefinitionSpec{
				Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{Name: "v1alpha1", Served: true}},
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
			ready, err := CRDsReady(t.Context(), builder.Build(), testCRDNameA, testCRDNameB)
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

func TestCRDsReadyGetError(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithInterceptorFuncs(interceptor.Funcs{
		Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
			return apierrors.NewInternalError(errors.New("boom"))
		},
	}).Build()
	ready, err := CRDsReady(t.Context(), cl, testCRDNameA)
	if err == nil || ready {
		t.Fatalf("ready = %v, err = %v; want error", ready, err)
	}
}

func TestWaitForCRDsGetInformerError(t *testing.T) {
	crdCache := &crdTestCache{getInformerErr: errors.New("informer unavailable")}
	if err := WaitForCRDs(t.Context(), crdCache, testCRDNameA); err == nil {
		t.Fatal("expected GetInformer error")
	}
}

func TestWaitForCRDsAddHandlerError(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
	informer := &crdTestInformer{addErr: errors.New("add failed")}
	crdCache := &crdTestCache{Reader: cl, informer: informer}
	if err := WaitForCRDs(t.Context(), crdCache, testCRDNameA); err == nil {
		t.Fatal("expected AddEventHandler error")
	}
}

func TestWaitForCRDsContextCancel(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
	informer := &crdTestInformer{handlers: make(chan toolscache.ResourceEventHandler, 1)}
	crdCache := &crdTestCache{Reader: cl, informer: informer}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := WaitForCRDs(ctx, crdCache, testCRDNameA); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestWaitForCRDsWaitsForEvent(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
	informer := &crdTestInformer{
		handlers:  make(chan toolscache.ResourceEventHandler, 1),
		removeErr: errors.New("remove failed"), // exercise RemoveEventHandler error log
	}
	crdCache := &crdTestCache{Reader: cl, informer: informer}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- WaitForCRDs(ctx, crdCache, testCRDNameA) }()

	handler := <-informer.handlers
	// Ignored notifications: wrong type, unwanted CRD, tombstone with unwanted object.
	handler.OnAdd("not-a-crd", false)
	handler.OnAdd(&apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: "other.example.com"},
	}, false)
	handler.OnDelete(toolscache.DeletedFinalStateUnknown{
		Obj: &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: "other.example.com"},
		},
	})

	ready := testCRDs()[0]
	if err := cl.Create(ctx, ready.DeepCopy()); err != nil {
		t.Fatal(err)
	}
	// Coalesce: multiple notifies before WaitForCRDs re-checks.
	handler.OnAdd(ready, false)
	handler.OnUpdate(nil, ready)
	handler.OnAdd(ready, false)

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("WaitForCRDs did not return after CRD became ready")
	}
}

func TestWaitForCRDsReadyCheckError(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithInterceptorFuncs(interceptor.Funcs{
		Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
			return apierrors.NewForbidden(schema.GroupResource{Resource: "customresourcedefinitions"}, testCRDNameA, errors.New("denied"))
		},
	}).Build()
	informer := &crdTestInformer{handlers: make(chan toolscache.ResourceEventHandler, 1)}
	crdCache := &crdTestCache{Reader: cl, informer: informer}
	if err := WaitForCRDs(t.Context(), crdCache, testCRDNameA); err == nil {
		t.Fatal("expected CRDsReady error")
	}
}

type crdTestInformer struct {
	cache.Informer
	handlers  chan toolscache.ResourceEventHandler
	removed   bool
	addErr    error
	removeErr error
}

func (i *crdTestInformer) AddEventHandler(handler toolscache.ResourceEventHandler) (toolscache.ResourceEventHandlerRegistration, error) {
	if i.addErr != nil {
		return nil, i.addErr
	}
	if i.handlers != nil {
		i.handlers <- handler
	}
	return nil, nil
}

func (i *crdTestInformer) RemoveEventHandler(toolscache.ResourceEventHandlerRegistration) error {
	i.removed = true
	return i.removeErr
}

type crdTestCache struct {
	cache.Cache
	client.Reader
	informer       *crdTestInformer
	getInformerErr error
}

func (c *crdTestCache) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return c.Reader.Get(ctx, key, obj, opts...)
}

func (c *crdTestCache) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return c.Reader.List(ctx, list, opts...)
}

func (c *crdTestCache) GetInformer(context.Context, client.Object, ...cache.InformerGetOption) (cache.Informer, error) {
	if c.getInformerErr != nil {
		return nil, c.getInformerErr
	}
	return c.informer, nil
}
