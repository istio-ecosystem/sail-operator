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

package v1alpha1

import (
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const TracingIntegrationKind = "TracingIntegration"

// TargetReference identifies a resource that the integration configures.
type TargetReference struct {
	// Kind specifies the kind of resource (e.g. "Istio", "Kiali").
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	Kind string `json:"kind"`

	// Name is the name of the target resource.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Namespace is the namespace of the target resource.
	// Only required for namespace-scoped resources like Kiali.
	Namespace string `json:"namespace,omitempty"`
}

// NamespacedReference identifies a namespaced resource.
type NamespacedReference struct {
	// Name is the name of the referenced resource.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Namespace is the namespace of the referenced resource.
	// +kubebuilder:validation:MinLength=1
	Namespace string `json:"namespace"`
}

// TracingIntegrationSpec defines the desired state of TracingIntegration.
type TracingIntegrationSpec struct {
	// TargetRefs specifies the resources that this integration configures.
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=10
	// +kubebuilder:validation:XValidation:rule="self.all(ref, self.filter(other, other.kind == ref.kind).size() == 1)",message="targetRefs must not contain multiple targets of the same kind"
	TargetRefs []TargetReference `json:"targetRefs"`

	// TelemetryName specifies the name of the Istio Telemetry resource managed by this integration.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:default=mesh-default
	TelemetryName string `json:"telemetryName,omitempty"`

	TracingConfig `json:",inline"`
}

// TracingType identifies the type of tracing integration.
type TracingType string

const (
	// TracingTypeOpenTelemetry configures tracing through an OpenTelemetry Collector.
	TracingTypeOpenTelemetry TracingType = "OpenTelemetry"
)

// TracingConfig configures a tracing backend.
type TracingConfig struct {
	// Type specifies the tracing integration type.
	// +kubebuilder:validation:Enum=OpenTelemetry
	Type TracingType `json:"type"`

	// OpenTelemetry configures integration with an OpenTelemetry Collector.
	OpenTelemetry *OpenTelemetryConfig `json:"openTelemetry,omitempty"`
}

// OpenTelemetryConfig configures the OpenTelemetry integration.
type OpenTelemetryConfig struct {
	// OTELCollectorRef is a reference to an OpenTelemetry Collector resource.
	OTELCollectorRef NamespacedReference `json:"otelCollectorRef"`
}

// TracingIntegrationStatus defines the observed state of TracingIntegration.
type TracingIntegrationStatus struct {
	// ObservedGeneration is the most recent generation observed for this
	// TracingIntegration object.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Represents the latest available observations of the object's current state.
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Reports the current state of the object.
	State TracingIntegrationConditionReason `json:"state,omitempty"`
}

// GetCondition returns the condition of the specified type.
func (s *TracingIntegrationStatus) GetCondition(conditionType TracingIntegrationConditionType) metav1.Condition {
	if s != nil {
		for _, condition := range s.Conditions {
			if condition.Type == string(conditionType) {
				return condition
			}
		}
	}
	return metav1.Condition{Type: string(conditionType), Status: metav1.ConditionUnknown}
}

// SetCondition sets a specific condition in the list of conditions.
func (s *TracingIntegrationStatus) SetCondition(condition metav1.Condition) {
	apimeta.SetStatusCondition(&s.Conditions, condition)
}

// TracingIntegrationConditionType represents the type of a TracingIntegration condition.
type TracingIntegrationConditionType string

// TracingIntegrationConditionReason represents the reason for a TracingIntegration condition.
type TracingIntegrationConditionReason string

const (
	// TracingIntegrationConditionReconciled signifies whether the controller has
	// successfully reconciled the resources defined through the CR.
	TracingIntegrationConditionReconciled TracingIntegrationConditionType = "Reconciled"

	// TracingIntegrationConditionConflicted signifies whether server-side apply
	// reached a field ownership conflict while reconciling the integration.
	TracingIntegrationConditionConflicted TracingIntegrationConditionType = "Conflicted"

	// TracingIntegrationReasonReconcileError indicates that reconciliation failed.
	TracingIntegrationReasonReconcileError TracingIntegrationConditionReason = "ReconcileError"

	// TracingIntegrationReasonInvalidConfiguration occurs if there's a validation error.
	TracingIntegrationReasonInvalidConfiguration TracingIntegrationConditionReason = "InvalidConfiguration"

	// TracingIntegrationReasonApplyConflict indicates that reconciliation reached
	// a server-side apply field ownership conflict.
	TracingIntegrationReasonApplyConflict TracingIntegrationConditionReason = "ApplyConflict"

	// TracingIntegrationReasonNoConflict indicates that no server-side apply field
	// ownership conflict is currently observed.
	TracingIntegrationReasonNoConflict TracingIntegrationConditionReason = "NoConflict"

	// TracingIntegrationReasonHealthy indicates that the integration is fully reconciled.
	TracingIntegrationReasonHealthy TracingIntegrationConditionReason = "Healthy"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,categories=istio-io
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.type",description="The tracing integration type."
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Reconciled\")].status",description="Whether the tracing integration is reconciled."
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.state",description="The current state of this object."
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="The age of the object"
// +kubebuilder:validation:XValidation:rule="self.spec.type == 'OpenTelemetry' ? has(self.spec.openTelemetry) : !has(self.spec.openTelemetry)",message="spec.openTelemetry must be set if and only if spec.type is OpenTelemetry"

// TracingIntegration configures Istio tracing integrations.
type TracingIntegration struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata"`

	// +optional
	Spec TracingIntegrationSpec `json:"spec"`

	// +optional
	Status TracingIntegrationStatus `json:"status"`
}

// +kubebuilder:object:root=true

// TracingIntegrationList contains a list of TracingIntegration.
type TracingIntegrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []TracingIntegration `json:"items"`
}
