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
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	configv1 "github.com/openshift/api/config/v1"
	openshiftcrypto "github.com/openshift/library-go/pkg/crypto"

	"istio.io/istio/pkg/log"
)

var istio1_29 = semver.MustParse("1.29.0")

// ApplyTLSConfig applies TLS configuration to the Istio values.
// If TLS settings are already set, they are not overridden.
func ApplyTLSConfig(tlsConfig *config.TLSConfig, istioVersion string, values *v1.Values) {
	if values == nil {
		return
	}

	// TODO: non-FIPS-approved entries from the OpenShift TLS profile are not filtered out here,
	// neither the ECDH groups (X25519 and X25519MLKEM768 — see the FIPS note in openshift/api
	// config/v1 types_tlssecurityprofile.go) nor the CHACHA20 cipher suites.
	//
	// Today this is masked by istiod, which strips both: EnforceCompliance (istio
	// pkg/model/fips.go) runs last, after applyDownstreamTLSDefaults (pilot listener.go) and
	// applyTLSDefaults (pilot cluster_tls.go), so it overwrites whatever is set below.
	//   - fips-140-2 (ApplyFipsValues for Istio <= 1.30) sets TlsParams.EcdhCurves = nil and
	//     filters CipherSuites against its FIPS list.
	//   - fips-140-3-redhat (set by the downstream OSSM operator) sets both TlsParams.EcdhCurves
	//     and TlsParams.CipherSuites to nil. Verified on a FIPS OCP 5.0 cluster: the profile's
	//     groups appear in the mesh ConfigMap but the proxy config dump has zero ecdh_curves.
	//
	// It is NOT masked for fips-140-3, which ApplyFipsValues sets for Istio > 1.30 upstream:
	// that branch only sets TlsParams.CompliancePolicies = FIPS_202205 and leaves both fields
	// intact. The same applies when COMPLIANCE_POLICY is empty or unrecognized, since those fall
	// through EnforceCompliance without stripping. On an OpenSSL-backed Envoy in FIPS mode the
	// values then reach the TLS context and are rejected atomically, taking the listener or
	// cluster with them. Both calls are in Envoy source/common/tls/context_impl.cc:
	//   - SSL_CTX_set1_curves_list fails if any single group is unavailable, so one bad entry
	//     discards the whole list ("Failed to initialize ECDH curves").
	//   - SSL_CTX_set_strict_cipher_list likewise ("Failed to initialize cipher suites"). In the
	//     OpenSSL-backed build this is not BoringSSL's function but the bssl-compat
	//     reimplementation in compat/openssl/source/SSL_CTX_set_strict_cipher_list.cc, which
	//     calls OpenSSL's tolerant SSL_CTX_set_cipher_list and then requires every input token
	//     to appear in SSL_CTX_get_ciphers(). Under FIPS the CHACHA20 entries are dropped from
	//     the resolved stack, so that second check fails.
	//
	// When addressing this, filter both on istiovalues.FipsEnabled. Note also that TLS 1.3 suite
	// names cannot be set through tls_params.cipher_suites at all — they are carried by
	// OPENSSL_TLS1_3_CIPHERSUITES below — yet that same token check still requires them to
	// resolve, so they are a hazard despite being inert. This is specific to the OpenSSL build;
	// BoringSSL's own strict function instead rejects unknown rules outright and excludes TLS
	// 1.3 ciphers from cipher-list queries entirely, which has not been tested here.

	// ECDH curves are applied independently of the cipher-suite driven configuration below,
	// so that on OpenShift the post-quantum hybrid group is offered even when the cluster
	// TLS profile is not honored (tlsAdherence is NoOpinion, leaving CipherSuites empty) or
	// when the profile specifies no groups. The profile's own groups are only consulted when
	// tlsAdherence requires it.
	applyECDHCurves(tlsConfig, values)

	if tlsConfig == nil || len(tlsConfig.CipherSuites) == 0 {
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

	if tlsConfig.OpenShift != nil {
		// Envoy does not support setting TLS ciphers for TLS 1.3 BUT the openssl
		// backend does allow you to customize these. On openshift the envoy image has a custom openssl
		// config file baked in that will populate the tls1.3 cipher suites through an env var.
		// Also important to note is that all non-FIPS approved ciphers will be silently dropped by
		// openssl, even if they are specified in the openssl config. So we do not need to filter out
		// FIPS ciphers here. We can pass them to the conf and openssl will drop them automatically on
		// a FIPS enabled cluster.
		if values.MeshConfig.DefaultConfig == nil {
			values.MeshConfig.DefaultConfig = &v1.MeshConfigProxyConfig{}
		}
		if values.MeshConfig.DefaultConfig.ProxyMetadata == nil {
			values.MeshConfig.DefaultConfig.ProxyMetadata = map[string]string{}
		}
		if _, ok := values.MeshConfig.DefaultConfig.ProxyMetadata["OPENSSL_TLS1_3_CIPHERSUITES"]; !ok {
			values.MeshConfig.DefaultConfig.ProxyMetadata["OPENSSL_TLS1_3_CIPHERSUITES"] = strings.Join(cipherNames, ":")
		}
	}

	if values.Pilot == nil {
		values.Pilot = &v1.PilotConfig{}
	}
	addExtraContainerArg(values.Pilot, "--tls-cipher-suites", strings.Join(cipherNames, ","))

	if minVersionName := tlsVersionName(tlsConfig.MinVersion); minVersionName != "" {
		v, err := semver.NewVersion(istioVersion)
		if err != nil || v == nil {
			log.Warnf("failed to parse Istio version %q: %v", istioVersion, err)
			return
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

// defaultECDHCurves is used when the cluster TLS profile supplies no groups, or when the
// profile is not honored at all. X25519MLKEM768 leads so the post-quantum hybrid group is
// preferred; the remaining entries mirror what the predefined OpenShift profiles yield, so
// that interop with peers lacking ML-KEM is preserved.
//
// NOTE: X25519MLKEM768 and X25519 do not work on a FIPS OpenShift cluster. Verified on FIPS
// OCP 5.0 with the OSSM 3.5 proxy image (Envoy 1.39.2-dev/OpenSSL on OpenSSL 3.5.8 with the
// RHEL 9 FIPS provider 3.0.7): the group parser rejects both names, while SecP256r1MLKEM768,
// SecP384r1MLKEM1024, P-256 and P-384 resolve — the 3.0.7 FIPS module predates ML-KEM and
// X25519 is not FIPS-approved. Because SSL_CTX_set1_curves_list is atomic (see above), one
// bad entry discards the whole list: a gateway with SIMPLE TLS then lands in error_state with
// "Failed to initialize ECDH curves X25519MLKEM768:X25519:P-256:P-384" and never serves.
//
// Today this is masked on OpenShift by fips-140-3-redhat nilling TlsParams.EcdhCurves, so the
// list never reaches Envoy. It is not masked under fips-140-3, nor when COMPLIANCE_POLICY is
// empty or unrecognized. Only meshConfig.tlsDefaults is affected, i.e. SIMPLE/MUTUAL gateway
// servers; in-mesh mTLS contexts never consult it. If the post-quantum hybrid group is wanted
// on FIPS OpenShift, SecP256r1MLKEM768 is the name that resolves there.
var defaultECDHCurves = []string{"X25519MLKEM768", "X25519", "P-256", "P-384"}

// applyECDHCurves sets meshConfig.tlsDefaults.ecdhCurves from the OpenShift TLS profile when
// the cluster profile is honored and provides groups, and otherwise falls back to
// defaultECDHCurves. A user-supplied value is never overridden.
//
// Note that only meshConfig.tlsDefaults is set: Istio rejects the whole mesh config when
// meshConfig.meshMTLS.ecdhCurves is present ("mesh TLS does not support ECDH curves
// configuration"), so mesh mTLS curves cannot be configured this way.
func applyECDHCurves(tlsConfig *config.TLSConfig, values *v1.Values) {
	// Only applies on OpenShift. Off OpenShift tlsConfig is nil and the proxy image is
	// not known to support the hybrid group, so the Envoy defaults are left in place.
	if tlsConfig == nil || tlsConfig.OpenShift == nil {
		return
	}

	// The cluster TLS profile is consulted only when tlsAdherence requires it. Under
	// NoOpinion and LegacyAdheringComponentsOnly the profile's groups are ignored, just as
	// its ciphers are, and defaultECDHCurves applies instead.
	var curves []string
	if openshiftcrypto.ShouldHonorClusterTLSProfile(tlsConfig.OpenShift.TLSAdherencePolicy) {
		// Normalize the names, given OpenShift allows a different naming from NIST.
		curves = copyECDHCurvesToConfig(tlsConfig.OpenShift.TLSProfileSpec.Groups)
	}
	if len(curves) == 0 {
		curves = slices.Clone(defaultECDHCurves)
	}

	if values.MeshConfig == nil {
		values.MeshConfig = &v1.MeshConfig{}
	}
	if values.MeshConfig.TlsDefaults == nil {
		values.MeshConfig.TlsDefaults = &v1.MeshConfigTLSConfig{}
	}
	if len(values.MeshConfig.TlsDefaults.EcdhCurves) == 0 {
		values.MeshConfig.TlsDefaults.EcdhCurves = curves
	}
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
