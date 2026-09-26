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

// Package perses provides embedded PersesDashboard manifests vendored from community-mixins.
//
// Paths are relative to this directory, e.g.:
//   - dashboards/istio-control-plane.yaml
//   - dashboards/README.md
package perses

import (
	"embed"
	"io/fs"
)

// FS contains embedded Perses dashboard manifests.
//
//go:embed dashboards
var FS embed.FS

// SubFS creates a sub-filesystem rooted at the specified directory.
func SubFS(fsys fs.FS, dir string) (fs.FS, error) {
	return fs.Sub(fsys, dir)
}
