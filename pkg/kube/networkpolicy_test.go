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

	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	. "github.com/onsi/gomega"
)

var operatorNamespace = "sail-operator"

func TestBuildNetworkPolicy(t *testing.T) {
	tests := []struct {
		name        string
		platform    config.Platform
		expectedDNS struct {
			namespace string
			podLabel  string
			ports     []string
		}
	}{
		{
			name:     "kubernetes platform",
			platform: config.PlatformKubernetes,
			expectedDNS: struct {
				namespace string
				podLabel  string
				ports     []string
			}{
				namespace: "kube-system",
				podLabel:  "k8s-app=kube-dns",
				ports:     []string{"53", "53"},
			},
		},
		{
			name:     "openshift platform",
			platform: config.PlatformOpenShift,
			expectedDNS: struct {
				namespace string
				podLabel  string
				ports     []string
			}{
				namespace: "openshift-dns",
				podLabel:  "dns.operator.openshift.io/daemonset-dns=default",
				ports:     []string{"dns-tcp", "dns"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			result := buildNetworkPolicy(operatorNamespace, tt.platform)

			g.Expect(result.Name).To(Equal(constants.NetworkPolicyName))
			g.Expect(result.Namespace).To(Equal(operatorNamespace))
			g.Expect(result.Labels).To(HaveKeyWithValue("control-plane", "sail-operator"))

			g.Expect(result.Spec.Egress).To(HaveLen(2))
			dnsRule := result.Spec.Egress[1]
			g.Expect(dnsRule.To[0].NamespaceSelector.MatchLabels).To(HaveKeyWithValue("kubernetes.io/metadata.name", tt.expectedDNS.namespace))
		})
	}
}
