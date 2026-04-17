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

package crdownership

import (
	"context"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/istio-ecosystem/sail-operator/chart"
	"github.com/istio-ecosystem/sail-operator/pkg/install"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/resources"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"
)

const (
	testCRDName = "virtualservices.networking.istio.io"
)

var crdGVR = schema.GroupVersionResource{
	Group:    "apiextensions.k8s.io",
	Version:  "v1",
	Resource: "customresourcedefinitions",
}

func loadCRDFromChart(name string) *unstructured.Unstructured {
	entries, err := fs.ReadDir(chart.CRDsFS, ".")
	Expect(err).NotTo(HaveOccurred())

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		data, err := fs.ReadFile(chart.CRDsFS, entry.Name())
		Expect(err).NotTo(HaveOccurred())

		var obj unstructured.Unstructured
		Expect(yaml.Unmarshal(data, &obj.Object)).To(Succeed())
		if obj.GetName() == name {
			return &obj
		}
	}
	Fail(fmt.Sprintf("CRD %s not found in chart", name))
	return nil
}

func findCRDInfo(crds []install.CRDInfo, name string) *install.CRDInfo {
	for i := range crds {
		if crds[i].Name == name {
			return &crds[i]
		}
	}
	return nil
}

func forEachChartCRD(fn func(obj *unstructured.Unstructured)) {
	entries, err := fs.ReadDir(chart.CRDsFS, ".")
	Expect(err).NotTo(HaveOccurred())

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		data, err := fs.ReadFile(chart.CRDsFS, entry.Name())
		Expect(err).NotTo(HaveOccurred())

		var obj unstructured.Unstructured
		if err := yaml.Unmarshal(data, &obj.Object); err != nil {
			continue
		}
		if obj.GetName() == "" {
			continue
		}
		fn(&obj)
	}
}

func deleteAllIstioCRDs(ctx context.Context) {
	forEachChartCRD(func(obj *unstructured.Unstructured) {
		_ = dynamicClient.Resource(crdGVR).Delete(ctx, obj.GetName(), metav1.DeleteOptions{})
	})
}

func createAllIstioCRDs(ctx context.Context) {
	forEachChartCRD(func(obj *unstructured.Unstructured) {
		_, err := dynamicClient.Resource(crdGVR).Create(ctx, obj, metav1.CreateOptions{})
		if err != nil && !apierrors.IsAlreadyExists(err) {
			GinkgoWriter.Printf("Warning: failed to restore CRD %s: %v\n", obj.GetName(), err)
		}
	})
}

var _ = Describe("CRD Ownership", Label("crd-ownership"), Ordered, func() {
	SetDefaultEventuallyTimeout(3 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	ctx := context.Background()

	BeforeAll(func() {
		deleteAllIstioCRDs(ctx)
		Expect(k.CreateNamespace(libraryNamespace)).To(Succeed())
		Success("Created namespace " + libraryNamespace)
	})

	When("OLM-managed CRDs exist before the library starts", func() {
		var lib *install.Library

		BeforeAll(func() {
			crd := loadCRDFromChart(testCRDName)
			labels := crd.GetLabels()
			if labels == nil {
				labels = map[string]string{}
			}
			labels["operators.coreos.com/managed-by"] = "fake-olm"
			crd.SetLabels(labels)

			_, err := dynamicClient.Resource(crdGVR).Create(ctx, crd, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
			Success("Created OLM-labeled CRD: " + testCRDName)

			Eventually(func(g Gomega) {
				got, err := dynamicClient.Resource(crdGVR).Get(ctx, testCRDName, metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				conditions, _, _ := unstructured.NestedSlice(got.Object, "status", "conditions")
				for _, c := range conditions {
					cond, ok := c.(map[string]any)
					if !ok {
						continue
					}
					if cond["type"] == "Established" && cond["status"] == "True" {
						return
					}
				}
				g.Expect(false).To(BeTrue(), "CRD not yet Established")
			}).Should(Succeed())
			Success("CRD is Established")

			lib, err = install.New(kubeConfig, resources.FS, chart.CRDsFS)
			Expect(err).NotTo(HaveOccurred())

			_, err = lib.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			err = lib.Apply(install.Options{
				Namespace:      libraryNamespace,
				Version:        istioversion.Default,
				Revision:       "crdtest",
				ManageCRDs:     true,
				IncludeAllCRDs: true,
			})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() int {
				return len(lib.Status().CRDs)
			}).Should(BeNumerically(">", 0))
			Success("Library reconciled CRDs")
		})

		It("does not overwrite the OLM-managed CRD", func() {
			got, err := dynamicClient.Resource(crdGVR).Get(ctx, testCRDName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())

			labels := got.GetLabels()
			Expect(labels).To(HaveKey("operators.coreos.com/managed-by"))
			Expect(labels).NotTo(HaveKeyWithValue("app.kubernetes.io/managed-by", "sail-library"))
			Success("OLM CRD labels preserved")
		})

		It("reports OLM CRD as not managed by library", func() {
			info := findCRDInfo(lib.Status().CRDs, testCRDName)
			Expect(info).NotTo(BeNil())
			Expect(info.Managed).To(BeFalse())
			Expect(info.Ready).To(BeTrue())
			Success("OLM CRD reported as unmanaged")
		})

		It("manages other CRDs normally", func() {
			var otherName string
			for _, info := range lib.Status().CRDs {
				if info.Name != testCRDName {
					otherName = info.Name
					break
				}
			}
			Expect(otherName).NotTo(BeEmpty(), "expected at least one non-OLM CRD")
			info := findCRDInfo(lib.Status().CRDs, otherName)
			Expect(info).NotTo(BeNil())
			Expect(info.Managed).To(BeTrue(), "CRD %s should be managed", otherName)
			Success("CRD " + otherName + " is managed by library")
		})

		AfterAll(func() {
			lib.Stop()
			deleteAllIstioCRDs(ctx)
			Success("Cleaned up scenario 1")
		})
	})

	When("library creates CRDs first and then OLM adopts one", func() {
		var lib *install.Library

		BeforeAll(func() {
			var err error
			lib, err = install.New(kubeConfig, resources.FS, chart.CRDsFS)
			Expect(err).NotTo(HaveOccurred())

			_, err = lib.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			err = lib.Apply(install.Options{
				Namespace:      libraryNamespace,
				Version:        istioversion.Default,
				Revision:       "crdtest",
				ManageCRDs:     true,
				IncludeAllCRDs: true,
			})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				info := findCRDInfo(lib.Status().CRDs, testCRDName)
				return info != nil && info.Managed
			}).Should(BeTrue())
			Success("Library created and manages " + testCRDName)
		})

		It("initially manages the CRD", func() {
			info := findCRDInfo(lib.Status().CRDs, testCRDName)
			Expect(info).NotTo(BeNil())
			Expect(info.Managed).To(BeTrue())
			Success("CRD is initially managed by library")
		})

		It("stops managing the CRD after OLM adopts it", func() {
			patch := []byte(`{"metadata":{"labels":{"operators.coreos.com/managed-by":"fake-olm"}}}`)
			_, err := dynamicClient.Resource(crdGVR).Patch(ctx, testCRDName, types.MergePatchType, patch, metav1.PatchOptions{})
			Expect(err).NotTo(HaveOccurred())
			Success("Patched CRD with OLM label")

			Eventually(func() bool {
				info := findCRDInfo(lib.Status().CRDs, testCRDName)
				return info != nil && !info.Managed
			}).Should(BeTrue())
			Success("Library stopped managing OLM-adopted CRD")
		})

		AfterAll(func() {
			lib.Stop()
			Success("Cleaned up scenario 2")
		})
	})

	When("OverwriteOLMManagedCRD func allows overwriting", func() {
		var lib *install.Library

		BeforeAll(func() {
			deleteAllIstioCRDs(ctx)

			crd := loadCRDFromChart(testCRDName)
			labels := crd.GetLabels()
			if labels == nil {
				labels = map[string]string{}
			}
			labels["operators.coreos.com/managed-by"] = "fake-olm"
			crd.SetLabels(labels)

			_, err := dynamicClient.Resource(crdGVR).Create(ctx, crd, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
			Success("Created OLM-labeled CRD: " + testCRDName)

			Eventually(func(g Gomega) {
				got, err := dynamicClient.Resource(crdGVR).Get(ctx, testCRDName, metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				conditions, _, _ := unstructured.NestedSlice(got.Object, "status", "conditions")
				for _, c := range conditions {
					cond, ok := c.(map[string]any)
					if !ok {
						continue
					}
					if cond["type"] == "Established" && cond["status"] == "True" {
						return
					}
				}
				g.Expect(false).To(BeTrue(), "CRD not yet Established")
			}).Should(Succeed())
			Success("CRD is Established")

			lib, err = install.New(kubeConfig, resources.FS, chart.CRDsFS)
			Expect(err).NotTo(HaveOccurred())

			_, err = lib.Start(ctx)
			Expect(err).NotTo(HaveOccurred())

			err = lib.Apply(install.Options{
				Namespace:      libraryNamespace,
				Version:        istioversion.Default,
				Revision:       "crdtest",
				ManageCRDs:     true,
				IncludeAllCRDs: true,
				OverwriteOLMManagedCRD: func(_ context.Context, _ *apiextensionsv1.CustomResourceDefinition) bool {
					return true
				},
			})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() int {
				return len(lib.Status().CRDs)
			}).Should(BeNumerically(">", 0))
			Success("Library reconciled CRDs with overwrite func")
		})

		It("overwrites the OLM-managed CRD", func() {
			Eventually(func(g Gomega) {
				got, err := dynamicClient.Resource(crdGVR).Get(ctx, testCRDName, metav1.GetOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				labels := got.GetLabels()
				g.Expect(labels).To(HaveKeyWithValue("app.kubernetes.io/managed-by", "sail-library"))
			}).Should(Succeed())
			Success("Library took ownership of OLM CRD")
		})

		It("reports OLM CRD as managed by library", func() {
			Eventually(func() bool {
				info := findCRDInfo(lib.Status().CRDs, testCRDName)
				return info != nil && info.Managed
			}).Should(BeTrue())
			Success("OLM CRD reported as managed")
		})

		AfterAll(func() {
			lib.Stop()
			deleteAllIstioCRDs(ctx)
			Success("Cleaned up scenario 3")
		})
	})

	AfterAll(func() {
		lib, err := install.New(kubeConfig, resources.FS, chart.CRDsFS)
		if err == nil {
			_ = lib.Uninstall(ctx, libraryNamespace, "crdtest")
		}
		deleteAllIstioCRDs(ctx)
		createAllIstioCRDs(ctx)
		k.Delete("namespace", libraryNamespace)
		Success("Final cleanup complete")
	})
})
