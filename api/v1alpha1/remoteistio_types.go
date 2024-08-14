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

const RemoteIstioKind = "RemoteIstio"

// RemoteIstioSpec defines the desired state of RemoteIstio
// +kubebuilder:validation:XValidation:rule="!has(self.values) || !has(self.values.global) || !has(self.values.global.istioNamespace) || self.values.global.istioNamespace == self.__namespace__",message="spec.values.global.istioNamespace must match spec.namespace"
type RemoteIstioSpec struct {
	// +sail:version
	// Defines the version of Istio to install.
	// Must be one of: v1.23.0-rc.1, v1.22.3, v1.21.5, latest.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,order=1,displayName="Istio Version",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:General", "urn:alm:descriptor:com.tectonic.ui:select:v1.23.0-rc.1", "urn:alm:descriptor:com.tectonic.ui:select:v1.22.3", "urn:alm:descriptor:com.tectonic.ui:select:v1.21.5", "urn:alm:descriptor:com.tectonic.ui:select:latest"}
	// +kubebuilder:validation:Enum=v1.23.0-rc.1;v1.22.3;v1.21.5;latest
	// +kubebuilder:default=v1.23.0-rc.1
	Version string `json:"version"`

	// Defines the update strategy to use when the version in the RemoteIstio CR is updated.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Update Strategy"
	// +kubebuilder:default={type: "InPlace"}
	UpdateStrategy *IstioUpdateStrategy `json:"updateStrategy,omitempty"`

	// +sail:profile
	// The built-in installation configuration profile to use.
	// The 'default' profile is always applied. On OpenShift, the 'openshift' profile is also applied on top of 'default'.
	// Must be one of: ambient, default, demo, empty, external, openshift-ambient, openshift, preview, stable.
	// +++PROFILES-DROPDOWN-HIDDEN-UNTIL-WE-FULLY-IMPLEMENT-THEM+++operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Profile",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:fieldGroup:General", "urn:alm:descriptor:com.tectonic.ui:select:ambient", "urn:alm:descriptor:com.tectonic.ui:select:default", "urn:alm:descriptor:com.tectonic.ui:select:demo", "urn:alm:descriptor:com.tectonic.ui:select:empty", "urn:alm:descriptor:com.tectonic.ui:select:external", "urn:alm:descriptor:com.tectonic.ui:select:minimal", "urn:alm:descriptor:com.tectonic.ui:select:preview", "urn:alm:descriptor:com.tectonic.ui:select:remote"}
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:hidden"}
	// +kubebuilder:validation:Enum=ambient;default;demo;empty;external;openshift-ambient;openshift;preview;stable
	Profile string `json:"profile,omitempty"`

	// Namespace to which the Istio components should be installed.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:io.kubernetes:Namespace"}
	// +kubebuilder:default=istio-system
	Namespace string `json:"namespace"`

	// Defines the values to be passed to the Helm charts when installing Istio.
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Helm Values"
	Values *Values `json:"values,omitempty"`
}

// RemoteIstioStatus defines the observed state of RemoteIstio
type RemoteIstioStatus struct {
	// ObservedGeneration is the most recent generation observed for this
	// RemoteIstio object. It corresponds to the object's generation, which is
	// updated on mutation by the API Server. The information in the status
	// pertains to this particular generation of the object.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Represents the latest available observations of the object's current state.
	Conditions []RemoteIstioCondition `json:"conditions,omitempty"`

	// Reports the current state of the object.
	State RemoteIstioConditionReason `json:"state,omitempty"`

	// Reports information about the underlying IstioRevisions.
	Revisions RevisionSummary `json:"revisions,omitempty"`
}

// GetCondition returns the condition of the specified type
func (s *RemoteIstioStatus) GetCondition(conditionType RemoteIstioConditionType) RemoteIstioCondition {
	if s != nil {
		for i := range s.Conditions {
			if s.Conditions[i].Type == conditionType {
				return s.Conditions[i]
			}
		}
	}
	return RemoteIstioCondition{Type: conditionType, Status: metav1.ConditionUnknown}
}

// SetCondition sets a specific condition in the list of conditions
func (s *RemoteIstioStatus) SetCondition(condition RemoteIstioCondition) {
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

// RemoteIstioCondition represents a specific observation of the RemoteIstioCondition object's state.
type RemoteIstioCondition struct {
	// The type of this condition.
	Type RemoteIstioConditionType `json:"type,omitempty"`

	// The status of this condition. Can be True, False or Unknown.
	Status metav1.ConditionStatus `json:"status,omitempty"`

	// Unique, single-word, CamelCase reason for the condition's last transition.
	Reason RemoteIstioConditionReason `json:"reason,omitempty"`

	// Human-readable message indicating details about the last transition.
	Message string `json:"message,omitempty"`

	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}

// RemoteIstioConditionType represents the type of the condition.  Condition stages are:
// Installed, Reconciled, Ready
type RemoteIstioConditionType string

// RemoteIstioConditionReason represents a short message indicating how the condition came
// to be in its present state.
type RemoteIstioConditionReason string

const (
	// RemoteIstioConditionReconciled signifies whether the controller has
	// successfully reconciled the resources defined through the CR.
	RemoteIstioConditionReconciled RemoteIstioConditionType = "Reconciled"

	// RemoteIstioReasonReconcileError indicates that the reconciliation of the resource has failed, but will be retried.
	RemoteIstioReasonReconcileError RemoteIstioConditionReason = "ReconcileError"
)

const (
	// RemoteIstioConditionReady signifies whether any Deployment, StatefulSet,
	// etc. resources are Ready.
	RemoteIstioConditionReady RemoteIstioConditionType = "Ready"

	// RemoteIstioReasonRevisionNotFound indicates that the active IstioRevision is not found.
	RemoteIstioReasonRevisionNotFound RemoteIstioConditionReason = "ActiveRevisionNotFound"

	// RemoteIstioReasonFailedToGetActiveRevision indicates that a failure occurred when getting the active IstioRevision
	RemoteIstioReasonFailedToGetActiveRevision RemoteIstioConditionReason = "FailedToGetActiveRevision"

	// RemoteIstioReasonIstiodNotReady indicates that the control plane is fully reconciled, but istiod is not ready.
	RemoteIstioReasonIstiodNotReady RemoteIstioConditionReason = "IstiodNotReady"

	// RemoteIstioReasonReadinessCheckFailed indicates that readiness could not be ascertained.
	RemoteIstioReasonReadinessCheckFailed RemoteIstioConditionReason = "ReadinessCheckFailed"
)

const (
	// RemoteIstioReasonHealthy indicates that the control plane is fully reconciled and that all components are ready.
	RemoteIstioReasonHealthy RemoteIstioConditionReason = "Healthy"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,categories=istio-io
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Revisions",type="string",JSONPath=".status.revisions.total",description="Total number of IstioRevision objects currently associated with this object."
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.revisions.ready",description="Number of revisions that are ready."
// +kubebuilder:printcolumn:name="In use",type="string",JSONPath=".status.revisions.inUse",description="Number of revisions that are currently being used by workloads."
// +kubebuilder:printcolumn:name="Active Revision",type="string",JSONPath=".status.state",description="The current state of the active revision."
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.version",description="The version of the control plane installation."
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="The age of the object"

// RemoteIstio represents a remote Istio Service Mesh deployment consisting of one or more
// remote control plane instances (represented by one or more IstioRevision objects).
type RemoteIstio struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:default={version: "v1.23.0-rc.1", namespace: "istio-system", updateStrategy: {type:"InPlace"}}
	Spec RemoteIstioSpec `json:"spec,omitempty"`

	Status RemoteIstioStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RemoteIstioList contains a list of RemoteIstio
type RemoteIstioList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RemoteIstio `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RemoteIstio{}, &RemoteIstioList{})
}
