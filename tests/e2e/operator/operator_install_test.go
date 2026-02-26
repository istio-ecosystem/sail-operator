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

package operator

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"time"

	"github.com/istio-ecosystem/sail-operator/pkg/env"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/cleaner"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	. "github.com/istio-ecosystem/sail-operator/tests/e2e/util/gomega"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var sailCRDs = []string{
	// TODO: Find an alternative to this list
	"authorizationpolicies.security.istio.io",
	"destinationrules.networking.istio.io",
	"envoyfilters.networking.istio.io",
	"gateways.networking.istio.io",
	"istiorevisions.sailoperator.io",
	"istios.sailoperator.io",
	"peerauthentications.security.istio.io",
	"proxyconfigs.networking.istio.io",
	"requestauthentications.security.istio.io",
	"serviceentries.networking.istio.io",
	"sidecars.networking.istio.io",
	"telemetries.telemetry.istio.io",
	"virtualservices.networking.istio.io",
	"wasmplugins.extensions.istio.io",
	"workloadentries.networking.istio.io",
	"workloadgroups.networking.istio.io",
}

var _ = Describe("Operator", Label("smoke", "operator"), Ordered, func() {
	SetDefaultEventuallyTimeout(180 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)
	clr := cleaner.New(cl)
	BeforeAll(func(ctx SpecContext) {
		clr.Record(ctx)
	})

	Describe("installation", func() {
		It("deploys all the CRDs", func(ctx SpecContext) {
			Eventually(common.GetList).WithArguments(ctx, cl, &apiextensionsv1.CustomResourceDefinitionList{}).
				Should(WithTransform(extractCRDNames, ContainElements(sailCRDs)),
					"Not all Istio and Sail CRDs are present")
			Success("Istio CRDs are present")
		})

		It("updates the CRDs status to Established", func(ctx SpecContext) {
			for _, crdName := range sailCRDs {
				common.AwaitCondition(ctx, apiextensionsv1.Established, kube.Key(crdName), &apiextensionsv1.CustomResourceDefinition{}, k, cl)
			}
			Success("CRDs are Established")
		})

		Specify("istio crd is present", func(ctx SpecContext) {
			// When the operator runs in OCP cluster, the CRD is created but not available at the moment
			Eventually(cl.Get).WithArguments(ctx, kube.Key("istios.sailoperator.io"), &apiextensionsv1.CustomResourceDefinition{}).
				Should(Succeed(), "Error getting Istio CRD")
			Success("Istio CRD is present")
		})

		It("starts successfully", func(ctx SpecContext) {
			Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(deploymentName, namespace), &appsv1.Deployment{}).
				Should(HaveConditionStatus(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Error getting Deployment status")
		})

		It("serves metrics securely", func(ctx SpecContext) {
			metricsReaderRoleName := "metrics-reader"
			metricsServiceName := deploymentName + "-metrics-service"

			By("creating a ClusterRoleBinding for the service account to allow access to metrics")
			err := k.CreateFromString(fmt.Sprintf(`
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: metrics-reader-rolebinding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: %s
subjects:
- kind: ServiceAccount
  name: %s
  namespace: %s
`, metricsReaderRoleName, deploymentName, namespace))
			Expect(err).NotTo(HaveOccurred(), "Failed to create ClusterRoleBinding")

			By("validating that the metrics service is available")
			cmd := exec.Command("kubectl", "get", "service", metricsServiceName, "-n", namespace)
			err = cmd.Run()
			Expect(err).NotTo(HaveOccurred(), "Metrics service should exist")

			By("getting the service account token")
			token, err := serviceAccountToken()
			Expect(err).NotTo(HaveOccurred())
			Expect(token).NotTo(BeEmpty())

			By("waiting for the metrics endpoint to be ready")
			verifyMetricsEndpointReady := func(g Gomega) {
				output, err := k.WithNamespace(namespace).GetYAML("endpoints", metricsServiceName)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("8443"), "Metrics endpoint is not ready")
			}
			Eventually(verifyMetricsEndpointReady).Should(Succeed())

			By("verifying that the controller manager is serving the metrics server")
			verifyMetricsServerStarted := func(g Gomega) {
				output, err := k.WithNamespace(namespace).Logs("deployment/"+deploymentName, nil)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("controller-runtime.metrics\tServing metrics server"),
					"Metrics server not yet started")
			}
			Eventually(verifyMetricsServerStarted).Should(Succeed())

			By("creating the curl-metrics namespace")
			Expect(k.CreateNamespace(curlNamespace)).To(Succeed(), "Namespace failed to be created")

			By("creating the curl-metrics pod to access the metrics endpoint")
			err = k.CreateFromString(fmt.Sprintf(`
apiVersion: batch/v1
kind: Job
metadata:
  name: curl-metrics
  namespace: %s
spec:
  template:
    spec:
      containers:
      - name: curl-metrics
        image: quay.io/curl/curl:8.11.1
        command: ['curl', '-v', '-k', '-H', 'Authorization: Bearer %s', 'https://%s.%s.svc.cluster.local:8443/metrics']
      restartPolicy: Never
`, curlNamespace, token, metricsServiceName, namespace))
			Expect(err).NotTo(HaveOccurred(), "Failed to create curl-metrics pod")

			By("waiting for the curl-metrics pod to complete.")
			verifyCurlUp := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "jobs", "curl-metrics",
					"-o", "jsonpath={.status.succeeded}",
					"-n", curlNamespace)
				output, err := cmd.Output()
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(string(output)).To(Equal("1"), "curl pod in wrong status")
			}
			Eventually(verifyCurlUp, 5*time.Minute).Should(Succeed())

			By("getting the metrics by checking curl-metrics logs")
			metricsOutput := getMetricsOutput()
			Expect(metricsOutput).To(ContainSubstring(
				"controller_runtime_reconcile_total",
			))
		})
	})

	Describe("TLS profile change", Label("openshift"), func() {
		var originalTLSProfile *configv1.TLSSecurityProfile
		apiServerKey := client.ObjectKey{Name: "cluster"}
		const sailOperatorContainerName = "sail-operator"

		BeforeAll(func(ctx SpecContext) {
			if !env.GetBool("OCP", false) {
				Skip("Skipping OpenShift-specific tests on non-OpenShift cluster")
			}

			Step("Saving the original APIServer tlsSecurityProfile")
			apiServer := &configv1.APIServer{}
			err := cl.Get(ctx, apiServerKey, apiServer)
			Expect(err).NotTo(HaveOccurred(), "Failed to get APIServer")
			if apiServer.Spec.TLSSecurityProfile != nil {
				originalTLSProfile = apiServer.Spec.TLSSecurityProfile.DeepCopy()
			}
		})

		AfterAll(func(ctx SpecContext) {
			Step("Restoring the original APIServer tlsSecurityProfile")

			apiServer := &configv1.APIServer{}
			err := cl.Get(ctx, apiServerKey, apiServer)
			Expect(err).NotTo(HaveOccurred(), "Failed to get APIServer for restoration")

			apiServer.Spec.TLSSecurityProfile = originalTLSProfile
			err = cl.Update(ctx, apiServer)
			Expect(err).NotTo(HaveOccurred(), "Failed to restore original tlsSecurityProfile")

			Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(deploymentName, namespace), &appsv1.Deployment{}).
				Should(HaveConditionStatus(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Operator should be available after restoration")
		})

		It("restarts the operator pod when the APIServer tlsSecurityProfile changes", func(ctx SpecContext) {
			operatorPodLabel := client.MatchingLabels{"control-plane": "sail-operator"}

			Step("Getting the current operator pod restart count")
			var initialRestartCount int32
			Eventually(func(g Gomega) {
				podList := &corev1.PodList{}
				err := cl.List(ctx, podList,
					client.InNamespace(namespace),
					operatorPodLabel)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(podList.Items).NotTo(BeEmpty(), "Operator pod should exist")
				podIdx := slices.IndexFunc(podList.Items, func(pod corev1.Pod) bool {
					return pod.Status.Phase == corev1.PodRunning
				})
				g.Expect(podIdx).NotTo(Equal(-1), "Operator pod should be running")
				runningPod := &podList.Items[podIdx]
				containerIdx := slices.IndexFunc(runningPod.Status.ContainerStatuses, func(cs corev1.ContainerStatus) bool {
					return cs.Name == sailOperatorContainerName
				})
				g.Expect(containerIdx).NotTo(Equal(-1), "Operator container should be present")
				initialRestartCount = runningPod.Status.ContainerStatuses[containerIdx].RestartCount
			}).Should(Succeed())

			Step("Applying a Custom TLS profile (superficial change from Intermediate)")
			// Ciphers match the Intermediate profile from: kubectl explain apiservers.spec.tlsSecurityProfile.intermediate
			ciphers := []string{
				"TLS_AES_128_GCM_SHA256",
				"TLS_AES_256_GCM_SHA384",
				"TLS_CHACHA20_POLY1305_SHA256",
				"ECDHE-ECDSA-AES128-GCM-SHA256",
				"ECDHE-RSA-AES128-GCM-SHA256",
				"ECDHE-ECDSA-AES256-GCM-SHA384",
				"ECDHE-RSA-AES256-GCM-SHA384",
				"ECDHE-ECDSA-CHACHA20-POLY1305",
				"ECDHE-RSA-CHACHA20-POLY1305",
				"DHE-RSA-AES128-GCM-SHA256",
				"DHE-RSA-AES256-GCM-SHA384",
				// Add an old cipher to ensure the operator restarts.
				// Note: it matters which cipher you add because unknown
				// ciphers are silently dropped by the library-go/crypto package.
				"AES256-SHA",
			}

			apiServer := &configv1.APIServer{}
			err := cl.Get(ctx, apiServerKey, apiServer)
			Expect(err).NotTo(HaveOccurred(), "Failed to get APIServer")

			apiServer.Spec.TLSSecurityProfile = &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
				Custom: &configv1.CustomTLSProfile{
					TLSProfileSpec: configv1.TLSProfileSpec{
						Ciphers:       ciphers,
						MinTLSVersion: configv1.VersionTLS12,
					},
				},
			}
			err = cl.Update(ctx, apiServer)
			Expect(err).NotTo(HaveOccurred(), "Failed to update APIServer with custom TLS profile")
			Success("Applied Custom TLS profile to APIServer")

			Step("Waiting for the operator container to be restarted")
			Eventually(func(g Gomega) {
				podList := &corev1.PodList{}
				err := cl.List(ctx, podList,
					client.InNamespace(namespace),
					operatorPodLabel)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(podList.Items).NotTo(BeEmpty(), "Operator pod should exist")

				podIdx := slices.IndexFunc(podList.Items, func(pod corev1.Pod) bool {
					return pod.Status.Phase == corev1.PodRunning
				})
				g.Expect(podIdx).NotTo(Equal(-1), "Operator pod should be running")
				runningPod := &podList.Items[podIdx]
				containerIdx := slices.IndexFunc(runningPod.Status.ContainerStatuses, func(cs corev1.ContainerStatus) bool {
					return cs.Name == sailOperatorContainerName
				})
				g.Expect(containerIdx).NotTo(Equal(-1), "Operator container should be present")
				g.Expect(runningPod.Status.ContainerStatuses[containerIdx].RestartCount).To(BeNumerically(">", initialRestartCount),
					"Operator container should have been restarted")
			}, 5*time.Minute, 5*time.Second).Should(Succeed())
			Success("Operator container was restarted after TLS profile change")

			Step("Verifying the operator is available after restart")
			Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(deploymentName, namespace), &appsv1.Deployment{}).
				Should(HaveConditionStatus(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Operator should be available after TLS profile change")
			Success("Operator is available after TLS profile change")
		})
	})

	AfterAll(func(ctx SpecContext) {
		if CurrentSpecReport().Failed() && keepOnFailure {
			return
		}

		if CurrentSpecReport().Failed() {
			common.LogDebugInfo(common.Operator, k)
		}
		clr.Cleanup(ctx)
	})
})

func extractCRDNames(crdList *apiextensionsv1.CustomResourceDefinitionList) []string {
	var names []string
	for _, crd := range crdList.Items {
		names = append(names, crd.ObjectMeta.Name)
	}
	return names
}

// serviceAccountToken returns a token for the specified service account in the given namespace.
// It uses the Kubernetes TokenRequest API to generate a token by directly sending a request
// and parsing the resulting token from the API response.
func serviceAccountToken() (string, error) {
	const tokenRequestRawString = `{
		"apiVersion": "authentication.k8s.io/v1",
		"kind": "TokenRequest"
	}`

	// Temporary file to store the token request
	secretName := fmt.Sprintf("%s-token-request", serviceAccountName)
	tokenRequestFile := filepath.Join("/tmp", secretName)
	err := os.WriteFile(tokenRequestFile, []byte(tokenRequestRawString), os.FileMode(0o644))
	if err != nil {
		return "", err
	}

	var out string
	verifyTokenCreation := func(g Gomega) {
		// Execute kubectl command to create the token
		cmd := exec.Command("kubectl", "create", "--raw", fmt.Sprintf(
			"/api/v1/namespaces/%s/serviceaccounts/%s/token",
			namespace,
			serviceAccountName,
		), "-f", tokenRequestFile)

		output, err := cmd.CombinedOutput()
		g.Expect(err).NotTo(HaveOccurred())

		// Parse the JSON output to extract the token
		var token tokenRequest
		err = json.Unmarshal(output, &token)
		g.Expect(err).NotTo(HaveOccurred())

		out = token.Status.Token
	}
	Eventually(verifyTokenCreation).Should(Succeed())

	return out, err
}

// getMetricsOutput retrieves and returns the logs from the curl pod used to access the metrics endpoint.
func getMetricsOutput() string {
	By("getting the curl-metrics logs")
	metricsOutput, err := k.WithNamespace(curlNamespace).Logs("jobs/curl-metrics", nil)
	Expect(err).NotTo(HaveOccurred(), "Failed to retrieve logs from curl pod")
	Expect(metricsOutput).To(ContainSubstring("< HTTP/1.1 200 OK"))
	return metricsOutput
}

// tokenRequest is a simplified representation of the Kubernetes TokenRequest API response,
// containing only the token field that we need to extract.
type tokenRequest struct {
	Status struct {
		Token string `json:"token"`
	} `json:"status"`
}
