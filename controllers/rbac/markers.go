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

// Package rbac centralizes kubebuilder RBAC markers that are shared across
// multiple controllers or are required by startup-time platform detection.
package rbac

// +kubebuilder:rbac:groups="",resources=configmaps;endpoints;events;namespaces;nodes;persistentvolumeclaims,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="",resources=pods;replicationcontrollers;resourcequotas,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="",resources=secrets;serviceaccounts;services,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="admissionregistration.k8s.io",resources=mutatingwebhookconfigurations,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="admissionregistration.k8s.io",resources=validatingwebhookconfigurations,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="admissionregistration.k8s.io",resources=validatingadmissionpolicies,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="admissionregistration.k8s.io",resources=validatingadmissionpolicybindings,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="apps",resources=daemonsets;deployments,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="autoscaling",resources=horizontalpodautoscalers,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="config.openshift.io",resources=apiservers;clusterversions,verbs=get;list;watch
// +kubebuilder:rbac:groups="k8s.cni.cncf.io",resources=network-attachment-definitions,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="networking.istio.io",resources=envoyfilters,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="networking.k8s.io",resources=networkpolicies,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="policy",resources=poddisruptionbudgets,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterrolebindings;clusterroles,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=rolebindings;roles,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterrolebindings;clusterroles;rolebindings;roles,verbs=bind;escalate
// +kubebuilder:rbac:groups=sailoperator.io,resources=istiorevisiontags,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=sailoperator.io,resources=istiorevisiontags/finalizers,verbs=update
// +kubebuilder:rbac:groups=sailoperator.io,resources=istiorevisiontags/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="security.openshift.io",resources=securitycontextconstraints,resourceNames=privileged,verbs=use
