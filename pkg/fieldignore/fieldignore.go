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
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// FieldIgnoreRule defines a set of fields to ignore for resources matching
// a specific Group/Version/Kind and optional name pattern.
//
// Field paths use dot notation with [*] for array wildcards:
//   - "webhooks[*].failurePolicy"           → deletes failurePolicy from each webhook
//   - "webhooks[*].clientConfig.caBundle"   → deletes caBundle nested inside each webhook
//   - "spec.template.metadata.annotations"  → deletes a deeply nested field
//
// This type is designed to be easily exposed through a CRD in the future.
type FieldIgnoreRule struct {
	// Group is the API group (e.g. "admissionregistration.k8s.io"). Empty matches the core group.
	Group string `json:"group"`

	// Version is the API version (e.g. "v1").
	Version string `json:"version"`

	// Kind is the resource kind (e.g. "ValidatingWebhookConfiguration").
	Kind string `json:"kind"`

	// NameRegex is an optional regex to match resource names. Empty matches all names.
	NameRegex string `json:"nameRegex,omitempty"`

	// Fields is the list of field paths to ignore.
	Fields []string `json:"fields"`

	// OnlyOnUpdate when true means these fields are only stripped during Helm
	// upgrades (post-render), not on initial installs. This is useful when you
	// want Helm to set the initial value but let another controller manage it
	// afterward.
	OnlyOnUpdate bool `json:"onlyOnUpdate,omitempty"`

	// compiledNameRegex is the pre-compiled form of NameRegex, populated by
	// CompileRules/MustCompileRules. If nil and NameRegex is non-empty,
	// Matches will compile on every call (slower).
	compiledNameRegex *regexp.Regexp
}

// CompileRules validates and pre-compiles the NameRegex in each rule.
// Returns an error if any NameRegex is invalid. The returned slice should be
// used in place of the input; the originals are not modified.
func CompileRules(rules []FieldIgnoreRule) ([]FieldIgnoreRule, error) {
	compiled := make([]FieldIgnoreRule, len(rules))
	for i, r := range rules {
		if r.NameRegex != "" {
			re, err := regexp.Compile("^(?:" + r.NameRegex + ")$")
			if err != nil {
				return nil, fmt.Errorf("invalid NameRegex %q in rule %d: %w", r.NameRegex, i, err)
			}
			r.compiledNameRegex = re
		}
		compiled[i] = r
	}
	return compiled, nil
}

// MustCompileRules is like CompileRules but panics on error.
// Use this for hardcoded rules that are known to be valid.
func MustCompileRules(rules []FieldIgnoreRule) []FieldIgnoreRule {
	compiled, err := CompileRules(rules)
	if err != nil {
		panic(err)
	}
	return compiled
}

// Matches returns true if the rule applies to a resource with the given GVK and name.
func (r FieldIgnoreRule) Matches(gvk schema.GroupVersionKind, name string) bool {
	if r.Group != gvk.Group || r.Version != gvk.Version || r.Kind != gvk.Kind {
		return false
	}
	if r.compiledNameRegex != nil {
		return r.compiledNameRegex.MatchString(name)
	}
	if r.NameRegex != "" {
		matched, err := regexp.MatchString("^(?:"+r.NameRegex+")$", name)
		if err != nil || !matched {
			return false
		}
	}
	return true
}

// MatchesManifest returns true if the rule applies to an unstructured manifest map.
func (r FieldIgnoreRule) MatchesManifest(manifest map[string]any) bool {
	apiVersion, _, _ := unstructured.NestedString(manifest, "apiVersion")
	kind, _, _ := unstructured.NestedString(manifest, "kind")
	name, _, _ := unstructured.NestedString(manifest, "metadata", "name")

	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return false
	}
	return r.Matches(schema.GroupVersionKind{Group: gv.Group, Version: gv.Version, Kind: kind}, name)
}

// RemoveFieldsFromManifest removes ignored fields from an unstructured manifest map.
// Rules with OnlyOnUpdate=true are skipped when isUpdate is false.
func RemoveFieldsFromManifest(manifest map[string]any, rules []FieldIgnoreRule, isUpdate bool) {
	for _, rule := range rules {
		if rule.OnlyOnUpdate && !isUpdate {
			continue
		}
		if !rule.MatchesManifest(manifest) {
			continue
		}
		for _, field := range rule.Fields {
			removeFieldPath(manifest, field)
		}
	}
}

// NewPredicate returns a predicate that ignores changes to fields specified by
// the given rules. On update events it converts both old and new objects to
// unstructured maps, removes the ignored fields (plus standard metadata noise
// like resourceVersion, generation, managedFields), and only triggers
// reconciliation when the remaining content differs.
func NewPredicate(scheme *runtime.Scheme, rules []FieldIgnoreRule) predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			if e.ObjectOld == nil || e.ObjectNew == nil {
				return false
			}
			return objectsChangedIgnoringFields(scheme, e.ObjectOld, e.ObjectNew, rules)
		},
	}
}

func objectsChangedIgnoringFields(scheme *runtime.Scheme, oldObj, newObj client.Object, rules []FieldIgnoreRule) bool {
	gvk, err := gvkForObject(scheme, newObj)
	if err != nil {
		return true
	}

	name := newObj.GetName()

	// Collect fields from all matching rules regardless of OnlyOnUpdate.
	// OnlyOnUpdate only controls post-renderer behavior (whether Helm strips
	// the field on install vs upgrade). In the predicate we always want to
	// ignore these fields to avoid unnecessary reconciliation.
	var matchingFields []string
	for _, rule := range rules {
		if rule.Matches(gvk, name) {
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

func gvkForObject(scheme *runtime.Scheme, obj client.Object) (schema.GroupVersionKind, error) {
	gvks, _, err := scheme.ObjectKinds(obj)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}
	if len(gvks) == 0 {
		return schema.GroupVersionKind{}, fmt.Errorf("no GVK found for object %T", obj)
	}
	return gvks[0], nil
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
// element of the array.
func removeFieldPath(obj map[string]any, path string) {
	segments := strings.Split(path, ".")
	removeFieldSegments(obj, segments)
}

func removeFieldSegments(obj map[string]any, segments []string) {
	if len(segments) == 0 || obj == nil {
		return
	}

	seg := segments[0]
	remaining := segments[1:]

	if strings.HasSuffix(seg, "[*]") {
		key := strings.TrimSuffix(seg, "[*]")
		if len(remaining) == 0 {
			// Bare [*] with no remaining segments (e.g. "items[*]") removes
			// the entire array field.
			delete(obj, key)
			return
		}
		arr, ok := obj[key].([]any)
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
