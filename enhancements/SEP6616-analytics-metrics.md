|Status                                  | Authors      | Created    | 
|----------------------------------------|--------------|------------|
|WIP                                     | @yxun        | 2026-09-17 |

# Sail Operator Analytics Metrics

## Overview

While we receive telemetry metrics for each version of the Sail Operator total installations and controller reconcile total counts, we do not have any metric for its CRDs and components usage. This SEP aims to introduce a set of custom metrics that help measure the magnitude of the APIs and CRDs usage.

## Goals

* Define and register custom metrics for the following resources managed by a Sail Operator on a given cluster.
  * Istio version counts (number of Istiod control planes at each Istio version)
  * ZTunnel Proxy counts (number of ZTunnel proxies managed by an Istiod)
  * Sidecar and Ambient namespace counts
* Record custom metrics in a Reconcile loop
* Configure ServiceMonitor resource(s) and send custom metrics to an in-cluster monitoring stack

## Non-goals

* Registering multi-cluster mesh custom metrics
* Modifying the existing CRDs or APIs
* Aggregating custom metrics data from all operator instances

## Design

A new `analytics` package will be introduced along with custom metric types. Each type will be defined as a standard Prometheus metric type. The custom metrics will be registered in the operator's scheme `init` function.

A new `generate-metricsdocs` target will be added in the Makefile. It will automatically create or update a list of the custom metrics with their description in a doc file.

The following two dependencies will be added in the `go.mod` file. They will be used for implementing metric types and a new controller.

- github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring
- github.com/prometheus/client_golang

A feature flag `ENABLE_ANALYTICS` will control the enablement of this new feature. By default, the Sail Operator will not enable analytics metrics recording. When it is set, a new `analytics` controller will run and start custom metrics collection.

The controller will watch `Istio` `IstioRevision` and `ZTunnel` custom resources. When those resources are created or deleted, the controller will increase or decrease their counts in the custom metrics accordingly. 

The controller will also watch namespace events. When a namespace is updated with the Istio sidecar injection or Ambient mode labels, the controller will record the number of those namespaces.

The controller will create a `ServiceMonitor` resource for scraping the Sail Operator existing metric endpoint and a `PrometheusRule` resource for defining recording rules based on the collected metrics.

Because there is an existing ClusterRole `sailoperator-metrics-reader` defined in the bundle manifests, the controller will create a ClusterRoleBinding with that ClusterRole and bind it with the default operator service account. This should allow access to the operator metric endpoint.

### User Stories

* A mesh admin wants to count the usage of Sail Operator CRDs and namespaces managed by an Istio service mesh.
* A mesh admin wants to automatically integrate Sail Operator metrics with existing in-cluster monitoring stack or telemetry pipeline.

### Performance Impact

The performance impact will depend on the implementation details. This design starts with a few basic custom resource counts and records those in Prometheus Gauges. The controller reconciles monitor resources for integration purpose. Those will not have performance impact. 

### Kubernetes vs OpenShift vs Other Distributions

Some of the metric names and descriptions will be adjusted to OpenShift Service Mesh names. The default namespace for monitor resources will be adjusted to an OpenShift specific namespace. 

## Implementation Plan

- [ ] Add a custom metric document that outlines the usage, limitation and cardinality
- [ ] Add custom metric types in the analytics package
- [ ] Implement controller changes for recording metrics and sending them to an in-cluster monitoring stack
- [ ] Add integration tests for verifying custom metrics from the operator metric endpoint

## Test Plan

* Ensure the `analytics` controller reconciles expected resources and it does not cause any conflict with other controllers.
* The first phase of integration tests will test configurations using a `curl` job for getting metrics from the operator metric endpoint.
* The next phase of integration tests will setup a monitoring stack such as Prometheus from a standard helm chart and verify metrics from the in-cluster Prometheus server.

## Change History (only required when making changes after SEP has been accepted)

