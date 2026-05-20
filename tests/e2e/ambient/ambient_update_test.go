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

package ambient

import (
	"fmt"
	"time"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/cleaner"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	. "github.com/istio-ecosystem/sail-operator/tests/e2e/util/gomega"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/update"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Ambient Update & Lifecycle", Label("ambient", "update", "slow"), Ordered, func() {
	SetDefaultEventuallyTimeout(time.Duration(defaultTimeout) * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)
	debugInfoLogged := false

	// Get two consecutive minor versions for update testing
	baseVersion, newVersion, err := update.GetTwoConsecutiveAmbientVersions(fipsCluster)
	if err != nil {
		Skip(fmt.Sprintf("Skipping ambient update tests: %v", err))
		return
	}

	Describe("In-Place Updates", func() {
		Context(fmt.Sprintf("Updating from %s to %s", baseVersion.Version, newVersion.Version), func() {
			clr := cleaner.New(cl)
			var validator *common.WorkloadValidator

			BeforeAll(func(ctx SpecContext) {
				clr.Record(ctx)
				Expect(k.CreateNamespace(controlPlaneNamespace)).To(Succeed())
				Expect(k.CreateNamespace(istioCniNamespace)).To(Succeed())
				Expect(k.CreateNamespace(ztunnelNamespace)).To(Succeed())
			})

			When("all components are created with base version", func() {
				BeforeAll(func(ctx SpecContext) {
					// Create IstioCNI
					cniYAML := fmt.Sprintf(`
apiVersion: sailoperator.io/v1
kind: IstioCNI
metadata:
  name: default
spec:
  version: %s
  namespace: %s
  profile: ambient`, baseVersion.Name, istioCniNamespace)
					Log("Creating IstioCNI with version:", baseVersion.Name)
					Expect(k.CreateFromString(cniYAML)).To(Succeed())

					// Create ZTunnel
					ztunnelYAML := fmt.Sprintf(`
apiVersion: sailoperator.io/v1
kind: ZTunnel
metadata:
  name: default
spec:
  version: %s
  namespace: %s`, baseVersion.Name, ztunnelNamespace)
					Log("Creating ZTunnel with version:", baseVersion.Name)
					Expect(k.CreateFromString(ztunnelYAML)).To(Succeed())

					// Create Istio
					istioYAML := fmt.Sprintf(`
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: %s
spec:
  version: %s
  namespace: %s
  values:
    profile: ambient
    pilot:
      cni:
        enabled: true
      trustedZtunnelNamespace: ztunnel`, istioName, baseVersion.Name, controlPlaneNamespace)
					Log("Creating Istio with version:", baseVersion.Name)
					Expect(k.CreateFromString(istioYAML)).To(Succeed())
					Success("All components created with base version")
				})

				It("should have IstioCNI Ready", func(ctx SpecContext) {
					common.AwaitCondition(ctx, v1.IstioCNIConditionReady, kube.Key("default"), &v1.IstioCNI{}, k, cl, 180*time.Second)
					Success("IstioCNI is Ready with base version")
				})

				It("should have ZTunnel Ready", func(ctx SpecContext) {
					common.AwaitCondition(ctx, v1.ZTunnelConditionReady, kube.Key("default"), &v1.ZTunnel{}, k, cl, 180*time.Second)
					Success("ZTunnel is Ready with base version")
				})

				It("should have Istio Ready", func(ctx SpecContext) {
					common.AwaitCondition(ctx, v1.IstioConditionReady, kube.Key(istioName), &v1.Istio{}, k, cl, 240*time.Second)
					Success("Istio is Ready with base version")
				})

				It("should have istiod deployment running", func(ctx SpecContext) {
					common.AwaitDeployment(ctx, "istiod", k, cl)
					Success("istiod deployment is running")
				})
			})

			When("workloads are deployed in ambient mode", func() {
				BeforeAll(func(ctx SpecContext) {
					// Step 1: Initialize WorkloadValidator for ambient mode testing
					// This sets up the validator to deploy and validate workloads in ambient dataplane mode
					validator = &common.WorkloadValidator{
						K:             k,
						Cl:            cl,
						Namespace:     "workload-update-test",
						DataplaneMode: common.DataplaneModeAmbient,
					}
					// Step 2: Deploy test workloads (sleep + httpbin)
					// - Creates workload-update-test and httpbin namespaces
					// - Labels both namespaces with istio.io/dataplane-mode=ambient
					// - Deploys sleep pod in workload-update-test namespace
					// - Deploys httpbin service in httpbin namespace
					Expect(validator.DeployWorkload(ctx)).To(Succeed(), "Failed to deploy ambient workloads")
					Success("Workloads deployed in ambient mode")
				})

				It("should have connectivity with base version components", func(ctx SpecContext) {
					Eventually(func(g Gomega) {
						// Step 3: Validate connectivity between workloads
						// Tests that sleep pod in workload-update-test can reach httpbin service in httpbin namespace
						// This verifies the ambient mesh is routing traffic correctly through ZTunnel proxies
						g.Expect(validator.ValidateConnectivity(ctx)).To(Succeed())
						// Step 4: Verify ZTunnel version matches base version
						// Checks the ZTunnel DaemonSet image tag to confirm it's running the expected base version
						g.Expect(validator.ValidateProxyVersion(ctx, baseVersion.Version)).To(Succeed())
					}).WithTimeout(120*time.Second).Should(Succeed(), "Workloads should have connectivity with old ZTunnel version")
					Success("Workloads have connectivity with old version")
				})
			})

			When("IstioCNI version is updated", func() {
				BeforeAll(func(ctx SpecContext) {
					cni := &v1.IstioCNI{}
					Expect(cl.Get(ctx, kube.Key("default"), cni)).To(Succeed())

					Log(fmt.Sprintf("Updating IstioCNI from %s to %s", baseVersion.Name, newVersion.Name))
					cni.Spec.Version = newVersion.Name
					Expect(cl.Update(ctx, cni)).To(Succeed())
					Success("IstioCNI version updated")
				})

				It("should reconcile and remain Ready", func(ctx SpecContext) {
					Eventually(func(g Gomega) {
						cni := &v1.IstioCNI{}
						g.Expect(cl.Get(ctx, kube.Key("default"), cni)).To(Succeed())
						g.Expect(cni.Spec.Version).To(Equal(newVersion.Name))
						g.Expect(cni).To(HaveConditionStatus(v1.IstioCNIConditionReady, metav1.ConditionTrue))
					}).WithTimeout(180*time.Second).Should(Succeed(), "IstioCNI should be Ready with new version")
					Success("IstioCNI successfully updated")
				})

				It("should update the DaemonSet", func(ctx SpecContext) {
					Eventually(func(g Gomega) {
						ds := &appsv1.DaemonSet{}
						g.Expect(cl.Get(ctx, kube.Key("istio-cni-node", istioCniNamespace), ds)).To(Succeed())
						g.Expect(ds.Status.NumberAvailable).To(BeNumerically(">", 0))
						g.Expect(ds.Status.UpdatedNumberScheduled).To(Equal(ds.Status.DesiredNumberScheduled))
					}).WithTimeout(180*time.Second).Should(Succeed(), "DaemonSet should be fully updated")
					Success("IstioCNI DaemonSet updated successfully")
				})

				It("workloads maintain connectivity after IstioCNI update", func(ctx SpecContext) {
					// Validate connectivity is still working after IstioCNI update
					// Tests that sleep pod can still reach httpbin service through the mesh
					// This confirms that updating the CNI component doesn't break existing connections
					Expect(validator.ValidateConnectivity(ctx)).To(Succeed(),
						"Workloads should maintain connectivity after IstioCNI update")
					Success("Workloads maintain connectivity")
				})
			})

			When("ZTunnel version is updated", func() {
				BeforeAll(func(ctx SpecContext) {
					ztunnel := &v1.ZTunnel{}
					Expect(cl.Get(ctx, kube.Key("default"), ztunnel)).To(Succeed())

					Log(fmt.Sprintf("Updating ZTunnel from %s to %s", baseVersion.Name, newVersion.Name))
					ztunnel.Spec.Version = newVersion.Name
					Expect(cl.Update(ctx, ztunnel)).To(Succeed())
					Success("ZTunnel version updated")
				})

				It("should reconcile and remain Ready", func(ctx SpecContext) {
					Eventually(func(g Gomega) {
						ztunnel := &v1.ZTunnel{}
						g.Expect(cl.Get(ctx, kube.Key("default"), ztunnel)).To(Succeed())
						g.Expect(ztunnel.Spec.Version).To(Equal(newVersion.Name))
						g.Expect(ztunnel).To(HaveConditionStatus(v1.ZTunnelConditionReady, metav1.ConditionTrue))
					}).WithTimeout(180*time.Second).Should(Succeed(), "ZTunnel should be Ready with new version")
					Success("ZTunnel successfully updated")
				})

				It("should update the DaemonSet", func(ctx SpecContext) {
					Eventually(func(g Gomega) {
						ds := &appsv1.DaemonSet{}
						g.Expect(cl.Get(ctx, kube.Key("ztunnel", ztunnelNamespace), ds)).To(Succeed())
						g.Expect(ds.Status.NumberAvailable).To(BeNumerically(">", 0))
						g.Expect(ds.Status.UpdatedNumberScheduled).To(Equal(ds.Status.DesiredNumberScheduled))
					}).WithTimeout(180*time.Second).Should(Succeed(), "ZTunnel DaemonSet should be fully updated")
					Success("ZTunnel DaemonSet updated successfully")
				})

				It("workloads have connectivity and use new ZTunnel version", func(ctx SpecContext) {
					Eventually(func(g Gomega) {
						// Validate connectivity after ZTunnel update
						// Tests that sleep pod can reach httpbin service through the updated ZTunnel proxies
						// This confirms the ZTunnel rolling update completed successfully without breaking traffic
						g.Expect(validator.ValidateConnectivity(ctx)).To(Succeed())
						// Verify ZTunnel version has been upgraded
						// ZTunnel DaemonSet should now have new version
						g.Expect(validator.ValidateProxyVersion(ctx, newVersion.Version)).To(Succeed())
					}).WithTimeout(120*time.Second).Should(Succeed(), "Workloads should have connectivity with new ZTunnel version")
					Success("Workloads have connectivity with new ZTunnel version")
				})
			})

			When("Istio version is updated", func() {
				BeforeAll(func(ctx SpecContext) {
					// Update the Istio CR to new version
					istio := &v1.Istio{}
					Expect(cl.Get(ctx, kube.Key(istioName), istio)).To(Succeed())

					Log(fmt.Sprintf("Updating Istio from %s to %s", baseVersion.Name, newVersion.Name))
					istio.Spec.Version = newVersion.Name
					Expect(cl.Update(ctx, istio)).To(Succeed())
					Success("Istio version updated")
				})

				It("should reconcile and remain Ready", func(ctx SpecContext) {
					Eventually(func(g Gomega) {
						istio := &v1.Istio{}
						g.Expect(cl.Get(ctx, kube.Key(istioName), istio)).To(Succeed())
						g.Expect(istio.Spec.Version).To(Equal(newVersion.Name))
						g.Expect(istio).To(HaveConditionStatus(v1.IstioConditionReady, metav1.ConditionTrue))
					}).WithTimeout(240*time.Second).Should(Succeed(), "Istio should be Ready with new version")
					Success("Istio successfully updated")
				})

				It("should update the istiod deployment", func(ctx SpecContext) {
					Eventually(func(g Gomega) {
						deployment := &appsv1.Deployment{}
						g.Expect(cl.Get(ctx, kube.Key("istiod", controlPlaneNamespace), deployment)).To(Succeed())
						g.Expect(deployment.Status.AvailableReplicas).To(BeNumerically(">", 0))
						g.Expect(deployment.Status.UpdatedReplicas).To(Equal(deployment.Status.Replicas))
					}).WithTimeout(180*time.Second).Should(Succeed(), "istiod deployment should be fully updated")
					Success("istiod deployment updated successfully")
				})
			})

			AfterAll(func(ctx SpecContext) {
				if CurrentSpecReport().Failed() {
					common.LogDebugInfo(common.Ambient, k)
					debugInfoLogged = true
					if keepOnFailure {
						return
					}
				}
				clr.Cleanup(ctx)
			})
		})
	})

	Describe("Revision-Based Updates", func() {
		Context(fmt.Sprintf("Canary update from %s to %s", baseVersion.Version, newVersion.Version), func() {
			clr := cleaner.New(cl)
			var validator *common.WorkloadValidator

			BeforeAll(func(ctx SpecContext) {
				clr.Record(ctx)
				Expect(k.CreateNamespace(controlPlaneNamespace)).To(Succeed())
				Expect(k.CreateNamespace(istioCniNamespace)).To(Succeed())
				Expect(k.CreateNamespace(ztunnelNamespace)).To(Succeed())

				// Create shared dependencies (IstioCNI and ZTunnel) with OLD version
				// In a real canary update, you start with all components at the old version,
				// then add a new revision at the new version
				cniYAML := fmt.Sprintf(`
apiVersion: sailoperator.io/v1
kind: IstioCNI
metadata:
  name: default
spec:
  version: %s
  namespace: %s
  profile: ambient`, baseVersion.Name, istioCniNamespace)
				Log("Creating IstioCNI with base version:", baseVersion.Name)
				Expect(k.CreateFromString(cniYAML)).To(Succeed())

				ztunnelYAML := fmt.Sprintf(`
apiVersion: sailoperator.io/v1
kind: ZTunnel
metadata:
  name: default
spec:
  version: %s
  namespace: %s`, baseVersion.Name, ztunnelNamespace)
				Log("Creating ZTunnel with base version:", baseVersion.Name)
				Expect(k.CreateFromString(ztunnelYAML)).To(Succeed())

				// Wait for IstioCNI to be ready
				Eventually(func(g Gomega) {
					cni := &v1.IstioCNI{}
					g.Expect(cl.Get(ctx, kube.Key("default"), cni)).To(Succeed())
					g.Expect(cni).To(HaveConditionStatus(v1.IstioCNIConditionReady, metav1.ConditionTrue))
				}).Should(Succeed(), "IstioCNI should be Ready")

				// Note: We don't wait for ZTunnel to be fully Ready here because it needs
				// istiod to be running to get XDS configuration. It will become Ready once
				// the first IstioRevision is created.

				Success("Shared dependencies created at base version")
			})

			When("Istio CR is created with base version (default revision)", func() {
				BeforeAll(func() {
					// Use Istio CR for the default/first revision
					// This creates an istiod service without revision suffix that ZTunnel can connect to
					istioYAML := fmt.Sprintf(`
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: %s
spec:
  version: %s
  namespace: %s
  values:
    profile: ambient
    pilot:
      cni:
        enabled: true
      trustedZtunnelNamespace: ztunnel`, istioName, baseVersion.Name, controlPlaneNamespace)
					Log("Creating Istio CR with base version:", baseVersion.Name)
					Expect(k.CreateFromString(istioYAML)).To(Succeed())
					Success("Istio CR created (default revision)")
				})

				It("should become Ready", func(ctx SpecContext) {
					common.AwaitCondition(ctx, v1.IstioConditionReady, kube.Key(istioName), &v1.Istio{}, k, cl, 240*time.Second)
					Success("Istio is Ready with base version")
				})

				It("should make ZTunnel Ready now that istiod is running", func(ctx SpecContext) {
					// ZTunnel should now become Ready since istiod is available
					Eventually(func(g Gomega) {
						ztunnel := &v1.ZTunnel{}
						g.Expect(cl.Get(ctx, kube.Key("default"), ztunnel)).To(Succeed())
						g.Expect(ztunnel).To(HaveConditionStatus(v1.ZTunnelConditionReady, metav1.ConditionTrue))
					}).WithTimeout(180*time.Second).Should(Succeed(), "ZTunnel should become Ready")
					Success("ZTunnel is now Ready with base version")
				})
			})

			When("workloads are deployed in ambient mode", func() {
				BeforeAll(func(ctx SpecContext) {
					// Step 1: Initialize WorkloadValidator for canary testing in ambient mode
					validator = &common.WorkloadValidator{
						K:             k,
						Cl:            cl,
						Namespace:     "workload-canary-test",
						DataplaneMode: common.DataplaneModeAmbient,
					}
					// Step 2: Deploy test workloads for canary scenario
					// - Creates workload-canary-test and httpbin namespaces
					// - Labels both namespaces with istio.io/dataplane-mode=ambient
					// - Deploys sleep pod and httpbin service
					Expect(validator.DeployWorkload(ctx)).To(Succeed(), "Failed to deploy ambient workloads")
					Success("Workloads deployed in ambient mode")
				})

				It("should have connectivity", func(ctx SpecContext) {
					Eventually(func(g Gomega) {
						// Validate initial connectivity with base version
						// Tests that sleep pod in workload-canary-test can reach httpbin service
						// This establishes the baseline before introducing the canary revision
						g.Expect(validator.ValidateConnectivity(ctx)).To(Succeed())
						// Verify initial ZTunnel version
						g.Expect(validator.ValidateProxyVersion(ctx, baseVersion.Version)).To(Succeed())
					}).WithTimeout(120*time.Second).Should(Succeed(), "Workloads should have connectivity with base version")
					Success("Workloads have connectivity with base version")
				})
			})

			When("shared dependencies are updated to latest version", func() {
				BeforeAll(func(ctx SpecContext) {
					// Upgrade IstioCNI
					cni := &v1.IstioCNI{}
					Expect(cl.Get(ctx, kube.Key("default"), cni)).To(Succeed())
					Log(fmt.Sprintf("Updating IstioCNI from %s to %s", baseVersion.Name, newVersion.Name))
					cni.Spec.Version = newVersion.Name
					Expect(cl.Update(ctx, cni)).To(Succeed())

					// Upgrade ZTunnel
					ztunnel := &v1.ZTunnel{}
					Expect(cl.Get(ctx, kube.Key("default"), ztunnel)).To(Succeed())
					Log(fmt.Sprintf("Updating ZTunnel from %s to %s", baseVersion.Name, newVersion.Name))
					ztunnel.Spec.Version = newVersion.Name
					Expect(cl.Update(ctx, ztunnel)).To(Succeed())

					Success("Shared dependencies updated to latest version")
				})

				It("should have IstioCNI Ready with latest version", func(ctx SpecContext) {
					Eventually(func(g Gomega) {
						cni := &v1.IstioCNI{}
						g.Expect(cl.Get(ctx, kube.Key("default"), cni)).To(Succeed())
						g.Expect(cni.Spec.Version).To(Equal(newVersion.Name))
						g.Expect(cni).To(HaveConditionStatus(v1.IstioCNIConditionReady, metav1.ConditionTrue))
					}).WithTimeout(180*time.Second).Should(Succeed(), "IstioCNI should be Ready with new version")
					Success("IstioCNI updated successfully")
				})

				It("should have ZTunnel Ready with latest version", func(ctx SpecContext) {
					Eventually(func(g Gomega) {
						ztunnel := &v1.ZTunnel{}
						g.Expect(cl.Get(ctx, kube.Key("default"), ztunnel)).To(Succeed())
						g.Expect(ztunnel.Spec.Version).To(Equal(newVersion.Name))
						g.Expect(ztunnel).To(HaveConditionStatus(v1.ZTunnelConditionReady, metav1.ConditionTrue))
					}).WithTimeout(180*time.Second).Should(Succeed(), "ZTunnel should be Ready with new version")
					Success("ZTunnel updated successfully")
				})

				It("should keep default Istio revision healthy with updated dependencies", func(ctx SpecContext) {
					// Default revision should remain healthy with new version of shared deps
					Eventually(func(g Gomega) {
						istio := &v1.Istio{}
						g.Expect(cl.Get(ctx, kube.Key(istioName), istio)).To(Succeed())
						g.Expect(istio).To(HaveConditionStatus(v1.IstioConditionReady, metav1.ConditionTrue))
					}).WithTimeout(60*time.Second).Should(Succeed(), "Default Istio revision should remain Ready after dependency update")
					Success("Default Istio revision remains healthy after dependency update")
				})

				It("should keep connectivity working after the update to latest version", func(ctx SpecContext) {
					Eventually(func(g Gomega) {
						// Validate connectivity after shared dependencies update
						// Tests that sleep pod can reach httpbin service through the upgraded ZTunnel and CNI
						// This confirms that updating shared components (IstioCNI + ZTunnel) doesn't affect
						// workloads using the default (old) control plane revision
						g.Expect(validator.ValidateConnectivity(ctx)).To(Succeed())
						// Verify ZTunnel has been upgraded
						// ZTunnel version should now be new version
						g.Expect(validator.ValidateProxyVersion(ctx, newVersion.Version)).To(Succeed())
					}).WithTimeout(120*time.Second).Should(Succeed(),
						"Workloads should maintain connectivity with latest ZTunnel version")
					Success("Workloads maintain connectivity with new shared dependencies")
				})
			})

			When("canary IstioRevision is created with new version", func() {
				BeforeAll(func() {
					revisionYAML := fmt.Sprintf(`
apiVersion: sailoperator.io/v1
kind: IstioRevision
metadata:
  name: canary
spec:
  version: %s
  namespace: %s
  values:
    profile: ambient
    revision: canary
    global:
      istioNamespace: %s
    pilot:
      cni:
        enabled: true
      trustedZtunnelNamespace: ztunnel`, newVersion.Name, controlPlaneNamespace, controlPlaneNamespace)
					Log("Creating canary IstioRevision with new version:", newVersion.Name)
					Expect(k.CreateFromString(revisionYAML)).To(Succeed())
					Success("Canary IstioRevision created")
				})

				It("should become Ready", func(ctx SpecContext) {
					Eventually(func(g Gomega) {
						revision := &v1.IstioRevision{}
						g.Expect(cl.Get(ctx, kube.Key("canary", controlPlaneNamespace), revision)).To(Succeed())
						g.Expect(revision).To(HaveConditionStatus(v1.IstioRevisionConditionReady, metav1.ConditionTrue))
					}).WithTimeout(240*time.Second).Should(Succeed(), "Canary revision should become Ready")
					Success("Canary IstioRevision is Ready")
				})

				It("should coexist with default revision", func(ctx SpecContext) {
					// Verify default revision (from Istio CR) is still Ready
					istio := &v1.Istio{}
					Expect(cl.Get(ctx, kube.Key(istioName), istio)).To(Succeed())
					Expect(istio).To(HaveConditionStatus(v1.IstioConditionReady, metav1.ConditionTrue))

					// Get the default revision name
					defaultRevisionName := istio.Status.ActiveRevisionName
					defaultRevision := &v1.IstioRevision{}
					Expect(cl.Get(ctx, kube.Key(defaultRevisionName, controlPlaneNamespace), defaultRevision)).To(Succeed())
					Expect(defaultRevision).To(HaveConditionStatus(v1.IstioRevisionConditionReady, metav1.ConditionTrue))

					// Verify canary revision is Ready
					canaryRevision := &v1.IstioRevision{}
					Expect(cl.Get(ctx, kube.Key("canary", controlPlaneNamespace), canaryRevision)).To(Succeed())
					Expect(canaryRevision).To(HaveConditionStatus(v1.IstioRevisionConditionReady, metav1.ConditionTrue))

					// Verify both deployments exist
					defaultDeployment := &appsv1.Deployment{}
					Expect(cl.Get(ctx, kube.Key("istiod", controlPlaneNamespace), defaultDeployment)).To(Succeed())
					Expect(defaultDeployment.Status.AvailableReplicas).To(BeNumerically(">", 0))

					canaryDeployment := &appsv1.Deployment{}
					Expect(cl.Get(ctx, kube.Key("istiod-canary", controlPlaneNamespace), canaryDeployment)).To(Succeed())
					Expect(canaryDeployment.Status.AvailableReplicas).To(BeNumerically(">", 0))

					Success("Default and canary revisions coexist successfully")
				})

				It("connectivity works with both revisions present", func(ctx SpecContext) {
					// Validate connectivity with both control plane revisions active
					// Tests that sleep pod can reach httpbin service while both default and canary
					// istiod revisions are running side-by-side
					// This confirms that introducing a canary revision doesn't disrupt existing traffic
					Expect(validator.ValidateConnectivity(ctx)).To(Succeed(),
						"Workloads should maintain connectivity with canary revision present")
					Success("Workloads maintain connectivity with both revisions")
				})
			})

			When("shared dependencies are verified", func() {
				It("should have only one IstioCNI DaemonSet", func(ctx SpecContext) {
					// List all DaemonSets in IstioCNI namespace
					dsList := &appsv1.DaemonSetList{}
					Expect(cl.List(ctx, dsList, client.InNamespace(istioCniNamespace))).To(Succeed())

					// Should have exactly one CNI DaemonSet shared by both revisions
					Expect(dsList.Items).To(HaveLen(1), "Should have exactly one IstioCNI DaemonSet")
					Success("Single IstioCNI DaemonSet shared by both revisions")
				})

				It("should have only one ZTunnel DaemonSet", func(ctx SpecContext) {
					// List all DaemonSets in ZTunnel namespace
					dsList := &appsv1.DaemonSetList{}
					Expect(cl.List(ctx, dsList, client.InNamespace(ztunnelNamespace))).To(Succeed())

					// Should have exactly one ZTunnel DaemonSet shared by both revisions
					Expect(dsList.Items).To(HaveLen(1), "Should have exactly one ZTunnel DaemonSet")
					Success("Single ZTunnel DaemonSet shared by both revisions")
				})

				It("should have IstioCNI and ZTunnel at new version", func(ctx SpecContext) {
					// Verify shared components were updated to new version
					cni := &v1.IstioCNI{}
					Expect(cl.Get(ctx, kube.Key("default"), cni)).To(Succeed())
					Expect(cni.Spec.Version).To(Equal(newVersion.Name))

					ztunnel := &v1.ZTunnel{}
					Expect(cl.Get(ctx, kube.Key("default"), ztunnel)).To(Succeed())
					Expect(ztunnel.Spec.Version).To(Equal(newVersion.Name))

					Success("Shared dependencies updated to new version")
				})
			})

			AfterAll(func(ctx SpecContext) {
				if CurrentSpecReport().Failed() {
					common.LogDebugInfo(common.Ambient, k)
					debugInfoLogged = true
					if keepOnFailure {
						return
					}
				}
				clr.Cleanup(ctx)
			})
		})
	})

	Describe("Lifecycle Transitions", func() {
		Context("Spec changes and finalizers", func() {
			clr := cleaner.New(cl)
			// Use the newer version for lifecycle testing
			testVersion := newVersion

			BeforeAll(func(ctx SpecContext) {
				clr.Record(ctx)
				Expect(k.CreateNamespace(controlPlaneNamespace)).To(Succeed())
				Expect(k.CreateNamespace(istioCniNamespace)).To(Succeed())
				Expect(k.CreateNamespace(ztunnelNamespace)).To(Succeed())

				// Create IstioCNI and Istio control plane so ZTunnel can become Ready
				cniYAML := fmt.Sprintf(`
apiVersion: sailoperator.io/v1
kind: IstioCNI
metadata:
  name: default
spec:
  version: %s
  namespace: %s
  profile: ambient`, testVersion.Name, istioCniNamespace)
				Expect(k.CreateFromString(cniYAML)).To(Succeed())

				istioYAML := fmt.Sprintf(`
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: %s
spec:
  version: %s
  namespace: %s
  values:
    profile: ambient
    pilot:
      cni:
        enabled: true
      trustedZtunnelNamespace: ztunnel`, istioName, testVersion.Name, controlPlaneNamespace)
				Expect(k.CreateFromString(istioYAML)).To(Succeed())

				// Wait for IstioCNI and Istio to be ready
				common.AwaitCondition(ctx, v1.IstioCNIConditionReady, kube.Key("default"), &v1.IstioCNI{}, k, cl, 180*time.Second)
				common.AwaitCondition(ctx, v1.IstioConditionReady, kube.Key(istioName), &v1.Istio{}, k, cl, 240*time.Second)

				Success("Control plane and IstioCNI created for lifecycle tests")
			})

			When("spec.values is updated on ZTunnel", func() {
				BeforeAll(func(ctx SpecContext) {
					ztunnelYAML := fmt.Sprintf(`
apiVersion: sailoperator.io/v1
kind: ZTunnel
metadata:
  name: default
spec:
  version: %s
  namespace: %s`, testVersion.Name, ztunnelNamespace)
					Expect(k.CreateFromString(ztunnelYAML)).To(Succeed())

					// Wait for it to be ready
					Eventually(func(g Gomega) {
						ztunnel := &v1.ZTunnel{}
						g.Expect(cl.Get(ctx, kube.Key("default"), ztunnel)).To(Succeed())
						g.Expect(ztunnel).To(HaveConditionStatus(v1.ZTunnelConditionReady, metav1.ConditionTrue))
					}).Should(Succeed(), "ZTunnel should become Ready")
					Success("ZTunnel created and ready")
				})

				It("should update DaemonSet when values change", func(ctx SpecContext) {
					// Update the spec.values
					ztunnel := &v1.ZTunnel{}
					Expect(cl.Get(ctx, kube.Key("default"), ztunnel)).To(Succeed())

					// Add a custom environment variable
					if ztunnel.Spec.Values == nil {
						ztunnel.Spec.Values = &v1.ZTunnelValues{}
					}
					if ztunnel.Spec.Values.ZTunnel == nil {
						ztunnel.Spec.Values.ZTunnel = &v1.ZTunnelConfig{}
					}
					if ztunnel.Spec.Values.ZTunnel.Env == nil {
						ztunnel.Spec.Values.ZTunnel.Env = make(map[string]string)
					}
					ztunnel.Spec.Values.ZTunnel.Env["TEST_VAR"] = "test-value"

					Log("Updating ZTunnel spec.values")
					Expect(cl.Update(ctx, ztunnel)).To(Succeed())

					// Verify the DaemonSet is updated with the new env var
					Eventually(func(g Gomega) {
						ds := &appsv1.DaemonSet{}
						g.Expect(cl.Get(ctx, kube.Key("ztunnel", ztunnelNamespace), ds)).To(Succeed())

						// Check if the env var exists in the DaemonSet
						found := false
						for _, container := range ds.Spec.Template.Spec.Containers {
							for _, env := range container.Env {
								if env.Name == "TEST_VAR" && env.Value == "test-value" {
									found = true
									break
								}
							}
						}
						g.Expect(found).To(BeTrue(), "TEST_VAR should be in DaemonSet")
					}).WithTimeout(120*time.Second).Should(Succeed(), "DaemonSet should be updated with new values")
					Success("ZTunnel DaemonSet updated after spec.values change")
				})
			})

			When("CR is deleted", func() {
				It("should have finalizer that prevents immediate deletion", func(ctx SpecContext) {
					// Check that ZTunnel has finalizers
					ztunnel := &v1.ZTunnel{}
					Expect(cl.Get(ctx, kube.Key("default"), ztunnel)).To(Succeed())
					Expect(ztunnel.Finalizers).NotTo(BeEmpty(), "ZTunnel should have finalizers")
					Success("ZTunnel has finalizers")
				})

				It("should cleanup resources when deleted", func(ctx SpecContext) {
					// Delete the ZTunnel CR
					ztunnel := &v1.ZTunnel{}
					Expect(cl.Get(ctx, kube.Key("default"), ztunnel)).To(Succeed())

					Log("Deleting ZTunnel CR")
					Expect(cl.Delete(ctx, ztunnel)).To(Succeed())

					// Verify the DaemonSet is deleted
					Eventually(func(g Gomega) {
						ds := &appsv1.DaemonSet{}
						err := cl.Get(ctx, kube.Key("ztunnel", ztunnelNamespace), ds)
						g.Expect(err).To(HaveOccurred())
						g.Expect(err.Error()).To(ContainSubstring("not found"))
					}).WithTimeout(120*time.Second).Should(Succeed(), "DaemonSet should be deleted")

					// Verify the CR is fully deleted
					Eventually(func(g Gomega) {
						zt := &v1.ZTunnel{}
						err := cl.Get(ctx, kube.Key("default"), zt)
						g.Expect(err).To(HaveOccurred())
						g.Expect(err.Error()).To(ContainSubstring("not found"))
					}).WithTimeout(60*time.Second).Should(Succeed(), "ZTunnel CR should be fully deleted")
					Success("ZTunnel CR and resources cleaned up successfully")
				})
			})

			AfterAll(func(ctx SpecContext) {
				if CurrentSpecReport().Failed() {
					common.LogDebugInfo(common.Ambient, k)
					debugInfoLogged = true
					if keepOnFailure {
						return
					}
				}
				clr.Cleanup(ctx)
			})
		})
	})

	AfterAll(func() {
		if CurrentSpecReport().Failed() {
			if !debugInfoLogged {
				common.LogDebugInfo(common.Ambient, k)
				debugInfoLogged = true

				if keepOnFailure {
					return
				}
			}
		}
	})
})
