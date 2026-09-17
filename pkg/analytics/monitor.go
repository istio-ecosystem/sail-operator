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
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	OperatorMonitorName = "sail-operator-controller-manager-metrics-monitor"
	IstiodMonitorName   = "istiod-metrics-monitor"
)

// NewOperatorServiceMonitor creates a ServiceMonitor(CR) for scraping metrics from the operator
func NewOperatorServiceMonitor(namespace string) *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{
		TypeMeta: metav1.TypeMeta{
			APIVersion: monitoringv1.SchemeGroupVersion.String(),
			Kind:       "ServiceMonitor",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      OperatorMonitorName,
			Namespace: namespace,
		},
		Spec: *NewOperatorServiceMonitorSpec(),
	}
}

// NewOperatorServiceMonitorSpec creates ServiceMonitorSpec for scraping metrics
func NewOperatorServiceMonitorSpec() *monitoringv1.ServiceMonitorSpec {
	return &monitoringv1.ServiceMonitorSpec{
		Endpoints: []monitoringv1.Endpoint{{
			BearerTokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token",
			Path:            "/metrics",
			Port:            "https",
		}},
		Selector: metav1.LabelSelector{MatchLabels: map[string]string{"control-plane": "sail-operator"}},
	}
}

// NewIstiodServiceMonitor creates a ServiceMonitor(CR) for scraping metrics from Istiod control plane
func NewIstiodServiceMonitor(namespace string) *monitoringv1.ServiceMonitor {
	return &monitoringv1.ServiceMonitor{
		TypeMeta: metav1.TypeMeta{
			APIVersion: monitoringv1.SchemeGroupVersion.String(),
			Kind:       "ServiceMonitor",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      IstiodMonitorName,
			Namespace: namespace,
		},
		Spec: *NewIstiodServiceMonitorSpec(),
	}
}

// NewIstiodServiceMonitorSpec creates ServiceMonitorSpec for scraping Istiod metrics
func NewIstiodServiceMonitorSpec() *monitoringv1.ServiceMonitorSpec {
	return &monitoringv1.ServiceMonitorSpec{
		Endpoints: []monitoringv1.Endpoint{{
			Path: "/metrics",
			Port: "http-monitoring",
		}},
		Selector: metav1.LabelSelector{MatchLabels: map[string]string{"istio": "pilot"}},
	}
}
