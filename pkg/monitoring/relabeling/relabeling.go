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

package relabeling

import (
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
)

// Config holds platform-specific relabeling defaults for Istio metrics integration.
type Config struct {
	// PodMonitorRelabelings are applied to podMetricsEndpoints.relabelings on PodMonitor resources.
	PodMonitorRelabelings []monitoringv1.RelabelConfig
	// ServiceMonitorRelabelings are applied to endpoints.relabelings on ServiceMonitor resources.
	ServiceMonitorRelabelings []monitoringv1.RelabelConfig
}

// ForPlatform returns default relabeling configuration for the given platform.
// meshID is the Istio CR name used as the mesh_id label on OpenShift; it is ignored on Kubernetes.
// tuningEnabled is an option for metric thinning usages.
func ForPlatform(platform config.Platform, meshID string, tuningEnabled bool) Config {
	switch platform {
	case config.PlatformOpenShift:
		if tuningEnabled {
			return Config{
				PodMonitorRelabelings:     openShiftMetricTuningPodMonitorRelabelings(meshID),
				ServiceMonitorRelabelings: nil,
			}
		}

		return Config{
			PodMonitorRelabelings:     openshiftPodMonitorRelabelings(meshID),
			ServiceMonitorRelabelings: nil,
		}
	default:
		return Config{
			PodMonitorRelabelings:     kubernetesPodMonitorRelabelings(),
			ServiceMonitorRelabelings: nil,
		}
	}
}

// kubernetesPodMonitorRelabelings returns relabeling rules from upstream Istio:
// samples/addons/extras/prometheus-operator.yaml
func kubernetesPodMonitorRelabelings() []monitoringv1.RelabelConfig {
	return cloneRelabelConfigs([]monitoringv1.RelabelConfig{
		keepContainer("istio-proxy"),
		keepAnnotationPresent(),
		addressReplaceIPv6(),
		addressReplaceIPv4(),
		{
			Action:      "labeldrop",
			Regex:       "__meta_kubernetes_pod_label_(.+)",
			TargetLabel: "",
		},
		{
			Action:       "replace",
			SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_namespace"},
			TargetLabel:  "namespace",
		},
		{
			Action:       "replace",
			SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_name"},
			TargetLabel:  "pod",
		},
	})
}

// openshiftPodMonitorRelabelings returns relabeling rules from Red Hat OpenShift Service Mesh 3.0:
// Observability > Metrics and Service Mesh
func openshiftPodMonitorRelabelings(meshID string) []monitoringv1.RelabelConfig {
	return cloneRelabelConfigs([]monitoringv1.RelabelConfig{
		keepContainer("istio-proxy"),
		keepAnnotationPresent(),
		addressReplaceIPv6(),
		addressReplaceIPv4(),
		{
			Action:       "replace",
			SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_label_app_kubernetes_io_name", "__meta_kubernetes_pod_label_app"},
			Separator:    strPtr(";"),
			TargetLabel:  "app",
			Regex:        "(.+);.*|.*;(.+)",
			Replacement:  strPtr("${1}${2}"),
		},
		{
			Action:       "replace",
			SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_label_app_kubernetes_io_version", "__meta_kubernetes_pod_label_version"},
			Separator:    strPtr(";"),
			TargetLabel:  "version",
			Regex:        "(.+);.*|.*;(.+)",
			Replacement:  strPtr("${1}${2}"),
		},
		{
			Action:       "replace",
			SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_namespace"},
			TargetLabel:  "namespace",
		},
		{
			Action:      "replace",
			TargetLabel: "mesh_id",
			Replacement: strPtr(meshID),
		},
	})
}

// openShiftMetricTuningPodMonitorRelabelings returns relabeling rules
// by dropping unnecessary metrics and labels from Service Mesh Istio default scraping jobs: 
func openShiftMetricTuningPodMonitorRelabelings(meshID string) []monitoringv1.RelabelConfig {
	return cloneRelabelConfigs(append(openshiftPodMonitorRelabelings(meshID),
		monitoringv1.RelabelConfig{
			Action:       "drop",
			SourceLabels: []monitoringv1.LabelName{"__name__"},
			Regex:        "istio_agent_.*|istiod_.*|istio_build|citadel_.*|galley_.*|pilot_[^psx].*|envoy_cluster_[^u].*|envoy_cluster_update.*|envoy_listener_[^dh].*|envoy_server_[^mu].*|envoy_wasm_.*",
		},
		monitoringv1.RelabelConfig{
			Action: "labeldrop",
			Regex:  "chart|destination_app|destination_version|heritage|.*operator.*|istio.*|release|security_istio_io_.*|service_istio_io_.*|sidecar_istio_io_inject|source_app|source_version",
		}))
}

func keepContainer(name string) monitoringv1.RelabelConfig {
	return monitoringv1.RelabelConfig{
		Action:       "keep",
		SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_container_name"},
		Regex:        name,
	}
}

func keepAnnotationPresent() monitoringv1.RelabelConfig {
	return monitoringv1.RelabelConfig{
		Action:       "keep",
		SourceLabels: []monitoringv1.LabelName{"__meta_kubernetes_pod_annotationpresent_prometheus_io_scrape"},
	}
}

func addressReplaceIPv6() monitoringv1.RelabelConfig {
	return monitoringv1.RelabelConfig{
		Action: "replace",
		Regex:  `(\d+);(([A-Fa-f0-9]{1,4}::?){1,7}[A-Fa-f0-9]{1,4})`,
		SourceLabels: []monitoringv1.LabelName{
			"__meta_kubernetes_pod_annotation_prometheus_io_port",
			"__meta_kubernetes_pod_ip",
		},
		TargetLabel: "__address__",
		Replacement: strPtr("[$2]:$1"),
	}
}

func addressReplaceIPv4() monitoringv1.RelabelConfig {
	return monitoringv1.RelabelConfig{
		Action: "replace",
		Regex:  `(\d+);((([0-9]+?)(\.|$)){4})`,
		SourceLabels: []monitoringv1.LabelName{
			"__meta_kubernetes_pod_annotation_prometheus_io_port",
			"__meta_kubernetes_pod_ip",
		},
		TargetLabel: "__address__",
		Replacement: strPtr("$2:$1"),
	}
}

func cloneRelabelConfigs(in []monitoringv1.RelabelConfig) []monitoringv1.RelabelConfig {
	out := make([]monitoringv1.RelabelConfig, len(in))
	for i, cfg := range in {
		out[i] = cfg
		if cfg.Separator != nil {
			out[i].Separator = strPtr(*cfg.Separator)
		}
		if cfg.Replacement != nil {
			out[i].Replacement = strPtr(*cfg.Replacement)
		}
		if len(cfg.SourceLabels) > 0 {
			out[i].SourceLabels = append([]monitoringv1.LabelName(nil), cfg.SourceLabels...)
		}
	}
	return out
}

func strPtr(s string) *string {
	return &s
}
