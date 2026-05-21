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
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/update"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Control Plane updates", Label("control-plane", "update", "slow"), Ordered, func() {
	SetDefaultEventuallyTimeout(time.Duration(env.GetInt("DEFAULT_TEST_TIMEOUT", 180)) * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)
	debugInfoLogged := false

	Describe("using IstioRevisionTag", func() {
		var baseVersion, newVersion istioversion.VersionInfo

		BeforeAll(func() {
			var err error
			baseVersion, newVersion, err = update.GetTwoConsecutiveSidecarVersions()
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
			})

			When("sample pod is deployed", func() {
				BeforeAll(func(ctx SpecContext) {
					Expect(k.CreateNamespace(sampleNamespace)).To(Succeed(), "Sample namespace failed to be created")
					Expect(k.Label("namespace", sampleNamespace, "istio-injection", "enabled")).To(Succeed(), "Error labeling sample namespace")
					Expect(k.WithNamespace(sampleNamespace).
						ApplyKustomize("helloworld", "version=v1")).
						To(Succeed(), "Error deploying sample")
					Success("sample deployed")

					samplePods := &corev1.PodList{}
					Eventually(common.CheckSamplePodsReady).WithArguments(ctx, cl).Should(Succeed(), "Error checking status of sample pods")
					Expect(cl.List(ctx, samplePods, client.InNamespace(sampleNamespace))).To(Succeed(), "Error getting the pods in sample namespace")

					Success("sample pods are ready")

					for _, pod := range samplePods.Items {
						sidecarVersion, err := common.GetProxyVersion(pod.Name, sampleNamespace)
						Expect(err).NotTo(HaveOccurred(), "Error getting sidecar version")
						Expect(sidecarVersion).To(Equal(baseVersion.Version), "Sidecar Istio version does not match the expected version")
					}
					Success("Istio sidecar version matches the expected base Istio version")
				})

				It("IstioRevisionTag state change to inUse true", func(ctx SpecContext) {
					common.AwaitCondition(ctx, v1.IstioRevisionTagConditionInUse, kube.Key("default"), &v1.IstioRevisionTag{}, k, cl)
				})
			})

			When("the Istio CR is updated to the new Istio version", func() {
				BeforeAll(func() {
					Expect(k.Patch("istio", "default", "merge", `{"spec":{"version":"`+newVersion.Name+`"}}`)).To(Succeed(), "Error updating Istio CR to new Istio version")
					Success("Istio CR updated")
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
						Eventually(func(g Gomega) {
							sidecarVersion, err := common.GetProxyVersion(pod.Name, sampleNamespace)
							g.Expect(err).NotTo(HaveOccurred(), "Error getting sidecar version")
							g.Expect(sidecarVersion).To(Equal(baseVersion.Version))
						}).Should(Succeed(), "Sidecar Istio version does not match the expected version")
					}
					Success("Istio sidecar version matches the expected Istio version")
				})
			})

			When("sample pod are restarted", func() {
				BeforeAll(func(ctx SpecContext) {
					samplePods := &corev1.PodList{}
					Expect(cl.List(ctx, samplePods, client.InNamespace(sampleNamespace))).To(Succeed())
					Expect(samplePods.Items).ToNot(BeEmpty(), "No pods found in sample namespace")

					for _, pod := range samplePods.Items {
						cl.Delete(ctx, &pod)
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
							sidecarVersion, err := common.GetProxyVersion(pod.Name, sampleNamespace)
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
				if CurrentSpecReport().Failed() {
					common.LogDebugInfo(common.ControlPlane, k)
					debugInfoLogged = true
					if keepOnFailure {
						return
					}
				}

				clr.Cleanup(ctx)
			})
		})

		AfterAll(func() {
			if CurrentSpecReport().Failed() {
				if !debugInfoLogged {
					common.LogDebugInfo(common.ControlPlane, k)
					debugInfoLogged = true

					if keepOnFailure {
						return
					}
				}
			}
		})
	})

	Describe("In-Place Updates", func() {
		var baseVersion, newVersion istioversion.VersionInfo

		BeforeAll(func() {
			var err error
			baseVersion, newVersion, err = update.GetTwoConsecutiveSidecarVersions()
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
						g.Expect(validator.ValidateConnectivity(ctx)).To(Succeed())
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
				BeforeAll(func() {
					Expect(k.Patch("istio", "default", "merge", `{"spec":{"version":"`+newVersion.Name+`"}}`)).
						To(Succeed(), "Error updating Istio CR version")
					Success("Istio CR version updated to " + newVersion.Name)
				})

				It("should remain a single IstioRevision", func(ctx SpecContext) {
					Consistently(func(g Gomega) {
						revisions := &v1.IstioRevisionList{}
						g.Expect(cl.List(ctx, revisions)).To(Succeed())
						g.Expect(revisions.Items).To(HaveLen(1), "Should have exactly one IstioRevision")
					}).WithTimeout(30 * time.Second).Should(Succeed())
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
					Success("istiod deployment updated")
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
						g.Expect(validator.ValidateConnectivity(ctx)).To(Succeed())
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
				if CurrentSpecReport().Failed() {
					common.LogDebugInfo(common.ControlPlane, k)
					if keepOnFailure {
						return
					}
				}
				clr.Cleanup(ctx)
			})
		})
	})

	Describe("Lifecycle Transitions", func() {
		var newVersion istioversion.VersionInfo

		BeforeAll(func() {
			var err error
			_, newVersion, err = update.GetTwoConsecutiveSidecarVersions()
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

				// Create Istio CR with test version
				common.CreateIstio(k, testVersion.Name, `
updateStrategy:
  type: InPlace`)

				// Wait for it to be ready
				common.AwaitCondition(ctx, v1.IstioConditionReady, kube.Key("default"), &v1.Istio{}, k, cl)
				Success("Istio CR created and ready for lifecycle tests")
			})

			When("spec.values is updated on Istio CR", func() {
				It("should update istiod deployment when values change", func(ctx SpecContext) {
					// Get current Istio CR
					istio := &v1.Istio{}
					Expect(cl.Get(ctx, kube.Key("default"), istio)).To(Succeed())

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

					Log("Updating Istio spec.values")
					Expect(cl.Update(ctx, istio)).To(Succeed())

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
				if CurrentSpecReport().Failed() {
					common.LogDebugInfo(common.ControlPlane, k)
					if keepOnFailure {
						return
					}
				}
				clr.Cleanup(ctx)
			})
		})
	})
})
