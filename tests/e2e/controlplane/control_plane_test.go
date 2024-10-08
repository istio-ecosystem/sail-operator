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
	"path/filepath"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/istio-ecosystem/sail-operator/api/v1alpha1"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	"github.com/istio-ecosystem/sail-operator/pkg/test/project"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/pkg/test/util/supportedversion"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	. "github.com/istio-ecosystem/sail-operator/tests/e2e/util/gomega"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/helm"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"istio.io/istio/pkg/ptr"
)

var _ = Describe("Control Plane Installation", Ordered, func() {
	SetDefaultEventuallyTimeout(180 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)
	debugInfoLogged := false

	BeforeAll(func(ctx SpecContext) {
		Expect(k.CreateNamespace(namespace)).To(Succeed(), "Namespace failed to be created")

		extraArg := ""
		if ocp {
			extraArg = "--set=platform=openshift"
		}

		if skipDeploy {
			Success("Skipping operator installation because it was deployed externally")
		} else {
			Expect(helm.Install("sail-operator", filepath.Join(project.RootDir, "chart"), "--namespace "+namespace, "--set=image="+image, extraArg)).
				To(Succeed(), "Operator failed to be deployed")
		}

		Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(deploymentName, namespace), &appsv1.Deployment{}).
			Should(HaveCondition(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Error getting Istio CRD")
		Success("Operator is deployed in the namespace and Running")
	})

	Describe("defaulting", func() {
		DescribeTable("IstioCNI",
			Entry("no spec", ""),
			Entry("empty spec", "spec: {}"),
			func(ctx SpecContext, spec string) {
				yaml := `
apiVersion: sailoperator.io/v1alpha1
kind: IstioCNI
metadata:
  name: default
` + spec
				Expect(k.CreateFromString(yaml)).To(Succeed(), "IstioCNI creation failed")
				Success("IstioCNI created")

				cni := &v1alpha1.IstioCNI{}
				Expect(cl.Get(ctx, kube.Key("default"), cni)).To(Succeed())
				Expect(cni.Spec.Version).To(Equal(supportedversion.Default))
				Expect(cni.Spec.Namespace).To(Equal("istio-cni"))

				Expect(cl.Delete(ctx, cni)).To(Succeed())
				Eventually(cl.Get).WithArguments(ctx, kube.Key("default"), cni).Should(ReturnNotFoundError())
			},
		)

		DescribeTable("Istio",
			Entry("no spec", ""),
			Entry("empty spec", "spec: {}"),
			Entry("empty updateStrategy", "spec: {updateStrategy: {}}"),
			func(ctx SpecContext, spec string) {
				yaml := `
apiVersion: sailoperator.io/v1alpha1
kind: Istio
metadata:
  name: default
` + spec
				Expect(k.CreateFromString(yaml)).To(Succeed(), "Istio creation failed")
				Success("Istio created")

				istio := &v1alpha1.Istio{}
				Expect(cl.Get(ctx, kube.Key("default"), istio)).To(Succeed())
				Expect(istio.Spec.Version).To(Equal(supportedversion.Default))
				Expect(istio.Spec.Namespace).To(Equal("istio-system"))
				Expect(istio.Spec.UpdateStrategy).ToNot(BeNil())
				Expect(istio.Spec.UpdateStrategy.Type).To(Equal(v1alpha1.UpdateStrategyTypeInPlace))

				Expect(cl.Delete(ctx, istio)).To(Succeed())
				Eventually(cl.Get).WithArguments(ctx, kube.Key("default"), istio).Should(ReturnNotFoundError())
			},
		)
	})

	Describe("given Istio version", func() {
		for _, version := range supportedversion.List {
			// Note: This var version is needed to avoid the closure of the loop
			version := version

			Context(version.Name, func() {
				BeforeAll(func() {
					Expect(k.CreateNamespace(controlPlaneNamespace)).To(Succeed(), "Istio namespace failed to be created")
					Expect(k.CreateNamespace(istioCniNamespace)).To(Succeed(), "IstioCNI namespace failed to be created")
				})

				When("the IstioCNI CR is created", func() {
					BeforeAll(func() {
						yaml := `
apiVersion: sailoperator.io/v1alpha1
kind: IstioCNI
metadata:
  name: default
spec:
  version: %s
  namespace: %s`
						yaml = fmt.Sprintf(yaml, version.Name, istioCniNamespace)
						Log("IstioCNI YAML:", indent(2, yaml))
						Expect(k.CreateFromString(yaml)).To(Succeed(), "IstioCNI creation failed")
						Success("IstioCNI created")
					})

					It("deploys the CNI DaemonSet", func(ctx SpecContext) {
						Eventually(func(g Gomega) {
							daemonset := &appsv1.DaemonSet{}
							g.Expect(cl.Get(ctx, kube.Key("istio-cni-node", istioCniNamespace), daemonset)).To(Succeed(), "Error getting IstioCNI DaemonSet")
							g.Expect(daemonset.Status.NumberAvailable).
								To(Equal(daemonset.Status.CurrentNumberScheduled), "CNI DaemonSet Pods not Available; expected numberAvailable to be equal to currentNumberScheduled")
						}).Should(Succeed(), "CNI DaemonSet Pods are not Available")
						Success("CNI DaemonSet is deployed in the namespace and Running")
					})

					It("uses the correct image", func(ctx SpecContext) {
						Expect(common.GetObject(ctx, cl, kube.Key("istio-cni-node", istioCniNamespace), &appsv1.DaemonSet{})).
							To(HaveContainersThat(HaveEach(ImageFromRegistry(expectedRegistry))))
					})

					It("updates the status to Reconciled", func(ctx SpecContext) {
						Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(istioCniName), &v1alpha1.IstioCNI{}).
							Should(HaveCondition(v1alpha1.IstioCNIConditionReconciled, metav1.ConditionTrue), "IstioCNI is not Reconciled; unexpected Condition")
						Success("IstioCNI is Reconciled")
					})

					It("updates the status to Ready", func(ctx SpecContext) {
						Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(istioCniName), &v1alpha1.IstioCNI{}).
							Should(HaveCondition(v1alpha1.IstioCNIConditionReady, metav1.ConditionTrue), "IstioCNI is not Ready; unexpected Condition")
						Success("IstioCNI is Ready")
					})

					It("doesn't continuously reconcile the IstioCNI CR", func() {
						Eventually(k.SetNamespace(namespace).Logs).WithArguments("deploy/"+deploymentName, ptr.Of(30*time.Second)).
							ShouldNot(ContainSubstring("Reconciliation done"), "IstioCNI is continuously reconciling")
						Success("IstioCNI stopped reconciling")
					})
				})

				When("the Istio CR is created", func() {
					BeforeAll(func() {
						istioYAML := `
apiVersion: sailoperator.io/v1alpha1
kind: Istio
metadata:
  name: default
spec:
  version: %s
  namespace: %s`
						istioYAML = fmt.Sprintf(istioYAML, version.Name, controlPlaneNamespace)
						Log("Istio YAML:", indent(2, istioYAML))
						Expect(k.CreateFromString(istioYAML)).
							To(Succeed(), "Istio CR failed to be created")
						Success("Istio CR created")
					})

					It("updates the Istio CR status to Reconciled", func(ctx SpecContext) {
						Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(istioName), &v1alpha1.Istio{}).
							Should(HaveCondition(v1alpha1.IstioConditionReconciled, metav1.ConditionTrue), "Istio is not Reconciled; unexpected Condition")
						Success("Istio CR is Reconciled")
					})

					It("updates the Istio CR status to Ready", func(ctx SpecContext) {
						Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(istioName), &v1alpha1.Istio{}).
							Should(HaveCondition(v1alpha1.IstioConditionReady, metav1.ConditionTrue), "Istio is not Ready; unexpected Condition")
						Success("Istio CR is Ready")
					})

					It("deploys istiod", func(ctx SpecContext) {
						Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(HaveCondition(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Istiod is not Available; unexpected Condition")
						Expect(common.GetVersionFromIstiod()).To(Equal(version.Version), "Unexpected istiod version")
						Success("Istiod is deployed in the namespace and Running")
					})

					It("uses the correct image", func(ctx SpecContext) {
						Expect(common.GetObject(ctx, cl, kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{})).
							To(HaveContainersThat(HaveEach(ImageFromRegistry(expectedRegistry))))
					})

					It("doesn't continuously reconcile the Istio CR", func() {
						Eventually(k.SetNamespace(namespace).Logs).WithArguments("deploy/"+deploymentName, ptr.Of(30*time.Second)).
							ShouldNot(ContainSubstring("Reconciliation done"), "Istio CR is continuously reconciling")
						Success("Istio CR stopped reconciling")
					})
				})

				When("bookinfo is deployed", func() {
					BeforeAll(func() {
						Expect(k.CreateNamespace(bookinfoNamespace)).To(Succeed(), "Bookinfo namespace failed to be created")
						Expect(k.Patch("namespace", bookinfoNamespace, "merge", `{"metadata":{"labels":{"istio-injection":"enabled"}}}`)).
							To(Succeed(), "Error patching bookinfo namespace")
						Expect(deployBookinfo(version)).To(Succeed(), "Error deploying bookinfo")
						Success("Bookinfo deployed")
					})

					bookinfoPods := &corev1.PodList{}

					It("updates the pods status to Running", func(ctx SpecContext) {
						Expect(cl.List(ctx, bookinfoPods, client.InNamespace(bookinfoNamespace))).To(Succeed())
						Expect(bookinfoPods.Items).ToNot(BeEmpty(), "No pods found in bookinfo namespace")

						for _, pod := range bookinfoPods.Items {
							Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(pod.Name, bookinfoNamespace), &corev1.Pod{}).
								Should(HaveCondition(corev1.PodReady, metav1.ConditionTrue), "Pod is not Ready")
						}
						Success("Bookinfo pods are ready")
					})

					It("has sidecars with the correct istio version", func(ctx SpecContext) {
						for _, pod := range bookinfoPods.Items {
							sidecarVersion, err := getProxyVersion(pod.Name, bookinfoNamespace)
							Expect(err).NotTo(HaveOccurred(), "Error getting sidecar version")
							Expect(sidecarVersion).To(Equal(version.Version), "Sidecar Istio version does not match the expected version")
						}
						Success("Istio sidecar version matches the expected Istio version")
					})

					AfterAll(func(ctx SpecContext) {
						By("Deleting bookinfo")
						Expect(k.DeleteNamespace(bookinfoNamespace)).To(Succeed(), "Bookinfo namespace failed to be deleted")
						Success("Bookinfo deleted")
					})
				})

				When("the Istio CR is deleted", func() {
					BeforeEach(func() {
						Expect(k.SetNamespace(controlPlaneNamespace).Delete("istio", istioName)).To(Succeed(), "Istio CR failed to be deleted")
						Success("Istio CR deleted")
					})

					It("removes everything from the namespace", func(ctx SpecContext) {
						Eventually(cl.Get).WithArguments(ctx, kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(ReturnNotFoundError(), "Istiod should not exist anymore")
						common.CheckNamespaceEmpty(ctx, cl, controlPlaneNamespace)
						Success("Namespace is empty")
					})
				})

				When("the IstioCNI CR is deleted", func() {
					BeforeEach(func() {
						Expect(k.SetNamespace(istioCniNamespace).Delete("istiocni", istioCniName)).To(Succeed(), "IstioCNI CR failed to be deleted")
						Success("IstioCNI deleted")
					})

					It("removes everything from the CNI namespace", func(ctx SpecContext) {
						daemonset := &appsv1.DaemonSet{}
						Eventually(cl.Get).WithArguments(ctx, kube.Key("istio-cni-node", istioCniNamespace), daemonset).
							Should(ReturnNotFoundError(), "IstioCNI DaemonSet should not exist anymore")
						common.CheckNamespaceEmpty(ctx, cl, istioCniNamespace)
						Success("CNI namespace is empty")
					})
				})
			})
		}

		AfterAll(func(ctx SpecContext) {
			if CurrentSpecReport().Failed() {
				common.LogDebugInfo()
				debugInfoLogged = true
			}

			By("Cleaning up the Istio namespace")
			Expect(cl.Delete(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: controlPlaneNamespace}})).To(Succeed(), "Istio Namespace failed to be deleted")

			By("Cleaning up the IstioCNI namespace")
			Expect(cl.Delete(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: istioCniNamespace}})).To(Succeed(), "IstioCNI Namespace failed to be deleted")

			By("Deleting any left-over Istio and IstioRevision resources")
			Expect(forceDeleteIstioResources()).To(Succeed())
			Success("Resources deleted")
			Success("Cleanup done")
		})
	})

	AfterAll(func() {
		if CurrentSpecReport().Failed() && !debugInfoLogged {
			common.LogDebugInfo()
			debugInfoLogged = true
		}

		if skipDeploy {
			Success("Skipping operator undeploy because it was deployed externally")
			return
		}

		By("Deleting operator deployment")
		Expect(helm.Uninstall("sail-operator", "--namespace "+namespace)).
			To(Succeed(), "Operator failed to be deleted")
		GinkgoWriter.Println("Operator uninstalled")

		if ocp {
			Success("Skipping deletion of operator namespace to avoid removal of operator container image from internal registry")
			return
		}
		Expect(k.DeleteNamespace(namespace)).To(Succeed(), "Namespace failed to be deleted")
		Success("Namespace deleted")
	})
})

func HaveContainersThat(matcher types.GomegaMatcher) types.GomegaMatcher {
	return HaveField("Spec.Template.Spec.Containers", matcher)
}

func ImageFromRegistry(regexp string) types.GomegaMatcher {
	return HaveField("Image", MatchRegexp(regexp))
}

func indent(level int, str string) string {
	indent := strings.Repeat(" ", level)
	return indent + strings.ReplaceAll(str, "\n", "\n"+indent)
}

func forceDeleteIstioResources() error {
	// This is a workaround to delete the Istio CRs that are left in the cluster
	// This will be improved by splitting the tests into different Nodes with their independent setups and cleanups
	err := k.ForceDelete("istio", istioName)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("failed to delete %s CR: %w", "istio", err)
	}

	err = k.ForceDelete("istiorevision", "default")
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("failed to delete %s CR: %w", "istiorevision", err)
	}

	err = k.Delete("istiocni", istioCniName)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("failed to delete %s CR: %w", "istiocni", err)
	}

	return nil
}

func getBookinfoURL(version supportedversion.VersionInfo) string {
	// Bookinfo YAML for the current version can be found from istio/istio repository
	// If the version is latest, we need to get the latest version from the master branch
	bookinfoURL := fmt.Sprintf("https://raw.githubusercontent.com/istio/istio/%s/samples/bookinfo/platform/kube/bookinfo.yaml", version.Version)
	if version.Name == "latest" {
		bookinfoURL = "https://raw.githubusercontent.com/istio/istio/master/samples/bookinfo/platform/kube/bookinfo.yaml"
	}

	return bookinfoURL
}

func deployBookinfo(version supportedversion.VersionInfo) error {
	bookinfoURL := getBookinfoURL(version)
	err := k.SetNamespace(bookinfoNamespace).Apply(bookinfoURL)
	if err != nil {
		return fmt.Errorf("error deploying bookinfo: %w", err)
	}

	return nil
}

func getProxyVersion(podName, namespace string) (*semver.Version, error) {
	output, err := k.SetNamespace(namespace).Exec(
		podName,
		"istio-proxy",
		`curl -s http://localhost:15000/server_info | grep "ISTIO_VERSION" | awk -F '"' '{print $4}'`)
	if err != nil {
		return nil, fmt.Errorf("error getting sidecar version: %w", err)
	}

	versionStr := strings.TrimSpace(output)
	version, err := semver.NewVersion(versionStr)
	if err != nil {
		return version, fmt.Errorf("error parsing sidecar version %q: %w", versionStr, err)
	}
	return version, err
}
