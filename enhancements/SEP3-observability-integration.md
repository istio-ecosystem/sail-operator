|Status                                             | Authors      | Created    | 
|---------------------------------------------------|--------------|------------|
|Implementation                                     | @mayleighnmyers, @yxun         | 2026-06-01 |

# Observability Integration (Metrics)

## Overview

Upstream Istio generates telemetry metrics for the control plane and sidecar proxies. Users can customize Istio metrics with the Telemetry API and create scraping jobs by updating a Prometheus ConfigMap. For example, [prometheus.yaml](https://raw.githubusercontent.com/istio/istio/master/samples/addons/prometheus.yaml). However, that approach requires restarting the Prometheus instance and users may not have enough permission to update the existing Prometheus's ConfigMap.

Alternatively, those can be configured by using two custom resources `ServiceMonitor` and `PodMonitor`. We want to automate the creation of `ServiceMonitor` and `PodMonitor` resources with default values in Sail Operator. And then users can query Istio metrics in Prometheus or Kiali dashboard without manual configuration step.

## Goals

* Provide a monitoring controller that reconciles `ServiceMonitor` and `PodMonitor` resources for Istio control plane and sidecar proxy metrics.
* Apply platform-appropriate default relabeling rules on Kubernetes and OpenShift.

## Non-goals

* Deploying observability stack components (Prometheus, OpenShift Cluster Observability Operator, OpenShift User-Workload Monitoring, etc.). We assume those are installed separately.
* Updating `ServiceMonitor` and `PodMonitor` for user customization of scraping paths, ports, or relabeling rules via the Sail Operator API.

  If custom relabeling or scraping configuration is required, leave `spec.monitoring.enabled: false` and deploy independent `ServiceMonitor` and `PodMonitor` resources manually.
* Observability Distributed Tracing integration.
* Ambient mesh metrics (`ZTunnel`, waypoint proxies).

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

#### Alternatives API Considered

A broader Observability Integration API with target references to monitoring and tracing stacks may be added in a future release. This enhancement focuses on the monitoring controller with a boolean field on the `Istio` CR.

### Architecture

We assume `ServiceMonitor` and `PodMonitor` CRDs are available under `monitoring.coreos.com/v1` and/or `monitoring.rhobs/v1` group. The Sail Operator ClusterRole grants permissions for both API groups' `ServiceMonitor` and `PodMonitor` custom resources.

A monitoring controller watches `IstioRevision` resources and reconciles `ServiceMonitor` and `PodMonitor` CRs when the boolean field `spec.monitoring.enabled` is set to `true` in the parent `Istio` CR. It also watches namespaces with the `istio-injection=enabled` label and reconciles `PodMonitor` CRs for Istio sidecar injection namespaces.

The monitoring controller applies platform-specific relabeling defaults when creating `ServiceMonitor` and `PodMonitor` CRs. The monitoring controller starts and detects the running platform at the Sail Operator startup time.

#### Resource lifecycle and deletion

The monitoring controller only interacts with monitoring resources it creates itself, identified by a fixed naming convention. It must **never** delete or modify user-managed `ServiceMonitor` or `PodMonitor` resources that were configured manually in existing environments.

The controller creates and reconciles only resources named `{IstioRevision.name}-istiod` (`ServiceMonitor`, in the control plane namespace) and `{IstioRevision.name}-proxies` (`PodMonitor`, in each matching workload namespace). It uses Get/Create/Update by exact name and does not list or bulk-delete monitoring resources in a namespace.

**ServiceMonitor:** carries an `ownerReferences` entry pointing at the owning `IstioRevision`. When the `IstioRevision` is deleted, Kubernetes garbage collection removes the `ServiceMonitor`. When `spec.monitoring.enabled` is `false`, the controller stops reconciling but does not delete the existing `ServiceMonitor`.

**PodMonitor:** does not carry an `ownerReferences` entry (cross-namespace owner references are not supported). `PodMonitor` resources are not automatically garbage-collected when an `IstioRevision` is deleted. Explicit controller cleanup of `{IstioRevision.name}-proxies` by name is planned for revision deletion, disabling monitoring, and namespace label changes.

User-managed monitors with other names are never touched. Because `spec.monitoring.enabled` defaults to `false`, existing clusters with manually configured monitors are unaffected until a user opts in.

#### Sidecar injection namespace selection

`PodMonitor` reconciliation is revision-scoped. Each `IstioRevision` receives a `PodMonitor` only in namespaces whose sidecar injection labels reference that revision:

| Namespace label | Resolved revision |
|-----------------|-------------------|
| `istio-injection: enabled` | `default` |
| `istio.io/rev: <name>` | `<name>` |

When both labels are present, `istio-injection` takes precedence. The Istio control plane namespace is excluded. The controller watches `Namespace` objects when `istio-injection` or `istio.io/rev` labels are added, removed, or changed.

#### Kubernetes vs OpenShift

On **Kubernetes** platform, applying a single `PodMonitor` CR in the control plane namespace with `namespaceSelector.any: true` is the preferred approach. That is matching upstream Istio Prometheus configuration example. 
On **OpenShift** platform, the User-Workload Monitoring stack ignores `namespaceSelector` field, so we need create a `PodMonitor` CR in each sidecar injection enabled namespace.

#### External CRD/API Group Detection

The monitoring controller must detect which Prometheus Operator API group is installed before creating `ServiceMonitor` or `PodMonitor` resources:

| API group | Typical platform |
|-----------|------------------|
| `monitoring.coreos.com/v1` | Upstream Prometheus Operator |
| `monitoring.rhobs/v1` | OpenShift Cluster Observability Operator |

Monitoring is available only when both `ServiceMonitor` and `PodMonitor` CRDs exist in the same API group. When both groups are present, prefer `monitoring.coreos.com/v1`.

#### Monitoring Availability

When `spec.monitoring.enabled` is `true` but the required CRDs are not installed, the monitoring controller should set a status condition on the parent `Istio` CR:

| Field | Value |
|-------|-------|
| `type` | `MonitoringAvailable` |
| `status` | `False` |
| `reason` | `MissingCRDs` |
| `message` | `Prometheus Operator CRDs not found` |

When `spec.monitoring.enabled` is `false`, the monitoring controller does not set this condition.

### Performance Impact

The `ServiceMonitor` reconciliation is only required in the control plane namespace and it has low performance impact. The `PodMonitor` reconciliation watches namespaces with the `istio-injection=enabled` label on OpenShift platform and it may impact performance on clusters when there are large number of existing namespaces.

## Implementation Plan

v1alpha1
- [x] Monitoring controller reconciling `ServiceMonitor` and `PodMonitor` for `IstioRevision`
- [x] `Istio.spec.monitoring.enabled` API
- [x] Platform-specific PodMonitor relabeling defaults
- [x] Unit tests for controller and relabeling package
- [ ] External CRD/API group detection (`monitoring.coreos.com` vs `monitoring.rhobs`)
- [ ] Kubernetes PodMonitor strategy using `namespaceSelector.any: true`
- [ ] Validation when monitoring CRDs are unavailable
- [ ] Integration tests (`tests/integration/api/monitoring_test.go`)
- [ ] KinD e2e tests
- [ ] User-facing documentation

v1alpha2
- [ ] Integration API with target references for metrics and tracing

## Test Plan

Functionality will be tested in unit tests, integration tests (envtest), and KinD e2e tests.

## Updates

* 2026-06-11: Updated with platform relabeling implementation, remaining v1alpha1 work items.
* 2026-06-23: Updated Overview, Goals, Non-goals, Design descriptions. Added Alternatives API Considered. Removed OSSM JIRA references.
