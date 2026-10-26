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
	"context"
	"fmt"
	"strings"
	"time"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/env"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/cleaner"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	. "github.com/istio-ecosystem/sail-operator/tests/e2e/util/gomega"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Control Plane updates", Label("control-plane", "update", "slow", "sidecar"), Ordered, func() {
	SetDefaultEventuallyTimeout(time.Duration(env.GetInt("DEFAULT_TEST_TIMEOUT", 180)) * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)

	BeforeAll(func() {
		// Update suites need two consecutive minors and extra memory; if tests set E2E_VERSIONS_LIMIT=1
		// we will skip the update tests to avoid failures in resource-constrained CI.
		if env.Get("E2E_VERSIONS_LIMIT", "") == "1" {
			Skip("Skipping control plane update tests when E2E_VERSIONS_LIMIT=1")
		}
	})

	Describe("using IstioRevisionTag", func() {
		var baseVersion, newVersion istioversion.VersionInfo

		BeforeAll(func() {
			var err error
			baseVersion, newVersion, err = istioversion.GetTwoConsecutiveMinorVersions(istioversion.Sidecar)
			if err != nil {
				Skip(fmt.Sprintf("Skipping update tests: %v", err))
			}
		})

		Context(fmt.Sprintf("updating from %s to %s", baseVersion.Name, newVersion.Name), func() {
			clr := cleaner.New(cl)

			BeforeAll(func(ctx SpecContext) {
				clr.Record(ctx)
				Expect(k.CreateNamespace(controlPlaneNamespace)).To(Succeed(), "Istio namespace failed to be created")
				Expect(k.CreateNamespace(istioCniNamespace)).To(Succeed(), "IstioCNI namespace failed to be created")

				common.CreateIstioCNI(k, baseVersion.Name)
				common.AwaitCondition(ctx, v1.IstioCNIConditionReady, kube.Key(istioCniName), &v1.IstioCNI{}, k, cl)
			})

			// Capture debug info immediately on test failure
			JustAfterEach(func(ctx SpecContext) {
				if CurrentSpecReport().Failed() {
					common.LogDebugInfo(common.ControlPlane, k)
				}
			})

			When(fmt.Sprintf("the Istio CR is created with RevisionBased updateStrategy for base version %s", baseVersion.Name), func() {
				BeforeAll(func() {
					common.CreateIstio(k, baseVersion.Name, `
updateStrategy:
  type: RevisionBased
  inactiveRevisionDeletionGracePeriodSeconds: 30`)
				})

				It("deploys istiod and pod is Ready", func(ctx SpecContext) {
					common.AwaitCondition(ctx, v1.IstioConditionReady, kube.Key("default"), &v1.Istio{}, k, cl)
				})
			})

			When("the IstioRevisionTag resource is created", func() {
				BeforeAll(func() {
					IstioRevisionTagYAML := `
apiVersion: sailoperator.io/v1
kind: IstioRevisionTag
metadata:
  name: default
spec:
  targetRef:
    kind: Istio
    name: default`
					Log("IstioRevisionTag YAML:", common.Indent(IstioRevisionTagYAML))
					Expect(k.CreateFromString(IstioRevisionTagYAML)).
						To(Succeed(), "IstioRevisionTag CR failed to be created")
					Success("IstioRevisionTag CR created")
				})

				It("creates the resource with condition InUse false", func(ctx SpecContext) {
					// Condition InUse is expected to be false because there are no pods using the IstioRevisionTag
					Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key("default"), &v1.IstioRevisionTag{}).
						Should(HaveConditionStatus(v1.IstioRevisionTagConditionInUse, metav1.ConditionFalse), "unexpected Condition; expected InUse False")
					Success("IstioRevisionTag created and not in use")
				})

				It("IstioRevisionTag revision name is equal to the IstioRevision base name", func(ctx SpecContext) {
					revisionName := strings.Replace(baseVersion.Name, ".", "-", -1)
					Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key("default"), &v1.IstioRevisionTag{}).
						Should(HaveField("Status.IstioRevision", ContainSubstring(revisionName)),
							"IstioRevisionTag version does not match the IstioRevision name of the base version")
					Success("IstioRevisionTag version matches the Istio version")
				})

				It("istiod-default-validator VWC has failurePolicy Fail", func(ctx SpecContext) {
					Eventually(func(g Gomega) {
						vwc := &admissionv1.ValidatingWebhookConfiguration{}
						g.Expect(cl.Get(ctx, kube.Key("istiod-default-validator"), vwc)).To(Succeed())
						g.Expect(vwc.Webhooks).NotTo(BeEmpty())
						g.Expect(vwc.Webhooks[0].FailurePolicy).To(HaveValue(Equal(admissionv1.Fail)),
							"failurePolicy must be Fail before testing validation; Ignore would make subsequent tests meaningless")
					}).Should(Succeed())
					Success("VWC failurePolicy is Fail — webhook is active")
				})

				It("istiod-default-validator VWC accepts valid Istio resources", func() {
					peerAuthYAML := fmt.Sprintf(`
apiVersion: security.istio.io/v1
kind: PeerAuthentication
metadata:
  name: vwc-test
  namespace: %s
spec:
  mtls:
    mode: STRICT`, controlPlaneNamespace)
					Expect(k.CreateFromString(peerAuthYAML)).To(Succeed(),
						"VWC should allow creating valid Istio resources; if this fails, the istiod-default-validator may point to the wrong service")
					DeferCleanup(func() {
						if err := k.Delete("peerauthentication", "vwc-test"); err != nil {
							Log(fmt.Sprintf("Failed to delete PeerAuthentication: %s", err))
						}
					})
					Success("Valid Istio resource accepted by the default-validator VWC")
				})

				It("istiod-default-validator VWC rejects invalid Istio resources", func() {
					invalidDRYAML := fmt.Sprintf(`
apiVersion: networking.istio.io/v1
kind: DestinationRule
metadata:
  name: vwc-test-invalid
  namespace: %s
spec:
  host: ""`, controlPlaneNamespace)
					Expect(k.CreateFromString(invalidDRYAML)).ToNot(Succeed(),
						"VWC should reject invalid Istio resources; if this passes, the webhook may not be validating")
					Success("Invalid Istio resource correctly rejected by the default-validator VWC")
				})
			})

			When("sample pod is deployed", func() {
				BeforeAll(func(ctx SpecContext) {
					Expect(k.CreateNamespace(sampleNamespace)).To(Succeed(), "Sample namespace failed to be created")
					Expect(k.Label("namespace", sampleNamespace, "istio-injection", "enabled")).To(Succeed(), "Error labeling sample namespace")
					// sleep and httpbin are the client/server pair used to measure traffic continuity
					// across the update. Both live in the sample namespace so that the restart step
					// below moves them to the new revision together; a workload left on the old
					// revision would keep it in use and it would never be pruned.
					Expect(k.WithNamespace(sampleNamespace).ApplyKustomize(common.SleepContainerName)).
						To(Succeed(), "Error deploying sleep client")
					Expect(k.WithNamespace(sampleNamespace).ApplyKustomize(common.HttpbinContainerName)).
						To(Succeed(), "Error deploying httpbin server")
					Success("sample deployed")

					samplePods := &corev1.PodList{}
					// CheckSamplePodsReady is satisfied by whatever pods exist at that moment, so wait
					// for both deployments to have created theirs first.
					Eventually(func(g Gomega) {
						g.Expect(cl.List(ctx, samplePods, client.InNamespace(sampleNamespace))).To(Succeed())
						g.Expect(samplePods.Items).To(HaveLen(2), "expected the sleep and httpbin pods")
					}).Should(Succeed(), "sample pods were not created")
					Eventually(common.CheckSamplePodsReady).WithArguments(ctx, cl).Should(Succeed(), "Error checking status of sample pods")
					Expect(cl.List(ctx, samplePods, client.InNamespace(sampleNamespace))).To(Succeed(), "Error getting the pods in sample namespace")

					Success("sample pods are ready")

					for _, pod := range samplePods.Items {
						podName := pod.Name
						Eventually(func(g Gomega) {
							sidecarVersion, err := common.GetProxyVersionFromPod(podName, sampleNamespace)
							g.Expect(err).NotTo(HaveOccurred(), "Error getting sidecar version")
							g.Expect(sidecarVersion).To(Equal(baseVersion.Version), "Sidecar Istio version does not match the expected version")
						}).Should(Succeed(), "Error verifying sidecar version for pod "+podName)
					}
					Success("Istio sidecar version matches the expected base Istio version")
				})

				It("IstioRevisionTag state change to inUse true", func(ctx SpecContext) {
					common.AwaitCondition(ctx, v1.IstioRevisionTagConditionInUse, kube.Key("default"), &v1.IstioRevisionTag{}, k, cl)
				})
			})

			When("the Istio CR is updated to the new Istio version", func() {
				var traffic *trafficMonitor

				BeforeAll(func(ctx SpecContext) {
					// Start traffic between the already-running workloads before the control plane is
					// touched, so the whole update happens with requests in flight. The workloads are
					// never restarted within this block, so every request that fails from here on is
					// attributable to the control plane update itself.
					traffic = startTrafficMonitor(ctx, sampleNamespace,
						fmt.Sprintf("httpbin.%s.svc.cluster.local:8000/get", sampleNamespace))
					Success("Continuous traffic established before the update")

					Expect(k.Patch("istio", "default", "merge", `{"spec":{"version":"`+newVersion.Name+`"}}`)).To(Succeed(), "Error updating Istio CR to new Istio version")
					Success("Istio CR updated")
				})

				AfterAll(func() {
					traffic.stop()
				})

				It("Istio resource has revisions in use equal to two", func(ctx SpecContext) {
					Eventually(func() bool {
						istioResource := &v1.Istio{}
						Expect(cl.Get(ctx, kube.Key("default"), istioResource)).To(Succeed())
						return istioResource.Status.Revisions.InUse == 2
					}).Should(BeTrue(), "Istio resource does not have two revisions in use")
					Success("Istio resource has two revisions in use")
				})

				It("two istiod pods are running", func(ctx SpecContext) {
					Eventually(func() bool {
						istiodPods := &corev1.PodList{}
						Expect(cl.List(ctx, istiodPods, client.InNamespace(controlPlaneNamespace), client.MatchingLabels{"app": "istiod"})).To(Succeed())
						for _, pod := range istiodPods.Items {
							if pod.Status.Phase != corev1.PodRunning {
								return false
							}
						}
						return true
					}).Should(BeTrue(), "At least one of the istiod pods is not running")
					Success("Istiod pods are Running")
				})

				It("there is one IstionRevision for each version", func(ctx SpecContext) {
					istioRevisions := &v1.IstioRevisionList{}
					Expect(cl.List(ctx, istioRevisions)).To(Succeed())
					Expect(istioRevisions.Items).To(HaveLen(2), "Unexpected number of IstioRevisionTags; expected 2")
					Expect(istioRevisions.Items).To(ContainElement(
						HaveField("Spec", HaveField("Version", ContainSubstring(baseVersion.Name)))),
						"Expected a revision with the base version")
					Expect(istioRevisions.Items).To(ContainElement(
						HaveField("Spec", HaveField("Version", ContainSubstring(newVersion.Name)))),
						"Expected a revision with the new version")
					Success("Two IstionRevision found")
				})

				It("both IstionRevision are in use", func(ctx SpecContext) {
					// Check that both IstioRevisionTags are in use. One is in use by the current proxies and the new because is being referenced by the tag
					istioRevisions := &v1.IstioRevisionList{}
					Expect(cl.List(ctx, istioRevisions)).To(Succeed())
					for _, revision := range istioRevisions.Items {
						Expect(revision).To(HaveConditionStatus(v1.IstioRevisionTagConditionInUse, metav1.ConditionTrue), "IstioRevisionTag is not in use")
					}
					Success("Both IstionRevision are in use")
				})

				It("proxy version on sample pods still is base version", func(ctx SpecContext) {
					samplePods := &corev1.PodList{}
					Expect(cl.List(ctx, samplePods, client.InNamespace(sampleNamespace))).To(Succeed())
					Expect(samplePods.Items).ToNot(BeEmpty(), "No pods found in sample namespace")

					for _, pod := range samplePods.Items {
						podName := pod.Name
						Eventually(func(g Gomega) {
							// Use GetProxyVersionFromPod to avoid XDS authentication issues
							// that can occur during control plane upgrades when two istiod pods
							// are running simultaneously on OCP.
							sidecarVersion, err := common.GetProxyVersionFromPod(podName, sampleNamespace)
							g.Expect(err).NotTo(HaveOccurred(), "Error getting sidecar version")
							g.Expect(sidecarVersion).To(Equal(baseVersion.Version))
						}).Should(Succeed(), "Sidecar Istio version does not match the expected version")
					}
					Success("Istio sidecar version matches the expected Istio version")
				})

				It("should not disrupt traffic between running workloads", func() {
					// Traffic has been running since before the version patch and the new revision is
					// fully rolled out by now, so the measured window covers the whole update. It ends
					// here rather than extending over the workload migration in the next block only
					// because of how the traffic is generated: the generator execs into a single fixed
					// pod name, sleep runs one replica and the next block deletes every pod at once,
					// so the client itself goes down. Covering the migration would need a multi-replica
					// client, a rolling restart, and a generator that re-resolves the pod per request.
					failures := traffic.stopAndGetFailures()

					// Proxies of already-running workloads stay attached to the old revision while the
					// new one is rolled out, so a RevisionBased control plane update must not drop a
					// single request.
					Expect(failures).To(BeEmpty(),
						"RevisionBased control plane update disrupted traffic between already-running workloads")
					Success("Traffic was uninterrupted throughout the RevisionBased update")
				})
			})

			When("sample pod are restarted", func() {
				BeforeAll(func(ctx SpecContext) {
					samplePods := &corev1.PodList{}
					Expect(cl.List(ctx, samplePods, client.InNamespace(sampleNamespace))).To(Succeed())
					Expect(samplePods.Items).ToNot(BeEmpty(), "No pods found in sample namespace")

					for _, pod := range samplePods.Items {
						Expect(cl.Delete(ctx, &pod)).To(Succeed())
					}

					Eventually(common.CheckSamplePodsReady).WithArguments(ctx, cl).Should(Succeed(), "Error checking status of sample pods")
					Success("sample pods restarted and are ready")
				})

				It("updates the proxy version to the new Istio version", func(ctx SpecContext) {
					Eventually(func() bool {
						samplePods := &corev1.PodList{}
						Expect(cl.List(ctx, samplePods, client.InNamespace(sampleNamespace))).To(Succeed())
						if len(samplePods.Items) == 0 {
							return false
						}

						for _, pod := range samplePods.Items {
							sidecarVersion, err := common.GetProxyVersionFromPod(pod.Name, sampleNamespace)
							if err != nil || !sidecarVersion.Equal(newVersion.Version) {
								return false
							}
						}
						return true
					}).Should(BeTrue(), "Sidecar Istio version does not match the expected version")
					Success("Istio sidecar version matches the expected new Istio version")
				})

				It("IstionRevision resource and old istiod pod is deleted", func(ctx SpecContext) {
					// The IstioRevisionTag is now in use by the new IstioRevision, so the old IstioRevision and the old istiod pod are deleted
					Eventually(func() bool {
						istioRevisions := &v1.IstioRevisionList{}
						Expect(cl.List(ctx, istioRevisions)).To(Succeed())
						if len(istioRevisions.Items) != 1 {
							return false
						}

						istiodPods := &corev1.PodList{}
						Expect(cl.List(ctx, istiodPods, client.InNamespace(controlPlaneNamespace), client.MatchingLabels{"app": "istiod"})).To(Succeed())
						return len(istiodPods.Items) == 1
					}).Should(BeTrue(), "IstionRevision or Istiod pods are not being deleted")
					Success("IstionRevision and istiod pods are being deleted")
				})

				It("IstioRevisionTag revision name is equal to the IstionRevision name of the new Istio version", func(ctx SpecContext) {
					revisionName := strings.Replace(newVersion.Name, ".", "-", -1)
					Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key("default"), &v1.IstioRevisionTag{}).
						Should(HaveField("Status.IstioRevision", ContainSubstring(revisionName)), "IstioRevisionTag version does not match the new IstioRevision name")
					Success("IstioRevisionTag points to the new IstioRevision")
				})
			})

			AfterAll(func(ctx SpecContext) {
				// Skip cleanup if test failed and keepOnFailure is set
				if CurrentSpecReport().Failed() && keepOnFailure {
					return
				}

				clr.Cleanup(ctx)
			})
		})
	})

	// Jumps from the oldest supported minor straight to the newest one in a single patch,
	// leaving out every minor in between.
	Describe("skipping intermediate versions", func() {
		oldestVersion, newestVersion, versionErr := istioversion.GetOldestAndNewestMinorVersions(istioversion.Sidecar)
		// The minor right below the newest one. If it is also the oldest supported minor, only two
		// minors qualify, so there is no intermediate version to skip over.
		previousVersion, _, previousVersionErr := istioversion.GetTwoConsecutiveMinorVersions(istioversion.Sidecar)

		BeforeAll(func() {
			if versionErr != nil {
				Skip(fmt.Sprintf("Skipping skip-version update tests: %v", versionErr))
			}
			if previousVersionErr == nil && previousVersion.Name == oldestVersion.Name {
				Skip(fmt.Sprintf("Skipping skip-version update tests: %s and %s are consecutive minors, nothing to skip over",
					oldestVersion.Name, newestVersion.Name))
			}
		})

		Context(fmt.Sprintf("updating from %s to %s in a single step", oldestVersion.Name, newestVersion.Name), func() {
			clr := cleaner.New(cl)

			BeforeAll(func(ctx SpecContext) {
				clr.Record(ctx)
				Expect(k.CreateNamespace(controlPlaneNamespace)).To(Succeed(), "Istio namespace failed to be created")
				Expect(k.CreateNamespace(istioCniNamespace)).To(Succeed(), "IstioCNI namespace failed to be created")

				common.CreateIstioCNI(k, oldestVersion.Name)
				common.AwaitCondition(ctx, v1.IstioCNIConditionReady, kube.Key(istioCniName), &v1.IstioCNI{}, k, cl)
			})

			// Capture debug info immediately on test failure
			JustAfterEach(func(ctx SpecContext) {
				if CurrentSpecReport().Failed() {
					common.LogDebugInfo(common.ControlPlane, k)
				}
			})

			When(fmt.Sprintf("the Istio CR is created with RevisionBased updateStrategy for the oldest version %s", oldestVersion.Name), func() {
				BeforeAll(func() {
					common.CreateIstio(k, oldestVersion.Name, `
updateStrategy:
  type: RevisionBased
  inactiveRevisionDeletionGracePeriodSeconds: 30`)

					// Workloads are injected via the istio-injection=enabled label, which resolves to
					// the "default" tag. With RevisionBased the revisions are named after the version,
					// so without this tag nothing would ever be injected.
					IstioRevisionTagYAML := `
apiVersion: sailoperator.io/v1
kind: IstioRevisionTag
metadata:
  name: default
spec:
  targetRef:
    kind: Istio
    name: default`
					Log("IstioRevisionTag YAML:", common.Indent(IstioRevisionTagYAML))
					Expect(k.CreateFromString(IstioRevisionTagYAML)).To(Succeed(), "IstioRevisionTag CR failed to be created")
				})

				It("deploys istiod and pod is Ready", func(ctx SpecContext) {
					common.AwaitCondition(ctx, v1.IstioConditionReady, kube.Key("default"), &v1.Istio{}, k, cl)
				})
			})

			When("sample pods are deployed", func() {
				BeforeAll(func(ctx SpecContext) {
					Expect(k.CreateNamespace(sampleNamespace)).To(Succeed(), "Sample namespace failed to be created")
					Expect(k.Label("namespace", sampleNamespace, "istio-injection", "enabled")).To(Succeed(), "Error labeling sample namespace")
					Expect(k.WithNamespace(sampleNamespace).ApplyKustomize(common.SleepContainerName)).
						To(Succeed(), "Error deploying sleep client")
					Expect(k.WithNamespace(sampleNamespace).ApplyKustomize(common.HttpbinContainerName)).
						To(Succeed(), "Error deploying httpbin server")
					Success("sample deployed")

					samplePods := &corev1.PodList{}
					Eventually(func(g Gomega) {
						g.Expect(cl.List(ctx, samplePods, client.InNamespace(sampleNamespace))).To(Succeed())
						g.Expect(samplePods.Items).To(HaveLen(2), "expected the sleep and httpbin pods")
					}).Should(Succeed(), "sample pods were not created")
					Eventually(common.CheckSamplePodsReady).WithArguments(ctx, cl).Should(Succeed(), "Error checking status of sample pods")
					Success("sample pods are ready")
				})

				It("injects sidecars of the oldest version", func(ctx SpecContext) {
					samplePods := &corev1.PodList{}
					Expect(cl.List(ctx, samplePods, client.InNamespace(sampleNamespace))).To(Succeed())
					Expect(samplePods.Items).ToNot(BeEmpty(), "No pods found in sample namespace")

					for _, pod := range samplePods.Items {
						podName := pod.Name
						Eventually(func(g Gomega) {
							sidecarVersion, err := common.GetProxyVersionFromPod(podName, sampleNamespace)
							g.Expect(err).NotTo(HaveOccurred(), "Error getting sidecar version")
							g.Expect(sidecarVersion).To(Equal(oldestVersion.Version))
						}).Should(Succeed(), "Error verifying sidecar version for pod "+podName)
					}
					Success("Istio sidecar version matches the oldest supported Istio version")
				})
			})

			When("the Istio CR is updated to the newest version in a single step", func() {
				var traffic *trafficMonitor

				BeforeAll(func(ctx SpecContext) {
					// Start traffic between the already-running workloads before the control plane is
					// touched, so the whole update happens with requests in flight.
					traffic = startTrafficMonitor(ctx, sampleNamespace,
						fmt.Sprintf("httpbin.%s.svc.cluster.local:8000/get", sampleNamespace))
					Success("Continuous traffic established before the update")

					Expect(k.Patch("istio", "default", "merge", `{"spec":{"version":"`+newestVersion.Name+`"}}`)).
						To(Succeed(), "Error updating Istio CR to the newest Istio version")
					Success("Istio CR updated")
				})

				AfterAll(func() {
					traffic.stop()
				})

				It("creates a second IstioRevision for the newest version", func(ctx SpecContext) {
					Eventually(func(g Gomega) {
						istioRevisions := &v1.IstioRevisionList{}
						g.Expect(cl.List(ctx, istioRevisions)).To(Succeed())
						g.Expect(istioRevisions.Items).To(HaveLen(2), "Unexpected number of IstioRevisions; expected 2")
						g.Expect(istioRevisions.Items).To(ContainElement(
							HaveField("Spec", HaveField("Version", ContainSubstring(oldestVersion.Name)))),
							"Expected a revision with the oldest version")
						g.Expect(istioRevisions.Items).To(ContainElement(
							HaveField("Spec", HaveField("Version", ContainSubstring(newestVersion.Name)))),
							"Expected a revision with the newest version")
					}).Should(Succeed())
					Success("Both IstioRevisions exist")
				})

				It("rolls out a second istiod and becomes Ready", func(ctx SpecContext) {
					// The istiod rollout has to be waited for explicitly. The Istio CR keeps
					// reporting Ready from the old revision until the controller reconciles the new
					// one, so asserting on that condition alone is satisfied by a stale status
					// before the new control plane exists. It would also close the traffic window
					// below while the update is still in flight.
					Eventually(func(g Gomega) {
						istiodPods := &corev1.PodList{}
						g.Expect(cl.List(ctx, istiodPods, client.InNamespace(controlPlaneNamespace), client.MatchingLabels{"app": "istiod"})).To(Succeed())
						g.Expect(istiodPods.Items).To(HaveLen(2), "expected one istiod pod per revision")
						for _, pod := range istiodPods.Items {
							g.Expect(pod.Status.Phase).To(Equal(corev1.PodRunning), "istiod pod "+pod.Name+" is not Running")
						}
					}).Should(Succeed(), "The istiod of the newest version did not roll out")

					Eventually(func(g Gomega) {
						istio := &v1.Istio{}
						g.Expect(cl.Get(ctx, kube.Key("default"), istio)).To(Succeed())
						g.Expect(istio.Status.Revisions.Ready).To(BeNumerically("==", 2), "Both revisions should be Ready")
					}).Should(Succeed())

					common.AwaitCondition(ctx, v1.IstioConditionReady, kube.Key("default"), &v1.Istio{}, k, cl)
					Success("Both control planes are running and the Istio CR is Ready")
				})

				It("keeps the proxies of running workloads on the oldest version", func(ctx SpecContext) {
					samplePods := &corev1.PodList{}
					Expect(cl.List(ctx, samplePods, client.InNamespace(sampleNamespace))).To(Succeed())
					Expect(samplePods.Items).ToNot(BeEmpty(), "No pods found in sample namespace")

					for _, pod := range samplePods.Items {
						podName := pod.Name
						Eventually(func(g Gomega) {
							sidecarVersion, err := common.GetProxyVersionFromPod(podName, sampleNamespace)
							g.Expect(err).NotTo(HaveOccurred(), "Error getting sidecar version")
							g.Expect(sidecarVersion).To(Equal(oldestVersion.Version))
						}).Should(Succeed(), "Sidecar Istio version does not match the expected version")
					}
					Success("Running workloads stayed on the old revision")
				})

				It("should not disrupt traffic between running workloads", func() {
					// Same measurement window as in the consecutive-version suite: from before the
					// version patch until the new revision is rolled out. The workloads are migrated
					// in the next block, which is not covered here because the traffic generator
					// execs into the single sleep pod that the migration deletes.
					failures := traffic.stopAndGetFailures()

					// Proxies of already-running workloads stay attached to the old revision while the
					// new one is rolled out, so how many minors the update spans must make no
					// difference to them.
					Expect(failures).To(BeEmpty(),
						"Skip-version control plane update disrupted traffic between already-running workloads")
					Success("Traffic was uninterrupted throughout the skip-version update")
				})
			})

			When("the sample pods are restarted", func() {
				BeforeAll(func(ctx SpecContext) {
					// The IstioCNI is deliberately still at the oldest version here: the workloads
					// are migrated to the new revision first and the CNI is updated afterwards.
					samplePods := &corev1.PodList{}
					Expect(cl.List(ctx, samplePods, client.InNamespace(sampleNamespace))).To(Succeed())
					Expect(samplePods.Items).ToNot(BeEmpty(), "No pods found in sample namespace")

					for _, pod := range samplePods.Items {
						Expect(cl.Delete(ctx, &pod)).To(Succeed())
					}

					Eventually(common.CheckSamplePodsReady).WithArguments(ctx, cl).Should(Succeed(), "Error checking status of sample pods")
					Success("sample pods restarted and are ready")
				})

				It("updates the proxy version to the newest Istio version", func(ctx SpecContext) {
					Eventually(func(g Gomega) {
						samplePods := &corev1.PodList{}
						g.Expect(cl.List(ctx, samplePods, client.InNamespace(sampleNamespace))).To(Succeed())
						g.Expect(samplePods.Items).ToNot(BeEmpty())

						for _, pod := range samplePods.Items {
							sidecarVersion, err := common.GetProxyVersionFromPod(pod.Name, sampleNamespace)
							g.Expect(err).NotTo(HaveOccurred(), "Error getting sidecar version")
							g.Expect(sidecarVersion).To(Equal(newestVersion.Version))
						}
					}).Should(Succeed(), "Sidecar Istio version does not match the expected version")
					Success("Istio sidecar version matches the newest Istio version")
				})

				It("prunes the IstioRevision and istiod pod of the oldest version", func(ctx SpecContext) {
					Eventually(func(g Gomega) {
						istioRevisions := &v1.IstioRevisionList{}
						g.Expect(cl.List(ctx, istioRevisions)).To(Succeed())
						g.Expect(istioRevisions.Items).To(HaveLen(1), "The old IstioRevision was not pruned")
						g.Expect(istioRevisions.Items[0].Spec.Version).To(Equal(newestVersion.Name))

						istiodPods := &corev1.PodList{}
						g.Expect(cl.List(ctx, istiodPods, client.InNamespace(controlPlaneNamespace), client.MatchingLabels{"app": "istiod"})).To(Succeed())
						g.Expect(istiodPods.Items).To(HaveLen(1), "The old istiod pod was not removed")
					}).Should(Succeed())
					Success("Old IstioRevision and istiod pod were pruned")
				})

				It("points the IstioRevisionTag at the IstioRevision of the newest version", func(ctx SpecContext) {
					revisionName := strings.Replace(newestVersion.Name, ".", "-", -1)
					Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key("default"), &v1.IstioRevisionTag{}).
						Should(HaveField("Status.IstioRevision", ContainSubstring(revisionName)),
							"IstioRevisionTag does not point to the IstioRevision of the newest version")
					Success("IstioRevisionTag points to the new IstioRevision")
				})
			})

			When("the IstioCNI is updated to the newest version", func() {
				var traffic *trafficMonitor

				BeforeAll(func(ctx SpecContext) {
					// Traffic runs from the restarted sleep pod, so it exercises the workloads that are
					// already on the new revision while the CNI DaemonSet is replaced underneath them.
					traffic = startTrafficMonitor(ctx, sampleNamespace,
						fmt.Sprintf("httpbin.%s.svc.cluster.local:8000/get", sampleNamespace))
					Success("Continuous traffic established before the IstioCNI update")

					// The CNI is updated last, after the control plane and after the workloads have
					// been moved to the new revision. A CNI at version 1.x supports a control plane
					// at 1.x and 1.x+1, so it keeps setting up the traffic redirection for the
					// newly injected proxies while it still runs the old version.
					Expect(k.Patch("istiocni", istioCniName, "merge", `{"spec":{"version":"`+newestVersion.Name+`"}}`)).
						To(Succeed(), "Error updating IstioCNI CR to the newest Istio version")
				})

				AfterAll(func() {
					traffic.stop()
				})

				It("becomes Ready at the newest version", func(ctx SpecContext) {
					Eventually(func(g Gomega) {
						cni := &v1.IstioCNI{}
						g.Expect(cl.Get(ctx, kube.Key(istioCniName), cni)).To(Succeed())
						g.Expect(cni.Spec.Version).To(Equal(newestVersion.Name))
						g.Expect(cni).To(HaveConditionStatus(v1.IstioCNIConditionReady, metav1.ConditionTrue))
					}).Should(Succeed(), "IstioCNI did not become Ready at the newest version")
					Success("IstioCNI updated to the newest Istio version")
				})

				It("rolls out the istio-cni-node DaemonSet", func(ctx SpecContext) {
					Eventually(func(g Gomega) {
						ds := &appsv1.DaemonSet{}
						g.Expect(cl.Get(ctx, kube.Key("istio-cni-node", istioCniNamespace), ds)).To(Succeed())
						g.Expect(ds.Status.DesiredNumberScheduled).To(BeNumerically(">", 0))
						g.Expect(ds.Status.UpdatedNumberScheduled).To(Equal(ds.Status.DesiredNumberScheduled))
						g.Expect(ds.Status.NumberAvailable).To(Equal(ds.Status.DesiredNumberScheduled))
					}).Should(Succeed(), "istio-cni-node DaemonSet was not fully rolled out")
					Success("istio-cni-node DaemonSet rolled out")
				})

				It("keeps the sample pods running", func(ctx SpecContext) {
					// The CNI only programs redirection for pods as they start, so an update of the
					// DaemonSet must leave the already-running workloads untouched.
					Expect(common.CheckSamplePodsReady(ctx, cl)).To(Succeed(), "Sample pods are not ready after the IstioCNI update")
					Success("Sample pods stayed ready across the IstioCNI update")
				})

				It("should not disrupt traffic between running workloads", func() {
					// The measured window starts before the IstioCNI patch and ends once the DaemonSet
					// is fully rolled out, so it covers the whole CNI update.
					failures := traffic.stopAndGetFailures()

					Expect(failures).To(BeEmpty(),
						"IstioCNI update disrupted traffic between already-running workloads")
					Success("Traffic was uninterrupted throughout the IstioCNI update")
				})

				It("still serves traffic once the whole update is complete", func(ctx SpecContext) {
					// A final request after everything has settled: the control plane, the proxies and
					// the CNI are all on the newest version at this point.
					pods := &corev1.PodList{}
					Expect(cl.List(ctx, pods, client.InNamespace(sampleNamespace), client.MatchingLabels{"app": common.SleepContainerName})).To(Succeed())
					Expect(pods.Items).ToNot(BeEmpty(), "No sleep pod available to send traffic from")

					Eventually(func() error {
						return common.CheckHTTPConnectivity(k, sampleNamespace, pods.Items[0].Name, common.SleepContainerName,
							fmt.Sprintf("httpbin.%s.svc.cluster.local:8000/get", sampleNamespace), "200", 10)
					}).Should(Succeed(), "Traffic does not work after the update completed")
					Success("Traffic works after the skip-version update of all components")
				})
			})

			AfterAll(func(ctx SpecContext) {
				// Skip cleanup if test failed and keepOnFailure is set
				if CurrentSpecReport().Failed() && keepOnFailure {
					return
				}

				clr.Cleanup(ctx)
			})
		})
	})

	Describe("In-Place Updates", func() {
		var baseVersion, newVersion istioversion.VersionInfo

		BeforeAll(func() {
			var err error
			baseVersion, newVersion, err = istioversion.GetTwoConsecutiveMinorVersions(istioversion.Sidecar)
			if err != nil {
				Skip(fmt.Sprintf("Skipping update tests: %v", err))
			}
		})

		Context(fmt.Sprintf("Updating from %s to %s", baseVersion.Name, newVersion.Name), func() {
			clr := cleaner.New(cl)
			var validator *common.WorkloadValidator

			BeforeAll(func(ctx SpecContext) {
				clr.Record(ctx)
				Expect(k.CreateNamespace(controlPlaneNamespace)).To(Succeed(), "Istio namespace failed to be created")
				Expect(k.CreateNamespace(istioCniNamespace)).To(Succeed(), "IstioCNI namespace failed to be created")

				common.CreateIstioCNI(k, baseVersion.Name)
				common.AwaitCondition(ctx, v1.IstioCNIConditionReady, kube.Key(istioCniName), &v1.IstioCNI{}, k, cl)
			})

			// Capture debug info immediately on test failure
			JustAfterEach(func(ctx SpecContext) {
				if CurrentSpecReport().Failed() {
					common.LogDebugInfo(common.ControlPlane, k)
				}
			})

			When(fmt.Sprintf("Istio CR is created with InPlace updateStrategy for version %s", baseVersion.Name), func() {
				BeforeAll(func() {
					common.CreateIstio(k, baseVersion.Name, `
updateStrategy:
  type: InPlace`)
				})

				It("should deploy istiod and become Ready", func(ctx SpecContext) {
					common.AwaitCondition(ctx, v1.IstioConditionReady, kube.Key("default"), &v1.Istio{}, k, cl)
					common.AwaitDeployment(ctx, "istiod", k, cl)
					Success("Istio CR is Ready and istiod deployment is available")
				})
			})

			When("workloads are deployed in sidecar mode", func() {
				BeforeAll(func(ctx SpecContext) {
					// Step 1: Initialize WorkloadValidator for sidecar mode testing
					// This sets up the validator to deploy and validate workloads with sidecar injection
					validator = &common.WorkloadValidator{
						K:             k,
						Cl:            cl,
						Namespace:     "workload-update-test",
						DataplaneMode: common.DataplaneModeSidecar,
					}
					// Step 2: Deploy test workloads (sleep + httpbin) with sidecar injection
					// - Creates workload-update-test and httpbin namespaces
					// - Labels both namespaces with istio-injection=enabled for sidecar injection
					// - Deploys sleep pod (with sidecar) in workload-update-test namespace
					// - Deploys httpbin service (with sidecar) in httpbin namespace
					Expect(validator.DeployWorkload(ctx)).To(Succeed(), "Failed to deploy workloads")
					Success("Workloads deployed")
				})

				It("should have connectivity with old version", func(ctx SpecContext) {
					Eventually(func(g Gomega) {
						// Step 3: Validate connectivity between sidecar-injected workloads
						// Tests that sleep pod can reach httpbin service through their sidecar proxies
						// This verifies the mesh is routing traffic correctly with base version sidecars
						g.Expect(validator.WaitForConnectivity(ctx)).To(Succeed())
					}).WithTimeout(120*time.Second).Should(Succeed(), "Workload connectivity failed")
					Success("Workloads have connectivity")
				})

				It("should have correct proxy version", func(ctx SpecContext) {
					Eventually(func(g Gomega) {
						// Step 4: Verify sidecar proxy version matches base version
						// Checks the istio-proxy container version in workload pods
						g.Expect(validator.ValidateProxyVersion(ctx, baseVersion.Version)).To(Succeed())
					}).WithTimeout(60*time.Second).Should(Succeed(), "Proxy version validation failed")
					Success("Proxy versions match old version")
				})
			})

			When("Istio CR version is updated", func() {
				var traffic *trafficMonitor

				BeforeAll(func(ctx SpecContext) {
					// Start traffic between the already-running workloads before the control plane is
					// touched, so the whole update happens with requests in flight. The workloads are
					// never restarted within this block, so every request that fails from here on is
					// attributable to the control plane update itself.
					traffic = startTrafficMonitor(ctx, validator.Namespace,
						fmt.Sprintf("httpbin.%s.svc.cluster.local:8000/get", common.HttpbinNamespace))
					Success("Continuous traffic established before the update")

					Expect(k.Patch("istio", "default", "merge", `{"spec":{"version":"`+newVersion.Name+`"}}`)).
						To(Succeed(), "Error updating Istio CR version")
					Success("Istio CR version updated to " + newVersion.Name)
				})

				AfterAll(func() {
					traffic.stop()
				})

				It("should remain a single IstioRevision", func(ctx SpecContext) {
					Consistently(func(g Gomega) {
						revisions := &v1.IstioRevisionList{}
						g.Expect(cl.List(ctx, revisions)).To(Succeed())
						g.Expect(revisions.Items).To(HaveLen(1), "Should have exactly one IstioRevision")
					}).WithTimeout(60 * time.Second).WithPolling(5 * time.Second).Should(Succeed())
					Success("Single IstioRevision maintained")
				})

				It("should update the IstioRevision spec.version in-place", func(ctx SpecContext) {
					Eventually(func(g Gomega) {
						revision := &v1.IstioRevision{}
						g.Expect(cl.Get(ctx, kube.Key("default", controlPlaneNamespace), revision)).To(Succeed())
						g.Expect(revision.Spec.Version).To(Equal(newVersion.Name))
					}).Should(Succeed(), "IstioRevision version should be updated in-place")
					Success("IstioRevision updated in-place")
				})

				It("should reconcile and remain Ready", func(ctx SpecContext) {
					common.AwaitCondition(ctx, v1.IstioConditionReady, kube.Key("default"), &v1.Istio{}, k, cl)
					Success("Istio CR remains Ready after version update")
				})

				It("should update the istiod deployment", func(ctx SpecContext) {
					Eventually(func(g Gomega) {
						version, err := common.GetVersionFromIstiod()
						g.Expect(err).NotTo(HaveOccurred())
						g.Expect(version).To(Equal(newVersion.Version))
					}).Should(Succeed(), "istiod deployment should be updated to new version")

					// The check above only proves that some istiod pod answers with the new version.
					// maxUnavailable floors to 0 for a single replica, so the new pod becomes ready
					// before the old one is removed and the rollout may still be in progress here.
					Eventually(func(g Gomega) {
						deployment := &appsv1.Deployment{}
						g.Expect(cl.Get(ctx, kube.Key("istiod", controlPlaneNamespace), deployment)).To(Succeed())
						g.Expect(deployment.Status.AvailableReplicas).To(BeNumerically(">", 0))
						g.Expect(deployment.Status.UpdatedReplicas).To(Equal(deployment.Status.Replicas))
					}).Should(Succeed(), "istiod deployment should be fully rolled out")
					Success("istiod deployment updated")
				})

				It("should not disrupt traffic between running workloads", func() {
					// Traffic has been running since before the version patch and the rollout is
					// complete by now, so the measured window covers the whole update.
					failures := traffic.stopAndGetFailures()

					// Existing sidecars keep their configuration across an istiod restart, so an
					// in-place control plane update must not drop a single request from workloads
					// that were already running.
					Expect(failures).To(BeEmpty(),
						"In-place control plane update disrupted traffic between already-running workloads")
					Success("Traffic was uninterrupted throughout the in-place update")
				})
			})

			When("workloads are restarted", func() {
				BeforeAll(func(ctx SpecContext) {
					// Delete pods to trigger restart with new sidecar version
					// This simulates pod restart which will pick up the updated injector webhook
					// and inject sidecars with the new version
					pods := &corev1.PodList{}
					Expect(cl.List(ctx, pods, client.InNamespace("workload-update-test"))).To(Succeed())
					for _, pod := range pods.Items {
						Expect(cl.Delete(ctx, &pod)).To(Succeed())
					}
					Success("Workload pods deleted for restart")
				})

				It("should have connectivity with new version", func(ctx SpecContext) {
					Eventually(func(g Gomega) {
						// Validate connectivity after pod restart with new sidecars
						// Tests that sleep pod (with new sidecar) can reach httpbin service (with new sidecar)
						// This confirms the in-place update completed successfully and new sidecars are working
						g.Expect(validator.WaitForConnectivity(ctx)).To(Succeed())
					}).WithTimeout(120*time.Second).Should(Succeed(), "Workload connectivity failed after restart")
					Success("Workloads have connectivity after restart")
				})

				It("should have updated proxy version", func(ctx SpecContext) {
					Eventually(func(g Gomega) {
						// Verify sidecar proxy version has been upgraded
						// Checks that restarted pods now have sidecars with the new version
						g.Expect(validator.ValidateProxyVersion(ctx, newVersion.Version)).To(Succeed())
					}).WithTimeout(120*time.Second).Should(Succeed(), "Proxy version should match new version")
					Success("Proxy versions updated to new version")
				})

				It("should still have only one IstioRevision", func(ctx SpecContext) {
					revisions := &v1.IstioRevisionList{}
					Expect(cl.List(ctx, revisions)).To(Succeed())
					Expect(revisions.Items).To(HaveLen(1), "Should still have exactly one IstioRevision")
					Expect(revisions.Items[0].Name).To(Equal("default"), "IstioRevision name should match Istio CR name")
					Success("Single IstioRevision confirmed")
				})
			})

			AfterAll(func(ctx SpecContext) {
				// Skip cleanup if test failed and keepOnFailure is set
				if CurrentSpecReport().Failed() && keepOnFailure {
					return
				}
				clr.Cleanup(ctx)
			})
		})
	})

	Describe("Lifecycle Transitions", func() {
		var newVersion istioversion.VersionInfo

		BeforeAll(func() {
			var err error
			_, newVersion, err = istioversion.GetTwoConsecutiveMinorVersions(istioversion.Sidecar)
			if err != nil {
				Skip(fmt.Sprintf("Skipping update tests: %v", err))
			}
		})

		Context("Spec changes and finalizers", func() {
			clr := cleaner.New(cl)
			testVersion := newVersion

			BeforeAll(func(ctx SpecContext) {
				clr.Record(ctx)
				Expect(k.CreateNamespace(controlPlaneNamespace)).To(Succeed(), "Istio namespace failed to be created")
				Expect(k.CreateNamespace(istioCniNamespace)).To(Succeed(), "IstioCNI namespace failed to be created")

				common.CreateIstioCNI(k, testVersion.Name)
				common.AwaitCondition(ctx, v1.IstioCNIConditionReady, kube.Key(istioCniName), &v1.IstioCNI{}, k, cl)

				// Create Istio CR with test version
				common.CreateIstio(k, testVersion.Name, `
updateStrategy:
  type: InPlace`)

				// Wait for it to be ready
				common.AwaitCondition(ctx, v1.IstioConditionReady, kube.Key("default"), &v1.Istio{}, k, cl)
				Success("Istio CR created and ready for lifecycle tests")
			})

			// Capture debug info immediately on test failure
			JustAfterEach(func(ctx SpecContext) {
				if CurrentSpecReport().Failed() {
					common.LogDebugInfo(common.ControlPlane, k)
				}
			})

			When("spec.values is updated on Istio CR", func() {
				It("should update istiod deployment when values change", func(ctx SpecContext) {
					Log("Updating Istio spec.values")
					Eventually(func(g Gomega) {
						// Get current Istio CR fresh on each attempt to avoid 409 Conflict
						istio := &v1.Istio{}
						g.Expect(cl.Get(ctx, kube.Key("default"), istio)).To(Succeed())

						// Modify spec.values to add a custom env var
						if istio.Spec.Values == nil {
							istio.Spec.Values = &v1.Values{}
						}
						if istio.Spec.Values.Pilot == nil {
							istio.Spec.Values.Pilot = &v1.PilotConfig{}
						}
						if istio.Spec.Values.Pilot.Env == nil {
							istio.Spec.Values.Pilot.Env = make(map[string]string)
						}
						istio.Spec.Values.Pilot.Env["TEST_VAR"] = "test-value"

						g.Expect(cl.Update(ctx, istio)).To(Succeed())
					}).Should(Succeed())

					// Verify the istiod Deployment is updated with the new env var
					Eventually(func(g Gomega) {
						deployment := &appsv1.Deployment{}
						g.Expect(cl.Get(ctx, kube.Key("istiod", controlPlaneNamespace), deployment)).To(Succeed())

						// Check if the env var exists in the deployment
						found := false
						for _, container := range deployment.Spec.Template.Spec.Containers {
							for _, env := range container.Env {
								if env.Name == "TEST_VAR" && env.Value == "test-value" {
									found = true
									break
								}
							}
						}
						g.Expect(found).To(BeTrue(), "TEST_VAR should be in istiod deployment")
					}).WithTimeout(120*time.Second).Should(Succeed(), "istiod deployment should have new env var")
					Success("istiod deployment updated after spec.values change")
				})
			})

			When("CR is deleted", func() {
				It("should have finalizer on IstioRevision that prevents immediate deletion", func(ctx SpecContext) {
					// IstioRevision has finalizers (not Istio CR itself)
					revision := &v1.IstioRevision{}
					Expect(cl.Get(ctx, kube.Key("default", controlPlaneNamespace), revision)).To(Succeed())
					Expect(revision.Finalizers).NotTo(BeEmpty(), "IstioRevision should have finalizers")
					Success("IstioRevision has finalizers")
				})

				It("should cleanup resources when deleted", func(ctx SpecContext) {
					istio := &v1.Istio{}
					Expect(cl.Get(ctx, kube.Key("default"), istio)).To(Succeed())

					Log("Deleting Istio CR")
					Expect(cl.Delete(ctx, istio)).To(Succeed())

					// Verify the istiod Deployment is deleted
					Eventually(func(g Gomega) {
						deployment := &appsv1.Deployment{}
						err := cl.Get(ctx, kube.Key("istiod", controlPlaneNamespace), deployment)
						g.Expect(err).To(HaveOccurred())
						g.Expect(err.Error()).To(ContainSubstring("not found"))
					}).WithTimeout(120*time.Second).Should(Succeed(), "istiod deployment should be deleted")

					// Verify the IstioRevision is deleted
					Eventually(func(g Gomega) {
						revision := &v1.IstioRevision{}
						err := cl.Get(ctx, kube.Key("default", controlPlaneNamespace), revision)
						g.Expect(err).To(HaveOccurred())
						g.Expect(err.Error()).To(ContainSubstring("not found"))
					}).WithTimeout(60*time.Second).Should(Succeed(), "IstioRevision should be deleted")

					// Verify the Istio CR is fully deleted
					Eventually(func(g Gomega) {
						ist := &v1.Istio{}
						err := cl.Get(ctx, kube.Key("default"), ist)
						g.Expect(err).To(HaveOccurred())
						g.Expect(err.Error()).To(ContainSubstring("not found"))
					}).WithTimeout(60*time.Second).Should(Succeed(), "Istio CR should be fully deleted")
					Success("Istio CR and all resources cleaned up successfully")
				})
			})

			AfterAll(func(ctx SpecContext) {
				// Skip cleanup if test failed and keepOnFailure is set
				if CurrentSpecReport().Failed() && keepOnFailure {
					return
				}
				clr.Cleanup(ctx)
			})
		})
	})
})

// trafficMonitor generates continuous HTTP traffic from a sleep pod for as long as an update is
// in progress, so that the requests failed during the update can be counted afterwards.
type trafficMonitor struct {
	stats            *common.HTTPTrafficStats
	cancel           context.CancelFunc
	baselineFailures int
}

// startTrafficMonitor starts traffic from the sleep pod in clientNamespace to targetURL and returns
// once a baseline of successful requests proves that the path works. Requests that failed while
// traffic was warming up are excluded from what stopAndGetFailures reports, so only failures
// recorded after this point count against whatever the caller does next.
func startTrafficMonitor(ctx context.Context, clientNamespace, targetURL string) *trafficMonitor {
	pods := &corev1.PodList{}
	Expect(cl.List(ctx, pods, client.InNamespace(clientNamespace), client.MatchingLabels{"app": common.SleepContainerName})).To(Succeed())
	Expect(pods.Items).ToNot(BeEmpty(), "No sleep pod available to generate traffic from")

	// context.Background() keeps traffic flowing across the It blocks of the enclosing container;
	// it is stopped by stopAndGetFailures and again in AfterAll as a safety net.
	stats, cancel := common.StartContinuousHTTPTraffic(
		context.Background(), k, clientNamespace, pods.Items[0].Name, common.SleepContainerName,
		targetURL, 500*time.Millisecond, nil)
	monitor := &trafficMonitor{stats: stats, cancel: cancel}

	// Establish a baseline so that the assertion on the failures cannot pass merely because no
	// traffic was ever sent.
	Eventually(func(g Gomega) {
		_, success, _, _ := stats.GetStats()
		g.Expect(success).To(BeNumerically(">=", 5), "Baseline traffic should flow before the update starts")
	}).WithTimeout(60 * time.Second).WithPolling(500 * time.Millisecond).Should(Succeed())

	_, _, _, baselineErrors := stats.GetStats()
	monitor.baselineFailures = len(baselineErrors)
	return monitor
}

// stop ends the traffic. It is safe to call more than once, and on a monitor that was never started
// because the setup that would have created it failed.
func (m *trafficMonitor) stop() {
	if m != nil && m.cancel != nil {
		m.cancel()
	}
}

// stopAndGetFailures stops the traffic and returns the requests that failed after the baseline.
func (m *trafficMonitor) stopAndGetFailures() []string {
	m.stop()
	// Let requests that are already in flight finish and be recorded.
	time.Sleep(2 * time.Second)

	total, success, failed, errors := m.stats.GetStats()
	Log(fmt.Sprintf("Traffic during update: %d total, %d success, %d failed", total, success, failed))
	return errors[m.baselineFailures:]
}
