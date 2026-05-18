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

var _ = Describe("Ambient Dependency Management", Label("ambient", "ambient-dependency", "smoke"), Ordered, func() {
	SetDefaultEventuallyTimeout(time.Duration(defaultTimeout) * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)

	Describe("Dependency Detection and Health Propagation", func() {
		for _, version := range istioversion.GetLatestPatchVersions() {
			// The minimum supported version for ambient is 1.24 (and above)
			if version.Version.LessThan(semver.MustParse("1.24.0")) {
				continue
			}

			// FIPS clusters do not support ambient mode for versions below 1.28
			if fipsCluster && version.Version.LessThan(semver.MustParse("1.28.0")) {
				continue
			}

			Context(fmt.Sprintf("Istio version %s", version.Version), func() {
				clr := cleaner.New(cl)

				BeforeAll(func(ctx SpecContext) {
					clr.Record(ctx)
					Expect(k.CreateNamespace(controlPlaneNamespace)).To(Succeed())
					Expect(k.CreateNamespace(istioCniNamespace)).To(Succeed())
					Expect(k.CreateNamespace(ztunnelNamespace)).To(Succeed())
				})

				When("IstioRevision with ambient profile is created before IstioCNI exists", func() {
					BeforeAll(func() {
						// Create IstioRevision with ambient profile (depends on IstioCNI)
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
      trustedZtunnelNamespace: ztunnel`, istioName, version.Name, controlPlaneNamespace)
						Log("Creating Istio CR with ambient profile before IstioCNI exists:", istioYAML)
						Expect(k.CreateFromString(istioYAML)).To(Succeed())
						Success("Istio CR created")
					})

					It("should show IstioCNINotFound reason in DependenciesHealthy condition", func(ctx SpecContext) {
						// Wait for the IstioRevision to be created (Istio creates a revision)
						Eventually(func(g Gomega) {
							istio := &v1.Istio{}
							g.Expect(cl.Get(ctx, kube.Key(istioName), istio)).To(Succeed())
							g.Expect(istio.Status.ActiveRevisionName).NotTo(BeEmpty(), "Active revision not set yet")
						}).Should(Succeed(), "Waiting for Istio to create active revision")

						// Get the active revision name
						istio := &v1.Istio{}
						Expect(cl.Get(ctx, kube.Key(istioName), istio)).To(Succeed())
						revisionName := istio.Status.ActiveRevisionName

						// Check that DependenciesHealthy condition shows IstioCNINotFound
						Eventually(func(g Gomega) {
							revision := &v1.IstioRevision{}
							g.Expect(cl.Get(ctx, kube.Key(revisionName, controlPlaneNamespace), revision)).To(Succeed())
							g.Expect(revision).To(HaveConditionStatus(v1.IstioRevisionConditionDependenciesHealthy, metav1.ConditionFalse))

							// Check the reason is IstioCNINotFound
							for _, cond := range revision.Status.Conditions {
								if cond.Type == v1.IstioRevisionConditionDependenciesHealthy {
									g.Expect(cond.Reason).To(Equal(v1.IstioRevisionReasonIstioCNINotFound),
										fmt.Sprintf("Expected reason IstioCNINotFound, got: %s", cond.Reason))
									break
								}
							}
						}).Should(Succeed(), "IstioRevision should show IstioCNINotFound")
						Success("IstioRevision correctly shows IstioCNINotFound")
					})
				})

				When("IstioCNI is created", func() {
					BeforeAll(func() {
						cniYAML := fmt.Sprintf(`
apiVersion: sailoperator.io/v1
kind: IstioCNI
metadata:
  name: default
spec:
  version: %s
  namespace: %s
  profile: ambient`, version.Name, istioCniNamespace)
						Log("Creating IstioCNI CR:", cniYAML)
						Expect(k.CreateFromString(cniYAML)).To(Succeed())
						Success("IstioCNI created")
					})

					It("should transition from IstioCNINotFound to ZTunnelNotFound", func(ctx SpecContext) {
						// Wait for IstioCNI to be Ready
						common.AwaitCondition(ctx, v1.IstioCNIConditionReady, kube.Key("default"), &v1.IstioCNI{}, k, cl)

						// Get the active revision name
						istio := &v1.Istio{}
						Expect(cl.Get(ctx, kube.Key(istioName), istio)).To(Succeed())
						revisionName := istio.Status.ActiveRevisionName

						// Now check that IstioRevision shows ZTunnelNotFound (next missing dependency)
						Eventually(func(g Gomega) {
							revision := &v1.IstioRevision{}
							g.Expect(cl.Get(ctx, kube.Key(revisionName, controlPlaneNamespace), revision)).To(Succeed())
							g.Expect(revision).To(HaveConditionStatus(v1.IstioRevisionConditionDependenciesHealthy, metav1.ConditionFalse))

							// Check the reason is ZTunnelNotFound
							for _, cond := range revision.Status.Conditions {
								if cond.Type == v1.IstioRevisionConditionDependenciesHealthy {
									g.Expect(cond.Reason).To(Equal(v1.IstioRevisionReasonZTunnelNotFound),
										fmt.Sprintf("Expected reason ZTunnelNotFound, got: %s", cond.Reason))
									break
								}
							}
						}).Should(Succeed(), "IstioRevision should show ZTunnelNotFound")
						Success("IstioRevision correctly transitioned to ZTunnelNotFound")
					})
				})

				When("ZTunnel is created", func() {
					BeforeAll(func() {
						ztunnelYAML := fmt.Sprintf(`
apiVersion: sailoperator.io/v1
kind: ZTunnel
metadata:
  name: default
spec:
  version: %s
  namespace: %s`, version.Name, ztunnelNamespace)
						Log("Creating ZTunnel CR:", ztunnelYAML)
						Expect(k.CreateFromString(ztunnelYAML)).To(Succeed())
						Success("ZTunnel created")
					})

					It("should make DependenciesHealthy condition True", func(ctx SpecContext) {
						// Wait for ZTunnel to be Ready
						common.AwaitCondition(ctx, v1.ZTunnelConditionReady, kube.Key("default"), &v1.ZTunnel{}, k, cl)

						// Get the active revision name
						istio := &v1.Istio{}
						Expect(cl.Get(ctx, kube.Key(istioName), istio)).To(Succeed())
						revisionName := istio.Status.ActiveRevisionName

						// Now check that IstioRevision shows DependenciesHealthy = True
						Eventually(func(g Gomega) {
							revision := &v1.IstioRevision{}
							g.Expect(cl.Get(ctx, kube.Key(revisionName, controlPlaneNamespace), revision)).To(Succeed())
							g.Expect(revision).To(HaveConditionStatus(v1.IstioRevisionConditionDependenciesHealthy, metav1.ConditionTrue))
						}).Should(Succeed(), "IstioRevision should have DependenciesHealthy=True")
						Success("All dependencies are healthy")
					})

					It("should make IstioRevision Ready", func(ctx SpecContext) {
						istio := &v1.Istio{}
						Expect(cl.Get(ctx, kube.Key(istioName), istio)).To(Succeed())
						revisionName := istio.Status.ActiveRevisionName

						// Check that IstioRevision becomes Ready
						common.AwaitCondition(ctx, v1.IstioRevisionConditionReady, kube.Key(revisionName, controlPlaneNamespace), &v1.IstioRevision{}, k, cl)
						Success("IstioRevision is Ready")
					})
				})

				When("IstioCNI becomes unhealthy", func() {
					It("should make IstioRevision show IstioCNINotHealthy", func(ctx SpecContext) {
						// Scale the IstioCNI DaemonSet to 0 to make it unhealthy
						daemonset := &appsv1.DaemonSet{}
						Expect(cl.Get(ctx, kube.Key("istio-cni-node", istioCniNamespace), daemonset)).To(Succeed())

						originalReplicas := daemonset.Status.DesiredNumberScheduled
						Log("Scaling IstioCNI DaemonSet to 0 replicas")

						// Update the DaemonSet to have 0 desired replicas by setting nodeSelector that matches no nodes
						daemonset.Spec.Template.Spec.NodeSelector = map[string]string{
							"non-existent-label": "true",
						}
						Expect(cl.Update(ctx, daemonset)).To(Succeed())

						// Wait for IstioCNI to become not Ready
						Eventually(func(g Gomega) {
							cni := &v1.IstioCNI{}
							g.Expect(cl.Get(ctx, kube.Key("default"), cni)).To(Succeed())
							// IstioCNI should show as not Ready when DaemonSet has no pods
							g.Expect(cni).To(HaveConditionStatus(v1.IstioCNIConditionReady, metav1.ConditionFalse))
						}).WithTimeout(60*time.Second).Should(Succeed(), "IstioCNI should become not Ready")

						// Get the active revision name
						istio := &v1.Istio{}
						Expect(cl.Get(ctx, kube.Key(istioName), istio)).To(Succeed())
						revisionName := istio.Status.ActiveRevisionName

						// Check that IstioRevision shows IstioCNINotHealthy
						Eventually(func(g Gomega) {
							revision := &v1.IstioRevision{}
							g.Expect(cl.Get(ctx, kube.Key(revisionName, controlPlaneNamespace), revision)).To(Succeed())
							g.Expect(revision).To(HaveConditionStatus(v1.IstioRevisionConditionDependenciesHealthy, metav1.ConditionFalse))

							// Check the reason
							for _, cond := range revision.Status.Conditions {
								if cond.Type == v1.IstioRevisionConditionDependenciesHealthy {
									g.Expect(cond.Reason).To(Equal(v1.IstioRevisionReasonIstioCNINotHealthy),
										fmt.Sprintf("Expected reason IstioCNINotHealthy, got: %s", cond.Reason))
									break
								}
							}
						}).WithTimeout(60*time.Second).Should(Succeed(), "IstioRevision should show IstioCNINotHealthy")
						Success("IstioRevision correctly detected unhealthy IstioCNI")

						// Restore the DaemonSet
						Log("Restoring IstioCNI DaemonSet")
						Expect(cl.Get(ctx, kube.Key("istio-cni-node", istioCniNamespace), daemonset)).To(Succeed())
						daemonset.Spec.Template.Spec.NodeSelector = nil
						Expect(cl.Update(ctx, daemonset)).To(Succeed())

						// Wait for recovery
						Eventually(func(g Gomega) {
							ds := &appsv1.DaemonSet{}
							g.Expect(cl.Get(ctx, kube.Key("istio-cni-node", istioCniNamespace), ds)).To(Succeed())
							g.Expect(ds.Status.NumberAvailable).To(Equal(originalReplicas))
						}).WithTimeout(60*time.Second).Should(Succeed(), "DaemonSet should recover")
						Success("IstioCNI DaemonSet restored")
					})
				})

				When("ZTunnel becomes unhealthy", func() {
					It("should make IstioRevision show ZTunnelNotHealthy", func(ctx SpecContext) {
						// Scale the ZTunnel DaemonSet to 0 to make it unhealthy
						daemonset := &appsv1.DaemonSet{}
						Expect(cl.Get(ctx, kube.Key("ztunnel", ztunnelNamespace), daemonset)).To(Succeed())

						originalReplicas := daemonset.Status.DesiredNumberScheduled
						Log("Scaling ZTunnel DaemonSet to 0 replicas")

						// Update the DaemonSet to have 0 desired replicas by setting nodeSelector that matches no nodes
						daemonset.Spec.Template.Spec.NodeSelector = map[string]string{
							"non-existent-label": "true",
						}
						Expect(cl.Update(ctx, daemonset)).To(Succeed())

						// Wait for ZTunnel to become not Ready
						Eventually(func(g Gomega) {
							ztunnel := &v1.ZTunnel{}
							g.Expect(cl.Get(ctx, kube.Key("default"), ztunnel)).To(Succeed())
							// ZTunnel should show as not Ready when DaemonSet has no pods
							g.Expect(ztunnel).To(HaveConditionStatus(v1.ZTunnelConditionReady, metav1.ConditionFalse))
						}).WithTimeout(60*time.Second).Should(Succeed(), "ZTunnel should become not Ready")

						// Get the active revision name
						istio := &v1.Istio{}
						Expect(cl.Get(ctx, kube.Key(istioName), istio)).To(Succeed())
						revisionName := istio.Status.ActiveRevisionName

						// Check that IstioRevision shows ZTunnelNotHealthy
						Eventually(func(g Gomega) {
							revision := &v1.IstioRevision{}
							g.Expect(cl.Get(ctx, kube.Key(revisionName, controlPlaneNamespace), revision)).To(Succeed())
							g.Expect(revision).To(HaveConditionStatus(v1.IstioRevisionConditionDependenciesHealthy, metav1.ConditionFalse))

							// Check the reason
							for _, cond := range revision.Status.Conditions {
								if cond.Type == v1.IstioRevisionConditionDependenciesHealthy {
									g.Expect(cond.Reason).To(Equal(v1.IstioRevisionReasonZTunnelNotHealthy),
										fmt.Sprintf("Expected reason ZTunnelNotHealthy, got: %s", cond.Reason))
									break
								}
							}
						}).WithTimeout(60*time.Second).Should(Succeed(), "IstioRevision should show ZTunnelNotHealthy")
						Success("IstioRevision correctly detected unhealthy ZTunnel")

						// Restore the DaemonSet
						Log("Restoring ZTunnel DaemonSet")
						Expect(cl.Get(ctx, kube.Key("ztunnel", ztunnelNamespace), daemonset)).To(Succeed())
						daemonset.Spec.Template.Spec.NodeSelector = nil
						Expect(cl.Update(ctx, daemonset)).To(Succeed())

						// Wait for recovery
						Eventually(func(g Gomega) {
							ds := &appsv1.DaemonSet{}
							g.Expect(cl.Get(ctx, kube.Key("ztunnel", ztunnelNamespace), ds)).To(Succeed())
							g.Expect(ds.Status.NumberAvailable).To(Equal(originalReplicas))
						}).WithTimeout(60*time.Second).Should(Succeed(), "DaemonSet should recover")
						Success("ZTunnel DaemonSet restored")
					})
				})

				When("dependencies are restored", func() {
					It("should make IstioRevision healthy again", func(ctx SpecContext) {
						// Wait for all dependencies to become Ready
						common.AwaitCondition(ctx, v1.IstioCNIConditionReady, kube.Key("default"), &v1.IstioCNI{}, k, cl, 60*time.Second)
						common.AwaitCondition(ctx, v1.ZTunnelConditionReady, kube.Key("default"), &v1.ZTunnel{}, k, cl, 60*time.Second)

						// Get the active revision name
						istio := &v1.Istio{}
						Expect(cl.Get(ctx, kube.Key(istioName), istio)).To(Succeed())
						revisionName := istio.Status.ActiveRevisionName

						// Check that IstioRevision shows DependenciesHealthy = True again
						Eventually(func(g Gomega) {
							revision := &v1.IstioRevision{}
							g.Expect(cl.Get(ctx, kube.Key(revisionName, controlPlaneNamespace), revision)).To(Succeed())
							g.Expect(revision).To(HaveConditionStatus(v1.IstioRevisionConditionDependenciesHealthy, metav1.ConditionTrue))
						}).WithTimeout(60*time.Second).Should(Succeed(), "IstioRevision should have DependenciesHealthy=True")
						Success("IstioRevision recovered after dependencies became healthy")
					})
				})

				AfterAll(func(ctx SpecContext) {
					if CurrentSpecReport().Failed() && keepOnFailure {
						return
					}
					clr.Cleanup(ctx)
				})
			})
		}
	})
})
