|Status                                             | Authors      | Created    | 
|---------------------------------------------------|--------------|------------|
|Implementation                                     | @mayleighnmyers, @yxun         | 2026-06-01 |

# Observability Metrics Integration

## Overview

Upstream Istio generates telemetry metrics for the control plane and sidecar proxies. Users can customize Istio metrics with the Telemetry API and create scraping jobs by updating a Prometheus ConfigMap. For example, [prometheus.yaml](https://raw.githubusercontent.com/istio/istio/master/samples/addons/prometheus.yaml). However, that approach requires restarting the Prometheus instance and users may not have enough permission to update the existing Prometheus's ConfigMap.

Alternatively, those can be configured by using two custom resources `ServiceMonitor` and `PodMonitor`. We want to automate the creation of `ServiceMonitor` and `PodMonitor` resources with default values in Sail Operator. And then users can query Istio metrics in Prometheus or Kiali dashboard without manual configuration step.

## Goals

* Provide a monitoring controller that reconciles `ServiceMonitor` and `PodMonitor` resources for Istio control plane and sidecar proxy metrics.
* Apply platform-appropriate default relabeling rules on Kubernetes and OpenShift.

## Non-goals

* Deploying observability stack components (Prometheus, OpenShift Cluster Observability Operator, OpenShift User-Workload Monitoring, etc.). We assume those are installed separately.
* Updating `ServiceMonitor` and `PodMonitor` for user customization of scraping paths, ports, or relabeling rules via the Sail Operator API.
* Observability Distributed Tracing integration.
* Ambient mode metrics integration is out of scope. Scraping configurations of `ZTunnel`component and Waypoint proxies are non-goals in this SEP.

## Design

### User Stories

1. As a user running Istio with a Prometheus monitoring stack, I want the Sail Operator to configure metrics scraping jobs for Istio-generated telemetry metrics without restart the Prometheus instance.
2. As a user running OpenShift Service Mesh(OSSM), I want the Sail Operator to create `PodMonitor` resources with OSSM-documented relabeling rules so metrics appear correctly in the OpenShift console and Kiali dashboard.

### API Changes

We will add a boolean field `spec.monitoring.enabled` in the `Istio` Custom Resource(CR). For example,

```yaml
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  monitoring:
    enabled: false
```

This field defaults to `false`. When it is set to `true`, the monitoring controller reconciles `ServiceMonitor` and `PodMonitor` CRs for each owned `IstioRevision`. 

If any custom relabeling or scraping configuration is required, users must leave `spec.monitoring.enabled` as default value `false` and they can apply independent `ServiceMonitor` and `PodMonitor` resources manually. The monitoring controller will not delete or update those independent resources in its reconciliation.

#### Alternatives API Considered

A broader Observability Integration API with target references to monitoring and tracing stacks may be added in a future release. This enhancement focuses on the monitoring controller with a boolean field on the `Istio` CR.

### Architecture

We assume `ServiceMonitor` and `PodMonitor` CRDs are available under `monitoring.coreos.com/v1` and/or `monitoring.rhobs/v1` group. The Sail Operator ClusterRole grants permissions for both API groups' `ServiceMonitor` and `PodMonitor` custom resources.

A monitoring controller watches `IstioRevision` resources and reconciles `ServiceMonitor` and `PodMonitor` CRs when the boolean field `spec.monitoring.enabled` is set to `true` in the parent `Istio` CR. It also watches namespaces with the `istio-injection=enabled` label and reconciles `PodMonitor` CRs for Istio sidecar injection namespaces.

The monitoring controller applies platform-specific relabeling defaults when creating `ServiceMonitor` and `PodMonitor` CRs. The monitoring controller starts and detects the running platform at the Sail Operator startup time.

#### Monitoring Controller Design

When the `spec.monitoring.enabled` is `true` in an `Istio` custom resource, the monitoring controller reconciles `ServiceMonitor` or `PodMonitor` resources and adds label such as `managed-by`: `mesh-operator` in those resources. When the `spec.monitoring.enabled` is `false` in an `Istio` custom resource, the monitoring controller stops reconciling.

The monitoring controller only interacts with monitoring resources it creates itself, identified by a naming convention and labels. It will not delete or update user-managed `ServiceMonitor` or `PodMonitor` resources that were configured manually in existing environments.

The controller creates and reconciles only resources named `{IstioRevision.name}-istiod-metrics` by applying a `ServiceMonitor` resource in the control plane namespace. And it creates and reconciles `{IstioRevision.name}-proxies-metrics` by applying `PodMonitor` resources in each matching application namespace. It uses Get/Create/Update by exact name and does not list or bulk-delete monitoring resources in a namespace.

#### Custom Resource Deletion

The **ServiceMonitor** custom resource carries an `ownerReferences` field which points at the owning `IstioRevision` resource. When the `IstioRevision` resource is deleted, Kubernetes garbage collection removes the `ServiceMonitor` resource. 

The **PodMonitor** custom resource does not carry an `ownerReferences` field because cross-namespace owner references are not supported. Those resources are not automatically garbage-collected when the owner `IstioRevision` is deleted. The monitoring controller explicitly handles the cleanup of those `PodMonitor` resources in the following conditions:
- The owner `IstioRevision` is deleted. 
- The sidecar injection label(s) are dropped or disabled in application namespaces.

#### Sidecar injection namespace selection

The monitoring controller reconciliation watches namespaces with the Istio sidecar injection labels. For example:

| Namespace label | Resolved revision |
|-----------------|-------------------|
| `istio-injection: enabled` | `default` |
| `istio.io/rev: <name>` | `<name>` |

#### Kubernetes vs OpenShift

On **Kubernetes** platform, the monitoring controller creates a `PodMonitor` resource in the control plane namespace. The resource uses a `namespaceSelector` field for namespace selection.

On **OpenShift** platform, the default Prometheus monitoring stack ignores a `namespaceSelector` field, so we need create `PodMonitor` resources in those sidecar injection enabled namespaces.

#### External CRD/API Group Detection

The monitoring controller should detect which Operator API group is available before creating `ServiceMonitor` or `PodMonitor` resources:

| API group | Typical platform |
|-----------|------------------|
| `monitoring.coreos.com/v1` | Kubernetes Prometheus Operator |
| `monitoring.rhobs/v1` | OpenShift Cluster Observability Operator |

By default, the monitoring controller will create the `ServiceMonitor` and `PodMonitor` resources using the `monitoring.coreos.com` group.

#### Monitoring Availability

When the `spec.monitoring.enabled` is `true` in an `Istio` custom resource, but the required `ServiceMonitor` and `PodMonitor` CRDs are not installed, the monitoring controller should set a status condition in the `Istio` custom resource. For example:

| Field | Value |
|-------|-------|
| `type` | `MonitoringAvailable` |
| `status` | `False` |
| `reason` | `MissingCRDs` |
| `message` | `ServiceMonitor and PodMonitor CRDs not found` |

### Performance Impact

The `ServiceMonitor` reconciliation is only required in a mesh's control plane namespace and it has low performance impact. 
The `PodMonitor` reconciliation watches namespaces with the sidecar injection labels on OpenShift platform. It may impact performance when there are large number of namespaces in a cluster.

## Implementation Plan

- [x] Monitoring controller reconciling `ServiceMonitor` and `PodMonitor` for `IstioRevision`
- [x] `Istio.spec.monitoring.enabled` API
- [x] Platform-specific PodMonitor relabeling defaults
- [x] Unit tests for controller and relabeling package
- [ ] External CRD/API group (`monitoring.coreos.com` and `monitoring.rhobs`) detection 
- [ ] Kubernetes PodMonitor strategy using `namespaceSelector.any: true`
- [ ] Set a status condition in the `Istio` custom resource
- [ ] Add Integration tests such as `tests/integration/api/monitoring_test.go`
- [ ] Add KinD e2e tests
- [ ] User-facing documentation

Alternatives API Considered
- [ ] Introduce an Integration API in v1alph1 api group. It uses target references fields for integrating more Observability services.

## Test Plan

Functionality will be tested in unit tests, integration tests (envtest), and KinD e2e tests.

## Updates

* 2026-06-11: Updated with platform relabeling implementation, remaining v1alpha1 work items.
* 2026-06-23: Updated Overview, Goals, Non-goals, Design descriptions. Added Alternatives API Considered. Removed OSSM JIRA references.
* 2026-07-01: Update descriptions. Addressed review comments. Renamed this file with JIRA work item number.
