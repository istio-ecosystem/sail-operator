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

package analyze

import (
	"context"
	"time"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// MetricDescription is an exported struct that defines the metric description (Name, Help)
// as a new type named MetricDescription.
type MetricDescription struct {
	Name string
	Help string
	Type string
}

// MetricsRecorder manages periodic metrics collection
type MetricsRecorder struct {
	client.Client
	interval time.Duration
	ticker   *time.Ticker
	done     chan struct{}
}

// metricsDescription is a map of string keys (metrics) to MetricDescription values (Name, Help).
var metricDescription = map[string]MetricDescription{
	"IstioVersionTotal": {
		Name: "servicemesh_istiod_total",
		Help: "Total number of Istiod control planes at each Istio version.",
		Type: "GaugeVec",
	},
	"SidecarProxyTotal": {
		Name: "servicemesh_sidecar_proxy_total",
		Help: "Total number of Envoy Sidecar proxies managed by an Istiod control plane.",
		Type: "Gauge",
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
	// SidecarProxyTotal will count how many Envoy sidecar proxies were injected.
	SidecarProxyTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: metricDescription["SidecarProxyTotal"].Name,
			Help: metricDescription["SidecarProxyTotal"].Help,
		},
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
		SidecarProxyTotal,
		SidecarNamespaceTotal,
		ZTunnelVersionTotal,
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

func NewMetricsRecorder(interval time.Duration, client client.Client) *MetricsRecorder {
	return &MetricsRecorder{
		Client:   client,
		interval: interval,
		done:     make(chan struct{}),
	}
}

// Start begins recording metrics every interval until context is canceled
func (m *MetricsRecorder) Start(ctx context.Context) {
	m.ticker = time.NewTicker(m.interval)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-m.done:
				return
			case <-m.ticker.C:
				m.recordMetrics(ctx)
			}
		}
	}()
}

// Stop cleans up the ticker
func (m *MetricsRecorder) Stop() {
	if m.ticker != nil {
		m.ticker.Stop()
	}
	close(m.done)
}

// recordMetrics lists custom resources such as Istio, IstioRevision, ZTunnel and records their counts.
func (m *MetricsRecorder) recordMetrics(ctx context.Context) {
	istiodCounts := m.listIstiod(ctx)
	ztunnelCounts := m.listZTunnel(ctx)
	sidecarProxyCounts := m.listSidecarProxies(ctx)
	sidecarNsCounts := m.listSidecarNamespace(ctx)
	ambientNsCounts := m.listAmbientNamespace(ctx)

	// Update GaugeVec values
	for version, count := range istiodCounts {
		IstioVersionTotal.WithLabelValues(version).Set(count)
	}
	for version, count := range ztunnelCounts {
		ZTunnelVersionTotal.WithLabelValues(version).Set(count)
	}

	SidecarProxyTotal.Set(sidecarProxyCounts)
	SidecarNamespaceTotal.Set(sidecarNsCounts)
	AmbientNamespaceTotal.Set(ambientNsCounts)
}

func (m *MetricsRecorder) listIstiod(ctx context.Context) map[string]float64 {
	log := logf.FromContext(ctx)
	istiodCounts := make(map[string]float64)

	istioList := v1.IstioList{}
	istioRevisionList := v1.IstioRevisionList{}
	if err := m.Client.List(ctx, &istioList); err != nil {
		log.V(4).Error(err, "failed to list Istio")
	}
	if err := m.Client.List(ctx, &istioRevisionList); err != nil {
		log.V(4).Error(err, "failed to list IstioRevision")
	}
	for _, item := range istioList.Items {
		if item.Spec.Version != "" {
			istiodCounts[item.Spec.Version]++
		}
	}
	for _, item := range istioRevisionList.Items {
		if item.Spec.Version != "" {
			istiodCounts[item.Spec.Version]++
		}
	}
	return istiodCounts
}

func (m *MetricsRecorder) listZTunnel(ctx context.Context) map[string]float64 {
	log := logf.FromContext(ctx)
	ztunnelCounts := make(map[string]float64)

	ztunnelList := v1.ZTunnelList{}
	if err := m.Client.List(ctx, &ztunnelList); err != nil {
		log.V(4).Error(err, "failed to list ZTunnel")
	}
	for _, item := range ztunnelList.Items {
		if item.Spec.Version != "" {
			ztunnelCounts[item.Spec.Version]++
		}
	}
	return ztunnelCounts
}

func (m *MetricsRecorder) listSidecarProxies(ctx context.Context) float64 {
	log := logf.FromContext(ctx)
	podList := &corev1.PodList{}
	// filter by security.istio.io/tlsMode=istio label
	if err := m.Client.List(ctx, podList, client.MatchingLabels{"security.istio.io/tlsMode": "istio"}); err != nil {
		log.V(4).Error(err, "failed to list Pod")
	}
	return float64(len(podList.Items))
}

func (m *MetricsRecorder) listSidecarNamespace(ctx context.Context) float64 {
	log := logf.FromContext(ctx)
	nsList := &corev1.NamespaceList{}
	nsListAlt := &corev1.NamespaceList{}
	// filter by istio-injection=enabled or istio.io/rev labels
	if err := m.Client.List(ctx, nsList, client.MatchingLabels{"istio-injection": "enabled"}); err != nil {
		log.V(4).Error(err, "failed to list namespace")
	}
	if err := m.Client.List(ctx, nsListAlt, client.HasLabels{"istio.io/rev"}); err != nil {
		log.V(4).Error(err, "failed to list namespace")
	}
	return float64(len(nsList.Items) + len(nsListAlt.Items))
}

func (m *MetricsRecorder) listAmbientNamespace(ctx context.Context) float64 {
	log := logf.FromContext(ctx)
	ambientNsList := &corev1.NamespaceList{}
	waypointNsList := &corev1.NamespaceList{}
	ingressNsList := &corev1.NamespaceList{}
	// filter by istio.io/dataplane-mode=ambient, istio.io/use-waypoint or istio.io/ingress-use-waypoint labels
	if err := m.Client.List(ctx, ambientNsList, client.MatchingLabels{"istio.io/dataplane-mode": "ambient"}); err != nil {
		log.V(4).Error(err, "failed to list namespace")
	}
	if err := m.Client.List(ctx, waypointNsList, client.HasLabels{"istio.io/use-waypoint"}); err != nil {
		log.V(4).Error(err, "failed to list namespace")
	}
	if err := m.Client.List(ctx, ingressNsList, client.HasLabels{"istio.io/ingress-use-waypoint"}); err != nil {
		log.V(4).Error(err, "failed to list namespace")
	}
	return float64(len(ambientNsList.Items) + len(waypointNsList.Items) + len(ingressNsList.Items))
}
