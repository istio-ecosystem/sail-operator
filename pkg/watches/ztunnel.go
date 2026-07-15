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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
)

// ZTunnelWatches lists resource types produced by the ztunnel Helm chart.
var ZTunnelWatches = []WatchedResource{
	{Object: &appsv1.DaemonSet{}},
	{Object: &corev1.ResourceQuota{}},
	{Object: &corev1.ServiceAccount{}, ShouldReconcile: IgnoreAllUpdates()},
	{Object: &networkingv1.NetworkPolicy{}, ShouldReconcile: IgnoreAllUpdates()},
	{Object: &rbacv1.ClusterRole{}},
	{Object: &rbacv1.ClusterRoleBinding{}},
}
