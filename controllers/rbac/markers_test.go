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

package rbac

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

var remoteIstioResourcePattern = regexp.MustCompile(`(?m)^\s*-\s+remoteistios?(?:/(?:finalizers|status))?\s*$`)

func TestKubebuilderRBACMarkersDoNotUseWildcards(t *testing.T) {
	repoRoot := repositoryRoot(t)
	err := filepath.WalkDir(filepath.Join(repoRoot, "controllers"), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		for lineNum, line := range strings.Split(string(content), "\n") {
			if !strings.Contains(line, "+kubebuilder:rbac:") {
				continue
			}
			if strings.Contains(line, `resources="*"`) || strings.Contains(line, `verbs="*"`) {
				t.Errorf("%s:%d uses wildcard kubebuilder RBAC marker: %s", path, lineNum+1, line)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSharedMarkersCoverStartupAndRBACEscalationPermissions(t *testing.T) {
	content := mustReadFile(t, filepath.Join(repositoryRoot(t), "controllers", "rbac", "markers.go"))

	requiredMarkers := []string{
		`groups="config.openshift.io",resources=apiservers;clusterversions,verbs=get;list;watch`,
		`groups="rbac.authorization.k8s.io",resources=clusterrolebindings;clusterroles;rolebindings;roles,verbs=bind;escalate`,
		`groups=sailoperator.io,resources=istiorevisiontags,verbs=get;list;watch;create;update;patch;delete`,
	}
	for _, marker := range requiredMarkers {
		if !strings.Contains(content, marker) {
			t.Fatalf("expected shared RBAC marker %q", marker)
		}
	}
}

func TestShippedRBACNoLongerGrantsRemoteIstioPermissions(t *testing.T) {
	repoRoot := repositoryRoot(t)
	files := []string{
		filepath.Join(repoRoot, "chart", "templates", "rbac", "role.yaml"),
		filepath.Join(repoRoot, "bundle", "manifests", "sailoperator.clusterserviceversion.yaml"),
	}
	for _, path := range files {
		if remoteIstioResourcePattern.MatchString(mustReadFile(t, path)) {
			t.Fatalf("%s still grants remoteistios RBAC", path)
		}
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to determine test file path")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func mustReadFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
