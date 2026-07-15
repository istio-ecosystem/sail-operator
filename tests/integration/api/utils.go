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
	"net/http"
	"time"

	. "github.com/onsi/gomega"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
)

const (
	istioRevisionController = "istiorevision"
	istioCNIController      = "istiocni"
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
