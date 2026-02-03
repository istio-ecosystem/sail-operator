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
	"os"
	"testing"
	"testing/fstest"
)

func TestLoadChart(t *testing.T) {
	testFS := os.DirFS("testdata")

	t.Run("loads chart successfully", func(t *testing.T) {
		chart, err := LoadChart(testFS, "chart")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if chart == nil {
			t.Fatal("expected chart to be non-nil")
		}
		if chart.Name() != "test-chart" {
			t.Errorf("expected chart name 'test-chart', got: %s", chart.Name())
		}
		if chart.Metadata.Version != "0.1.0" {
			t.Errorf("expected chart version '0.1.0', got: %s", chart.Metadata.Version)
		}
	})

	t.Run("returns error for non-existent path", func(t *testing.T) {
		_, err := LoadChart(testFS, "nonexistent")
		if err == nil {
			t.Fatal("expected error for non-existent path")
		}
	})

	t.Run("returns error for empty directory", func(t *testing.T) {
		emptyFS := fstest.MapFS{
			"empty/.gitkeep": &fstest.MapFile{}, // directory marker, but we skip it
		}
		// Create a truly empty directory by having only a subdirectory
		emptyDirFS := fstest.MapFS{
			"emptydir/subdir/.gitkeep": &fstest.MapFile{},
		}
		_, err := LoadChart(emptyDirFS, "emptydir/subdir")
		if err == nil {
			t.Fatal("expected error for empty directory")
		}
		_ = emptyFS // silence unused variable
	})

	t.Run("loads chart from nested path", func(t *testing.T) {
		// Create a mock filesystem with nested chart structure
		nestedFS := fstest.MapFS{
			"v1.28.0/charts/istiod/Chart.yaml": &fstest.MapFile{
				Data: []byte("apiVersion: v2\nname: istiod\nversion: 1.28.0\n"),
			},
			"v1.28.0/charts/istiod/values.yaml": &fstest.MapFile{
				Data: []byte("# default values\n"),
			},
		}

		chart, err := LoadChart(nestedFS, "v1.28.0/charts/istiod")
		if err != nil {
			t.Fatalf("expected no error, got: %v", err)
		}
		if chart.Name() != "istiod" {
			t.Errorf("expected chart name 'istiod', got: %s", chart.Name())
		}
	})
}
