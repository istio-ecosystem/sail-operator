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

package dualstack

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/istio-ecosystem/sail-operator/api/v1alpha1"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	"github.com/istio-ecosystem/sail-operator/pkg/test/project"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/pkg/test/util/supportedversion"
	common "github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	. "github.com/istio-ecosystem/sail-operator/tests/e2e/util/gomega"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/helm"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Installation on a dualStack cluster", Ordered, func() {
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

	Describe("using supported Istio version", func() {
		for _, version := range supportedversion.List {
			// Note: This var version is needed to avoid the closure of the loop
			version := version

			// The minimum supported version is 1.23 (and above)
			if version.Major == 1 && version.Minor < 23 {
				continue
			}

			Context(version.Name, func() {
				BeforeAll(func() {
					Expect(k.CreateNamespace(controlPlaneNamespace)).To(Succeed(), "Istio namespace failed to be created")
				})

				When("the Istio CR is created", func() {
					BeforeAll(func() {
						istioYAML := `
apiVersion: sailoperator.io/v1alpha1
kind: Istio
metadata:
  name: default
spec:
  values:
    meshConfig:
      defaultConfig:
        proxyMetadata:
          ISTIO_DUAL_STACK: "true"
    pilot:
      ipFamilyPolicy: %s
      env:
        ISTIO_DUAL_STACK: "true"
  version: %s
  namespace: %s`
						istioYAML = fmt.Sprintf(istioYAML, corev1.IPFamilyPolicyRequireDualStack, version.Name, controlPlaneNamespace)
						Log("Istio YAML:", istioYAML)
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

					It("has ISTIO_DUAL_STACK env variable set", func(ctx SpecContext) {
						Expect(common.GetObject(ctx, cl, kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{})).
							To(HaveContainersThat(ContainElement(WithTransform(getEnvVars, ContainElement(corev1.EnvVar{Name: "ISTIO_DUAL_STACK", Value: "true"})))),
								"Expected ISTIO_DUAL_STACK to be set to true, but not found")
					})

					It("deploys istiod service in dualStack mode", func(ctx SpecContext) {
						var istiodSvcObj corev1.Service

						Eventually(func() error {
							_, err := common.GetObject(ctx, cl, kube.Key("istiod", controlPlaneNamespace), &istiodSvcObj)
							return err
						}).Should(Succeed(), "Expected to retrieve the 'istiod' service")

						Expect(istiodSvcObj.Spec.IPFamilyPolicy).ToNot(BeNil(), "Expected IPFamilyPolicy to be set")
						Expect(*istiodSvcObj.Spec.IPFamilyPolicy).To(Equal(corev1.IPFamilyPolicyRequireDualStack), "Expected ipFamilyPolicy to be 'RequireDualStack'")
						Success("Istio Service is deployed in the namespace and Running")
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
			})
		}

		AfterAll(func(ctx SpecContext) {
			if CurrentSpecReport().Failed() {
				common.LogDebugInfo()
				debugInfoLogged = true
			}

			By("Cleaning up the Istio namespace")
			Expect(cl.Delete(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: controlPlaneNamespace}})).To(Succeed(), "Istio Namespace failed to be deleted")

			By("Deleting any left-over Istio and IstioRevision resources")
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

func getEnvVars(container corev1.Container) []corev1.EnvVar {
	return container.Env
}
