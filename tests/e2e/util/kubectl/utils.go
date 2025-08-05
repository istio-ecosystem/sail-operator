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

package kubectl

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/istio-ecosystem/sail-operator/pkg/test/project"
)


// getKustomizeDir returns the path to the Kustomize directory for a test application.
// The path is determined with the following priority:
// 1. App-specific environment variable (e.g., HTTPBIN_KUSTOMIZE_PATH).
// 2. Custom base path defined in CUSTOM_SAMPLES_PATH.
// 3. Default path within the project in this case will be: `tests/e2e/samples/httpbin“.
func getKustomizeDir(appName string) string {
	// If app specific environment variable is set, use it.
	if customPath := os.Getenv(strings.ToUpper(strings.ReplaceAll(appName, "-", "_") + "_KUSTOMIZE_PATH")); customPath != "" {
		return customPath
	}

	// If CUSTOM_SAMPLES_PATH is set, use it as the base path.
	if basePath := os.Getenv("CUSTOM_SAMPLES_PATH"); basePath != "" {
		return filepath.Join(basePath, appName)
	}

	// If no custom path is set, use the default path within the project.
	defaultBasePath := filepath.Join(project.RootDir, "tests", "e2e", "samples")
	return filepath.Join(defaultBasePath, config.defaultDir)
}
