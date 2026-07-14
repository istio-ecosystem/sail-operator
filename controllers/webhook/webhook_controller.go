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
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// DefaultDegradedWindow is how long a webhook stays not-ready after a call failure, so an
// intermittently failing control plane doesn't flap back to ready between events. It applies
// unless overridden per-webhook via the WebhookDegradedWindowAnnotationKey annotation.
const DefaultDegradedWindow = 2 * time.Minute

const failureHistoryCleanupInterval = 30 * time.Second

const webhookFailurePrefix = `failed calling webhook "`

// legacyReadinessAnnotationKeys are annotation keys older operator versions used to report
// readiness-probe results; they are removed when a webhook configuration is reconciled.
var legacyReadinessAnnotationKeys = []string{
	"sailoperator.io/readinessProbe.status",
	"sailoperator.io/readinessProbe.reason",
	"sailoperator.io/readinessProbe.periodSeconds",
	"sailoperator.io/readinessProbe.timeoutSeconds",
}

// failureHistory records, per webhook name, the time of its most recently observed call
// failure. It is safe for concurrent use.
type failureHistory struct {
	mu      sync.Mutex
	entries map[string]time.Time
}

func newFailureHistory() failureHistory {
	return failureHistory{entries: make(map[string]time.Time)}
}

// Add records name as having failed at the given time.
func (h *failureHistory) Add(name string, at time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.entries[name] = at
}

// Get returns the last recorded failure time for name, and whether one exists.
func (h *failureHistory) Get(name string) (time.Time, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	last, ok := h.entries[name]
	return last, ok
}

// Remove deletes name from the history, if present.
func (h *failureHistory) Remove(name string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.entries, name)
}

// RemoveOlderThan deletes entries whose recorded failure is at least window in the past,
// returning the number removed.
func (h *failureHistory) RemoveOlderThan(window time.Duration) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	pruned := 0
	for name, last := range h.entries {
		if now.Sub(last) >= window {
			delete(h.entries, name)
			pruned++
		}
	}
	return pruned
}

// Len returns the number of tracked entries.
func (h *failureHistory) Len() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.entries)
}

// Reconciler determines the readiness of a MutatingWebhookConfiguration pointing to a remote
// Istio control plane from the configuration's fields and webhook call failure events.
//
// Failure detection is passive: Kubernetes controllers record a Warning event only when an API
// call fails because a webhook was unreachable. A webhook that receives no API traffic cannot
// be detected, so the status reports "no known failures" rather than an active health check.
type Reconciler struct {
	Config config.ReconcilerConfig
	client.Client
	Scheme *runtime.Scheme

	failureHistory failureHistory
}

func NewReconciler(cfg config.ReconcilerConfig, client client.Client, scheme *runtime.Scheme) *Reconciler {
	return &Reconciler{
		Config:         cfg,
		Client:         client,
		Scheme:         scheme,
		failureHistory: newFailureHistory(),
	}
}

// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=mutatingwebhookconfigurations,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch

// Reconcile records a MutatingWebhookConfiguration's readiness in its annotations, updating it
// only when the readiness (or legacy) annotations changed.
func (r *Reconciler) Reconcile(ctx context.Context, webhook *admissionv1.MutatingWebhookConfiguration) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	result := r.evaluateReadiness(webhook.GetName(), webhook.Webhooks, degradedWindow(log, webhook))
	if !result.ready {
		log.V(3).Info("Webhook not ready", "reason", result.reason)
	}

	status := strconv.FormatBool(result.ready)
	annotations := webhook.GetAnnotations()
	changed := annotations[constants.WebhookReadinessStatusAnnotationKey] != status ||
		annotations[constants.WebhookReadinessReasonAnnotationKey] != result.reason
	if !changed {
		for _, key := range legacyReadinessAnnotationKeys {
			if _, ok := annotations[key]; ok {
				changed = true
				break
			}
		}
	}
	if changed {
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[constants.WebhookReadinessStatusAnnotationKey] = status
		annotations[constants.WebhookReadinessReasonAnnotationKey] = result.reason
		for _, key := range legacyReadinessAnnotationKeys {
			delete(annotations, key)
		}
		webhook.SetAnnotations(annotations)
		if err := r.Client.Update(ctx, webhook); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{RequeueAfter: result.requeue}, nil
}

// readinessResult is the outcome of evaluateReadiness. reason is only set when ready is false;
// requeue is only set for a degraded webhook, so the controller re-evaluates once the degraded
// window expires (watches drive reconciliation otherwise).
type readinessResult struct {
	ready   bool
	reason  string
	requeue time.Duration
}

func (r *Reconciler) evaluateReadiness(name string, webhooks []admissionv1.MutatingWebhook, degradedWindow time.Duration) readinessResult {
	if len(webhooks) == 0 {
		return readinessResult{reason: "webhook configuration contains no webhooks"}
	}
	cc := webhooks[0].ClientConfig
	if cc.Service == nil && cc.URL == nil {
		return readinessResult{reason: "no endpoint configured in webhooks[].clientConfig"}
	}
	if len(cc.CABundle) == 0 {
		return readinessResult{reason: "webhooks[].clientConfig.caBundle hasn't been set; check if the remote istiod can access this cluster"}
	}
	if requeue := r.isDegraded(name, degradedWindow); requeue > 0 {
		return readinessResult{reason: "webhook call failures reported in cluster events", requeue: requeue}
	}
	return readinessResult{ready: true}
}

// degradedWindow returns the WebhookDegradedWindowAnnotationKey override on webhook, if present
// and valid, or DefaultDegradedWindow otherwise.
func degradedWindow(log logr.Logger, webhook *admissionv1.MutatingWebhookConfiguration) time.Duration {
	value, ok := webhook.GetAnnotations()[constants.WebhookDegradedWindowAnnotationKey]
	if !ok {
		return DefaultDegradedWindow
	}
	window, err := time.ParseDuration(value)
	if err != nil {
		log.Error(err, "invalid "+constants.WebhookDegradedWindowAnnotationKey+" annotation value; using the default",
			"value", value, "default", DefaultDegradedWindow)
		return DefaultDegradedWindow
	}
	return window
}

// isDegraded returns the time remaining in the degraded window after a webhook's most recent
// recorded failure, or 0 if not degraded. It removes the entry once the window has expired.
func (r *Reconciler) isDegraded(name string, window time.Duration) time.Duration {
	last, ok := r.failureHistory.Get(name)
	if !ok {
		return 0
	}
	remaining := window - time.Since(last)
	if remaining <= 0 {
		r.failureHistory.Remove(name)
		return 0
	}
	return remaining
}

func (r *Reconciler) recordFailure(name string) {
	r.failureHistory.Add(name, time.Now())
}

// pruneStaleFailures drops failureHistory entries whose degraded window has expired,
// returning the count. It sweeps the whole map, so it also reclaims entries for deleted
// configs that isDegraded can never reach.
func (r *Reconciler) pruneStaleFailures() int {
	return r.failureHistory.RemoveOlderThan(DefaultDegradedWindow)
}

func (r *Reconciler) failureHistoryJanitor(log logr.Logger, interval time.Duration) manager.Runnable {
	return manager.RunnableFunc(func(ctx context.Context) error {
		wait.Until(func() {
			if n := r.pruneStaleFailures(); n > 0 {
				log.V(3).Info("Pruned stale webhook failure history entries", "count", n)
			}
		}, interval, ctx.Done())
		return nil
	})
}

// leaderElectionRunnable gates a Runnable to the leader (runs immediately when leader
// election is disabled).
type leaderElectionRunnable struct {
	manager.Runnable
}

func (leaderElectionRunnable) NeedLeaderElection() bool { return true }

// SetupWithManager sets up the controller with the Manager.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	logger := mgr.GetLogger().WithName("ctrlr").WithName("webhook")

	// objectHandler handles the MutatingWebhookConfiguration watch events
	objectHandler := wrapEventHandler(logger, &handler.EnqueueRequestForObject{})
	failureEventHandler := wrapEventHandler(logger, handler.EnqueueRequestsFromMapFunc(r.mapFailureEventToWebhook))

	err := ctrl.NewControllerManagedBy(mgr).
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
		Watches(&admissionv1.MutatingWebhookConfiguration{}, objectHandler, builder.WithPredicates(ownedByRemoteIstioRevisionPredicate(mgr.GetClient()))).
		Watches(&corev1.Event{}, failureEventHandler, builder.WithPredicates(webhookFailureEventPredicate())).
		Named("mutatingwebhookconfiguration").
		Complete(reconciler.NewStandardReconciler[*admissionv1.MutatingWebhookConfiguration](r.Client, r.Reconcile))
	if err != nil {
		return err
	}

	// Gate the janitor to the leader: only the leader populates failureHistory.
	return mgr.Add(leaderElectionRunnable{Runnable: r.failureHistoryJanitor(logger, failureHistoryCleanupInterval)})
}

// mapFailureEventToWebhook resolves a webhook failure event to its owning
// MutatingWebhookConfiguration, records the failure, and enqueues it for reconciliation.
func (r *Reconciler) mapFailureEventToWebhook(ctx context.Context, obj client.Object) []reconcile.Request {
	evt, ok := obj.(*corev1.Event)
	if !ok {
		return nil
	}
	webhookName := ExtractWebhookName(evt.Message)
	if webhookName == "" {
		return nil
	}

	configName := r.findOwnedWebhookConfig(ctx, webhookName)
	if configName == "" {
		return nil
	}

	r.recordFailure(configName)

	logf.FromContext(ctx).V(3).Info("Detected webhook call failure", "webhook", webhookName, "config", configName)
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: configName}}}
}

// findOwnedWebhookConfig resolves a webhook name to the MutatingWebhookConfiguration that
// contains it and is owned by a remote-control-plane IstioRevision, or "" if none exists.
func (r *Reconciler) findOwnedWebhookConfig(ctx context.Context, webhookName string) string {
	log := logf.FromContext(ctx)
	var configs admissionv1.MutatingWebhookConfigurationList
	if err := r.Client.List(ctx, &configs); err != nil {
		log.Error(err, "failed to list MutatingWebhookConfigurations")
		return ""
	}
	for i := range configs.Items {
		for _, webhook := range configs.Items[i].Webhooks {
			if webhook.Name == webhookName && IsOwnedByRevisionWithRemoteControlPlane(r.Client, &configs.Items[i]) {
				return configs.Items[i].Name
			}
		}
	}
	return ""
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

// webhookFailureEventPredicate matches Warning events that report a webhook call failure.
// Delete and generic events are ignored: an expiring or deleted event is not a new failure, and
// re-recording it would spuriously mark the webhook as degraded.
func webhookFailureEventPredicate() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return isWebhookFailureEvent(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return isWebhookFailureEvent(e.ObjectNew)
		},
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
	}
}

func isWebhookFailureEvent(obj client.Object) bool {
	evt, ok := obj.(*corev1.Event)
	if !ok {
		return false
	}
	return evt.Type == corev1.EventTypeWarning && strings.Contains(evt.Message, webhookFailurePrefix)
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

func wrapEventHandler(logger logr.Logger, handler handler.EventHandler) handler.EventHandler {
	return enqueuelogger.WrapIfNecessary("MutatingWebhookConfiguration", logger, handler)
}
