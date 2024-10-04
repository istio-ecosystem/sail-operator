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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DualStackNamespace = "dual-stack"
	IPv4Namespace      = "ipv4"
	IPv6Namespace      = "ipv6"
	SleepNamespace     = "sleep"
)

var _ = Describe("DualStack configuration ", Ordered, func() {
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

	Describe("for supported versions", func() {
		for _, version := range supportedversion.List {
			// Note: This var version is needed to avoid the closure of the loop
			version := version

			// The minimum supported version is 1.23 (and above)
			if version.Major == 1 && version.Minor < 23 {
				continue
			}

			Context("Istio version is: "+version.Version, func() {
				BeforeAll(func() {
					Expect(k.CreateNamespace(controlPlaneNamespace)).To(Succeed(), "Istio namespace failed to be created")
					Expect(k.CreateNamespace(istioCniNamespace)).To(Succeed(), "IstioCNI namespace failed to be created")
				})

				When("the IstioCNI CR is created", func() {
					BeforeAll(func() {
						cniYAML := `
apiVersion: sailoperator.io/v1alpha1
kind: IstioCNI
metadata:
  name: default
spec:
  version: %s
  namespace: %s`
						cniYAML = fmt.Sprintf(cniYAML, version.Name, istioCniNamespace)
						Log("IstioCNI YAML:", cniYAML)
						Expect(k.CreateFromString(cniYAML)).To(Succeed(), "IstioCNI creation failed")
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
				})

				When("the Istio CR is created with DualStack configuration", func() {
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

				// We spawn the following pods to verify the data-path connectivity.
				// 1. a dualStack service in dual-stack namespace which listens on both IPv4 and IPv6 addresses
				// 2. an ipv4 only service in ipv4 namespace which listens only on IPv4 address
				// 3. an ipv6 only service in ipv6 namespace which listens only on IPv6 address
				// Using a sleep pod from the sleep namespace, we try to connect to all the three services to verify that connectivity is successful.
				When("sample apps are deployed in the cluster", func() {
					BeforeAll(func(ctx SpecContext) {
						Expect(k.CreateNamespace(DualStackNamespace)).To(Succeed(), "Failed to create dual-stack namespace")
						Expect(k.CreateNamespace(IPv4Namespace)).To(Succeed(), "Failed to create ipv4 namespace")
						Expect(k.CreateNamespace(IPv6Namespace)).To(Succeed(), "Failed to create ipv6 namespace")
						Expect(k.CreateNamespace(SleepNamespace)).To(Succeed(), "Failed to create sleep namespace")

						Expect(k.Patch("namespace", DualStackNamespace, "merge", `{"metadata":{"labels":{"istio-injection":"enabled"}}}`)).
							To(Succeed(), "Error patching dual-stack namespace")
						Expect(k.Patch("namespace", IPv4Namespace, "merge", `{"metadata":{"labels":{"istio-injection":"enabled"}}}`)).
							To(Succeed(), "Error patching ipv4 namespace")
						Expect(k.Patch("namespace", IPv6Namespace, "merge", `{"metadata":{"labels":{"istio-injection":"enabled"}}}`)).
							To(Succeed(), "Error patching ipv6 namespace")
						Expect(k.Patch("namespace", SleepNamespace, "merge", `{"metadata":{"labels":{"istio-injection":"enabled"}}}`)).
							To(Succeed(), "Error patching sleep namespace")

						deployDualStackValidationPods(version)
						Success("dualStack validation pods deployed")
					})

					sleepPod := &corev1.PodList{}
					It("updates the status of pods to Running", func(ctx SpecContext) {
						_, err = checkPodsReady(ctx, DualStackNamespace)
						Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("Error checking status of dual-stack pods: %v", err))

						_, err = checkPodsReady(ctx, IPv4Namespace)
						Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("Error checking status of ipv4 pods: %v", err))

						_, err = checkPodsReady(ctx, IPv6Namespace)
						Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("Error checking status of ipv6 pods: %v", err))

						sleepPod, err = checkPodsReady(ctx, SleepNamespace)
						Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("Error checking status of sleep pods: %v", err))
					})

					It("can access the dual-stack service from the sleep pod", func(ctx SpecContext) {
						checkPodConnectivity(sleepPod.Items[0].Name, SleepNamespace, DualStackNamespace)
					})

					It("can access the ipv4 only service from the sleep pod", func(ctx SpecContext) {
						checkPodConnectivity(sleepPod.Items[0].Name, SleepNamespace, IPv4Namespace)
					})

					It("can access the ipv6 only service from the sleep pod", func(ctx SpecContext) {
						checkPodConnectivity(sleepPod.Items[0].Name, SleepNamespace, IPv6Namespace)
					})

					AfterAll(func(ctx SpecContext) {
						By("Deleting the pods")
						Expect(k.DeleteNamespace(DualStackNamespace)).To(Succeed(), fmt.Sprintf("Failed to delete the %q namespace", DualStackNamespace))
						Expect(k.DeleteNamespace(IPv4Namespace)).To(Succeed(), fmt.Sprintf("Failed to delete the %q namespace", IPv4Namespace))
						Expect(k.DeleteNamespace(IPv6Namespace)).To(Succeed(), fmt.Sprintf("Failed to delete the %q namespace", IPv6Namespace))
						Expect(k.DeleteNamespace(SleepNamespace)).To(Succeed(), fmt.Sprintf("Failed to delete the %q namespace", SleepNamespace))
						Success("DualStack validation pods deleted")
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

func getPodURL(version supportedversion.VersionInfo, namespace string) string {
	var url string

	switch namespace {
	case DualStackNamespace:
		url = "samples/tcp-echo/tcp-echo-dual-stack.yaml"
	case IPv4Namespace:
		url = "samples/tcp-echo/tcp-echo-ipv4.yaml"
	case IPv6Namespace:
		url = "samples/tcp-echo/tcp-echo-ipv6.yaml"
	case SleepNamespace:
		url = "samples/sleep/sleep.yaml"
	default:
		return ""
	}

	if version.Name == "latest" {
		return fmt.Sprintf("https://raw.githubusercontent.com/istio/istio/master/%s", url)
	}

	return fmt.Sprintf("https://raw.githubusercontent.com/istio/istio/%s/%s", version.Version, url)
}

func deployDualStackValidationPods(version supportedversion.VersionInfo) {
	Expect(k.SetNamespace(DualStackNamespace).Apply(getPodURL(version, DualStackNamespace))).To(Succeed(), "error deploying tcpDualStack pod")
	Expect(k.SetNamespace(IPv4Namespace).Apply(getPodURL(version, IPv4Namespace))).To(Succeed(), "error deploying ipv4 pod")
	Expect(k.SetNamespace(IPv6Namespace).Apply(getPodURL(version, IPv6Namespace))).To(Succeed(), "error deploying ipv6 pod")
	Expect(k.SetNamespace(SleepNamespace).Apply(getPodURL(version, SleepNamespace))).To(Succeed(), "error deploying sleep pod")
}

func checkPodsReady(ctx SpecContext, namespace string) (*corev1.PodList, error) {
	podList := &corev1.PodList{}

	err := cl.List(ctx, podList, client.InNamespace(namespace))
	if err != nil {
		return nil, fmt.Errorf("failed to list pods in %s namespace: %w", namespace, err)
	}

	Expect(podList.Items).ToNot(BeEmpty(), fmt.Sprintf("No pods found in %s namespace", namespace))

	for _, pod := range podList.Items {
		Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(pod.Name, namespace), &corev1.Pod{}).
			Should(HaveCondition(corev1.PodReady, metav1.ConditionTrue), fmt.Sprintf("%q Pod in %q namespace is not Ready", pod.Name, namespace))
	}

	Success(fmt.Sprintf("Pods in %q namespace are ready", namespace))
	return podList, nil
}

func checkPodConnectivity(podName, namespace, echoStr string) {
	command := fmt.Sprintf(`sh -c 'echo %s | nc tcp-echo.%s 9000'`, echoStr, echoStr)
	response, err := k.SetNamespace(namespace).Exec(podName, "sleep", command)
	Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("error connecting to the %q pod", podName))
	Expect(response).To(ContainSubstring(fmt.Sprintf("hello %s", echoStr)), fmt.Sprintf("Unexpected response from %s pod", podName))
}
