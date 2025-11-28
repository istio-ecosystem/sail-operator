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
	"fmt"
	"time"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/cleaner"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
)

var _ = Describe("Multi control plane deployment model", Label("smoke", "multicontrol-plane"), Ordered, func() {
	SetDefaultEventuallyTimeout(180 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)
	debugInfoLogged := false
	latestVersion := istioversion.GetLatestPatchVersions()[0]

	Describe("for supported versions", func() {
		for _, version := range istioversion.GetLatestPatchVersions() {
			Context(fmt.Sprintf("Istio version %s", version.Version), func() {
				clr := cleaner.New(cl)

				BeforeAll(func(ctx SpecContext) {
					clr.Record(ctx)
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
						common.CreateIstioCNI(k, latestVersion.Name)
						common.AwaitCondition(ctx, v1.IstioCNIConditionReady, kube.Key(istioCniName), &v1.IstioCNI{}, k, cl)
					})

					DescribeTable("Installs Istios",
						Entry("Mesh 1", istioName1, controlPlaneNamespace1, latestVersion.Name),
						Entry("Mesh 2", istioName2, controlPlaneNamespace2, version.Name),
						func(ctx SpecContext, name, ns, version string) {
							Expect(k.CreateFromString(fmt.Sprintf(`
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: %s
spec:
  version: %s
  namespace: %s
  values:
    meshConfig:
      discoverySelectors:
        - matchLabels:
            mesh: %s`, name, version, ns, name))).To(Succeed(), "failed to create Istio CR")

							Expect(k.CreateFromString(fmt.Sprintf(`
apiVersion: security.istio.io/v1
kind: PeerAuthentication
metadata:
  name: default
  namespace: %s
spec:
  mtls:
    mode: STRICT`, ns))).To(Succeed(), "failed to create PeerAuthentication")
						},
					)

					DescribeTable("Waits for Istios",
						Entry("Mesh 1", istioName1),
						Entry("Mesh 2", istioName2),
						func(ctx SpecContext, name string) {
							common.AwaitCondition(ctx, v1.IstioConditionReconciled, kube.Key(name), &v1.Istio{}, k, cl)
							common.AwaitCondition(ctx, v1.IstioConditionReady, kube.Key(name), &v1.Istio{}, k, cl)
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
									ApplyKustomize(appName)).
									To(Succeed(), "Failed to deploy application")
							}

							Success(fmt.Sprintf("Applications in namespace %s deployed", ns))
						},
					)

					DescribeTable("Waits for apps to be ready",
						Entry("App 1", appNamespace1),
						Entry("App 2a", appNamespace2a),
						Entry("App 2b", appNamespace2b),
						func(ctx SpecContext, ns string) {
							for _, deployment := range []string{"sleep", "httpbin"} {
								common.AwaitCondition(ctx, appsv1.DeploymentAvailable, kube.Key(deployment, ns), &appsv1.Deployment{}, k, cl)
							}
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

				AfterAll(func(ctx SpecContext) {
					if CurrentSpecReport().Failed() {
						common.LogDebugInfo(common.ControlPlane, k)
						debugInfoLogged = true
					}
					clr.Cleanup(ctx)
				})
			})
		}

		AfterAll(func(ctx SpecContext) {
			if CurrentSpecReport().Failed() {
				if !debugInfoLogged {
					common.LogDebugInfo(common.MultiControlPlane, k)
					debugInfoLogged = true
				}
			}
		})
	})
})
