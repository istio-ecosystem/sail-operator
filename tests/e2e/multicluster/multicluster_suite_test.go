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

package multicluster

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/certs"
	k8sclient "github.com/istio-ecosystem/sail-operator/tests/e2e/util/client"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/env"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/kubectl"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	clPrimary             client.Client
	clRemote              client.Client
	err                   error
	ocp                   = env.GetBool("OCP", false)
	namespace             = env.Get("NAMESPACE", "sail-operator")
	deploymentName        = env.Get("DEPLOYMENT_NAME", "sail-operator")
	controlPlaneNamespace = env.Get("CONTROL_PLANE_NS", "istio-system")
	istioName             = env.Get("ISTIO_NAME", "default")
	image                 = env.Get("IMAGE", "quay.io/maistra-dev/sail-operator:latest")
	skipDeploy            = env.GetBool("SKIP_DEPLOY", false)
	multicluster          = env.GetBool("MULTICLUSTER", false)
	kubeconfig            = env.Get("KUBECONFIG", "")
	kubeconfig2           = env.Get("KUBECONFIG2", "")
	artifacts             = env.Get("ARTIFACTS", "/tmp/artifacts")

	eastGatewayYAML   string
	westGatewayYAML   string
	exposeServiceYAML string
	exposeIstiodYAML  string

	kubectlClient1 *kubectl.KubectlBuilder
	kubectlClient2 *kubectl.KubectlBuilder
)

func TestInstall(t *testing.T) {
	if !multicluster {
		t.Skip("Skipping test. Only valid for multicluster")
	}
	if ocp {
		t.Skip("Skipping test. Not valid for OCP")
		// TODO: Implement the steps to run the test on OCP
		// https://github.com/istio-ecosystem/sail-operator/issues/365
	}
	RegisterFailHandler(Fail)
	setup(t)
	RunSpecs(t, "Control Plane Suite")
}

func setup(t *testing.T) {
	GinkgoWriter.Println("************ Running Setup ************")

	GinkgoWriter.Println("Initializing k8s client")
	clPrimary, err = k8sclient.InitK8sClient(kubeconfig)
	clRemote, err = k8sclient.InitK8sClient(kubeconfig2)
	if err != nil {
		t.Fatalf("Error initializing k8s client: %v", err)
	}

	err := certs.CreateIntermediateCA(artifacts)
	if err != nil {
		t.Fatalf("Error creating intermediate CA: %v", err)
	}

	// Set the path for the multicluster YAML files to be used
	workDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Error getting working directory: %v", err)
	}

	// Set base path
	baseRepoDir := filepath.Join(workDir, "../../..")
	eastGatewayYAML = fmt.Sprintf("%s/docs/multicluster/east-west-gateway-net1.yaml", baseRepoDir)
	westGatewayYAML = fmt.Sprintf("%s/docs/multicluster/east-west-gateway-net2.yaml", baseRepoDir)
	exposeServiceYAML = fmt.Sprintf("%s/docs/multicluster/expose-services.yaml", baseRepoDir)
	exposeIstiodYAML = fmt.Sprintf("%s/docs/multicluster/expose-istiod.yaml", baseRepoDir)

	// Initialize kubectl utilities, one for each cluster
	kubectlClient1 = kubectl.NewKubectlBuilder().SetKubeconfig(kubeconfig)
	kubectlClient2 = kubectl.NewKubectlBuilder().SetKubeconfig(kubeconfig2)
}
