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
	"testing"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/stretchr/testify/assert"

	"istio.io/istio/pkg/ptr"
)

func TestDependsOnIstioCNI(t *testing.T) {
	tests := []struct {
		name     string
		rev      *v1.IstioRevision
		expected bool
	}{
		{
			name: "NilValues",
			rev: &v1.IstioRevision{
				Spec: v1.IstioRevisionSpec{
					Values: nil,
				},
			},
			expected: false,
		},
		{
			name: "NilGlobal",
			rev: &v1.IstioRevision{
				Spec: v1.IstioRevisionSpec{
					Values: &v1.Values{
						Global: nil,
					},
				},
			},
			expected: false,
		},
		{
			name: "NilGlobalPlatform",
			rev: &v1.IstioRevision{
				Spec: v1.IstioRevisionSpec{
					Values: &v1.Values{
						Global: &v1.GlobalConfig{
							Platform: nil,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "GlobalPlatformOpenshift",
			rev: &v1.IstioRevision{
				Spec: v1.IstioRevisionSpec{
					Values: &v1.Values{
						Global: &v1.GlobalConfig{
							Platform: ptr.Of("openshift"),
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "GlobalPlatformNotOpenshift",
			rev: &v1.IstioRevision{
				Spec: v1.IstioRevisionSpec{
					Values: &v1.Values{
						Global: &v1.GlobalConfig{
							Platform: ptr.Of("kind"),
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "NilPilot",
			rev: &v1.IstioRevision{
				Spec: v1.IstioRevisionSpec{
					Values: &v1.Values{
						Pilot: nil,
					},
				},
			},
			expected: false,
		},
		{
			name: "NilPilotCni",
			rev: &v1.IstioRevision{
				Spec: v1.IstioRevisionSpec{
					Values: &v1.Values{
						Pilot: &v1.PilotConfig{
							Cni: nil,
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "NilPilotCniEnabled",
			rev: &v1.IstioRevision{
				Spec: v1.IstioRevisionSpec{
					Values: &v1.Values{
						Pilot: &v1.PilotConfig{
							Cni: &v1.CNIUsageConfig{
								Enabled: nil,
							},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "PilotCniEnabled",
			rev: &v1.IstioRevision{
				Spec: v1.IstioRevisionSpec{
					Values: &v1.Values{
						Pilot: &v1.PilotConfig{
							Cni: &v1.CNIUsageConfig{
								Enabled: ptr.Of(true),
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "PilotCniDisabled",
			rev: &v1.IstioRevision{
				Spec: v1.IstioRevisionSpec{
					Values: &v1.Values{
						Pilot: &v1.PilotConfig{
							Cni: &v1.CNIUsageConfig{
								Enabled: ptr.Of(false),
							},
						},
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DependsOnIstioCNI(tt.rev)
			assert.Equal(t, tt.expected, result)
		})
	}
}
