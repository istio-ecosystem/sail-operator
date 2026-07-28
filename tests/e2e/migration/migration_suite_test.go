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

package migration

import (
	"context"
	"testing"

	"github.com/istio-ecosystem/sail-operator/pkg/env"
	k8sclient "github.com/istio-ecosystem/sail-operator/tests/e2e/util/client"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/kubectl"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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
	multicluster          = env.GetBool("MULTICLUSTER", false)
	keepOnFailure         = env.GetBool("KEEP_ON_FAILURE", false)
	defaultTimeout        = env.GetInt("DEFAULT_TEST_TIMEOUT", 180)

	k kubectl.Kubectl
)

func TestMigration(t *testing.T) {
	if multicluster {
		t.Skip("Skipping the Migration tests in multicluster environment")
	}

	RegisterFailHandler(Fail)
	setup()
	RunSpecs(t, "Migration Test Suite")
}

func setup() {
	GinkgoWriter.Println("************ Running Setup ************")

	GinkgoWriter.Println("Initializing k8s client")
	cl, err = k8sclient.InitK8sClient("")
	Expect(err).NotTo(HaveOccurred())

	k = kubectl.New()

	// Install Gateway API CRDs if not already present (needed for waypoint)
	ctx := context.Background()
	crd := &apiextensionsv1.CustomResourceDefinition{}
	err = cl.Get(ctx, client.ObjectKey{Name: "gateways.gateway.networking.k8s.io"}, crd)
	if err != nil {
		GinkgoWriter.Println("Gateway API CRDs not found, installing from upstream")
		Expect(k.Apply("https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.0/standard-install.yaml")).
			To(Succeed(), "Failed to install Gateway API CRDs")
		GinkgoWriter.Println("Gateway API CRDs installed (standard channel v1.2.0)")
	} else {
		GinkgoWriter.Println("Gateway API CRDs already present in cluster")
	}
}
