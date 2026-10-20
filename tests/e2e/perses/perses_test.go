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
	BeforeAll(func(ctx SpecContext) {
		Expect(installer.InstallDashboardCRD(ctx)).To(Succeed())
		DeferCleanup(func(ctx SpecContext) {
			Expect(installer.UninstallDashboardCRD(ctx)).To(Succeed())
		})
	})

	It("does not create PersesDashboard resources when enablePersesDashboards is disabled", func(ctx SpecContext) {
		if env.GetBool("OLM", false) {
			Skip("OLM bundle always enables Perses dashboards and cannot be turned off via Helm")
		}

		ensureOperatorPersesDashboards(ctx, false)
		Expect(deleteAllPersesDashboards(ctx)).To(Succeed())
		Expect(installer.RolloutRestartOperator(operatorDeployment)).To(Succeed())
		Eventually(deploymentIsRolledOut).WithArguments(ctx, operatorNamespace, operatorDeployment).Should(BeTrue())

		Eventually(func(g Gomega) {
			items, err := listPersesDashboards(ctx)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(items).To(BeEmpty())
		}).Should(Succeed())
	})

	It("installs bundled PersesDashboard resources when enablePersesDashboards is enabled", func(ctx SpecContext) {
		if !env.GetBool("OLM", false) {
			DeferCleanup(func(ctx SpecContext) {
				ensureOperatorPersesDashboards(ctx, false)
			})
		}
		ensureOperatorPersesDashboards(ctx, true)

		want := bundledDashboardCount()
		Expect(want).To(Equal(len(expectedDashboardNames)))

		Eventually(func(g Gomega) {
			items, err := listPersesDashboards(ctx)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(items).To(HaveLen(want))

			names := make([]string, 0, len(items))
			for _, item := range items {
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

// ensureOperatorPersesDashboards sets enablePersesDashboards and waits for a rolled-out operator.
// On OLM the flag is fixed true in the bundle; only a rollout restart is applied.
func ensureOperatorPersesDashboards(ctx context.Context, wantEnabled bool) {
	if env.GetBool("OLM", false) {
		Expect(wantEnabled).To(BeTrue(), "the OLM bundle must enable Perses dashboards")
		Expect(installer.RolloutRestartOperator(operatorDeployment)).To(Succeed())
	} else {
		Expect(installer.SetOperatorPersesDashboards(wantEnabled)).To(Succeed())
	}
	Eventually(deploymentIsRolledOut).WithArguments(ctx, operatorNamespace, operatorDeployment).Should(BeTrue())
	Eventually(func(g Gomega) {
		enabled, err := operatorPersesDashboardsEnabled(ctx)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(enabled).To(Equal(wantEnabled))
	}).Should(Succeed())
}

func deleteAllPersesDashboards(ctx context.Context) error {
	items, err := listPersesDashboards(ctx)
	if err != nil {
		return err
	}
	for i := range items {
		if err := cl.Delete(ctx, &items[i]); err != nil {
			return err
		}
	}
	return nil
}

func listPersesDashboards(ctx context.Context) ([]unstructured.Unstructured, error) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(perses.DashboardGVK)
	if err := cl.List(ctx, list, client.InNamespace(operatorNamespace)); err != nil {
		return nil, err
	}
	return list.Items, nil
}

func bundledDashboardCount() int {
	matches, err := fs.Glob(persesresources.ChartFS, path.Join("files", "*.yaml"))
	if err != nil {
		return 0
	}
	return len(matches)
}

func operatorPersesDashboardsEnabled(ctx context.Context) (bool, error) {
	deployment := &appsv1.Deployment{}
	if err := cl.Get(ctx, client.ObjectKey{Namespace: operatorNamespace, Name: operatorDeployment}, deployment); err != nil {
		return false, err
	}
	if len(deployment.Spec.Template.Spec.Containers) == 0 {
		return false, nil
	}
	for _, arg := range deployment.Spec.Template.Spec.Containers[0].Args {
		if arg == "--enable-perses-dashboards=true" {
			return true, nil
		}
	}
	return false, nil
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
