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

package ambient

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
	cl                    client.Client
	err                   error
	controlPlaneNamespace = common.ControlPlaneNamespace
	istioName             = env.Get("ISTIO_NAME", "default")
	istioCniNamespace     = common.IstioCniNamespace
	ztunnelNamespace      = common.ZtunnelNamespace
	istioCniName          = env.Get("ISTIOCNI_NAME", "default")
	expectedRegistry      = env.Get("EXPECTED_REGISTRY", "^docker\\.io|^gcr\\.io")
	multicluster          = env.GetBool("MULTICLUSTER", false)
	keepOnFailure         = env.GetBool("KEEP_ON_FAILURE", false)

	k kubectl.Kubectl
)

func TestAmbient(t *testing.T) {
	if multicluster {
		t.Skip("Skipping the Ambient tests")
	}

	RegisterFailHandler(Fail)
	setup()
	RunSpecs(t, "Ambient Test Suite")
}

func setup() {
	GinkgoWriter.Println("************ Running Setup ************")

	GinkgoWriter.Println("Initializing k8s client")
	cl, err = k8sclient.InitK8sClient("")
	Expect(err).NotTo(HaveOccurred())

	k = kubectl.New()
}
