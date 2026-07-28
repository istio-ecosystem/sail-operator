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
	"crypto/tls"
	"strings"

	"github.com/Masterminds/semver/v3"
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	configv1 "github.com/openshift/api/config/v1"

	"istio.io/istio/pkg/log"
)

var istio1_29 = semver.MustParse("1.29.0")

// ApplyTLSConfig applies TLS configuration to the Istio values.
// If TLS settings are already set, they are not overridden.
func ApplyTLSConfig(tlsConfig *config.TLSConfig, istioVersion string, values *v1.Values) {
	if tlsConfig == nil || values == nil || len(tlsConfig.CipherSuites) == 0 {
		return
	}

	cipherNames := make([]string, len(tlsConfig.CipherSuites))
	for i, id := range tlsConfig.CipherSuites {
		cipherNames[i] = tls.CipherSuiteName(id)
	}

	if values.MeshConfig == nil {
		values.MeshConfig = &v1.MeshConfig{}
	}

	if values.MeshConfig.TlsDefaults == nil {
		values.MeshConfig.TlsDefaults = &v1.MeshConfigTLSConfig{}
	}
	if len(values.MeshConfig.TlsDefaults.CipherSuites) == 0 {
		values.MeshConfig.TlsDefaults.CipherSuites = cipherNames
	}

	if values.MeshConfig.TlsDefaults.MinProtocolVersion == "" {
		values.MeshConfig.TlsDefaults.MinProtocolVersion = tlsProtocolVersion(tlsConfig.MinVersion)
	}

	if values.MeshConfig.MeshMTLS == nil {
		values.MeshConfig.MeshMTLS = &v1.MeshConfigTLSConfig{}
	}
	if len(values.MeshConfig.MeshMTLS.CipherSuites) == 0 {
		values.MeshConfig.MeshMTLS.CipherSuites = cipherNames
	}

	if values.MeshConfig.MeshMTLS.MinProtocolVersion == "" {
		values.MeshConfig.MeshMTLS.MinProtocolVersion = tlsProtocolVersion(tlsConfig.MinVersion)
	}

	// Envoy does not support setting TLS ciphers when minProtocolVersion is 1.3
	if values.MeshConfig.TlsDefaults.MinProtocolVersion == v1.MeshConfigTLSConfigTLSProtocolTlsv13 {
		values.MeshConfig.TlsDefaults.CipherSuites = nil
	}

	if values.MeshConfig.MeshMTLS.MinProtocolVersion == v1.MeshConfigTLSConfigTLSProtocolTlsv13 {
		values.MeshConfig.MeshMTLS.CipherSuites = nil
	}

	// Configure the Ecdhcurves if they are available on Openshift configuration. Normalize before
	// copying the names, given Openshift allows a different naming from NIST
	if tlsConfig.OpenShift != nil && len(tlsConfig.OpenShift.TLSProfileSpec.Groups) > 0 {
		ecdhCurves := copyECDHCurvesToConfig(tlsConfig.OpenShift.TLSProfileSpec.Groups)
		if len(values.MeshConfig.TlsDefaults.EcdhCurves) == 0 {
			values.MeshConfig.TlsDefaults.EcdhCurves = ecdhCurves
		}
		// TODO: MeshMTLS does not support setting ecdhCurves and this will break Gateway provisioning.
		// we should take care of ecdhCurves for mesh as a second step
	}

	if values.Pilot == nil {
		values.Pilot = &v1.PilotConfig{}
	}
	addExtraContainerArg(values.Pilot, "--tls-cipher-suites", strings.Join(cipherNames, ","))

	if minVersionName := tlsVersionName(tlsConfig.MinVersion); minVersionName != "" {
		v, err := semver.NewVersion(istioVersion)
		if err != nil {
			log.Warnf("failed to parse Istio version %q: %v", istioVersion, err)
		}

		// This flag is only supported on Istio 1.29+. TODO: Remove this check when we drop support for Istio 1.28
		if v.GreaterThanEqual(istio1_29) {
			addExtraContainerArg(values.Pilot, "--tls-min-version", minVersionName)
		} else {
			log.Infof("Istio version %q is less than 1.29 and --tls-min-version flag for istiod is only supported on Istio 1.29+. Skipping sync of flag.", istioVersion)
		}
	}
}

func tlsProtocolVersion(v uint16) v1.MeshConfigTLSConfigTLSProtocol {
	switch v {
	case tls.VersionTLS12:
		return v1.MeshConfigTLSConfigTLSProtocolTlsv12
	case tls.VersionTLS13:
		return v1.MeshConfigTLSConfigTLSProtocolTlsv13
	default:
		return ""
	}
}

func tlsVersionName(v uint16) string {
	switch v {
	case tls.VersionTLS12:
		return "1.2"
	case tls.VersionTLS13:
		return "1.3"
	default:
		return ""
	}
}

// addExtraContainerArg adds an argument to ExtraContainerArgs if not already present.
func addExtraContainerArg(pilot *v1.PilotConfig, argName, argValue string) {
	argNameWithEquals := argName + "="
	for _, arg := range pilot.ExtraContainerArgs {
		if arg == argName || strings.HasPrefix(arg, argNameWithEquals) {
			return
		}
	}
	pilot.ExtraContainerArgs = append(pilot.ExtraContainerArgs, argNameWithEquals+argValue)
}

// copyECDHCurvesToConfig creates a new slice containing the normalized ECDHCurves to be
// used on meshConfig fields
func copyECDHCurvesToConfig(groups []configv1.TLSGroup) []string {
	values := make([]string, len(groups))
	for i := range groups {
		values[i] = normalizeECDHCurve(string(groups[i]))
	}
	return values
}

// normalizeECDHCurve maps common curve aliases to Envoy-supported names
func normalizeECDHCurve(curve string) string {
	switch strings.ToLower(strings.TrimSpace(curve)) {
	case "secp256r1", "prime256v1", "p-256", "p256":
		return "P-256"
	case "secp384r1", "p-384", "p384":
		return "P-384"
	case "x25519":
		return "X25519"
	default:
		return curve // Return as-is if unmapped
	}
}
