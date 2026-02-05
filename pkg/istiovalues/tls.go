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
	"fmt"
	"strings"

	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
)

// ApplyTLSConfig applies TLS configuration to the Helm values.
// If TLS settings are already set, they are not overridden.
func ApplyTLSConfig(tlsConfig *config.TLSConfig, values helm.Values) (helm.Values, error) {
	if tlsConfig == nil {
		return values, nil
	}

	if len(tlsConfig.CipherSuites) > 0 {
		cipherNames := make([]string, len(tlsConfig.CipherSuites))
		cipherSlice := make([]any, len(tlsConfig.CipherSuites))
		for i, id := range tlsConfig.CipherSuites {
			name := tls.CipherSuiteName(id)
			cipherNames[i] = name
			cipherSlice[i] = name
		}
		if err := values.SetIfAbsent("meshConfig.tlsDefaults.cipherSuites", cipherSlice); err != nil {
			return nil, fmt.Errorf("failed to set meshConfig.tlsDefaults.cipherSuites: %w", err)
		}
		if err := values.SetIfAbsent("meshConfig.meshMTLS.cipherSuites", cipherSlice); err != nil {
			return nil, fmt.Errorf("failed to set meshConfig.meshMTLS.cipherSuites: %w", err)
		}

		if err := addExtraContainerArg(values, "--tls-cipher-suites", strings.Join(cipherNames, ",")); err != nil {
			return nil, fmt.Errorf("failed to set pilot.extraContainerArgs: %w", err)
		}
	}

	return values, nil
}

// addExtraContainerArg adds an argument to pilot.extraContainerArgs if not already present.
func addExtraContainerArg(values helm.Values, argName, argValue string) error {
	existingArgs, _, err := values.GetSlice("pilot.extraContainerArgs")
	if err != nil {
		return fmt.Errorf("pilot.extraContainerArgs is not a slice: %w", err)
	}

	argWithValue := argName + "=" + argValue
	for _, arg := range existingArgs {
		if argStr, ok := arg.(string); ok {
			// Skip if already set (don't override user-provided values)
			if strings.HasPrefix(argStr, argName) {
				return nil
			}
		}
	}

	// Add the new argument
	newArgs := append(existingArgs, argWithValue)
	return values.Set("pilot.extraContainerArgs", newArgs)
}
