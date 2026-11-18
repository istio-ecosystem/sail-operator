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
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/client_golang/prometheus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
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
	labels     map[string]string
}

const (
	metricsMonitorSuffix  = "-metrics-monitor"
	metricsServiceSuffix  = "-metrics-service"
	serviceCAFile         = "/etc/prometheus/configmaps/serving-certs-ca-bundle/service-ca.crt"
	operatorNameLabel     = "app.kubernetes.io/part-of"
	deploymentNameLabel   = "control-plane"
	defaultOperatorName   = "sailoperator"
	defaultDeploymentName = "sail-operator"
)

// metricPrefix will be populated at build time via -ldflags
var metricPrefix = "sail_operator"

// metricsDescription is a map of string keys (metrics) to MetricDescription values (Name, Help).
var metricDescription = map[string]MetricDescription{
	"IstioVersionCurrent": {
		Name: fmt.Sprintf("%s_istiod_current", metricPrefix),
		Help: "Total number of Istiod control planes at each Istio version.",
		Type: "GaugeVec",
	},
	"SidecarProxyCurrent": {
		Name: fmt.Sprintf("%s_sidecar_proxy_current", metricPrefix),
		Help: "Total number of Envoy Sidecar proxies managed by an Istiod control plane.",
		Type: "Gauge",
	},
	"ZTunnelVersionCurrent": {
		Name: fmt.Sprintf("%s_ztunnel_current", metricPrefix),
		Help: "Total number of ZTunnel proxies managed by an Istiod control plane in Ambient mode.",
		Type: "GaugeVec",
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
	// ZTunnelVersionCurrent will count how many ZTunnel custom resources were created at each Istio version.
	ZTunnelVersionCurrent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: metricDescription["ZTunnelVersionCurrent"].Name,
			Help: metricDescription["ZTunnelVersionCurrent"].Help,
		},
		[]string{"app.kubernetes.io/version"},
	)
)

// RegisterMetrics will register metrics with the global prometheus registry
func RegisterMetrics() {
	metrics.Registry.MustRegister(
		IstioVersionCurrent,
		SidecarProxyCurrent,
		ZTunnelVersionCurrent,
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

func NewMetricsRecorder(interval time.Duration, client client.Client, metricsCfg config.ReconcilerConfig, labels map[string]string) *MetricsRecorder {
	if labels == nil {
		labels = make(map[string]string)
	}
	return &MetricsRecorder{
		Client:     client,
		metricsCfg: metricsCfg,
		interval:   interval,
		labels:     labels,
	}
}

// getOperatorName returns the operator name taken from the downward API labels.
func (m *MetricsRecorder) getOperatorName() string {
	if name := m.labels[operatorNameLabel]; name != "" {
		return name
	}
	return defaultOperatorName
}

// getDeploymentName returns the name of the Deployment running the operator, taken from the downward API labels.
func (m *MetricsRecorder) getDeploymentName() string {
	if name := m.labels[deploymentNameLabel]; name != "" {
		return name
	}
	return defaultDeploymentName
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

	// Update GaugeVec values
	for version, count := range istiodCounts {
		IstioVersionCurrent.WithLabelValues(version).Set(count)
	}
	for version, count := range ztunnelCounts {
		ZTunnelVersionCurrent.WithLabelValues(version).Set(count)
	}

	SidecarProxyCurrent.Set(sidecarProxyCounts)
}

func (m *MetricsRecorder) listSidecarProxies(ctx context.Context) float64 {
	log := logf.FromContext(ctx)
	podList := &metav1.PartialObjectMetadataList{}
	podList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("PodList"))
	if err := m.Client.List(ctx, podList, client.MatchingLabels{"security.istio.io/tlsMode": "istio"}); err != nil {
		log.V(4).Error(err, "failed to list Pod")
	}
	return float64(len(podList.Items))
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
	case err != nil:
		return fmt.Errorf("failed to get ServiceMonitor: %w", err)
	}

	// adopt an older monitor that still configures wrong tls serverName
	patchBase := client.MergeFrom(existing.DeepCopy())
	existing.Labels = desired.Labels
	existing.Spec = desired.Spec
	return m.Client.Patch(ctx, existing, patchBase)
}

func (m *MetricsRecorder) buildServiceMonitor() *monitoringv1.ServiceMonitor {
	operatorName := m.getOperatorName()
	deploymentName := m.getDeploymentName()
	smLabels := m.labels
	smLabels["release"] = "prometheus" // Prometheus serviceMonitorSelector

	ns := m.metricsCfg.OperatorNamespace
	return &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      operatorName + metricsMonitorSuffix,
			Namespace: ns,
			Labels:    smLabels,
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{deploymentNameLabel: deploymentName},
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

// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=prometheusrules,verbs=get;list;create;delete

// EnsureRecordingRules ensures the recording rules that derive namespace enrolment counts from kube-state-metrics.
// These are computed in Prometheus rather than by the operator, so the operator does not have to list every namespace on every collection interval.
func (m *MetricsRecorder) EnsureRecordingRules(ctx context.Context) error {
	desired := m.buildPrometheusRule()

	existing := &monitoringv1.PrometheusRule{}
	err := m.Client.Get(ctx, client.ObjectKeyFromObject(desired), existing)
	switch {
	case apierrors.IsNotFound(err):
		if err := m.Client.Create(ctx, desired); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create PrometheusRule: %w", err)
		}
		return nil
	case err != nil:
		return fmt.Errorf("failed to get PrometheusRule: %w", err)
	}

	patchBase := client.MergeFrom(existing.DeepCopy())
	existing.Spec = desired.Spec
	return m.Client.Patch(ctx, existing, patchBase)
}

func (m *MetricsRecorder) buildPrometheusRule() *monitoringv1.PrometheusRule {
	operatorName := m.getOperatorName()

	return &monitoringv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      operatorName + "-metrics-rules",
			Namespace: m.metricsCfg.OperatorNamespace,
			Labels:    m.labels,
		},
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{{
				Name:     operatorName + ".namespaces",
				Interval: new(monitoringv1.Duration("60s")),
				Rules: []monitoringv1.Rule{
					{
						Record: fmt.Sprintf("%s_sidecar_namespace_current", operatorName),
						Expr: intstr.FromString(`count(group by(namespace) (kube_namespace_labels{label_istio_injection="enabled"})` +
							` or group by(namespace) (kube_namespace_labels{label_istio_io_rev!=""}))` +
							` or vector(0)`),
					},
					{
						Record: fmt.Sprintf("%s_ambient_namespace_current", operatorName),
						Expr: intstr.FromString(`count(group by (namespace) (kube_namespace_labels{label_istio_io_dataplane_mode="ambient"})` +
							` or group by (namespace) (kube_namespace_labels{label_istio_io_use_waypoint!=""})` +
							` or group by (namespace) (kube_namespace_labels{label_istio_io_ingress_use_waypoint!=""})) or vector(0)`),
					},
				},
			}},
		},
	}
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
			istiodCounts[mapVersions(item.Spec.Version)]++
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
			ztunnelCounts[mapVersions(item.Spec.Version)]++
		}
	}
	return ztunnelCounts
}

// mapVersions reduces an Istio or ZTunnel version name ("v1.30.3", "v1.31-latest", "latest") to its "vX.Y" form
func mapVersions(version string) string {
	if info, ok := istioversion.Map[version]; ok && info.Version != nil {
		return fmt.Sprintf("v%d.%d", info.Version.Major(), info.Version.Minor())
	}
	return version
}
