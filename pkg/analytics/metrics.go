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
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// MetricDescription is an exported struct that defines the metric description (Name, Help)
// as a new type named MetricDescription.
type MetricDescription struct {
	Name string
	Help string
	Type string
}

// metricsDescription is a map of string keys (metrics) to MetricDescription values (Name, Help).
var metricDescription = map[string]MetricDescription{
	"IstioVersionTotal": {
		Name: "servicemesh_istiod_total",
		Help: "Total number of Istiod control planes at each Istio version.",
		Type: "GaugeVec",
	},
	"SidecarProxyVersionTotal": {
		Name: "servicemesh_sidecar_proxy_total",
		Help: "Total number of Envoy Sidecar proxies managed by an Istiod control plane.",
		Type: "GaugeVec",
	},
	"SidecarNamespaceTotal": {
		Name: "servicemesh_sidecar_namespace_total",
		Help: "Total number of namespaces enrolled in Istio sidecar mode.",
		Type: "Gauge",
	},
	"ZTunnelVersionTotal": {
		Name: "servicemesh_ztunnel_total",
		Help: "Total number of ZTunnel proxies managed by an Istiod control plane in Ambient mode.",
		Type: "GaugeVec",
	},
	"WaypointProxyTotal": {
		Name: "servicemesh_waypoint_proxy_total",
		Help: "Total number of Waypoint proxies managed by an Istiod control plane in Ambient mode.",
		Type: "Gauge",
	},
	"AmbientNamespaceTotal": {
		Name: "servicemesh_ambient_namespace_total",
		Help: "Total number of namespaces enrolled in Istio Ambient mode.",
		Type: "Gauge",
	},
}

var (
	// IstioVersionTotal will count how many Istio custom resources were created at each Istio version.
	IstioVersionTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: metricDescription["IstioVersionTotal"].Name,
			Help: metricDescription["IstioVersionTotal"].Help,
		},
		[]string{"app.kubernetes.io/version"},
	)
	// SidecarProxyVersionTotal will count how many Envoy sidecar proxies were injected at each Istio version.
	SidecarProxyVersionTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: metricDescription["SidecarProxyVersionTotal"].Name,
			Help: metricDescription["SidecarProxyVersionTotal"].Help,
		},
		[]string{"app.kubernetes.io/version"},
	)
	// SidecarNamespaceTotal will count how many namespaces were enabled in Istio sidecar mode.
	SidecarNamespaceTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: metricDescription["SidecarNamespaceTotal"].Name,
			Help: metricDescription["SidecarNamespaceTotal"].Help,
		},
	)
	// ZTunnelVersionTotal will count how many ZTunnel custom resources were created at each Istio version.
	ZTunnelVersionTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: metricDescription["ZTunnelVersionTotal"].Name,
			Help: metricDescription["ZTunnelVersionTotal"].Help,
		},
		[]string{"app.kubernetes.io/version"},
	)
	// WaypointProxyTotal will count how many Waypoint proxies were created in Istio Ambient mode.
	WaypointProxyTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: metricDescription["WaypointProxyTotal"].Name,
			Help: metricDescription["WaypointProxyTotal"].Help,
		},
	)
	// AmbientNamespaceTotal will count how many namespaces were enabled in Istio Ambient mode.
	AmbientNamespaceTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: metricDescription["AmbientNamespaceTotal"].Name,
			Help: metricDescription["AmbientNamespaceTotal"].Help,
		},
	)
)

// RegisterMetrics will register metrics with the global prometheus registry
func RegisterMetrics() {
	metrics.Registry.MustRegister(
		IstioVersionTotal,
		SidecarProxyVersionTotal,
		SidecarNamespaceTotal,
		ZTunnelVersionTotal,
		WaypointProxyTotal,
		AmbientNamespaceTotal,
	)
}

// ListMetrics will create a slice with the metrics available in metricDescription
func ListMetrics() []MetricDescription {
	v := make([]MetricDescription, 0, len(metricDescription))
	// Insert value (Name, Help) for each metric
	for _, value := range metricDescription {
		v = append(v, value)
	}

	return v
}
