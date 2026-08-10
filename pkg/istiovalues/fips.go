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

package istiovalues

import (
	"crypto/fips140"
	"fmt"

	"github.com/Masterminds/semver/v3"
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"

	"istio.io/istio/pkg/log"
)

const (
	fips140_2 = "fips-140-2"
	fips140_3 = "fips-140-3"
)

var istio1_30 = semver.MustParse("1.30.0")

// This is separated out solely to let tests override it.
var fipsEnabled = fips140.Enabled

// ApplyFipsValues sets pilot.env.COMPLIANCE_POLICY if FIPS mode is enabled in the system.
// For versions > 1.30, the policy is set to "fips-140-3".
// For versions <= 1.30, the policy is set to "fips-140-2".
func ApplyFipsValues(values *v1.Values, version string) error {
	if !fipsEnabled() || values == nil {
		return nil
	}

	v, err := semver.NewVersion(version)
	if err != nil {
		return fmt.Errorf("failed to parse version %q: %w", version, err)
	}

	// istio 1.31 is built with go1.26.
	// FIPS mode for go1.26, which is what setting fips-140-3 enables,
	// is certified on platforms with an openssl backend i.e. OpenShift.
	policy := fips140_2
	if v.GreaterThan(istio1_30) {
		policy = fips140_3
	}

	if values.Pilot == nil {
		values.Pilot = &v1.PilotConfig{}
	}
	if values.Pilot.Env == nil {
		values.Pilot.Env = make(map[string]string)
	}
	if _, found := values.Pilot.Env["COMPLIANCE_POLICY"]; !found {
		values.Pilot.Env["COMPLIANCE_POLICY"] = policy
	}
	return nil
}

// ApplyZTunnelFipsValues sets ztunnel.env.TLS12_ENABLED if FIPS mode is enabled in the system.
// For versions > 1.30, TLS12_ENABLED is removed because ztunnel
// defaults to using only FIPS 140-3 approved ciphers.
func ApplyZTunnelFipsValues(values *v1.ZTunnelValues, version string) {
	if !fipsEnabled() || values == nil {
		return
	}

	v, err := semver.NewVersion(version)
	if err != nil {
		log.Warnf("failed to parse ztunnel version %q: %v", version, err)
	}
	if v != nil && v.GreaterThan(istio1_30) {
		if values.ZTunnel != nil && values.ZTunnel.Env != nil {
			delete(values.ZTunnel.Env, "TLS12_ENABLED")
		}
		return
	}

	if values.ZTunnel == nil {
		values.ZTunnel = &v1.ZTunnelConfig{}
	}
	if values.ZTunnel.Env == nil {
		values.ZTunnel.Env = make(map[string]string)
	}
	// TODO: Remove this after 1.29 is no longer supported.
	if _, found := values.ZTunnel.Env["TLS12_ENABLED"]; !found {
		values.ZTunnel.Env["TLS12_ENABLED"] = "true"
	}
}
