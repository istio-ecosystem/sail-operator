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
	"crypto/tls"
	"io/fs"
	"strings"

	"github.com/magiconair/properties"
	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/pkg/crypto"
)

var Config = OperatorConfig{}

type OperatorConfig struct {
	ImageDigests map[string]IstioImageConfig `properties:"images"`
}

type IstioImageConfig struct {
	IstiodImage  string `properties:"istiod"`
	ProxyImage   string `properties:"proxy"`
	CNIImage     string `properties:"cni"`
	ZTunnelImage string `properties:"ztunnel"`
}

type ReconcilerConfig struct {
	ResourceFS              fs.FS
	Platform                Platform
	DefaultProfile          string
	OperatorNamespace       string
	MaxConcurrentReconciles int
}

// TLSConfig represents the TLS configuration to be applied globally.
type TLSConfig struct {
	// CipherSuites is a list of TLS cipher suites.
	CipherSuites []tls.CipherSuite
}

// TLSConfigFromAPIServer extracts TLS configuration from an OpenShift APIServer resource.
func TLSConfigFromAPIServer(apiServer *configv1.APIServer) TLSConfig {
	profile := apiServer.Spec.TLSSecurityProfile
	profileType := configv1.TLSProfileIntermediateType
	if profile != nil {
		profileType = profile.Type
	}

	var profileSpec *configv1.TLSProfileSpec
	if profileType == configv1.TLSProfileCustomType {
		if profile.Custom != nil {
			profileSpec = &profile.Custom.TLSProfileSpec
		}
	} else {
		profileSpec = configv1.TLSProfiles[profileType]
	}

	if profileSpec == nil {
		profileSpec = configv1.TLSProfiles[configv1.TLSProfileIntermediateType]
	}

	return TLSConfig{
		CipherSuites: cipherSuitesFromNames(crypto.OpenSSLToIANACipherSuites(profileSpec.Ciphers)),
	}
}

// cipherSuitesFromNames converts IANA cipher suite names to tls.CipherSuite structs.
func cipherSuitesFromNames(names []string) []tls.CipherSuite {
	cipherMap := make(map[uint16]*tls.CipherSuite)
	for _, cs := range tls.CipherSuites() {
		cipherMap[cs.ID] = cs
	}
	for _, cs := range tls.InsecureCipherSuites() {
		cipherMap[cs.ID] = cs
	}

	var suites []tls.CipherSuite
	for _, name := range names {
		cs, err := crypto.CipherSuite(name)
		if err != nil {
			// Ignore unknown cipher suites.
			continue
		}
		if cs, ok := cipherMap[cs]; ok {
			suites = append(suites, *cs)
		}
	}
	return suites
}

func Read(configFile string) error {
	p, err := properties.LoadFile(configFile, properties.UTF8)
	if err != nil {
		return err
	}
	// remove quotes
	for _, key := range p.Keys() {
		val, _ := p.Get(key)
		_, _, _ = p.Set(key, strings.Trim(val, `"`))
	}
	err = p.Decode(&Config)
	if err != nil {
		return err
	}
	// replace "_" in versions with "." (e.g. v1_20_0 => v1.20.0)
	newImageDigests := make(map[string]IstioImageConfig, len(Config.ImageDigests))
	for k, v := range Config.ImageDigests {
		newImageDigests[strings.Replace(k, "_", ".", -1)] = v
	}
	Config.ImageDigests = newImageDigests
	return nil
}
