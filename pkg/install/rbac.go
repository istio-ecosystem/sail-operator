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

package install

import (
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/istio-ecosystem/sail-operator/pkg/watches"
	discoveryv1 "k8s.io/api/discovery/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

var (
	helmManagedVerbs = []string{"get", "list", "watch", "create", "update", "patch", "delete"}
	readOnlyVerbs    = []string{"get", "list", "watch"}
	crdVerbs         = []string{"get", "list", "watch", "create", "update", "patch"}
)

var readOnlyTypes = map[reflect.Type]bool{
	reflect.TypeOf(&discoveryv1.EndpointSlice{}): true,
}

// LibraryRBACRules returns the RBAC PolicyRules that the library consumer
// needs to grant to the service account running the library. Rules are
// derived from watches.IstiodWatches plus static entries for CRDs,
// namespaces, and Helm release storage.
func LibraryRBACRules() []rbacv1.PolicyRule {
	type ruleKey struct {
		apiGroup string
		verbs    string
	}
	grouped := map[ruleKey][]string{}

	addEntry := func(apiGroup, resource string, verbs []string) {
		key := ruleKey{apiGroup: apiGroup, verbs: strings.Join(verbs, ",")}
		if !slices.Contains(grouped[key], resource) {
			grouped[key] = append(grouped[key], resource)
		}
	}

	for _, wr := range watches.IstiodWatches {
		gvks, _, err := clientgoscheme.Scheme.ObjectKinds(wr.Object)
		if err != nil || len(gvks) == 0 {
			continue
		}
		plural, _ := meta.UnsafeGuessKindToResource(gvks[0])
		verbs := helmManagedVerbs
		if readOnlyTypes[reflect.TypeOf(wr.Object)] {
			verbs = readOnlyVerbs
		}
		addEntry(plural.Group, plural.Resource, verbs)
	}

	addEntry("", "namespaces", readOnlyVerbs)
	addEntry("", "endpoints", readOnlyVerbs)
	addEntry("", "pods", readOnlyVerbs)
	addEntry("", "secrets", helmManagedVerbs)
	addEntry("apiextensions.k8s.io", "customresourcedefinitions", crdVerbs)

	var rules []rbacv1.PolicyRule
	for key, resources := range grouped {
		sort.Strings(resources)
		rules = append(rules, rbacv1.PolicyRule{
			APIGroups: []string{key.apiGroup},
			Resources: resources,
			Verbs:     strings.Split(key.verbs, ","),
		})
	}

	sort.Slice(rules, func(i, j int) bool {
		if rules[i].APIGroups[0] != rules[j].APIGroups[0] {
			return rules[i].APIGroups[0] < rules[j].APIGroups[0]
		}
		return rules[i].Resources[0] < rules[j].Resources[0]
	})

	return rules
}
