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

package monitoring

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	"github.com/istio-ecosystem/sail-operator/pkg/env"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	"github.com/istio-ecosystem/sail-operator/pkg/test/project"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/cleaner"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/shell"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	prometheusNamespace           = "monitoring"
	prometheusRelease             = "kube-prometheus-stack"
	managedByValue                = "sail-operator"
	kubePrometheusValue           = "kube-prometheus"
	kubernetesRelabelCount        = 8
	prometheusDiscoverySkipReason = "Prometheus only selects monitors with release: kube-prometheus-stack; " +
		"Sail still uses release: istio from the upstream sample"
)

var monitoringGV = monitoringv1.SchemeGroupVersion

var _ = Describe("Monitoring Controller", Label("smoke", "monitoring"), Ordered, func() {
	SetDefaultEventuallyTimeout(time.Duration(env.GetInt("DEFAULT_TEST_TIMEOUT", 180)) * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)
	debugInfoLogged := false

	version := istioversion.Default
	serviceMonitorName := istioName + "-istiod-metrics"
	podMonitorName := istioName + "-proxies-metrics"

	clr := cleaner.New(cl)

	BeforeAll(func(ctx SpecContext) {
		if !env.GetBool("OCP", false) {
			runKubePrometheusStack("install")
			DeferCleanup(func(_ SpecContext) {
				runKubePrometheusStack("uninstall")
			})
		}

		clr.Record(ctx)
		Expect(k.CreateNamespace(controlPlaneNamespace)).To(Succeed(), "Istio namespace failed to be created")
		Expect(k.CreateNamespace(istioCniNamespace)).To(Succeed(), "IstioCNI namespace failed to be created")
		DeferCleanup(func(ctx SpecContext) {
			clr.Cleanup(ctx)
		})
	})

	When("Istio is installed with monitoring enabled", func() {
		BeforeAll(func(ctx SpecContext) {
			common.CreateIstioCNI(k, version)
			common.CreateIstio(k, version)

			istio := &v1.Istio{}
			Eventually(func(g Gomega) {
				g.Expect(cl.Get(ctx, kube.Key(istioName), istio)).To(Succeed())
				if istio.Annotations == nil {
					istio.Annotations = map[string]string{}
				}
				istio.Annotations[constants.MonitoringAnnotationKey] = constants.MonitoringAnnotationEnabledValue
				g.Expect(cl.Update(ctx, istio)).To(Succeed())
			}).Should(Succeed())
		})

		It("reconciles the Istio control plane", func(ctx SpecContext) {
			common.AwaitCondition(ctx, v1.IstioConditionReconciled, kube.Key(istioName), &v1.Istio{}, k, cl)
			common.AwaitCondition(ctx, v1.IstioConditionReady, kube.Key(istioName), &v1.Istio{}, k, cl)
			common.AwaitDeployment(ctx, "istiod", k, cl)
		})

		It("creates a ServiceMonitor for istiod", func(ctx SpecContext) {
			sm := &monitoringv1.ServiceMonitor{}
			sm.SetGroupVersionKind(monitoringGV.WithKind("ServiceMonitor"))

			Eventually(func(g Gomega) {
				g.Expect(cl.Get(ctx, client.ObjectKey{Name: serviceMonitorName, Namespace: controlPlaneNamespace}, sm)).To(Succeed())
				g.Expect(sm.Labels).To(HaveKeyWithValue("app", "istiod"))
				g.Expect(sm.Labels).To(HaveKeyWithValue("managed-by", managedByValue))
				g.Expect(sm.Labels).To(HaveKeyWithValue("monitored-by", kubePrometheusValue))
				g.Expect(sm.Labels).To(HaveKeyWithValue("release", "istio"))
				g.Expect(sm.Labels).To(HaveKeyWithValue("monitoring", "istio-components"))
				g.Expect(sm.Spec.JobLabel).To(Equal("istio"))
				g.Expect(sm.Spec.Endpoints).To(HaveLen(1))
				g.Expect(sm.Spec.Endpoints[0].Port).To(Equal("http-monitoring"))
				g.Expect(string(sm.Spec.Endpoints[0].Interval)).To(Equal("15s"))
				g.Expect(sm.Spec.Selector.MatchExpressions).To(ContainElement(metav1.LabelSelectorRequirement{
					Key:      "istio",
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{"pilot"},
				}))
				g.Expect(sm.OwnerReferences).NotTo(BeEmpty())
				g.Expect(sm.OwnerReferences[0].Kind).To(Equal(v1.IstioRevisionKind))
				g.Expect(sm.OwnerReferences[0].Name).To(Equal(istioName))
			}).Should(Succeed())
			Success("ServiceMonitor for istiod exists")
		})

		It("discovers the istiod ServiceMonitor in Prometheus", func(ctx SpecContext) {
			Skip(prometheusDiscoverySkipReason)
			targetsPath := fmt.Sprintf(
				"/api/v1/namespaces/%s/services/%s-prometheus:http-web/proxy/api/v1/targets",
				prometheusNamespace,
				prometheusRelease,
			)

			Eventually(func(g Gomega) {
				targets, err := k.GetRaw(targetsPath)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(targets).To(ContainSubstring(serviceMonitorName))
				g.Expect(strings.ToLower(targets)).To(ContainSubstring("istiod"))
			}).Should(Succeed())
			Success("Prometheus discovered istiod ServiceMonitor")
		})

		It("creates a PodMonitor in a namespace labeled istio-injection=enabled", func(ctx SpecContext) {
			Expect(k.CreateNamespace(injectionEnabledNamespace)).To(Succeed())
			Expect(k.Label("namespace", injectionEnabledNamespace, "istio-injection", "enabled")).To(Succeed())

			pm := &monitoringv1.PodMonitor{}
			pm.SetGroupVersionKind(monitoringGV.WithKind("PodMonitor"))

			Eventually(func(g Gomega) {
				g.Expect(cl.Get(ctx, client.ObjectKey{Name: podMonitorName, Namespace: injectionEnabledNamespace}, pm)).To(Succeed())
				assertPodMonitor(g, pm)
			}).Should(Succeed())
			Success("PodMonitor exists in istio-injection namespace")
		})

		It("creates a PodMonitor in a namespace labeled istio.io/rev", func(ctx SpecContext) {
			Expect(k.CreateNamespace(revLabelNamespace)).To(Succeed())
			Expect(k.Label("namespace", revLabelNamespace, "istio.io/rev", istioName)).To(Succeed())

			pm := &monitoringv1.PodMonitor{}
			pm.SetGroupVersionKind(monitoringGV.WithKind("PodMonitor"))

			Eventually(func(g Gomega) {
				g.Expect(cl.Get(ctx, client.ObjectKey{Name: podMonitorName, Namespace: revLabelNamespace}, pm)).To(Succeed())
				assertPodMonitor(g, pm)
			}).Should(Succeed())
			Success("PodMonitor exists in istio.io/rev namespace")
		})

		It("does not create a PodMonitor in the control plane namespace", func(ctx SpecContext) {
			pm := &monitoringv1.PodMonitor{}
			pm.SetGroupVersionKind(monitoringGV.WithKind("PodMonitor"))
			Consistently(cl.Get).WithArguments(ctx, client.ObjectKey{Name: podMonitorName, Namespace: controlPlaneNamespace}, pm).
				ShouldNot(Succeed(), "PodMonitor should not be created in istio-system")
			Success("PodMonitor not created in control plane namespace")
		})

		It("keeps the PodMonitor when the istio.io/rev label is removed", func(ctx SpecContext) {
			assertPodMonitorKept(ctx, revLabelNamespace, "istio.io/rev", podMonitorName)
			Success("PodMonitor kept after istio.io/rev label removal")
		})

		It("discovers the proxy PodMonitor target in Prometheus", func(ctx SpecContext) {
			Skip(prometheusDiscoverySkipReason)
			Expect(k.WithNamespace(injectionEnabledNamespace).ApplyKustomize("sleep")).To(Succeed())
			Eventually(common.CheckPodsReady).WithArguments(ctx, cl, injectionEnabledNamespace).Should(Succeed())

			targetsPath := fmt.Sprintf(
				"/api/v1/namespaces/%s/services/%s-prometheus:http-web/proxy/api/v1/targets",
				prometheusNamespace,
				prometheusRelease,
			)

			Eventually(func(g Gomega) {
				targets, err := k.GetRaw(targetsPath)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(targets).To(ContainSubstring(podMonitorName))
				g.Expect(targets).To(ContainSubstring(injectionEnabledNamespace))
				g.Expect(strings.ToLower(targets)).To(ContainSubstring("istio-proxy"))
			}).Should(Succeed())
			Success("Prometheus discovered proxy PodMonitor target")
		})

		It("keeps the PodMonitor when the istio-injection label is removed", func(ctx SpecContext) {
			assertPodMonitorKept(ctx, injectionEnabledNamespace, "istio-injection", podMonitorName)
			Success("PodMonitor kept after istio-injection label removal")
		})
	})

	AfterEach(func() {
		if CurrentSpecReport().Failed() && !debugInfoLogged {
			common.LogDebugInfo(common.Monitoring, k)
			debugInfoLogged = true
		}
	})
})

func assertPodMonitor(g Gomega, pm *monitoringv1.PodMonitor) {
	g.Expect(pm.Labels).To(HaveKeyWithValue("app", "istio-proxy"))
	g.Expect(pm.Labels).To(HaveKeyWithValue("managed-by", managedByValue))
	g.Expect(pm.Labels).To(HaveKeyWithValue("monitored-by", kubePrometheusValue))
	g.Expect(pm.Labels).To(HaveKeyWithValue("release", "istio"))
	g.Expect(pm.Labels).To(HaveKeyWithValue("monitoring", "istio-proxies"))
	g.Expect(pm.Spec.JobLabel).To(Equal("envoy-stats"))
	g.Expect(pm.Spec.PodMetricsEndpoints).To(HaveLen(1))
	g.Expect(pm.Spec.PodMetricsEndpoints[0].Path).To(Equal("/stats/prometheus"))
	g.Expect(string(pm.Spec.PodMetricsEndpoints[0].Interval)).To(Equal("15s"))
	g.Expect(pm.Spec.PodMetricsEndpoints[0].RelabelConfigs).To(HaveLen(kubernetesRelabelCount))
	g.Expect(pm.Spec.Selector.MatchExpressions).To(ContainElement(metav1.LabelSelectorRequirement{
		Key:      "istio-prometheus-ignore",
		Operator: metav1.LabelSelectorOpDoesNotExist,
	}))
	g.Expect(pm.OwnerReferences).NotTo(BeEmpty())
	g.Expect(pm.OwnerReferences[0].Kind).To(Equal(v1.IstioKind))
	g.Expect(pm.OwnerReferences[0].Name).To(Equal(istioName))
}

func assertPodMonitorKept(ctx SpecContext, namespace, labelKey, name string) {
	Expect(k.RemoveLabel("namespace", namespace, labelKey)).To(Succeed())

	pm := &monitoringv1.PodMonitor{}
	pm.SetGroupVersionKind(monitoringGV.WithKind("PodMonitor"))
	Consistently(cl.Get).WithTimeout(15*time.Second).
		WithArguments(ctx, client.ObjectKey{Name: name, Namespace: namespace}, pm).
		Should(Succeed(), "PodMonitor should be kept while sidecar workloads may still exist")

	pmList := &monitoringv1.PodMonitorList{}
	Expect(cl.List(ctx, pmList, client.InNamespace(namespace))).To(Succeed())
	Expect(pmList.Items).To(HaveLen(1), "namespace label changes should not create a second PodMonitor")
	Expect(pmList.Items[0].Name).To(Equal(name))
}

func kubePrometheusStackScript() string {
	return filepath.Join(project.RootDir, "tests", "e2e", "monitoring", "install-kube-prometheus-stack.sh")
}

func runKubePrometheusStack(action string) {
	out, err := shell.ExecuteCommand(fmt.Sprintf("%q %s", kubePrometheusStackScript(), action))
	Expect(err).NotTo(HaveOccurred(), out)
}
