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

package perses

import (
	"context"
	"io/fs"
	"path"

	"github.com/istio-ecosystem/sail-operator/pkg/env"
	"github.com/istio-ecosystem/sail-operator/pkg/perses"
	persesresources "github.com/istio-ecosystem/sail-operator/pkg/perses/resources"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var expectedDashboardNames = []string{
	"istio-control-plane-dashboard",
	"istio-mesh-dashboard",
	"istio-performance-dashboard",
	"istio-service-dashboard",
	"istio-workload-dashboard",
	"istio-wasm-extension-dashboard",
	"istio-ztunnel-dashboard",
}

var _ = Describe("Perses dashboards", Label("perses"), Ordered, Serial, func() {
	var initialPersesDashboardsEnabled bool
	var operatorConfigChanged bool

	BeforeAll(func(ctx SpecContext) {
		Expect(installer.InstallDashboardCRD(ctx)).To(Succeed())
		initialPersesDashboardsEnabled = operatorPersesDashboardsEnabled(ctx)
	})

	AfterAll(func(ctx SpecContext) {
		if operatorConfigChanged {
			Expect(installer.SetOperatorPersesDashboards(initialPersesDashboardsEnabled)).To(Succeed())
			Eventually(deploymentIsRolledOut).WithArguments(ctx, operatorNamespace, operatorDeployment).Should(BeTrue())
		}
		Expect(installer.UninstallDashboardCRD(ctx)).To(Succeed())
	})

	When("enablePersesDashboards is disabled", func() {
		It("does not create PersesDashboard resources in the operator namespace", func(ctx SpecContext) {
			if initialPersesDashboardsEnabled {
				Skip("operator was installed with --enable-perses-dashboards=true")
			}
			if env.GetBool("OLM", false) {
				Skip("OLM manages the operator configuration")
			}
			list, err := dc.Resource(perses.PersesDashboardGVR).Namespace(operatorNamespace).List(ctx, metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(list.Items).To(BeEmpty())
		})
	})

	When("enablePersesDashboards is enabled", func() {
		BeforeAll(func(ctx SpecContext) {
			if env.GetBool("OLM", false) {
				Expect(operatorPersesDashboardsEnabled(ctx)).To(BeTrue(), "the OLM bundle must enable Perses dashboards")
				return
			}
			if initialPersesDashboardsEnabled {
				return
			}
			Expect(installer.SetOperatorPersesDashboards(true)).To(Succeed())
			operatorConfigChanged = true
			Eventually(deploymentIsRolledOut).WithArguments(ctx, operatorNamespace, operatorDeployment).Should(BeTrue())
		})

		It("installs bundled PersesDashboard resources in the operator namespace", func(ctx SpecContext) {
			want := bundledDashboardCount()
			Expect(want).To(Equal(len(expectedDashboardNames)))

			Eventually(func(g Gomega) {
				list, err := dc.Resource(perses.PersesDashboardGVR).Namespace(operatorNamespace).List(ctx, metav1.ListOptions{})
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(list.Items).To(HaveLen(want))

				names := make([]string, 0, len(list.Items))
				for _, item := range list.Items {
					names = append(names, item.GetName())
					spec, found, err := unstructured.NestedMap(item.Object, "spec")
					g.Expect(err).NotTo(HaveOccurred())
					g.Expect(found).To(BeTrue(), "dashboard %s should have spec", item.GetName())
					g.Expect(spec).NotTo(BeEmpty())
				}
				g.Expect(names).To(ConsistOf(expectedDashboardNames))
			}).Should(Succeed())
		})
	})

	When("OpenShift console validation is requested", func() {
		It("validates dashboards in the Perses UI with live metrics", func(ctx SpecContext) {
			if !env.GetBool("OCP", false) || !env.GetBool("PERSES_E2E_CONSOLE", false) {
				Skip("Set OCP=true and PERSES_E2E_CONSOLE=true after COO Perses, PersesDatasource, and mesh metrics are configured; see docs/addons/perses.adoc")
			}
			// The Ginkgo/kubectl e2e framework does not drive the OpenShift console or Perses HTTP API.
			// Manual or downstream automation should verify Observe → Monitoring after the prerequisites in perses.adoc.
			Skip("console and Perses UI checks are out of scope for the default e2e runner")
		})
	})
})

func bundledDashboardCount() int {
	matches, err := fs.Glob(persesresources.ChartFS, path.Join("files", "*.yaml"))
	if err != nil {
		return 0
	}
	return len(matches)
}

func operatorPersesDashboardsEnabled(ctx context.Context) bool {
	deployment := &appsv1.Deployment{}
	if err := cl.Get(ctx, client.ObjectKey{Namespace: operatorNamespace, Name: operatorDeployment}, deployment); err != nil {
		return false
	}
	if len(deployment.Spec.Template.Spec.Containers) == 0 {
		return false
	}
	for _, arg := range deployment.Spec.Template.Spec.Containers[0].Args {
		if arg == "--enable-perses-dashboards=true" {
			return true
		}
	}
	return false
}

func deploymentIsRolledOut(ctx context.Context, namespace, name string) bool {
	deployment := &appsv1.Deployment{}
	if err := cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, deployment); err != nil {
		return false
	}
	desired := int32(1)
	if deployment.Spec.Replicas != nil {
		desired = *deployment.Spec.Replicas
	}
	return deployment.Status.ObservedGeneration == deployment.Generation &&
		deployment.Status.UpdatedReplicas == desired &&
		deployment.Status.AvailableReplicas == desired
}
