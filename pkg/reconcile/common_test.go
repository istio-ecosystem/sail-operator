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

package reconcile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetChartPath(t *testing.T) {
	tests := []struct {
		version   string
		chartName string
		expected  string
	}{
		{
			version:   "v1.24.0",
			chartName: "istiod",
			expected:  "v1.24.0/charts/istiod",
		},
		{
			version:   "v1.23.0",
			chartName: "base",
			expected:  "v1.23.0/charts/base",
		},
	}

	for _, tt := range tests {
		t.Run(tt.version+"-"+tt.chartName, func(t *testing.T) {
			result := GetChartPath(tt.version, tt.chartName)
			assert.Equal(t, tt.expected, result)
		})
	}
}
