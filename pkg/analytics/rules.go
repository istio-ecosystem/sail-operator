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

package analytics

import (
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	RuleName         = "service-mesh-usage-rules"
	istiodRuleGroup  = "service-mesh.istiod.rules"
	sidecarRuleGroup = "service-mesh.sidecar.rules"
	ztunnelRuleGroup = "service-mesh.ztunnel.rules"
)

// NewPrometheusRule creates a PrometheusRule(CR) for the operator to have recording rules
func NewPrometheusRule(namespace string) *monitoringv1.PrometheusRule {
	return &monitoringv1.PrometheusRule{
		TypeMeta: metav1.TypeMeta{
			APIVersion: monitoringv1.SchemeGroupVersion.String(),
			Kind:       "PrometheusRule",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      RuleName,
			Namespace: namespace,
		},
		Spec: *NewPrometheusRuleSpec(),
	}
}

// NewPrometheusRuleSpec creates PrometheusRuleSpec for recording rules
func NewPrometheusRuleSpec() *monitoringv1.PrometheusRuleSpec {
	return &monitoringv1.PrometheusRuleSpec{
		Groups: []monitoringv1.RuleGroup{
			{Name: istiodRuleGroup, Rules: []monitoringv1.Rule{createOperatorTotalRecordingRule("servicemesh_istiod_total")}},
			{Name: sidecarRuleGroup, Rules: []monitoringv1.Rule{createOperatorTotalRecordingRule("servicemesh_sidecar_proxy_total")}},
			{Name: ztunnelRuleGroup, Rules: []monitoringv1.Rule{createOperatorTotalRecordingRule("servicemesh_ztunnel_total")}},
		},
	}
}

// createOperatorTotalRecordingRule create a recording rule for all Istio versions
func createOperatorTotalRecordingRule(metricName string) monitoringv1.Rule {
	return monitoringv1.Rule{
		Record: metricName,
		Expr:   intstr.FromString(fmt.Sprintf("sum(%s) by (app.kubernetes.io/version)", metricName)),
	}
}
