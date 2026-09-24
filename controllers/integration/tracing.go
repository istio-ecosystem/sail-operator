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

package integration

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/api/v1alpha1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	sailcontroller "github.com/istio-ecosystem/sail-operator/pkg/controller"
	"github.com/istio-ecosystem/sail-operator/pkg/enqueuelogger"
	applyv1 "github.com/istio-ecosystem/sail-operator/pkg/generated/applyconfigurations/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/reconciler"
	otelv1beta1 "github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	telemetryapiv1 "istio.io/api/telemetry/v1"
	telemetryv1 "istio.io/client-go/pkg/apis/telemetry/v1"
	telemetryapplyv1 "istio.io/client-go/pkg/applyconfiguration/telemetry/v1"
)

const (
	fieldManager              = "sail-operator-tracing-integration"
	otelCollectorKind         = "OpenTelemetryCollector"
	defaultOTLPGRPCPort       = uint32(4317)
	defaultOTLPHTTPPort       = uint32(4318)
	defaultOTLPHTTPTracesPath = "/v1/traces"
	telemetryResourceLabel    = "sailoperator.io/tracing-integration"
)

// TracingReconciler reconciles TracingIntegration objects.
type TracingReconciler struct {
	client.Client
	Config config.ReconcilerConfig
	Scheme *runtime.Scheme
}

// telemetryApplyConfiguration adds the marker method required by Kubernetes 0.36,
// which Istio's generated apply configuration does not yet provide.
type telemetryApplyConfiguration struct {
	*telemetryapplyv1.TelemetryApplyConfiguration
}

func (telemetryApplyConfiguration) IsApplyConfiguration() {}

func (c telemetryApplyConfiguration) GetName() *string {
	return c.Name
}

func (c telemetryApplyConfiguration) GetNamespace() *string {
	return c.Namespace
}

func (c telemetryApplyConfiguration) GetKind() *string {
	return c.Kind
}

func (c telemetryApplyConfiguration) GetAPIVersion() *string {
	return c.APIVersion
}

func NewTracingReconciler(cfg config.ReconcilerConfig, client client.Client, scheme *runtime.Scheme) *TracingReconciler {
	return &TracingReconciler{
		Config: cfg,
		Client: client,
		Scheme: scheme,
	}
}

// +kubebuilder:rbac:groups=sailoperator.io,resources=tracingintegrations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sailoperator.io,resources=tracingintegrations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=sailoperator.io,resources=istios,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups=telemetry.istio.io,resources=telemetries,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=opentelemetry.io,resources=opentelemetrycollectors,verbs=get;list;watch
// Reconcile takes a TracingIntegration resource and wires up the trace integration with the targetRefs
// creating the necessary resources to accomplish this.
func (r *TracingReconciler) Reconcile(ctx context.Context, tracingIntegration *v1alpha1.TracingIntegration) (ctrl.Result, error) {
	reconcileErr := r.doReconcile(ctx, tracingIntegration)
	statusErr := r.updateStatus(ctx, tracingIntegration, reconcileErr)

	if apierrors.IsConflict(reconcileErr) {
		// Server-side apply conflicts should be visible in status, but the controller
		// must not force ownership away from users or other controllers.
		return ctrl.Result{}, statusErr
	}
	return ctrl.Result{}, errors.Join(reconcileErr, statusErr)
}

func (r *TracingReconciler) doReconcile(ctx context.Context, tracingIntegration *v1alpha1.TracingIntegration) error {
	if err := r.validateProvider(ctx, tracingIntegration); err != nil {
		return err
	}

	if err := r.validateTargetRefUniqueness(ctx, tracingIntegration); err != nil {
		return err
	}

	for _, targetRef := range tracingIntegration.Spec.TargetRefs {
		switch targetRef.Kind {
		case v1.IstioKind:
			if err := r.reconcileIstioTarget(ctx, tracingIntegration, targetRef.Name); err != nil {
				return err
			}
		default:
			return reconciler.NewValidationError(fmt.Sprintf("unsupported tracing target kind %q", targetRef.Kind))
		}
	}

	return nil
}

func (r *TracingReconciler) validateProvider(ctx context.Context, tracingIntegration *v1alpha1.TracingIntegration) error {
	if tracingIntegration.Spec.OpenTelemetry == nil {
		return reconciler.NewValidationError("spec.openTelemetry must be set when spec.type is OpenTelemetry")
	}

	ref := tracingIntegration.Spec.OpenTelemetry.OTELCollectorRef
	collector := &otelv1beta1.OpenTelemetryCollector{}
	if err := r.Client.Get(ctx, client.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}, collector); err != nil {
		if apierrors.IsNotFound(err) {
			// TODO: Add an E2E test covering status updates for a missing OpenTelemetryCollector.
			return reconciler.NewValidationError(fmt.Sprintf("referenced %s %s/%s was not found", otelCollectorKind, ref.Namespace, ref.Name))
		}
		return err
	}

	return nil
}

// validateTargetRefUniqueness ensures that each target reference has a single owner.
// When multiple TracingIntegrations target the same resource, the oldest one owns it;
// newer integrations receive a validation error. Creation timestamps are resolved by name.
func (r *TracingReconciler) validateTargetRefUniqueness(ctx context.Context, tracingIntegration *v1alpha1.TracingIntegration) error {
	integrations := &v1alpha1.TracingIntegrationList{}
	if err := r.Client.List(ctx, integrations); err != nil {
		return err
	}

	targetRefs := map[v1alpha1.TargetReference]struct{}{}
	for _, ref := range tracingIntegration.Spec.TargetRefs {
		// Assuming we've already validated that there's not multiple targetRefs targeting the same kind.
		targetRefs[ref] = struct{}{}
	}

	for _, integration := range integrations.Items {
		// Skip self.
		if integration.Name == tracingIntegration.Name {
			continue
		}

		for _, ref := range integration.Spec.TargetRefs {
			if _, alreadyTargeted := targetRefs[ref]; alreadyTargeted {
				// If this is already targeted, emit a validation error for the newer TracingIntegration.
				if integration.CreationTimestamp.Before(&tracingIntegration.CreationTimestamp) ||
					(integration.CreationTimestamp.Equal(&tracingIntegration.CreationTimestamp) && integration.Name < tracingIntegration.Name) {
					return reconciler.NewValidationError(fmt.Sprintf(
						"targetRef %s/%s is already managed by TracingIntegration %q", ref.Kind, ref.Name, integration.Name))
				}
			}
		}
	}

	return nil
}

func (r *TracingReconciler) reconcileIstioTarget(
	ctx context.Context, tracingIntegration *v1alpha1.TracingIntegration, istioName string,
) error {
	istio := &v1.Istio{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: istioName}, istio); err != nil {
		if apierrors.IsNotFound(err) {
			return reconciler.NewValidationError(fmt.Sprintf("target Istio %q was not found", istioName))
		}
		return err
	}

	if err := r.applyIstioTracing(ctx, istio, tracingIntegration); err != nil {
		return err
	}
	if err := r.applyTelemetry(ctx, tracingIntegration, istio); err != nil {
		return err
	}

	return nil
}

func (r *TracingReconciler) applyIstioTracing(ctx context.Context, istio *v1.Istio, tracingIntegration *v1alpha1.TracingIntegration) error {
	collectorRef := tracingIntegration.Spec.OpenTelemetry.OTELCollectorRef
	providerName := collectorRef.Name

	collector := &otelv1beta1.OpenTelemetryCollector{}
	if err := r.Client.Get(ctx, client.ObjectKey{Namespace: collectorRef.Namespace, Name: collectorRef.Name}, collector); err != nil {
		if apierrors.IsNotFound(err) {
			return reconciler.NewValidationError(fmt.Sprintf("referenced %s %s/%s was not found", otelCollectorKind, collectorRef.Namespace, collectorRef.Name))
		}
		return err
	}

	otelProvider, err := openTelemetryTracingProvider(collector)
	if err != nil {
		return err
	}

	extensionProvider := &v1.MeshConfigExtensionProvider{
		Name:          new(providerName),
		Opentelemetry: otelProvider,
	}
	applyConfig := applyv1.Istio(istio.Name).WithSpec(
		applyv1.IstioSpec().WithValues(
			applyv1.Values().WithMeshConfig(
				applyv1.MeshConfig().WithEnableTracing(true).WithExtensionProviders(&extensionProvider),
			),
		),
	)
	return r.Client.Apply(ctx, applyConfig, client.FieldOwner(fieldManager))
}

func openTelemetryTracingProvider(
	collector *otelv1beta1.OpenTelemetryCollector,
) (*v1.MeshConfigExtensionProviderOpenTelemetryTracingProvider, error) {
	port, useHTTP, httpPath, err := otelReceiverPort(collector)
	if err != nil {
		return nil, err
	}

	provider := &v1.MeshConfigExtensionProviderOpenTelemetryTracingProvider{
		// TODO: should we hardcode this or try and fetch the Service?
		// This URL may also depend on deployment options specified for the otel collector.
		Service: new(fmt.Sprintf("%s-collector.%s.svc.cluster.local", collector.Name, collector.Namespace)),
		Port:    new(port),
	}
	if useHTTP {
		provider.Http = &v1.MeshConfigExtensionProviderHttpService{
			Path: new(httpPath),
		}
	}
	return provider, nil
}

func otelReceiverPort(collector *otelv1beta1.OpenTelemetryCollector) (uint32, bool, string, error) {
	protocols, err := otlpReceiverProtocols(collector)
	if err != nil {
		return 0, false, "", err
	}

	// Prefer gRPC when both OTLP receiver protocols are configured.
	if grpcConfig, ok := protocols["grpc"]; ok {
		port, err := otelProtocolPort(grpcConfig, defaultOTLPGRPCPort, "grpc")
		return port, false, "", err
	}
	if httpConfig, ok := protocols["http"]; ok {
		port, err := otelProtocolPort(httpConfig, defaultOTLPHTTPPort, "http")
		if err != nil {
			return 0, false, "", err
		}
		return port, true, otelHTTPTracesPath(httpConfig), nil
	}

	return 0, false, "", reconciler.NewValidationError("OpenTelemetryCollector spec.config.receivers.otlp.protocols must define grpc or http")
}

func otlpReceiverProtocols(collector *otelv1beta1.OpenTelemetryCollector) (map[string]any, error) {
	receivers := collector.Spec.Config.Receivers.Object
	otlpReceiver, ok := receivers["otlp"].(map[string]any)
	if !ok {
		return nil, reconciler.NewValidationError("OpenTelemetryCollector spec.config.receivers must define an otlp receiver")
	}
	protocols, ok := otlpReceiver["protocols"].(map[string]any)
	if !ok {
		return nil, reconciler.NewValidationError("OpenTelemetryCollector spec.config.receivers.otlp must define protocols")
	}
	return protocols, nil
}

func otelProtocolPort(protocolConfig any, defaultPort uint32, protocol string) (uint32, error) {
	config, ok := protocolConfig.(map[string]any)
	if !ok {
		return 0, reconciler.NewValidationError(fmt.Sprintf("OpenTelemetryCollector otlp %s protocol config must be an object", protocol))
	}

	endpoint, ok := config["endpoint"].(string)
	if !ok || strings.TrimSpace(endpoint) == "" {
		return defaultPort, nil
	}

	port, err := parseEndpointPort(endpoint)
	if err != nil {
		return 0, reconciler.NewValidationError(fmt.Sprintf("OpenTelemetryCollector otlp %s endpoint %q must include a valid port: %v", protocol, endpoint, err))
	}
	return port, nil
}

func otelHTTPTracesPath(protocolConfig any) string {
	config, ok := protocolConfig.(map[string]any)
	if !ok {
		return defaultOTLPHTTPTracesPath
	}
	if path, ok := config["traces_url_path"].(string); ok && strings.TrimSpace(path) != "" {
		return path
	}
	return defaultOTLPHTTPTracesPath
}

func parseEndpointPort(endpoint string) (uint32, error) {
	port := ""
	parsedURL, urlErr := url.Parse(endpoint)
	if urlErr == nil {
		port = parsedURL.Port()
	}
	if port == "" {
		_, splitPort, splitErr := net.SplitHostPort(endpoint)
		if splitErr == nil {
			port = splitPort
		} else if separator := strings.LastIndexByte(endpoint, ':'); separator >= 0 {
			// The OpenTelemetry Operator supports environment substitutions such as
			// ${env:MY_POD_IP}:4317, which net.SplitHostPort cannot parse as a host.
			port = endpoint[separator+1:]
		} else if urlErr != nil {
			return 0, urlErr
		} else {
			return 0, splitErr
		}
	}

	parsedPort, err := strconv.ParseUint(port, 10, 16)
	if err != nil {
		return 0, err
	}
	if parsedPort == 0 {
		return 0, fmt.Errorf("port must be greater than 0")
	}
	return uint32(parsedPort), nil
}

func (r *TracingReconciler) applyTelemetry(
	ctx context.Context, tracingIntegration *v1alpha1.TracingIntegration, istio *v1.Istio,
) error {
	// TODO: Should we put an ownerref on this or not?
	// It is expected that this mesh wide telemetry object will
	// be shared by users so probably we shouldn't add an ownerref
	// to it even if it means leaving the resource around when the
	// integration gets deleted.
	applyConfig := telemetryapplyv1.Telemetry(tracingIntegration.Spec.TelemetryName, istio.Spec.Namespace).
		WithLabels(map[string]string{
			telemetryResourceLabel: tracingIntegration.Name,
		}).
		WithSpec(telemetryapiv1.Telemetry{
			Tracing: []*telemetryapiv1.Tracing{
				{
					Providers: []*telemetryapiv1.ProviderRef{
						{Name: tracingIntegration.Spec.OpenTelemetry.OTELCollectorRef.Name},
					},
				},
			},
		})

	return r.Client.Apply(ctx, &telemetryApplyConfiguration{applyConfig}, client.FieldOwner(fieldManager))
}

func (r *TracingReconciler) updateStatus(ctx context.Context, tracingIntegration *v1alpha1.TracingIntegration, reconcileErr error) error {
	status := *tracingIntegration.Status.DeepCopy()
	status.ObservedGeneration = tracingIntegration.Generation
	status.SetCondition(r.determineReconciledCondition(tracingIntegration.Generation, reconcileErr))
	status.SetCondition(r.determineConflictedCondition(tracingIntegration.Generation, reconcileErr))
	status.State = determineState(status)
	return reconciler.UpdateStatus(ctx, r.Client, tracingIntegration, tracingIntegration.Status, status, nil)
}

func (r *TracingReconciler) determineReconciledCondition(generation int64, err error) metav1.Condition {
	condition := metav1.Condition{
		ObservedGeneration: generation,
		Type:               string(v1alpha1.TracingIntegrationConditionReconciled),
	}
	// We still consider the resource reconciled even if there are conflict errors
	// because of Server Side Apply. In the future we will retry the Apply minus the conflicts.
	if err == nil || apierrors.IsConflict(err) {
		condition.Status = metav1.ConditionTrue
		condition.Reason = string(v1alpha1.TracingIntegrationConditionReconciled)
		return condition
	}

	condition.Status = metav1.ConditionFalse
	condition.Message = fmt.Sprintf("error reconciling resource: %v", err)
	if reconciler.IsValidationError(err) {
		condition.Reason = string(v1alpha1.TracingIntegrationReasonInvalidConfiguration)
	} else {
		condition.Reason = string(v1alpha1.TracingIntegrationReasonReconcileError)
	}
	return condition
}

func (r *TracingReconciler) determineConflictedCondition(generation int64, err error) metav1.Condition {
	condition := metav1.Condition{
		ObservedGeneration: generation,
		Type:               string(v1alpha1.TracingIntegrationConditionConflicted),
	}
	if apierrors.IsConflict(err) {
		condition.Status = metav1.ConditionTrue
		condition.Reason = string(v1alpha1.TracingIntegrationReasonApplyConflict)
		condition.Message = fmt.Sprintf("server-side apply conflict: %v", err)
		return condition
	}

	condition.Status = metav1.ConditionFalse
	condition.Reason = string(v1alpha1.TracingIntegrationReasonNoConflict)
	return condition
}

func determineState(status v1alpha1.TracingIntegrationStatus) v1alpha1.TracingIntegrationConditionReason {
	reconciledCondition := status.GetCondition(v1alpha1.TracingIntegrationConditionReconciled)
	if reconciledCondition.Status != metav1.ConditionTrue {
		return v1alpha1.TracingIntegrationConditionReason(reconciledCondition.Reason)
	}
	conflictedCondition := status.GetCondition(v1alpha1.TracingIntegrationConditionConflicted)
	if conflictedCondition.Status == metav1.ConditionTrue {
		return v1alpha1.TracingIntegrationReasonApplyConflict
	}
	return v1alpha1.TracingIntegrationReasonHealthy
}

// SetupWithManager sets up the controller with the Manager.
// This reconciler will wait for tracing CRDs to be ready
// before starting the reconciler.
func (r *TracingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	logger := mgr.GetLogger().WithName("ctrlr").WithName("tracingintegration")

	mainObjectHandler := wrapEventHandler(logger, &handler.EnqueueRequestForObject{})
	istioHandler := wrapEventHandler(logger, handler.EnqueueRequestsFromMapFunc(r.mapIstioToReconcileRequest))
	otelCollectorHandler := wrapEventHandler(logger, handler.EnqueueRequestsFromMapFunc(r.mapOTELCollectorToReconcileRequest))
	telemetryHandler := wrapEventHandler(logger, handler.EnqueueRequestsFromMapFunc(r.mapTelemetryToReconcileRequest))

	options := controller.Options{
		LogConstructor: func(req *reconcile.Request) logr.Logger {
			log := logger
			if req != nil {
				log = log.WithValues("tracingintegration", req.Name)
			}
			return log
		},
		MaxConcurrentReconciles: r.Config.MaxConcurrentReconciles,
		Reconciler:              reconciler.NewStandardReconciler[*v1alpha1.TracingIntegration](r.Client, r.Reconcile),
	}
	options.DefaultFromConfig(mgr.GetControllerOptions())
	c, err := sailcontroller.NewDelayedStartController("tracingintegration", options, mgr.GetCache())
	if err != nil {
		return err
	}
	for _, watch := range []source.Source{
		source.Kind[client.Object](mgr.GetCache(), &v1alpha1.TracingIntegration{}, mainObjectHandler),
		source.Kind[client.Object](mgr.GetCache(), &v1.Istio{}, istioHandler),
	} {
		if err := c.Watch(watch); err != nil {
			return err
		}
	}

	c.DelayedWatch(constants.OpenTelemetryCollectorCRDName,
		source.Kind[client.Object](mgr.GetCache(), &otelv1beta1.OpenTelemetryCollector{}, otelCollectorHandler))
	c.DelayedWatch(constants.TelemetryCRDName, source.Kind[client.Object](mgr.GetCache(), &telemetryv1.Telemetry{}, telemetryHandler))

	return mgr.Add(c)
}

func (r *TracingReconciler) mapIstioToReconcileRequest(ctx context.Context, obj client.Object) []reconcile.Request {
	return r.mapIntegrations(ctx, func(tracingIntegration v1alpha1.TracingIntegration) bool {
		for _, targetRef := range tracingIntegration.Spec.TargetRefs {
			if targetRef.Kind == v1.IstioKind && targetRef.Name == obj.GetName() {
				return true
			}
		}
		return false
	})
}

func (r *TracingReconciler) mapOTELCollectorToReconcileRequest(ctx context.Context, obj client.Object) []reconcile.Request {
	return r.mapIntegrations(ctx, func(tracingIntegration v1alpha1.TracingIntegration) bool {
		if tracingIntegration.Spec.OpenTelemetry == nil {
			return false
		}
		ref := tracingIntegration.Spec.OpenTelemetry.OTELCollectorRef
		return ref.Name == obj.GetName() && ref.Namespace == obj.GetNamespace()
	})
}

// mapTelemetryToReconcileRequest maps a Telemetry back to the integrations that apply it.
// The Telemetry is deliberately left without an owner reference, so it is matched by name
// instead; that way changes made to it by users or other controllers are still noticed.
func (r *TracingReconciler) mapTelemetryToReconcileRequest(ctx context.Context, obj client.Object) []reconcile.Request {
	return r.mapIntegrations(ctx, func(tracingIntegration v1alpha1.TracingIntegration) bool {
		return tracingIntegration.Spec.TelemetryName == obj.GetName()
	})
}

func (r *TracingReconciler) mapIntegrations(
	ctx context.Context, matches func(v1alpha1.TracingIntegration) bool,
) []reconcile.Request {
	log := logf.FromContext(ctx)
	integrations := &v1alpha1.TracingIntegrationList{}
	if err := r.Client.List(ctx, integrations); err != nil {
		log.Error(err, "failed to list TracingIntegrations")
		return nil
	}

	requests := []reconcile.Request{}
	for _, tracingIntegration := range integrations.Items {
		if matches(tracingIntegration) {
			requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKey{Name: tracingIntegration.Name}})
		}
	}
	return requests
}

func wrapEventHandler(logger logr.Logger, handler handler.EventHandler) handler.EventHandler {
	return enqueuelogger.WrapIfNecessary(v1alpha1.TracingIntegrationKind, logger, handler)
}
