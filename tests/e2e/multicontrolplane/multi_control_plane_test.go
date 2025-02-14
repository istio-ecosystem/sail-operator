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
// WITHOUT WARRANTIES OR Condition OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controlplane

import (
	"fmt"
	"time"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversions"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	. "github.com/istio-ecosystem/sail-operator/tests/e2e/util/gomega"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Multi control plane deployment model", Ordered, func() {
	SetDefaultEventuallyTimeout(180 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)
	debugInfoLogged := false

	BeforeAll(func(ctx SpecContext) {
		Expect(k.CreateNamespace(namespace)).To(Succeed(), "Namespace failed to be created")

		if skipDeploy {
			Success("Skipping operator installation because it was deployed externally")
		} else {
			Expect(common.InstallOperatorViaHelm()).
				To(Succeed(), "Operator failed to be deployed")
		}

		Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(deploymentName, namespace), &appsv1.Deployment{}).
			Should(HaveCondition(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Error getting Istio CRD")
		Success("Operator is deployed in the namespace and Running")
	})

	Describe("Installation", func() {
		It("Sets up namespaces", func(ctx SpecContext) {
			Expect(k.CreateNamespace(istioCniNamespace)).To(Succeed(), "IstioCNI namespace failed to be created")
			Expect(k.CreateNamespace(controlPlaneNamespace1)).To(Succeed(), "Istio namespace failed to be created")
			Expect(k.CreateNamespace(controlPlaneNamespace2)).To(Succeed(), "Istio namespace failed to be created")

			Expect(k.Label("namespace", controlPlaneNamespace1, "mesh", istioName1)).To(Succeed(), "Failed to label namespace")
			Expect(k.Label("namespace", controlPlaneNamespace2, "mesh", istioName2)).To(Succeed(), "Failed to label namespace")
		})

		It("Installs IstioCNI", func(ctx SpecContext) {
			yaml := `
apiVersion: sailoperator.io/v1
kind: IstioCNI
metadata:
  name: default
spec:
  version: %s
  namespace: %s`
			yaml = fmt.Sprintf(yaml, version, istioCniNamespace)
			Expect(k.CreateFromString(yaml)).To(Succeed(), "failed to create IstioCNI")
			Success("IstioCNI created")

			Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(istioCniName), &v1.IstioCNI{}).
				Should(HaveCondition(v1.IstioCNIConditionReady, metav1.ConditionTrue), "IstioCNI is not Ready; unexpected Condition")
			Success("IstioCNI is Ready")
		})

		DescribeTable("Installs Istios",
			Entry("Mesh 1", istioName1, controlPlaneNamespace1),
			Entry("Mesh 2", istioName2, controlPlaneNamespace2),
			func(ctx SpecContext, name, ns string) {
				Expect(k.CreateFromString(`
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: `+name+`
spec:
  version: `+version+`
  namespace: `+ns+`
  values:
    meshConfig:
      discoverySelectors:
      - matchLabels:
          mesh: `+name)).To(Succeed(), "failed to create Istio CR")

				Expect(k.CreateFromString(`
apiVersion: security.istio.io/v1
kind: PeerAuthentication
metadata:
  name: default
  namespace: `+ns+`
spec:
  mtls:
    mode: STRICT`)).To(Succeed(), "failed to create PeerAuthentication")
			})

		DescribeTable("Waits for Istios",
			Entry("Mesh 1", istioName1),
			Entry("Mesh 2", istioName2),
			func(ctx SpecContext, name string) {
				Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(name), &v1.Istio{}).
					Should(
						And(
							HaveCondition(v1.IstioConditionReconciled, metav1.ConditionTrue),
							HaveCondition(v1.IstioConditionReady, metav1.ConditionTrue),
						), "Istio is not Reconciled and Ready; unexpected Condition")
				Success(fmt.Sprintf("Istio %s ready", name))
			})

		DescribeTable("Deploys applications",
			Entry("App 1", appNamespace1, istioName1),
			Entry("App 2a", appNamespace2a, istioName2),
			Entry("App 2b", appNamespace2b, istioName2),
			func(ns, mesh string) {
				Expect(k.CreateNamespace(ns)).To(Succeed(), "Failed to create namespace")
				Expect(k.Label("namespace", ns, "mesh", mesh)).To(Succeed(), "Failed to label namespace")
				Expect(k.Label("namespace", ns, "istio.io/rev", mesh)).To(Succeed(), "Failed to label namespace")
				for _, appName := range []string{"sleep", "httpbin"} {
					Expect(k.WithNamespace(ns).
						Apply(common.GetSampleYAML(istioversions.Map[version], appName))).
						To(Succeed(), "Failed to deploy application")
				}
				Success(fmt.Sprintf("Applications in namespace %s deployed", ns))
			})

		DescribeTable("Waits for apps to be ready",
			Entry("App 1", appNamespace1),
			Entry("App 2a", appNamespace2a),
			Entry("App 2b", appNamespace2b),
			func(ctx SpecContext, ns string) {
				for _, deployment := range []string{"sleep", "httpbin"} {
					Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(deployment, ns), &appsv1.Deployment{}).
						Should(HaveCondition(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Error waiting for deployment to be available")
				}
				Success(fmt.Sprintf("Applications in namespace %s ready", ns))
			})
	})

	Describe("Verification", func() {
		It("Verifies app2a cannot connect to app1", func(ctx SpecContext) {
			output, err := k.WithNamespace(appNamespace2a).
				Exec("deploy/sleep", "sleep", fmt.Sprintf("curl -sIL http://httpbin.%s:8000", appNamespace1))
			Expect(err).NotTo(HaveOccurred(), "error running curl in sleep pod")
			Expect(output).To(ContainSubstring("503 Service Unavailable"), fmt.Sprintf("Unexpected response from sleep pod in namespace %s", appNamespace1))
			Success("As expected, app2a in mesh2 is not allowed to communicate with app1 in mesh1")
		})

		It("Verifies app2a can connect to app2b", func(ctx SpecContext) {
			output, err := k.WithNamespace(appNamespace2a).
				Exec("deploy/sleep", "sleep", fmt.Sprintf("curl -sIL http://httpbin.%s:8000", appNamespace2b))
			Expect(err).NotTo(HaveOccurred(), "error running curl in sleep pod")
			Expect(output).To(ContainSubstring("200 OK"), fmt.Sprintf("Unexpected response from sleep pod in namespace %s", appNamespace2b))
			Success("As expected, app2a in mesh2 can communicate with app2b in the same mesh")
		})
	})

	AfterAll(func() {
		By("Cleaning up the application namespaces")
		Expect(k.DeleteNamespace(appNamespace1, appNamespace2a, appNamespace2b)).To(Succeed())

		By("Cleaning up the Istio namespace")
		Expect(k.DeleteNamespace(controlPlaneNamespace1, controlPlaneNamespace2)).To(Succeed(), "Istio Namespaces failed to be deleted")

		By("Cleaning up the IstioCNI namespace")
		Expect(k.DeleteNamespace(istioCniNamespace)).To(Succeed(), "IstioCNI Namespace failed to be deleted")

		By("Deleting any left-over Istio and IstioRevision resources")
		Expect(k.Delete("istio", istioName1)).To(Succeed(), "Failed to delete Istio")
		Expect(k.Delete("istio", istioName2)).To(Succeed(), "Failed to delete Istio")
		Expect(k.Delete("istiocni", istioCniName)).To(Succeed(), "Failed to delete IstioCNI")
		Success("Istio Resources deleted")
		Success("Cleanup done")
	})

	AfterAll(func() {
		if CurrentSpecReport().Failed() && !debugInfoLogged {
			common.LogDebugInfo(common.MultiControlPlane, k)
			debugInfoLogged = true
		}

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
})
