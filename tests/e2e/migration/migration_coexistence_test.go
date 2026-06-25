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
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// expectedPodRestartError is the expected HTTP 503 error during pod restart in migration
	expectedPodRestartError = "unexpected status code: 503"
)

var _ = Describe("Migration Sidecar-Ambient Coexistence Validation", Ordered,
	Label("migration", "migration-coexistence", "slow"), func() {
		// Tests sidecar-ambient coexistence during migration with continuous traffic.
		// Uses a stable external client (sidecar mode, never migrates) to send traffic
		// to a server that transitions from sidecar → HBONE → ambient while traffic flows.
		// This validates cross-mode communication and verifies traffic resilience during migration.
		// Validates:
		//   1. Cross-mode communication works (sidecar client → ambient server)
		//   2. Limited disruption during pod restart (≤10 503 errors - brief, acceptable)
		//   3. Traffic RECOVERS after disruption (≥9/10 consecutive successes post-migration)
		SetDefaultEventuallyTimeout(time.Duration(defaultTimeout) * time.Second)
		SetDefaultEventuallyPollingInterval(time.Second)

		version := istioversion.GetLatestAmbientVersion()

		Context(fmt.Sprintf("Migration testing with Istio %s", version.Version), func() {
			var clr cleaner.Cleaner
			workloadNamespace := "workload-traffic"
			stableClientNamespace := "stable-client"

			BeforeAll(func(ctx SpecContext) {
				clr = cleaner.New(cl)
				clr.Record(ctx)

				Expect(k.CreateNamespace(controlPlaneNamespace)).To(Succeed())
				Expect(k.CreateNamespace(istioCniNamespace)).To(Succeed())
				Expect(k.CreateNamespace(ztunnelNamespace)).To(Succeed())

				common.CreateIstioCNI(k, version.Name)
				common.AwaitCondition(ctx, v1.IstioCNIConditionReady, kube.Key(istioCniName), &v1.IstioCNI{}, k, cl)
				Success("IstioCNI created (no profile - sidecar mode)")

				common.CreateIstio(k, version.Name)
				common.AwaitCondition(ctx, v1.IstioConditionReady, kube.Key(istioName), &v1.Istio{}, k, cl)
				Success("Istio created (default profile - sidecar mode)")

				// Wait for sidecar injector webhook to be ready
				Eventually(func(g Gomega) {
					webhook := &admissionregistrationv1.MutatingWebhookConfiguration{}
					g.Expect(cl.Get(ctx, kube.Key("istio-sidecar-injector"), webhook)).To(Succeed())
				}).Should(Succeed())
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

			Context("Sidecar-Ambient Coexistence with Continuous Traffic", Ordered, func() {
				BeforeAll(func(ctx SpecContext) {
					// Create and deploy stable client (never migrates, always stays in sidecar mode)
					Expect(k.CreateNamespace(stableClientNamespace)).To(Succeed())
					Expect(k.Label("namespace", stableClientNamespace, "istio-injection", "enabled")).To(Succeed())
					Expect(k.WithNamespace(stableClientNamespace).ApplyKustomize("sleep")).To(Succeed())
					Success("Stable sleep client deployed with sidecar injection")
					Eventually(func(g Gomega) {
						pods := &corev1.PodList{}
						g.Expect(cl.List(ctx, pods, client.InNamespace(stableClientNamespace))).To(Succeed())
						g.Expect(pods.Items).To(HaveLen(1))
						pod := pods.Items[0]
						g.Expect(pod.Status.Phase).To(Equal(corev1.PodRunning))
						for _, containerStatus := range pod.Status.ContainerStatuses {
							g.Expect(containerStatus.Ready).To(BeTrue(),
								"Container %s should be ready", containerStatus.Name)
						}
						g.Expect(common.HasSidecarInjected(pod)).To(BeTrue(),
							"Stable client pod should have sidecar injected")
					}).Should(Succeed())
					Success("Stable client ready with sidecar")
				})
				When("server workloads are deployed in sidecar mode", func() {
					It("creates and labels namespace for sidecar injection", func(ctx SpecContext) {
						Expect(k.CreateNamespace(workloadNamespace)).To(Succeed())
						Expect(k.Label("namespace", workloadNamespace, "istio-injection", "enabled")).To(Succeed())
						Success(fmt.Sprintf("Namespace %s created and labeled for sidecar injection", workloadNamespace))
					})
					It("deploys httpbin server in sidecar mode", func(ctx SpecContext) {
						// Only deploy httpbin - client is in stable-client namespace
						Expect(k.WithNamespace(workloadNamespace).ApplyKustomize("httpbin")).To(Succeed())
						Success("Httpbin server deployed in sidecar mode")
					})
					It("waits for httpbin pod to be ready with sidecar", func(ctx SpecContext) {
						Eventually(func(g Gomega) {
							pods := &corev1.PodList{}
							g.Expect(cl.List(ctx, pods, client.InNamespace(workloadNamespace))).To(Succeed())
							g.Expect(pods.Items).To(HaveLen(1), "Expected 1 pod (httpbin)")
							pod := pods.Items[0]
							g.Expect(pod.Status.Phase).To(Equal(corev1.PodRunning))
							g.Expect(common.HasSidecarInjected(pod)).To(BeTrue(),
								"Pod %s should have istio-proxy in either containers or init containers", pod.Name)
						}).Should(Succeed())
						Success("Httpbin running with sidecar")
					})
					It("verifies initial traffic works from stable client", func(ctx SpecContext) {
						// Get stable client pod (never migrated)
						pods := &corev1.PodList{}
						Expect(cl.List(ctx, pods, client.InNamespace(stableClientNamespace), client.MatchingLabels{"app": "sleep"})).To(Succeed())
						Expect(pods.Items).To(HaveLen(1))
						clientPod := pods.Items[0].Name
						GinkgoWriter.Printf("Using stable client: %s (in namespace %s)\n", clientPod, stableClientNamespace)
						Eventually(func() error {
							// Stable client (sidecar mode) → httpbin (sidecar mode)
							targetURL := fmt.Sprintf("httpbin.%s.svc.cluster.local:8000/get", workloadNamespace)
							return common.CheckHTTPConnectivity(k, stableClientNamespace, clientPod, "sleep", targetURL, 5, "200")
						}).Should(Succeed())
						Success("Traffic flows from stable client to httpbin (both sidecar mode)")
					})
				})
				When("namespace is switched to ambient mode during traffic", func() {
					var (
						stats      *common.HTTPTrafficStats
						cancelFunc context.CancelFunc
					)
					// Ensure continuous traffic is stopped when this When block completes
					AfterAll(func() {
						if cancelFunc != nil {
							GinkgoWriter.Printf("Cleaning up: stopping any remaining continuous traffic...\n")
							cancelFunc()
							time.Sleep(500 * time.Millisecond)
						}
					})
					It("starts continuous traffic from stable client", func(ctx SpecContext) {
						pods := &corev1.PodList{}
						Expect(cl.List(ctx, pods, client.InNamespace(stableClientNamespace), client.MatchingLabels{"app": "sleep"})).To(Succeed())
						Expect(pods.Items).To(HaveLen(1))
						clientPod := pods.Items[0].Name
						GinkgoWriter.Printf("Using stable client pod: %s (in namespace %s)\n", clientPod, stableClientNamespace)
						// Start continuous traffic from stable client → httpbin server
						// Client stays in sidecar mode, server will migrate to ambient
						// Background context is used so traffic continues
						// across multiple It blocks until we explicitly stop it
						targetService := fmt.Sprintf("httpbin.%s.svc.cluster.local:8000/get", workloadNamespace)
						stats, cancelFunc = common.StartContinuousHTTPTraffic(
							context.Background(), k, stableClientNamespace, clientPod, "sleep",
							targetService, 100*time.Millisecond, nil)
						// Wait for baseline traffic to establish (use Eventually to handle slower OCP clusters)
						GinkgoWriter.Printf("Waiting for baseline traffic to establish (target: >=5 successful requests)...\n")
						var total, success, failed int64
						var errors []string
						Eventually(func(g Gomega) {
							total, success, failed, errors = stats.GetStats()
							GinkgoWriter.Printf("[Baseline check] %d total, %d success, %d failed\n", total, success, failed)
							g.Expect(success).To(BeNumerically(">=", 5), "Should have at least 5 successful requests for baseline")
						}).WithTimeout(30 * time.Second).WithPolling(500 * time.Millisecond).Should(Succeed())
						total, success, failed, errors = stats.GetStats()
						GinkgoWriter.Printf("Baseline traffic established: %d total, %d success, %d failed\n", total, success, failed)
						if failed > 0 && len(errors) > 0 {
							GinkgoWriter.Printf("Baseline errors (showing first 5):\n")
							for i, errMsg := range errors {
								if i >= 5 {
									break
								}
								GinkgoWriter.Printf("  [%d] %s\n", i+1, errMsg)
							}
						}
						Success("Continuous traffic started from stable client")
					})
					It("migrates Istio to ambient profile while traffic is running", func(ctx SpecContext) {
						GinkgoWriter.Printf("Updating Istio to ambient profile...\n")
						patch := fmt.Sprintf(`{"spec":{"profile":"ambient","values":{"pilot":{"trustedZtunnelNamespace":"%s"}}}}`, ztunnelNamespace)
						Expect(k.Patch("istio", istioName, "merge", patch)).To(Succeed())
						common.AwaitCondition(ctx, v1.IstioConditionReady, kube.Key(istioName), &v1.Istio{}, k, cl)
						Success("Istio updated to ambient profile")
					})
					It("migrates IstioCNI to ambient profile while traffic is running", func(ctx SpecContext) {
						GinkgoWriter.Printf("Updating IstioCNI to ambient profile...\n")
						Expect(k.Patch("istiocni", istioCniName, "merge", `{"spec":{"profile":"ambient"}}`)).To(Succeed())
						common.AwaitCondition(ctx, v1.IstioCNIConditionReady, kube.Key(istioCniName), &v1.IstioCNI{}, k, cl)
						Success("IstioCNI updated to ambient profile")
					})
					It("deploys ZTunnel while traffic is running", func(ctx SpecContext) {
						GinkgoWriter.Printf("Deploying ZTunnel...\n")
						common.CreateZTunnel(k, version.Name)
						common.AwaitCondition(ctx, v1.ZTunnelConditionReady, kube.Key("default"), &v1.ZTunnel{}, k, cl)
						Success("ZTunnel deployed and ready")
					})
					It("updates server namespace to revision-based injection for HBONE-aware sidecars", func(ctx SpecContext) {
						GinkgoWriter.Printf("Waiting for sidecar injector webhook to be recreated...\n")
						Eventually(func(g Gomega) {
							webhook := &admissionregistrationv1.MutatingWebhookConfiguration{}
							g.Expect(cl.Get(ctx, kube.Key("istio-sidecar-injector"), webhook)).To(Succeed())
						}).Should(Succeed())
						Success("Sidecar injector webhook is ready")
						// Update namespace to use revision-based injection (HBONE-aware sidecars)
						ns := &corev1.Namespace{}
						Expect(cl.Get(ctx, kube.Key(workloadNamespace), ns)).To(Succeed())
						delete(ns.Labels, "istio-injection")
						ns.Labels["istio.io/rev"] = "default"
						Expect(cl.Update(ctx, ns)).To(Succeed())
						Success("Namespace updated to revision-based injection for HBONE-aware sidecars")
					})
					It("restarts httpbin to get HBONE-aware sidecars while traffic is running", func(ctx SpecContext) {
						GinkgoWriter.Printf("Restarting httpbin to get HBONE-aware sidecars...\n")
						_, err := k.WithNamespace(workloadNamespace).RolloutRestart("deployment/httpbin")
						Expect(err).NotTo(HaveOccurred())
						Eventually(func(g Gomega) {
							_, err := k.WithNamespace(workloadNamespace).RolloutStatus("deployment/httpbin")
							g.Expect(err).NotTo(HaveOccurred())
						}).Should(Succeed())
						Success("Httpbin restarted with HBONE-aware sidecars")
					})
					It("removes revision label and adds ambient label while traffic is running", func(ctx SpecContext) {
						// Remove revision-based injection and add ambient label
						// This is the final step to pure ambient mode
						ns := &corev1.Namespace{}
						Expect(cl.Get(ctx, kube.Key(workloadNamespace), ns)).To(Succeed())
						delete(ns.Labels, "istio.io/rev")
						ns.Labels["istio.io/dataplane-mode"] = "ambient"
						Expect(cl.Update(ctx, ns)).To(Succeed())
						Success("Namespace switched to ambient mode (client stays in sidecar mode, traffic still running)")
					})
					It("restarts httpbin server to apply pure ambient mode while traffic is running", func(ctx SpecContext) {
						GinkgoWriter.Printf("Restarting httpbin server (stable client stays running)...\n")
						// Only restart httpbin server - stable client pod stays running in different namespace
						_, err := k.WithNamespace(workloadNamespace).RolloutRestart("deployment/httpbin")
						Expect(err).NotTo(HaveOccurred(), "Failed to restart httpbin")
						Eventually(func(g Gomega) {
							_, err := k.WithNamespace(workloadNamespace).RolloutStatus("deployment/httpbin")
							g.Expect(err).NotTo(HaveOccurred())
						}).Should(Succeed())
						Success("Httpbin restarted in ambient mode (client never restarted, traffic continuous)")
					})
					It("waits for httpbin server to be ready in ambient mode", func(ctx SpecContext) {
						Eventually(func(g Gomega) {
							pods := &corev1.PodList{}
							g.Expect(cl.List(ctx, pods, client.InNamespace(workloadNamespace))).To(Succeed())
							g.Expect(pods.Items).To(HaveLen(1), "Expected only httpbin pod in server namespace")
							pod := pods.Items[0]
							g.Expect(pod.Status.Phase).To(Equal(corev1.PodRunning))
							g.Expect(pod.Spec.Containers).To(HaveLen(1), "Httpbin should not have sidecar in ambient mode")
							g.Expect(pod.Annotations).To(HaveKey("ambient.istio.io/redirection"))
						}).Should(Succeed())
						Success("Httpbin server running in ambient mode without sidecar")
						// Verify stable client still has sidecar (never migrated)
						Eventually(func(g Gomega) {
							pods := &corev1.PodList{}
							g.Expect(cl.List(ctx, pods, client.InNamespace(stableClientNamespace))).To(Succeed())
							g.Expect(pods.Items).To(HaveLen(1))
							clientPod := pods.Items[0]
							g.Expect(clientPod.Status.Phase).To(Equal(corev1.PodRunning))
							g.Expect(common.HasSidecarInjected(clientPod)).To(BeTrue(), "Stable client should still have sidecar")
						}).Should(Succeed())
						Success("Stable client still running in sidecar mode (never migrated)")
					})
					It("verifies traffic recovers after brief disruption during migration", func(ctx SpecContext) {
						// Key insight: During pod restart, we expect:
						// 1. Some 503 errors (pod terminating/starting) - ACCEPTABLE
						// 2. Then traffic RESUMES successfully - CRITICAL
						// 3. Sustained success after recovery - PROVES STABILITY
						GinkgoWriter.Printf("Letting continuous traffic run for 5 more seconds during ambient mode...\n")
						time.Sleep(5 * time.Second)
						GinkgoWriter.Printf("Stopping continuous traffic and analyzing recovery pattern...\n")
						if cancelFunc != nil {
							cancelFunc()
						}
						time.Sleep(500 * time.Millisecond) // Allow final requests to complete
						// Analyze the traffic pattern
						total, success, failed, errors := stats.GetStats()
						GinkgoWriter.Printf("Total traffic: %d requests, %d success, %d failed\n", total, success, failed)
						// Ensure we actually sent meaningful traffic
						Expect(total).To(BeNumerically(">=", 10), "Should have sent at least 10 requests during test")
						// Verify 503 errors are bounded (not unlimited failures)
						count503 := 0
						for _, err := range errors {
							if err == expectedPodRestartError {
								count503++
							}
						}
						GinkgoWriter.Printf("503 errors during pod restart: %d\n", count503)
						if count503 > 0 {
							GinkgoWriter.Printf("First few 503 errors (expected during pod restart):\n")
							shown := 0
							for _, err := range errors {
								if err == expectedPodRestartError && shown < 3 {
									GinkgoWriter.Printf("  - %s\n", err)
									shown++
								}
							}
						}
						// Assert: 503 errors should be limited (pod restart is brief)
						// Allow up to 10 503s (generous for slow OCP pod restart)
						Expect(count503).To(BeNumerically("<=", 10),
							"503 errors should be limited to pod restart window (≤10), got %d", count503)
						// Critical assertion: Verify traffic RECOVERED after 503s
						// Look for consecutive successes AFTER the disruption
						pods := &corev1.PodList{}
						Expect(cl.List(ctx, pods, client.InNamespace(stableClientNamespace), client.MatchingLabels{"app": "sleep"})).To(Succeed())
						Expect(pods.Items).To(HaveLen(1))
						clientPod := pods.Items[0].Name
						targetService := fmt.Sprintf("httpbin.%s.svc.cluster.local:8000/get", workloadNamespace)
						GinkgoWriter.Printf("Verifying sustained recovery (10 consecutive requests)...\n")
						// Verify sustained recovery using MustPassRepeatedly
						// All 10 attempts must succeed (200ms interval between attempts)
						Eventually(func() error {
							return common.CheckHTTPConnectivity(
								k, stableClientNamespace, clientPod, "sleep", targetService, 5, "200")
						}).WithPolling(200*time.Millisecond).MustPassRepeatedly(10).Should(Succeed(),
							"Traffic should be stable after recovery (all 10 consecutive requests must succeed)")
						// Summary
						GinkgoWriter.Printf("\n=== Migration Traffic Pattern Summary ===\n")
						GinkgoWriter.Printf("Total requests during migration: %d\n", total)
						GinkgoWriter.Printf("503 errors (pod restart): %d (≤10 acceptable)\n", count503)
						GinkgoWriter.Printf("Recovery stability: 10/10 consecutive successes (verified with MustPassRepeatedly)\n")
						GinkgoWriter.Printf("========================================\n")
						Success(fmt.Sprintf("Traffic recovered successfully after %d 503 errors during pod restart", count503))
					})
					It("verifies cross-mode communication works", func(ctx SpecContext) {
						// Final verification: stable client (sidecar) can communicate with httpbin (ambient)
						pods := &corev1.PodList{}
						Expect(cl.List(ctx, pods, client.InNamespace(stableClientNamespace), client.MatchingLabels{"app": "sleep"})).To(Succeed())
						Expect(pods.Items).To(HaveLen(1))
						clientPod := pods.Items[0].Name
						Eventually(func() error {
							targetURL := fmt.Sprintf("httpbin.%s.svc.cluster.local:8000/get", workloadNamespace)
							return common.CheckHTTPConnectivity(k, stableClientNamespace, clientPod, "sleep", targetURL, 5, "200")
						}).Should(Succeed())
						Success("Cross-mode communication verified: sidecar client → ambient server")
					})
				})
			})
		})
	})
