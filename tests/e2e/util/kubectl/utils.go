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

// appConfig defines the configuration for a test application.
type appConfig struct {
	envVar     string // Environment variable for a specific path.
	defaultDir string // Default directory name in tests/e2e/samples.
}

// appConfigs maps application names to their configuration.
var appConfigs = map[string]appConfig{
	"httpbin":             {envVar: "HTTPBIN_KUSTOMIZE_PATH", defaultDir: "httpbin"},
	"helloworld":          {envVar: "HELLOWORLD_KUSTOMIZE_PATH", defaultDir: "helloworld"},
	"sleep":               {envVar: "SLEEP_KUSTOMIZE_PATH", defaultDir: "sleep"},
	"tcp-echo-dual-stack": {envVar: "TCP_ECHO_DUAL_STACK_KUSTOMIZE_PATH", defaultDir: "tcp-echo-dual-stack"},
	"tcp-echo-ipv4":       {envVar: "TCP_ECHO_IPV4_KUSTOMIZE_PATH", defaultDir: "tcp-echo-ipv4"},
	"tcp-echo-ipv6":       {envVar: "TCP_ECHO_IPV6_KUSTOMIZE_PATH", defaultDir: "tcp-echo-ipv6"},
}

// getKustomizeDir returns the path to the Kustomize directory for a test application.
// The path is determined with the following priority:
// 1. App-specific environment variable (e.g., HTTPBIN_KUSTOMIZE_PATH).
// 2. Custom base path defined in CUSTOM_SAMPLES_PATH.
// 3. Default path within the project in this case will be: `tests/e2e/samples/httpbin“.
func getKustomizeDir(appName string) string {
	config, exists := appConfigs[appName]
	if !exists {
		return "" // return empty string if appName is not configured
	}

	// If app specific environment variable is set, use it.
	if customPath := os.Getenv(strings.ToUpper(strings.ReplaceAll(appName, "-", "_") + "_KUSTOMIZE_PATH")); customPath != "" {
		return customPath
	}

	// If CUSTOM_SAMPLES_PATH is set, use it as the base path.
	if basePath := os.Getenv("CUSTOM_SAMPLES_PATH"); basePath != "" {
		return filepath.Join(basePath, config.defaultDir)
	}

	// If no custom path is set, use the default path within the project.
	defaultBasePath := filepath.Join(project.RootDir, "tests", "e2e", "samples")
	return filepath.Join(defaultBasePath, config.defaultDir)
}
