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

package apiserver

import (
	"context"
	"testing"

	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	configv1 "github.com/openshift/api/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestReconcile(t *testing.T) {
	ctx := context.Background()

	t.Run("calls shutdown when TLS config differs from initial", func(t *testing.T) {
		shutdownCalled := false

		apiServer := &configv1.APIServer{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cluster",
			},
			Spec: configv1.APIServerSpec{
				TLSSecurityProfile: &configv1.TLSSecurityProfile{
					Type: configv1.TLSProfileModernType,
				},
			},
		}

		cl := fake.NewClientBuilder().
			WithScheme(scheme.Scheme).
			WithObjects(apiServer).
			Build()

		initialConfig := config.TLSConfigFromAPIServer(&configv1.APIServer{
			Spec: configv1.APIServerSpec{
				TLSSecurityProfile: &configv1.TLSSecurityProfile{
					Type: configv1.TLSProfileIntermediateType,
				},
			},
		})

		shutdown := func() {
			shutdownCalled = true
		}

		reconciler := NewReconciler(cl, initialConfig, shutdown)

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{Name: "cluster"},
		}

		_, err := reconciler.Reconcile(ctx, req)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if !shutdownCalled {
			t.Error("Expected shutdown to be called when TLS config differs, but it was not")
		}
	})

	t.Run("does not call shutdown when TLS config matches initial", func(t *testing.T) {
		shutdownCalled := false

		apiServer := &configv1.APIServer{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cluster",
			},
			Spec: configv1.APIServerSpec{
				TLSSecurityProfile: &configv1.TLSSecurityProfile{
					Type: configv1.TLSProfileIntermediateType,
				},
			},
		}

		cl := fake.NewClientBuilder().
			WithScheme(scheme.Scheme).
			WithObjects(apiServer).
			Build()

		initialConfig := config.TLSConfigFromAPIServer(apiServer)

		shutdown := func() {
			shutdownCalled = true
		}

		reconciler := NewReconciler(cl, initialConfig, shutdown)

		req := reconcile.Request{
			NamespacedName: types.NamespacedName{Name: "cluster"},
		}

		_, err := reconciler.Reconcile(ctx, req)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}

		if shutdownCalled {
			t.Error("Expected shutdown not to be called when TLS config matches, but it was")
		}
	})
}

func TestTLSProfileChangedPredicate(t *testing.T) {
	pred := tlsProfileChangedPredicate()

	t.Run("CreateFunc returns true", func(t *testing.T) {
		e := event.CreateEvent{
			Object: &configv1.APIServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
			},
		}
		if !pred.Create(e) {
			t.Error("Expected CreateFunc to return true")
		}
	})

	t.Run("DeleteFunc returns false", func(t *testing.T) {
		e := event.DeleteEvent{
			Object: &configv1.APIServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
			},
		}
		if pred.Delete(e) {
			t.Error("Expected DeleteFunc to return false")
		}
	})

	t.Run("GenericFunc returns false", func(t *testing.T) {
		e := event.GenericEvent{
			Object: &configv1.APIServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
			},
		}
		if pred.Generic(e) {
			t.Error("Expected GenericFunc to return false")
		}
	})

	t.Run("UpdateFunc returns true when TLS profile changes", func(t *testing.T) {
		oldAPIServer := &configv1.APIServer{
			ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
			Spec: configv1.APIServerSpec{
				TLSSecurityProfile: &configv1.TLSSecurityProfile{
					Type: configv1.TLSProfileIntermediateType,
				},
			},
		}
		newAPIServer := &configv1.APIServer{
			ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
			Spec: configv1.APIServerSpec{
				TLSSecurityProfile: &configv1.TLSSecurityProfile{
					Type: configv1.TLSProfileModernType,
				},
			},
		}

		e := event.UpdateEvent{
			ObjectOld: oldAPIServer,
			ObjectNew: newAPIServer,
		}

		if !pred.Update(e) {
			t.Error("Expected UpdateFunc to return true when TLS profile changes")
		}
	})

	t.Run("UpdateFunc returns false when TLS profile unchanged", func(t *testing.T) {
		apiServer := &configv1.APIServer{
			ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
			Spec: configv1.APIServerSpec{
				TLSSecurityProfile: &configv1.TLSSecurityProfile{
					Type: configv1.TLSProfileIntermediateType,
				},
			},
		}
		apiServerCopy := apiServer.DeepCopy()

		e := event.UpdateEvent{
			ObjectOld: apiServer,
			ObjectNew: apiServerCopy,
		}

		if pred.Update(e) {
			t.Error("Expected UpdateFunc to return false when TLS profile is unchanged")
		}
	})

	t.Run("UpdateFunc returns true when TLS profile changes from nil to set", func(t *testing.T) {
		oldAPIServer := &configv1.APIServer{
			ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
			Spec: configv1.APIServerSpec{
				TLSSecurityProfile: nil,
			},
		}
		newAPIServer := &configv1.APIServer{
			ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
			Spec: configv1.APIServerSpec{
				TLSSecurityProfile: &configv1.TLSSecurityProfile{
					Type: configv1.TLSProfileModernType,
				},
			},
		}

		e := event.UpdateEvent{
			ObjectOld: oldAPIServer,
			ObjectNew: newAPIServer,
		}

		if !pred.Update(e) {
			t.Error("Expected UpdateFunc to return true when TLS profile changes from nil to set")
		}
	})

	t.Run("UpdateFunc returns false when other fields change but TLS profile unchanged", func(t *testing.T) {
		oldAPIServer := &configv1.APIServer{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "cluster",
				ResourceVersion: "1",
			},
			Spec: configv1.APIServerSpec{
				TLSSecurityProfile: &configv1.TLSSecurityProfile{
					Type: configv1.TLSProfileIntermediateType,
				},
			},
		}
		newAPIServer := &configv1.APIServer{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "cluster",
				ResourceVersion: "2",
			},
			Spec: configv1.APIServerSpec{
				TLSSecurityProfile: &configv1.TLSSecurityProfile{
					Type: configv1.TLSProfileIntermediateType,
				},
				Audit: configv1.Audit{
					Profile: configv1.DefaultAuditProfileType,
				},
			},
		}

		e := event.UpdateEvent{
			ObjectOld: oldAPIServer,
			ObjectNew: newAPIServer,
		}

		if pred.Update(e) {
			t.Error("Expected UpdateFunc to return false when only non-TLS fields change")
		}
	})
}
