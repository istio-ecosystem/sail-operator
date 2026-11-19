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
	"testing"
	"time"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/api/v1alpha1"
	"github.com/istio-ecosystem/sail-operator/pkg/scheme"
	. "github.com/onsi/gomega"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func newMetricsIntegrationFakeClient(objects ...client.Object) client.Client {
	return newFakeClientBuilder().
		WithObjects(objects...).
		WithStatusSubresource(&v1alpha1.MetricsIntegration{}).
		Build()
}

func TestReconcileMetricsIntegration(t *testing.T) {
	cfg := newReconcilerTestConfig()

	ownedRev := &v1.IstioRevision{
		ObjectMeta: revisionMeta,
		Spec: v1.IstioRevisionSpec{
			Version:   "v1.24.0",
			Namespace: istioNamespace,
		},
	}

	t.Run("reconciles UWM and creates monitors for the referenced Istio", func(t *testing.T) {
		g := NewWithT(t)
		mi := newUserWorkloadMetricsIntegration("uwm", istioName)
		istio := testIstioWithoutMonitoring()
		cl := newMetricsIntegrationFakeClient(mi, istio, ownedRev)
		r := NewReconciler(cfg, cl, scheme.Scheme)

		_, err := r.ReconcileMetricsIntegration(ctx, mi)
		g.Expect(err).ToNot(HaveOccurred())

		updated := &v1alpha1.MetricsIntegration{}
		g.Expect(cl.Get(ctx, types.NamespacedName{Name: mi.Name}, updated)).To(Succeed())
		g.Expect(updated.Status.State).To(Equal(v1alpha1.MetricsIntegrationReasonHealthy))
		g.Expect(updated.Status.GetCondition(v1alpha1.MetricsIntegrationConditionReconciled).Status).To(Equal(metav1.ConditionTrue))

		sm := &monitoringv1.ServiceMonitor{}
		g.Expect(cl.Get(ctx, types.NamespacedName{
			Name:      istioName + serviceMonitorNameSuffix,
			Namespace: istioNamespace,
		}, sm)).To(Succeed())
		expectOwnedByMetricsIntegration(g, sm.OwnerReferences, mi.Name)
		g.Expect(sm.OwnerReferences[0].UID).To(Equal(mi.UID))
	})

	t.Run("sets RefNotFound when the referenced Istio does not exist", func(t *testing.T) {
		g := NewWithT(t)
		mi := newUserWorkloadMetricsIntegration("uwm", istioName)
		cl := newMetricsIntegrationFakeClient(mi)
		r := NewReconciler(cfg, cl, scheme.Scheme)

		_, err := r.ReconcileMetricsIntegration(ctx, mi)
		g.Expect(err).To(HaveOccurred())

		updated := &v1alpha1.MetricsIntegration{}
		g.Expect(cl.Get(ctx, types.NamespacedName{Name: mi.Name}, updated)).To(Succeed())
		g.Expect(updated.Status.State).To(Equal(v1alpha1.MetricsIntegrationReasonReferenceNotFound))
		g.Expect(updated.Status.GetCondition(v1alpha1.MetricsIntegrationConditionReconciled).Status).To(Equal(metav1.ConditionFalse))
	})

	t.Run("rejects ClusterObservabilityOperator until it is implemented", func(t *testing.T) {
		g := NewWithT(t)
		mi := &v1alpha1.MetricsIntegration{
			ObjectMeta: metav1.ObjectMeta{Name: "coo"},
			Spec: v1alpha1.MetricsIntegrationSpec{
				TargetRefs: []v1alpha1.TargetReference{
					{Kind: v1.IstioKind, Name: istioName},
				},
				MetricsConfig: v1alpha1.MetricsConfig{
					Type: v1alpha1.MetricsTypeClusterObservabilityOperator,
					ClusterObservabilityOperator: &v1alpha1.ClusterObservabilityOperatorConfig{
						MonitoringStackRef: v1alpha1.NamespacedReference{
							Name:      "stack",
							Namespace: "observability",
						},
					},
				},
			},
		}
		cl := newMetricsIntegrationFakeClient(mi, testIstioWithoutMonitoring())
		r := NewReconciler(cfg, cl, scheme.Scheme)

		_, err := r.ReconcileMetricsIntegration(ctx, mi)
		g.Expect(err).To(HaveOccurred())

		updated := &v1alpha1.MetricsIntegration{}
		g.Expect(cl.Get(ctx, types.NamespacedName{Name: mi.Name}, updated)).To(Succeed())
		g.Expect(updated.Status.State).To(Equal(v1alpha1.MetricsIntegrationReasonNotImplemented))
	})

	t.Run("marks later MetricsIntegration as duplicate for the same target", func(t *testing.T) {
		g := NewWithT(t)
		earlier := newUserWorkloadMetricsIntegration("first", istioName)
		earlier.CreationTimestamp = metav1.NewTime(time.Unix(1, 0))
		later := newUserWorkloadMetricsIntegration("second", istioName)
		later.CreationTimestamp = metav1.NewTime(time.Unix(2, 0))

		cl := newMetricsIntegrationFakeClient(earlier, later, testIstioWithoutMonitoring(), ownedRev)
		r := NewReconciler(cfg, cl, scheme.Scheme)

		_, err := r.ReconcileMetricsIntegration(ctx, later)
		g.Expect(err).To(HaveOccurred())

		updated := &v1alpha1.MetricsIntegration{}
		g.Expect(cl.Get(ctx, types.NamespacedName{Name: later.Name}, updated)).To(Succeed())
		g.Expect(updated.Status.State).To(Equal(v1alpha1.MetricsIntegrationReasonDuplicateTarget))
	})

	t.Run("targetRefs not set", func(t *testing.T) {
		g := NewWithT(t)
		mi := &v1alpha1.MetricsIntegration{
			ObjectMeta: metav1.ObjectMeta{Name: "invalid"},
			Spec: v1alpha1.MetricsIntegrationSpec{
				MetricsConfig: v1alpha1.MetricsConfig{
					Type:                   v1alpha1.MetricsTypeUserWorkloadMonitoring,
					UserWorkloadMonitoring: &v1alpha1.UserWorkloadMonitoringConfig{},
				},
			},
		}
		cl := newMetricsIntegrationFakeClient(mi)
		r := NewReconciler(cfg, cl, scheme.Scheme)

		_, err := r.ReconcileMetricsIntegration(ctx, mi)
		g.Expect(err).To(HaveOccurred())

		updated := &v1alpha1.MetricsIntegration{}
		g.Expect(cl.Get(ctx, types.NamespacedName{Name: mi.Name}, updated)).To(Succeed())
		g.Expect(updated.Status.State).To(Equal(v1alpha1.MetricsIntegrationReasonInvalidSpec))
	})
}

func TestMapMetricsIntegrationToIstio(t *testing.T) {
	g := NewWithT(t)
	r := NewReconciler(newReconcilerTestConfig(), newFakeClientBuilder().Build(), scheme.Scheme)

	mi := &v1alpha1.MetricsIntegration{
		ObjectMeta: metav1.ObjectMeta{Name: "uwm"},
		Spec: v1alpha1.MetricsIntegrationSpec{
			TargetRefs: []v1alpha1.TargetReference{
				{Kind: v1.IstioKind, Name: "default"},
				{Kind: v1.IstioKind, Name: "default"},
				{Kind: targetKindKiali, Name: "kiali", Namespace: "istio-system"},
				{Kind: v1.IstioKind, Name: "other"},
			},
		},
	}

	reqs := r.mapMetricsIntegrationToIstio(ctx, mi)
	g.Expect(reqs).To(HaveLen(2))
	g.Expect(reqs[0].Name).To(Equal("default"))
	g.Expect(reqs[1].Name).To(Equal("other"))
}

func TestMapIstioToMetricsIntegration(t *testing.T) {
	g := NewWithT(t)
	mi := newUserWorkloadMetricsIntegration("uwm", istioName)
	other := newUserWorkloadMetricsIntegration("other", "other-istio")
	cl := newFakeClientBuilder().WithObjects(mi, other).Build()
	r := NewReconciler(newReconcilerTestConfig(), cl, scheme.Scheme)

	reqs := r.mapIstioToMetricsIntegration(ctx, &v1.Istio{ObjectMeta: metav1.ObjectMeta{Name: istioName}})
	g.Expect(reqs).To(HaveLen(1))
	g.Expect(reqs[0].Name).To(Equal("uwm"))
}

func TestMapMonitorToMetricsIntegration(t *testing.T) {
	g := NewWithT(t)
	mi := newUserWorkloadMetricsIntegration("uwm", istioName)
	cl := newFakeClientBuilder().WithObjects(mi).Build()
	r := NewReconciler(newReconcilerTestConfig(), cl, scheme.Scheme)

	t.Run("enqueues owning MetricsIntegration", func(t *testing.T) {
		g := NewWithT(t)
		sm := &monitoringv1.ServiceMonitor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      istioName + serviceMonitorNameSuffix,
				Namespace: istioNamespace,
				OwnerReferences: []metav1.OwnerReference{
					metricsIntegrationOwnerReference(mi),
				},
			},
		}
		reqs := r.mapMonitorToMetricsIntegration(ctx, sm)
		g.Expect(reqs).To(HaveLen(1))
		g.Expect(reqs[0].Name).To(Equal("uwm"))
	})

	t.Run("falls back to MetricsIntegrations targeting Istio owner", func(t *testing.T) {
		g := NewWithT(t)
		sm := &monitoringv1.ServiceMonitor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      istioName + serviceMonitorNameSuffix,
				Namespace: istioNamespace,
				OwnerReferences: []metav1.OwnerReference{
					istioOwnerReference(&v1.Istio{ObjectMeta: metav1.ObjectMeta{Name: istioName, UID: "uid"}}),
				},
			},
		}
		reqs := r.mapMonitorToMetricsIntegration(ctx, sm)
		g.Expect(reqs).To(HaveLen(1))
		g.Expect(reqs[0].Name).To(Equal("uwm"))
	})

	g.Expect(r.mapMonitorToMetricsIntegration(ctx, &monitoringv1.PodMonitor{
		ObjectMeta: metav1.ObjectMeta{Name: "unowned"},
	})).To(BeEmpty())
}

func TestDuplicateTargetOf(t *testing.T) {
	g := NewWithT(t)

	first := newUserWorkloadMetricsIntegration("first", "default")
	first.CreationTimestamp = metav1.NewTime(time.Unix(1, 0))
	second := newUserWorkloadMetricsIntegration("second", "default")
	second.CreationTimestamp = metav1.NewTime(time.Unix(2, 0))

	g.Expect(duplicateTargetOf(first, []v1alpha1.MetricsIntegration{*first, *second})).To(BeEmpty())
	g.Expect(duplicateTargetOf(second, []v1alpha1.MetricsIntegration{*first, *second})).To(ContainSubstring("referenced by first"))
}
