# Sail Operator Metrics
This document describes the custom telemetry metrics for the Sail operator and CRD usage.It aims to provide a list of those metrics that are collected and exposed by the operator.

The following section outlines the usage and limitations on metric counts and cardinality.The last section provides a development guide about how to add additional metrics for new functionalities.

## Sail Operator Custom Metrics List
### servicemesh_ambient_namespace_total
Total number of namespaces enrolled in Istio Ambient mode. Type: Gauge.

### servicemesh_istiod_total
Total number of Istiod control planes at each Istio version. Type: GaugeVec.

### servicemesh_sidecar_namespace_total
Total number of namespaces enrolled in Istio sidecar mode. Type: Gauge.

### servicemesh_sidecar_proxy_total
Total number of Envoy Sidecar proxies managed by an Istiod control plane. Type: GaugeVec.

### servicemesh_waypoint_proxy_total
Total number of Waypoint proxies managed by an Istiod control plane in Ambient mode. Type: Gauge.

### servicemesh_ztunnel_total
Total number of ZTunnel proxies managed by an Istiod control plane in Ambient mode. Type: GaugeVec.

## Developing new metrics
After developing new metrics or changing old ones, please run "make generate-metricsdocs" to regenerate this document.
If you feel that the new metric doesn't follow these rules, please change "analytics/metricsdocs" according to your needs.
