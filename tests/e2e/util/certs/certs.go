//go:build e2e

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

package certs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/kubectl"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/shell"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// CreateIntermediateCA creates the intermediate CA
func CreateIntermediateCA(basePath string) error {
	certsDir := filepath.Join(basePath, "certs")

	// Create the certs directory
	err := os.MkdirAll(certsDir, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create certs directory: %w", err)
	}

	// Create the root CA configuration file
	err = createRootCAConf(certsDir)
	if err != nil {
		return fmt.Errorf("failed to create root-ca.conf: %w", err)
	}

	// Step 1: Generate root-key.pem
	rootKey := filepath.Join(certsDir, "root-key.pem")
	_, err = shell.ExecuteCommand(fmt.Sprintf("openssl genrsa -out %s 4096", rootKey))
	if err != nil {
		return fmt.Errorf("failed to generate root-key.pem: %w", err)
	}

	// Step 2: Generate root-cert.csr using root-key.pem and root-ca.conf
	rootCSR := filepath.Join(certsDir, "root-cert.csr")
	rootConf := filepath.Join(certsDir, "root-ca.conf") // You'll need to ensure root-ca.conf exists
	_, err = shell.ExecuteCommand(fmt.Sprintf("openssl req -sha256 -new -key %s -config %s -out %s", rootKey, rootConf, rootCSR))
	if err != nil {
		return fmt.Errorf("failed to generate root-cert.csr: %w", err)
	}

	// Step 3: Generate root-cert.pem
	rootCert := filepath.Join(certsDir, "root-cert.pem")
	_, err = shell.ExecuteCommand(
		fmt.Sprintf("openssl x509 -req -sha256 -days 3650 -signkey %s -extensions req_ext -extfile %s -in %s -out %s",
			rootKey, rootConf, rootCSR, rootCert))
	if err != nil {
		return fmt.Errorf("failed to generate root-cert.pem: %w", err)
	}

	// Step 4: Generate east-cacerts (self-signed intermediate certificates)
	// Create directories for east and west if needed
	eastDir := filepath.Join(certsDir, "east")
	westDir := filepath.Join(certsDir, "west")

	// Create the east and west directories
	err = os.MkdirAll(eastDir, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create east directory: %w", err)
	}
	err = os.MkdirAll(westDir, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create west directory: %w", err)
	}

	// Create the intermediate CA configuration file
	err = createIntermediateCAConf(eastDir)
	if err != nil {
		return fmt.Errorf("failed to create ca.conf on east dir: %w", err)
	}

	err = createIntermediateCAConf(westDir)
	if err != nil {
		return fmt.Errorf("failed to create ca.conf on west dir: %w", err)
	}

	err = generateIntermediateCACertificates(eastDir, rootCert, rootKey)
	if err != nil {
		return fmt.Errorf("failed to generate east intermediate CA certificates: %w", err)
	}

	err = generateIntermediateCACertificates(westDir, rootCert, rootKey)
	if err != nil {
		return fmt.Errorf("failed to generate west intermediate CA certificates: %w", err)
	}

	return nil
}

func generateIntermediateCACertificates(dir string, rootCert string, rootKey string) error {
	caKey := filepath.Join(dir, "ca-key.pem")
	_, err := shell.ExecuteCommand(fmt.Sprintf("openssl genrsa -out %s 4096", caKey))
	if err != nil {
		return fmt.Errorf("failed to generate east-ca-key.pem: %w", err)
	}

	caCSR := filepath.Join(dir, "ca-cert.csr")
	caConf := filepath.Join(dir, "ca.conf")
	_, err = shell.ExecuteCommand(fmt.Sprintf("openssl req -sha256 -new -config %s -key %s -out %s", caConf, caKey, caCSR))
	if err != nil {
		return fmt.Errorf("failed to generate east-ca-cert.csr: %w", err)
	}

	caCert := filepath.Join(dir, "ca-cert.pem")
	_, err = shell.ExecuteCommand(
		fmt.Sprintf("openssl x509 -req -sha256 -days 3650 -CA %s -CAkey %s -CAcreateserial -extensions req_ext -extfile %s -in %s -out %s",
			rootCert, rootKey, caConf, caCSR, caCert))
	if err != nil {
		return fmt.Errorf("failed to generate east-ca-cert.pem: %w", err)
	}

	certChain := filepath.Join(dir, "cert-chain.pem")
	_, err = shell.ExecuteCommand(fmt.Sprintf("cat %s %s > %s", caCert, rootCert, certChain))
	if err != nil {
		return fmt.Errorf("failed to generate east-cert-chain.pem: %w", err)
	}

	return nil
}

// createRootCAConf creates the root CA configuration file
func createRootCAConf(certsDir string) error {
	confPath := filepath.Join(certsDir, "root-ca.conf")
	confContent := `
[ req ]
encrypt_key = no
prompt = no
utf8 = yes
default_md = sha256
default_bits = 4096
req_extensions = req_ext
x509_extensions = req_ext
distinguished_name = req_dn

[ req_ext ]
subjectKeyIdentifier = hash
basicConstraints = critical, CA:true
keyUsage = critical, digitalSignature, nonRepudiation, keyEncipherment, keyCertSign

[ req_dn ]
O = Istio
CN = Root CA
`

	// Write the configuration file to the directory
	return writeFile(confPath, confContent)
}

// createIntermediateCAConf creates the intermediate CA configuration file
func createIntermediateCAConf(certsDir string) error {
	confPath := filepath.Join(certsDir, "ca.conf")
	confContent := fmt.Sprintf(`
[ req ]
encrypt_key = no
prompt = no
utf8 = yes
default_md = sha256
default_bits = 4096
req_extensions = req_ext
x509_extensions = req_ext
distinguished_name = req_dn

[ req_ext ]
subjectKeyIdentifier = hash
basicConstraints = critical, CA:true, pathlen:0
keyUsage = critical, digitalSignature, nonRepudiation, keyEncipherment, keyCertSign
subjectAltName=@san

[ san ]
DNS.1 = istiod.istio-system.svc

[ req_dn ]
O = Istio
CN = Intermediate CA
L = %s
`, confPath)

	// Write the configuration file to the directory
	return writeFile(confPath, confContent)
}

// writeFile writes the content to the file
func writeFile(confPath string, confContent string) error {
	file, err := os.Create(confPath)
	if err != nil {
		return fmt.Errorf("failed to create %s: %v", confPath, err)
	}
	defer file.Close()

	_, err = file.WriteString(confContent)
	if err != nil {
		return fmt.Errorf("failed to write to %s: %v", confPath, err)
	}

	return nil
}

// PushIntermediateCA pushes the intermediate CA to the cluster
func PushIntermediateCA(k kubectl.Kubectl, ns, zone, network, basePath string, cl client.Client) error {
	// Set cert dir
	certDir := filepath.Join(basePath, "certs")

	// Check if the secret exists in the cluster
	_, err := common.GetObject(context.Background(), cl, kube.Key("cacerts", ns), &corev1.Secret{})
	if err != nil {
		// Read the pem content from the files
		caCertPath := filepath.Join(certDir, zone, "ca-cert.pem")
		caKeyPath := filepath.Join(certDir, zone, "ca-key.pem")
		rootCertPath := filepath.Join(certDir, "root-cert.pem")
		certChainPath := filepath.Join(certDir, zone, "cert-chain.pem")

		// Read the pem content from the files to create the secret
		caCert, err := os.ReadFile(caCertPath)
		if err != nil {
			return fmt.Errorf("failed to read ca-cert.pem: %w", err)
		}
		caKey, err := os.ReadFile(caKeyPath)
		if err != nil {
			return fmt.Errorf("failed to read ca-key.pem: %w", err)
		}
		rootCert, err := os.ReadFile(rootCertPath)
		if err != nil {
			return fmt.Errorf("failed to read root-cert.pem: %w", err)
		}
		certChain, err := os.ReadFile(certChainPath)
		if err != nil {
			return fmt.Errorf("failed to read cert-chain.pem: %w", err)
		}

		// Create the secret by using the client in the cluster and the files created in the setup
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cacerts",
				Namespace: ns,
			},
			Data: map[string][]byte{
				"ca-cert.pem":    caCert,
				"ca-key.pem":     caKey,
				"root-cert.pem":  rootCert,
				"cert-chain.pem": certChain,
			},
		}

		err = cl.Create(context.Background(), secret)
		if err != nil {
			return fmt.Errorf("failed to create secret: %w", err)
		}
	}

	return nil
}
