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

package operator

import (
	"testing"

	"github.com/istio-ecosystem/sail-operator/pkg/env"
	k8sclient "github.com/istio-ecosystem/sail-operator/tests/e2e/util/client"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/kubectl"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	cl                 client.Client
	skipDeploy         = env.GetBool("SKIP_DEPLOY", false)
	namespace          = common.OperatorNamespace
	deploymentName     = env.Get("DEPLOYMENT_NAME", "sail-operator")
	serviceAccountName = deploymentName
	multicluster       = env.GetBool("MULTICLUSTER", false)
	curlNamespace      = "curl-metrics"

	k kubectl.Kubectl
)

func TestInstall(t *testing.T) {
	if multicluster {
		t.Skip("Skipping test for multicluster")
	}
	RegisterFailHandler(Fail)
	setup()
	RunSpecs(t, "Operator Installation Test Suite")
}

func setup() {
	GinkgoWriter.Println("************ Running Setup ************")

	GinkgoWriter.Println("Initializing k8s client")
	var err error
	cl, err = k8sclient.InitK8sClient("")
	Expect(err).NotTo(HaveOccurred())

	k = kubectl.New("clOperator")
}
