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

	"istio.io/istio/pkg/ptr"
)

// mockComputeValues returns the input values without any computation
// this simulates what ComputeValues would do but without requiring actual files
func mockComputeValues(values *v1.Values, _, _ string, platform config.Platform, defaultProfile, userProfile, _, _ string) (*v1.Values, error) {
	if values == nil {
		values = &v1.Values{}
	}

	// If platform is OpenShift and there is no explicit CNI configured, enable CNI
	if platform == config.PlatformOpenShift && (values.Pilot == nil || values.Pilot.Cni == nil || values.Pilot.Cni.Enabled == nil) {
		if values.Pilot == nil {
			values.Pilot = &v1.PilotConfig{}
		}
		if values.Pilot.Cni == nil {
			values.Pilot.Cni = &v1.CNIUsageConfig{}
		}
		values.Pilot.Cni.Enabled = ptr.Of(true)
	}

	profile := userProfile
	if profile == "" {
		profile = defaultProfile
	}

	// If profile is ambient, set the profile in values and enable PILOT_ENABLE_AMBIENT
	if profile == "ambient" {
		values.Profile = ptr.Of("ambient")
		if values.Pilot == nil {
			values.Pilot = &v1.PilotConfig{}
		}
		if values.Pilot.Env == nil {
			values.Pilot.Env = make(map[string]string)
		}
		values.Pilot.Env["PILOT_ENABLE_AMBIENT"] = "true"
	}

	return values, nil
}

func TestDependsOnIstioCNI(t *testing.T) {
	// Replace ComputeValues with mock for testing
	defaultComputeValues = mockComputeValues
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
								Enabled: ptr.Of(true),
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
								Enabled: ptr.Of(false),
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
							Platform: ptr.Of("openshift"),
						},
						Pilot: &v1.PilotConfig{
							Cni: &v1.CNIUsageConfig{
								Enabled: ptr.Of(false),
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
							Platform: ptr.Of("openshift"),
						},
						Pilot: &v1.PilotConfig{
							Cni: &v1.CNIUsageConfig{
								Enabled: ptr.Of(true),
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
			name: "OpenshiftPlatformCNINotConfigured",
			rev: &v1.IstioRevision{
				Spec: v1.IstioRevisionSpec{
					Values: &v1.Values{
						Global: &v1.GlobalConfig{
							Platform: ptr.Of("openshift"),
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
	defaultComputeValues = mockComputeValues
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
						Profile: ptr.Of("ambient"),
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
						Profile: ptr.Of("default"),
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
						Profile: ptr.Of("ambient"),
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
						Profile: ptr.Of(""),
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
