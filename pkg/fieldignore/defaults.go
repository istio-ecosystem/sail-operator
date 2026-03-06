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

package fieldignore

import (
	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
)

// DefaultRules defines the default set of fields to ignore on resources managed
// by the operator. These prevent reconciliation loops caused by other
// controllers (e.g. istiod, pull-secret injectors, Azure admission enforcer)
// updating fields that the operator also manages via Helm.
//
// Use RulesFor to extract typed rules for a specific resource type.
var DefaultRules = []RuleSet{
	TypedRuleSet[*admissionv1.ValidatingWebhookConfiguration]{
		{Fields: []string{"webhooks[*].failurePolicy"}, Scope: IgnoreScopeReconcileAndUpgrade},
		{Fields: []string{"webhooks[*].clientConfig.caBundle"}},
	},
	TypedRuleSet[*admissionv1.MutatingWebhookConfiguration]{
		{Fields: []string{"webhooks[*].clientConfig.caBundle"}},
		// AKS manipulates MutatingWebhookConfigurations. See https://github.com/istio-ecosystem/sail-operator/issues/1148
		{Fields: []string{"webhooks[*].namespaceSelector.matchExpressions[key=kubernetes.azure.com/managedby]"}, Scope: IgnoreScopeReconcile},
	},
	TypedRuleSet[*corev1.ServiceAccount]{
		{Fields: []string{"imagePullSecrets"}},
		{Fields: []string{"automountServiceAccountToken"}},
		{Fields: []string{"secrets"}},
	},
}
