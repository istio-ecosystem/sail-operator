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
	"regexp"
	"strings"
	"time"

	"github.com/istio-ecosystem/sail-operator/api/v1alpha1"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/pkg/test/util/supportedversion"
	common "github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	. "github.com/istio-ecosystem/sail-operator/tests/e2e/util/gomega"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/helm"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/kubectl"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"istio.io/istio/pkg/ptr"
)

var istiodVersionRegex = regexp.MustCompile(`Version:"(\d+\.\d+(\.\d+|-\w+))`)

var _ = Describe("Control Plane Installation", Ordered, func() {
	SetDefaultEventuallyTimeout(120 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)

	Describe("given Istio version", func() {

		BeforeAll(func(ctx SpecContext) {
			Expect(kubectl.CreateNamespace(namespace)).To(Succeed(), "Namespace failed to be created")

			extraArg := ""
			if ocp {
				extraArg = "--set=platform=openshift"
			}

			Expect(helm.Install("sail-operator", filepath.Join(baseDir, "chart"), "--namespace "+namespace, "--set=image="+image, extraArg)).
				To(Succeed(), "Operator failed to be deployed")

			Eventually(common.GetObject).WithArguments(ctx, cl, common.Key(deploymentName, namespace), &appsv1.Deployment{}).
				Should(HaveCondition(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Error getting Istio CRD")
			Success("Operator is deployed in the namespace and Running")
		})

		for _, version := range supportedversion.List {
			// Note: This var version is needed to avoid the closure of the loop
			version := version

			Context(version.Name, func() {
				BeforeAll(func() {
					Expect(kubectl.CreateNamespace(controlPlaneNamespace)).To(Succeed(), "Istio namespace failed to be created")
					Expect(kubectl.CreateNamespace(istioCniNamespace)).To(Succeed(), "IstioCNI namespace failed to be created")
				})

				When("the IstioCNI CR is created", func() {
					BeforeAll(func() {
						yaml := `
apiVersion: operator.istio.io/v1alpha1
kind: IstioCNI
metadata:
  name: default
spec:
  version: %s
  namespace: %s`
						yaml = fmt.Sprintf(yaml, version.Name, istioCniNamespace)
						Log("IstioCNI YAML:", indent(2, yaml))
						Expect(kubectl.ApplyString(yaml)).To(Succeed(), "IstioCNI creation failed")
						Success("IstioCNI created")
					})

					It("deploys the CNI DaemonSet", func(ctx SpecContext) {
						Eventually(func(g Gomega) {
							daemonset := &appsv1.DaemonSet{}
							g.Expect(cl.Get(ctx, common.Key("istio-cni-node", istioCniNamespace), daemonset)).To(Succeed(), "Error getting IstioCNI DaemonSet")
							g.Expect(daemonset.Status.NumberAvailable).
								To(Equal(daemonset.Status.CurrentNumberScheduled), "CNI DaemonSet Pods not Available; expected numberAvailable to be equal to currentNumberScheduled")
						}).Should(Succeed(), "CNI DaemonSet Pods are not Available")
						Success("CNI DaemonSet is deployed in the namespace and Running")
					})

					It("updates the status to Reconciled", func(ctx SpecContext) {
						Eventually(common.GetObject).WithArguments(ctx, cl, common.Key(istioCniName), &v1alpha1.IstioCNI{}).
							Should(HaveCondition(v1alpha1.IstioCNIConditionReconciled, metav1.ConditionTrue), "IstioCNI is not Reconciled; unexpected Condition")
						Success("IstioCNI is Reconciled")
					})

					It("updates the status to Ready", func(ctx SpecContext) {
						Eventually(common.GetObject).WithArguments(ctx, cl, common.Key(istioCniName), &v1alpha1.IstioCNI{}).
							Should(HaveCondition(v1alpha1.IstioCNIConditionReady, metav1.ConditionTrue), "IstioCNI is not Ready; unexpected Condition")
						Success("IstioCNI is Ready")
					})

					It("doesn't continuously reconcile the IstioCNI CR", func() {
						Eventually(kubectl.Logs).WithArguments(namespace, "deploy/"+deploymentName, ptr.Of(30*time.Second)).
							ShouldNot(ContainSubstring("Reconciliation done"), "Istio Operator is continuously reconciling")
						Success("Istio Operator stopped reconciling")
					})
				})

				When("the Istio CR is created", func() {
					BeforeAll(func() {
						istioYAML := `
apiVersion: operator.istio.io/v1alpha1
kind: Istio
metadata:
  name: default
spec:
  version: %s
  namespace: %s`
						istioYAML = fmt.Sprintf(istioYAML, version.Name, controlPlaneNamespace)
						Log("Istio YAML:", indent(2, istioYAML))
						Expect(kubectl.ApplyString(istioYAML)).
							To(Succeed(), "Istio CR failed to be created")
						Success("Istio CR created")
					})

					It("updates the Istio CR status to Reconciled", func(ctx SpecContext) {
						Eventually(common.GetObject).WithArguments(ctx, cl, common.Key(istioName), &v1alpha1.Istio{}).
							Should(HaveCondition(v1alpha1.IstioConditionReconciled, metav1.ConditionTrue), "Istio is not Reconciled; unexpected Condition")
						Success("Istio CR is Reconciled")
					})

					It("updates the Istio CR status to Ready", func(ctx SpecContext) {
						Eventually(common.GetObject).WithArguments(ctx, cl, common.Key(istioName), &v1alpha1.Istio{}).
							Should(HaveCondition(v1alpha1.IstioConditionReady, metav1.ConditionTrue), "Istio is not Ready; unexpected Condition")
						Success("Istio CR is Ready")
					})

					It("deploys istiod", func(ctx SpecContext) {
						Eventually(common.GetObject).WithArguments(ctx, cl, common.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(HaveCondition(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Istiod is not Available; unexpected Condition")
						Expect(getVersionFromIstiod()).To(Equal(version.Version), "Unexpected istiod version")
						Success("Istiod is deployed in the namespace and Running")
					})

					It("doesn't continuously reconcile the Istio CR", func() {
						Eventually(kubectl.Logs).WithArguments(namespace, "deploy/"+deploymentName, ptr.Of(30*time.Second)).
							ShouldNot(ContainSubstring("Reconciliation done"), "Istio Operator is continuously reconciling")
						Success("Istio Operator stopped reconciling")
					})
				})

				When("the Istio CR is deleted", func() {
					BeforeEach(func() {
						Expect(kubectl.Delete(controlPlaneNamespace, "istio", istioName)).To(Succeed(), "Istio CR failed to be deleted")
						Success("Istio CR deleted")
					})

					It("removes everything from the namespace", func(ctx SpecContext) {
						Eventually(cl.Get).WithArguments(ctx, common.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{}).
							Should(ReturnNotFoundError(), "Istiod should not exist anymore")
						common.CheckNamespaceEmpty(ctx, cl, controlPlaneNamespace)
						Success("Namespace is empty")
					})
				})

				When("the IstioCNI CR is deleted", func() {
					BeforeEach(func() {
						Expect(kubectl.Delete(istioCniNamespace, "istiocni", istioCniName)).To(Succeed(), "IstioCNI CR failed to be deleted")
						Success("IstioCNI deleted")
					})

					It("removes everything from the CNI namespace", func(ctx SpecContext) {
						daemonset := &appsv1.DaemonSet{}
						Eventually(cl.Get).WithArguments(ctx, common.Key("istio-cni-node", istioCniNamespace), daemonset).
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
		By("Deleting operator deployment")
		Expect(helm.Uninstall("sail-operator", "--namespace "+namespace)).
			To(Succeed(), "Operator failed to be deleted")
		GinkgoWriter.Println("Operator uninstalled")

		Expect(kubectl.DeleteNamespace(namespace)).To(Succeed(), "Namespace failed to be deleted")
		Success("Namespace deleted")
	})
})

func getVersionFromIstiod() (string, error) {
	output, err := kubectl.Exec(controlPlaneNamespace, "deploy/istiod", "pilot-discovery version")
	if err != nil {
		return "", fmt.Errorf("error getting version from istiod: %v", err)
	}

	matches := istiodVersionRegex.FindStringSubmatch(output)
	if len(matches) > 1 && matches[1] != "" {
		return matches[1], nil
	}
	return "", fmt.Errorf("error getting version from istiod: version not found in output: %s", output)
}

func indent(level int, str string) string {
	indent := strings.Repeat(" ", level)
	return indent + strings.ReplaceAll(str, "\n", "\n"+indent)
}

func forceDeleteIstioResources() error {
	// This is a workaround to delete the Istio CRs that are left in the cluster
	// This will be improved by splitting the tests into different Nodes with their independent setups and cleanups
	err := kubectl.ForceDelete("", "istio", istioName)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("failed to delete %s CR: %v", "istio", err)
	}

	err = kubectl.ForceDelete("", "istiorevision", "default")
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("failed to delete %s CR: %v", "istiorevision", err)
	}

	err = kubectl.Delete("", "istiocni", istioCniName)
	if err != nil && !strings.Contains(err.Error(), "not found") {
		return fmt.Errorf("failed to delete %s CR: %v", "istiocni", err)
	}

	return nil
}
