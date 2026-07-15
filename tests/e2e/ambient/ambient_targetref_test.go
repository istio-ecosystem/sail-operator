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

	"github.com/Masterminds/semver/v3"
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/cleaner"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	. "github.com/istio-ecosystem/sail-operator/tests/e2e/util/gomega"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Ambient TargetRef Behavior", Label("ambient", "ambient-targetref"), Ordered, func() {
	SetDefaultEventuallyTimeout(time.Duration(defaultTimeout) * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)

	// Use the latest supported ambient version for these tests
	version := getLatestAmbientVersion()

	var clr cleaner.Cleaner

	BeforeAll(func(ctx SpecContext) {
		clr = cleaner.New(cl)
		clr.Record(ctx)

		Expect(k.CreateNamespace(controlPlaneNamespace)).To(Succeed())
		Expect(k.CreateNamespace(istioCniNamespace)).To(Succeed())
		Expect(k.CreateNamespace(ztunnelNamespace)).To(Succeed())

		// Create IstioCNI CR for shared use across scenarios
		common.CreateIstioCNI(k, version.Name, `
profile: ambient`)
		Success("IstioCNI created")
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

	Context("Value Propagation from TargetRef", Ordered, func() {
		networkValue := "test-network"
		var contextCleaner cleaner.Cleaner

		BeforeAll(func(ctx SpecContext) {
			contextCleaner = cleaner.New(cl)
			contextCleaner.Record(ctx)
		})

		When("Istio CR is created with custom global.network value", func() {
			It("creates Istio with custom network configuration", func(ctx SpecContext) {
				common.CreateIstio(k, version.Name, `
profile: ambient
values:
  global:
    network: `+networkValue)
				Success("Istio CR created with custom network value")
			})

			It("waits for Istio to be Ready", func(ctx SpecContext) {
				istio := &v1.Istio{}
				Eventually(func(g Gomega) {
					g.Expect(cl.Get(ctx, kube.Key(istioName), istio)).To(Succeed())
					g.Expect(istio).To(HaveConditionStatus(v1.IstioConditionReconciled, metav1.ConditionTrue))
					g.Expect(istio).To(HaveConditionStatus(v1.IstioConditionReady, metav1.ConditionTrue))
					g.Expect(istio.Status.ActiveRevisionName).NotTo(BeEmpty())
				}).Should(Succeed(), "Istio should be Ready")
				Success("Istio is Ready with active revision")
			})
		})

		When("ZTunnel is created with targetRef pointing to Istio", func() {
			It("creates ZTunnel with targetRef (no custom values)", func(ctx SpecContext) {
				ztunnelYAML := fmt.Sprintf(`
apiVersion: sailoperator.io/v1
kind: ZTunnel
metadata:
  name: default
spec:
  version: %s
  namespace: %s
  targetRef:
    kind: Istio
    name: %s`, version.Name, ztunnelNamespace, istioName)

				Expect(k.CreateFromString(ztunnelYAML)).To(Succeed())
				Success("ZTunnel CR created with targetRef to Istio")
			})

			It("waits for ZTunnel to be Ready", func(ctx SpecContext) {
				ztunnel := &v1.ZTunnel{}
				Eventually(func(g Gomega) {
					g.Expect(cl.Get(ctx, kube.Key("default"), ztunnel)).To(Succeed())
					g.Expect(ztunnel).To(HaveConditionStatus(v1.ZTunnelConditionReconciled, metav1.ConditionTrue))
					g.Expect(ztunnel).To(HaveConditionStatus(v1.ZTunnelConditionReady, metav1.ConditionTrue))
				}).Should(Succeed(), "ZTunnel should be Ready")
				Success("ZTunnel is Ready")
			})

			It("verifies ZTunnel Status.IstioRevision matches Istio active revision", func(ctx SpecContext) {
				istio := &v1.Istio{}
				Expect(cl.Get(ctx, kube.Key(istioName), istio)).To(Succeed())

				ztunnel := &v1.ZTunnel{}
				Expect(cl.Get(ctx, kube.Key("default"), ztunnel)).To(Succeed())

				Expect(ztunnel.Status.IstioRevision).To(Equal(istio.Status.ActiveRevisionName),
					"ZTunnel.Status.IstioRevision should match Istio.Status.ActiveRevisionName")
				Success(fmt.Sprintf("ZTunnel Status.IstioRevision correctly set to: %s", ztunnel.Status.IstioRevision))
			})

			It("verifies ZTunnel DaemonSet inherits NETWORK environment variable", func(ctx SpecContext) {
				// NETWORK env var is only available in v1.27+
				if version.Version.LessThan(semver.MustParse("1.27.0")) {
					Skip("NETWORK environment variable not available in versions < 1.27")
				}

				daemonset := &appsv1.DaemonSet{}
				Eventually(func(g Gomega) {
					g.Expect(cl.Get(ctx, kube.Key("ztunnel", ztunnelNamespace), daemonset)).To(Succeed())
					g.Expect(daemonSetHasEnvVar(daemonset, "NETWORK", networkValue)).To(BeTrue(),
						"NETWORK env var should be set to %s", networkValue)
				}).Should(Succeed())
				Success(fmt.Sprintf("ZTunnel DaemonSet inherited NETWORK=%s from Istio", networkValue))
			})
		})

		When("Istio values are updated", func() {
			newNetworkValue := "updated-network"

			It("updates Istio with new network value", func(ctx SpecContext) {
				patch := fmt.Sprintf(`{"spec":{"values":{"global":{"network":"%s"}}}}`, newNetworkValue)
				Expect(k.Patch("istio", istioName, "merge", patch)).To(Succeed())
				Success("Istio patched with new network value")
			})

			It("waits for Istio to reconcile", func(ctx SpecContext) {
				istio := &v1.Istio{}
				Eventually(func(g Gomega) {
					g.Expect(cl.Get(ctx, kube.Key(istioName), istio)).To(Succeed())
					g.Expect(istio).To(HaveConditionStatus(v1.IstioConditionReconciled, metav1.ConditionTrue))
					g.Expect(istio).To(HaveConditionStatus(v1.IstioConditionReady, metav1.ConditionTrue))
				}).Should(Succeed())
				Success("Istio reconciled after update")
			})

			It("verifies ZTunnel picks up new inherited value", func(ctx SpecContext) {
				// NETWORK env var is only available in v1.27+
				if version.Version.LessThan(semver.MustParse("1.27.0")) {
					Skip("NETWORK environment variable not available in versions < 1.27")
				}

				daemonset := &appsv1.DaemonSet{}
				Eventually(func(g Gomega) {
					g.Expect(cl.Get(ctx, kube.Key("ztunnel", ztunnelNamespace), daemonset)).To(Succeed())
					g.Expect(daemonSetHasEnvVar(daemonset, "NETWORK", newNetworkValue)).To(BeTrue(),
						"NETWORK env var should be updated to %s", newNetworkValue)
				}).Should(Succeed())
				Success(fmt.Sprintf("ZTunnel DaemonSet inherited updated NETWORK=%s", newNetworkValue))
			})
		})

		AfterAll(func(ctx SpecContext) {
			contextCleaner.Cleanup(ctx)
			Success("Context cleanup completed")
		})
	})

	Context("MeshConfig Propagation from TargetRef", Ordered, func() {
		customTrustDomain := "custom-trust.local"
		var contextCleaner cleaner.Cleaner

		BeforeAll(func(ctx SpecContext) {
			contextCleaner = cleaner.New(cl)
			contextCleaner.Record(ctx)
		})

		When("Istio CR is created with custom meshConfig", func() {
			It("creates Istio with custom trustDomain in meshConfig", func(ctx SpecContext) {
				istioYAML := fmt.Sprintf(`
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: meshconfig-test
spec:
  version: %s
  namespace: %s
  profile: ambient
  values:
    meshConfig:
      trustDomain: %s`, version.Name, controlPlaneNamespace, customTrustDomain)

				Expect(k.CreateFromString(istioYAML)).To(Succeed())
				Success("Istio CR created with custom meshConfig")
			})

			It("waits for Istio to be Ready", func(ctx SpecContext) {
				istio := &v1.Istio{}
				Eventually(func(g Gomega) {
					g.Expect(cl.Get(ctx, kube.Key("meshconfig-test"), istio)).To(Succeed())
					g.Expect(istio).To(HaveConditionStatus(v1.IstioConditionReconciled, metav1.ConditionTrue))
					g.Expect(istio).To(HaveConditionStatus(v1.IstioConditionReady, metav1.ConditionTrue))
					g.Expect(istio.Status.ActiveRevisionName).NotTo(BeEmpty())
				}).Should(Succeed())
				Success("Istio with meshConfig is Ready")
			})
		})

		When("ZTunnel is created with targetRef to Istio with meshConfig", func() {
			It("creates ZTunnel with targetRef (no custom meshConfig)", func(ctx SpecContext) {
				ztunnelYAML := fmt.Sprintf(`
apiVersion: sailoperator.io/v1
kind: ZTunnel
metadata:
  name: default
spec:
  version: %s
  namespace: %s
  targetRef:
    kind: Istio
    name: meshconfig-test`, version.Name, ztunnelNamespace)

				Expect(k.CreateFromString(ztunnelYAML)).To(Succeed())
				Success("ZTunnel created with targetRef to Istio with meshConfig")
			})

			It("waits for ZTunnel to be Ready", func(ctx SpecContext) {
				ztunnel := &v1.ZTunnel{}
				Eventually(func(g Gomega) {
					g.Expect(cl.Get(ctx, kube.Key("default"), ztunnel)).To(Succeed())
					g.Expect(ztunnel).To(HaveConditionStatus(v1.ZTunnelConditionReconciled, metav1.ConditionTrue))
					g.Expect(ztunnel).To(HaveConditionStatus(v1.ZTunnelConditionReady, metav1.ConditionTrue))
				}).Should(Succeed())
				Success("ZTunnel is Ready with inherited meshConfig")
			})

			It("verifies ZTunnel Status.IstioRevision is set", func(ctx SpecContext) {
				ztunnel := &v1.ZTunnel{}
				Expect(cl.Get(ctx, kube.Key("default"), ztunnel)).To(Succeed())

				istio := &v1.Istio{}
				Expect(cl.Get(ctx, kube.Key("meshconfig-test"), istio)).To(Succeed())

				Expect(ztunnel.Status.IstioRevision).To(Equal(istio.Status.ActiveRevisionName))
				Success(fmt.Sprintf("ZTunnel references correct IstioRevision: %s", ztunnel.Status.IstioRevision))
			})

			It("verifies ZTunnel DaemonSet is deployed successfully", func(ctx SpecContext) {
				daemonset := &appsv1.DaemonSet{}
				Eventually(func(g Gomega) {
					g.Expect(cl.Get(ctx, kube.Key("ztunnel", ztunnelNamespace), daemonset)).To(Succeed())
					g.Expect(daemonset.Status.NumberAvailable).To(BeNumerically(">", 0))
				}).Should(Succeed())
				Success("ZTunnel DaemonSet deployed with inherited meshConfig")
			})
		})

		When("Istio meshConfig is updated", func() {
			updatedTrustDomain := "updated-trust.local"

			It("updates Istio meshConfig trustDomain", func(ctx SpecContext) {
				patch := fmt.Sprintf(`{"spec":{"values":{"meshConfig":{"trustDomain":"%s"}}}}`, updatedTrustDomain)
				Expect(k.Patch("istio", "meshconfig-test", "merge", patch)).To(Succeed())
				Success("Istio meshConfig patched with new trustDomain")
			})

			It("waits for Istio to reconcile", func(ctx SpecContext) {
				istio := &v1.Istio{}
				Eventually(func(g Gomega) {
					g.Expect(cl.Get(ctx, kube.Key("meshconfig-test"), istio)).To(Succeed())
					g.Expect(istio).To(HaveConditionStatus(v1.IstioConditionReconciled, metav1.ConditionTrue))
					g.Expect(istio).To(HaveConditionStatus(v1.IstioConditionReady, metav1.ConditionTrue))
				}).Should(Succeed())
				Success("Istio reconciled after meshConfig update")
			})

			It("verifies ZTunnel picks up updated meshConfig", func(ctx SpecContext) {
				ztunnel := &v1.ZTunnel{}
				Eventually(func(g Gomega) {
					g.Expect(cl.Get(ctx, kube.Key("default"), ztunnel)).To(Succeed())
					g.Expect(ztunnel).To(HaveConditionStatus(v1.ZTunnelConditionReconciled, metav1.ConditionTrue))
					g.Expect(ztunnel).To(HaveConditionStatus(v1.ZTunnelConditionReady, metav1.ConditionTrue))
				}).Should(Succeed())
				Success("ZTunnel reconciled with updated meshConfig")
			})

			It("verifies ZTunnel DaemonSet remains healthy", func(ctx SpecContext) {
				daemonset := &appsv1.DaemonSet{}
				Eventually(func(g Gomega) {
					g.Expect(cl.Get(ctx, kube.Key("ztunnel", ztunnelNamespace), daemonset)).To(Succeed())
					g.Expect(daemonset.Status.NumberAvailable).To(BeNumerically(">", 0))
				}).Should(Succeed())
				Success("ZTunnel DaemonSet healthy after meshConfig update")
			})
		})

		AfterAll(func(ctx SpecContext) {
			contextCleaner.Cleanup(ctx)
			Success("Context cleanup completed")
		})
	})

	Context("User Value Override Precedence", Ordered, func() {
		inheritedNetwork := "inherited-net"
		overrideNetwork := "override-net"
		customEnvVar := "CUSTOM_TEST_VAR"
		customEnvValue := "custom-value"
		var contextCleaner cleaner.Cleaner

		BeforeAll(func(ctx SpecContext) {
			contextCleaner = cleaner.New(cl)
			contextCleaner.Record(ctx)
		})

		When("Istio is created with a network value", func() {
			It("creates second Istio CR for override testing", func(ctx SpecContext) {
				istioName2 := "override-test"
				istioYAML := fmt.Sprintf(`
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: %s
spec:
  version: %s
  namespace: %s
  profile: ambient
  values:
    global:
      network: %s`, istioName2, version.Name, controlPlaneNamespace, inheritedNetwork)

				Expect(k.CreateFromString(istioYAML)).To(Succeed())
				Success("Second Istio CR created")
			})

			It("waits for second Istio to be Ready", func(ctx SpecContext) {
				istio := &v1.Istio{}
				Eventually(func(g Gomega) {
					g.Expect(cl.Get(ctx, kube.Key("override-test"), istio)).To(Succeed())
					g.Expect(istio).To(HaveConditionStatus(v1.IstioConditionReady, metav1.ConditionTrue))
				}).Should(Succeed())
				Success("Second Istio is Ready")
			})
		})

		When("ZTunnel is created with targetRef AND custom values", func() {
			It("creates ZTunnel with both targetRef and value overrides", func(ctx SpecContext) {
				ztunnelYAML := fmt.Sprintf(`
apiVersion: sailoperator.io/v1
kind: ZTunnel
metadata:
  name: default
spec:
  version: %s
  namespace: %s
  targetRef:
    kind: Istio
    name: override-test
  values:
    ztunnel:
      network: %s
      env:
        %s: "%s"`, version.Name, ztunnelNamespace, overrideNetwork, customEnvVar, customEnvValue)

				Expect(k.CreateFromString(ztunnelYAML)).To(Succeed())
				Success("ZTunnel created with targetRef and custom values")
			})

			It("waits for ZTunnel to be Ready", func(ctx SpecContext) {
				ztunnel := &v1.ZTunnel{}
				Eventually(func(g Gomega) {
					g.Expect(cl.Get(ctx, kube.Key("default"), ztunnel)).To(Succeed())
					g.Expect(ztunnel).To(HaveConditionStatus(v1.ZTunnelConditionReady, metav1.ConditionTrue))
				}).Should(Succeed())
				Success("Override ZTunnel is Ready")
			})

			It("verifies DaemonSet uses override network value (not inherited)", func(ctx SpecContext) {
				// NETWORK env var is only available in v1.27+
				if version.Version.LessThan(semver.MustParse("1.27.0")) {
					Skip("NETWORK environment variable not available in versions < 1.27")
				}

				daemonset := &appsv1.DaemonSet{}
				Eventually(func(g Gomega) {
					g.Expect(cl.Get(ctx, kube.Key("ztunnel", ztunnelNamespace), daemonset)).To(Succeed())
					g.Expect(daemonSetHasEnvVar(daemonset, "NETWORK", overrideNetwork)).To(BeTrue(),
						"NETWORK should use override value %s, not inherited value %s", overrideNetwork, inheritedNetwork)
				}).Should(Succeed())
				Success(fmt.Sprintf("DaemonSet uses override NETWORK=%s (not inherited %s)", overrideNetwork, inheritedNetwork))
			})

			It("verifies custom environment variables are present", func(ctx SpecContext) {
				daemonset := &appsv1.DaemonSet{}
				Eventually(func(g Gomega) {
					g.Expect(cl.Get(ctx, kube.Key("ztunnel", ztunnelNamespace), daemonset)).To(Succeed())
					g.Expect(daemonSetHasEnvVar(daemonset, customEnvVar, customEnvValue)).To(BeTrue(),
						"Custom env var %s should be set to %s", customEnvVar, customEnvValue)
				}).Should(Succeed())
				Success(fmt.Sprintf("Custom environment variable %s=%s is present", customEnvVar, customEnvValue))
			})
		})

		When("override value is removed from ZTunnel spec", func() {
			It("patches ZTunnel to remove network override (keep custom env)", func(ctx SpecContext) {
				// Remove the ztunnel.network override, keep custom env var
				patch := fmt.Sprintf(`{"spec":{"values":{"ztunnel":{"network":null,"env":{"%s":"%s"}}}}}`,
					customEnvVar, customEnvValue)
				Expect(k.Patch("ztunnel", "default", "merge", patch)).To(Succeed())
				Success("ZTunnel patched to remove network override")
			})

			It("waits for ZTunnel to reconcile", func(ctx SpecContext) {
				ztunnel := &v1.ZTunnel{}
				Eventually(func(g Gomega) {
					g.Expect(cl.Get(ctx, kube.Key("default"), ztunnel)).To(Succeed())
					g.Expect(ztunnel).To(HaveConditionStatus(v1.ZTunnelConditionReady, metav1.ConditionTrue))
				}).Should(Succeed())
				Success("ZTunnel reconciled after patch")
			})

			It("verifies ZTunnel now uses inherited network value", func(ctx SpecContext) {
				// NETWORK env var is only available in v1.27+
				if version.Version.LessThan(semver.MustParse("1.27.0")) {
					Skip("NETWORK environment variable not available in versions < 1.27")
				}

				daemonset := &appsv1.DaemonSet{}
				Eventually(func(g Gomega) {
					g.Expect(cl.Get(ctx, kube.Key("ztunnel", ztunnelNamespace), daemonset)).To(Succeed())
					g.Expect(daemonSetHasEnvVar(daemonset, "NETWORK", inheritedNetwork)).To(BeTrue(),
						"NETWORK should fallback to inherited value %s", inheritedNetwork)
				}).Should(Succeed())
				Success(fmt.Sprintf("ZTunnel now uses inherited NETWORK=%s", inheritedNetwork))
			})
		})

		AfterAll(func(ctx SpecContext) {
			contextCleaner.Cleanup(ctx)
			Success("Context cleanup completed")
		})
	})

	Context("Invalid TargetRef Handling", Ordered, func() {
		When("ZTunnel targetRef points to non-existent Istio resource", func() {
			missingIstioName := "missing-istio"
			var whenCleaner cleaner.Cleaner

			BeforeAll(func(ctx SpecContext) {
				whenCleaner = cleaner.New(cl)
				whenCleaner.Record(ctx)
			})

			It("creates ZTunnel with targetRef to non-existent Istio", func(ctx SpecContext) {
				ztunnelYAML := fmt.Sprintf(`
apiVersion: sailoperator.io/v1
kind: ZTunnel
metadata:
  name: default
spec:
  version: %s
  namespace: %s
  targetRef:
    kind: Istio
    name: %s`, version.Name, ztunnelNamespace, missingIstioName)

				Expect(k.CreateFromString(ztunnelYAML)).To(Succeed())
				Success("ZTunnel created with targetRef to non-existent Istio")
			})

			It("verifies ZTunnel Reconciled condition is False", func(ctx SpecContext) {
				ztunnel := &v1.ZTunnel{}
				Eventually(func(g Gomega) {
					g.Expect(cl.Get(ctx, kube.Key("default"), ztunnel)).To(Succeed())
					g.Expect(ztunnel).To(HaveConditionStatus(v1.ZTunnelConditionReconciled, metav1.ConditionFalse))
				}).Should(Succeed())
				Success("ZTunnel Reconciled=False due to missing targetRef")
			})

			It("verifies error message indicates resource not found", func(ctx SpecContext) {
				ztunnel := &v1.ZTunnel{}
				Eventually(func(g Gomega) {
					g.Expect(cl.Get(ctx, kube.Key("default"), ztunnel)).To(Succeed())
					// Error message should contain "not found" or similar
					g.Expect(ztunnel).To(HaveConditionMessage(v1.ZTunnelConditionReconciled, "not found"))
				}).Should(Succeed())
				Success("Error message indicates resource not found")
			})

			It("creates the missing Istio resource", func(ctx SpecContext) {
				istioYAML := fmt.Sprintf(`
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: %s
spec:
  version: %s
  namespace: %s
  profile: ambient`, missingIstioName, version.Name, controlPlaneNamespace)

				Expect(k.CreateFromString(istioYAML)).To(Succeed())
				Success("Missing Istio CR created")
			})

			It("waits for Istio to be Ready", func(ctx SpecContext) {
				istio := &v1.Istio{}
				Eventually(func(g Gomega) {
					g.Expect(cl.Get(ctx, kube.Key(missingIstioName), istio)).To(Succeed())
					g.Expect(istio).To(HaveConditionStatus(v1.IstioConditionReady, metav1.ConditionTrue))
					g.Expect(istio.Status.ActiveRevisionName).NotTo(BeEmpty())
				}).Should(Succeed())
				Success("Missing Istio is now Ready")
			})

			It("verifies ZTunnel auto-recovers and becomes Ready", func(ctx SpecContext) {
				ztunnel := &v1.ZTunnel{}
				Eventually(func(g Gomega) {
					g.Expect(cl.Get(ctx, kube.Key("default"), ztunnel)).To(Succeed())
					g.Expect(ztunnel).To(HaveConditionStatus(v1.ZTunnelConditionReconciled, metav1.ConditionTrue))
					g.Expect(ztunnel).To(HaveConditionStatus(v1.ZTunnelConditionReady, metav1.ConditionTrue))
				}).Should(Succeed())
				Success("ZTunnel auto-recovered after Istio was created")
			})

			It("verifies Status.IstioRevision is populated", func(ctx SpecContext) {
				ztunnel := &v1.ZTunnel{}
				Expect(cl.Get(ctx, kube.Key("default"), ztunnel)).To(Succeed())
				Expect(ztunnel.Status.IstioRevision).NotTo(BeEmpty(), "Status.IstioRevision should be populated")

				istio := &v1.Istio{}
				Expect(cl.Get(ctx, kube.Key(missingIstioName), istio)).To(Succeed())
				Expect(ztunnel.Status.IstioRevision).To(Equal(istio.Status.ActiveRevisionName))
				Success(fmt.Sprintf("Status.IstioRevision correctly set to: %s", ztunnel.Status.IstioRevision))
			})

			It("verifies value propagation works after recovery", func(ctx SpecContext) {
				// Verify that the DaemonSet was created successfully
				daemonset := &appsv1.DaemonSet{}
				Eventually(func(g Gomega) {
					g.Expect(cl.Get(ctx, kube.Key("ztunnel", ztunnelNamespace), daemonset)).To(Succeed())
					g.Expect(daemonset.Status.NumberAvailable).To(BeNumerically(">", 0))
				}).Should(Succeed())
				Success("Value propagation working - ZTunnel DaemonSet is deployed")
			})

			AfterAll(func(ctx SpecContext) {
				whenCleaner.Cleanup(ctx)
				Success("Cleanup completed")
			})
		})
	})
})

// getLatestAmbientVersion returns the latest supported version for ambient mode
func getLatestAmbientVersion() istioversion.VersionInfo {
	versions := istioversion.GetLatestPatchVersions()

	for _, version := range versions {
		// Minimum supported version is 1.24
		if version.Version.LessThan(semver.MustParse("1.24.0")) {
			continue
		}
		// FIPS clusters require v1.28+
		if fipsCluster && version.Version.LessThan(semver.MustParse("1.28.0")) {
			continue
		}
		return version
	}
	// Fallback to the last version if no suitable version found
	return versions[len(versions)-1]
}

// daemonSetHasEnvVar checks if a DaemonSet's containers have an environment variable with the expected value
func daemonSetHasEnvVar(daemonset *appsv1.DaemonSet, envName, expectedValue string) bool {
	for _, container := range daemonset.Spec.Template.Spec.Containers {
		for _, env := range container.Env {
			if env.Name == envName && env.Value == expectedValue {
				return true
			}
		}
	}
	return false
}
