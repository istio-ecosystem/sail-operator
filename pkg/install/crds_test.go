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
	"testing/fstest"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/chart"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAggregateState(t *testing.T) {
	tests := []struct {
		name          string
		infos         []CRDInfo
		expectState   CRDManagementState
		expectMessage string
	}{
		{
			name:          "empty list",
			infos:         nil,
			expectState:   CRDManagementStateUnknown,
			expectMessage: "no CRDs to manage",
		},
		{
			name: "all ready",
			infos: []CRDInfo{
				{Name: "a.example.com", Ready: true, Managed: true},
				{Name: "b.example.com", Ready: true, Managed: true},
			},
			expectState:   CRDManagementStateReady,
			expectMessage: "",
		},
		{
			name: "one not ready",
			infos: []CRDInfo{
				{Name: "a.example.com", Ready: true, Managed: true},
				{Name: "b.example.com", Ready: false, Managed: true},
			},
			expectState:   CRDManagementStateNotReady,
			expectMessage: "not all CRDs are ready",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			state, msg := AggregateState(tc.infos)
			g.Expect(state).To(Equal(tc.expectState))
			g.Expect(msg).To(Equal(tc.expectMessage))
		})
	}
}

func TestClassifyCRD(t *testing.T) {
	m := &crdManager{}

	tests := []struct {
		name   string
		crd    *apiextensionsv1.CustomResourceDefinition
		expect crdOwnership
	}{
		{
			name: "OLM managed",
			crd: &apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{OLMManagedLabel: "test-operator"},
				},
			},
			expect: crdManagedByOLM,
		},
		{
			name: "library managed",
			crd: &apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app.kubernetes.io/managed-by": managedByValue},
				},
			},
			expect: crdManagedByLibrary,
		},
		{
			name: "unmanaged",
			crd: &apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"some-other-label": "value"},
				},
			},
			expect: crdUnmanaged,
		},
		{
			name:   "no labels",
			crd:    &apiextensionsv1.CustomResourceDefinition{},
			expect: crdUnmanaged,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(m.classifyCRD(tc.crd)).To(Equal(tc.expect))
		})
	}
}

func TestIsCRDReady(t *testing.T) {
	m := &crdManager{}

	tests := []struct {
		name   string
		crd    *apiextensionsv1.CustomResourceDefinition
		expect bool
	}{
		{
			name: "established",
			crd: &apiextensionsv1.CustomResourceDefinition{
				Status: apiextensionsv1.CustomResourceDefinitionStatus{
					Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
						{Type: apiextensionsv1.Established, Status: apiextensionsv1.ConditionTrue},
					},
				},
			},
			expect: true,
		},
		{
			name: "not established",
			crd: &apiextensionsv1.CustomResourceDefinition{
				Status: apiextensionsv1.CustomResourceDefinitionStatus{
					Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
						{Type: apiextensionsv1.Established, Status: apiextensionsv1.ConditionFalse},
					},
				},
			},
			expect: false,
		},
		{
			name:   "no conditions",
			crd:    &apiextensionsv1.CustomResourceDefinition{},
			expect: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(m.isCRDReady(tc.crd)).To(Equal(tc.expect))
		})
	}
}

const testManifests = `
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: virtualservices.networking.istio.io
spec:
  group: networking.istio.io
  names:
    kind: VirtualService
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: gateways.networking.istio.io
spec:
  group: networking.istio.io
  names:
    kind: Gateway
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: istio
`

func TestLoadCRDs_filterByPilotInclude(t *testing.T) {
	g := NewWithT(t)
	vals := &v1.Values{
		Pilot: &v1.PilotConfig{
			Env: map[string]string{
				"PILOT_INCLUDE_RESOURCES": "VirtualService.networking.istio.io",
			},
		},
	}
	m := &crdManager{crdFS: fstest.MapFS{
		"crds.yaml": &fstest.MapFile{Data: []byte(testManifests)},
	}}
	crds, err := m.loadCRDs(Options{ManageCRDs: true, Values: vals})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(crds).To(HaveLen(1))
	g.Expect(crds[0].Name).To(Equal("virtualservices.networking.istio.io"))
}

func TestLoadCRDs_noFilterIncludesAllCRDs(t *testing.T) {
	g := NewWithT(t)
	m := &crdManager{crdFS: fstest.MapFS{
		"crds.yaml": &fstest.MapFile{Data: []byte(testManifests)},
	}}
	crds, err := m.loadCRDs(Options{ManageCRDs: true})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(crds).To(HaveLen(2))
}

func TestMatchesCRDFilter(t *testing.T) {
	tests := []struct {
		name        string
		kind        string
		group       string
		targetKinds map[string]bool
		expect      bool
	}{
		{
			name:        "nil targetKinds matches all",
			kind:        "VirtualService",
			group:       "networking.istio.io",
			targetKinds: nil,
			expect:      true,
		},
		{
			name:        "empty targetKinds matches all",
			kind:        "VirtualService",
			group:       "networking.istio.io",
			targetKinds: map[string]bool{},
			expect:      true,
		},
		{
			name:        "matching kind",
			kind:        "VirtualService",
			group:       "networking.istio.io",
			targetKinds: map[string]bool{"virtualservice.networking.istio.io": true},
			expect:      true,
		},
		{
			name:        "non-matching kind",
			kind:        "Gateway",
			group:       "networking.istio.io",
			targetKinds: map[string]bool{"virtualservice.networking.istio.io": true},
			expect:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			crd := &apiextensionsv1.CustomResourceDefinition{
				Spec: apiextensionsv1.CustomResourceDefinitionSpec{
					Names: apiextensionsv1.CustomResourceDefinitionNames{Kind: tc.kind},
					Group: tc.group,
				},
			}
			g.Expect(matchesCRDFilter(crd, tc.targetKinds)).To(Equal(tc.expect))
		})
	}
}

func TestLoadCRDs(t *testing.T) {
	tests := []struct {
		name      string
		files     fstest.MapFS
		opts      Options
		expectLen int
	}{
		{
			name: "loads CRD from yaml file",
			files: fstest.MapFS{
				"crds.yaml": &fstest.MapFile{Data: []byte(testManifests)},
			},
			opts:      Options{ManageCRDs: true, IncludeAllCRDs: true},
			expectLen: 2,
		},
		{
			name: "skips non-yaml files",
			files: fstest.MapFS{
				"crds.yaml": &fstest.MapFile{Data: []byte(testManifests)},
				"readme.md": &fstest.MapFile{Data: []byte("not a CRD")},
			},
			opts:      Options{ManageCRDs: true, IncludeAllCRDs: true},
			expectLen: 2,
		},
		{
			name:      "empty directory",
			files:     fstest.MapFS{},
			opts:      Options{ManageCRDs: true, IncludeAllCRDs: true},
			expectLen: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			m := &crdManager{crdFS: tc.files}
			crds, err := m.loadCRDs(tc.opts)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(crds).To(HaveLen(tc.expectLen))
		})
	}
}

func TestUnmanagedCRDNotTakenOver(t *testing.T) {
	g := NewWithT(t)

	existingVS := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "virtualservices.networking.istio.io",
			Labels:      map[string]string{"some-label": "some-value"},
			Annotations: map[string]string{"some-annotation": "ann-value"},
		},
		Status: apiextensionsv1.CustomResourceDefinitionStatus{
			Conditions: []apiextensionsv1.CustomResourceDefinitionCondition{
				{Type: apiextensionsv1.Established, Status: apiextensionsv1.ConditionTrue},
			},
		},
	}
	existingGW := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "gateways.networking.istio.io",
			Labels:      map[string]string{"some-label": "some-value"},
			Annotations: map[string]string{"some-annotation": "ann-value"},
		},
	}

	s := runtime.NewScheme()
	g.Expect(apiextensionsv1.AddToScheme(s)).To(Succeed())
	cl := fake.NewClientBuilder().WithScheme(s).WithObjects(existingVS, existingGW).Build()

	m := &crdManager{
		cl:    cl,
		crdFS: chart.CRDsFS,
	}

	ctx := t.Context()
	infos, err := m.Reconcile(ctx, Options{ManageCRDs: true, IncludeAllCRDs: true})
	g.Expect(err).NotTo(HaveOccurred())

	for _, info := range infos {
		var updated apiextensionsv1.CustomResourceDefinition
		g.Expect(cl.Get(ctx, types.NamespacedName{Name: info.Name}, &updated)).To(Succeed())
		if info.Name == "virtualservices.networking.istio.io" || info.Name == "gateways.networking.istio.io" {
			g.Expect(info.Managed).To(BeFalse(), "unmanaged CRD %s should not be taken over", info.Name)
			g.Expect(updated.Labels).NotTo(HaveKey(kubeManagedByLabel), "managed-by label should not be added to unmanaged CRD")
			g.Expect(updated.Labels).To(Equal(existingGW.Labels), "existing labels should not be modified")
			g.Expect(updated.Annotations).To(Equal(existingGW.Annotations), "existing annotations should not be modified")
		} else {
			g.Expect(info.Managed).To(BeTrue(), "CRD %s should be managed by the library", info.Name)
			g.Expect(updated.Labels).To(HaveKey(kubeManagedByLabel))
			g.Expect(updated.Labels[kubeManagedByLabel]).To(Equal(managedByValue))
		}
	}
}

func TestTargetCRDKinds(t *testing.T) {
	tests := []struct {
		name       string
		includeAll bool
		values     *v1.Values
		expectNil  bool
		expectKeys []string
	}{
		{
			name:       "includeAll returns nil",
			includeAll: true,
			expectNil:  true,
		},
		{
			name:      "nil values returns nil",
			values:    nil,
			expectNil: true,
		},
		{
			name: "parses PILOT_INCLUDE_RESOURCES",
			values: &v1.Values{
				Pilot: &v1.PilotConfig{
					Env: map[string]string{
						"PILOT_INCLUDE_RESOURCES": "VirtualService.networking.istio.io, Gateway.networking.istio.io",
					},
				},
			},
			expectKeys: []string{
				"virtualservice.networking.istio.io",
				"gateway.networking.istio.io",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			result := targetCRDKinds(tc.includeAll, tc.values)
			if tc.expectNil {
				g.Expect(result).To(BeNil())
			} else {
				for _, key := range tc.expectKeys {
					g.Expect(result).To(HaveKey(key))
				}
			}
		})
	}
}
