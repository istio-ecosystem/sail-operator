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

	"github.com/istio-ecosystem/sail-operator/pkg/env"
	k8sclient "github.com/istio-ecosystem/sail-operator/tests/e2e/util/client"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/kubectl"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	cl  client.Client
	err error
	// version will be the first version in the list, this version is the newest Istio version in the versions.yaml file
	controlPlaneNamespace1 = env.Get("CONTROL_PLANE_NS1", "istio-system1")
	controlPlaneNamespace2 = env.Get("CONTROL_PLANE_NS2", "istio-system2")
	istioName1             = env.Get("ISTIO_NAME1", "mesh1")
	istioName2             = env.Get("ISTIO_NAME2", "mesh2")
	istioCniNamespace      = common.IstioCniNamespace
	istioCniName           = env.Get("ISTIOCNI_NAME", "default")
	appNamespace1          = env.Get("APP_NAMESPACE1", "app1")
	appNamespace2a         = env.Get("APP_NAMESPACE2A", "app2a")
	appNamespace2b         = env.Get("APP_NAMESPACE2B", "app2b")
	multicluster           = env.GetBool("MULTICLUSTER", false)
	ipFamily               = env.Get("IP_FAMILY", "ipv4")

	k kubectl.Kubectl
)

func TestMultiControlPlane(t *testing.T) {
	if ipFamily == "dual" || multicluster {
		t.Skip("Skipping the multi control plane tests")
	}
	RegisterFailHandler(Fail)
	setup()
	RunSpecs(t, "Multiple Control Planes Test Suite")
}

func setup() {
	GinkgoWriter.Println("************ Running Setup ************")

	GinkgoWriter.Println("Initializing k8s client")
	cl, err = k8sclient.InitK8sClient("")
	Expect(err).NotTo(HaveOccurred())

	k = kubectl.New()
}
