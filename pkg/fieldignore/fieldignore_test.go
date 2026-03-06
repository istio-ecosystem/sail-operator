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
	"testing"

	"github.com/google/go-cmp/cmp"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestRemoveFieldPath(t *testing.T) {
	tests := []struct {
		name     string
		obj      map[string]any
		path     string
		expected map[string]any
	}{
		{
			name:     "simple top-level field",
			obj:      map[string]any{"foo": "bar", "baz": "qux"},
			path:     "foo",
			expected: map[string]any{"baz": "qux"},
		},
		{
			name:     "nested field",
			obj:      map[string]any{"spec": map[string]any{"replicas": 3, "selector": "app"}},
			path:     "spec.replicas",
			expected: map[string]any{"spec": map[string]any{"selector": "app"}},
		},
		{
			name: "array wildcard",
			obj: map[string]any{
				"webhooks": []any{
					map[string]any{"name": "w1", "failurePolicy": "Fail"},
					map[string]any{"name": "w2", "failurePolicy": "Ignore"},
				},
			},
			path: "webhooks[*].failurePolicy",
			expected: map[string]any{
				"webhooks": []any{
					map[string]any{"name": "w1"},
					map[string]any{"name": "w2"},
				},
			},
		},
		{
			name: "nested array wildcard",
			obj: map[string]any{
				"webhooks": []any{
					map[string]any{
						"name":         "w1",
						"clientConfig": map[string]any{"caBundle": "abc", "url": "https://example.com"},
					},
				},
			},
			path: "webhooks[*].clientConfig.caBundle",
			expected: map[string]any{
				"webhooks": []any{
					map[string]any{
						"name":         "w1",
						"clientConfig": map[string]any{"url": "https://example.com"},
					},
				},
			},
		},
		{
			name:     "non-existent field is a no-op",
			obj:      map[string]any{"foo": "bar"},
			path:     "nonexistent.field",
			expected: map[string]any{"foo": "bar"},
		},
		{
			name: "bare array wildcard removes entire array",
			obj: map[string]any{
				"webhooks": []any{
					map[string]any{"name": "w1"},
					map[string]any{"name": "w2"},
				},
				"other": "value",
			},
			path:     "webhooks[*]",
			expected: map[string]any{"other": "value"},
		},
		{
			name:     "non-existent array is a no-op",
			obj:      map[string]any{"foo": "bar"},
			path:     "items[*].field",
			expected: map[string]any{"foo": "bar"},
		},
		{
			name: "[key=value] removes matching elements",
			obj: map[string]any{
				"matchExpressions": []any{
					map[string]any{"key": "azure", "operator": "Exists"},
					map[string]any{"key": "other", "operator": "In"},
				},
			},
			path: "matchExpressions[key=azure]",
			expected: map[string]any{
				"matchExpressions": []any{
					map[string]any{"key": "other", "operator": "In"},
				},
			},
		},
		{
			name: "[key=value] recurses into matching elements only",
			obj: map[string]any{
				"matchExpressions": []any{
					map[string]any{"key": "azure", "operator": "Exists"},
					map[string]any{"key": "other", "operator": "In"},
				},
			},
			path: "matchExpressions[key=azure].operator",
			expected: map[string]any{
				"matchExpressions": []any{
					map[string]any{"key": "azure"},
					map[string]any{"key": "other", "operator": "In"},
				},
			},
		},
		{
			name: "[key=value] with non-existent key is a no-op",
			obj: map[string]any{
				"matchExpressions": []any{
					map[string]any{"key": "azure", "operator": "Exists"},
				},
			},
			path: "matchExpressions[key=nonexistent]",
			expected: map[string]any{
				"matchExpressions": []any{
					map[string]any{"key": "azure", "operator": "Exists"},
				},
			},
		},
		{
			name: "nested wildcard + [key=value] predicate",
			obj: map[string]any{
				"webhooks": []any{
					map[string]any{
						"name": "w1",
						"namespaceSelector": map[string]any{
							"matchExpressions": []any{
								map[string]any{"key": "azure", "operator": "Exists"},
								map[string]any{"key": "keep", "operator": "In"},
							},
						},
					},
					map[string]any{
						"name": "w2",
						"namespaceSelector": map[string]any{
							"matchExpressions": []any{
								map[string]any{"key": "azure", "operator": "DoesNotExist"},
							},
						},
					},
				},
			},
			path: "webhooks[*].namespaceSelector.matchExpressions[key=azure]",
			expected: map[string]any{
				"webhooks": []any{
					map[string]any{
						"name": "w1",
						"namespaceSelector": map[string]any{
							"matchExpressions": []any{
								map[string]any{"key": "keep", "operator": "In"},
							},
						},
					},
					map[string]any{
						"name": "w2",
						"namespaceSelector": map[string]any{
							"matchExpressions": []any{},
						},
					},
				},
			},
		},
		{
			name: "[key=value] with dots in value",
			obj: map[string]any{
				"webhooks": []any{
					map[string]any{
						"name": "w1",
						"namespaceSelector": map[string]any{
							"matchExpressions": []any{
								map[string]any{"key": "kubernetes.azure.com/managedby", "operator": "In", "values": []any{"aks"}},
								map[string]any{"key": "keep", "operator": "In"},
							},
						},
					},
				},
			},
			path: "webhooks[*].namespaceSelector.matchExpressions[key=kubernetes.azure.com/managedby]",
			expected: map[string]any{
				"webhooks": []any{
					map[string]any{
						"name": "w1",
						"namespaceSelector": map[string]any{
							"matchExpressions": []any{
								map[string]any{"key": "keep", "operator": "In"},
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			removeFieldPath(tc.obj, tc.path)
			if diff := cmp.Diff(tc.expected, tc.obj); diff != "" {
				t.Errorf("removeFieldPath mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSplitFieldPath(t *testing.T) {
	tests := []struct {
		path string
		want []string
	}{
		{"foo", []string{"foo"}},
		{"foo.bar", []string{"foo", "bar"}},
		{"webhooks[*].failurePolicy", []string{"webhooks[*]", "failurePolicy"}},
		{"spec.someField[key=some.key/andSlash]", []string{"spec", "someField[key=some.key/andSlash]"}},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			got := splitFieldPath(tc.path)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("splitFieldPath(%q) mismatch (-want +got):\n%s", tc.path, diff)
			}
		})
	}
}

func TestParseArrayPredicate(t *testing.T) {
	tests := []struct {
		seg       string
		arrayKey  string
		predField string
		predValue string
		hasPred   bool
	}{
		{"webhooks[*]", "webhooks", "*", "", true},
		{"matchExpressions[key=foo]", "matchExpressions", "key", "foo", true},
		{"plain", "plain", "", "", false},
		{"matchExpressions[key=kubernetes.azure.com/managedby]", "matchExpressions", "key", "kubernetes.azure.com/managedby", true},
	}

	for _, tc := range tests {
		t.Run(tc.seg, func(t *testing.T) {
			arrayKey, predField, predValue, hasPred := parseArrayPredicate(tc.seg)
			if arrayKey != tc.arrayKey || predField != tc.predField || predValue != tc.predValue || hasPred != tc.hasPred {
				t.Errorf("parseArrayPredicate(%q) = (%q, %q, %q, %v), want (%q, %q, %q, %v)",
					tc.seg, arrayKey, predField, predValue, hasPred,
					tc.arrayKey, tc.predField, tc.predValue, tc.hasPred)
			}
		})
	}
}

func TestMatchesManifest(t *testing.T) {
	tests := []struct {
		name     string
		rule     UntypedFieldIgnoreRule
		manifest map[string]any
		want     bool
	}{
		{
			name: "matching manifest with exact name",
			rule: UntypedFieldIgnoreRule{
				Group:   "admissionregistration.k8s.io",
				Version: "v1",
				Kind:    "ValidatingWebhookConfiguration",
				Name:    "istiod-istio-system-validator",
				Fields:  []string{"webhooks[*].failurePolicy"},
			},
			manifest: map[string]any{
				"apiVersion": "admissionregistration.k8s.io/v1",
				"kind":       "ValidatingWebhookConfiguration",
				"metadata":   map[string]any{"name": "istiod-istio-system-validator"},
			},
			want: true,
		},
		{
			name: "matching manifest with no name requirement",
			rule: UntypedFieldIgnoreRule{
				Group:   "admissionregistration.k8s.io",
				Version: "v1",
				Kind:    "ValidatingWebhookConfiguration",
				Fields:  []string{"webhooks[*].failurePolicy"},
			},
			manifest: map[string]any{
				"apiVersion": "admissionregistration.k8s.io/v1",
				"kind":       "ValidatingWebhookConfiguration",
				"metadata":   map[string]any{"name": "any-webhook"},
			},
			want: true,
		},
		{
			name: "non-matching kind",
			rule: UntypedFieldIgnoreRule{
				Group:   "admissionregistration.k8s.io",
				Version: "v1",
				Kind:    "MutatingWebhookConfiguration",
				Fields:  []string{"webhooks[*].failurePolicy"},
			},
			manifest: map[string]any{
				"apiVersion": "admissionregistration.k8s.io/v1",
				"kind":       "ValidatingWebhookConfiguration",
				"metadata":   map[string]any{"name": "istiod-istio-system-validator"},
			},
			want: false,
		},
		{
			name: "non-matching name",
			rule: UntypedFieldIgnoreRule{
				Group:   "admissionregistration.k8s.io",
				Version: "v1",
				Kind:    "ValidatingWebhookConfiguration",
				Name:    "istiod-istio-system-validator",
				Fields:  []string{"webhooks[*].failurePolicy"},
			},
			manifest: map[string]any{
				"apiVersion": "admissionregistration.k8s.io/v1",
				"kind":       "ValidatingWebhookConfiguration",
				"metadata":   map[string]any{"name": "other-webhook"},
			},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.rule.MatchesManifest(tc.manifest)
			if got != tc.want {
				t.Errorf("MatchesManifest() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRemoveFieldsFromManifest(t *testing.T) {
	reconcileAndUpgradeRules := []UntypedFieldIgnoreRule{
		{
			Group:   "admissionregistration.k8s.io",
			Version: "v1",
			Kind:    "ValidatingWebhookConfiguration",
			Fields:  []string{"webhooks[*].failurePolicy"},
			Scope:   IgnoreScopeReconcileAndUpgrade,
		},
	}
	reconcileOnlyRules := []UntypedFieldIgnoreRule{
		{
			Group:   "admissionregistration.k8s.io",
			Version: "v1",
			Kind:    "ValidatingWebhookConfiguration",
			Fields:  []string{"webhooks[*].failurePolicy"},
			Scope:   IgnoreScopeReconcile,
		},
	}

	webhookManifest := func() map[string]any {
		return map[string]any{
			"apiVersion": "admissionregistration.k8s.io/v1",
			"kind":       "ValidatingWebhookConfiguration",
			"metadata":   map[string]any{"name": "test"},
			"webhooks":   []any{map[string]any{"name": "w1", "failurePolicy": "Fail"}},
		}
	}

	tests := []struct {
		name     string
		rules    []UntypedFieldIgnoreRule
		manifest map[string]any
		isUpdate bool
		expected map[string]any
	}{
		{
			name:     "removes field on update",
			rules:    reconcileAndUpgradeRules,
			manifest: webhookManifest(),
			isUpdate: true,
			expected: map[string]any{
				"apiVersion": "admissionregistration.k8s.io/v1",
				"kind":       "ValidatingWebhookConfiguration",
				"metadata":   map[string]any{"name": "test"},
				"webhooks":   []any{map[string]any{"name": "w1"}},
			},
		},
		{
			name:     "preserves field on install when Scope=ReconcileAndUpgrade",
			rules:    reconcileAndUpgradeRules,
			manifest: webhookManifest(),
			isUpdate: false,
			expected: webhookManifest(),
		},
		{
			name:     "Scope=Reconcile never strips field on install",
			rules:    reconcileOnlyRules,
			manifest: webhookManifest(),
			isUpdate: false,
			expected: webhookManifest(),
		},
		{
			name:     "Scope=Reconcile never strips field on update",
			rules:    reconcileOnlyRules,
			manifest: webhookManifest(),
			isUpdate: true,
			expected: webhookManifest(),
		},
		{
			name:  "non-matching manifest is untouched",
			rules: reconcileAndUpgradeRules,
			manifest: map[string]any{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata":   map[string]any{"name": "test"},
				"spec":       map[string]any{"replicas": 1},
			},
			isUpdate: true,
			expected: map[string]any{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata":   map[string]any{"name": "test"},
				"spec":       map[string]any{"replicas": 1},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			RemoveFieldsFromManifest(tc.manifest, tc.rules, tc.isUpdate)
			if diff := cmp.Diff(tc.expected, tc.manifest); diff != "" {
				t.Errorf("RemoveFieldsFromManifest mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestNewPredicateWithEmptyName(t *testing.T) {
	rules := TypedRuleSet[*admissionv1.ValidatingWebhookConfiguration]{
		{Fields: []string{"webhooks[*].failurePolicy"}},
	}
	pred := rules.NewPredicate()

	failPolicy := admissionv1.Fail
	ignorePolicy := admissionv1.Ignore

	tests := []struct {
		name   string
		oldObj *admissionv1.ValidatingWebhookConfiguration
		newObj *admissionv1.ValidatingWebhookConfiguration
		want   bool
	}{
		{
			name: "ignores failurePolicy change on any webhook name",
			oldObj: &admissionv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: "any-webhook-name"},
				Webhooks: []admissionv1.ValidatingWebhook{
					{Name: "w1", FailurePolicy: &failPolicy},
				},
			},
			newObj: &admissionv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: "any-webhook-name"},
				Webhooks: []admissionv1.ValidatingWebhook{
					{Name: "w1", FailurePolicy: &ignorePolicy},
				},
			},
			want: false,
		},
		{
			name: "detects non-ignored change on any webhook name",
			oldObj: &admissionv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: "another-webhook"},
				Webhooks: []admissionv1.ValidatingWebhook{
					{Name: "w1", FailurePolicy: &failPolicy},
				},
			},
			newObj: &admissionv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: "another-webhook"},
				Webhooks: []admissionv1.ValidatingWebhook{
					{Name: "w1-renamed", FailurePolicy: &failPolicy},
				},
			},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := pred.Update(event.UpdateEvent{ObjectOld: tc.oldObj, ObjectNew: tc.newObj})
			if result != tc.want {
				t.Errorf("pred.Update() = %v, want %v", result, tc.want)
			}
		})
	}
}

func TestNewPredicate(t *testing.T) {
	rules := TypedRuleSet[*admissionv1.ValidatingWebhookConfiguration]{
		{Name: "istiod-istio-system-validator", Fields: []string{"webhooks[*].failurePolicy"}},
	}
	pred := rules.NewPredicate()

	failPolicy := admissionv1.Fail
	ignorePolicy := admissionv1.Ignore

	tests := []struct {
		name   string
		oldObj *admissionv1.ValidatingWebhookConfiguration
		newObj *admissionv1.ValidatingWebhookConfiguration
		want   bool
	}{
		{
			name: "ignores failurePolicy-only change",
			oldObj: &admissionv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: "istiod-istio-system-validator"},
				Webhooks: []admissionv1.ValidatingWebhook{
					{Name: "w1", FailurePolicy: &failPolicy},
				},
			},
			newObj: &admissionv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: "istiod-istio-system-validator"},
				Webhooks: []admissionv1.ValidatingWebhook{
					{Name: "w1", FailurePolicy: &ignorePolicy},
				},
			},
			want: false,
		},
		{
			name: "detects non-ignored field change",
			oldObj: &admissionv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: "istiod-istio-system-validator"},
				Webhooks: []admissionv1.ValidatingWebhook{
					{Name: "w1", FailurePolicy: &failPolicy},
				},
			},
			newObj: &admissionv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: "istiod-istio-system-validator"},
				Webhooks: []admissionv1.ValidatingWebhook{
					{Name: "w1-renamed", FailurePolicy: &ignorePolicy},
				},
			},
			want: true,
		},
		{
			name: "non-matching name passes through",
			oldObj: &admissionv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: "some-other-webhook"},
				Webhooks: []admissionv1.ValidatingWebhook{
					{Name: "w1", FailurePolicy: &failPolicy},
				},
			},
			newObj: &admissionv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: "some-other-webhook"},
				Webhooks: []admissionv1.ValidatingWebhook{
					{Name: "w1", FailurePolicy: &ignorePolicy},
				},
			},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := pred.Update(event.UpdateEvent{ObjectOld: tc.oldObj, ObjectNew: tc.newObj})
			if result != tc.want {
				t.Errorf("pred.Update() = %v, want %v", result, tc.want)
			}
		})
	}
}

func TestNewPredicateMutatingWebhookConfiguration(t *testing.T) {
	rules := TypedRuleSet[*admissionv1.MutatingWebhookConfiguration]{
		{Fields: []string{"webhooks[*].clientConfig.caBundle"}},
	}
	pred := rules.NewPredicate()

	tests := []struct {
		name   string
		oldObj *admissionv1.MutatingWebhookConfiguration
		newObj *admissionv1.MutatingWebhookConfiguration
		want   bool
	}{
		{
			name: "ignores caBundle-only change",
			oldObj: &admissionv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: "istio-sidecar-injector"},
				Webhooks: []admissionv1.MutatingWebhook{
					{Name: "w1", ClientConfig: admissionv1.WebhookClientConfig{CABundle: []byte("old-ca")}},
				},
			},
			newObj: &admissionv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: "istio-sidecar-injector"},
				Webhooks: []admissionv1.MutatingWebhook{
					{Name: "w1", ClientConfig: admissionv1.WebhookClientConfig{CABundle: []byte("new-ca")}},
				},
			},
			want: false,
		},
		{
			name: "detects non-ignored field change",
			oldObj: &admissionv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: "istio-sidecar-injector"},
				Webhooks: []admissionv1.MutatingWebhook{
					{Name: "w1", ClientConfig: admissionv1.WebhookClientConfig{CABundle: []byte("old-ca")}},
				},
			},
			newObj: &admissionv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{Name: "istio-sidecar-injector"},
				Webhooks: []admissionv1.MutatingWebhook{
					{Name: "w1-renamed", ClientConfig: admissionv1.WebhookClientConfig{CABundle: []byte("new-ca")}},
				},
			},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := pred.Update(event.UpdateEvent{ObjectOld: tc.oldObj, ObjectNew: tc.newObj})
			if result != tc.want {
				t.Errorf("pred.Update() = %v, want %v", result, tc.want)
			}
		})
	}
}

func TestIntoUntyped(t *testing.T) {
	rules := TypedRuleSet[*admissionv1.ValidatingWebhookConfiguration]{
		{Name: "istiod-validator", Fields: []string{"webhooks[*].failurePolicy"}, Scope: IgnoreScopeReconcileAndUpgrade},
	}

	untyped := rules.IntoUntyped()
	if len(untyped) != 1 {
		t.Fatalf("expected 1 untyped rule, got %d", len(untyped))
	}
	r := untyped[0]
	if r.Group != "admissionregistration.k8s.io" || r.Version != "v1" || r.Kind != "ValidatingWebhookConfiguration" {
		t.Errorf("GVK = %s/%s %s, want admissionregistration.k8s.io/v1 ValidatingWebhookConfiguration", r.Group, r.Version, r.Kind)
	}
	if r.Name != "istiod-validator" {
		t.Errorf("Name = %q, want %q", r.Name, "istiod-validator")
	}
	if len(r.Fields) != 1 || r.Fields[0] != "webhooks[*].failurePolicy" {
		t.Errorf("Fields = %v, want [webhooks[*].failurePolicy]", r.Fields)
	}
	if r.Scope != IgnoreScopeReconcileAndUpgrade {
		t.Errorf("Scope = %q, want %q", r.Scope, IgnoreScopeReconcileAndUpgrade)
	}
}
