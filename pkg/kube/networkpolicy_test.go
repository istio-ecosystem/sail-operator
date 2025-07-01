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

package kube

import (
	"testing"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	. "github.com/onsi/gomega"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"istio.io/istio/pkg/ptr"
)

func TestGetIstiodNetworkPolicyName(t *testing.T) {
	tests := []struct {
		name     string
		revision *v1.IstioRevision
		expected string
	}{
		{
			name: "default revision",
			revision: &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{Name: "default"},
				Spec: v1.IstioRevisionSpec{
					Values: &v1.Values{
						Revision: ptr.Of(""),
					},
				},
			},
			expected: "istio-istiod",
		},
		{
			name: "named revision",
			revision: &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{Name: "stable"},
				Spec: v1.IstioRevisionSpec{
					Values: &v1.Values{
						Revision: ptr.Of("stable"),
					},
				},
			},
			expected: "istio-istiod-stable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			result := GetIstiodNetworkPolicyName(tt.revision)
			g.Expect(result).To(Equal(tt.expected))
		})
	}
}

func TestBuildIstiodNetworkPolicy(t *testing.T) {
	tests := []struct {
		name     string
		revision *v1.IstioRevision
		expected struct {
			name     string
			appLabel string
		}
	}{
		{
			name: "default revision",
			revision: &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "default",
					UID:  "test-uid",
				},
				Spec: v1.IstioRevisionSpec{
					Namespace: "istio-system",
					Values: &v1.Values{
						Revision: ptr.Of(""),
					},
				},
			},
			expected: struct {
				name     string
				appLabel string
			}{
				name:     "istio-istiod",
				appLabel: "istiod",
			},
		},
		{
			name: "named revision",
			revision: &v1.IstioRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name: "stable",
					UID:  "test-uid-stable",
				},
				Spec: v1.IstioRevisionSpec{
					Namespace: "istio-system",
					Values: &v1.Values{
						Revision: ptr.Of("stable"),
					},
				},
			},
			expected: struct {
				name     string
				appLabel string
			}{
				name:     "istio-istiod-stable",
				appLabel: "istiod",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			result := BuildIstiodNetworkPolicy(tt.revision)

			g.Expect(result.Name).To(Equal(tt.expected.name))
			g.Expect(result.Namespace).To(Equal("istio-system"))
			g.Expect(result.Labels).To(HaveKeyWithValue("app", tt.expected.appLabel))
			g.Expect(result.Spec.PodSelector.MatchLabels).To(HaveKeyWithValue("app", tt.expected.appLabel))

			// Verify owner reference
			g.Expect(result.OwnerReferences).To(HaveLen(1))
			g.Expect(result.OwnerReferences[0].Name).To(Equal(tt.revision.Name))
			g.Expect(result.OwnerReferences[0].UID).To(Equal(tt.revision.UID))
			g.Expect(*result.OwnerReferences[0].Controller).To(BeTrue())

			// Verify policy types
			g.Expect(result.Spec.PolicyTypes).To(ContainElements(
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			))
		})
	}
}
