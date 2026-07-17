|Status                                             | Authors      | Created    | 
|---------------------------------------------------|--------------|------------|
|Implementation                                     | @mayleighnmyers, @yxun         | 2026-06-01 |

# Observability Metrics Integration

## Overview

Upstream Istio generates telemetry metrics for the control plane and sidecar proxies. Users can customize Istio metrics with the Telemetry API and create scraping jobs by updating a Prometheus ConfigMap. For example, The following sample deploys a Prometheus instance and configures Istio telemetry integration in the Prometheus ConfigMap [samples/addons/prometheus.yaml](https://raw.githubusercontent.com/istio/istio/master/samples/addons/prometheus.yaml). However, that approach requires restarting the Prometheus instance and users may not have enough permission to update the existing Prometheus's ConfigMap.

Alternatively, those can be configured by using two custom resources `ServiceMonitor` and `PodMonitor`. We want to automate the creation of `ServiceMonitor` and `PodMonitor` resources with default values in Sail Operator. And then users can query Istio metrics in Prometheus or Kiali dashboard.

## Goals

* Provide a monitoring controller that reconciles `ServiceMonitor` and `PodMonitor` resources from the Prometheus Operator.
* Apply default configurations for scraping Istio control plane and sidecar proxy metrics.
* Apply platform-appropriate default relabeling rules on Kubernetes and OpenShift platforms.

## Non-goals

* Deploying observability stack components (Prometheus, Prometheus Operator, OpenShift Cluster Observability Operator, OpenShift User-Workload Monitoring, etc.). This enhancement requires that the Prometheus Operator and Prometheus instance are running in a cluster.
* Updating `ServiceMonitor` and `PodMonitor` for user customization of scraping paths, ports, or relabeling rules via the Sail Operator API.
* Attaching `ServiceMonitor` and `PodMonitor` custom labels for selector use cases.
* Adding Observability Distributed Tracing integration.
* Ambient mode metrics integration is out of scope. Scraping configurations of `ZTunnel`component and Waypoint proxies are non-goals in this SEP.

## Design

### User Stories

1. As a user running Istio with a Prometheus production stack, I want the Sail Operator to configure metrics scraping jobs for Istio-generated telemetry metrics.
2. As a user running OpenShift Service Mesh, I want the Sail Operator to create `PodMonitor` resources with platform-appropriate relabeling rules. So those metrics can be formatted and displayed correctly in the OpenShift console and Kiali dashboard.

### API Changes

We will add an annotation `sailoperator.io/monitoring` in the `Istio` Custom Resource(CR). For example,

```yaml
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
  annotations:
    sailoperator.io/monitoring: enabled
```

When the annotation `sailoperator.io/monitoring: enabled` is set, the monitoring controller reconciles `ServiceMonitor` and `PodMonitor` CRs for each owned `IstioRevision`.

If any custom relabeling or scraping configuration is required, users may set the annotation `sailoperator.io/monitoring: enabled` and then they can apply/update independent `ServiceMonitor` and `PodMonitor` resource changes. The monitoring controller will not overwrite or update those independent changes in its reconciliation.

#### Alternatives API Considered

A broader Observability Integration API and CRD can be added in a future enhancement. It uses target reference fields and helps integrating more monitoring and tracing component services.

Additional configuration options can be added as new `Istio` Custom Resource spec `type` fields. We may revisit those ideas after the initial implementation of metrics integration in Sail Operator.

### Architecture

We assume `ServiceMonitor` and `PodMonitor` CRDs are available from the Prometheus Operator. They are under the resource group `monitoring.coreos.com`. We use the Sail Operator ClusterRole grants permissions for both `ServiceMonitor` and `PodMonitor` custom resources.

A monitoring controller watches `IstioRevision` resources and reconciles `ServiceMonitor` and `PodMonitor` CRs when the annotation `sailoperator.io/monitoring: enabled` is set in the parent `Istio` CR. It also watches namespaces with the Istio sidecar injection labels and reconciles `PodMonitor` CRs for those namespaces.

The monitoring controller applies platform-specific relabeling defaults when creating `ServiceMonitor` and `PodMonitor` CRs. It detects the running platform at the Sail Operator startup time.

Attaching `ServiceMonitor` and `PodMonitor` custom labels for selector use cases is a non-goal in this enhancement. We will address common integration use cases in the monitoring controller default label(s) creation such as mapping Prometheus stack helm chart release name to a Monitor resource's `release` label. Other use cases can be added in next enhancement proposal.

#### Monitoring Controller Design

When the annotation `sailoperator.io/monitoring: enabled` is set in an `Istio` custom resource, the monitoring controller reconciles `ServiceMonitor` or `PodMonitor` resources. When the annotation `sailoperator.io/monitoring: enabled` is dropped, the monitoring controller stops reconciling.

The monitoring controller only creates monitoring resources if they do not exist. Those resources are created by a naming convention and ownership label(s). It will not overwrite or update user-managed `ServiceMonitor` or `PodMonitor` resource changes in existing environments.

The controller creates and reconciles `ServiceMonitor` resources named as `{IstioRevision.name}-istiod-metrics` in the control plane namespace. And it creates and reconciles `PodMonitor` resources named as `{IstioRevision.name}-proxies-metrics` in each matching application namespace. It uses Get/Create/Update methods and does not list or bulk-delete monitoring resources in a namespace.

#### Custom Resource Deletion

The **ServiceMonitor** custom resource carries an `ownerReferences` field which points at the owning `IstioRevision` resource. When the `IstioRevision` resource is deleted, Kubernetes garbage collection removes the `ServiceMonitor` resource. 

The **PodMonitor** custom resource does not carry an `ownerReferences` field because cross-namespace owner references are not supported. Those resources are not automatically garbage-collected when the owner `IstioRevision` is deleted. The monitoring controller explicitly handles the cleanup of those `PodMonitor` resources when the sidecar injection label(s) are dropped or disabled in application namespaces or pods.

#### Sidecar injection namespace selection

The monitoring controller reconciliation watches namespaces and pods with the Istio sidecar injection labels. For example:

| Namespace label | Resolved revision |
|-----------------|-------------------|
| `istio-injection: enabled` | `default` |
| `istio.io/rev: <name>` | `<name>` |

| Pod label | Resolved revision |
|-----------------|-------------------|
| `sidecar.istio.io/inject: true` | `default` |

It also watches `IstioRevisionTag` custom resource objects and reconciles `PodMonitor` resource(s) in their namespace(s).

#### Kubernetes vs OpenShift

On **Kubernetes** platform, the monitoring controller creates a `PodMonitor` resource in the control plane namespace. The resource uses a `namespaceSelector` field for namespace selection.

On **OpenShift** platform, the default Prometheus monitoring stack ignores a `namespaceSelector` field, so we need create `PodMonitor` resources in those sidecar injection enabled namespaces.

#### External CRD/API Group

The monitoring controller should detect an API group is available from the Prometheus Operator before creating `ServiceMonitor` or `PodMonitor` resources:

| API group | Typical platform |
|-----------|------------------|
| `monitoring.coreos.com/v1` | Kubernetes Prometheus Operator |
| `monitoring.rhobs/v1` | OpenShift Cluster Observability Operator |

By default, the monitoring controller will create the `ServiceMonitor` and `PodMonitor` resources using the `monitoring.coreos.com` group.

#### Monitoring Availability

When the annotation `sailoperator.io/monitoring: enabled` is set in an `Istio` custom resource, but the expected `ServiceMonitor` and `PodMonitor` objects are not installed, the monitoring controller should set a status condition in the `Istio` custom resource. For example:

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
- [x] Add KinD e2e tests
- [ ] Add a watch for `IstioRevisionTag` custom resources and a watch for Pods with `sidecar.istio.io/inject` label
- [ ] User-facing documentation

Alternatives API Considered
- [ ] Introduce an Integration API in v1alpha1 api group. It uses target references fields for integrating more Observability services.

## Test Plan

Functionality will be tested in unit tests, integration tests (envtest), and KinD e2e tests.

## Updates

* 2026-06-11: Updated with platform relabeling implementation, remaining v1alpha1 work items.
* 2026-06-23: Updated Overview, Goals, Non-goals, Design descriptions. Added Alternatives API Considered. Removed OSSM JIRA references.
* 2026-07-01: Update descriptions. Addressed review comments. Renamed this file with JIRA work item number.
* 2026-07-07: Update API changes.
* 2026-07-16: Address API change review feedback and move custom labels for selector use cases as a non-goal in this initial enhancement.