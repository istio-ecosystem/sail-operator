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

package helm

import (
	"fmt"
	"io/fs"
	"strings"

	"helm.sh/helm/v3/pkg/chart"
	chartLoader "helm.sh/helm/v3/pkg/chart/loader"
)

// LoadChart loads a Helm chart from an fs.FS at the specified path.
// This allows loading charts from embed.FS, os.DirFS, or any other fs.FS implementation.
//
// The chartPath should be the path to the chart directory within the filesystem,
// e.g., "v1.28.2/charts/istiod".
func LoadChart(resourceFS fs.FS, chartPath string) (*chart.Chart, error) {
	var files []*chartLoader.BufferedFile

	err := fs.WalkDir(resourceFS, chartPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		data, err := fs.ReadFile(resourceFS, path)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %w", path, err)
		}

		// Make path relative to chart root
		// e.g., "v1.28.2/charts/istiod/Chart.yaml" -> "Chart.yaml"
		relPath := strings.TrimPrefix(path, chartPath)
		relPath = strings.TrimPrefix(relPath, "/")

		files = append(files, &chartLoader.BufferedFile{
			Name: relPath,
			Data: data,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk chart directory %s: %w", chartPath, err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no files found in chart directory %s", chartPath)
	}

	loadedChart, err := chartLoader.LoadFiles(files)
	if err != nil {
		return nil, fmt.Errorf("failed to load chart from files: %w", err)
	}

	return loadedChart, nil
}
