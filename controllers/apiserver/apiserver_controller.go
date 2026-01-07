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
	"reflect"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	configv1 "github.com/openshift/api/config/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Reconciler watches the OpenShift APIServer resource and calls shutdown when the
// parsed TLS profile differs from the initial configuration.
type Reconciler struct {
	client.Client
	initialTLSConfig config.TLSConfig
	shutdown         func()
}

// NewReconciler creates a new Reconciler.
func NewReconciler(client client.Client, initialTLSConfig config.TLSConfig, shutdown func()) *Reconciler {
	return &Reconciler{
		Client:           client,
		initialTLSConfig: initialTLSConfig,
		shutdown:         shutdown,
	}
}

// +kubebuilder:rbac:groups=config.openshift.io,resources=apiservers,verbs=get;list;watch

// Reconcile handles updates to the APIServer resource.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	log.Info("Reconciling TLS profile change to APIServer")
	apiServer := &configv1.APIServer{}
	if err := r.Client.Get(ctx, req.NamespacedName, apiServer); err != nil {
		return ctrl.Result{}, err
	}

	currentTLSConfig := config.TLSConfigFromAPIServer(apiServer)
	if diff := cmp.Diff(r.initialTLSConfig, currentTLSConfig); diff != "" {
		log.Info("APIServer TLS profile changed from initial configuration, calling shutdown", "diff", diff)
		r.shutdown()
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	logger := mgr.GetLogger().WithName("ctrlr").WithName("apiserver")
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{
			LogConstructor: func(req *reconcile.Request) logr.Logger {
				log := logger
				if req != nil {
					log = log.WithValues("APIServer", req.Name)
				}
				return log
			},
		}).
		For(&configv1.APIServer{}, builder.WithPredicates(tlsProfileChangedPredicate())).
		Named("apiserver").
		Complete(r)
}

// tlsProfileChangedPredicate returns a predicate that filters APIServer events
// to only trigger reconciliation when the TLS security profile changes.
func tlsProfileChangedPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldAPIServer, ok := e.ObjectOld.(*configv1.APIServer)
			if !ok {
				return false
			}
			newAPIServer, ok := e.ObjectNew.(*configv1.APIServer)
			if !ok {
				return false
			}
			return !reflect.DeepEqual(oldAPIServer.Spec.TLSSecurityProfile, newAPIServer.Spec.TLSSecurityProfile)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}
