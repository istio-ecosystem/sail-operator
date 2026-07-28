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

package config

import (
	"context"
	"crypto/tls"
	"fmt"

	"github.com/go-logr/logr"
	configv1 "github.com/openshift/api/config/v1"
	openshifttls "github.com/openshift/controller-runtime-common/pkg/tls"
	openshiftcrypto "github.com/openshift/library-go/pkg/crypto"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TLSConfig represents the TLS configuration to be applied globally.
type TLSConfig struct {
	// CipherSuites is a list of TLS cipher suite IDs.
	CipherSuites []uint16

	// MinVersion is the minimum TLS version (e.g. tls.VersionTLS12).
	// Zero means no minimum version is configured.
	MinVersion uint16

	// OpenShift holds OpenShift-specific TLS configuration.
	// It is nil when not running on OpenShift.
	OpenShift *OpenShiftTLS
}

// OpenShiftTLS holds TLS settings fetched from the OpenShift APIServer.
type OpenShiftTLS struct {
	TLSProfileSpec     configv1.TLSProfileSpec
	TLSAdherencePolicy configv1.TLSAdherencePolicy

	// TLSConfigFunc applies the TLS profile to a *tls.Config. It can be
	// appended to the metrics server TLS options. Nil when the cluster TLS
	// profile should not be honored.
	TLSConfigFunc func(*tls.Config)
}

// These functions are ripped directly from:
// https://github.com/openshift/controller-runtime-common/blob/64ee174f5e2ebc630fbb554dd114d7a7a878693f/pkg/tls/tls.go#L134-L168
// until https://github.com/openshift/controller-runtime-common/issues/19 is resolved.

// cipherCode returns the TLS cipher code for an OpenSSL or IANA cipher name.
// Returns 0 if the cipher is not supported.
func cipherCode(cipher string) uint16 {
	// First try as IANA name directly.
	if code, err := openshiftcrypto.CipherSuite(cipher); err == nil {
		return code
	}

	// Try converting from OpenSSL name to IANA name.
	ianaCiphers := openshiftcrypto.OpenSSLToIANACipherSuites([]string{cipher})
	if len(ianaCiphers) == 1 {
		if code, err := openshiftcrypto.CipherSuite(ianaCiphers[0]); err == nil {
			return code
		}
	}

	// Return 0 if the cipher is not supported.
	return 0
}

// cipherCodes converts a list of cipher names (OpenSSL or IANA format) to their uint16 codes.
// Returns the converted codes and a list of any unsupported cipher names.
func cipherCodes(ciphers []string) (codes []uint16, unsupportedCiphers []string) {
	for _, cipher := range ciphers {
		code := cipherCode(cipher)
		if code == 0 {
			unsupportedCiphers = append(unsupportedCiphers, cipher)
			continue
		}

		codes = append(codes, code)
	}

	return codes, unsupportedCiphers
}

// FetchTLSConfigForOpenShift fetches TLS configuration from the OpenShift
// APIServer and returns a TLSConfig with the OpenShift field populated.
func FetchTLSConfigForOpenShift(ctx context.Context, log logr.Logger, cl client.Client) (*TLSConfig, error) {
	adherencePolicy, err := openshifttls.FetchAPIServerTLSAdherencePolicy(ctx, cl)
	if err != nil {
		return nil, fmt.Errorf("fetching TLS adherence policy from APIServer: %w", err)
	}

	profileSpec, err := openshifttls.FetchAPIServerTLSProfile(ctx, cl)
	if err != nil {
		return nil, fmt.Errorf("fetching TLS profile from APIServer: %w", err)
	}

	return NewTLSConfigForOpenShift(profileSpec, adherencePolicy, log), nil
}

// NewTLSConfigForOpenShift builds a TLSConfig from the given profile spec
// and adherence policy without fetching from the API server.
func NewTLSConfigForOpenShift(profileSpec configv1.TLSProfileSpec, adherencePolicy configv1.TLSAdherencePolicy, log logr.Logger) *TLSConfig {
	tlsConfig := &TLSConfig{
		OpenShift: &OpenShiftTLS{
			TLSProfileSpec:     profileSpec,
			TLSAdherencePolicy: adherencePolicy,
		},
	}

	if openshiftcrypto.ShouldHonorClusterTLSProfile(adherencePolicy) {
		tlsConfigFunc, _ := openshifttls.NewTLSConfigFromProfile(profileSpec)
		tlsConfig.OpenShift.TLSConfigFunc = tlsConfigFunc

		goTLSConfig := &tls.Config{MinVersion: tls.VersionTLS12}
		tlsConfigFunc(goTLSConfig)
		tlsConfig.MinVersion = goTLSConfig.MinVersion
		// Resolve cipher suites directly from the profile spec
		// instead of reading them back from the tls.Config set by tlsConfigFunc,
		// because the controller-runtime-common lib filters out the CipherSuites when
		// MinTLSVersion is 1.3. We still need the cipher IDs to configure non-Go components like Envoy.
		cipherSuites, unsupportedCiphers := cipherCodes(profileSpec.Ciphers)
		tlsConfig.CipherSuites = cipherSuites

		if len(unsupportedCiphers) > 0 {
			log.Info("Some ciphers from TLS profile are unsupported and will be ignored", "unsupportedCiphers", unsupportedCiphers)
		}
	}

	return tlsConfig
}
