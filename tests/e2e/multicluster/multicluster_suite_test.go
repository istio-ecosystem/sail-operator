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

	"github.com/istio-ecosystem/sail-operator/pkg/env"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/certs"
	k8sclient "github.com/istio-ecosystem/sail-operator/tests/e2e/util/client"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/kubectl"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	clPrimary                     client.Client
	clRemote                      client.Client
	err                           error
	debugInfoLogged               bool
	controlPlaneNamespace         = common.ControlPlaneNamespace
	externalControlPlaneNamespace = env.Get("EXTERNAL_CONTROL_PLANE_NS", "external-istiod")
	istioName                     = env.Get("ISTIO_NAME", "default")
	istioCniNamespace             = common.IstioCniNamespace
	istioCniName                  = env.Get("ISTIOCNI_NAME", "default")
	multicluster                  = env.GetBool("MULTICLUSTER", false)
	keepOnFailure                 = env.GetBool("KEEP_ON_FAILURE", false)
	kubeconfig                    = env.Get("KUBECONFIG", "")
	kubeconfig2                   = env.Get("KUBECONFIG2", "")
	artifacts                     = env.Get("ARTIFACTS", "/tmp/artifacts")
	sampleNamespace               = env.Get("SAMPLE_NAMESPACE", "sample")

	controlPlaneGatewayYAML string
	eastGatewayYAML         string
	westGatewayYAML         string
	exposeServiceYAML       string
	exposeIstiodYAML        string

	k1 kubectl.Kubectl
	k2 kubectl.Kubectl
)

func TestMultiCluster(t *testing.T) {
	if !multicluster {
		t.Skip("Skipping test. Only valid for multicluster")
	}
	if kubeconfig == "" && kubeconfig2 == "" {
		t.Skip("Skipping test. Two clusters required for multicluster test")
	}
	RegisterFailHandler(Fail)
	setup(t)
	RunSpecs(t, "Multi-Cluster Test Suite")
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
	controlPlaneGatewayYAML = fmt.Sprintf("%s/docs/deployment-models/resources/controlplane-gateway.yaml", baseRepoDir)
	eastGatewayYAML = fmt.Sprintf("%s/docs/deployment-models/resources/east-west-gateway-net1.yaml", baseRepoDir)
	westGatewayYAML = fmt.Sprintf("%s/docs/deployment-models/resources/east-west-gateway-net2.yaml", baseRepoDir)
	exposeServiceYAML = fmt.Sprintf("%s/docs/deployment-models/resources/expose-services.yaml", baseRepoDir)
	exposeIstiodYAML = fmt.Sprintf("%s/docs/deployment-models/resources/expose-istiod.yaml", baseRepoDir)

	// Initialize kubectl utilities, one for each cluster
	k1 = kubectl.New().WithKubeconfig(kubeconfig).WithClusterName("primary")
	k2 = kubectl.New().WithKubeconfig(kubeconfig2).WithClusterName("remote")
}

var _ = ReportAfterSuite("Conditional cleanup", func(ctx SpecContext, r Report) {
	if !r.SuiteSucceeded && !debugInfoLogged {
		common.LogDebugInfo(common.MultiCluster, k1, k2)
	}
})
