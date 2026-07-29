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
	"fmt"
	"time"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/cleaner"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	. "github.com/istio-ecosystem/sail-operator/tests/e2e/util/gomega"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Migration Procedure Validation", Ordered,
	Label("migration", "migration-procedure", "slow"), func() {
		// Tests the complete migration workflow from sidecar to ambient mode.
		// Validates L4 connectivity via ztunnel and L7 policy enforcement via waypoint in a single namespace.
		// Migration flow: sidecar → HBONE coexistence → pure ambient → waypoint activation
		SetDefaultEventuallyTimeout(time.Duration(defaultTimeout) * time.Second)
		SetDefaultEventuallyPollingInterval(time.Second)

		version := istioversion.GetLatestAmbientVersion()

		Context(fmt.Sprintf("Migration from sidecar to ambient with Istio %s version", version.Version), func() {
			var clr cleaner.Cleaner
			workloadNamespace := "workload-migration"

			BeforeAll(func(ctx SpecContext) {
				clr = cleaner.New(cl)
				clr.Record(ctx)

				Expect(k.CreateNamespace(controlPlaneNamespace)).To(Succeed())
				Expect(k.CreateNamespace(istioCniNamespace)).To(Succeed())
				Expect(k.CreateNamespace(ztunnelNamespace)).To(Succeed())
				Expect(k.CreateNamespace(workloadNamespace)).To(Succeed())

				common.CreateIstioCNI(k, version.Name)
				common.AwaitCondition(ctx, v1.IstioCNIConditionReady, kube.Key(istioCniName), &v1.IstioCNI{}, k, cl)
				Success("IstioCNI created (no profile - sidecar mode)")

				Eventually(func(g Gomega) {
					daemonset := &appsv1.DaemonSet{}
					g.Expect(cl.Get(ctx, kube.Key("istio-cni-node", istioCniNamespace), daemonset)).To(Succeed(), "Error getting IstioCNI DaemonSet")
					g.Expect(daemonset.Status.NumberAvailable).
						To(Equal(daemonset.Status.CurrentNumberScheduled), "CNI DaemonSet Pods not Available")
				}).Should(Succeed(), "CNI DaemonSet Pods are not Available")
				Success("CNI DaemonSet is running")

				common.CreateIstio(k, version.Name)
				common.AwaitCondition(ctx, v1.IstioConditionReady, kube.Key(istioName), &v1.Istio{}, k, cl)
				Success("Istio created (default profile - sidecar mode)")

				common.AwaitDeployment(ctx, "istiod", k, cl)
				Success("Istiod deployment is ready")

				// Wait for sidecar injector webhook to be ready (required for injection to work)
				Eventually(func(g Gomega) {
					webhook := &admissionregistrationv1.MutatingWebhookConfiguration{}
					g.Expect(cl.Get(ctx, kube.Key("istio-sidecar-injector"), webhook)).To(Succeed(), "Sidecar injector webhook should exist")
				}).Should(Succeed(), "Waiting for istio-sidecar-injector webhook to be created")
				Success("Sidecar injector webhook is ready")
			})

			AfterAll(func(ctx SpecContext) {
				if CurrentSpecReport().Failed() {
					common.LogDebugInfo(common.Ambient, k)
					if keepOnFailure {
						return
					}
				}
				clr.Cleanup(ctx)
			})

			Context("Sidecar to Ambient Migration", Ordered, func() {
				// Tests the complete migration workflow from sidecar to ambient mode.
				// Migration is performed without active traffic, and connectivity is verified after completion.
				// Validates: migration procedure correctness, coexistence mode (HBONE-aware sidecars),
				// L4 connectivity via ztunnel, and L7 policy enforcement via waypoint.

				When("workloads are deployed with sidecar injection", func() {
					BeforeAll(func(ctx SpecContext) {
						Expect(k.Label("namespace", workloadNamespace, "istio-injection", "enabled")).To(Succeed(), "Error labeling namespace for sidecar injection")
						Expect(k.WithNamespace(workloadNamespace).ApplyKustomize("httpbin")).To(Succeed(), "Error deploying httpbin")
						Expect(k.WithNamespace(workloadNamespace).ApplyKustomize("sleep")).To(Succeed(), "Error deploying sleep")
						Success("Httpbin and sleep workloads deployed with sidecar injection enabled")
					})

					samplePods := &corev1.PodList{}
					It("updates the pods status to Running", func(ctx SpecContext) {
						Eventually(common.CheckPodsReady).WithArguments(ctx, cl, workloadNamespace).Should(Succeed(), "Error checking status of sample pods")
						Expect(cl.List(ctx, samplePods, client.InNamespace(workloadNamespace))).To(Succeed(), "Error getting pods in workload namespace")
						Success("Workload pods are ready")
					})

					It("has sidecars injected", func(ctx SpecContext) {
						pods := &corev1.PodList{}
						Expect(cl.List(ctx, pods, client.InNamespace(workloadNamespace))).To(Succeed(), "Error getting pods in workload namespace")
						Expect(pods.Items).NotTo(BeEmpty(), "Expected at least one pod")

						for _, pod := range pods.Items {
							Expect(common.HasSidecarInjected(pod)).To(BeTrue(),
								"Pod %s should have istio-proxy in either containers or init containers", pod.Name)
						}
						Success("All pods have istio-proxy sidecar injected")
					})

					It("verifies L4 connectivity in sidecar mode", func(ctx SpecContext) {
						pods := &corev1.PodList{}
						Expect(cl.List(ctx, pods, client.InNamespace(workloadNamespace),
							client.MatchingLabels{"app": "sleep"})).To(Succeed())
						Expect(pods.Items).To(HaveLen(1))
						sleepPod := pods.Items[0].Name

						Eventually(func() error {
							return common.CheckHTTPConnectivity(k, workloadNamespace, sleepPod, "sleep", "httpbin:8000/get", 5, "200")
						}).Should(Succeed())
						Success("L4 connectivity verified in sidecar mode")
					})
				})

				When("L7 policies are deployed for sidecar mode", func() {
					It("deploys VirtualService for httpbin", func(ctx SpecContext) {
						// VirtualService configures L7 traffic routing for httpbin service:
						// - Routes all requests matching prefix "/" to httpbin:8000
						// - Sets request timeout to 10 seconds
						// - Configures retry policy: up to 3 attempts with 2-second per-try timeout
						// This validates that L7 routing works correctly in both sidecar and waypoint modes.
						// Expected behavior:
						//   - In sidecar mode: Envoy sidecar processes the VirtualService rules
						//   - In waypoint mode: Waypoint gateway processes the VirtualService rules
						virtualServiceYAML := fmt.Sprintf(`
apiVersion: networking.istio.io/v1
kind: VirtualService
metadata:
  name: httpbin
  namespace: %s
spec:
  hosts:
  - httpbin
  http:
  - match:
    - uri:
        prefix: "/"
    route:
    - destination:
        host: httpbin
        port:
          number: 8000
    timeout: 10s
    retries:
      attempts: 3
      perTryTimeout: 2s`, workloadNamespace)

						Expect(k.CreateFromString(virtualServiceYAML)).To(Succeed())
						Success("VirtualService created for sidecar mode")
					})

					It("deploys AuthorizationPolicy for httpbin", func(ctx SpecContext) {
						authzPolicyYAML := fmt.Sprintf(`
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: httpbin-authz
  namespace: %s
spec:
  selector:
    matchLabels:
      app: httpbin
  action: DENY
  rules:
  - to:
    - operation:
        paths: ["/deny"]`, workloadNamespace)

						Expect(k.CreateFromString(authzPolicyYAML)).To(Succeed())
						Success("AuthorizationPolicy created for sidecar mode")
					})

					It("verifies L7 traffic routing works in sidecar mode", func(ctx SpecContext) {
						pods := &corev1.PodList{}
						Expect(cl.List(ctx, pods, client.InNamespace(workloadNamespace),
							client.MatchingLabels{"app": "sleep"})).To(Succeed())
						sleepPod := pods.Items[0].Name

						// Test that VirtualService routes traffic correctly
						Eventually(func() error {
							return common.CheckHTTPConnectivity(k, workloadNamespace, sleepPod, "sleep", "httpbin:8000/get", 5, "200")
						}).Should(Succeed())
						Success("VirtualService routing verified in sidecar mode")
					})

					It("verifies L7 AuthorizationPolicy is enforced in sidecar mode", func(ctx SpecContext) {
						pods := &corev1.PodList{}
						Expect(cl.List(ctx, pods, client.InNamespace(workloadNamespace),
							client.MatchingLabels{"app": "sleep"})).To(Succeed())
						sleepPod := pods.Items[0].Name

						// Test allowed path
						Eventually(func() error {
							return common.CheckHTTPConnectivity(k, workloadNamespace, sleepPod, "sleep", "httpbin:8000/get", 5, "200")
						}).Should(Succeed())

						// Test denied path
						Eventually(func() error {
							return common.CheckHTTPConnectivity(k, workloadNamespace, sleepPod, "sleep", "httpbin:8000/deny", 5, "403")
						}).Should(Succeed())

						Success("AuthorizationPolicy enforcement verified in sidecar mode")
					})
				})

				When("configures Istio and IstioCNI in ambient mode", func() {
					It("updates Istio to ambient profile", func(ctx SpecContext) {
						patch := fmt.Sprintf(`{"spec":{"profile":"ambient","values":{"pilot":{"trustedZtunnelNamespace":"%s"}}}}`, ztunnelNamespace)
						Expect(k.Patch("istio", istioName, "merge", patch)).To(Succeed())
						Success("Istio updated to ambient profile with trustedZtunnelNamespace")
					})

					It("waits for Istio to reconcile with ambient profile", func(ctx SpecContext) {
						istio := &v1.Istio{}
						Eventually(func(g Gomega) {
							g.Expect(cl.Get(ctx, kube.Key(istioName), istio)).To(Succeed())
							g.Expect(istio).To(HaveConditionStatus(v1.IstioConditionReconciled, metav1.ConditionTrue))
							g.Expect(istio).To(HaveConditionStatus(v1.IstioConditionReady, metav1.ConditionTrue))
						}).Should(Succeed())
						Success("Istio reconciled with ambient profile")
					})

					It("updates IstioCNI to ambient profile", func(ctx SpecContext) {
						patch := `{"spec":{"profile":"ambient"}}`
						Expect(k.Patch("istiocni", istioCniName, "merge", patch)).To(Succeed())
						Success("IstioCNI updated to ambient profile")
					})

					It("waits for IstioCNI to reconcile with ambient profile", func(ctx SpecContext) {
						istioCNI := &v1.IstioCNI{}
						Eventually(func(g Gomega) {
							g.Expect(cl.Get(ctx, kube.Key(istioCniName), istioCNI)).To(Succeed())
							g.Expect(istioCNI).To(HaveConditionStatus(v1.IstioCNIConditionReconciled, metav1.ConditionTrue))
							g.Expect(istioCNI).To(HaveConditionStatus(v1.IstioCNIConditionReady, metav1.ConditionTrue))
						}).Should(Succeed())
						Success("IstioCNI reconciled with ambient profile")
					})

					It("creates ZTunnel CR", func(ctx SpecContext) {
						common.CreateZTunnel(k, version.Name)
						Success("ZTunnel CR created")
					})

					It("waits for ZTunnel to be Ready", func(ctx SpecContext) {
						ztunnel := &v1.ZTunnel{}
						Eventually(func(g Gomega) {
							g.Expect(cl.Get(ctx, kube.Key("default"), ztunnel)).To(Succeed())
							g.Expect(ztunnel).To(HaveConditionStatus(v1.ZTunnelConditionReconciled, metav1.ConditionTrue))
							g.Expect(ztunnel).To(HaveConditionStatus(v1.ZTunnelConditionReady, metav1.ConditionTrue))
						}).Should(Succeed())
						Success("ZTunnel is Ready")
					})

					It("verifies ZTunnel DaemonSet is running", func(ctx SpecContext) {
						Eventually(func(g Gomega) {
							daemonset := &appsv1.DaemonSet{}
							g.Expect(cl.Get(ctx, kube.Key("ztunnel", ztunnelNamespace), daemonset)).To(Succeed())
							g.Expect(daemonset.Status.NumberAvailable).To(Equal(daemonset.Status.CurrentNumberScheduled))
						}).Should(Succeed())
						Success("ZTunnel DaemonSet is running")
					})
				})

				When("workloads are restarted to get HBONE-aware sidecars", func() {
					It("waits for sidecar injector webhook to be recreated", func(ctx SpecContext) {
						// After switching to ambient profile, the webhook is recreated
						// Wait for it to exist before restarting pods
						Eventually(func(g Gomega) {
							webhook := &admissionregistrationv1.MutatingWebhookConfiguration{}
							g.Expect(cl.Get(ctx, kube.Key("istio-sidecar-injector"), webhook)).To(Succeed(), "Sidecar injector webhook should exist in coexistence mode")
						}).Should(Succeed(), "Waiting for istio-sidecar-injector webhook to be recreated")
						Success("Sidecar injector webhook is ready")
					})

					It("restarts deployments to get HBONE-aware sidecars", func(ctx SpecContext) {
						// Restart pods with existing istio-injection=enabled label
						// The ambient-configured webhook will inject HBONE-aware sidecars
						_, err := k.WithNamespace(workloadNamespace).RolloutRestart("deployment/httpbin")
						Expect(err).NotTo(HaveOccurred(), "Failed to restart deployment")
						Success("Deployment restart initiated")

						Eventually(func(g Gomega) {
							_, err := k.WithNamespace(workloadNamespace).RolloutStatus("deployment/httpbin")
							g.Expect(err).NotTo(HaveOccurred(), "Rollout status check failed")
						}).Should(Succeed(), "Waiting for deployment rollout to complete")
						Success("Deployment rollout completed")
					})

					It("waits for pods to be ready after restart", func(ctx SpecContext) {
						Eventually(func(g Gomega) {
							pods := &corev1.PodList{}
							g.Expect(cl.List(ctx, pods, client.InNamespace(workloadNamespace))).To(Succeed())
							g.Expect(pods.Items).NotTo(BeEmpty())

							for _, pod := range pods.Items {
								g.Expect(pod.DeletionTimestamp).To(BeNil(), "Pod %s should not be terminating", pod.Name)
							}

							for _, pod := range pods.Items {
								g.Expect(pod.Status.Phase).To(Equal(corev1.PodRunning), "Pod %s should be running", pod.Name)

								readyCount := 0
								for _, condition := range pod.Status.Conditions {
									if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
										readyCount++
									}
								}
								g.Expect(readyCount).To(Equal(1), "Pod %s should have Ready condition", pod.Name)
							}
						}).Should(Succeed(), "Waiting for pods to be fully ready without any terminating")
						Success("Pods are ready after restart")
					})

					It("verifies sidecars are present after switching to ambient profile", func(ctx SpecContext) {
						// After switching to ambient profile and restarting pods, verify istio-proxy still exists
						// Use the same check as the initial sidecar verification
						pods := &corev1.PodList{}
						Expect(cl.List(ctx, pods, client.InNamespace(workloadNamespace))).To(Succeed(), "Error getting pods in workload namespace")
						Expect(pods.Items).NotTo(BeEmpty(), "Expected at least one pod")

						for _, pod := range pods.Items {
							Expect(common.HasSidecarInjected(pod)).To(BeTrue(),
								"Pod %s should have istio-proxy in either containers or init containers", pod.Name)
						}
						Success("All pods still have istio-proxy after switching to ambient profile")
					})

					It("verifies HBONE capability is enabled in sidecars", func(ctx SpecContext) {
						pods := &corev1.PodList{}
						Expect(cl.List(ctx, pods, client.InNamespace(workloadNamespace))).To(Succeed())
						Expect(pods.Items).NotTo(BeEmpty())

						pod := pods.Items[0]
						Expect(common.HasSidecarInjected(pod)).To(BeTrue(), "istio-proxy container not found in pod %s", pod.Name)
						Expect(common.HasHBONEEnabled(pod)).To(BeTrue(), "ISTIO_META_ENABLE_HBONE env var not set to 'true' in istio-proxy container")
						Success("HBONE capability verified in sidecar (ISTIO_META_ENABLE_HBONE=true)")
					})

					It("verifies L4 connectivity works with HBONE-aware sidecars", func(ctx SpecContext) {
						pods := &corev1.PodList{}
						Expect(cl.List(ctx, pods, client.InNamespace(workloadNamespace),
							client.MatchingLabels{"app": "sleep"})).To(Succeed())
						Expect(pods.Items).To(HaveLen(1))
						sleepPod := pods.Items[0].Name

						Eventually(func() error {
							targetURL := fmt.Sprintf("httpbin.%s.svc.cluster.local:8000/get", workloadNamespace)
							return common.CheckHTTPConnectivity(k, workloadNamespace, sleepPod, "sleep", targetURL, 5, "200")
						}).Should(Succeed())
						Success("L4 connectivity verified with HBONE-aware sidecars in coexistence mode")
					})
				})

				When("namespace is switched to ambient mode", func() {
					It("removes sidecar injection label and adds ambient label", func(ctx SpecContext) {
						// Remove istio-injection label and add ambient dataplane mode
						ns := &corev1.Namespace{}
						Expect(cl.Get(ctx, kube.Key(workloadNamespace), ns)).To(Succeed())
						delete(ns.Labels, "istio-injection")
						ns.Labels["istio.io/dataplane-mode"] = "ambient"
						Expect(cl.Update(ctx, ns)).To(Succeed())
						Success(fmt.Sprintf("Namespace %s switched to ambient mode", workloadNamespace))
					})

					It("restarts httpbin pods to remove sidecars", func(ctx SpecContext) {
						_, err := k.WithNamespace(workloadNamespace).RolloutRestart("deployment/httpbin")
						Expect(err).NotTo(HaveOccurred(), "Failed to restart deployment")
						Success("Deployment restart initiated")

						Eventually(func(g Gomega) {
							_, err := k.WithNamespace(workloadNamespace).RolloutStatus("deployment/httpbin")
							g.Expect(err).NotTo(HaveOccurred(), "Rollout status check failed")
						}).Should(Succeed(), "Waiting for deployment rollout to complete")
						Success("Deployment rollout completed")
					})

					It("restarts sleep deployment to remove sidecars", func(ctx SpecContext) {
						_, err := k.WithNamespace(workloadNamespace).RolloutRestart("deployment/sleep")
						Expect(err).NotTo(HaveOccurred(), "Failed to restart sleep deployment")

						Eventually(func(g Gomega) {
							_, err := k.WithNamespace(workloadNamespace).RolloutStatus("deployment/sleep")
							g.Expect(err).NotTo(HaveOccurred(), "Rollout status check failed")
						}).Should(Succeed(), "Waiting for sleep deployment rollout to complete")
						Success("Sleep deployment rollout completed")
					})

					It("waits for pods to be ready in ambient mode without sidecar", func(ctx SpecContext) {
						Eventually(func(g Gomega) {
							pods := &corev1.PodList{}
							g.Expect(cl.List(ctx, pods, client.InNamespace(workloadNamespace))).To(Succeed())

							// Filter out waypoint pods
							workloadPods := []corev1.Pod{}
							for _, pod := range pods.Items {
								if pod.DeletionTimestamp == nil {
									isWaypoint := false
									for key := range pod.Labels {
										if key == "gateway.istio.io/managed" || key == "gateway.networking.k8s.io/gateway-name" {
											isWaypoint = true
											break
										}
									}
									if !isWaypoint {
										workloadPods = append(workloadPods, pod)
									}
								}
							}

							g.Expect(workloadPods).NotTo(BeEmpty(), "Expected at least one workload pod")

							for _, pod := range workloadPods {
								g.Expect(pod.Status.Phase).To(Equal(corev1.PodRunning), "Pod %s should be running", pod.Name)
								g.Expect(pod.Spec.Containers).To(HaveLen(1), "Pod %s should have 1 container (no sidecar in ambient mode)", pod.Name)
								g.Expect(pod.Annotations).To(HaveKey("ambient.istio.io/redirection"), "Ambient redirection annotation should be present")
							}
						}).Should(Succeed())
						Success("Pods running in ambient mode without sidecars")
					})

					It("verifies workloads are registered with ztunnel", func(ctx SpecContext) {
						pods := &corev1.PodList{}
						Expect(cl.List(ctx, pods, client.InNamespace(workloadNamespace))).To(Succeed())
						Expect(pods.Items).NotTo(BeEmpty())

						// Verify pods are detected by ztunnel
						Eventually(func(g Gomega) {
							for _, pod := range pods.Items {
								// Check pod annotations indicate ambient mode
								g.Expect(pod.Annotations).To(HaveKey("ambient.istio.io/redirection"))
							}
						}).Should(Succeed())
						Success("Workloads registered with ztunnel")
					})

					It("verifies L4 connectivity in ambient mode", func(ctx SpecContext) {
						pods := &corev1.PodList{}
						Expect(cl.List(ctx, pods, client.InNamespace(workloadNamespace), client.MatchingLabels{"app": "sleep"})).To(Succeed())
						Expect(pods.Items).To(HaveLen(1))

						clientPod := pods.Items[0].Name
						Eventually(func() error {
							// Test connectivity to httpbin service (should go through ztunnel)
							targetURL := fmt.Sprintf("httpbin.%s.svc.cluster.local:8000/get", workloadNamespace)
							return common.CheckHTTPConnectivity(k, workloadNamespace, clientPod, "sleep", targetURL, 5, "200")
						}).Should(Succeed())
						Success("L4 connectivity verified in ambient mode via ztunnel")
					})
				})

				When("waypoint is deployed for L7 processing", func() {
					It("deploys waypoint Gateway", func(ctx SpecContext) {
						waypointYAML := fmt.Sprintf(`
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: waypoint
  namespace: %s
  labels:
    istio.io/waypoint-for: service
spec:
  gatewayClassName: istio-waypoint
  listeners:
  - name: mesh
    port: 15008
    protocol: HBONE`, workloadNamespace)

						Expect(k.CreateFromString(waypointYAML)).To(Succeed())
						Success("Waypoint Gateway created")
					})

					It("waits for waypoint deployment to be ready", func(ctx SpecContext) {
						Eventually(func(g Gomega) {
							deployment := &appsv1.Deployment{}
							g.Expect(cl.Get(ctx, kube.Key("waypoint", workloadNamespace), deployment)).To(Succeed())
							g.Expect(deployment.Status.ReadyReplicas).To(BeNumerically(">", 0))
						}).Should(Succeed())
						Success("Waypoint deployment is ready")
					})

					It("updates AuthorizationPolicy to target waypoint", func(ctx SpecContext) {
						// Delete the old sidecar-mode AuthorizationPolicy
						Expect(k.WithNamespace(workloadNamespace).Delete("authorizationpolicy", "httpbin-authz")).To(Succeed())

						// Create new AuthorizationPolicy targeting waypoint
						authzPolicyYAML := fmt.Sprintf(`
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: httpbin-authz
  namespace: %s
spec:
  targetRefs:
  - kind: Gateway
    group: gateway.networking.k8s.io
    name: waypoint
  action: DENY
  rules:
  - to:
    - operation:
        paths: ["/deny"]`, workloadNamespace)

						Expect(k.CreateFromString(authzPolicyYAML)).To(Succeed())
						Success("AuthorizationPolicy updated to target waypoint")
					})

					It("verifies waypoint is not yet processing traffic (dormant)", func(ctx SpecContext) {
						ns := &corev1.Namespace{}
						Expect(cl.Get(ctx, kube.Key(workloadNamespace), ns)).To(Succeed())
						Expect(ns.Labels).NotTo(HaveKey("istio.io/use-waypoint"))
						Success("Waypoint is dormant (not activated yet)")
					})
				})

				When("waypoint is activated for L7 processing", func() {
					It("labels namespace to activate waypoint", func(ctx SpecContext) {
						Expect(k.Label("namespace", workloadNamespace, "istio.io/use-waypoint", "waypoint")).To(Succeed())
						Success("Waypoint activated")
					})

					It("verifies VirtualService routing works via waypoint", func(ctx SpecContext) {
						pods := &corev1.PodList{}
						Expect(cl.List(ctx, pods, client.InNamespace(workloadNamespace),
							client.MatchingLabels{"app": "sleep"})).To(Succeed())
						sleepPod := pods.Items[0].Name

						// VirtualService should continue to work (routes traffic through waypoint now)
						Eventually(func() error {
							return common.CheckHTTPConnectivity(k, workloadNamespace, sleepPod, "sleep", "httpbin:8000/get", 5, "200")
						}).Should(Succeed())
						Success("VirtualService routing verified via waypoint")
					})

					It("verifies L7 AuthorizationPolicy is enforced via waypoint", func(ctx SpecContext) {
						pods := &corev1.PodList{}
						Expect(cl.List(ctx, pods, client.InNamespace(workloadNamespace),
							client.MatchingLabels{"app": "sleep"})).To(Succeed())
						sleepPod := pods.Items[0].Name

						// Test allowed path
						Eventually(func() error {
							return common.CheckHTTPConnectivity(k, workloadNamespace, sleepPod, "sleep", "httpbin:8000/get", 5, "200")
						}).Should(Succeed())

						// Test denied path (L7 policy enforcement)
						Eventually(func() error {
							return common.CheckHTTPConnectivity(k, workloadNamespace, sleepPod, "sleep", "httpbin:8000/deny", 5, "403")
						}).Should(Succeed())

						Success("L7 AuthorizationPolicy successfully enforced via waypoint")
					})
				})
			})
		})
	})
