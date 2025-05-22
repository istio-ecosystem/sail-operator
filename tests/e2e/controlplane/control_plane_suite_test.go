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
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	k8sclient "github.com/istio-ecosystem/sail-operator/tests/e2e/util/client"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	. "github.com/istio-ecosystem/sail-operator/tests/e2e/util/gomega"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/kubectl"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	cl                    client.Client
	err                   error
	namespace             = common.OperatorNamespace
	deploymentName        = env.Get("DEPLOYMENT_NAME", "sail-operator")
	controlPlaneNamespace = env.Get("CONTROL_PLANE_NS", "istio-system")
	istioName             = env.Get("ISTIO_NAME", "default")
	istioCniNamespace     = env.Get("ISTIOCNI_NAMESPACE", "istio-cni")
	istioCniName          = env.Get("ISTIOCNI_NAME", "default")
	skipDeploy            = env.GetBool("SKIP_DEPLOY", false)
	expectedRegistry      = env.Get("EXPECTED_REGISTRY", "^docker\\.io|^gcr\\.io")
	sampleNamespace       = env.Get("SAMPLE_NAMESPACE", "sample")
	multicluster          = env.GetBool("MULTICLUSTER", false)
	keepOnFailure         = env.GetBool("KEEP_ON_FAILURE", false)
	ipFamily              = env.Get("IP_FAMILY", "ipv4")

	k kubectl.Kubectl
)

func TestControlPlane(t *testing.T) {
	if ipFamily == "dual" || multicluster {
		t.Skip("Skipping the control plane tests")
	}
	RegisterFailHandler(Fail)
	setup()
	RunSpecs(t, "Control Plane Test Suite")
}

func setup() {
	GinkgoWriter.Println("************ Running Setup ************")

	GinkgoWriter.Println("Initializing k8s client")
	cl, err = k8sclient.InitK8sClient("")
	Expect(err).NotTo(HaveOccurred())

	k = kubectl.New()
}

var _ = BeforeSuite(func(ctx SpecContext) {
	Expect(k.CreateNamespace(namespace)).To(Succeed(), "Namespace failed to be created")

	if skipDeploy {
		Success("Skipping operator installation because it was deployed externally")
	} else {
		Expect(common.InstallOperatorViaHelm()).
			To(Succeed(), "Operator failed to be deployed")
	}

	Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(deploymentName, namespace), &appsv1.Deployment{}).
		Should(HaveConditionStatus(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Error getting Istio CRD")
	Success("Operator is deployed in the namespace and Running")
})

var _ = AfterSuite(func(ctx SpecContext) {
	if skipDeploy {
		Success("Skipping operator undeploy because it was deployed externally")
		return
	}

	By("Deleting operator deployment")
	Expect(common.UninstallOperator()).
		To(Succeed(), "Operator failed to be deleted")
	GinkgoWriter.Println("Operator uninstalled")

	Expect(k.DeleteNamespace(namespace)).To(Succeed(), "Namespace failed to be deleted")
	Success("Namespace deleted")
})
