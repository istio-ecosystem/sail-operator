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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ZTunnelKind = "ZTunnel"
)

// ZTunnelSpec defines the desired state of ZTunnel
type ZTunnelSpec struct {
	// +sail:version
	// Defines the version of Istio to install.
	// Must be one of: v1.28-latest, v1.28.3, v1.28.2, v1.28.1, v1.28.0, v1.27-latest, v1.27.5, v1.27.4, v1.27.3, v1.27.2, v1.27.1, v1.27.0, master, v1.30-alpha.d9e77b1d.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,order=1,displayName="Istio Version",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:General", "urn:alm:descriptor:com.tectonic.ui:select:v1.28-latest", "urn:alm:descriptor:com.tectonic.ui:select:v1.28.3", "urn:alm:descriptor:com.tectonic.ui:select:v1.28.2", "urn:alm:descriptor:com.tectonic.ui:select:v1.28.1", "urn:alm:descriptor:com.tectonic.ui:select:v1.28.0", "urn:alm:descriptor:com.tectonic.ui:select:v1.27-latest", "urn:alm:descriptor:com.tectonic.ui:select:v1.27.5", "urn:alm:descriptor:com.tectonic.ui:select:v1.27.4", "urn:alm:descriptor:com.tectonic.ui:select:v1.27.3", "urn:alm:descriptor:com.tectonic.ui:select:v1.27.2", "urn:alm:descriptor:com.tectonic.ui:select:v1.27.1", "urn:alm:descriptor:com.tectonic.ui:select:v1.27.0", "urn:alm:descriptor:com.tectonic.ui:select:master", "urn:alm:descriptor:com.tectonic.ui:select:v1.30-alpha.d9e77b1d"}
	// +kubebuilder:validation:Enum=v1.28-latest;v1.28.3;v1.28.2;v1.28.1;v1.28.0;v1.27-latest;v1.27.5;v1.27.4;v1.27.3;v1.27.2;v1.27.1;v1.27.0;v1.26-latest;v1.26.8;v1.26.7;v1.26.6;v1.26.5;v1.26.4;v1.26.3;v1.26.2;v1.26.1;v1.26.0;v1.25-latest;v1.25.5;v1.25.4;v1.25.3;v1.25.2;v1.25.1;v1.24-latest;v1.24.6;v1.24.5;v1.24.4;v1.24.3;v1.24.2;v1.24.1;v1.24.0;master;v1.30-alpha.d9e77b1d
	// +kubebuilder:default=v1.28.3
	Version string `json:"version"`

	// Namespace to which the Istio ztunnel component should be installed.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:io.kubernetes:Namespace"}
	// +kubebuilder:default=ztunnel
	Namespace string `json:"namespace"`

	// Defines the values to be passed to the Helm charts when installing Istio ztunnel.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Helm Values"
	Values *ZTunnelValues `json:"values,omitempty"`
}

// ZTunnelStatus defines the observed state of ZTunnel
type ZTunnelStatus struct {
	// ObservedGeneration is the most recent generation observed for this
	// ZTunnel object. It corresponds to the object's generation, which is
	// updated on mutation by the API Server. The information in the status
	// pertains to this particular generation of the object.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Represents the latest available observations of the object's current state.
	Conditions []ZTunnelCondition `json:"conditions,omitempty"`

	// Reports the current state of the object.
	State ZTunnelConditionReason `json:"state,omitempty"`
}

// GetCondition returns the condition of the specified type
func (s *ZTunnelStatus) GetCondition(conditionType ZTunnelConditionType) ZTunnelCondition {
	if s != nil {
		for i := range s.Conditions {
			if s.Conditions[i].Type == conditionType {
				return s.Conditions[i]
			}
		}
	}
	return ZTunnelCondition{Type: conditionType, Status: metav1.ConditionUnknown}
}

// SetCondition sets a specific condition in the list of conditions
func (s *ZTunnelStatus) SetCondition(condition ZTunnelCondition) {
	var now time.Time
	if testTime == nil {
		now = time.Now()
	} else {
		now = *testTime
	}

	// The lastTransitionTime only gets serialized out to the second.  This can
	// break update skipping, as the time in the resource returned from the client
	// may not match the time in our cached status during a reconcile.  We truncate
	// here to save any problems down the line.
	lastTransitionTime := metav1.NewTime(now.Truncate(time.Second))

	for i, prevCondition := range s.Conditions {
		if prevCondition.Type == condition.Type {
			if prevCondition.Status != condition.Status {
				condition.LastTransitionTime = lastTransitionTime
			} else {
				condition.LastTransitionTime = prevCondition.LastTransitionTime
			}
			s.Conditions[i] = condition
			return
		}
	}

	// If the condition does not exist, initialize the lastTransitionTime
	condition.LastTransitionTime = lastTransitionTime
	s.Conditions = append(s.Conditions, condition)
}

// ZTunnelCondition represents a specific observation of the ZTunnel object's state.
type ZTunnelCondition struct {
	// The type of this condition.
	Type ZTunnelConditionType `json:"type,omitempty"`

	// The status of this condition. Can be True, False or Unknown.
	Status metav1.ConditionStatus `json:"status,omitempty"`

	// Unique, single-word, CamelCase reason for the condition's last transition.
	Reason ZTunnelConditionReason `json:"reason,omitempty"`

	// Human-readable message indicating details about the last transition.
	Message string `json:"message,omitempty"`

	// Last time the condition transitioned from one status to another.
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitzero"`
}

// ZTunnelConditionType represents the type of the condition.  Condition stages are:
// Installed, Reconciled, Ready
type ZTunnelConditionType string

// ZTunnelConditionReason represents a short message indicating how the condition came
// to be in its present state.
type ZTunnelConditionReason string

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
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="The age of the object"
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'default'",message="metadata.name must be 'default'"

// ZTunnel represents a deployment of the Istio ztunnel component.
type ZTunnel struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata"`

	// +kubebuilder:default={version: "v1.28.3", namespace: "ztunnel"}
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
