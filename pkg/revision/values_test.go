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

package revision

import (
	"os"
	"path"
	"reflect"
	"testing"

	"github.com/istio-ecosystem/sail-operator/api/v1alpha1"
)

// TestComputeValues tests that the values are sourced from the following sources
// (with each source overriding the values from the previous sources):
//   - default profile(s)
//   - profile selected in IstioRevision.spec.profile
//   - IstioRevision.spec.values
//   - other (non-value) fields in the IstioRevision resource (e.g. the value global.istioNamespace is set from IstioRevision.spec.namespace)
func TestComputeValues(t *testing.T) {
	const (
		namespace    = "istio-system"
		version      = "my-version"
		revisionName = "my-revision"
	)
	resourceDir := t.TempDir()
	profilesDir := path.Join(resourceDir, version, "profiles")
	Must(t, os.MkdirAll(profilesDir, 0o755))

	Must(t, os.WriteFile(path.Join(profilesDir, "default.yaml"), []byte((`
apiVersion: sailoperator.io/v1alpha1
kind: IstioRevision
spec:
  values:
    pilot: 
      hub: from-default-profile
      tag: from-default-profile      # this gets overridden in my-profile
      image: from-default-profile    # this gets overridden in my-profile and values`)), 0o644))

	Must(t, os.WriteFile(path.Join(profilesDir, "my-profile.yaml"), []byte((`
apiVersion: sailoperator.io/v1alpha1
kind: IstioRevision
spec:
  values:
    pilot:
      tag: from-my-profile
      image: from-my-profile  # this gets overridden in values`)), 0o644))

	values := &v1alpha1.Values{
		Pilot: &v1alpha1.PilotConfig{
			Image: "from-istio-spec-values",
		},
	}

	result, err := ComputeValues(values, namespace, version, "default", "my-profile", resourceDir, revisionName)
	if err != nil {
		t.Errorf("Expected no error, but got an error: %v", err)
	}

	expected := &v1alpha1.Values{
		Pilot: &v1alpha1.PilotConfig{
			Hub:   "from-default-profile",
			Tag:   "from-my-profile",
			Image: "from-istio-spec-values",
		},
		Global: &v1alpha1.GlobalConfig{
			IstioNamespace: namespace, // this value is always added/overridden based on IstioRevision.spec.namespace
		},
		Revision: revisionName,
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Result does not match the expected Values.\nExpected: %v\nActual: %v", expected, result)
	}
}

func Must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
