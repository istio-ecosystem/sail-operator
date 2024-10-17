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

package webhook

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"github.com/istio-ecosystem/sail-operator/api/v1alpha1"
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	"github.com/istio-ecosystem/sail-operator/pkg/enqueuelogger"
	"github.com/istio-ecosystem/sail-operator/pkg/reconciler"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	defaultPeriodSeconds  = 3 // matches the period in the istiod chart
	defaultTimeoutSeconds = 5 // matches the timeout in the istiod chart
)

// overrides the default dial context; only used in unit tests
var customDialContext func(ctx context.Context, network, addr string) (net.Conn, error)

// Reconciler checks the readiness of MutatingWebhookConfiguration pointing to a remote Istio control plane
type Reconciler struct {
	client.Client
	Scheme *runtime.Scheme
	probe  func(context.Context, *admissionv1.MutatingWebhookConfiguration) (bool, error)
}

func NewReconciler(client client.Client, scheme *runtime.Scheme) *Reconciler {
	return &Reconciler{
		Client: client,
		Scheme: scheme,
		probe:  doProbe,
	}
}

// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=mutatingwebhookconfigurations,verbs=get;list;watch;create;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *Reconciler) Reconcile(ctx context.Context, webhook *admissionv1.MutatingWebhookConfiguration) (ctrl.Result, error) {
	isReady, err := r.probe(ctx, webhook)
	if err != nil {
		isReady = false
	}

	if webhook.Annotations == nil {
		webhook.Annotations = make(map[string]string)
	}
	webhook.Annotations[constants.WebhookReadinessProbeStatusAnnotationKey] = strconv.FormatBool(isReady)
	err = r.Client.Update(ctx, webhook)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: getPeriod(webhook)}, nil
}

func doProbe(ctx context.Context, webhook *admissionv1.MutatingWebhookConfiguration) (bool, error) {
	log := logf.FromContext(ctx).V(3)
	if len(webhook.Webhooks) == 0 {
		return false, errors.New("mutatingwebhookconfiguration contains no webhooks")
	}
	clientConfig := webhook.Webhooks[0].ClientConfig
	if clientConfig.Service == nil {
		return false, errors.New("missing webhooks[].clientConfig.service")
	}

	if len(clientConfig.CABundle) == 0 {
		return false, errors.New("webhooks[].clientConfig.caBundle hasn't been set; check if the remote istiod can access this cluster")
	}
	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM(clientConfig.CABundle); !ok {
		return false, errors.New("failed to append CA bundle to cert pool")
	}

	httpClient := http.Client{
		Timeout: getTimeout(webhook),
		Transport: &http.Transport{
			DialContext: customDialContext,
			TLSClientConfig: &tls.Config{
				RootCAs:    caCertPool,
				MinVersion: tls.VersionTLS12,
			},
		},
	}

	url, err := getReadinessProbeURL(clientConfig)
	if err != nil {
		return false, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}

	log.Info("Executing readiness probe on remote control plane", "url", req.URL.String())
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Info("Probe failed", "error", err)
		return false, err
	}
	log.Info("Probe response", "response", resp.StatusCode)

	return resp.StatusCode == http.StatusOK, nil
}

func getReadinessProbeURL(config admissionv1.WebhookClientConfig) (string, error) {
	switch {
	case config.URL != nil:
		return "", errors.New("only webhooks pointing to a Service are supported")

	case config.Service != nil:
		svc := config.Service
		port := 443
		if svc.Port != nil {
			port = int(*svc.Port)
		}
		return fmt.Sprintf("https://%s.%s.svc:%d/ready", svc.Name, svc.Namespace, port), nil

	default:
		return "", errors.New("no URL or Service specified in WebhookClientConfig")
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	logger := mgr.GetLogger().WithName("ctrlr").WithName("webhook")

	// objectHandler handles the MutatingWebhookConfiguration watch events
	objectHandler := wrapEventHandler(logger, &handler.EnqueueRequestForObject{})

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{
			LogConstructor: func(req *reconcile.Request) logr.Logger {
				log := logger
				if req != nil {
					log = log.WithValues("MutatingWebhookConfiguration", req.Name)
				}
				return log
			},
		}).

		// we use the Watches function instead of For(), so that we can wrap the handler so that events that cause the object to be enqueued are logged
		// +lint-watches:ignore: IstioRevision (not found in charts, but this is the main resource watched by this controller)
		Watches(&admissionv1.MutatingWebhookConfiguration{}, objectHandler, builder.WithPredicates(ownedByRemoteIstioPredicate(mgr.GetClient()))).
		Named("mutatingwebhookconfiguration").
		Complete(reconciler.NewStandardReconciler[*admissionv1.MutatingWebhookConfiguration](r.Client, r.Reconcile))
}

func ownedByRemoteIstioPredicate(cl client.Client) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return isOwnedByRemoteIstio(cl, e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return isOwnedByRemoteIstio(cl, e.ObjectNew)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return isOwnedByRemoteIstio(cl, e.Object)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return isOwnedByRemoteIstio(cl, e.Object)
		},
	}
}

func isOwnedByRemoteIstio(cl client.Client, obj client.Object) bool {
	for _, ownerRef := range obj.GetOwnerReferences() {
		if ownerRef.APIVersion == v1alpha1.GroupVersion.String() && ownerRef.Kind == v1alpha1.IstioRevisionKind {
			rev := &v1alpha1.IstioRevision{}
			err := cl.Get(context.Background(), client.ObjectKey{Name: ownerRef.Name}, rev)
			if err != nil {
				// TODO log error
			} else if rev.Spec.Type == v1alpha1.IstioRevisionTypeRemote {
				return true
			}
		}
	}
	return false
}

func getPeriod(webhook *admissionv1.MutatingWebhookConfiguration) time.Duration {
	if period, ok := webhook.Annotations[constants.WebhookReadinessProbePeriodSecondsAnnotationKey]; ok {
		if p, err := strconv.Atoi(period); err == nil {
			return time.Duration(p) * time.Second
		}
	}
	return defaultPeriodSeconds * time.Second
}

func getTimeout(webhook *admissionv1.MutatingWebhookConfiguration) time.Duration {
	if period, ok := webhook.Annotations[constants.WebhookReadinessProbeTimeoutSecondsAnnotationKey]; ok {
		if p, err := strconv.Atoi(period); err == nil {
			return time.Duration(p) * time.Second
		}
	}
	return defaultTimeoutSeconds * time.Second
}

func wrapEventHandler(logger logr.Logger, handler handler.EventHandler) handler.EventHandler {
	return enqueuelogger.WrapIfNecessary("MutatingWebhookConfiguration", logger, handler)
}
