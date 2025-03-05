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
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	. "github.com/istio-ecosystem/sail-operator/tests/e2e/util/gomega"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Control Plane updates", Label("update"), Ordered, func() {
	SetDefaultEventuallyTimeout(180 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)
	debugInfoLogged := false

	Describe("using IstioRevisionTag", func() {
		// istioversion.Old is the version second version in versions.yaml file and istioversion.New is the first version in the List
		// istioversion.Old is going to be the base version from where we are going to update to istioversion.New
		// TODO: improve this: https://github.com/istio-ecosystem/sail-operator/issues/681
		baseVersion := istioversion.Old
		newVersion := istioversion.New
		Context(baseVersion, func() {
			BeforeAll(func(ctx SpecContext) {
				if len(istioversion.List) < 2 {
					Skip("Skipping update tests because there are not enough versions in versions.yaml")
				}

				Expect(k.CreateNamespace(controlPlaneNamespace)).To(Succeed(), "Istio namespace failed to be created")
				Expect(k.CreateNamespace(istioCniNamespace)).To(Succeed(), "IstioCNI namespace failed to be created")

				yaml := `
apiVersion: sailoperator.io/v1
kind: IstioCNI
metadata:
  name: default
spec:
  version: %s
  namespace: %s`
				yaml = fmt.Sprintf(yaml, baseVersion, istioCniNamespace)
				Log("IstioCNI YAML:", indent(yaml))
				Expect(k.CreateFromString(yaml)).To(Succeed(), "IstioCNI creation failed")
				Success("IstioCNI created")

				Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(istioCniName), &v1.IstioCNI{}).
					Should(HaveCondition(v1.IstioCNIConditionReady, metav1.ConditionTrue), "IstioCNI is not Ready; unexpected Condition")
				Success("IstioCNI is Ready")
			})

			When(fmt.Sprintf("the Istio CR is created with RevisionBased updateStrategy for base version %s", baseVersion), func() {
				BeforeAll(func() {
					istioYAML := `
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  version: %s
  namespace: %s
  updateStrategy:
    type: RevisionBased
    inactiveRevisionDeletionGracePeriodSeconds: 30`
					istioYAML = fmt.Sprintf(istioYAML, baseVersion, controlPlaneNamespace)
					Log("Istio YAML:", indent(istioYAML))
					Expect(k.CreateFromString(istioYAML)).
						To(Succeed(), "Istio CR failed to be created")
					Success("Istio CR created")
				})

				It("deploys istiod and pod is Ready", func(ctx SpecContext) {
					Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key("default"), &v1.Istio{}).
						Should(HaveCondition(v1.IstioConditionReady, metav1.ConditionTrue), "Istiod is not Available; unexpected Condition")
					Success("Istiod is deployed in the namespace and Running")
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
					Log("IstioRevisionTag YAML:", indent(IstioRevisionTagYAML))
					Expect(k.CreateFromString(IstioRevisionTagYAML)).
						To(Succeed(), "IstioRevisionTag CR failed to be created")
					Success("IstioRevisionTag CR created")
				})

				It("creates the resource with condition InUse false", func(ctx SpecContext) {
					// Condition InUse is expected to be false because there are no pods using the IstioRevisionTag
					Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key("default"), &v1.IstioRevisionTag{}).
						Should(HaveCondition(v1.IstioRevisionTagConditionInUse, metav1.ConditionFalse), "unexpected Condition; expected InUse False")
					Success("IstioRevisionTag created and not in use")
				})

				It("IstioRevisionTag revision name is equal to the IstioRevision base name", func(ctx SpecContext) {
					revisionName := strings.Replace(baseVersion, ".", "-", -1)
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
						ApplyWithLabels(common.GetSampleYAML(istioversion.Map[baseVersion], sampleNamespace), "version=v1")).
						To(Succeed(), "Error deploying sample")
					Success("sample deployed")

					samplePods := &corev1.PodList{}

					Eventually(func() bool {
						Expect(cl.List(ctx, samplePods, client.InNamespace(sampleNamespace))).To(Succeed())
						return len(samplePods.Items) > 0
					}).Should(BeTrue(), "No sample pods found")

					for _, pod := range samplePods.Items {
						Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(pod.Name, sampleNamespace), &corev1.Pod{}).
							Should(HaveCondition(corev1.PodReady, metav1.ConditionTrue), "Pod is not Ready")
					}
					Success("sample pods are ready")

					for _, pod := range samplePods.Items {
						sidecarVersion, err := getProxyVersion(pod.Name, sampleNamespace)
						Expect(err).NotTo(HaveOccurred(), "Error getting sidecar version")
						Expect(sidecarVersion).To(Equal(istioversion.Map[baseVersion].Version), "Sidecar Istio version does not match the expected version")
					}
					Success("Istio sidecar version matches the expected base Istio version")
				})

				It("IstioRevisionTag state change to inUse true", func(ctx SpecContext) {
					Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key("default"), &v1.IstioRevisionTag{}).
						Should(HaveCondition(v1.IstioRevisionTagConditionInUse, metav1.ConditionTrue), "unexpected Condition; expected InUse true")
					Success("IstioRevisionTag is in use by the sample pods")
				})
			})

			When("the Istio CR is updated to the new Istio version", func() {
				BeforeAll(func() {
					Expect(k.Patch("istio", "default", "merge", `{"spec":{"version":"`+newVersion+`"}}`)).To(Succeed(), "Error updating Istio CR to new Istio version")
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
						HaveField("Spec", HaveField("Version", ContainSubstring(baseVersion)))),
						"Expected a revision with the base version")
					Expect(istioRevisions.Items).To(ContainElement(
						HaveField("Spec", HaveField("Version", ContainSubstring(newVersion)))),
						"Expected a revision with the new version")
					Success("Two IstionRevision found")
				})

				It("both IstionRevision are in use", func(ctx SpecContext) {
					// Check that both IstioRevisionTags are in use. One is in use by the current proxies and the new because is being referenced by the tag
					istioRevisions := &v1.IstioRevisionList{}
					Expect(cl.List(ctx, istioRevisions)).To(Succeed())
					for _, revision := range istioRevisions.Items {
						Expect(revision).To(HaveCondition(v1.IstioRevisionTagConditionInUse, metav1.ConditionTrue), "IstioRevisionTag is not in use")
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
						}).Should(Equal(istioversion.Map[baseVersion].Version), "Sidecar Istio version does not match the expected version")
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

					Expect(cl.List(ctx, samplePods, client.InNamespace(sampleNamespace))).To(Succeed())
					Expect(samplePods.Items).ToNot(BeEmpty(), "No pods found in sample namespace")
					for _, pod := range samplePods.Items {
						Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(pod.Name, sampleNamespace), &corev1.Pod{}).
							Should(HaveCondition(corev1.PodReady, metav1.ConditionTrue), "Pod is not Ready")
					}

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
							if err != nil || !sidecarVersion.Equal(istioversion.Map[newVersion].Version) {
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
					revisionName := strings.Replace(newVersion, ".", "-", -1)
					Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key("default"), &v1.IstioRevisionTag{}).
						Should(HaveField("Status.IstioRevision", ContainSubstring(revisionName)), "IstioRevisionTag version does not match the new IstioRevision name")
					Success("IstioRevisionTag points to the new IstioRevision")
				})
			})

			AfterAll(func(ctx SpecContext) {
				if CurrentSpecReport().Failed() {
					common.LogDebugInfo(common.ControlPlane, k)
					debugInfoLogged = true
				}

				By("Cleaning up sample namespace")
				Expect(k.DeleteNamespace(sampleNamespace)).To(Succeed(), "Sample Namespace failed to be deleted")

				By("Cleaning up the Istio namespace")
				Expect(k.Delete("istio", istioName)).To(Succeed(), "Istio CR failed to be deleted")
				Expect(k.DeleteNamespace(controlPlaneNamespace)).To(Succeed(), "Istio Namespace failed to be deleted")

				By("Cleaning up the IstioCNI namespace")
				Expect(k.Delete("istiocni", istioCniName)).To(Succeed(), "IstioCNI CR failed to be deleted")
				Expect(k.DeleteNamespace(istioCniNamespace)).To(Succeed(), "IstioCNI Namespace failed to be deleted")

				By("Deleting the IstioRevisionTag")
				Expect(k.Delete("istiorevisiontag", "default")).To(Succeed(), "IstioRevisionTag failed to be deleted")
				Success("Cleanup done")
			})
		})

		AfterAll(func() {
			if CurrentSpecReport().Failed() && !debugInfoLogged {
				common.LogDebugInfo(common.ControlPlane, k)
				debugInfoLogged = true
			}
		})
	})
})
