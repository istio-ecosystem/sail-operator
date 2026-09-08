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
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/api/v1alpha1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/reconciler"
	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	otelv1beta1 "github.com/open-telemetry/opentelemetry-operator/apis/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	telemetryapiv1 "istio.io/api/telemetry/v1"
	telemetryv1 "istio.io/client-go/pkg/apis/telemetry/v1"
)

const (
	istioNamespace    = "istio-system"
	telemetryName     = "mesh-default"
	otherFieldManager = "some-other-controller"
)

func TestParseEndpointPort(t *testing.T) {
	for _, tc := range []struct {
		name     string
		endpoint string
		want     uint32
		wantErr  bool
	}{
		{name: "IPv4 address", endpoint: "0.0.0.0:4317", want: 4317},
		{name: "environment variable", endpoint: "${env:MY_POD_IP}:4317", want: 4317},
		{name: "IPv6 address", endpoint: "[::]:4317", want: 4317},
		{name: "URL", endpoint: "http://collector:4318", want: 4318},
		{name: "missing port", endpoint: "collector", wantErr: true},
		{name: "invalid port", endpoint: "collector:invalid", wantErr: true},
		{name: "zero port", endpoint: "collector:0", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseEndpointPort(tc.endpoint)
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Fatalf("parseEndpointPort(%q) = %d, %v; want %d, error=%v", tc.endpoint, got, err, tc.want, tc.wantErr)
			}
		})
	}
}

func TestReconcile(t *testing.T) {
	generation := int64(1)
	simulatedError := errors.New("simulated error")
	collectorNotFound := reconciler.NewValidationError("referenced OpenTelemetryCollector istio-system/collector was not found")
	istioNotFound := reconciler.NewValidationError(`target Istio "missing" was not found`)

	noConflictCondition := metav1.Condition{
		Type:               string(v1alpha1.TracingIntegrationConditionConflicted),
		Status:             metav1.ConditionFalse,
		Reason:             string(v1alpha1.TracingIntegrationReasonNoConflict),
		ObservedGeneration: generation,
	}

	for name, tc := range map[string]struct {
		withCollector       bool
		targetRefs          []v1alpha1.TargetReference
		existingObjects     []client.Object
		interceptors        interceptor.Funcs
		wantError           string
		wantValidationError bool
		expectedStatus      v1alpha1.TracingIntegrationStatus
	}{
		"healthy": {
			withCollector: true,
			expectedStatus: v1alpha1.TracingIntegrationStatus{
				ObservedGeneration: generation,
				State:              v1alpha1.TracingIntegrationReasonHealthy,
				Conditions: []metav1.Condition{
					{
						Type:               string(v1alpha1.TracingIntegrationConditionReconciled),
						Status:             metav1.ConditionTrue,
						Reason:             string(v1alpha1.TracingIntegrationConditionReconciled),
						ObservedGeneration: generation,
					},
					noConflictCondition,
				},
			},
		},
		"invalid configuration": {
			wantError:           collectorNotFound.Error(),
			wantValidationError: true,
			expectedStatus: v1alpha1.TracingIntegrationStatus{
				ObservedGeneration: generation,
				State:              v1alpha1.TracingIntegrationReasonInvalidConfiguration,
				Conditions: []metav1.Condition{
					{
						Type:               string(v1alpha1.TracingIntegrationConditionReconciled),
						Status:             metav1.ConditionFalse,
						Reason:             string(v1alpha1.TracingIntegrationReasonInvalidConfiguration),
						Message:            fmt.Sprintf("error reconciling resource: %v", collectorNotFound),
						ObservedGeneration: generation,
					},
					noConflictCondition,
				},
			},
		},
		"reconciliation error": {
			interceptors: interceptor.Funcs{
				Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					if _, ok := obj.(*otelv1beta1.OpenTelemetryCollector); ok {
						return simulatedError
					}
					return cl.Get(ctx, key, obj, opts...)
				},
			},
			wantError: simulatedError.Error(),
			expectedStatus: v1alpha1.TracingIntegrationStatus{
				ObservedGeneration: generation,
				State:              v1alpha1.TracingIntegrationReasonReconcileError,
				Conditions: []metav1.Condition{
					{
						Type:               string(v1alpha1.TracingIntegrationConditionReconciled),
						Status:             metav1.ConditionFalse,
						Reason:             string(v1alpha1.TracingIntegrationReasonReconcileError),
						Message:            fmt.Sprintf("error reconciling resource: %v", simulatedError),
						ObservedGeneration: generation,
					},
					noConflictCondition,
				},
			},
		},
		"non-existent istio ref produces validation error": {
			withCollector:       true,
			targetRefs:          []v1alpha1.TargetReference{{Kind: v1.IstioKind, Name: "missing"}},
			wantError:           istioNotFound.Error(),
			wantValidationError: true,
			expectedStatus: v1alpha1.TracingIntegrationStatus{
				ObservedGeneration: generation,
				State:              v1alpha1.TracingIntegrationReasonInvalidConfiguration,
				Conditions: []metav1.Condition{
					{
						Type:               string(v1alpha1.TracingIntegrationConditionReconciled),
						Status:             metav1.ConditionFalse,
						Reason:             string(v1alpha1.TracingIntegrationReasonInvalidConfiguration),
						Message:            fmt.Sprintf("error reconciling resource: %v", istioNotFound),
						ObservedGeneration: generation,
					},
					noConflictCondition,
				},
			},
		},
		"someone else has modified telemetry object - should report conflict": {
			withCollector: true,
			targetRefs:    []v1alpha1.TargetReference{{Kind: v1.IstioKind, Name: "default"}},
			existingObjects: []client.Object{
				&v1.Istio{
					ObjectMeta: metav1.ObjectMeta{
						Name: "default",
						// Without setting the managed fields entry, the fake client's
						// SSA implementation will default to one. In this case it will
						// mean the conflict status will report a conflict for this Istio
						// and not the Telemetry which is what we want to test.
						ManagedFields: []metav1.ManagedFieldsEntry{
							managedFieldsEntry(fieldManager, v1.GroupVersion.String(), `{"f:spec":{"f:values":{}}}`),
						},
					},
					Spec: v1.IstioSpec{Namespace: istioNamespace},
				},
				// A Telemetry whose tracing config is owned by another field manager,
				// so that applying our own tracing config conflicts with it.
				&telemetryv1.Telemetry{
					ObjectMeta: metav1.ObjectMeta{
						Name:      telemetryName,
						Namespace: istioNamespace,
						ManagedFields: []metav1.ManagedFieldsEntry{
							managedFieldsEntry(otherFieldManager, telemetryv1.SchemeGroupVersion.String(), `{"f:spec":{"f:tracing":{}}}`),
						},
					},
					Spec: telemetryapiv1.Telemetry{
						Tracing: []*telemetryapiv1.Tracing{{
							Providers: []*telemetryapiv1.ProviderRef{{Name: "some-other-collector"}},
						}},
					},
				},
			},
			expectedStatus: v1alpha1.TracingIntegrationStatus{
				ObservedGeneration: generation,
				State:              v1alpha1.TracingIntegrationReasonApplyConflict,
				Conditions: []metav1.Condition{
					{
						Type:               string(v1alpha1.TracingIntegrationConditionReconciled),
						Status:             metav1.ConditionTrue,
						Reason:             string(v1alpha1.TracingIntegrationConditionReconciled),
						ObservedGeneration: generation,
					},
					{
						Type:   string(v1alpha1.TracingIntegrationConditionConflicted),
						Status: metav1.ConditionTrue,
						Reason: string(v1alpha1.TracingIntegrationReasonApplyConflict),
						Message: fmt.Sprintf("server-side apply conflict: Apply failed with 1 conflict: conflict with %q: .spec.tracing",
							otherFieldManager),
						ObservedGeneration: generation,
					},
				},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			integration := &v1alpha1.TracingIntegration{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Generation: generation},
				Spec: v1alpha1.TracingIntegrationSpec{
					TargetRefs:    tc.targetRefs,
					TelemetryName: telemetryName,
					TracingConfig: v1alpha1.TracingConfig{
						Type: v1alpha1.TracingTypeOpenTelemetry,
						OpenTelemetry: &v1alpha1.OpenTelemetryConfig{
							OTELCollectorRef: v1alpha1.NamespacedReference{Name: "collector", Namespace: istioNamespace},
						},
					},
				},
			}
			objects := []client.Object{integration}
			objects = append(objects, tc.existingObjects...)
			if tc.withCollector {
				objects = append(objects, &otelv1beta1.OpenTelemetryCollector{
					ObjectMeta: metav1.ObjectMeta{Name: "collector", Namespace: istioNamespace},
					Spec: otelv1beta1.OpenTelemetryCollectorSpec{
						Config: otelv1beta1.Config{
							Receivers: otelv1beta1.AnyConfig{
								Object: map[string]any{
									"otlp": map[string]any{
										"protocols": map[string]any{
											"grpc": map[string]any{"endpoint": "${env:MY_POD_IP}:4317"},
										},
									},
								},
							},
						},
					},
				})
			}
			cl := fake.NewClientBuilder().
				WithScheme(scheme.Scheme).
				WithStatusSubresource(&v1alpha1.TracingIntegration{}).
				WithObjects(objects...).
				WithInterceptorFuncs(tc.interceptors).
				Build()

			_, err := NewTracingReconciler(config.ReconcilerConfig{}, cl, scheme.Scheme).Reconcile(t.Context(), integration)
			switch {
			case tc.wantValidationError && !reconciler.IsValidationError(err):
				t.Fatalf("Reconcile() error = %v; want validation error", err)
			case tc.wantError == "" && err != nil:
				t.Fatalf("Reconcile() error = %v; want nil", err)
			case tc.wantError != "" && (err == nil || !strings.Contains(err.Error(), tc.wantError)):
				t.Fatalf("Reconcile() error = %v; want it to contain %q", err, tc.wantError)
			}

			actual := &v1alpha1.TracingIntegration{}
			if err := cl.Get(t.Context(), client.ObjectKeyFromObject(integration), actual); err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tc.expectedStatus, clearTimestamps(actual.Status)); diff != "" {
				t.Errorf("status wasn't as expected; diff (-expected, +actual):\n%v", diff)
			}
		})
	}
}

// TestMapTelemetryToReconcileRequest covers the watch that lets the controller notice when a
// user or another controller writes to the Telemetry it manages. The Telemetry has no owner
// reference, so it has to be matched by name.
func TestMapTelemetryToReconcileRequest(t *testing.T) {
	integration := &v1alpha1.TracingIntegration{
		Name: "test",
		Spec: v1alpha1.TracingIntegrationSpec{TelemetryName: telemetryName},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(integration).Build()
	reconciler := NewTracingReconciler(config.ReconcilerConfig{}, cl, scheme.Scheme)

	for name, tc := range map[string]struct {
		telemetryName string
		want          []reconcile.Request
	}{
		"managed telemetry": {
			telemetryName: telemetryName,
			want:          []reconcile.Request{{NamespacedName: client.ObjectKey{Name: "test"}}},
		},
		"unrelated telemetry": {
			telemetryName: "other",
			want:          []reconcile.Request{},
		},
	} {
		t.Run(name, func(t *testing.T) {
			telemetry := &telemetryv1.Telemetry{Name: tc.telemetryName, Namespace: istioNamespace}
			got := reconciler.mapTelemetryToReconcileRequest(t.Context(), telemetry)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("requests weren't as expected; diff (-expected, +actual):\n%v", diff)
			}
		})
	}
}

// managedFieldsEntry returns an apply entry that makes manager the owner of the fields in fieldsV1.
func managedFieldsEntry(manager, apiVersion, fieldsV1 string) metav1.ManagedFieldsEntry {
	return metav1.ManagedFieldsEntry{
		Manager:    manager,
		Operation:  metav1.ManagedFieldsOperationApply,
		APIVersion: apiVersion,
		FieldsType: "FieldsV1",
		FieldsV1:   metav1.NewFieldsV1(fieldsV1),
	}
}

func clearTimestamps(status v1alpha1.TracingIntegrationStatus) v1alpha1.TracingIntegrationStatus {
	for i := range status.Conditions {
		status.Conditions[i].LastTransitionTime = metav1.Time{}
	}
	return status
}
