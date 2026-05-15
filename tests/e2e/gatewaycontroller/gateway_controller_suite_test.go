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

package gatewaycontroller

import (
	"os"
	"testing"

	"github.com/istio-ecosystem/sail-operator/pkg/env"
	k8sclient "github.com/istio-ecosystem/sail-operator/tests/e2e/util/client"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/kubectl"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	cl         client.Client
	kubeConfig *rest.Config
	err        error

	controlPlaneNamespace = common.ControlPlaneNamespace
	istioCniNamespace     = common.IstioCniNamespace
	keepOnFailure         = env.GetBool("KEEP_ON_FAILURE", false)
	artifactsDir          = env.Get("ARTIFACTS", "/tmp/artifacts")

	k kubectl.Kubectl
)

const (
	libraryNamespace = "library-system"
	gatewayClassName = "custom-gateway"
	gatewayNamespace = "gateway-test"
	noMeshNamespace  = "no-mesh"
	meshNamespace    = "mesh-workload"
	trustBundleName  = "custom-trust-bundle"
)

func TestGatewayController(t *testing.T) {
	RegisterFailHandler(Fail)
	setup()
	RunSpecs(t, "Gateway Controller Test Suite")
}

func setup() {
	GinkgoWriter.Println("************ Running Setup ************")
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	GinkgoWriter.Println("Initializing k8s client")
	cl, err = k8sclient.InitK8sClient("")
	Expect(err).NotTo(HaveOccurred())

	GinkgoWriter.Println("Building kubeconfig")
	kubeConfig, err = clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	Expect(err).NotTo(HaveOccurred())

	k = kubectl.New()
}
