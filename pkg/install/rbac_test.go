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
	"slices"
	"testing"

	. "github.com/onsi/gomega"
)

func TestLibraryRBACRules(t *testing.T) {
	g := NewWithT(t)
	rules := LibraryRBACRules()
	g.Expect(rules).NotTo(BeEmpty())

	rulesByGroup := map[string][]string{}
	for _, r := range rules {
		for _, group := range r.APIGroups {
			rulesByGroup[group] = append(rulesByGroup[group], r.Resources...)
		}
	}

	g.Expect(rulesByGroup).To(HaveKey(""))
	g.Expect(rulesByGroup[""]).To(ContainElements("configmaps", "secrets", "serviceaccounts", "services", "namespaces"))
	g.Expect(rulesByGroup).To(HaveKey("apps"))
	g.Expect(rulesByGroup["apps"]).To(ContainElement("deployments"))
	g.Expect(rulesByGroup).To(HaveKey("apiextensions.k8s.io"))
	g.Expect(rulesByGroup["apiextensions.k8s.io"]).To(ContainElement("customresourcedefinitions"))
	g.Expect(rulesByGroup).To(HaveKey("rbac.authorization.k8s.io"))
	g.Expect(rulesByGroup["rbac.authorization.k8s.io"]).To(ContainElements("clusterroles", "clusterrolebindings", "roles", "rolebindings"))
	g.Expect(rulesByGroup).To(HaveKey("admissionregistration.k8s.io"))
	g.Expect(rulesByGroup["admissionregistration.k8s.io"]).To(ContainElements(
		"mutatingwebhookconfigurations", "validatingwebhookconfigurations",
		"validatingadmissionpolicies", "validatingadmissionpolicybindings",
	))
}

func TestLibraryRBACRules_NoWildcardVerbs(t *testing.T) {
	g := NewWithT(t)
	for _, rule := range LibraryRBACRules() {
		g.Expect(rule.Verbs).NotTo(ContainElement("*"), "rule for %v should not use wildcard verbs", rule.Resources)
	}
}

func TestLibraryRBACRules_EndpointSliceIsReadOnly(t *testing.T) {
	g := NewWithT(t)
	for _, rule := range LibraryRBACRules() {
		if slices.Contains(rule.Resources, "endpointslices") {
			g.Expect(rule.Verbs).To(Equal([]string{"get", "list", "watch"}))
			return
		}
	}
	t.Fatal("endpointslices rule not found")
}

func TestLibraryRBACRules_CRDsNoDelete(t *testing.T) {
	g := NewWithT(t)
	for _, rule := range LibraryRBACRules() {
		if slices.Contains(rule.Resources, "customresourcedefinitions") {
			g.Expect(rule.Verbs).NotTo(ContainElement("delete"))
			g.Expect(rule.Verbs).To(ContainElements("get", "list", "watch", "create", "update", "patch"))
			return
		}
	}
	t.Fatal("customresourcedefinitions rule not found")
}
