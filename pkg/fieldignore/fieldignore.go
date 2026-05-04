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
	"fmt"
	"reflect"
	"strings"

	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// IgnoreScope controls when a field ignore rule takes effect.
type IgnoreScope string

const (
	// IgnoreScopeAlways is the zero value: the field is always stripped from
	// Helm output (install and upgrade) and always ignored in the predicate.
	IgnoreScopeAlways IgnoreScope = ""

	// IgnoreScopeReconcile means the field is ignored by the predicate only.
	// Helm renders the field normally on both install and upgrade.
	IgnoreScopeReconcile IgnoreScope = "Reconcile"

	// IgnoreScopeReconcileAndUpgrade means the field is ignored by the
	// predicate, and stripped from Helm output on upgrades but not on initial
	// installs. Use this when Helm should set the initial value but another
	// controller manages it afterward.
	IgnoreScopeReconcileAndUpgrade IgnoreScope = "ReconcileAndUpgrade"
)

// FieldIgnoreRule defines a set of fields to ignore for resources of a specific type.
//
// Field paths use dot notation with [*] for array wildcards:
//   - "webhooks[*].failurePolicy"           → deletes failurePolicy from each webhook
//   - "webhooks[*].clientConfig.caBundle"   → deletes caBundle nested inside each webhook
//   - "spec.template.metadata.annotations"  → deletes a deeply nested field
type FieldIgnoreRule[T client.Object] struct {
	// obj is an empty instance of the object type. It is used to extract the GVK.
	// If go supported accessing a subset of the Type parameter's methods, we could avoid this.
	// e.g. T.GetGroupVersionKind()
	obj T

	// Name is an optional exact name match. Empty matches all names.
	Name string `json:"name,omitempty"`

	// Fields is the list of field paths to ignore.
	Fields []string `json:"fields"`

	// Scope controls when this rule takes effect. See IgnoreScope constants.
	Scope IgnoreScope `json:"scope,omitempty"`
}

func NewFieldIgnoreRule[T client.Object](obj T, fields []string, scope IgnoreScope) FieldIgnoreRule[T] {
	return FieldIgnoreRule[T]{
		obj:    obj,
		Fields: fields,
		Scope:  scope,
	}
}

func NewFieldIgnoreRuleWithName[T client.Object](obj T, name string, fields []string, scope IgnoreScope) FieldIgnoreRule[T] {
	return FieldIgnoreRule[T]{
		obj:    obj,
		Name:   name,
		Fields: fields,
		Scope:  scope,
	}
}

// RuleSet is a collection of field ignore rules for a specific resource type.
type RuleSet[T client.Object] []FieldIgnoreRule[T]

// NewPredicate returns a predicate that ignores changes to fields specified by
// the given typed rules. On update events it converts both old and new objects to
// unstructured maps, removes the ignored fields (plus standard metadata noise
// like resourceVersion, generation, managedFields), and only triggers
// reconciliation when the remaining content differs.
func (rules RuleSet[T]) NewPredicate() predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectOld == nil || e.ObjectNew == nil {
				return false
			}
			return objectsChangedIgnoringFields(e.ObjectOld, e.ObjectNew, rules)
		},
	}
}

// GenericFieldIgnoreRule is the generic version used for manifest matching.
// This is primarily for Helm post-rendering and runtime matching against unstructured manifests.
type GenericFieldIgnoreRule = FieldIgnoreRule[client.Object]

// MatchesManifest returns true if the untyped rule applies to an unstructured manifest map.
func MatchesManifest(rule GenericFieldIgnoreRule, manifest map[string]any) bool {
	apiVersion, _, _ := unstructured.NestedString(manifest, "apiVersion")
	kind, _, _ := unstructured.NestedString(manifest, "kind")
	name, _, _ := unstructured.NestedString(manifest, "metadata", "name")

	// Unstructured is handled by this even if the type is not registered in the scheme.
	gvk, err := apiutil.GVKForObject(rule.obj, scheme.Scheme)
	if err != nil {
		return false
	}

	if gvk.GroupVersion().Identifier() != apiVersion || gvk.Kind != kind {
		return false
	}

	if rule.Name != "" && rule.Name != name {
		return false
	}

	return true
}

// RemoveFieldsFromManifest removes ignored fields from an unstructured manifest map.
// Rules with Scope=Reconcile are never applied (predicate-only). Rules with
// Scope=ReconcileAndUpgrade are only applied when isUpdate is true.
func RemoveFieldsFromManifest(manifest map[string]any, rules []GenericFieldIgnoreRule, isUpdate bool) {
	for _, rule := range rules {
		switch rule.Scope {
		case IgnoreScopeReconcile:
			continue
		case IgnoreScopeReconcileAndUpgrade:
			if !isUpdate {
				continue
			}
		}
		if !MatchesManifest(rule, manifest) {
			continue
		}
		for _, field := range rule.Fields {
			removeFieldPath(manifest, field)
		}
	}
}

func objectsChangedIgnoringFields[T client.Object](oldObj, newObj client.Object, rules RuleSet[T]) bool {
	name := newObj.GetName()

	// Collect fields from all matching rules regardless of Scope.
	// Scope only controls post-renderer behavior (whether Helm strips the
	// field). In the predicate we always want to ignore these fields to avoid
	// unnecessary reconciliation.
	var matchingFields []string
	for _, rule := range rules {
		if rule.Name == "" || rule.Name == name {
			matchingFields = append(matchingFields, rule.Fields...)
		}
	}
	if len(matchingFields) == 0 {
		return true
	}

	oldMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(oldObj)
	if err != nil {
		return true
	}
	newMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(newObj)
	if err != nil {
		return true
	}

	clearMetadataFields(oldMap)
	clearMetadataFields(newMap)

	for _, field := range matchingFields {
		removeFieldPath(oldMap, field)
		removeFieldPath(newMap, field)
	}

	return !reflect.DeepEqual(oldMap, newMap)
}

// clearMetadataFields removes standard metadata fields that change on every
// update and should never trigger reconciliation.
func clearMetadataFields(obj map[string]any) {
	if metadata, ok := obj["metadata"].(map[string]any); ok {
		delete(metadata, "resourceVersion")
		delete(metadata, "generation")
		delete(metadata, "managedFields")
	}
}

// removeFieldPath removes a field from a nested map using dot-separated path
// notation. Array wildcards ([*]) cause the operation to be applied to every
// element of the array. A [key=value] predicate matches only array elements
// where the given field equals the given value.
func removeFieldPath(obj map[string]any, path string) {
	segments := splitFieldPath(path)
	removeFieldSegments(obj, segments)
}

// splitFieldPath splits a dot-separated field path into segments, but does not
// split on dots inside bracket expressions (e.g. [key=kubernetes.azure.com/managedby]).
func splitFieldPath(path string) []string {
	var segments []string
	depth := 0
	start := 0
	for i := 0; i < len(path); i++ {
		switch path[i] {
		case '[':
			depth++
		case ']':
			depth--
		case '.':
			if depth == 0 {
				segments = append(segments, path[start:i])
				start = i + 1
			}
		}
	}
	segments = append(segments, path[start:])
	return segments
}

// parseArrayPredicate parses an array segment like "webhooks[*]" or
// "matchExpressions[key=value]". Returns the array key, predicate field,
// predicate value, and whether a predicate was found at all.
func parseArrayPredicate(seg string) (arrayKey, predField, predValue string, hasPredicate bool) {
	openBracket := strings.IndexByte(seg, '[')
	if openBracket < 0 || !strings.HasSuffix(seg, "]") {
		return seg, "", "", false
	}
	arrayKey = seg[:openBracket]
	inner := seg[openBracket+1 : len(seg)-1]
	if inner == "*" {
		return arrayKey, "*", "", true
	}
	if eqIdx := strings.IndexByte(inner, '='); eqIdx >= 0 {
		return arrayKey, inner[:eqIdx], inner[eqIdx+1:], true
	}
	return seg, "", "", false
}

func removeFieldSegments(obj map[string]any, segments []string) {
	if len(segments) == 0 || obj == nil {
		return
	}

	seg := segments[0]
	remaining := segments[1:]

	arrayKey, predField, predValue, hasPredicate := parseArrayPredicate(seg)
	if hasPredicate {
		if predField == "*" {
			if len(remaining) == 0 {
				delete(obj, arrayKey)
				return
			}
			arr, ok := obj[arrayKey].([]any)
			if !ok {
				return
			}
			for _, item := range arr {
				if m, ok := item.(map[string]any); ok {
					removeFieldSegments(m, remaining)
				}
			}
			return
		}

		arr, ok := obj[arrayKey].([]any)
		if !ok {
			return
		}
		if len(remaining) == 0 {
			filtered := make([]any, 0, len(arr))
			for _, item := range arr {
				m, ok := item.(map[string]any)
				if !ok || fmt.Sprintf("%v", m[predField]) != predValue {
					filtered = append(filtered, item)
				}
			}
			obj[arrayKey] = filtered
			return
		}
		for _, item := range arr {
			if m, ok := item.(map[string]any); ok {
				if fmt.Sprintf("%v", m[predField]) == predValue {
					removeFieldSegments(m, remaining)
				}
			}
		}
		return
	}

	if len(remaining) == 0 {
		delete(obj, seg)
		return
	}

	child, ok := obj[seg].(map[string]any)
	if !ok {
		return
	}
	removeFieldSegments(child, remaining)
}
