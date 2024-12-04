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

package version

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	. "github.com/onsi/gomega"
)

func TestConstraint(t *testing.T) {
	t.Run("valid constraint", func(t *testing.T) {
		_ = Constraint(">1.0.0")
	})

	t.Run("invalid constraint", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for invalid constraint")
			}
		}()
		_ = Constraint("invalid_version")
	})
}

func TestIsSupported(t *testing.T) {
	testCases := []struct {
		name      string
		version   string
		expectErr string
		config    config.OperatorConfig
	}{
		{
			name:      "empty version",
			version:   "",
			expectErr: "spec.version not set",
		},
		{
			name:      "invalid version",
			version:   "v.-1.0",
			expectErr: "spec.version is not a valid semver",
		},
		{
			name:      "version higher than maximum istio version",
			version:   "v2.1.0",
			expectErr: "spec.version is not supported",
			config: config.OperatorConfig{
				MaximumIstioVersion: semver.MustParse("v2.0.0"),
			},
		},
		{
			name:    "latest version",
			version: "latest",
		},
	}
	for _, tc := range testCases {
		config.Config = tc.config
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			err := IsSupported(tc.version)
			if tc.expectErr == "" {
				g.Expect(err).ToNot(HaveOccurred())
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectErr))
			}
		})
	}
}
