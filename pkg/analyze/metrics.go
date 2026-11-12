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
	"fmt"
	"time"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
	metricsCfg config.ReconcilerConfig
	interval   time.Duration
}

const (
	metricsMonitorSuffix = "-metrics-monitor"
	metricsServiceSuffix = "-metrics-service"
	serviceCAFile        = "/etc/prometheus/configmaps/serving-certs-ca-bundle/service-ca.crt"
)

// populated at build time via -ldflags
var (
	operatorName   = "sailoperator"  // chart .Values.name
	deploymentName = "sail-operator" // chart .Values.deployment.name
)

// metricsDescription is a map of string keys (metrics) to MetricDescription values (Name, Help).
var metricDescription = map[string]MetricDescription{
	"IstioVersionCurrent": {
		Name: fmt.Sprintf("%s_istiod_current", operatorName),
		Help: "Total number of Istiod control planes at each Istio version.",
		Type: "GaugeVec",
	},
	"SidecarProxyCurrent": {
		Name: fmt.Sprintf("%s_sidecar_proxy_current", operatorName),
		Help: "Total number of Envoy Sidecar proxies managed by an Istiod control plane.",
		Type: "Gauge",
	},
	"SidecarNamespaceCurrent": {
		Name: fmt.Sprintf("%s_sidecar_namespace_current", operatorName),
		Help: "Total number of namespaces enrolled in Istio sidecar mode.",
		Type: "Gauge",
	},
	"ZTunnelVersionCurrent": {
		Name: fmt.Sprintf("%s_ztunnel_current", operatorName),
		Help: "Total number of ZTunnel proxies managed by an Istiod control plane in Ambient mode.",
		Type: "GaugeVec",
	},
	"AmbientNamespaceCurrent": {
		Name: fmt.Sprintf("%s_ambient_namespace_current", operatorName),
		Help: "Total number of namespaces enrolled in Istio Ambient mode.",
		Type: "Gauge",
	},
}

var (
	// IstioVersionCurrent will count how many Istio custom resources were created at each Istio version.
	IstioVersionCurrent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: metricDescription["IstioVersionCurrent"].Name,
			Help: metricDescription["IstioVersionCurrent"].Help,
		},
		[]string{"app.kubernetes.io/version"},
	)
	// SidecarProxyCurrent will count how many Envoy sidecar proxies were injected.
	SidecarProxyCurrent = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: metricDescription["SidecarProxyCurrent"].Name,
			Help: metricDescription["SidecarProxyCurrent"].Help,
		},
	)
	// SidecarNamespaceCurrent will count how many namespaces were enabled in Istio sidecar mode.
	SidecarNamespaceCurrent = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: metricDescription["SidecarNamespaceCurrent"].Name,
			Help: metricDescription["SidecarNamespaceCurrent"].Help,
		},
	)
	// ZTunnelVersionCurrent will count how many ZTunnel custom resources were created at each Istio version.
	ZTunnelVersionCurrent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: metricDescription["ZTunnelVersionCurrent"].Name,
			Help: metricDescription["ZTunnelVersionCurrent"].Help,
		},
		[]string{"app.kubernetes.io/version"},
	)
	// AmbientNamespaceCurrent will count how many namespaces were enabled in Istio Ambient mode.
	AmbientNamespaceCurrent = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: metricDescription["AmbientNamespaceCurrent"].Name,
			Help: metricDescription["AmbientNamespaceCurrent"].Help,
		},
	)
)

// RegisterMetrics will register metrics with the global prometheus registry
func RegisterMetrics() {
	metrics.Registry.MustRegister(
		IstioVersionCurrent,
		SidecarProxyCurrent,
		SidecarNamespaceCurrent,
		ZTunnelVersionCurrent,
		AmbientNamespaceCurrent,
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

func NewMetricsRecorder(interval time.Duration, client client.Client, metricsCfg config.ReconcilerConfig) *MetricsRecorder {
	return &MetricsRecorder{
		Client:     client,
		metricsCfg: metricsCfg,
		interval:   interval,
	}
}

// Start begins recording metrics every interval until context is canceled
func (m *MetricsRecorder) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(m.interval)
		defer ticker.Stop() // Cleans up ticker whenever the goroutine exits
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.recordMetrics(ctx)
			}
		}
	}()
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
		IstioVersionCurrent.WithLabelValues(version).Set(count)
	}
	for version, count := range ztunnelCounts {
		ZTunnelVersionCurrent.WithLabelValues(version).Set(count)
	}

	SidecarProxyCurrent.Set(sidecarProxyCounts)
	SidecarNamespaceCurrent.Set(sidecarNsCounts)
	AmbientNamespaceCurrent.Set(ambientNsCounts)
}

func (m *MetricsRecorder) listIstiod(ctx context.Context) map[string]float64 {
	log := logf.FromContext(ctx)
	istiodCounts := make(map[string]float64)

	istioList := v1.IstioList{}
	if err := m.Client.List(ctx, &istioList); err != nil {
		log.V(4).Error(err, "failed to list Istio")
	}

	for _, item := range istioList.Items {
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
	setNs := make(map[string]corev1.Namespace)

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

	for _, ns := range ambientNsList.Items {
		setNs[ns.Name] = ns
	}
	for _, ns := range waypointNsList.Items {
		setNs[ns.Name] = ns
	}
	for _, ns := range ingressNsList.Items {
		setNs[ns.Name] = ns
	}

	return float64(len(setNs))
}

// EnsureNamespaceLabel ensures label openshift.io/cluster-monitoring=true for the operator namespace
// When the operator is running on OpenShift
func (m *MetricsRecorder) EnsureNamespaceLabel(ctx context.Context) error {
	ns := &corev1.Namespace{}
	if err := m.Client.Get(ctx, types.NamespacedName{Name: m.metricsCfg.OperatorNamespace}, ns); err != nil {
		return fmt.Errorf("failed to get namespace %s: %w", m.metricsCfg.OperatorNamespace, err)
	}

	patchBase := client.MergeFrom(ns.DeepCopy())
	if ns.Labels == nil {
		ns.Labels = make(map[string]string)
	}
	ns.Labels["openshift.io/cluster-monitoring"] = "true"

	if err := m.Client.Patch(ctx, ns, patchBase); err != nil {
		return fmt.Errorf("failed to ensure OpenShift cluster-monitoring label: %w", err)
	}
	return nil
}

// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors,verbs=get;list;create;delete

// EnsureServiceMonitor ensures a ServiceMonitor resource for scraping metrics from the operator metrics-service
func (m *MetricsRecorder) EnsureServiceMonitor(ctx context.Context) error {
	desired := m.buildServiceMonitor()

	existing := &monitoringv1.ServiceMonitor{}
	err := m.Client.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	switch {
	case apierrors.IsNotFound(err):
		if err := m.Client.Create(ctx, desired); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create ServiceMonitor: %w", err)
		}
		return nil
	case meta.IsNoMatchError(err) || runtime.IsNotRegisteredError(err):
		// prometheus-operator CRDs absent — nothing to do
		return nil
	case err != nil:
		return fmt.Errorf("failed to get ServiceMonitor: %w", err)
	}

	// adopt an older monitor that still points at the wrong namespace
	patchBase := client.MergeFrom(existing.DeepCopy())
	existing.Labels = desired.Labels
	existing.Spec = desired.Spec
	return m.Client.Patch(ctx, existing, patchBase)
}

func (m *MetricsRecorder) buildServiceMonitor() *monitoringv1.ServiceMonitor {
	ns := m.metricsCfg.OperatorNamespace
	return &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      operatorName + metricsMonitorSuffix,
			Namespace: ns,
			Labels: map[string]string{
				"app.kubernetes.io/component":  operatorName,
				"app.kubernetes.io/managed-by": constants.ManagedByLabelValue,
				"app.kubernetes.io/name":       operatorName,
				"app.kubernetes.io/part-of":    operatorName,
				"release":                      "prometheus", // Prometheus serviceMonitorSelector
			},
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{"control-plane": deploymentName},
			},
			Endpoints: []monitoringv1.Endpoint{{
				Port:            "https",
				Path:            "/metrics",
				Scheme:          new(monitoringv1.SchemeHTTPS),
				Interval:        monitoringv1.Duration("30s"),
				HonorLabels:     true,
				BearerTokenFile: "/var/run/secrets/kubernetes.io/serviceaccount/token",
				HTTPConfigWithProxyAndTLSFiles: monitoringv1.HTTPConfigWithProxyAndTLSFiles{
					HTTPConfigWithTLSFiles: monitoringv1.HTTPConfigWithTLSFiles{
						TLSConfig: &monitoringv1.TLSConfig{
							TLSFilesConfig: monitoringv1.TLSFilesConfig{CAFile: serviceCAFile},
							SafeTLSConfig: monitoringv1.SafeTLSConfig{
								ServerName: new(fmt.Sprintf("%s%s.%s.svc", deploymentName, metricsServiceSuffix, ns)),
							},
						},
					},
				},
			}},
		},
	}
}
