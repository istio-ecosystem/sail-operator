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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestCompileRules(t *testing.T) {
	t.Run("valid rules compile successfully", func(t *testing.T) {
		rules := []FieldIgnoreRule{
			{Group: "apps", Version: "v1", Kind: "Deployment", NameRegex: "istio.*"},
			{Group: "apps", Version: "v1", Kind: "Deployment"},
		}
		compiled, err := CompileRules(rules)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if compiled[0].compiledNameRegex == nil {
			t.Error("expected compiledNameRegex to be set for rule with NameRegex")
		}
		if compiled[1].compiledNameRegex != nil {
			t.Error("expected compiledNameRegex to be nil for rule without NameRegex")
		}
	})

	t.Run("invalid regex returns error", func(t *testing.T) {
		rules := []FieldIgnoreRule{
			{Group: "apps", Version: "v1", Kind: "Deployment", NameRegex: "[invalid"},
		}
		_, err := CompileRules(rules)
		if err == nil {
			t.Error("expected error for invalid regex")
		}
	})

	t.Run("compiled rule uses pre-compiled regex", func(t *testing.T) {
		rules := []FieldIgnoreRule{
			{
				Group:     "admissionregistration.k8s.io",
				Version:   "v1",
				Kind:      "ValidatingWebhookConfiguration",
				NameRegex: "istiod-.*-validator",
			},
		}
		compiled, _ := CompileRules(rules)
		gvk := schema.GroupVersionKind{Group: "admissionregistration.k8s.io", Version: "v1", Kind: "ValidatingWebhookConfiguration"}
		if !compiled[0].Matches(gvk, "istiod-istio-system-validator") {
			t.Error("compiled rule should match")
		}
		if compiled[0].Matches(gvk, "other-webhook") {
			t.Error("compiled rule should not match non-matching name")
		}
	})
}

func TestMustCompileRulesPanicsOnInvalidRegex(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid regex")
		}
	}()
	MustCompileRules([]FieldIgnoreRule{
		{Group: "apps", Version: "v1", Kind: "Deployment", NameRegex: "[invalid"},
	})
}

func TestFieldIgnoreRuleMatches(t *testing.T) {
	rule := FieldIgnoreRule{
		Group:     "admissionregistration.k8s.io",
		Version:   "v1",
		Kind:      "ValidatingWebhookConfiguration",
		NameRegex: "istiod-.*-validator|istio-validator.*",
	}

	tests := []struct {
		name    string
		gvk     schema.GroupVersionKind
		objName string
		want    bool
	}{
		{
			name:    "matching gvk and name",
			gvk:     schema.GroupVersionKind{Group: "admissionregistration.k8s.io", Version: "v1", Kind: "ValidatingWebhookConfiguration"},
			objName: "istiod-istio-system-validator",
			want:    true,
		},
		{
			name:    "matching gvk and name with istio-validator prefix",
			gvk:     schema.GroupVersionKind{Group: "admissionregistration.k8s.io", Version: "v1", Kind: "ValidatingWebhookConfiguration"},
			objName: "istio-validator-default-istio-system",
			want:    true,
		},
		{
			name:    "matching gvk but non-matching name",
			gvk:     schema.GroupVersionKind{Group: "admissionregistration.k8s.io", Version: "v1", Kind: "ValidatingWebhookConfiguration"},
			objName: "some-other-webhook",
			want:    false,
		},
		{
			name:    "non-matching group",
			gvk:     schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "ValidatingWebhookConfiguration"},
			objName: "istiod-istio-system-validator",
			want:    false,
		},
		{
			name:    "non-matching kind",
			gvk:     schema.GroupVersionKind{Group: "admissionregistration.k8s.io", Version: "v1", Kind: "MutatingWebhookConfiguration"},
			objName: "istiod-istio-system-validator",
			want:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := rule.Matches(tc.gvk, tc.objName)
			if got != tc.want {
				t.Errorf("Matches() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFieldIgnoreRuleMatchesNoNameRegex(t *testing.T) {
	rule := FieldIgnoreRule{
		Group:   "admissionregistration.k8s.io",
		Version: "v1",
		Kind:    "ValidatingWebhookConfiguration",
	}
	gvk := schema.GroupVersionKind{Group: "admissionregistration.k8s.io", Version: "v1", Kind: "ValidatingWebhookConfiguration"}
	if !rule.Matches(gvk, "anything") {
		t.Error("rule with empty NameRegex should match any name")
	}
}

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

func TestMatchesManifest(t *testing.T) {
	rule := FieldIgnoreRule{
		Group:     "admissionregistration.k8s.io",
		Version:   "v1",
		Kind:      "ValidatingWebhookConfiguration",
		NameRegex: "istiod-.*-validator",
	}

	tests := []struct {
		name     string
		manifest map[string]any
		want     bool
	}{
		{
			name: "matching manifest",
			manifest: map[string]any{
				"apiVersion": "admissionregistration.k8s.io/v1",
				"kind":       "ValidatingWebhookConfiguration",
				"metadata":   map[string]any{"name": "istiod-istio-system-validator"},
			},
			want: true,
		},
		{
			name: "non-matching kind",
			manifest: map[string]any{
				"apiVersion": "admissionregistration.k8s.io/v1",
				"kind":       "MutatingWebhookConfiguration",
				"metadata":   map[string]any{"name": "istiod-istio-system-validator"},
			},
			want: false,
		},
		{
			name: "non-matching name",
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
			got := rule.MatchesManifest(tc.manifest)
			if got != tc.want {
				t.Errorf("MatchesManifest() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRemoveFieldsFromManifest(t *testing.T) {
	rules := []FieldIgnoreRule{
		{
			Group:   "admissionregistration.k8s.io",
			Version: "v1",
			Kind:    "ValidatingWebhookConfiguration",
			Fields:  []string{"webhooks[*].failurePolicy"},

			OnlyOnUpdate: true,
		},
	}

	t.Run("removes field on update", func(t *testing.T) {
		manifest := map[string]any{
			"apiVersion": "admissionregistration.k8s.io/v1",
			"kind":       "ValidatingWebhookConfiguration",
			"metadata":   map[string]any{"name": "test"},
			"webhooks": []any{
				map[string]any{"name": "w1", "failurePolicy": "Fail"},
			},
		}
		RemoveFieldsFromManifest(manifest, rules, true)

		webhooks := manifest["webhooks"].([]any)
		w := webhooks[0].(map[string]any)
		if _, exists := w["failurePolicy"]; exists {
			t.Error("failurePolicy should have been removed on update")
		}
	})

	t.Run("preserves field on install", func(t *testing.T) {
		manifest := map[string]any{
			"apiVersion": "admissionregistration.k8s.io/v1",
			"kind":       "ValidatingWebhookConfiguration",
			"metadata":   map[string]any{"name": "test"},
			"webhooks": []any{
				map[string]any{"name": "w1", "failurePolicy": "Fail"},
			},
		}
		RemoveFieldsFromManifest(manifest, rules, false)

		webhooks := manifest["webhooks"].([]any)
		w := webhooks[0].(map[string]any)
		if _, exists := w["failurePolicy"]; !exists {
			t.Error("failurePolicy should be preserved on install when OnlyOnUpdate=true")
		}
	})

	t.Run("non-matching manifest is untouched", func(t *testing.T) {
		manifest := map[string]any{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata":   map[string]any{"name": "test"},
			"spec":       map[string]any{"replicas": 1},
		}
		RemoveFieldsFromManifest(manifest, rules, true)

		if manifest["spec"].(map[string]any)["replicas"] != 1 {
			t.Error("non-matching manifest should be untouched")
		}
	})
}

func TestNewPredicate(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = admissionv1.AddToScheme(scheme)

	rules := []FieldIgnoreRule{
		{
			Group:     "admissionregistration.k8s.io",
			Version:   "v1",
			Kind:      "ValidatingWebhookConfiguration",
			NameRegex: "istiod-.*-validator",
			Fields:    []string{"webhooks[*].failurePolicy"},
		},
	}

	pred := NewPredicate(scheme, rules)

	failPolicy := admissionv1.Fail
	ignorePolicy := admissionv1.Ignore

	t.Run("ignores failurePolicy-only change", func(t *testing.T) {
		oldObj := &admissionv1.ValidatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{Name: "istiod-istio-system-validator"},
			Webhooks: []admissionv1.ValidatingWebhook{
				{Name: "w1", FailurePolicy: &failPolicy},
			},
		}
		newObj := &admissionv1.ValidatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{Name: "istiod-istio-system-validator"},
			Webhooks: []admissionv1.ValidatingWebhook{
				{Name: "w1", FailurePolicy: &ignorePolicy},
			},
		}

		result := pred.Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj})
		if result {
			t.Error("predicate should return false when only ignored fields change")
		}
	})

	t.Run("detects non-ignored field change", func(t *testing.T) {
		oldObj := &admissionv1.ValidatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{Name: "istiod-istio-system-validator"},
			Webhooks: []admissionv1.ValidatingWebhook{
				{Name: "w1", FailurePolicy: &failPolicy},
			},
		}
		newObj := &admissionv1.ValidatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{Name: "istiod-istio-system-validator"},
			Webhooks: []admissionv1.ValidatingWebhook{
				{Name: "w1-renamed", FailurePolicy: &ignorePolicy},
			},
		}

		result := pred.Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj})
		if !result {
			t.Error("predicate should return true when non-ignored fields change")
		}
	})

	t.Run("non-matching name passes through", func(t *testing.T) {
		oldObj := &admissionv1.ValidatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{Name: "some-other-webhook"},
			Webhooks: []admissionv1.ValidatingWebhook{
				{Name: "w1", FailurePolicy: &failPolicy},
			},
		}
		newObj := &admissionv1.ValidatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{Name: "some-other-webhook"},
			Webhooks: []admissionv1.ValidatingWebhook{
				{Name: "w1", FailurePolicy: &ignorePolicy},
			},
		}

		result := pred.Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj})
		if !result {
			t.Error("predicate should return true for non-matching names (no filtering)")
		}
	})
}

func TestNewPredicateMutatingWebhookConfiguration(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = admissionv1.AddToScheme(scheme)

	rules := []FieldIgnoreRule{
		{
			Group:   "admissionregistration.k8s.io",
			Version: "v1",
			Kind:    "MutatingWebhookConfiguration",
			Fields:  []string{"webhooks[*].clientConfig.caBundle"},
		},
	}

	pred := NewPredicate(scheme, rules)

	t.Run("ignores caBundle-only change", func(t *testing.T) {
		oldObj := &admissionv1.MutatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{Name: "istio-sidecar-injector"},
			Webhooks: []admissionv1.MutatingWebhook{
				{
					Name: "w1",
					ClientConfig: admissionv1.WebhookClientConfig{
						CABundle: []byte("old-ca"),
					},
				},
			},
		}
		newObj := &admissionv1.MutatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{Name: "istio-sidecar-injector"},
			Webhooks: []admissionv1.MutatingWebhook{
				{
					Name: "w1",
					ClientConfig: admissionv1.WebhookClientConfig{
						CABundle: []byte("new-ca"),
					},
				},
			},
		}

		result := pred.Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj})
		if result {
			t.Error("predicate should return false when only caBundle changes on MutatingWebhookConfiguration")
		}
	})

	t.Run("detects non-ignored field change", func(t *testing.T) {
		oldObj := &admissionv1.MutatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{Name: "istio-sidecar-injector"},
			Webhooks: []admissionv1.MutatingWebhook{
				{
					Name: "w1",
					ClientConfig: admissionv1.WebhookClientConfig{
						CABundle: []byte("old-ca"),
					},
				},
			},
		}
		newObj := &admissionv1.MutatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{Name: "istio-sidecar-injector"},
			Webhooks: []admissionv1.MutatingWebhook{
				{
					Name: "w1-renamed",
					ClientConfig: admissionv1.WebhookClientConfig{
						CABundle: []byte("new-ca"),
					},
				},
			},
		}

		result := pred.Update(event.UpdateEvent{ObjectOld: oldObj, ObjectNew: newObj})
		if !result {
			t.Error("predicate should return true when non-ignored fields change on MutatingWebhookConfiguration")
		}
	})
}

func TestClearMetadataFields(t *testing.T) {
	obj := map[string]any{
		"metadata": map[string]any{
			"name":            "test",
			"resourceVersion": "12345",
			"generation":      int64(3),
			"managedFields":   []any{"field1"},
		},
	}

	clearMetadataFields(obj)

	metadata := obj["metadata"].(map[string]any)
	if _, exists := metadata["resourceVersion"]; exists {
		t.Error("resourceVersion should be cleared")
	}
	if _, exists := metadata["generation"]; exists {
		t.Error("generation should be cleared")
	}
	if _, exists := metadata["managedFields"]; exists {
		t.Error("managedFields should be cleared")
	}
	if metadata["name"] != "test" {
		t.Error("name should be preserved")
	}
}
