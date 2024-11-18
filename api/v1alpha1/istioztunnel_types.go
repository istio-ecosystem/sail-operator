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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)


const (
	IstioZtunnelKind = "IstioZtunnel"
)

// IstioZtunnelSpec defines the desired state of IstioZtunnel
type IstioZtunnelSpec struct {
	// +sail:version
	// Defines the version of Istio to install.
	// Must be one of: v1.24.0 or latest.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,order=1,displayName="Istio Version",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:General", "urn:alm:descriptor:com.tectonic.ui:select:v1.24.0", "urn:alm:descriptor:com.tectonic.ui:select:latest"}
	// +kubebuilder:validation:Enum=v1.24.0;latest
	// +kubebuilder:default=v1.24.0
	Version string `json:"version"`

	// +sail:profile
	// The built-in installation configuration profile to use.
	// The 'default' profile is always applied. On OpenShift, the 'openshift' profile is also applied on top of 'default'.
	// Must be one of: ambient, default, demo, empty, external, openshift-ambient, openshift, preview, remote, stable.
	// +++PROFILES-DROPDOWN-HIDDEN-UNTIL-WE-FULLY-IMPLEMENT-THEM+++operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Profile",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:General", "urn:alm:descriptor:com.tectonic.ui:select:ambient", "urn:alm:descriptor:com.tectonic.ui:select:default", "urn:alm:descriptor:com.tectonic.ui:select:demo", "urn:alm:descriptor:com.tectonic.ui:select:empty", "urn:alm:descriptor:com.tectonic.ui:select:external", "urn:alm:descriptor:com.tectonic.ui:select:minimal", "urn:alm:descriptor:com.tectonic.ui:select:preview", "urn:alm:descriptor:com.tectonic.ui:select:remote"}
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	// +kubebuilder:validation:Enum=ambient;default;demo;empty;external;openshift-ambient;openshift;preview;remote;stable
	Profile string `json:"profile,omitempty"`

	// Namespace to which the Istio ztunnel component should be installed.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:io.kubernetes:Namespace"}
	// +kubebuilder:default=kube-system
	Namespace string `json:"namespace"`

	// Defines the values to be passed to the Helm charts when installing Istio ztunnel.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Helm Values"
	Values *ZtunnelValues `json:"values,omitempty"`
}

// IstioZtunnelStatus defines the observed state of IstioZtunnel
type IstioZtunnelStatus struct {
	// ObservedGeneration is the most recent generation observed for this
	// IstioZtunnel object. It corresponds to the object's generation, which is
	// updated on mutation by the API Server. The information in the status
	// pertains to this particular generation of the object.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Represents the latest available observations of the object's current state.
	Conditions []IstioZtunnelCondition `json:"conditions,omitempty"`

	// Reports the current state of the object.
	State IstioZtunnelConditionReason `json:"state,omitempty"`
}

// GetCondition returns the condition of the specified type
func (s *IstioZtunnelStatus) GetCondition(conditionType IstioZtunnelConditionType) IstioZtunnelCondition {
	if s != nil {
		for i := range s.Conditions {
			if s.Conditions[i].Type == conditionType {
				return s.Conditions[i]
			}
		}
	}
	return IstioZtunnelCondition{Type: conditionType, Status: metav1.ConditionUnknown}
}

// SetCondition sets a specific condition in the list of conditions
func (s *IstioZtunnelStatus) SetCondition(condition IstioZtunnelCondition) {
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

// IstioZtunnelCondition represents a specific observation of the IstioZtunnel object's state.
type IstioZtunnelCondition struct {
	// The type of this condition.
	Type IstioZtunnelConditionType `json:"type,omitempty"`

	// The status of this condition. Can be True, False or Unknown.
	Status metav1.ConditionStatus `json:"status,omitempty"`

	// Unique, single-word, CamelCase reason for the condition's last transition.
	Reason IstioZtunnelConditionReason `json:"reason,omitempty"`

	// Human-readable message indicating details about the last transition.
	Message string `json:"message,omitempty"`

	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}

// IstioZtunnelConditionType represents the type of the condition.  Condition stages are:
// Installed, Reconciled, Ready
type IstioZtunnelConditionType string

// IstioZtunnelConditionReason represents a short message indicating how the condition came
// to be in its present state.
type IstioZtunnelConditionReason string

const (
	// IstioZtunnelConditionReconciled signifies whether the controller has
	// successfully reconciled the resources defined through the CR.
	IstioZtunnelConditionReconciled IstioZtunnelConditionType = "Reconciled"

	// IstioZtunnelReasonReconcileError indicates that the reconciliation of the resource has failed, but will be retried.
	IstioZtunnelReasonReconcileError IstioZtunnelConditionReason = "ReconcileError"
)

const (
	// IstioZtunnelConditionReady signifies whether the ztunnel DaemonSet is ready.
	IstioZtunnelConditionReady IstioZtunnelConditionType = "Ready"

	// IstioZtunnelDaemonSetNotReady indicates that the ztunnel DaemonSet is not ready.
	IstioZtunnelDaemonSetNotReady IstioZtunnelConditionReason = "DaemonSetNotReady"

	// IstioZtunnelReasonReadinessCheckFailed indicates that the DaemonSet readiness status could not be ascertained.
	IstioZtunnelReasonReadinessCheckFailed IstioZtunnelConditionReason = "ReadinessCheckFailed"
)

const (
	// IstioZtunnelReasonHealthy indicates that the control plane is fully reconciled and that all components are ready.
	IstioZtunnelReasonHealthy IstioZtunnelConditionReason = "Healthy"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,categories=istio-io
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"Ready\")].status",description="Whether the Istio ztunnel installation is ready to handle requests."
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.state",description="The current state of this object."
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.version",description="The version of the Istio ztunnel installation."
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="The age of the object"
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'default'",message="metadata.name must be 'default'"

// IstioZtunnel represents a deployment of the Istio ztunnel component.
type IstioZtunnel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:default={version: "v1.24.0", namespace: "kube-system"}
	Spec IstioZtunnelSpec `json:"spec,omitempty"`

	Status IstioZtunnelStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// IstioZtunnelList contains a list of IstioZtunnel
type IstioZtunnelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IstioZtunnel `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IstioZtunnel{}, &IstioZtunnelList{})
}
