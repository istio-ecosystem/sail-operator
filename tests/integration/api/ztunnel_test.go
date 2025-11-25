//go:build integration

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

package integration

import (
	"context"
	"time"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/api/v1alpha1"
	"github.com/istio-ecosystem/sail-operator/pkg/enqueuelogger"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ztunnelName      = "default"
	ztunnelNamespace = "ztunnel-test"
)

var ztunnelKey = client.ObjectKey{Name: ztunnelName}

var _ = Describe("ZTunnel DaemonSet status changes", Label("ztunnel"), Ordered, func() {
	SetDefaultEventuallyPollingInterval(time.Second)
	SetDefaultEventuallyTimeout(30 * time.Second)

	enqueuelogger.LogEnqueueEvents = true

	ctx := context.Background()

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ztunnelNamespace,
		},
	}

	daemonsetKey := client.ObjectKey{Name: "ztunnel", Namespace: ztunnelNamespace}

	BeforeAll(func() {
		Expect(k8sClient.Create(ctx, namespace)).To(Succeed())
	})

	AfterAll(func() {
		Expect(k8sClient.Delete(ctx, namespace)).To(Succeed())
	})

	for _, apiVersion := range []string{"v1", "v1alpha1"} {
		Describe("API version "+apiVersion, func() {
			ds := &appsv1.DaemonSet{}

			BeforeAll(func() {
				if apiVersion == "v1" {
					ztunnel := &v1.ZTunnel{
						ObjectMeta: metav1.ObjectMeta{
							Name: ztunnelName,
						},
						Spec: v1.ZTunnelSpec{
							Version:   istioversion.Default,
							Namespace: ztunnelNamespace,
						},
					}
					Expect(k8sClient.Create(ctx, ztunnel)).To(Succeed())
				} else {
					ztunnel := &v1alpha1.ZTunnel{
						ObjectMeta: metav1.ObjectMeta{
							Name: ztunnelName,
						},
						Spec: v1alpha1.ZTunnelSpec{
							Version:   istioversion.Default,
							Namespace: ztunnelNamespace,
						},
					}
					Expect(k8sClient.Create(ctx, ztunnel)).To(Succeed())
				}
			})

			AfterAll(func() {
				if apiVersion == "v1" {
					ztunnel := &v1.ZTunnel{}
					Expect(k8sClient.Get(ctx, ztunnelKey, ztunnel)).To(Succeed())
					Expect(k8sClient.Delete(ctx, ztunnel)).To(Succeed())
					Eventually(k8sClient.Get).WithArguments(ctx, ztunnelKey, ztunnel).Should(ReturnNotFoundError())
				} else {
					ztunnel := &v1alpha1.ZTunnel{}
					Expect(k8sClient.Get(ctx, ztunnelKey, ztunnel)).To(Succeed())
					Expect(k8sClient.Delete(ctx, ztunnel)).To(Succeed())
					Eventually(k8sClient.Get).WithArguments(ctx, ztunnelKey, ztunnel).Should(ReturnNotFoundError())
				}
			})

			It("creates the ztunnel DaemonSet", func() {
				Eventually(k8sClient.Get).WithArguments(ctx, daemonsetKey, ds).Should(Succeed())
				if apiVersion == "v1" {
					ztunnel := &v1.ZTunnel{}
					Expect(k8sClient.Get(ctx, ztunnelKey, ztunnel)).To(Succeed())
					Expect(ds.ObjectMeta.OwnerReferences).To(ContainElement(NewOwnerReference(ztunnel)))
				} else {
					ztunnel := &v1alpha1.ZTunnel{}
					Expect(k8sClient.Get(ctx, ztunnelKey, ztunnel)).To(Succeed())
					Expect(ds.ObjectMeta.OwnerReferences).To(ContainElement(NewOwnerReference(ztunnel)))
				}
			})

			It("updates the status of the ZTunnel resource", func() {
				if apiVersion == "v1" {
					ztunnel := &v1.ZTunnel{}
					Eventually(func(g Gomega) {
						g.Expect(k8sClient.Get(ctx, ztunnelKey, ztunnel)).To(Succeed())
						g.Expect(ztunnel.Status.ObservedGeneration).To(Equal(ztunnel.ObjectMeta.Generation))
					}).Should(Succeed())
				} else {
					ztunnel := &v1alpha1.ZTunnel{}
					Eventually(func(g Gomega) {
						g.Expect(k8sClient.Get(ctx, ztunnelKey, ztunnel)).To(Succeed())
						g.Expect(ztunnel.Status.ObservedGeneration).To(Equal(ztunnel.ObjectMeta.Generation))
					}).Should(Succeed())
				}
			})

			When("DaemonSet becomes ready", func() {
				BeforeAll(func() {
					Expect(k8sClient.Get(ctx, daemonsetKey, ds)).To(Succeed())
					ds.Status.CurrentNumberScheduled = 3
					ds.Status.NumberReady = 3
					Expect(k8sClient.Status().Update(ctx, ds)).To(Succeed())
				})

				It("marks the ZTunnel resource as ready", func() {
					if apiVersion == "v1" {
						expectZTunnelV1Condition(ctx, v1.ZTunnelConditionReady, metav1.ConditionTrue)
					} else {
						expectZTunnelV1Alpha1Condition(ctx, v1alpha1.ZTunnelConditionReady, metav1.ConditionTrue)
					}
				})
			})

			When("DaemonSet becomes not ready", func() {
				BeforeAll(func() {
					Expect(k8sClient.Get(ctx, daemonsetKey, ds)).To(Succeed())
					ds.Status.CurrentNumberScheduled = 3
					ds.Status.NumberReady = 2
					Expect(k8sClient.Status().Update(ctx, ds)).To(Succeed())
				})

				It("marks the ZTunnel resource as not ready", func() {
					if apiVersion == "v1" {
						expectZTunnelV1Condition(ctx, v1.ZTunnelConditionReady, metav1.ConditionFalse)
					} else {
						expectZTunnelV1Alpha1Condition(ctx, v1alpha1.ZTunnelConditionReady, metav1.ConditionFalse)
					}
				})
			})
		})
	}
})

// expectZTunnelV1Condition on the v1.ZTunnel resource to eventually have a given status.
func expectZTunnelV1Condition(ctx context.Context, condition v1.ZTunnelConditionType, status metav1.ConditionStatus,
	extraChecks ...func(Gomega, *v1.ZTunnelCondition),
) {
	ztunnel := v1.ZTunnel{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, ztunnelKey, &ztunnel)).To(Succeed())
		g.Expect(ztunnel.Status.ObservedGeneration).To(Equal(ztunnel.ObjectMeta.Generation))

		condition := ztunnel.Status.GetCondition(condition)
		g.Expect(condition.Status).To(Equal(status))
		for _, check := range extraChecks {
			check(g, &condition)
		}
	}).Should(Succeed())
}

// expectZTunnelV1Alpha1Condition on the v1alpha1.ZTunnel resource to eventually have a given status.
func expectZTunnelV1Alpha1Condition(ctx context.Context, condition v1alpha1.ZTunnelConditionType, status metav1.ConditionStatus,
	extraChecks ...func(Gomega, *v1alpha1.ZTunnelCondition),
) {
	ztunnel := v1alpha1.ZTunnel{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, ztunnelKey, &ztunnel)).To(Succeed())
		g.Expect(ztunnel.Status.ObservedGeneration).To(Equal(ztunnel.ObjectMeta.Generation))

		condition := ztunnel.Status.GetCondition(condition)
		g.Expect(condition.Status).To(Equal(status))
		for _, check := range extraChecks {
			check(g, &condition)
		}
	}).Should(Succeed())
}
