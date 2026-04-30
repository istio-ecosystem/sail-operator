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

package ztwim

import (
	"testing"

	"github.com/istio-ecosystem/sail-operator/pkg/env"
	k8sclient "github.com/istio-ecosystem/sail-operator/tests/e2e/util/client"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/kubectl"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/shell"
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
	istioCniName          = env.Get("ISTIOCNI_NAME", "default")
	ztwimNamespace        = env.Get("ZTWIM_NAMESPACE", "zero-trust-workload-identity-manager")
	trustDomain           = env.Get("TRUST_DOMAIN", "ocp.one")
	jwtIssuer             = env.Get("JWT_ISSUER", "")
	multicluster          = env.GetBool("MULTICLUSTER", false)

	k kubectl.Kubectl
)

// isOpenshift dynamically checks if the cluster is OCP by looking for a core OpenShift API resource
func isOpenshift() bool {
	_, err := shell.ExecuteShell("kubectl get clusterversion", "")
	return err == nil
}

func TestZTWIM(t *testing.T) {
	// SKIPPING test until https://github.com/istio-ecosystem/sail-operator/issues/1898 is fixed.
	// This is causing test failures on CI and we want to unblock sync jobs.
	t.Skip("Skipping ZTWIM test until https://github.com/istio-ecosystem/sail-operator/issues/1898 is fixed")
	if multicluster {
		t.Skip("Skipping test for multicluster")
	}

	if !isOpenshift() {
		t.Skip("Skipping test: ZTWIM is only supported on OpenShift (OCP)")
	}

	RegisterFailHandler(Fail)
	setup()
	RunSpecs(t, "Zero Trust Workload Identity Manager Test Suite")
}

func setup() {
	GinkgoWriter.Println("************ Running Setup ************")

	GinkgoWriter.Println("Initializing k8s client")
	cl, err = k8sclient.InitK8sClient("")
	Expect(err).NotTo(HaveOccurred())

	k = kubectl.New()
}
