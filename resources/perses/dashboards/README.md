# Istio Perses dashboards

Vendored `PersesDashboard` manifests from [perses/community-mixins](https://github.com/perses/community-mixins).

| Field | Value |
|-------|-------|
| Source | `examples/dashboards/operator/istio/` |
| Ref | `v0.7.0` |
| Last updated | `2026-09-23` |

## Bundled dashboards

| Dashboard ID | Display name |
|--------------|--------------|
| `istio-control-plane` | Istio Control Plane Dashboard |
| `istio-mesh-dashboard` | Istio Mesh Dashboard |
| `istio-performance` | Istio Performance Dashboard |
| `istio-service-dashboard` | Istio Service Dashboard |
| `istio-workload-dashboard` | Istio Workload Dashboard |
| `istio-ztunnel-dashboard` | Istio Ztunnel Dashboard |

Dashboard IDs must remain stable for Kiali and other consumers.

## Behavior

When the `PersesDashboard` CRD is present in the cluster, the Sail Operator creates these dashboards in its own namespace (the namespace where the operator pod runs). Missing CRDs do not block operator installation or report a failure.

Reconciliation is create-if-not-exists: existing dashboards are left unchanged. Dashboards are not deleted and have no ownerReferences.

## PersesDatasource requirement

The Sail Operator does **not** create or manage `PersesDatasource` resources. You must create one named `prometheus-datasource` in the operator namespace before the dashboards can display data. Support for `PersesGlobalDatasource` is out of scope.

Example:

```yaml
apiVersion: perses.dev/v1alpha2
kind: PersesDatasource
metadata:
  name: prometheus-datasource   # required name
  namespace: sail-operator      # operator namespace
spec:
  # configure your Prometheus/Thanos endpoint
```

Regenerate these files with `hack/perses/update-dashboards.sh [COMMUNITY_MIXINS_REF]`.

For an OpenShift validation environment, see `hack/perses/README.md`.
