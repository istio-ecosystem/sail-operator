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

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/istiovalues"

	"istio.io/istio/pkg/ptr"
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
apiVersion: sailoperator.io/v1
kind: IstioRevision
spec:
  values:
    pilot: 
      hub: from-default-profile
      tag: from-default-profile      # this gets overridden in my-profile
      image: from-default-profile    # this gets overridden in my-profile and values`)), 0o644))

	Must(t, os.WriteFile(path.Join(profilesDir, "my-profile.yaml"), []byte((`
apiVersion: sailoperator.io/v1
kind: IstioRevision
spec:
  values:
    pilot:
      tag: from-my-profile
      image: from-my-profile  # this gets overridden in values`)), 0o644))

	values := &v1.Values{
		Pilot: &v1.PilotConfig{
			Image: ptr.Of("from-istio-spec-values"),
		},
	}

	result, err := ComputeValues(values, namespace, version, config.PlatformOpenShift, "default", "my-profile", resourceDir, revisionName)
	if err != nil {
		t.Errorf("Expected no error, but got an error: %v", err)
	}

	expected := &v1.Values{
		Pilot: &v1.PilotConfig{
			Hub:   ptr.Of("from-default-profile"),
			Tag:   ptr.Of("from-my-profile"),
			Image: ptr.Of("from-istio-spec-values"),
		},
		Global: &v1.GlobalConfig{
			Platform:       ptr.Of("openshift"),
			IstioNamespace: ptr.Of(namespace), // this value is always added/overridden based on IstioRevision.spec.namespace
		},
		Revision:        ptr.Of(revisionName),
		DefaultRevision: ptr.Of(""),
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Result does not match the expected Values.\nExpected: %v\nActual: %v", expected, result)
	}
}

// TestFipsComputeValues tests that the pilot.env.COMPLIANCE_POLICY is set in values
func TestFipsComputeValues(t *testing.T) {
	const (
		namespace    = "istio-system"
		version      = "my-version"
		revisionName = "my-revision"
	)
	resourceDir := t.TempDir()
	profilesDir := path.Join(resourceDir, version, "profiles")
	Must(t, os.MkdirAll(profilesDir, 0o755))

	Must(t, os.WriteFile(path.Join(profilesDir, "default.yaml"), []byte((`
apiVersion: sailoperator.io/v1
kind: IstioRevision
spec:`)), 0o644))

	istiovalues.FipsEnabled = true
	values := &v1.Values{}
	result, err := ComputeValues(values, namespace, version, config.PlatformOpenShift, "default", "",
		resourceDir, revisionName)
	if err != nil {
		t.Errorf("Expected no error, but got an error: %v", err)
	}

	expected := &v1.Values{
		Pilot: &v1.PilotConfig{
			Env: map[string]string{"COMPLIANCE_POLICY": "fips-140-2"},
		},
		Global: &v1.GlobalConfig{
			Platform:       ptr.Of("openshift"),
			IstioNamespace: ptr.Of(namespace), // this value is always added/overridden based on IstioRevision.spec.namespace
		},
		Revision:        ptr.Of(revisionName),
		DefaultRevision: ptr.Of(""),
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Result does not match the expected Values.\nExpected: %v\nActual: %v", expected, result)
	}
} // when checking a temp test file content.

func Must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
