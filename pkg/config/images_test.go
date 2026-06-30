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

package config

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseImageDigestsFromAnnotations(t *testing.T) {
	rhIstiod := "registry.redhat.io/openshift-service-mesh/istio-pilot-rhel9@sha256:abc123"
	rhProxy := "registry.redhat.io/openshift-service-mesh/istio-proxyv2-rhel9@sha256:def456"
	rhCNI := "registry.redhat.io/openshift-service-mesh/istio-cni-rhel9@sha256:ghi789"
	rhZtunnel := "registry.redhat.io/openshift-service-mesh/istio-ztunnel-rhel9@sha256:jkl012"

	testCases := []struct {
		name    string
		ann     map[string]string
		want    map[string]IstioImageConfig
		wantErr bool
	}{
		{
			name: "rh digest annotations",
			ann: map[string]string{
				"images.v1_30_1.istiod":  rhIstiod,
				"images.v1_30_1.proxy":   rhProxy,
				"images.v1_30_1.cni":     rhCNI,
				"images.v1_30_1.ztunnel": rhZtunnel,
			},
			want: map[string]IstioImageConfig{
				"v1.30.1": {
					IstiodImage:  rhIstiod,
					ProxyImage:   rhProxy,
					CNIImage:     rhCNI,
					ZTunnelImage: rhZtunnel,
				},
			},
		},
		{
			name: "ignores unrelated annotations",
			ann: map[string]string{
				"foo":                    "bar",
				"images.v1_30_1.istiod":  rhIstiod,
				"images.v1_30_1.proxy":   rhProxy,
				"images.v1_30_1.cni":     rhCNI,
				"images.v1_30_1.ztunnel": rhZtunnel,
			},
			want: map[string]IstioImageConfig{
				"v1.30.1": {
					IstiodImage:  rhIstiod,
					ProxyImage:   rhProxy,
					CNIImage:     rhCNI,
					ZTunnelImage: rhZtunnel,
				},
			},
		},
		{
			name: "ignores must-gather",
			ann: map[string]string{
				"images.v1_30_1.istiod":       rhIstiod,
				"images.v1_30_1.proxy":        rhProxy,
				"images.v1_30_1.cni":          rhCNI,
				"images.v1_30_1.ztunnel":      rhZtunnel,
				"images.v1_30_1.must-gather":  "registry.example.com/must-gather:latest",
			},
			want: map[string]IstioImageConfig{
				"v1.30.1": {
					IstiodImage:  rhIstiod,
					ProxyImage:   rhProxy,
					CNIImage:     rhCNI,
					ZTunnelImage: rhZtunnel,
				},
			},
		},
		{
			name: "empty annotations",
			ann:  map[string]string{},
			want: map[string]IstioImageConfig{},
		},
		{
			name: "no matching prefix",
			ann: map[string]string{
				"other.key": "value",
			},
			want: map[string]IstioImageConfig{},
		},
		{
			name: "rejects unresolved placeholder",
			ann: map[string]string{
				"images.v1_30_1.istiod": "${ISTIO_PILOT_1_30}",
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseImageDigestsFromAnnotations(tc.ann)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Fatal("digests did not match expectation:\n", diff)
			}
		})
	}
}

func TestMergeImageDigests(t *testing.T) {
	orig := Config.ImageDigests
	t.Cleanup(func() {
		Config.ImageDigests = orig
	})

	Config.ImageDigests = map[string]IstioImageConfig{
		"v1.29.0": testImages,
	}

	overlay := map[string]IstioImageConfig{
		"v1.30.1": {
			IstiodImage: "overlay-istiod",
		},
	}
	MergeImageDigests(overlay)

	want := map[string]IstioImageConfig{
		"v1.29.0": testImages,
		"v1.30.1": {IstiodImage: "overlay-istiod"},
	}
	if diff := cmp.Diff(want, Config.ImageDigests); diff != "" {
		t.Fatal("merged config did not match expectation:\n", diff)
	}
}
