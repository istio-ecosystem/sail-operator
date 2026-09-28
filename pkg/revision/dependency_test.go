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
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/stretchr/testify/assert"
)

func TestDependsOnIstioCNI(t *testing.T) {
	defaultCfg := config.ReconcilerConfig{
		Platform:       config.PlatformKubernetes,
		DefaultProfile: "default",
	}

	tests := []struct {
		name     string
		rev      *v1.IstioRevision
		cfg      config.ReconcilerConfig
		expected bool
	}{
		{
			name: "NilValues",
			rev: &v1.IstioRevision{
				Spec: v1.IstioRevisionSpec{
					Values: nil,
				},
			},
			cfg:      defaultCfg,
			expected: false,
		},
		{
			name: "PlatformOpenshift",
			rev: &v1.IstioRevision{
				Spec: v1.IstioRevisionSpec{
					Values: nil,
				},
			},
			cfg: config.ReconcilerConfig{
				Platform: config.PlatformOpenShift,
			},
			expected: true,
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
			cfg:      defaultCfg,
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
			cfg:      defaultCfg,
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
			cfg:      defaultCfg,
			expected: false,
		},
		{
			name: "PilotCniEnabled",
			rev: &v1.IstioRevision{
				Spec: v1.IstioRevisionSpec{
					Values: &v1.Values{
						Pilot: &v1.PilotConfig{
							Cni: &v1.CNIUsageConfig{
								Enabled: new(true),
							},
						},
					},
				},
			},
			cfg:      defaultCfg,
			expected: true,
		},
		{
			name: "PilotCniDisabled",
			rev: &v1.IstioRevision{
				Spec: v1.IstioRevisionSpec{
					Values: &v1.Values{
						Pilot: &v1.PilotConfig{
							Cni: &v1.CNIUsageConfig{
								Enabled: new(false),
							},
						},
					},
				},
			},
			cfg:      defaultCfg,
			expected: false,
		},
		{
			name: "OpenshiftPlatformCNIExplicitlyDisabled",
			rev: &v1.IstioRevision{
				Spec: v1.IstioRevisionSpec{
					Values: &v1.Values{
						Global: &v1.GlobalConfig{
							Platform: new("openshift"),
						},
						Pilot: &v1.PilotConfig{
							Cni: &v1.CNIUsageConfig{
								Enabled: new(false),
							},
						},
					},
				},
			},
			cfg: config.ReconcilerConfig{
				Platform: config.PlatformOpenShift,
			},
			expected: false,
		},
		{
			name: "OpenshiftPlatformCNIExplicitlyEnabled",
			rev: &v1.IstioRevision{
				Spec: v1.IstioRevisionSpec{
					Values: &v1.Values{
						Global: &v1.GlobalConfig{
							Platform: new("openshift"),
						},
						Pilot: &v1.PilotConfig{
							Cni: &v1.CNIUsageConfig{
								Enabled: new(true),
							},
						},
					},
				},
			},
			cfg: config.ReconcilerConfig{
				Platform: config.PlatformOpenShift,
			},
			expected: true,
		},
		{
			name: "OpenshiftPlatformWithoutGlobalPlatformInValues",
			rev: &v1.IstioRevision{
				Spec: v1.IstioRevisionSpec{
					Values: &v1.Values{
						Pilot: &v1.PilotConfig{},
					},
				},
			},
			cfg: config.ReconcilerConfig{
				Platform: config.PlatformOpenShift,
			},
			expected: true,
		},
		{
			name: "OpenshiftPlatformCNINotConfigured",
			rev: &v1.IstioRevision{
				Spec: v1.IstioRevisionSpec{
					Values: &v1.Values{
						Global: &v1.GlobalConfig{
							Platform: new("openshift"),
						},
						Pilot: &v1.PilotConfig{}, // CNI not configured
					},
				},
			},
			cfg: config.ReconcilerConfig{
				Platform: config.PlatformOpenShift,
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DependsOnIstioCNI(tt.rev, tt.cfg)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDependsOnZTunnel(t *testing.T) {
	defaultCfg := config.ReconcilerConfig{
		Platform:       config.PlatformKubernetes,
		DefaultProfile: "default",
	}

	tests := []struct {
		name     string
		rev      *v1.IstioRevision
		cfg      config.ReconcilerConfig
		expected bool
	}{
		{
			name: "NilValues",
			rev: &v1.IstioRevision{
				Spec: v1.IstioRevisionSpec{
					Values: nil,
				},
			},
			cfg:      defaultCfg,
			expected: false,
		},
		{
			name: "NoPilot",
			rev: &v1.IstioRevision{
				Spec: v1.IstioRevisionSpec{
					Values: &v1.Values{
						Pilot: nil,
					},
				},
			},
			cfg:      defaultCfg,
			expected: false,
		},
		{
			name: "NoPilotEnv",
			rev: &v1.IstioRevision{
				Spec: v1.IstioRevisionSpec{
					Values: &v1.Values{
						Pilot: &v1.PilotConfig{
							Env: nil,
						},
					},
				},
			},
			cfg:      defaultCfg,
			expected: false,
		},
		{
			name: "PilotEnvAmbientDisabled",
			rev: &v1.IstioRevision{
				Spec: v1.IstioRevisionSpec{
					Values: &v1.Values{
						Pilot: &v1.PilotConfig{
							Env: map[string]string{
								"PILOT_ENABLE_AMBIENT": "false",
							},
						},
					},
				},
			},
			cfg:      defaultCfg,
			expected: false,
		},
		{
			name: "PilotEnvAmbientEnabled",
			rev: &v1.IstioRevision{
				Spec: v1.IstioRevisionSpec{
					Values: &v1.Values{
						Pilot: &v1.PilotConfig{
							Env: map[string]string{
								"PILOT_ENABLE_AMBIENT": "true",
							},
						},
					},
				},
			},
			cfg:      defaultCfg,
			expected: true,
		},
		{
			name: "AmbientProfileSet",
			rev: &v1.IstioRevision{
				Spec: v1.IstioRevisionSpec{
					Values: &v1.Values{
						Profile: new("ambient"),
					},
				},
			},
			cfg:      defaultCfg,
			expected: true,
		},
		{
			name: "NonAmbientProfile",
			rev: &v1.IstioRevision{
				Spec: v1.IstioRevisionSpec{
					Values: &v1.Values{
						Profile: new("default"),
					},
				},
			},
			cfg:      defaultCfg,
			expected: false,
		},
		{
			name: "AmbientProfileWithPilotEnvTrue",
			rev: &v1.IstioRevision{
				Spec: v1.IstioRevisionSpec{
					Values: &v1.Values{
						Profile: new("ambient"),
						Pilot: &v1.PilotConfig{
							Env: map[string]string{
								"PILOT_ENABLE_AMBIENT": "true",
							},
						},
					},
				},
			},
			cfg:      defaultCfg,
			expected: true,
		},
		{
			name: "EmptyProfile",
			rev: &v1.IstioRevision{
				Spec: v1.IstioRevisionSpec{
					Values: &v1.Values{
						Profile: new(""),
					},
				},
			},
			cfg:      defaultCfg,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DependsOnZTunnel(tt.rev, tt.cfg)
			assert.Equal(t, tt.expected, result)
		})
	}
}
