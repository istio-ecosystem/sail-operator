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

// NewTLSConfigForOpenShift fetches TLS configuration from the OpenShift
// APIServer and returns a TLSConfig with the OpenShift field populated.
func NewTLSConfigForOpenShift(ctx context.Context, log logr.Logger, cl client.Client) (*TLSConfig, error) {
	adherencePolicy, err := openshifttls.FetchAPIServerTLSAdherencePolicy(ctx, cl)
	if err != nil {
		return nil, fmt.Errorf("fetching TLS adherence policy from APIServer: %w", err)
	}

	profileSpec, err := openshifttls.FetchAPIServerTLSProfile(ctx, cl)
	if err != nil {
		return nil, fmt.Errorf("fetching TLS profile from APIServer: %w", err)
	}

	tlsConfig := &TLSConfig{
		OpenShift: &OpenShiftTLS{
			TLSProfileSpec:     profileSpec,
			TLSAdherencePolicy: adherencePolicy,
		},
	}

	if openshiftcrypto.ShouldHonorClusterTLSProfile(adherencePolicy) {
		tlsConfigFunc, unsupportedCiphers := openshifttls.NewTLSConfigFromProfile(profileSpec)
		tlsConfig.OpenShift.TLSConfigFunc = tlsConfigFunc

		if len(unsupportedCiphers) > 0 {
			log.Info("Some ciphers from TLS profile are unsupported and will be ignored", "unsupportedCiphers", unsupportedCiphers)
		}

		goTLSConfig := &tls.Config{MinVersion: tls.VersionTLS12}
		tlsConfigFunc(goTLSConfig)
		tlsConfig.CipherSuites = goTLSConfig.CipherSuites
		tlsConfig.MinVersion = goTLSConfig.MinVersion
	}

	return tlsConfig, nil
}
