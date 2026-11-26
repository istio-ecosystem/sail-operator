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
	"net/http"
	"time"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

const (
	istioRevisionController    = "istiorevision"
	istioRevisionTagController = "istiorevisiontag"
	istioCNIController         = "istiocni"
)

func getReconcileCount(g Gomega, controllerName string) float64 {
	resp, err := http.Get("http://localhost:8080/metrics")
	g.Expect(err).NotTo(HaveOccurred())
	defer resp.Body.Close()

	parser := expfmt.NewTextParser(model.UTF8Validation)
	metricFamilies, err := parser.TextToMetricFamilies(resp.Body)
	g.Expect(err).NotTo(HaveOccurred())

	metricName := "controller_runtime_reconcile_total"
	mf := metricFamilies[metricName]
	sum := float64(0)
	for _, metric := range mf.Metric {
		for _, l := range metric.Label {
			if *l.Name == "controller" && *l.Value == controllerName {
				sum += metric.GetCounter().GetValue()
			}
		}
	}
	return sum
}

func expectNoReconciliation(controller string, action func()) {
	reconcileCount := getReconcileCount(Default, controller)

	action()

	Consistently(func(g Gomega) {
		latestCount := getReconcileCount(g, controller)
		g.Expect(latestCount).To(Equal(reconcileCount))
	}, 5*time.Second).Should(Succeed(), "%s was reconciled when it shouldn't have been", controller)
}

func deleteAll[T client.Object, L client.ObjectList](ctx context.Context, object T, list L) {
	gvk, err := apiutil.GVKForObject(object, k8sClient.Scheme())
	Expect(err).To(Succeed())
	Step("Deleting all " + gvk.Kind + "s")

	Eventually(k8sClient.DeleteAllOf).WithArguments(ctx, object).Should(Succeed())
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.List(ctx, list)).To(Succeed())
		g.Expect(meta.LenList(list)).To(BeZero())
	}).Should(Succeed())
}

func deleteAllIstioRevisions(ctx context.Context) {
	deleteAll(ctx, &v1.IstioRevision{}, &v1.IstioRevisionList{})
}

func deleteAllIstioRevisionTags(ctx context.Context) {
	deleteAll(ctx, &v1.IstioRevisionTag{}, &v1.IstioRevisionTagList{})
}

func deleteAllIstiosAndRevisions(ctx context.Context) {
	deleteAll(ctx, &v1.Istio{}, &v1.IstioList{})
	deleteAllIstioRevisions(ctx)
}

func deleteAllIstioCNIs(ctx context.Context) {
	deleteAll(ctx, &v1.IstioCNI{}, &v1.IstioCNIList{})
}
