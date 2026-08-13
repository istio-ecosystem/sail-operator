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
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	"github.com/istio-ecosystem/sail-operator/pkg/install"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/resources"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	downgradeTestNamespace = "library-test-downgrade"
)

var _ = Describe("CRD Downgrade Protection", Label("library", "crd-downgrade"), Ordered, func() {
	SetDefaultEventuallyTimeout(3 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	ctx := context.Background()

	BeforeAll(func() {
		if istioversion.Base == "" {
			Skip("Only one Istio version available, cannot test downgrades")
		}

		deleteAllIstioCRDs(ctx)
		Expect(k.CreateNamespace(downgradeTestNamespace)).To(Succeed())
		Success("Created namespace " + downgradeTestNamespace)
	})

	When("library installs CRDs with a newer version and then attempts a downgrade", func() {
		var lib *install.Library

		newVersion := istioversion.New
		baseVersion := istioversion.Base

		BeforeAll(func() {
			var err error
			lib, err = install.New(kubeConfig, resources.FS, chart.CRDsFS)
			Expect(err).NotTo(HaveOccurred())

			_, err = lib.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			err = lib.Apply(install.Options{
				Namespace:      downgradeTestNamespace,
				Version:        newVersion,
				Revision:       "downgradetest",
				ManageCRDs:     true,
				IncludeAllCRDs: true,
			})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				s := lib.Status()
				g.Expect(s.Error).NotTo(HaveOccurred(), "library install error")
				g.Expect(s.CRDs).NotTo(BeEmpty())
			}).Should(Succeed())
			Success(fmt.Sprintf("Library installed CRDs with version %s", newVersion))
		})

		It("stamps version annotation on CRDs", func() {
			forEachChartCRD(func(obj *unstructured.Unstructured) {
				got, err := dynamicClient.Resource(crdGVR).Get(ctx, obj.GetName(), metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())

				annotations := got.GetAnnotations()
				Expect(annotations).To(
					HaveKeyWithValue(constants.KubernetesAppVersionKey, newVersion),
					fmt.Sprintf("CRD %s should have version annotation %s", obj.GetName(), newVersion),
				)
			})
			Success("All CRDs have version annotation " + newVersion)
		})

		It("stamps managed-by label on CRDs", func() {
			forEachChartCRD(func(obj *unstructured.Unstructured) {
				got, err := dynamicClient.Resource(crdGVR).Get(ctx, obj.GetName(), metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())

				labels := got.GetLabels()
				Expect(labels).To(
					HaveKeyWithValue("app.kubernetes.io/managed-by", "sail-library"),
					fmt.Sprintf("CRD %s should have managed-by label", obj.GetName()),
				)
			})
			Success("All CRDs have managed-by label")
		})

		It("skips CRD update when applying an older version", func() {
			lib.Stop()

			var err error
			lib, err = install.New(kubeConfig, resources.FS, chart.CRDsFS)
			Expect(err).NotTo(HaveOccurred())

			_, err = lib.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			err = lib.Apply(install.Options{
				Namespace:      downgradeTestNamespace,
				Version:        baseVersion,
				Revision:       "downgradetest",
				ManageCRDs:     true,
				IncludeAllCRDs: true,
			})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				s := lib.Status()
				g.Expect(s.Error).NotTo(HaveOccurred(), "library reconcile error")
				g.Expect(s.CRDs).NotTo(BeEmpty())
			}).Should(Succeed())
			Success(fmt.Sprintf("Library reconciled with older version %s", baseVersion))

			By("verifying CRDs still have the newer version annotation")
			forEachChartCRD(func(obj *unstructured.Unstructured) {
				got, err := dynamicClient.Resource(crdGVR).Get(ctx, obj.GetName(), metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())

				annotations := got.GetAnnotations()
				Expect(annotations).To(
					HaveKeyWithValue(constants.KubernetesAppVersionKey, newVersion),
					fmt.Sprintf("CRD %s should still have version %s, not downgraded to %s", obj.GetName(), newVersion, baseVersion),
				)
			})
			Success("CRDs were not downgraded — version annotation preserved")
		})

		It("updates CRDs when applying a newer version", func() {
			lib.Stop()

			var err error
			lib, err = install.New(kubeConfig, resources.FS, chart.CRDsFS)
			Expect(err).NotTo(HaveOccurred())

			_, err = lib.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			err = lib.Apply(install.Options{
				Namespace:      downgradeTestNamespace,
				Version:        newVersion,
				Revision:       "downgradetest",
				ManageCRDs:     true,
				IncludeAllCRDs: true,
			})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func(g Gomega) {
				s := lib.Status()
				g.Expect(s.Error).NotTo(HaveOccurred(), "library reconcile error")
				g.Expect(s.CRDs).NotTo(BeEmpty())
			}).Should(Succeed())

			forEachChartCRD(func(obj *unstructured.Unstructured) {
				got, err := dynamicClient.Resource(crdGVR).Get(ctx, obj.GetName(), metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())

				annotations := got.GetAnnotations()
				Expect(annotations).To(
					HaveKeyWithValue(constants.KubernetesAppVersionKey, newVersion),
					fmt.Sprintf("CRD %s should have version %s after upgrade", obj.GetName(), newVersion),
				)
			})
			Success("CRDs updated with newer version annotation")
		})

		AfterAll(func() {
			lib.Stop()
			deleteAllIstioCRDs(ctx)
			Success("Cleaned up downgrade protection test")
		})
	})

	AfterAll(func() {
		createAllIstioCRDs(ctx)
		k.Delete("namespace", downgradeTestNamespace)
		Success("Final cleanup complete")
	})
})
