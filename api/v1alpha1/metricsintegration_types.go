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
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	MetricsIntegrationKind = "MetricsIntegration"
)

// TargetReference identifies a resource that the integration configures.
type TargetReference struct {
	// Kind is the kind of the target resource.
	//
	// +kubebuilder:validation:Enum=Istio;Kiali;Perses
	// +kubebuilder:validation:Required
	Kind string `json:"kind"`

	// Name is the name of the target resource.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace is the namespace of the target resource.
	// Only required for namespace-scoped resources like Kiali.
	//
	// +kubebuilder:validation:MaxLength=63
	Namespace string `json:"namespace,omitempty"`
}

// NamespacedReference references a namespaced Kubernetes object.
type NamespacedReference struct {
	// Name is the name of the referenced object.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace is the namespace of the referenced object.
	//
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`
}

// MetricsType identifies the type of metrics integration.
//
// +kubebuilder:validation:Enum=UserWorkloadMonitoring;ClusterObservabilityOperator
type MetricsType string

const (
	// MetricsTypeUserWorkloadMonitoring integrates Istio with OpenShift User Workload Monitoring.
	MetricsTypeUserWorkloadMonitoring MetricsType = "UserWorkloadMonitoring"
	// MetricsTypeClusterObservabilityOperator integrates Istio with a Cluster Observability Operator MonitoringStack.
	MetricsTypeClusterObservabilityOperator MetricsType = "ClusterObservabilityOperator"
)

// MetricsConfig configures a metrics backend.
type MetricsConfig struct {
	// Type specifies the metrics integration type.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,order=1,displayName="Type",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:General", "urn:alm:descriptor:com.tectonic.ui:select:UserWorkloadMonitoring", "urn:alm:descriptor:com.tectonic.ui:select:ClusterObservabilityOperator"}
	// +kubebuilder:validation:Required
	Type MetricsType `json:"type"`

	// UserWorkloadMonitoring configures integration with OpenShift User Workload Monitoring.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="User Workload Monitoring"
	UserWorkloadMonitoring *UserWorkloadMonitoringConfig `json:"userWorkloadMonitoring,omitempty"`

	// ClusterObservabilityOperator configures integration with the Cluster Observability
	// Operator's MonitoringStack resource for metrics collection.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Cluster Observability Operator"
	ClusterObservabilityOperator *ClusterObservabilityOperatorConfig `json:"clusterObservabilityOperator,omitempty"`
}

// UserWorkloadMonitoringConfig configures the OpenShift User Workload Monitoring integration.
type UserWorkloadMonitoringConfig struct{}

// ClusterObservabilityOperatorConfig configures the Cluster Observability Operator integration.
type ClusterObservabilityOperatorConfig struct {
	// MonitoringStackRef is a reference to a MonitoringStack resource that defines
	// the Prometheus stack used for scraping Istio metrics.
	//
	// +kubebuilder:validation:Required
	MonitoringStackRef NamespacedReference `json:"monitoringStackRef"`
}

// MetricsIntegrationSpec defines the desired state of MetricsIntegration
//
// +kubebuilder:validation:XValidation:rule="self.type == 'UserWorkloadMonitoring' ? has(self.userWorkloadMonitoring) : true",message="userWorkloadMonitoring is required when type is UserWorkloadMonitoring"
// +kubebuilder:validation:XValidation:rule="self.type == 'ClusterObservabilityOperator' ? has(self.clusterObservabilityOperator) : true",message="clusterObservabilityOperator is required when type is ClusterObservabilityOperator"
// +kubebuilder:validation:XValidation:rule="self.type != 'UserWorkloadMonitoring' || !has(self.clusterObservabilityOperator)",message="clusterObservabilityOperator must not be set when type is UserWorkloadMonitoring"
// +kubebuilder:validation:XValidation:rule="self.type != 'ClusterObservabilityOperator' || !has(self.userWorkloadMonitoring)",message="userWorkloadMonitoring must not be set when type is ClusterObservabilityOperator"
type MetricsIntegrationSpec struct {
	// TargetRefs specifies the resources that this integration configures.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,order=2,displayName="Target References"
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:Required
	TargetRefs []TargetReference `json:"targetRefs"`

	MetricsConfig `json:",inline"`
}

// MetricsIntegrationStatus defines the observed state of MetricsIntegration
type MetricsIntegrationStatus struct {
	// ObservedGeneration is the most recent generation observed for this
	// MetricsIntegration object. It corresponds to the object's generation, which is
	// updated on mutation by the API Server. The information in the status
	// pertains to this particular generation of the object.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Represents the latest available observations of the object's current state.
	Conditions []v1.StatusCondition `json:"conditions,omitempty"`

	// Reports the current state of the object.
	State MetricsIntegrationConditionReason `json:"state,omitempty"`
}

// GetCondition returns the condition of the specified type
func (s *MetricsIntegrationStatus) GetCondition(conditionType MetricsIntegrationConditionType) v1.StatusCondition {
	if s != nil {
		return v1.GetCondition(s.Conditions, v1.ConditionType(conditionType))
	}
	return v1.StatusCondition{Type: v1.ConditionType(conditionType), Status: metav1.ConditionUnknown}
}

// SetCondition sets a specific condition in the list of conditions
func (s *MetricsIntegrationStatus) SetCondition(condition v1.StatusCondition) {
	v1.SetCondition(&s.Conditions, condition)
}

// MetricsIntegrationConditionType represents the type of a MetricsIntegration condition.
type MetricsIntegrationConditionType string

// MetricsIntegrationConditionReason represents the reason for a MetricsIntegration condition.
type MetricsIntegrationConditionReason string

const (
	// MetricsIntegrationConditionReconciled signifies whether the controller has
	// successfully reconciled the resources defined through the CR.
	MetricsIntegrationConditionReconciled MetricsIntegrationConditionType = "Reconciled"

	// MetricsIntegrationReasonReconcileError indicates that the reconciliation of the resource has failed, but will be retried.
	MetricsIntegrationReasonReconcileError MetricsIntegrationConditionReason = "ReconcileError"
)

const (
	// MetricsIntegrationReasonReferenceNotFound indicates that the resource referenced by the integration's TargetRefs was not found.
	MetricsIntegrationReasonReferenceNotFound MetricsIntegrationConditionReason = "RefNotFound"

	// MetricsIntegrationReasonInvalidSpec indicates that the spec is invalid.
	MetricsIntegrationReasonInvalidSpec MetricsIntegrationConditionReason = "InvalidSpec"

	// MetricsIntegrationReasonDuplicateTarget indicates that a MetricsIntegration already exists for one of the referenced resources.
	MetricsIntegrationReasonDuplicateTarget MetricsIntegrationConditionReason = "DuplicateTarget"

	// MetricsIntegrationReasonNotImplemented indicates that the requested integration type is not yet implemented.
	MetricsIntegrationReasonNotImplemented MetricsIntegrationConditionReason = "NotImplemented"
)

const (
	// MetricsIntegrationReasonHealthy indicates that the integration has been successfully reconciled.
	MetricsIntegrationReasonHealthy MetricsIntegrationConditionReason = "Healthy"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName=metricsint,categories=istio-io
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.type",description="The metrics integration type."
// +kubebuilder:printcolumn:name="Reconciled",type="string",JSONPath=".status.conditions[?(@.type==\"Reconciled\")].status",description="Whether the MetricsIntegration has been reconciled."
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.state",description="The current state of this object."
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="The age of the object"

// MetricsIntegration configures metrics collection integrations for Istio and related resources.
type MetricsIntegration struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata"`

	// +optional
	Spec MetricsIntegrationSpec `json:"spec"`

	// +optional
	Status MetricsIntegrationStatus `json:"status"`
}

// +kubebuilder:object:root=true

// MetricsIntegrationList contains a list of MetricsIntegrations
type MetricsIntegrationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []MetricsIntegration `json:"items"`
}
