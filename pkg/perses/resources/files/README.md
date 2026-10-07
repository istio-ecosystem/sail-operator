# Istio Perses dashboards

Vendored `PersesDashboard` manifests from [perses/community-mixins](https://github.com/perses/community-mixins).

Source: `examples/dashboards/operator/istio/`

| Sail Version | Ref | Updated
|-------|-------|-------|
| 1.31.0 | `d4e23ad64ea8087c74b94ffc4bd2aae1122f2386` | `2026-10-06` |

Pin cadence: bump the Ref only when community-mixins has dashboard changes worth
shipping (typically after Istio or Grafana mixin updates land there). There is no
required bump per Sail release. Prefer not to change the pin on patch releases
unless a dashboard fix is needed. See `docs/addons/perses.adoc` for the refresh
procedure.

## Bundled dashboards

| Dashboard ID | Display name |
|--------------|--------------|
| `istio-control-plane-dashboard` | Istio Control Plane Dashboard |
| `istio-mesh-dashboard` | Istio Mesh Dashboard |
| `istio-performance-dashboard` | Istio Performance Dashboard |
| `istio-service-dashboard` | Istio Service Dashboard |
| `istio-workload-dashboard` | Istio Workload Dashboard |
| `istio-ztunnel-dashboard` | Istio Ztunnel Dashboard |
| `istio-wasm-extension-dashboard` | Istio Wasm Extension Dashboard |

Dashboard IDs must remain stable for Kiali and other consumers.

These files are packaged in the embedded Helm chart at `pkg/perses/resources/` (`Chart.yaml`, `files/`, `templates/`).

## Behavior

When `enablePersesDashboards` is true and the `PersesDashboard` CRD is present,
the Sail Operator installs or upgrades every YAML in this directory into its own namespace (the namespace
where the operator pod runs) via Helm release `sail-perses-dashboards`. The feature is disabled by default upstream. Enable with
`--enable-perses-dashboards=true` or Helm `operator.enablePersesDashboards: true`.
Missing CRDs do not block operator installation.

Bundled dashboards are upgraded when the operator starts with a newer chart. Dashboards are not deleted when the feature is disabled and have no ownerReferences.

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

To refresh these files from upstream, see `docs/addons/perses.adoc`.
