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
	"testing"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	. "github.com/onsi/gomega"

	"istio.io/istio/pkg/ptr"
)

func TestMergeValues(t *testing.T) {
	tests := []struct {
		name    string
		base    *v1.Values
		overlay *v1.Values
		check   func(g Gomega, result *v1.Values)
	}{
		{
			name:    "nil base returns overlay",
			base:    nil,
			overlay: &v1.Values{Global: &v1.GlobalConfig{IstioNamespace: ptr.Of("custom")}},
			check: func(g Gomega, result *v1.Values) {
				g.Expect(result.Global.IstioNamespace).To(Equal(ptr.Of("custom")))
			},
		},
		{
			name:    "nil overlay returns base",
			base:    &v1.Values{Global: &v1.GlobalConfig{IstioNamespace: ptr.Of("base-ns")}},
			overlay: nil,
			check: func(g Gomega, result *v1.Values) {
				g.Expect(result.Global.IstioNamespace).To(Equal(ptr.Of("base-ns")))
			},
		},
		{
			name: "overlay takes precedence",
			base: &v1.Values{
				Pilot: &v1.PilotConfig{
					Env: map[string]string{"KEY1": "base-val", "KEY2": "base-val"},
				},
			},
			overlay: &v1.Values{
				Pilot: &v1.PilotConfig{
					Env: map[string]string{"KEY1": "overlay-val"},
				},
			},
			check: func(g Gomega, result *v1.Values) {
				g.Expect(result.Pilot.Env).To(HaveKeyWithValue("KEY1", "overlay-val"))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			result, err := MergeValues(tc.base, tc.overlay)
			g.Expect(err).NotTo(HaveOccurred())
			tc.check(g, result)
		})
	}
}
