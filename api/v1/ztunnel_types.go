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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ZTunnelKind = "ZTunnel"
)

// ZTunnelSpec defines the desired state of ZTunnel
type ZTunnelSpec struct {
	// +sail:version
	// Defines the version of Istio to install.
	// Must be one of: v1.30-latest, v1.30.0-alpha.2, v1.29-latest, v1.29.2, v1.29.1, v1.29.0, v1.28-latest, v1.28.6, v1.28.5, v1.28.4, v1.28.3, v1.28.2, v1.28.1, v1.28.0, master, v1.31.0-alpha.30efb2b4.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,order=1,displayName="Istio Version",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:General", "urn:alm:descriptor:com.tectonic.ui:select:v1.30-latest", "urn:alm:descriptor:com.tectonic.ui:select:v1.30.0-alpha.2", "urn:alm:descriptor:com.tectonic.ui:select:v1.29-latest", "urn:alm:descriptor:com.tectonic.ui:select:v1.29.2", "urn:alm:descriptor:com.tectonic.ui:select:v1.29.1", "urn:alm:descriptor:com.tectonic.ui:select:v1.29.0", "urn:alm:descriptor:com.tectonic.ui:select:v1.28-latest", "urn:alm:descriptor:com.tectonic.ui:select:v1.28.6", "urn:alm:descriptor:com.tectonic.ui:select:v1.28.5", "urn:alm:descriptor:com.tectonic.ui:select:v1.28.4", "urn:alm:descriptor:com.tectonic.ui:select:v1.28.3", "urn:alm:descriptor:com.tectonic.ui:select:v1.28.2", "urn:alm:descriptor:com.tectonic.ui:select:v1.28.1", "urn:alm:descriptor:com.tectonic.ui:select:v1.28.0", "urn:alm:descriptor:com.tectonic.ui:select:master", "urn:alm:descriptor:com.tectonic.ui:select:v1.31.0-alpha.30efb2b4"}
	// +kubebuilder:validation:Enum=v1.30-latest;v1.30.0-alpha.2;v1.29-latest;v1.29.2;v1.29.1;v1.29.0;v1.28-latest;v1.28.6;v1.28.5;v1.28.4;v1.28.3;v1.28.2;v1.28.1;v1.28.0;v1.27-latest;v1.27.9;v1.27.8;v1.27.7;v1.27.6;v1.27.5;v1.27.4;v1.27.3;v1.27.2;v1.27.1;v1.27.0;v1.26-latest;v1.26.8;v1.26.7;v1.26.6;v1.26.5;v1.26.4;v1.26.3;v1.26.2;v1.26.1;v1.26.0;v1.25-latest;v1.25.5;v1.25.4;v1.25.3;v1.25.2;v1.25.1;v1.24-latest;v1.24.6;v1.24.5;v1.24.4;v1.24.3;v1.24.2;v1.24.1;v1.24.0;master;v1.31.0-alpha.30efb2b4
	// +kubebuilder:default=v1.30.0-alpha.2
	Version string `json:"version"`

	// Namespace to which the Istio ztunnel component should be installed.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:io.kubernetes:Namespace"}
	// +kubebuilder:default=ztunnel
	Namespace string `json:"namespace"`

	// Defines the values to be passed to the Helm charts when installing Istio ztunnel.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Helm Values"
	Values *ZTunnelValues `json:"values,omitempty"`

	// The Istio control plane that this ZTunnel instance is associated with. Valid references are Istio and IstioRevision resources, Istio resources are always resolved to their current active revision.
	// Values relevant for ZTunnel will be copied from the referenced IstioRevision resource, these are `spec.values.global`, `spec.values.meshConfig`, `spec.values.revision`. Any user configuration in the ZTunnel spec will always take precedence over the settings copied from the Istio resource, however.
	TargetRef *TargetReference `json:"targetRef,omitempty"`
}

// ZTunnelStatus defines the observed state of ZTunnel
type ZTunnelStatus struct {
	// ObservedGeneration is the most recent generation observed for this
	// ZTunnel object. It corresponds to the object's generation, which is
	// updated on mutation by the API Server. The information in the status
	// pertains to this particular generation of the object.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Represents the latest available observations of the object's current state.
	Conditions []StatusCondition `json:"conditions,omitempty"`

	// Reports the current state of the object.
	State ZTunnelConditionReason `json:"state,omitempty"`

	// IstioRevision stores the name of the referenced IstioRevision
	IstioRevision string `json:"istioRevision,omitempty"`
}

// GetCondition returns the condition of the specified type
func (s *ZTunnelStatus) GetCondition(conditionType ZTunnelConditionType) StatusCondition {
	if s == nil {
		return StatusCondition{Type: conditionType, Status: metav1.ConditionUnknown}
	}
	return GetCondition(s.Conditions, conditionType)
}

// SetCondition sets a specific condition in the list of conditions
func (s *ZTunnelStatus) SetCondition(condition StatusCondition) {
	SetCondition(&s.Conditions, condition)
}

// ZTunnelConditionType is an alias for ConditionType.
type ZTunnelConditionType = ConditionType

// ZTunnelConditionReason is an alias for ConditionReason.
type ZTunnelConditionReason = ConditionReason

const (
	// ZTunnelConditionReconciled signifies whether the controller has
	// successfully reconciled the resources defined through the CR.
	ZTunnelConditionReconciled ZTunnelConditionType = "Reconciled"

	// ZTunnelReasonReconcileError indicates that the reconciliation of the resource has failed, but will be retried.
	ZTunnelReasonReconcileError ZTunnelConditionReason = "ReconcileError"
)

const (
	// ZTunnelConditionReady signifies whether the ztunnel DaemonSet is ready.
	ZTunnelConditionReady ZTunnelConditionType = "Ready"

	// ZTunnelDaemonSetNotReady indicates that the ztunnel DaemonSet is not ready.
	ZTunnelDaemonSetNotReady ZTunnelConditionReason = "DaemonSetNotReady"

	// ZTunnelReasonReadinessCheckFailed indicates that the DaemonSet readiness status could not be ascertained.
	ZTunnelReasonReadinessCheckFailed ZTunnelConditionReason = "ReadinessCheckFailed"
)

const (
	// ZTunnelReasonHealthy indicates that the control plane is fully reconciled and that all components are ready.
	ZTunnelReasonHealthy ZTunnelConditionReason = "Healthy"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,categories=istio-io
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Namespace",type="string",JSONPath=".spec.namespace",description="The namespace for the ztunnel component."
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description="Whether the Istio ztunnel installation is ready to handle requests."
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.state",description="The current state of this object."
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.version",description="The version of the Istio ztunnel installation."
// +kubebuilder:printcolumn:name="Revision",type="string",JSONPath=".status.istioRevision",description="The referenced IstioRevision."
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="The age of the object"
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'default'",message="metadata.name must be 'default'"

// ZTunnel represents a deployment of the Istio ztunnel component.
type ZTunnel struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata"`

	// +kubebuilder:default={version: "v1.30.0-alpha.2", namespace: "ztunnel"}
	// +optional
	Spec ZTunnelSpec `json:"spec"`

	// +optional
	Status ZTunnelStatus `json:"status"`
}

// +kubebuilder:object:root=true

// ZTunnelList contains a list of ZTunnel
type ZTunnelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []ZTunnel `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ZTunnel{}, &ZTunnelList{})
}
