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

package library

import (
	"context"
	"fmt"
	"time"

	"github.com/istio-ecosystem/sail-operator/chart"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	"github.com/istio-ecosystem/sail-operator/pkg/install"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/resources"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"

	"istio.io/istio/pkg/ptr"
)

const (
	tlsTestNamespace     = "library-test-tls"
	loopTestNamespace    = "library-test-loop"
	driftTestNamespace   = "library-test-drift"
	upgradeTestNamespace = "library-test-upgrade"
)

var _ = Describe("Library Reconciliation", Label("library", "reconciliation"), Ordered, func() {
	SetDefaultEventuallyTimeout(3 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	ctx := context.Background()

	When("TLS options are passed via Options", func() {
		var lib *install.Library

		const (
			namespace = tlsTestNamespace
			revision  = "tlstest"
		)

		openshiftTLS := &config.OpenShiftTLS{
			TLSProfileSpec: configv1.TLSProfileSpec{
				Ciphers: []string{
					"ECDHE-RSA-AES128-GCM-SHA256",
					"ECDHE-RSA-AES256-GCM-SHA384",
				},
				MinTLSVersion: configv1.VersionTLS12,
			},
			TLSAdherencePolicy: configv1.TLSAdherencePolicyStrictAllComponents,
		}

		BeforeAll(func() {
			common.EnsureNamespaceWithCleanup(k, namespace)

			var err error
			lib, err = install.New(kubeConfig, resources.FS, chart.CRDsFS)
			Expect(err).NotTo(HaveOccurred())

			_, err = lib.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			DeferCleanup(func() {
				Expect(lib.Uninstall(ctx, namespace, revision)).To(Succeed())
				lib.Stop()
				Success("Cleaned up TLS test")
			})

			Expect(lib.Apply(install.Options{
				Values:         install.GatewayAPIDefaults(namespace),
				Namespace:      namespace,
				Version:        istioversion.Default,
				Revision:       revision,
				OpenShiftTLS:   openshiftTLS,
				ManageCRDs:     true,
				IncludeAllCRDs: true,
			})).To(Succeed())

			Eventually(func(g Gomega) {
				s := lib.Status()
				g.Expect(s.Error).NotTo(HaveOccurred(), "library install error")
				g.Expect(s.Installed).To(BeTrue(), "library not yet installed")
			}).Should(Succeed())
			Success("Library installed with TLS config")
		})

		It("propagates cipher suites and min version to istiod container args", func(ctx SpecContext) {
			Eventually(func(g Gomega) {
				deploy := &appsv1.Deployment{}
				g.Expect(cl.Get(ctx, types.NamespacedName{
					Name:      "istiod-" + revision,
					Namespace: namespace,
				}, deploy)).To(Succeed())

				var discoveryArgs []string
				for _, c := range deploy.Spec.Template.Spec.Containers {
					if c.Name == "discovery" {
						discoveryArgs = c.Args
						break
					}
				}
				g.Expect(discoveryArgs).NotTo(BeEmpty(), "discovery container not found or has no args")

				g.Expect(discoveryArgs).To(ContainElement(
					"--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"))
				g.Expect(discoveryArgs).To(ContainElement("--tls-min-version=1.2"))
			}).Should(Succeed())
			Success("Istiod deployment has correct TLS args")
		})

		It("propagates TLS defaults to the mesh config", func(ctx SpecContext) {
			Eventually(func(g Gomega) {
				cm := &corev1.ConfigMap{}
				g.Expect(cl.Get(ctx, types.NamespacedName{
					Name:      "istio-" + revision,
					Namespace: namespace,
				}, cm)).To(Succeed())

				meshYAML, ok := cm.Data["mesh"]
				g.Expect(ok).To(BeTrue(), "ConfigMap missing 'mesh' key")

				var meshConfig map[string]any
				g.Expect(yaml.Unmarshal([]byte(meshYAML), &meshConfig)).To(Succeed())

				tlsDefaults, _ := meshConfig["tlsDefaults"].(map[string]any)
				g.Expect(tlsDefaults).NotTo(BeNil(), "meshConfig.tlsDefaults not found")
				g.Expect(tlsDefaults["minProtocolVersion"]).To(Equal("TLSV1_2"))
				ciphers, _ := tlsDefaults["cipherSuites"].([]any)
				g.Expect(ciphers).To(ContainElements("TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"))

				meshMTLS, _ := meshConfig["meshMTLS"].(map[string]any)
				g.Expect(meshMTLS).NotTo(BeNil(), "meshConfig.meshMTLS not found")
				g.Expect(meshMTLS["minProtocolVersion"]).To(Equal("TLSV1_2"))
				mtlsCiphers, _ := meshMTLS["cipherSuites"].([]any)
				g.Expect(mtlsCiphers).To(ContainElements("TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"))
			}).Should(Succeed())
			Success("Mesh config has correct TLS defaults")
		})
	})

	When("the library reconcile loop stabilizes after install", func() {
		var lib *install.Library

		const (
			namespace = loopTestNamespace
			revision  = "crdtest"
		)

		installOpts := install.Options{
			Values:         install.GatewayAPIDefaults(namespace),
			Namespace:      namespace,
			Version:        istioversion.Default,
			Revision:       revision,
			ManageCRDs:     true,
			IncludeAllCRDs: true,
		}

		BeforeAll(func() {
			common.EnsureNamespaceWithCleanup(k, namespace)

			var err error
			lib, err = install.New(kubeConfig, resources.FS, chart.CRDsFS)
			Expect(err).NotTo(HaveOccurred())

			_, err = lib.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			DeferCleanup(func() {
				Expect(lib.Uninstall(ctx, namespace, revision)).To(Succeed())
				lib.Stop()
				Success("Cleaned up reconcile loop test")
			})

			Expect(lib.Apply(installOpts)).To(Succeed())

			Eventually(func(g Gomega) {
				s := lib.Status()
				g.Expect(s.Error).NotTo(HaveOccurred(), "library install error")
				g.Expect(s.Installed).To(BeTrue(), "library not yet installed")
			}).Should(Succeed())
			Success("Library installed successfully")
		})

		It("does not enter an infinite reconcile loop", func() {
			initialGeneration := lib.Status().Generation
			Expect(initialGeneration).To(Equal(uint64(1)))

			Consistently(func(g Gomega) {
				g.Expect(lib.Apply(installOpts)).To(Succeed())
				g.Expect(lib.Status().Generation).To(Equal(initialGeneration),
					fmt.Sprintf("library generation changed from %d to %d, indicating a reconcile loop", initialGeneration, lib.Status().Generation))
			}, 10*time.Second, 2*time.Second).Should(Succeed())
			Success("Library generation is stable — no reconcile loop detected")
		})
	})

	When("drift detection reconciles owned istiod resources", func() {
		var lib *install.Library

		const (
			namespace = driftTestNamespace
			revision  = "drifttest"
		)

		installOpts := install.Options{
			Values:         install.GatewayAPIDefaults(namespace),
			Namespace:      namespace,
			Version:        istioversion.Default,
			Revision:       revision,
			ManageCRDs:     false,
			IncludeAllCRDs: false,
		}

		BeforeAll(func() {
			common.EnsureNamespaceWithCleanup(k, namespace)

			var err error
			lib, err = install.New(kubeConfig, resources.FS, chart.CRDsFS)
			Expect(err).NotTo(HaveOccurred())

			_, err = lib.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			DeferCleanup(func() {
				if err := lib.Uninstall(ctx, namespace, revision); err != nil {
					GinkgoWriter.Printf("drift test uninstall (best-effort): %v\n", err)
				}
				lib.Stop()
				Success("Cleaned up drift detection test")
			})

			Expect(lib.Apply(installOpts)).To(Succeed())

			Eventually(func(g Gomega) {
				s := lib.Status()
				g.Expect(s.Error).NotTo(HaveOccurred(), "library install error")
				g.Expect(s.Installed).To(BeTrue(), "library not yet installed")
			}).Should(Succeed())
			Success("Library installed for drift detection test")
		})

		It("labels istiod resources with managed-by=sail-library", func(ctx SpecContext) {
			deploy := &appsv1.Deployment{}
			Expect(cl.Get(ctx, types.NamespacedName{
				Name:      "istiod-" + revision,
				Namespace: namespace,
			}, deploy)).To(Succeed())
			Expect(deploy.Labels).To(HaveKeyWithValue(constants.ManagedByLabelKey, "sail-library"))
			Success("Istiod deployment has managed-by=sail-library label")
		})

		It("re-reconciles when a watched resource is modified", func(ctx SpecContext) {
			webhookKey := types.NamespacedName{
				Name: fmt.Sprintf("istio-validator-%s-%s", revision, namespace),
			}

			// Refresh and retry: the background reconciler may update the webhook
			// concurrently (causing a 409 conflict on Update) or transiently clear
			// the Webhooks list, so we assert rather than index directly.
			Eventually(func(g Gomega) {
				webhook := &admissionv1.ValidatingWebhookConfiguration{}
				g.Expect(cl.Get(ctx, webhookKey, webhook)).To(Succeed())
				g.Expect(webhook.Webhooks).NotTo(BeEmpty(), "webhook has no entries")
				webhook.Webhooks[0].Name = "xyz.xyz.xyz"
				webhook.Webhooks[0].FailurePolicy = ptr.Of(admissionv1.Fail)
				g.Expect(cl.Update(ctx, webhook)).To(Succeed())
			}).Should(Succeed())

			Eventually(func(g Gomega) {
				restored := &admissionv1.ValidatingWebhookConfiguration{}
				g.Expect(cl.Get(ctx, webhookKey, restored)).To(Succeed())
				g.Expect(restored.Webhooks[0].Name).NotTo(Equal("xyz.xyz.xyz"))
				g.Expect(restored.Webhooks[0].FailurePolicy).To(HaveValue(Equal(admissionv1.Fail)))
			}).Should(Succeed())
			Success("Library re-reconciled validating webhook after drift")
		})
	})

	When("the library upgrades istiod to a new version", func() {
		var lib *install.Library

		const (
			namespace = upgradeTestNamespace
			revision  = "upgradetest"
		)

		BeforeAll(func() {
			if istioversion.Base == "" {
				Skip("Only one Istio version available, cannot test upgrades")
			}

			common.EnsureNamespaceWithCleanup(k, namespace)

			var err error
			lib, err = install.New(kubeConfig, resources.FS, chart.CRDsFS)
			Expect(err).NotTo(HaveOccurred())

			_, err = lib.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			DeferCleanup(func() {
				if err := lib.Uninstall(ctx, namespace, revision); err != nil {
					GinkgoWriter.Printf("upgrade test uninstall (best-effort): %v\n", err)
				}
				lib.Stop()
				Success("Cleaned up upgrade test")
			})
		})

		It("installs the base version and then upgrades to the new version", func(ctx SpecContext) {
			baseVersion := istioversion.Base
			newVersion := istioversion.New

			GinkgoWriter.Printf("Upgrade test: %s -> %s\n", baseVersion, newVersion)

			By(fmt.Sprintf("installing base version %s", baseVersion))
			Expect(lib.Apply(install.Options{
				Values:    install.GatewayAPIDefaults(namespace),
				Namespace: namespace,
				Version:   baseVersion,
				Revision:  revision,
			})).To(Succeed())

			Eventually(func(g Gomega) {
				s := lib.Status()
				g.Expect(s.Error).NotTo(HaveOccurred(), "library install error")
				g.Expect(s.Installed).To(BeTrue(), "library not yet installed")
				g.Expect(s.Version).To(Equal(baseVersion))
			}).Should(Succeed())
			Success(fmt.Sprintf("Base version %s installed", baseVersion))

			deployKey := types.NamespacedName{
				Name:      "istiod-" + revision,
				Namespace: namespace,
			}

			// Capture the base deployment's image
			var baseImage string
			Eventually(func(g Gomega) {
				deploy := &appsv1.Deployment{}
				g.Expect(cl.Get(ctx, deployKey, deploy)).To(Succeed())
				for _, c := range deploy.Spec.Template.Spec.Containers {
					if c.Name == "discovery" {
						baseImage = c.Image
						break
					}
				}
				g.Expect(baseImage).NotTo(BeEmpty(), "discovery container not found")
			}).Should(Succeed())

			By(fmt.Sprintf("upgrading to new version %s", newVersion))
			Expect(lib.Apply(install.Options{
				Values:    install.GatewayAPIDefaults(namespace),
				Namespace: namespace,
				Version:   newVersion,
				Revision:  revision,
			})).To(Succeed())

			Eventually(func(g Gomega) {
				s := lib.Status()
				g.Expect(s.Error).NotTo(HaveOccurred(), "library upgrade error")
				g.Expect(s.Installed).To(BeTrue(), "library not installed after upgrade")
				g.Expect(s.Version).To(Equal(newVersion))
			}).Should(Succeed())

			// Verify the deployment image changed
			Eventually(func(g Gomega) {
				deploy := &appsv1.Deployment{}
				g.Expect(cl.Get(ctx, deployKey, deploy)).To(Succeed())
				var newImage string
				for _, c := range deploy.Spec.Template.Spec.Containers {
					if c.Name == "discovery" {
						newImage = c.Image
						break
					}
				}
				g.Expect(newImage).NotTo(BeEmpty(), "discovery container not found after upgrade")
				g.Expect(newImage).NotTo(Equal(baseImage),
					fmt.Sprintf("istiod image should change after upgrade from %s to %s", baseVersion, newVersion))
			}).Should(Succeed())

			// Verify the new deployment is available
			Eventually(func(g Gomega) {
				deploy := &appsv1.Deployment{}
				g.Expect(cl.Get(ctx, deployKey, deploy)).To(Succeed())
				g.Expect(deploy.Status.AvailableReplicas).To(BeNumerically(">=", 1),
					"istiod should have at least one available replica after upgrade")
			}).Should(Succeed())
			Success(fmt.Sprintf("Upgraded from %s to %s", baseVersion, newVersion))
		})
	})
})
