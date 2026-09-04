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
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/env"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/cleaner"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	. "github.com/istio-ecosystem/sail-operator/tests/e2e/util/gomega"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/kubectl"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

var apiServerKey = client.ObjectKey{Name: "cluster"}

var _ = Describe("Operator", Label("smoke", "operator"), Ordered, func() {
	SetDefaultEventuallyTimeout(time.Duration(env.GetInt("DEFAULT_TEST_TIMEOUT", 180)) * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)
	clr := cleaner.New(cl)

	BeforeAll(func(ctx SpecContext) {
		clr.Record(ctx)
		DeferCleanup(func(ctx SpecContext) {
			if CurrentSpecReport().Failed() {
				common.LogDebugInfo(common.Operator, k)
			}

			clr.Cleanup(ctx)
		})
	})

	// Capture debug info immediately on test failure
	JustAfterEach(func(ctx SpecContext) {
		if CurrentSpecReport().Failed() {
			common.LogDebugInfo(common.Operator, k)
		}
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
			By("discovering the metrics reader ClusterRole installed on the cluster")
			metricsReaderRoleName, err := discoverMetricsReaderClusterRole()
			Expect(err).NotTo(HaveOccurred(), "metrics reader ClusterRole must exist")

			metricsServiceName := deploymentName + "-metrics-service"

			By("creating a ClusterRoleBinding for the service account to allow access to metrics")
			err = k.CreateFromString(fmt.Sprintf(`
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: metrics-reader-test-rolebinding
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

	// These tests verify the operator's TLS behavior when the APIServer TLS settings change.
	// The first test runs on all OpenShift clusters; the second requires OpenShift >= 4.22
	// because the TLSAdherence field was introduced in 4.22.
	// NOTE: Running this test may have side effects such as setting feature gates on OpenShift.
	Describe("TLS profile change", Label("tls-profile"), func() {
		var ocpVersion *semver.Version

		BeforeAll(func(ctx SpecContext) {
			if !env.GetBool("OCP", false) {
				Skip("Skipping OpenShift-specific tests on non-OpenShift cluster")
			}

			// On hosted clusters, the APIServer resource is read-only and TLS settings cannot be changed,
			// so skip all TLS profile tests. If the test run in a regular cluster, the APIServer resource is writable and the tests can run as normal.
			Step("Checking if this is a Hosted Cluster")
			infra := &configv1.Infrastructure{}
			infraErr := cl.Get(ctx, client.ObjectKey{Name: "cluster"}, infra)
			Expect(infraErr).NotTo(HaveOccurred(), "Failed to get Infrastructure resource")
			if infra.Status.ControlPlaneTopology == configv1.ExternalTopologyMode {
				Skip("Skipping TLS profile tests on hosted cluster: APIServer resource is read-only on hosted clusters")
			}

			// The TLS profile tests must update the cluster-scoped APIServer resource. Rather than
			// enumerate cluster flavors, probe writability directly: perform a no-op server-side
			// dry-run update and skip if admission rejects it. This covers managed clusters (e.g.
			// ROSA/OSD, whose Red Hat SRE webhooks forbid the write) and any other case where the
			// resource is not manageable, without persisting any change.
			Step("Checking whether the APIServer resource is writable")
			apiServerProbe := &configv1.APIServer{}
			Expect(cl.Get(ctx, apiServerKey, apiServerProbe)).To(Succeed(), "Failed to get APIServer")
			if probeErr := cl.Update(ctx, apiServerProbe, client.DryRunAll); probeErr != nil {
				if apierrors.IsForbidden(probeErr) {
					Skip("Skipping TLS profile tests: the APIServer resource is not writable on this cluster: " + probeErr.Error())
				}
				Expect(probeErr).NotTo(HaveOccurred(), "Unexpected error while probing APIServer writability")
			}

			Step("Determining OpenShift version")
			cv := &configv1.ClusterVersion{}
			err := cl.Get(ctx, client.ObjectKey{Name: "version"}, cv)
			Expect(err).NotTo(HaveOccurred(), "Failed to get ClusterVersion")
			ocpVersion, err = semver.NewVersion(cv.Status.Desired.Version)
			Expect(err).NotTo(HaveOccurred(), "Failed to parse ClusterVersion %q", cv.Status.Desired.Version)

			// TLSAdherence is behind a TechPreview feature gate on OCP 4.22+.
			// Enable it via CustomNoUpgrade only when TLSAdherence is still listed in the
			// cluster FeatureGate resource. On newer releases (e.g. 5.0) where TLSAdherence
			// is GA, it is no longer present in FeatureGate status and can be configured
			// directly on the APIServer resource.
			if ocpVersion.GreaterThanEqual(semver.MustParse("4.22.0")) {
				featureGate := &configv1.FeatureGate{}
				err = cl.Get(ctx, client.ObjectKey{Name: "cluster"}, featureGate)
				Expect(err).NotTo(HaveOccurred(), "Failed to get FeatureGate")

				// TLSAdherence is GA on OCP 5.0+ and is no longer listed in FeatureGate
				// status; only attempt to enable the gate when it is still a gated feature.
				if featureGateHasTLSAdherence(featureGate) {
					if !tlsAdherenceFeatureGateEnabled(featureGate) {
						Step("Enabling TLSAdherence feature gate")
						featureGate.Spec.FeatureSet = configv1.CustomNoUpgrade
						featureGate.Spec.CustomNoUpgrade = &configv1.CustomFeatureGates{
							Enabled: []configv1.FeatureGateName{"TLSAdherence"},
						}
						err = cl.Update(ctx, featureGate)
						Expect(err).NotTo(HaveOccurred(), "Failed to enable TLSAdherence feature gate")
						Success("TLSAdherence feature gate enabled")

						Step("Waiting for TLSAdherence feature gate to become active")
						Eventually(func(g Gomega) {
							fg := &configv1.FeatureGate{}
							g.Expect(cl.Get(ctx, client.ObjectKey{Name: "cluster"}, fg)).To(Succeed())
							found := false
							for _, details := range fg.Status.FeatureGates {
								for _, enabled := range details.Enabled {
									if enabled.Name == "TLSAdherence" {
										found = true
										break
									}
								}
							}
							g.Expect(found).To(BeTrue(),
								"TLSAdherence not yet listed in FeatureGate status; kube-apiserver may not have rolled out yet")
						}).WithTimeout(30*time.Minute).WithPolling(30*time.Second).Should(Succeed(),
							"TLSAdherence feature gate should appear in FeatureGate status after rollout")
						Success("TLSAdherence feature gate is active")

						Step("Waiting for kube-apiserver to finish rolling out after feature gate change")
						waitForAPIServerStable(ctx, cl)
						Success("kube-apiserver is stable after feature gate change")
					}
				} else {
					Log(fmt.Sprintf("TLSAdherence is not listed in FeatureGate on OpenShift %s; assuming it is a GA feature", ocpVersion))
				}
			}

			Step("Saving the original APIServer TLS settings")
			apiServer := &configv1.APIServer{}
			err = cl.Get(ctx, apiServerKey, apiServer)
			Expect(err).NotTo(HaveOccurred(), "Failed to get APIServer")
			var originalTLSProfile *configv1.TLSSecurityProfile
			if apiServer.Spec.TLSSecurityProfile != nil {
				originalTLSProfile = apiServer.Spec.TLSSecurityProfile.DeepCopy()
			}
			originalTLSAdherence := apiServer.Spec.TLSAdherence

			DeferCleanup(func(ctx SpecContext) {
				Step("Restoring the original APIServer TLS settings")
				Eventually(func(g Gomega) {
					apiServer := &configv1.APIServer{}
					g.Expect(cl.Get(ctx, apiServerKey, apiServer)).To(Succeed(), "Failed to get APIServer")
					apiServer.Spec.TLSSecurityProfile = originalTLSProfile
					// TLSAdherence cannot be set back to NoOpinion once set,
					// so only restore it if the original was a non-empty value.
					if originalTLSAdherence != configv1.TLSAdherencePolicyNoOpinion {
						apiServer.Spec.TLSAdherence = originalTLSAdherence
					}
					g.Expect(cl.Update(ctx, apiServer)).To(Succeed(), "Failed to update APIServer TLS settings")
				}).WithTimeout(30 * time.Second).WithPolling(2 * time.Second).Should(Succeed())

				Step("Waiting for kube-apiserver to finish rolling out after restoring TLS settings")
				waitForAPIServerStable(ctx, cl)
			})

			Step("Creating Istio")
			// The cleaner should delete these.
			common.EnsureNamespace(ctx, cl, common.ControlPlaneNamespace)
			common.CreateIstio(k, istioversion.Default)

			Step("Creating IstioCNI")
			common.EnsureNamespace(ctx, cl, common.IstioCniNamespace)
			common.CreateIstioCNI(k, istioversion.Default)

			Step("Waiting for IstioRevision to be healthy")
			common.AwaitCondition(ctx, v1.IstioRevisionConditionReady, kube.Key("default"), &v1.IstioRevision{}, k, cl)
		})

		// When TLSAdherence is NoOpinion (empty), TLS profile changes should not
		// affect the operator's metrics TLS or the Istio resource values. This covers
		// both OpenShift <= 4.21 (where the TLSAdherence field does not exist) and
		// 4.22+ (where it defaults to NoOpinion). Runs on all OpenShift versions.
		// Note: TLSAdherence cannot be set back to NoOpinion once set, so this test
		// must run first and requires the cluster to have the default NoOpinion state.
		It("does not sync TLS settings when TLSAdherence is NoOpinion", func(ctx SpecContext) {
			Step("Verifying TLSAdherence is NoOpinion")
			apiServer := &configv1.APIServer{}
			Expect(cl.Get(ctx, apiServerKey, apiServer)).To(Succeed(), "Failed to get APIServer")
			if apiServer.Spec.TLSAdherence != configv1.TLSAdherencePolicyNoOpinion {
				Skip(fmt.Sprintf("TLSAdherence is already set to %q; cannot reset to NoOpinion. Skipping test.", apiServer.Spec.TLSAdherence))
			}

			Step("Verifying IstioRevision does not have TLS cipher suites")
			Consistently(func(g Gomega) {
				rev := &v1.IstioRevision{}
				g.Expect(cl.Get(ctx, client.ObjectKey{Name: "default"}, rev)).To(Succeed())
				g.Expect(getIstioRevisionCipherSuites(rev)).To(BeEmpty(),
					"IstioRevision should not have cipher suites when TLSAdherence is NoOpinion")
			}).WithTimeout(30 * time.Second).WithPolling(5 * time.Second).Should(Succeed())

			Step("Verifying metrics endpoint accepts a TLS 1.2 cipher")
			Expect(metricsEndpointAcceptsCipher(k, "ECDHE-RSA-AES256-GCM-SHA384", "1.2")).To(BeTrue(),
				"Metrics endpoint should accept TLS 1.2 ciphers when TLSAdherence is NoOpinion")
			Success("TLS settings were not synced when TLSAdherence is NoOpinion")
		})

		// When TLSAdherence changes to StrictAllComponents, the operator should
		// apply the TLS profile to both the metrics endpoint and the Istio resource.
		It("syncs TLS settings when TLSAdherence is set to StrictAllComponents", func(ctx SpecContext) {
			if !ocpVersion.GreaterThanEqual(semver.MustParse("4.22.0")) {
				Skip(fmt.Sprintf("TLSAdherence field requires OpenShift >= 4.22. Current version: '%s'. Skipping test.", ocpVersion))
			}

			Step("Clearing TLS profile")
			apiServer := &configv1.APIServer{}
			Expect(cl.Get(ctx, apiServerKey, apiServer)).To(Succeed(), "Failed to get APIServer")
			apiServer.Spec.TLSSecurityProfile = nil
			Expect(cl.Update(ctx, apiServer)).To(Succeed(), "Failed to update APIServer TLS profile")

			ensureTLSAdherence(ctx, cl, configv1.TLSAdherencePolicyLegacyAdheringComponentsOnly)

			Eventually(func(g Gomega) {
				rev := &v1.IstioRevision{}
				g.Expect(cl.Get(ctx, client.ObjectKey{Name: "default"}, rev)).To(Succeed())
				ciphers := getIstioRevisionCipherSuites(rev)
				g.Expect(ciphers).To(BeEmpty(), "IstioRevision should not have cipher suites")

				var pilotExtraArgs []string
				if rev.Spec.Values != nil && rev.Spec.Values.Pilot != nil {
					pilotExtraArgs = rev.Spec.Values.Pilot.ExtraContainerArgs
				}
				g.Expect(pilotExtraArgs).To(Not(ContainElement(ContainSubstring("--tls-cipher-suites="))))
			}).Should(Succeed(), "IstioRevision is syncing TLS settings but should NOT be")

			ensureTLSAdherence(ctx, cl, configv1.TLSAdherencePolicyStrictAllComponents)

			Step("Verifying IstioRevision has TLS cipher suites from the Intermediate profile")
			Eventually(func(g Gomega) {
				rev := &v1.IstioRevision{}
				g.Expect(cl.Get(ctx, client.ObjectKey{Name: "default"}, rev)).To(Succeed())
				ciphers := getIstioRevisionCipherSuites(rev)
				g.Expect(ciphers).NotTo(BeEmpty(), "IstioRevision should have cipher suites")

				g.Expect(rev.Spec.Values.Pilot).NotTo(BeNil())
				g.Expect(rev.Spec.Values.Pilot.ExtraContainerArgs).To(
					ContainElement(ContainSubstring("--tls-cipher-suites=")),
					"IstioRevision should have --tls-cipher-suites in pilot.extraContainerArgs",
				)
			}).WithTimeout(5*time.Minute).WithPolling(5*time.Second).Should(Succeed(),
				"IstioRevision is not syncing TLS settings but should be")

			Step("Applying Modern TLS profile (TLS 1.3 only)")
			applyModernTLSProfile(ctx, cl)

			Step("Verifying IstioRevision has TLS 1.3 settings from the Modern profile")
			Eventually(func(g Gomega) {
				rev := &v1.IstioRevision{}
				g.Expect(cl.Get(ctx, client.ObjectKey{Name: "default"}, rev)).To(Succeed())
				g.Expect(rev.Spec.Values).NotTo(BeNil())
				g.Expect(rev.Spec.Values.MeshConfig).NotTo(BeNil())

				g.Expect(rev.Spec.Values.MeshConfig.TlsDefaults).NotTo(BeNil())
				g.Expect(rev.Spec.Values.MeshConfig.TlsDefaults.MinProtocolVersion).To(
					Equal(v1.MeshConfigTLSConfigTLSProtocolTlsv13),
					"tlsDefaults.minProtocolVersion should be TLSV1_3",
				)
				g.Expect(rev.Spec.Values.MeshConfig.TlsDefaults.CipherSuites).To(BeEmpty(),
					"tlsDefaults.cipherSuites should be empty for TLS 1.3")

				g.Expect(rev.Spec.Values.MeshConfig.MeshMTLS).NotTo(BeNil())
				g.Expect(rev.Spec.Values.MeshConfig.MeshMTLS.MinProtocolVersion).To(
					Equal(v1.MeshConfigTLSConfigTLSProtocolTlsv13),
					"meshMTLS.minProtocolVersion should be TLSV1_3",
				)
				g.Expect(rev.Spec.Values.MeshConfig.MeshMTLS.CipherSuites).To(BeEmpty(),
					"meshMTLS.cipherSuites should be empty for TLS 1.3")

				proxyMeta := getIstioRevisionProxyMetadata(rev)
				g.Expect(proxyMeta).To(HaveKey("OPENSSL_TLS1_3_CIPHERSUITES"),
					"proxyMetadata should contain OPENSSL_TLS1_3_CIPHERSUITES for TLS 1.3")
				tls13Ciphers := proxyMeta["OPENSSL_TLS1_3_CIPHERSUITES"]
				g.Expect(tls13Ciphers).NotTo(BeEmpty(),
					"OPENSSL_TLS1_3_CIPHERSUITES should not be empty")
				for _, cipher := range configv1.TLSProfiles[configv1.TLSProfileModernType].Ciphers {
					g.Expect(tls13Ciphers).To(ContainSubstring(cipher),
						fmt.Sprintf("OPENSSL_TLS1_3_CIPHERSUITES should contain %s from the Modern profile", cipher))
				}
			}).WithTimeout(5*time.Minute).WithPolling(5*time.Second).Should(Succeed(),
				"IstioRevision should have TLS 1.3 settings from the Modern profile")

			Step("Verifying metrics endpoint rejects TLS 1.2 connections")
			Eventually(func() bool {
				return metricsEndpointAcceptsCipher(k, "ECDHE-RSA-AES256-GCM-SHA384", "1.2")
			}).Should(BeFalse(),
				"Metrics endpoint should reject TLS 1.2 connections when Modern profile is active")
			Success("TLS settings were synced after switching to Modern profile")

			Step("Deploying a proxy pod to verify synced TLS settings produce a healthy sidecar")
			const tlsTestNamespace = "tls-test"
			Expect(k.CreateNamespace(tlsTestNamespace)).To(Succeed(), "Failed to create test namespace")
			DeferCleanup(func() {
				_ = k.Delete("namespace", tlsTestNamespace)
			})
			Expect(k.Label("namespace", tlsTestNamespace, "istio-injection", "enabled")).To(Succeed(),
				"Failed to label namespace for sidecar injection")
			Expect(k.WithNamespace(tlsTestNamespace).ApplyKustomize("sleep")).To(Succeed(),
				"Failed to deploy sleep workload")

			Eventually(func() error {
				return common.CheckPodsReady(ctx, cl, tlsTestNamespace)
			}).WithTimeout(5*time.Minute).WithPolling(5*time.Second).Should(Succeed(),
				"Sleep pod with sidecar should be ready after TLS settings are synced")
			Success("Proxy pod is healthy with synced TLS settings")
		})
	})

	AfterAll(func(ctx SpecContext) {
		if CurrentSpecReport().Failed() && keepOnFailure {
			return
		}
	})
})

func extractCRDNames(crdList *apiextensionsv1.CustomResourceDefinitionList) []string {
	var names []string
	for _, crd := range crdList.Items {
		names = append(names, crd.ObjectMeta.Name)
	}
	return names
}

const metricsReaderClusterRoleLabel = "app.kubernetes.io/component=kube-rbac-proxy"

// discoverMetricsReaderClusterRole lists ClusterRoles with the kube-rbac-proxy component label
// (same as chart/bundle), then picks the one whose name ends with "-metrics-reader".
func discoverMetricsReaderClusterRole() (string, error) {
	out, err := k.GetClusterRoleNamesByLabel(metricsReaderClusterRoleLabel)
	if err != nil {
		return "", err
	}
	var matches []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name := line
		if i := strings.LastIndex(line, "/"); i >= 0 {
			name = line[i+1:]
		}
		if strings.HasSuffix(name, "-metrics-reader") {
			matches = append(matches, name)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no *-metrics-reader ClusterRole under label %s", metricsReaderClusterRoleLabel)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("ambiguous *-metrics-reader ClusterRoles: %v", matches)
	}
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

func featureGateHasTLSAdherence(featureGate *configv1.FeatureGate) bool {
	return slices.ContainsFunc(featureGate.Status.FeatureGates, func(fg configv1.FeatureGateDetails) bool {
		return featureGateAttributesContains(fg.Enabled, "TLSAdherence") ||
			featureGateAttributesContains(fg.Disabled, "TLSAdherence")
	})
}

func tlsAdherenceFeatureGateEnabled(featureGate *configv1.FeatureGate) bool {
	return slices.ContainsFunc(featureGate.Status.FeatureGates, func(fg configv1.FeatureGateDetails) bool {
		return featureGateAttributesContains(fg.Enabled, "TLSAdherence")
	})
}

func featureGateAttributesContains(attrs []configv1.FeatureGateAttributes, name string) bool {
	return slices.ContainsFunc(attrs, func(attr configv1.FeatureGateAttributes) bool {
		return attr.Name == configv1.FeatureGateName(name)
	})
}

func waitForAPIServerStable(ctx context.Context, cl client.Client) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		co := &configv1.ClusterOperator{}
		g.Expect(cl.Get(ctx, client.ObjectKey{Name: "kube-apiserver"}, co)).To(Succeed())
		for _, cond := range co.Status.Conditions {
			if cond.Type == configv1.OperatorProgressing {
				g.Expect(cond.Status).To(Equal(configv1.ConditionFalse), "kube-apiserver should not be Progressing")
			}
		}
	}).WithTimeout(30*time.Minute).WithPolling(30*time.Second).Should(Succeed(),
		"kube-apiserver should finish rolling out")
}

func ensureTLSAdherence(ctx context.Context, cl client.Client, policy configv1.TLSAdherencePolicy) {
	GinkgoHelper()

	apiServer := &configv1.APIServer{}
	Expect(cl.Get(ctx, apiServerKey, apiServer)).To(Succeed(), "Failed to get APIServer")

	if apiServer.Spec.TLSAdherence == policy {
		return
	}

	totalRestarts := func(pod corev1.Pod) int32 {
		var n int32
		for _, cs := range pod.Status.ContainerStatuses {
			n += cs.RestartCount
		}
		return n
	}
	podReady := func(pod corev1.Pod) bool {
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady {
				return cond.Status == corev1.ConditionTrue
			}
		}
		return false
	}

	oldRestarts := make(map[string]int32)
	podList := &corev1.PodList{}
	Expect(cl.List(ctx, podList, client.InNamespace(namespace),
		client.MatchingLabels{"control-plane": deploymentName})).To(Succeed(),
		"Failed to list operator pods before TLSAdherence change")
	for _, pod := range podList.Items {
		oldRestarts[string(pod.UID)] = totalRestarts(pod)
	}

	Step(fmt.Sprintf("Updating APIServer TLSAdherence from: %s to %s", apiServer.Spec.TLSAdherence, policy))
	apiServer.Spec.TLSAdherence = policy
	Expect(cl.Update(ctx, apiServer)).To(Succeed(), "Failed to update APIServer TLSAdherence")

	Step("Waiting for operator to restart and be ready after TLSAdherence change")
	Eventually(func(g Gomega) {
		pods := &corev1.PodList{}
		g.Expect(cl.List(ctx, pods, client.InNamespace(namespace),
			client.MatchingLabels{"control-plane": deploymentName})).To(Succeed())

		restarted := false
		for _, pod := range pods.Items {
			if pod.DeletionTimestamp != nil || pod.Status.Phase != corev1.PodRunning || !podReady(pod) {
				continue
			}
			if prev, seen := oldRestarts[string(pod.UID)]; !seen || totalRestarts(pod) > prev {
				restarted = true
				break
			}
		}
		g.Expect(restarted).To(BeTrue(), "Operator did not restart (in place or via a new pod) after TLSAdherence change to %s", policy)
	}).WithTimeout(5*time.Minute).WithPolling(5*time.Second).Should(Succeed(),
		"Operator should restart and be ready after TLSAdherence change")
}

func applyModernTLSProfile(ctx context.Context, cl client.Client) {
	GinkgoHelper()
	Step("Applying Modern TLS profile")

	apiServer := &configv1.APIServer{}
	Expect(cl.Get(ctx, apiServerKey, apiServer)).To(Succeed(), "Failed to get APIServer")
	apiServer.Spec.TLSSecurityProfile = &configv1.TLSSecurityProfile{
		Type:   configv1.TLSProfileModernType,
		Modern: &configv1.ModernTLSProfile{},
	}
	Expect(cl.Update(ctx, apiServer)).To(Succeed(), "Failed to update APIServer with Modern TLS profile")
	Success("Applied Modern TLS profile to APIServer")

	Step("Waiting for kube-apiserver to finish rolling out after TLS profile change")
	waitForAPIServerStable(ctx, cl)
	Success("kube-apiserver is stable after TLS profile change")
}

func getIstioRevisionCipherSuites(rev *v1.IstioRevision) []string {
	if rev.Spec.Values == nil || rev.Spec.Values.MeshConfig == nil || rev.Spec.Values.MeshConfig.TlsDefaults == nil {
		return nil
	}
	return rev.Spec.Values.MeshConfig.TlsDefaults.CipherSuites
}

func getIstioRevisionProxyMetadata(rev *v1.IstioRevision) map[string]string {
	if rev.Spec.Values == nil || rev.Spec.Values.MeshConfig == nil || rev.Spec.Values.MeshConfig.DefaultConfig == nil {
		return nil
	}
	return rev.Spec.Values.MeshConfig.DefaultConfig.ProxyMetadata
}

// metricsEndpointAcceptsCipher tests whether the operator's metrics endpoint
// accepts a TLS connection using the specified cipher and maximum TLS version.
// It creates a curl Job that caps the TLS version at maxTLSVersion (e.g. "1.2")
// and offers only the given cipher. Returns true if the connection succeeds,
// false if the TLS handshake is rejected.
func metricsEndpointAcceptsCipher(k kubectl.Kubectl, cipher, maxTLSVersion string) bool {
	GinkgoHelper()
	// Generate a random suffix to avoid conflicts with other tests.
	var randBytes [4]byte
	_, _ = rand.Read(randBytes[:])
	jobName := fmt.Sprintf("tls-check-%x", randBytes)
	metricsServiceName := deploymentName + "-metrics-service"

	token, err := serviceAccountToken()
	Expect(err).NotTo(HaveOccurred())
	Expect(token).NotTo(BeEmpty())

	Expect(k.CreateNamespace(curlNamespace)).To(Succeed(), "Failed to create curl namespace")

	err = k.CreateFromString(fmt.Sprintf(`
apiVersion: batch/v1
kind: Job
metadata:
  name: %s
  namespace: %s
spec:
  backoffLimit: 0
  template:
    spec:
      containers:
      - name: curl
        image: quay.io/curl/curl:8.11.1
        command: ['curl', '-s', '-o', '/dev/null', '-k', '--max-time', '10',
                  '--tls-max', '%s', '--ciphers', '%s',
                  '-H', 'Authorization: Bearer %s',
                  'https://%s.%s.svc.cluster.local:8443/metrics']
      restartPolicy: Never
`, jobName, curlNamespace, maxTLSVersion, cipher, token, metricsServiceName, namespace))
	Expect(err).NotTo(HaveOccurred(), "Failed to create TLS check job")

	DeferCleanup(func() {
		_ = exec.Command("kubectl", "delete", "job", jobName, "-n", curlNamespace, "--ignore-not-found").Run()
	})

	// Wait for the Job to complete (succeeded or failed)
	Eventually(func() bool {
		succeeded, _ := exec.Command("kubectl", "get", "jobs", jobName,
			"-o", "jsonpath={.status.succeeded}",
			"-n", curlNamespace).Output()
		failed, _ := exec.Command("kubectl", "get", "jobs", jobName,
			"-o", "jsonpath={.status.failed}",
			"-n", curlNamespace).Output()
		return string(succeeded) == "1" || string(failed) == "1"
	}, 2*time.Minute).Should(BeTrue(), "TLS check job should complete")

	output, err := exec.Command("kubectl", "get", "jobs", jobName,
		"-o", "jsonpath={.status.succeeded}",
		"-n", curlNamespace).Output()
	Expect(err).NotTo(HaveOccurred())
	return string(output) == "1"
}
