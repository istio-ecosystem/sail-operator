|Status                                             | Authors      | Created    | 
|---------------------------------------------------|--------------|------------|
|Proposed | @mayleighnmyers, @yxun         | 2026-06-01 |

## Overview
Upstream Istio generates detailed telemetry metrics for the mesh control plane and each sidecar proxies generate a set of metrics about all traffic passing through the proxy. Users can customize Istio metrics with Telemetry API. However, those APIs are not covering collection, scraping of metrics from a backend service such as a Prometheus Addon. We want to make it easier for users to integrate Istio with Observability metrics components using Sail operator.

We are targeting two Custom Resource Definitions (CRDs), `ServiceMonitor` and `PodMonitor`, that provide management of Prometheus related scraping jobs. By having a monitoring controller in the Sail operator, it would automate the creation, deletion and updates of those two CRDs and reduce manual configuration steps for users to query Istio generated telemetry metrics in a Prometheus or Kiali dashboard.

## Goals
- Implement a monitoring controller that manages two Custom Resource Definitions (CRDs), `ServiceMonitor` and `PodMonitor` reconciliation so users don't have to configure metrics scraping jobs manually. 

## Non-goals
- Deployment of the Observability metrics components are not goals. We will implement validation methods for checking required CRD groups and CRDs. We assume the Observability components such Observability operator(s), Prometheus monitoring stacks have been deployed in a Kubernetes environment. 

## Design

### User Stories
1. As a user of Istio's metrics monitoring using Prometheus components, I want to automatically configure Prometheus metrics scraping jobs for Istio generated telemetry metrics.

### API Changes
We will add a boolean field `spec.monitoring.enabled` in the `Istio` and `IstioRevision` CRDs. When a user enables it, a monitoring controller will reconcile a `ServiceMonitor` CR for the mesh control plane and `PodMonitor` CRs for each namespace labeled by the Istio sidecar injection label.

### Architecture
We assume the related `ServiceMonitor` and `PodMonitor` CRDs and the API groups such as "monitoring.coreos.com/v1" , "monitoring.rhobs/v1" are available. We will add required RBAC permissions for creating, deleting and updating those two resources in the Sail Operator ClusterRole.

A monitoring controller should watch `Istio` and `IstioRevision` resources before reconciling a `ServiceMonitor` object in the mesh control plane namespace. It should watch namespaces with Istio sidecar injection labels before reconciling `PodMonitor` objects for the application namespaces.
When the monitoring controller creates or updates `ServiceMonitor` and `PodMonitor` resources, it will construct default metadata and spec fields for scraping Istio generated telemetry metrics.

### Performance Impact
The `ServiceMonitor` CR reconciliation will not affect the Sail Operator performance because it only watches the mesh control plane namespace. 

The `PodMonitor` CRs reconciliation need to watch all namespaces and determine which one(s) has the Istio sidecar injection label. It will impact the performance when a cluster has large number of namespaces. We may consider an alternative option for implementing a field selector and reduce this performance impact.

## Implementation Plan
v1alpha1
- [X] Initial implementation of a monitoring controller and integration tests.
- [] Provide customized metrics relabeling configuration specs for a `PodMonitor` resource builder.

## Test Plan
Functionality will be tested in integration tests.


