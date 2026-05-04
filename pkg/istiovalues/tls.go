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
