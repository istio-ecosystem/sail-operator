//go:build e2e

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

package operandimages

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-cmp/cmp"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"gopkg.in/yaml.v3"
)

const sampleCSV = `
spec:
  install:
    spec:
      deployments:
        - name: sail-operator
          spec:
            template:
              metadata:
                annotations:
                  images.v1_30_1.istiod: registry.istio.io/testing/pilot@sha256:abc
                  images.v1_30_1.proxy: registry.istio.io/testing/proxyv2@sha256:def
                  images.v1_30_1.cni: registry.istio.io/testing/install-cni@sha256:ghi
                  images.v1_30_1.ztunnel: registry.istio.io/testing/ztunnel@sha256:jkl
        - name: other-deployment
          spec:
            template:
              metadata:
                annotations: {}
`

func TestParseInstallDeploymentAnnotations(t *testing.T) {
	ann, err := parseInstallDeploymentAnnotations(sampleCSV, "sail-operator")
	if err != nil {
		t.Fatal(err)
	}

	digests, err := config.ParseImageDigestsFromAnnotations(ann)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]config.IstioImageConfig{
		"v1.30.1": {
			IstiodImage:  "registry.istio.io/testing/pilot@sha256:abc",
			ProxyImage:   "registry.istio.io/testing/proxyv2@sha256:def",
			CNIImage:     "registry.istio.io/testing/install-cni@sha256:ghi",
			ZTunnelImage: "registry.istio.io/testing/ztunnel@sha256:jkl",
		},
	}
	if diff := cmp.Diff(want, digests); diff != "" {
		t.Fatal("digests did not match expectation:\n", diff)
	}
}

func TestParseInstallDeploymentAnnotationsMissingDeployment(t *testing.T) {
	_, err := parseInstallDeploymentAnnotations(sampleCSV, "missing")
	if err == nil {
		t.Fatal("expected error for missing deployment")
	}
}

func TestCSVSemver(t *testing.T) {
	version, err := csvSemver("sailoperator.v1.30.0")
	if err != nil {
		t.Fatal(err)
	}
	if version.String() != "1.30.0" {
		t.Fatalf("expected 1.30.0, got %s", version)
	}
}

func TestHighestSucceededCSVFromYAML(t *testing.T) {
	const csvListYAML = `
items:
  - metadata:
      name: sailoperator.v1.28.0
    status:
      phase: Succeeded
  - metadata:
      name: sailoperator.v1.30.0
    status:
      phase: Succeeded
  - metadata:
      name: sailoperator.v1.27.0
    status:
      phase: Failed
`
	var list csvList
	if err := yaml.Unmarshal([]byte(csvListYAML), &list); err != nil {
		t.Fatal(err)
	}

	var (
		bestName    string
		bestVersion = mustSemver(t, "0.0.0")
	)
	for _, item := range list.Items {
		if item.Status.Phase != csvSucceededPhase {
			continue
		}
		version, err := csvSemver(item.Metadata.Name)
		if err != nil {
			continue
		}
		if version.GreaterThan(bestVersion) {
			bestVersion = version
			bestName = item.Metadata.Name
		}
	}
	if bestName != "sailoperator.v1.30.0" {
		t.Fatalf("expected sailoperator.v1.30.0, got %q", bestName)
	}
}

func mustSemver(t *testing.T, v string) *semver.Version {
	t.Helper()
	sv, err := semver.NewVersion(v)
	if err != nil {
		t.Fatal(err)
	}
	return sv
}
