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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	"github.com/istio-ecosystem/sail-operator/pkg/enqueuelogger"
	"github.com/istio-ecosystem/sail-operator/pkg/reconciler"
	"github.com/istio-ecosystem/sail-operator/pkg/revision"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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

const minFailureGap = 30 * time.Second

const webhookFailurePrefix = `failed calling webhook "`

type failureEntry struct {
	prev, last time.Time
}

// Reconciler determines the readiness of webhook configurations pointing to a remote Istio control plane
// by observing the webhook configuration's fields and webhook call failure events.
type Reconciler struct {
	Config config.ReconcilerConfig
	client.Client
	Scheme *runtime.Scheme

	mu             sync.Mutex
	failureHistory map[string]failureEntry
}

func NewReconciler(cfg config.ReconcilerConfig, client client.Client, scheme *runtime.Scheme) *Reconciler {
	return &Reconciler{
		Config:         cfg,
		Client:         client,
		Scheme:         scheme,
		failureHistory: make(map[string]failureEntry),
	}
}

// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=mutatingwebhookconfigurations,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch

func (r *Reconciler) ReconcileMutating(ctx context.Context, webhook *admissionv1.MutatingWebhookConfiguration) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	isReady, reason, requeue := r.evaluateReadiness(webhook.Name, len(webhook.Webhooks), webhook.Webhooks[0].ClientConfig)
	if !isReady {
		log.V(3).Info("Webhook not ready", "reason", reason)
	}

	if webhook.Annotations == nil {
		webhook.Annotations = make(map[string]string)
	}
	webhook.Annotations[constants.WebhookReadinessStatusAnnotationKey] = strconv.FormatBool(isReady)
	webhook.Annotations[constants.WebhookReadinessReasonAnnotationKey] = reason

	err := r.Client.Update(ctx, webhook)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: requeue}, nil
}

func (r *Reconciler) ReconcileValidating(ctx context.Context, webhook *admissionv1.ValidatingWebhookConfiguration) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	isReady, reason, requeue := r.evaluateReadiness(webhook.Name, len(webhook.Webhooks), webhook.Webhooks[0].ClientConfig)
	if !isReady {
		log.V(3).Info("Webhook not ready", "reason", reason)
	}

	if webhook.Annotations == nil {
		webhook.Annotations = make(map[string]string)
	}
	webhook.Annotations[constants.WebhookReadinessStatusAnnotationKey] = strconv.FormatBool(isReady)
	webhook.Annotations[constants.WebhookReadinessReasonAnnotationKey] = reason

	err := r.Client.Update(ctx, webhook)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: requeue}, nil
}

// evaluateReadiness checks readiness (config fields) and health (recent failure events).
// When degraded, it returns a requeue duration so the controller re-evaluates after the
// degraded window expires. When not degraded, requeue is zero (watches drive reconciliation).
func (r *Reconciler) evaluateReadiness(name string, webhookCount int, cc admissionv1.WebhookClientConfig) (bool, string, time.Duration) {
	if webhookCount == 0 {
		return false, "webhook configuration contains no webhooks", 0
	}
	if cc.Service == nil && cc.URL == nil {
		return false, "no endpoint configured in webhooks[].clientConfig", 0
	}
	if len(cc.CABundle) == 0 {
		return false, "webhooks[].clientConfig.caBundle hasn't been set; check if the remote istiod can access this cluster", 0
	}
	if requeue := r.isDegraded(name); requeue > 0 {
		return false, "apiserver reported webhook call failures", requeue
	}
	return true, "", 0
}

// isDegraded returns how long the webhook should remain in a degraded state.
// It uses the gap between the two most recent failures to estimate the controller's
// backoff interval, and clears the degraded state after 2x that gap with no new failures.
// Returns 0 if not degraded; otherwise the time remaining until the degraded window expires.
func (r *Reconciler) isDegraded(name string) time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry, ok := r.failureHistory[name]
	if !ok {
		return 0
	}
	gap := minFailureGap
	if !entry.prev.IsZero() {
		if g := entry.last.Sub(entry.prev); g > gap {
			gap = g
		}
	}
	remaining := 2*gap - time.Since(entry.last)
	if remaining <= 0 {
		delete(r.failureHistory, name)
		return 0
	}
	return remaining
}

func (r *Reconciler) recordFailure(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry := r.failureHistory[name]
	entry.prev = entry.last
	entry.last = time.Now()
	r.failureHistory[name] = entry
}

// SetupWithManager sets up both the mutating and validating webhook controllers with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := r.setupMutatingController(mgr); err != nil {
		return err
	}
	return r.setupValidatingController(mgr)
}

func (r *Reconciler) setupMutatingController(mgr ctrl.Manager) error {
	logger := mgr.GetLogger().WithName("ctrlr").WithName("webhook")

	objectHandler := wrapEventHandler("MutatingWebhookConfiguration", logger, &handler.EnqueueRequestForObject{})
	failureEventHandler := wrapEventHandler("MutatingWebhookConfiguration", logger,
		handler.EnqueueRequestsFromMapFunc(r.mapFailureEventToMutatingWebhook))

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{
			LogConstructor: func(req *reconcile.Request) logr.Logger {
				log := logger
				if req != nil {
					log = log.WithValues("MutatingWebhookConfiguration", req.Name)
				}
				return log
			},
			MaxConcurrentReconciles: r.Config.MaxConcurrentReconciles,
		}).

		// we use the Watches function instead of For(), so that we can wrap the handler so that events that cause the object to be enqueued are logged
		// +lint-watches:ignore: IstioRevision (not found in charts, but this is the main resource watched by this controller)
		Watches(&admissionv1.MutatingWebhookConfiguration{}, objectHandler, builder.WithPredicates(ownedByRemoteIstioRevisionPredicate(mgr.GetClient()))).

		// +lint-watches:ignore: Event (not found in charts, watched for webhook failure detection)
		Watches(&corev1.Event{}, failureEventHandler, builder.WithPredicates(webhookFailureEventPredicate())).
		Named("mutatingwebhookconfiguration").
		Complete(reconciler.NewStandardReconciler[*admissionv1.MutatingWebhookConfiguration](r.Client, r.ReconcileMutating))
}

func (r *Reconciler) setupValidatingController(mgr ctrl.Manager) error {
	logger := mgr.GetLogger().WithName("ctrlr").WithName("webhook-validating")

	objectHandler := wrapEventHandler("ValidatingWebhookConfiguration", logger, &handler.EnqueueRequestForObject{})
	failureEventHandler := wrapEventHandler("ValidatingWebhookConfiguration", logger,
		handler.EnqueueRequestsFromMapFunc(r.mapFailureEventToValidatingWebhook))

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(controller.Options{
			LogConstructor: func(req *reconcile.Request) logr.Logger {
				log := logger
				if req != nil {
					log = log.WithValues("ValidatingWebhookConfiguration", req.Name)
				}
				return log
			},
			MaxConcurrentReconciles: r.Config.MaxConcurrentReconciles,
		}).

		// +lint-watches:ignore: IstioRevision (not found in charts, but this is the main resource watched by this controller)
		Watches(&admissionv1.ValidatingWebhookConfiguration{}, objectHandler, builder.WithPredicates(ownedByRemoteIstioRevisionPredicate(mgr.GetClient()))).

		// +lint-watches:ignore: Event (not found in charts, watched for webhook failure detection)
		Watches(&corev1.Event{}, failureEventHandler, builder.WithPredicates(webhookFailureEventPredicate())).
		Named("validatingwebhookconfiguration").
		Complete(reconciler.NewStandardReconciler[*admissionv1.ValidatingWebhookConfiguration](r.Client, r.ReconcileValidating))
}

type webhookConfigRef struct {
	name       string
	isMutating bool
}

// mapFailureEventToMutatingWebhook extracts the webhook name from a failure event,
// resolves it to a MutatingWebhookConfiguration name, records the failure,
// and enqueues the webhook for reconciliation.
func (r *Reconciler) mapFailureEventToMutatingWebhook(ctx context.Context, obj client.Object) []reconcile.Request {
	return r.mapFailureEvent(ctx, obj, true)
}

// mapFailureEventToValidatingWebhook extracts the webhook name from a failure event,
// resolves it to a ValidatingWebhookConfiguration name, records the failure,
// and enqueues the webhook for reconciliation.
func (r *Reconciler) mapFailureEventToValidatingWebhook(ctx context.Context, obj client.Object) []reconcile.Request {
	return r.mapFailureEvent(ctx, obj, false)
}

func (r *Reconciler) mapFailureEvent(ctx context.Context, obj client.Object, wantMutating bool) []reconcile.Request {
	evt, ok := obj.(*corev1.Event)
	if !ok {
		return nil
	}
	webhookName := ExtractWebhookName(evt.Message)
	if webhookName == "" {
		return nil
	}

	ref := r.findWebhookConfig(ctx, webhookName)
	if ref == nil || ref.isMutating != wantMutating {
		return nil
	}

	r.recordFailure(ref.name)

	logf.FromContext(ctx).V(3).Info("Detected webhook call failure", "webhook", webhookName, "config", ref.name)
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: ref.name}}}
}

// findWebhookConfig resolves an individual webhook name (e.g. "sidecar-injector.istio.io")
// to the webhook configuration that contains it. Searches both Mutating and Validating configurations.
// Returns nil if not found.
func (r *Reconciler) findWebhookConfig(ctx context.Context, webhookName string) *webhookConfigRef {
	var mutatingConfigs admissionv1.MutatingWebhookConfigurationList
	if err := r.Client.List(ctx, &mutatingConfigs); err != nil {
		logf.FromContext(ctx).Error(err, "Failed to list MutatingWebhookConfigurations")
	} else {
		for i := range mutatingConfigs.Items {
			for j := range mutatingConfigs.Items[i].Webhooks {
				if mutatingConfigs.Items[i].Webhooks[j].Name == webhookName {
					return &webhookConfigRef{name: mutatingConfigs.Items[i].Name, isMutating: true}
				}
			}
		}
	}

	var validatingConfigs admissionv1.ValidatingWebhookConfigurationList
	if err := r.Client.List(ctx, &validatingConfigs); err != nil {
		logf.FromContext(ctx).Error(err, "Failed to list ValidatingWebhookConfigurations")
	} else {
		for i := range validatingConfigs.Items {
			for j := range validatingConfigs.Items[i].Webhooks {
				if validatingConfigs.Items[i].Webhooks[j].Name == webhookName {
					return &webhookConfigRef{name: validatingConfigs.Items[i].Name, isMutating: false}
				}
			}
		}
	}

	return nil
}

func ExtractWebhookName(message string) string {
	_, after, found := strings.Cut(message, webhookFailurePrefix)
	if !found {
		return ""
	}
	name, _, found := strings.Cut(after, `"`)
	if !found {
		return ""
	}
	return name
}

func webhookFailureEventPredicate() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		evt, ok := obj.(*corev1.Event)
		if !ok {
			return false
		}
		return evt.Type == corev1.EventTypeWarning && strings.Contains(evt.Message, webhookFailurePrefix)
	})
}

func ownedByRemoteIstioRevisionPredicate(cl client.Client) predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return IsOwnedByRevisionWithRemoteControlPlane(cl, e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return IsOwnedByRevisionWithRemoteControlPlane(cl, e.ObjectNew)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return IsOwnedByRevisionWithRemoteControlPlane(cl, e.Object)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return IsOwnedByRevisionWithRemoteControlPlane(cl, e.Object)
		},
	}
}

func IsOwnedByRevisionWithRemoteControlPlane(cl client.Client, obj client.Object) bool {
	for _, ownerRef := range obj.GetOwnerReferences() {
		if ownerRef.APIVersion == v1.GroupVersion.String() && ownerRef.Kind == v1.IstioRevisionKind {
			rev := &v1.IstioRevision{}
			err := cl.Get(context.Background(), client.ObjectKey{Name: ownerRef.Name}, rev)
			if err != nil {
				return false
			}
			if revision.IsUsingRemoteControlPlane(rev) {
				return true
			}
		}
	}
	return false
}

func wrapEventHandler(resourceName string, logger logr.Logger, handler handler.EventHandler) handler.EventHandler {
	return enqueuelogger.WrapIfNecessary(resourceName, logger, handler)
}
