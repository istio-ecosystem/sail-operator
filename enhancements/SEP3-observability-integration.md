|Status                                             | Authors      | Created    | 
|---------------------------------------------------|--------------|------------|
|Implementation                                     | @mayleighnmyers, @yxun         | 2026-06-01 |

# Observability Integration (Metrics)

## Overview

Upstream Istio generates telemetry metrics for the control plane and sidecar proxies. Users can customize Istio metrics with the Telemetry API, but that does not cover scraping those metrics with Prometheus Operator. We want to automate creation of `ServiceMonitor` and `PodMonitor` resources so users can query Istio metrics in Prometheus or Kiali without manual configuration.

## Goals

* Provide a monitoring controller that reconciles `ServiceMonitor` and `PodMonitor` resources for Istio control plane and sidecar proxy metrics
* Apply platform-appropriate default relabeling rules on Kubernetes and OpenShift

## Non-goals

* Deploying observability stack components (Prometheus, COO, OpenShift user-workload monitoring, etc.). We assume those are installed separately.
* User customization of scrape paths, ports, or relabeling rules via the Sail API
* Tracing integration (tracked separately under [OSSM-14058](https://redhat.atlassian.net/browse/OSSM-14058))

## Design

### User Stories

1. As a user running Istio with a Prometheus Operator-based stack, I want the operator to configure metrics scraping jobs for Istio-generated telemetry metrics.
2. As a user running OpenShift Service Mesh, I want `PodMonitor` resources with OSSM-documented relabeling rules so metrics appear correctly in the OpenShift console and Kiali.

### API Changes

We will add a boolean field `spec.monitoring.enabled` on the `Istio` CR:

```yaml
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  monitoring:
    enabled: true
```

When enabled, the monitoring controller reconciles monitor CRs for each owned `IstioRevision`. The field defaults to `false`.

A broader observability integration API with references to external metrics and tracing stacks is being designed in [OSSM-14058](https://redhat.atlassian.net/browse/OSSM-14058). That work may supersede this boolean field in a future release.

### Architecture

We assume `ServiceMonitor` and `PodMonitor` CRDs are available under `monitoring.coreos.com/v1` and/or `monitoring.rhobs/v1`. The Sail Operator ClusterRole grants permissions for both API groups.

The monitoring controller watches `IstioRevision` resources and reconciles monitor CRs when monitoring is enabled on the parent `Istio` CR. It also watches `Istio` (for changes to `spec.monitoring.enabled`) and namespaces with the `istio-injection=enabled` label (for PodMonitor placement on OpenShift).

Monitor CRs are built using prometheus-operator Go API types and applied via the controller-runtime client. Platform-specific relabeling defaults are selected from `pkg/monitoring/relabeling` based on `ReconcilerConfig.Platform` (detected at startup via `config.DetectPlatform()`).

#### ServiceMonitor (istiod)

One `ServiceMonitor` per `IstioRevision` in the control plane namespace, named `{revision}-istiod`, with an owner reference to the `IstioRevision`. The spec follows upstream Istio and OSSM samples: selector `istio: pilot`, `targetLabels: [app]`, endpoint port `http-monitoring`, path `/metrics`, interval `30s`. No endpoint relabelings.

#### PodMonitor (istio-proxy)

One or more `PodMonitor` resources per `IstioRevision`, named `{revision}-proxies`. The spec uses selector `istio-prometheus-ignore` DoesNotExist, path `/stats/prometheus`, interval `30s`, and annotation-based scrape relabelings (no explicit port field).

Relabeling rules follow [upstream Istio prometheus-operator.yaml](https://github.com/istio/istio/blob/master/samples/addons/extras/prometheus-operator.yaml) on Kubernetes and the [OSSM 3.0 metrics documentation](https://docs.redhat.com/en/documentation/red_hat_openshift_service_mesh/3.0/html/observability/metrics-and-service-mesh) on OpenShift (additional `app`, `version`, and `mesh_id` labels; `mesh_id` is set to the parent `Istio` CR name).

#### Kubernetes vs OpenShift

On **Kubernetes**, a single `PodMonitor` in the control plane namespace with `namespaceSelector.any: true` is the preferred approach (matching upstream Istio). On **OpenShift**, user-workload monitoring ignores `namespaceSelector`, so a `PodMonitor` must be created in each mesh namespace with sidecar injection enabled.

The controller currently creates a `PodMonitor` per namespace with `istio-injection=enabled`. Platform-specific API group selection (`monitoring.coreos.com` vs `monitoring.rhobs`) is not yet implemented; created objects use `monitoring.rhobs/v1`.

### Performance Impact

`ServiceMonitor` reconciliation is limited to the control plane namespace and has low impact. `PodMonitor` reconciliation on OpenShift requires listing namespaces and may impact performance on clusters with many namespaces. Using `namespaceSelector` on Kubernetes avoids this per-namespace loop.

## Alternatives Considered

### Embedded YAML templates in Go

Rejected. Monitor CR specs should be built entirely from typed Go structs via the controller-runtime client, not from YAML fragments embedded in controller code.

### Named port scraping (`http-envoy-prom`)

Rejected. Upstream Istio and OSSM samples use annotation-based address relabeling without an explicit port field.

### Single PodMonitor strategy for all platforms

Rejected on OpenShift. User-workload monitoring ignores `namespaceSelector` on monitor CRs, requiring per-namespace PodMonitors per OSSM documentation.

### Standalone Integration CR with target references

A standalone `Integration` CR referencing `Istio`, metrics stacks, and tracing stacks (similar to `targetRef` on `ZTunnel` and `IstioRevisionTag`) is preferred long term for opt-in RBAC and stack-specific configuration. That design is tracked in [OSSM-14058](https://redhat.atlassian.net/browse/OSSM-14058) rather than in this SEP.

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
- [ ] [OSSM-14058](https://redhat.atlassian.net/browse/OSSM-14058) — Integration API with target references for metrics and tracing

## Test Plan

Functionality will be tested in unit tests, integration tests (envtest), and KinD e2e tests.

## Updates

* 2026-06-11: Updated with platform relabeling implementation, remaining v1alpha1 work items, and OSSM-14058 reference
