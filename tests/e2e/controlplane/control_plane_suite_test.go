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

package controlplane

import (
	"testing"

	k8sclient "github.com/istio-ecosystem/sail-operator/tests/e2e/util/client"
	env "github.com/istio-ecosystem/sail-operator/tests/e2e/util/env"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	cl                    client.Client
	err                   error
	ocp                   = env.GetBool("OCP", false)
	namespace             = env.Get("NAMESPACE", "sail-operator")
	deploymentName        = env.Get("DEPLOYMENT_NAME", "sail-operator")
	controlPlaneNamespace = env.Get("CONTROL_PLANE_NS", "istio-system")
	istioName             = env.Get("ISTIO_NAME", "default")
	istioCniNamespace     = env.Get("ISTIOCNI_NAMESPACE", "istio-cni")
	istioCniName          = env.Get("ISTIOCNI_NAME", "default")
	image                 = env.Get("IMAGE", "quay.io/maistra-dev/sail-operator:latest")
	skipDeploy            = env.GetBool("SKIP_DEPLOY", false)
	expectedRegistry      = env.Get("EXPECTED_REGISTRY", "^docker\\.io|^gcr\\.io")
	bookinfoNamespace     = env.Get("BOOKINFO_NAMESPACE", "bookinfo")
)

func TestInstall(t *testing.T) {
	RegisterFailHandler(Fail)
	setup()
	RunSpecs(t, "Control Plane Suite")
}

func setup() {
	GinkgoWriter.Println("************ Running Setup ************")

	GinkgoWriter.Println("Initializing k8s client")
	cl, err = k8sclient.InitK8sClient()
	Expect(err).NotTo(HaveOccurred())
}
