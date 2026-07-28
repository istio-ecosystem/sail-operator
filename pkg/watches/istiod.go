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

package watches

import (
	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

// IstiodWatches lists resource types produced by the base and istiod Helm charts.
var IstiodWatches = []WatchedResource{
	{Object: &admissionv1.MutatingWebhookConfiguration{}, ShouldReconcile: WebhookFilter()},
	// +lint-watches:ignore: ValidatingAdmissionPolicy (TODO: add watch when CI supports k8s 1.30)
	{Object: &admissionv1.ValidatingAdmissionPolicy{}, Skipped: true},
	// +lint-watches:ignore: ValidatingAdmissionPolicyBinding (TODO: add watch when CI supports k8s 1.30)
	{Object: &admissionv1.ValidatingAdmissionPolicyBinding{}, Skipped: true},
	{Object: &admissionv1.ValidatingWebhookConfiguration{}, ShouldReconcile: WebhookFilter()},
	{Object: &appsv1.Deployment{}},
	{Object: &autoscalingv2.HorizontalPodAutoscaler{}, ShouldReconcile: IgnoreStatusChanges()},
	{Object: &corev1.ConfigMap{}},
	{Object: &corev1.Service{}, ShouldReconcile: IgnoreStatusChanges()},
	{Object: &corev1.ServiceAccount{}, ShouldReconcile: IgnoreAllUpdates()},
	{Object: &discoveryv1.EndpointSlice{}},
	{Object: &networkingv1.NetworkPolicy{}, ShouldReconcile: IgnoreStatusChanges()},
	{Object: &policyv1.PodDisruptionBudget{}, ShouldReconcile: IgnoreStatusChanges()},
	{Object: &rbacv1.Role{}},
	{Object: &rbacv1.RoleBinding{}},
	{Object: &rbacv1.ClusterRole{}},
	{Object: &rbacv1.ClusterRoleBinding{}},
}
