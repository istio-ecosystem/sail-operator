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

package reconcile

import (
	"context"
	"fmt"

	"istio.io/istio/pkg/ptr"
	"k8s.io/client-go/discovery"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	v1client "github.com/prometheus-operator/prometheus-operator/pkg/client/versioned/typed/monitoring/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	coreosGroupVersion = "monitoring.coreos.com/v1" // default API group used by Prometheus Operator
	rhobsGroupVersion  = "monitoring.rhobs/v1"      // latest API group used by OpenShift Cluster Observability Operator
	serviceMonitorKind = "ServiceMonitor"
	podMonitorKind     = "PodMonitor"
)

var (
	serviceMonitorExists = false
	podMonitorExists     = false
)

// MonitorReconciler handles reconciliation of the ServiceMonitor and PodMonitor resources.
type MonitorReconciler struct {
	discoveryClient  *discovery.DiscoveryClient
	monitoringClient *v1client.MonitoringV1Client
	endpointConfigs  []monitoringv1.PodMetricsEndpoint
}

// defaultPodMetricRelabelConfig defines the default configuration of Istio proxies metrics path and relabelings.
func defaultPodMetricRelabelConfig() []monitoringv1.PodMetricsEndpoint {
	return []monitoringv1.PodMetricsEndpoint{
		{
			Path:     "/stats/prometheus",
			Interval: "30s",
			RelabelConfigs: []monitoringv1.RelabelConfig{
				{
					Action:       "keep",
					SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_container_name"},
					Regex:        "istio-proxy",
				},
				{
					Action:       "keep",
					SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_annotationpresent_prometheus_io_scrape"},
				},
				{
					Action:       "replace",
					Regex:        `(\d+);(([A-Fa-f0-9]{1,4}::?){1,7}[A-Fa-f0-9]{1,4})`,
					Replacement:  ptr.Of("[$2]:$1"),
					SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_annotation_prometheus_io_port", "__meta_kubernetes_pod_ip"},
					TargetLabel:  "__address__",
				},
				{
					Action:       "replace",
					Regex:        `(\d+);((([0-9]+?)(\.|$)){4})`,
					Replacement:  ptr.Of("$2:$1"),
					SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_annotation_prometheus_io_port", "__meta_kubernetes_pod_ip"},
					TargetLabel:  "__address__",
				},
				{
					SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_label_app_kubernetes_io_name", "__meta_kubernetes_pod_label_app"},
					Separator:    ptr.Of(";"),
					TargetLabel:  "app",
					Action:       "replace",
					Regex:        `(.+);.*|.*;(.+)`,
					Replacement:  ptr.Of("${1}${2}"),
				},
				{
					SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_label_app_kubernetes_io_version", "__meta_kubernetes_pod_label_version"},
					Separator:    ptr.Of(";"),
					TargetLabel:  "version",
					Action:       "replace",
					Regex:        `(.+);.*|.*;(.+)`,
					Replacement:  ptr.Of("${1}${2}"),
				},
				{
					SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_namespace"},
					Action:       "replace",
					TargetLabel:  "namespace",
				},
				{
					Action:      "replace",
					Replacement: ptr.Of("mesh1"),
					TargetLabel: "mesh_id",
				},
			},
		},
	}
}

// NewMonitorReconciler creates a new MonitorReconciler.
func NewMonitorReconciler(configs []monitoringv1.PodMetricsEndpoint) (*MonitorReconciler, error) {
	// Running in cluster and use the cluster provided kubeconfig.
	cfg := ctrl.GetConfigOrDie()
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("Error creating discovery client: %w", err)
	}

	if configs == nil {
		configs = defaultPodMetricRelabelConfig()
	}

	return &MonitorReconciler{
		discoveryClient:  discoveryClient,
		monitoringClient: v1client.New(discoveryClient.RESTClient()),
		endpointConfigs:  configs,
	}, nil
}

// Validate checks custom resource definitions ServiceMonitor and PodMonitor exist in the cluster.
func (r *MonitorReconciler) Validate(ctx context.Context) error {
	// Get all resources from API server.
	resources, err := r.discoveryClient.ServerPreferredResources()
	if err != nil {
		return fmt.Errorf("Error getting all resources: %w", err)
	}

	for _, list := range resources {
		if list.GroupVersion == coreosGroupVersion || list.GroupVersion == rhobsGroupVersion {
			for _, resource := range list.APIResources {
				if resource.Kind == serviceMonitorKind {
					serviceMonitorExists = true
					break
				}
			}
		}
	}

	for _, list := range resources {
		if list.GroupVersion == coreosGroupVersion || list.GroupVersion == rhobsGroupVersion {
			for _, resource := range list.APIResources {
				if resource.Kind == podMonitorKind {
					podMonitorExists = true
					break
				}
			}
		}
	}

	if serviceMonitorExists && podMonitorExists {
		fmt.Printf("Monitoring CRD ServiceMonitor and PodMonitor exist")
	} else {
		if !serviceMonitorExists {
			return fmt.Errorf("Monitoring CRD ServiceMonitor does not exist.")
		}
		if !podMonitorExists {
			return fmt.Errorf("Monitoring CRD PodMonitor does not exist.")
		}
	}
	return nil
}

// InstallServiceMonitor installs or upgrades the ServiceMonitor object.
func (r *MonitorReconciler) InstallServiceMonitor(ctx context.Context, namespace string) error {
	serviceMonitor := &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "istiod-monitor",
			Namespace: namespace,
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			TargetLabels: []string{"app"},
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{"istio": "pilot"},
			},
			Endpoints: []monitoringv1.Endpoint{
				{
					Port:     "http-monitoring",
					Path:     "/metrics",
					Interval: "30s",
				},
			},
		},
	}

	_, err := r.monitoringClient.ServiceMonitors("istio-system").Create(context.TODO(), serviceMonitor, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("Failed to create istiod-monitor: %w", err)
	}
	return nil
}

// InstallPodMonitor installs or upgrades the PodMonitor objects.
func (r *MonitorReconciler) InstallPodMonitor(ctx context.Context, namespaces []string) error {
	for _, ns := range namespaces {
		podMonitor := &monitoringv1.PodMonitor{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "istio-proxies-monitor",
				Namespace: ns,
			},
			Spec: monitoringv1.PodMonitorSpec{
				Selector: metav1.LabelSelector{
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{Key: "istio-prometheus-ignore", Operator: metav1.LabelSelectorOpDoesNotExist},
					},
				},
				PodMetricsEndpoints: r.endpointConfigs,
			},
		}

		_, err := r.monitoringClient.PodMonitors(ns).Create(context.TODO(), podMonitor, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("Failed to create istio-proxies-monitor in namespace %s: %w", ns, err)
		}
	}
	return nil
}

// UninstallServiceMonitor removes the ServiceMonitor object.
func (r *MonitorReconciler) UninstallServiceMonitor(ctx context.Context) error {
	err := r.monitoringClient.ServiceMonitors("istio-system").Delete(context.TODO(), "istiod-monitor", metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("Failed to delete istiod-monitor: %w", err)
	}
	return nil
}

// UninstallPodMonitor removes the PodMonitor objects.
func (r *MonitorReconciler) UninstallPodMonitor(ctx context.Context, namespaces []string) error {
	for _, ns := range namespaces {
		err := r.monitoringClient.PodMonitors(ns).Delete(context.TODO(), "istio-proxies-monitor", metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("Failed to delete istio-proxies-monitor in namespace %s: %w", ns, err)
		}
	}
	return nil
}
