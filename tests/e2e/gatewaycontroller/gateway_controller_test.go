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
	"context"
	"encoding/json"
	"fmt"
	"time"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/chart"
	"github.com/istio-ecosystem/sail-operator/pkg/env"
	"github.com/istio-ecosystem/sail-operator/pkg/install"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/resources"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/certs"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"istio.io/istio/pkg/ptr"
)

var _ = Describe("Gateway Controller with Install Library", Label("gateway-controller"), Ordered, func() {
	SetDefaultEventuallyTimeout(time.Duration(env.GetInt("DEFAULT_TEST_TIMEOUT", 180)) * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)

	var lib *install.Library

	BeforeAll(func() {
		// Create namespaces that need cacerts before istiod starts
		Expect(k.CreateNamespace(libraryNamespace)).To(Succeed())
		Expect(k.CreateNamespace(controlPlaneNamespace)).To(Succeed())

		// Generate a shared root CA with two intermediate CAs
		Expect(certs.CreateIntermediateCA(artifactsDir)).To(Succeed())

		// Push different intermediate CAs (same root) to each istiod namespace
		Expect(certs.PushIntermediateCA(k, libraryNamespace, "east", "", artifactsDir, cl)).To(Succeed())
		Expect(certs.PushIntermediateCA(k, controlPlaneNamespace, "west", "", artifactsDir, cl)).To(Succeed())
		Success("Shared root CA with intermediate CAs pushed to both namespaces")

		var err error
		lib, err = install.New(kubeConfig, resources.FS, chart.CRDsFS)
		Expect(err).NotTo(HaveOccurred())
	})

	When("the install library deploys istiod as a gateway controller", func() {
		BeforeAll(func() {
			_, err := lib.Start(context.Background())
			Expect(err).NotTo(HaveOccurred())

			gatewayDefaults := install.GatewayAPIDefaults(libraryNamespace)

			gatewayClassesJSON, err := json.Marshal(map[string]any{
				gatewayClassName: map[string]any{},
			})
			Expect(err).NotTo(HaveOccurred())

			overlay := &v1.Values{
				Revision: ptr.Of("library"),
				Pilot: &v1.PilotConfig{
					Env: map[string]string{
						"PILOT_GATEWAY_API_CONTROLLER_NAME":           "library.io/controller",
						"PILOT_GATEWAY_API_DEFAULT_GATEWAYCLASS_NAME": gatewayClassName,
					},
				},
				Global: &v1.GlobalConfig{
					TrustBundleName: ptr.Of(trustBundleName),
				},
				MeshConfig: &v1.MeshConfig{
					AccessLogFile: ptr.Of("/dev/stdout"),
				},
				GatewayClasses: json.RawMessage(gatewayClassesJSON),
			}

			merged, err := install.MergeValues(gatewayDefaults, overlay)
			Expect(err).NotTo(HaveOccurred())

			err = lib.Apply(install.Options{
				Namespace:  libraryNamespace,
				Version:    istioversion.Default,
				Revision:   "library",
				Values:     merged,
				ManageCRDs: true,
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("deploys istiod in the library namespace", func(ctx SpecContext) {
			Eventually(func(g Gomega) {
				dep := &appsv1.Deployment{}
				g.Expect(cl.Get(ctx, kube.Key("istiod-library", libraryNamespace), dep)).To(Succeed())
				g.Expect(dep.Status.AvailableReplicas).To(BeNumerically(">=", 1))
			}).Should(Succeed())
			Success("istiod-library is deployed in " + libraryNamespace)
		})

		It("reaches a healthy status", func() {
			Eventually(func() error {
				status := lib.Status()
				if status.Installed && status.Error == nil {
					return nil
				}
				if status.Error != nil {
					return status.Error
				}
				return fmt.Errorf("not yet installed")
			}).Should(Succeed())
			Success("Library reports healthy status")
		})
	})

	When("a custom GatewayClass and Gateway are created", func() {
		BeforeAll(func() {
			gatewayClassYAML := fmt.Sprintf(`
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: %s
spec:
  controllerName: library.io/controller`, gatewayClassName)
			Expect(k.ApplyString(gatewayClassYAML)).To(Succeed())
			Success("GatewayClass created")

			Expect(k.CreateNamespace(gatewayNamespace)).To(Succeed())
			Expect(k.Label("namespace", gatewayNamespace, "istio.io/rev", "library")).To(Succeed())

			gatewayYAML := fmt.Sprintf(`
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: test-gateway
  namespace: %s
  labels:
    istio.io/rev: library
spec:
  gatewayClassName: %s
  listeners:
  - name: http
    port: 80
    protocol: HTTP
    allowedRoutes:
      namespaces:
        from: All`, gatewayNamespace, gatewayClassName)
			Expect(k.ApplyString(gatewayYAML)).To(Succeed())
			Success("Gateway created")

			// Deploy a curl client pod for traffic testing
			curlPodYAML := fmt.Sprintf(`
apiVersion: v1
kind: Pod
metadata:
  name: curl-client
  namespace: %s
  labels:
    sidecar.istio.io/inject: "false"
spec:
  containers:
  - name: curl
    image: curlimages/curl
    command: ["sleep", "3600"]`, gatewayNamespace)
			Expect(k.ApplyString(curlPodYAML)).To(Succeed())
		})

		It("gateway deployment becomes available", func(ctx SpecContext) {
			Eventually(func(g Gomega) {
				depList := &appsv1.DeploymentList{}
				g.Expect(cl.List(ctx, depList, client.InNamespace(gatewayNamespace))).To(Succeed())
				g.Expect(depList.Items).NotTo(BeEmpty(), "Gateway deployment should exist")
				for _, dep := range depList.Items {
					g.Expect(dep.Status.AvailableReplicas).To(BeNumerically(">=", 1))
				}
			}).Should(Succeed())
			Success("Gateway deployment is available")
		})

		It("curl client pod is ready", func(ctx SpecContext) {
			Eventually(func() error {
				return common.CheckPodsReady(ctx, cl, gatewayNamespace)
			}).Should(Succeed())
			Success("All pods in gateway namespace are ready")
		})
	})

	When("a workload outside the mesh routes traffic through the gateway", func() {
		BeforeAll(func() {
			Expect(k.CreateNamespace(noMeshNamespace)).To(Succeed())
			Expect(k.WithNamespace(noMeshNamespace).
				ApplyKustomize("helloworld")).
				To(Succeed())

			httpRouteYAML := fmt.Sprintf(`
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: nomesh-route
  namespace: %s
  labels:
    istio.io/rev: library
spec:
  parentRefs:
  - name: test-gateway
    namespace: %s
  hostnames:
  - nomesh.example.com
  rules:
  - backendRefs:
    - name: helloworld
      port: 5000`, noMeshNamespace, gatewayNamespace)
			Expect(k.ApplyString(httpRouteYAML)).To(Succeed())
			Success("Non-mesh workload and HTTPRoute created")
		})

		It("workload pods are ready", func(ctx SpecContext) {
			Eventually(func() error {
				return common.CheckPodsReady(ctx, cl, noMeshNamespace)
			}).Should(Succeed())
			Success("Non-mesh workload pods are ready")
		})

		It("traffic flows through the gateway to the non-mesh backend", func(ctx SpecContext) {
			gatewayAddr := fmt.Sprintf("test-gateway-%s.%s.svc.cluster.local", gatewayClassName, gatewayNamespace)

			Eventually(func(g Gomega) {
				command := fmt.Sprintf(`curl -s -o /dev/null -w "%%{http_code}" -H "Host: nomesh.example.com" http://%s/hello`, gatewayAddr)
				response, err := k.WithNamespace(gatewayNamespace).Exec("curl-client", "", command)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(response).To(ContainSubstring("200"))
			}).Should(Succeed())
			Success("Traffic flows through gateway to non-mesh backend")
		})
	})

	When("the operator installs istiod and CNI alongside", func() {
		BeforeAll(func() {
			Expect(k.CreateNamespace(istioCniNamespace)).To(Succeed())

			common.CreateIstioCNI(k, istioversion.Default)
			common.CreateIstio(k, istioversion.Default)
		})

		It("deploys the operator-managed istiod", func(ctx SpecContext) {
			common.AwaitDeployment(ctx, "istiod", k, cl)
			Success("Operator-managed istiod is deployed")
		})

		It("deploys the CNI DaemonSet", func(ctx SpecContext) {
			common.AwaitCniDaemonSet(ctx, k, cl)
			Success("CNI DaemonSet is deployed")
		})

		It("library istiod is still running", func(ctx SpecContext) {
			dep := &appsv1.Deployment{}
			Expect(cl.Get(ctx, kube.Key("istiod-library", libraryNamespace), dep)).To(Succeed())
			Expect(dep.Status.AvailableReplicas).To(BeNumerically(">=", 1))
			Success("Library istiod is still running")
		})
	})

	When("a mesh workload routes traffic through the gateway", func() {
		BeforeAll(func() {
			Expect(k.CreateNamespace(meshNamespace)).To(Succeed())
			Expect(k.Label("namespace", meshNamespace, "istio-injection", "enabled")).To(Succeed())
			Expect(k.WithNamespace(meshNamespace).
				ApplyKustomize("helloworld")).
				To(Succeed())

			httpRouteYAML := fmt.Sprintf(`
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: mesh-route
  namespace: %s
  labels:
    istio.io/rev: library
spec:
  parentRefs:
  - name: test-gateway
    namespace: %s
  hostnames:
  - mesh.example.com
  rules:
  - backendRefs:
    - name: helloworld
      port: 5000`, meshNamespace, gatewayNamespace)
			Expect(k.ApplyString(httpRouteYAML)).To(Succeed())
			Success("Mesh workload and HTTPRoute created")
		})

		It("workload pods are ready with sidecars", func(ctx SpecContext) {
			Eventually(func() error {
				return common.CheckPodsReady(ctx, cl, meshNamespace)
			}).Should(Succeed())
			Success("Mesh workload pods are ready")
		})

		It("traffic flows through the gateway to the mesh backend", func(ctx SpecContext) {
			gatewayAddr := fmt.Sprintf("test-gateway-%s.%s.svc.cluster.local", gatewayClassName, gatewayNamespace)

			Eventually(func(g Gomega) {
				command := fmt.Sprintf(`curl -s -o /dev/null -w "%%{http_code}" -H "Host: mesh.example.com" http://%s/hello`, gatewayAddr)
				response, err := k.WithNamespace(gatewayNamespace).Exec("curl-client", "", command)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(response).To(ContainSubstring("200"))
			}).Should(Succeed())
			Success("Traffic flows through gateway to mesh backend")
		})
	})

	AfterAll(func(ctx SpecContext) {
		if CurrentSpecReport().Failed() {
			common.LogDebugInfo(common.ControlPlane, k)
		}

		if CurrentSpecReport().Failed() && keepOnFailure {
			return
		}

		lib.Stop()
		if err := lib.Uninstall(ctx, libraryNamespace, "library"); err != nil {
			GinkgoWriter.Printf("Warning: library uninstall failed: %v\n", err)
		}

		k.Delete("istio", "default")
		k.Delete("istiocni", "default")
		k.WithNamespace(noMeshNamespace).Delete("httproute", "nomesh-route")
		k.WithNamespace(meshNamespace).Delete("httproute", "mesh-route")
		k.WithNamespace(gatewayNamespace).Delete("gateway", "test-gateway")
		k.Delete("gatewayclass", gatewayClassName)

		k.Delete("namespace", meshNamespace)
		k.Delete("namespace", noMeshNamespace)
		k.Delete("namespace", gatewayNamespace)
		k.Delete("namespace", libraryNamespace)
		k.Delete("namespace", controlPlaneNamespace)
		k.Delete("namespace", istioCniNamespace)
	})
})
