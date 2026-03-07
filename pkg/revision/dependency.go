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
	"io/fs"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
)

type computeValuesFunc func(*v1.Values, string, string, config.Platform, string, string, fs.FS, string) (*v1.Values, error)

var defaultComputeValues computeValuesFunc = ComputeValues

// DependsOnIstioCNI returns true if CNI is enabled in the revision
func DependsOnIstioCNI(rev *v1.IstioRevision, cfg config.ReconcilerConfig) bool {
	values, err := defaultComputeValues(rev.Spec.Values, rev.Spec.Namespace, rev.Spec.Version,
		cfg.Platform, cfg.DefaultProfile, "", cfg.ResourceFS, rev.Name)
	if err != nil || values == nil {
		return false
	}
	global := values.Global
	pilot := values.Pilot

	isOCPPlatform := global != nil && global.Platform != nil && *global.Platform == "openshift"
	isCNIEnabled := pilot != nil && pilot.Cni != nil && pilot.Cni.Enabled != nil && *pilot.Cni.Enabled
	isCNIExplicitlyDisabled := pilot != nil && pilot.Cni != nil && pilot.Cni.Enabled != nil && !*pilot.Cni.Enabled

	// For OpenShift: CNI is enabled by default unless explicitly disabled
	// For non-OpenShift: CNI must be explicitly enabled
	if isOCPPlatform {
		return !isCNIExplicitlyDisabled
	}
	return isCNIEnabled
}

// DependsOnZTunnel returns true if the revision is configured for ambient mode and requires ZTunnel
func DependsOnZTunnel(rev *v1.IstioRevision, cfg config.ReconcilerConfig) bool {
	values, err := defaultComputeValues(rev.Spec.Values, rev.Spec.Namespace, rev.Spec.Version,
		cfg.Platform, cfg.DefaultProfile, "", cfg.ResourceFS, rev.Name)
	if err != nil || values == nil {
		return false
	}

	// Check if ambient mode is enabled via pilot.env.PILOT_ENABLE_AMBIENT
	if values.Pilot != nil && values.Pilot.Env != nil {
		if ambientEnabled, exists := values.Pilot.Env["PILOT_ENABLE_AMBIENT"]; exists && ambientEnabled == "true" {
			return true
		}
	}

	// Also check if ambient profile is being used
	if values.Profile != nil && *values.Profile == "ambient" {
		return true
	}

	return false
}
