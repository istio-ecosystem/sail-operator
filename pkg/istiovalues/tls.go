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

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
)

// ApplyTLSConfig applies TLS configuration to the Istio values.
// If TLS settings are already set, they are not overridden.
func ApplyTLSConfig(tlsConfig *config.TLSConfig, values *v1.Values) {
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

	if values.MeshConfig.MeshMTLS == nil {
		values.MeshConfig.MeshMTLS = &v1.MeshConfigTLSConfig{}
	}
	if len(values.MeshConfig.MeshMTLS.CipherSuites) == 0 {
		values.MeshConfig.MeshMTLS.CipherSuites = cipherNames
	}

	if values.Pilot == nil {
		values.Pilot = &v1.PilotConfig{}
	}
	addExtraContainerArg(values.Pilot, "--tls-cipher-suites", strings.Join(cipherNames, ","))
}

// addExtraContainerArg adds an argument to ExtraContainerArgs if not already present.
func addExtraContainerArg(pilot *v1.PilotConfig, argName, argValue string) {
	for _, arg := range pilot.ExtraContainerArgs {
		if strings.HasPrefix(arg, argName) {
			return
		}
	}
	pilot.ExtraContainerArgs = append(pilot.ExtraContainerArgs, argName+"="+argValue)
}
