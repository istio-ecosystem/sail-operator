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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Control Plane updates", Label("control-plane", "slow"), Ordered, func() {
	SetDefaultEventuallyTimeout(180 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)
	debugInfoLogged := false

	Describe("using IstioRevisionTag", func() {
		BeforeAll(func() {
			if istioversion.Base == "" || istioversion.New == "" {
				Skip("Skipping update tests because there are not enough versions in versions.yaml")
			}
		})

		Context(istioversion.Base, func() {
			clr := cleaner.New(cl)

			BeforeAll(func(ctx SpecContext) {
				if len(istioversion.List) < 2 {
					Skip("Skipping update tests because there are not enough versions in versions.yaml")
				}

				clr.Record(ctx)
				Expect(k.CreateNamespace(controlPlaneNamespace)).To(Succeed(), "Istio namespace failed to be created")
				Expect(k.CreateNamespace(istioCniNamespace)).To(Succeed(), "IstioCNI namespace failed to be created")

				common.CreateIstioCNI(k, istioversion.Base)
				common.AwaitCondition(ctx, v1.IstioCNIConditionReady, kube.Key(istioCniName), &v1.IstioCNI{}, k, cl)
			})

			When(fmt.Sprintf("the Istio CR is created with RevisionBased updateStrategy for base version %s", istioversion.Base), func() {
				BeforeAll(func() {
					common.CreateIstio(k, istioversion.Base, `
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
					revisionName := strings.Replace(istioversion.Base, ".", "-", -1)
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
						sidecarVersion, err := getProxyVersion(pod.Name, sampleNamespace)
						Expect(err).NotTo(HaveOccurred(), "Error getting sidecar version")
						Expect(sidecarVersion).To(Equal(istioversion.Map[istioversion.Base].Version), "Sidecar Istio version does not match the expected version")
					}
					Success("Istio sidecar version matches the expected base Istio version")
				})

				It("IstioRevisionTag state change to inUse true", func(ctx SpecContext) {
					common.AwaitCondition(ctx, v1.IstioRevisionTagConditionInUse, kube.Key("default"), &v1.IstioRevisionTag{}, k, cl)
				})
			})

			When("the Istio CR is updated to the new Istio version", func() {
				BeforeAll(func() {
					Expect(k.Patch("istio", "default", "merge", `{"spec":{"version":"`+istioversion.New+`"}}`)).To(Succeed(), "Error updating Istio CR to new Istio version")
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
						HaveField("Spec", HaveField("Version", ContainSubstring(istioversion.Base)))),
						"Expected a revision with the base version")
					Expect(istioRevisions.Items).To(ContainElement(
						HaveField("Spec", HaveField("Version", ContainSubstring(istioversion.New)))),
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
						Eventually(func() *semver.Version {
							sidecarVersion, err := getProxyVersion(pod.Name, sampleNamespace)
							Expect(err).NotTo(HaveOccurred(), "Error getting sidecar version")
							return sidecarVersion
						}).Should(Equal(istioversion.Map[istioversion.Base].Version), "Sidecar Istio version does not match the expected version")
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
							sidecarVersion, err := getProxyVersion(pod.Name, sampleNamespace)
							if err != nil || !sidecarVersion.Equal(istioversion.Map[istioversion.New].Version) {
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
					revisionName := strings.Replace(istioversion.New, ".", "-", -1)
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
})
