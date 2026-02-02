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

// Package resources provides embedded Istio Helm charts and profiles.
//
// This package embeds all version directories (v1.28.2, etc.) containing
// Helm charts and profiles. Importing this package will increase the binary
// size significantly (~10MB) as it includes all chart files.
//
// This package is intended for consumers who want to embed the charts
// directly in their binary.
//
// Usage:
//
//	import "github.com/istio-ecosystem/sail-operator/resources"
//
//	cfg := config.ReconcilerConfig{
//	    ResourceFS: resources.FS,
//	}
//
// The embedded paths are relative to this directory, e.g.:
//   - v1.28.2/charts/istiod/Chart.yaml
//   - v1.28.2/profiles/default.yaml
package resources

import (
	"embed"
	"io/fs"
)

// FS contains the embedded resources directory with all Helm charts and profiles.
// Paths are relative to this directory (e.g., "v1.28.2/charts/istiod").
//
//go:embed all:v*
var FS embed.FS

// SubFS creates a sub-filesystem rooted at the specified directory.
// This is useful for stripping prefixes from embedded filesystems.
//
// Example:
//
//	// If you have your own embed with a prefix:
//	//go:embed my-resources
//	var rawFS embed.FS
//	fs := resources.SubFS(rawFS, "my-resources")
func SubFS(fsys fs.FS, dir string) (fs.FS, error) {
	return fs.Sub(fsys, dir)
}

// MustSubFS is like SubFS but panics on error.
// Use this when the directory is known to exist.
func MustSubFS(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic("failed to create sub-filesystem for " + dir + ": " + err.Error())
	}
	return sub
}
