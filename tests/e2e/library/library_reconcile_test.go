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
	"github.com/istio-ecosystem/sail-operator/pkg/install"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/resources"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"
)

var _ = Describe("Library Reconciliation", Label("reconciliation"), Ordered, func() {
	SetDefaultEventuallyTimeout(3 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	ctx := context.Background()

	BeforeAll(func() {
		common.EnsureNamespaceWithCleanup(k, libraryNamespace)
	})

	When("TLS options are passed via Options", func() {
		var lib *install.Library

		const revision = "tlstest"

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
			var err error
			lib, err = install.New(kubeConfig, resources.FS, chart.CRDsFS)
			Expect(err).NotTo(HaveOccurred())

			_, err = lib.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			DeferCleanup(func() {
				Expect(lib.Uninstall(ctx, libraryNamespace, revision)).To(Succeed())
				lib.Stop()
				Success("Cleaned up TLS test")
			})

			Expect(lib.Apply(install.Options{
				Values:         install.GatewayAPIDefaults(libraryNamespace),
				Namespace:      libraryNamespace,
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
					Namespace: libraryNamespace,
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
					Namespace: libraryNamespace,
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
		installOpts := install.Options{
			Values:         install.GatewayAPIDefaults(libraryNamespace),
			Namespace:      libraryNamespace,
			Version:        istioversion.Default,
			Revision:       "crdtest",
			ManageCRDs:     true,
			IncludeAllCRDs: true,
		}

		BeforeAll(func() {
			var err error
			lib, err = install.New(kubeConfig, resources.FS, chart.CRDsFS)
			Expect(err).NotTo(HaveOccurred())

			_, err = lib.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			DeferCleanup(func() {
				Expect(lib.Uninstall(ctx, libraryNamespace, "crdtest")).To(Succeed())
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
})
