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
	"slices"

	admissionv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DefaultRules defines the default set of fields to ignore on resources managed
// by the operator. These prevent reconciliation loops caused by other
// controllers (e.g. istiod, pull-secret injectors, Azure admission enforcer)
// updating fields that the operator also manages via Helm.
var DefaultIgnoreRules = slices.Concat(
	convertToGenericRules(ValidatingWebhookIgnoreRules),
	convertToGenericRules(MutatingWebhookIgnoreRules),
	convertToGenericRules(ServiceAccountIgnoreRules),
)

// convertToGenericRules converts a slice of a specific impl and returns a generic slice.
func convertToGenericRules[T client.Object](rules RuleSet[T]) RuleSet[client.Object] {
	var rs RuleSet[client.Object]
	for _, rule := range rules {
		if rule.Name != "" {
			rs = append(rs, NewFieldIgnoreRuleWithName[client.Object](rule.obj, rule.Name, rule.Fields, rule.Scope))
		} else {
			rs = append(rs, NewFieldIgnoreRule[client.Object](rule.obj, rule.Fields, rule.Scope))
		}
	}
	return rs
}

var ValidatingWebhookIgnoreRules = RuleSet[*admissionv1.ValidatingWebhookConfiguration]{
	NewFieldIgnoreRule(&admissionv1.ValidatingWebhookConfiguration{}, []string{"webhooks[*].failurePolicy"}, IgnoreScopeReconcileAndUpgrade),
	NewFieldIgnoreRule(&admissionv1.ValidatingWebhookConfiguration{}, []string{"webhooks[*].clientConfig.caBundle"}, IgnoreScopeAlways),
}

var MutatingWebhookIgnoreRules = RuleSet[*admissionv1.MutatingWebhookConfiguration]{
	NewFieldIgnoreRule(&admissionv1.MutatingWebhookConfiguration{}, []string{"webhooks[*].clientConfig.caBundle"}, IgnoreScopeAlways),
	// AKS manipulates MutatingWebhookConfigurations. See https://github.com/istio-ecosystem/sail-operator/issues/1148
	NewFieldIgnoreRule(
		&admissionv1.MutatingWebhookConfiguration{},
		[]string{"webhooks[*].namespaceSelector.matchExpressions[key=kubernetes.azure.com/managedby]"},
		IgnoreScopeReconcile,
	),
}

var ServiceAccountIgnoreRules = RuleSet[*corev1.ServiceAccount]{
	NewFieldIgnoreRule(&corev1.ServiceAccount{}, []string{"imagePullSecrets"}, IgnoreScopeAlways),
	NewFieldIgnoreRule(&corev1.ServiceAccount{}, []string{"automountServiceAccountToken"}, IgnoreScopeAlways),
	NewFieldIgnoreRule(&corev1.ServiceAccount{}, []string{"secrets"}, IgnoreScopeAlways),
}
