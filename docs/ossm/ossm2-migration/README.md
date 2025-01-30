# OpenShift Service Mesh 2.6 migration to 3.0

This document details how to migrate from 2.6 to OpenShift Service Mesh 3.0.

## Pre-migration Checklist

Before you begin to migrate your controlplane from OpenShift Service Mesh 2.6 to 3.0, ensure you have done the following:

- Read through the key OSSM 2.6 vs 3.0 Differences [document](./../ossm2-vs-ossm3.md) 
- Upgrade your 2.6 OpenShift Service Mesh Operator to the latest release. See warning below.
- Upgrade your `ServiceMeshControlPlane` version to the latest OpenShift Service Mesh release.
- Install your 3.0 Openshift Service Mesh Operator. See warning below.
- Upgrade your Kiali operator to the latest release.
- Disable the following features on your `ServiceMeshControlPlane`. These fields are unsupported in 3.0 and must be disabled prior to migration.
  - Network Policy management: `spec.security.manageNetworkPolicy=false`. If you wish to keep the Network Policies created by the 2.6 `ServiceMeshControlPlane`, you will need to recreate and manage these manually.
  - Disabled addons. The OpenShift Service Mesh 3.0 operator does not manage addons. If these are enabled on your `SerivceMeshControlPlane`, you will now need to manage these yourself by first migrating to an external instance and then disabling the addon.
    - Prometheus: `spec.addons.prometheus.enabled=false`. Follow [these instructions](https://docs.redhat.com/en/documentation/openshift_container_platform/4.17/html/service_mesh/service-mesh-2-x#ossm-integrating-with-user-workload-monitoring_observability) to configure your `ServiceMeshControlPlane` with OpenShift Monitoring as a replacement for the prometheus addon. Note that these instructions also detail how to install a standalone `Kiali` resource and it is ok to do both of these at the same time. If you are not using OpenShift Monitoring and instead are using a different prometheus solution, you may follow [these instructions](https://docs.redhat.com/en/documentation/red_hat_openshift_service_on_aws/4/html/service_mesh/service-mesh-2-x#integration-with-external-prometheus-installation).
    - Tracing: `spec.tracing.type=None`. Follow [these instructions](https://docs.redhat.com/en/documentation/openshift_container_platform/4.17/html/service_mesh/service-mesh-2-x#ossm-configuring-distr-tracing-tempo_observability) for instructions on how to configure your `ServiceMeshControlPlane` with OpenShift Distributed Tracing as a replacement for the tracing addon. If not using OpenShift Distributed Tracing, you can add the custom tracing endpoint to your `spec.meshConfig.extensionProviders` configuration and create the corresponding `Telemetry` resource.
    - Kiali: `spec.addons.kiali.enabled=false`. You will need to create a standalone `Kiali` resource to replace the addon. If you haven't already done this as part of the previous steps for Prometheus or Tracing, follow [these instructions](https://docs.redhat.com/en/documentation/red_hat_openshift_service_mesh/3.0.0tp1/html/observability/kiali-operator-provided-by-red-hat#ossm-install-kiali-operator_ossm-kiali-assembly) on how to install and configure Kiali as a standalone resource.
    - Grafana: `spec.addons.grafana.enabled=false`. Grafana is not supported in OpenShift Service Mesh 3.0.
  - Istio OpenShift Routing (IOR) is disabled. Follow [these instructions](https://docs.redhat.com/en/documentation/openshift_container_platform/4.17/html/service_mesh/service-mesh-2-x#ossm-route-migration) to migrate off of IOR.
  - Default ingress/egress gateways are disabled. Follow [these instructions](https://docs.redhat.com/en/documentation/openshift_container_platform/4.17/html/service_mesh/service-mesh-2-x#ossm-gateway-migration) to migrate off of the default ingress/egress gateways.
- Run the [migration-checker script](migration-checker.sh) to detect any issues with your environment.

> [!WARNING]
> You must upgrade your OpenShift Service Mesh 2 Operator to the latest release **before** you install the OpenShift Service Mesh 3 operator. If you upgrade your OpenShift Service Mesh 2 operator _after_ you install your OpenShift Service Mesh 3 operator, you will need to then uninstall and reinstall your OpenShift Service Mesh 3 operator to ensure the included CRDs are up to date.

By the end of this checklist, your `ServiceMeshControlPlane` should look something like this:

```yaml
apiVersion: maistra.io/v2
kind: ServiceMeshControlPlane
metadata:
  name: basic
  namespace: istio-system
spec:
  version: v2.6 # 1.
  security: # 2.
    manageNetworkPolicy: false
  addons: # 3.
    grafana:
      enabled: false
    kiali:
      enabled: false
    prometheus:
      enabled: false
  meshConfig: # 4.
    extensionProviders:
      - name: prometheus
        prometheus: {}
      - name: otel
        opentelemetry:
          port: 4317
          service: otel-collector.istio-system.svc.cluster.local
  gateways: # 5.
    enabled: false
    openshiftRoute:
      enabled: false
  mode: MultiTenant # or ClusterWide
  tracing: # 3.
    type: None
```

1. Your `ServiceMeshControlPlane` has been updated to the latest version.
2. Network policy management has been disabled.
3. Addons and tracing have all been disabled.
4. Your `ServiceMeshControlPlane` is configured to use external metrics and tracing providers.
5. Managed gateways are disabled.

You will also have a `Telemetry` resource in your root namespace e.g. `istio-system` that looks roughly like this:

```yaml
apiVersion: telemetry.istio.io/v1
kind: Telemetry
metadata:
  name: mesh-default
  namespace: istio-system
spec:
  metrics: # 1.
    - providers:
        - name: prometheus
  tracing: # 2.
    - providers:
        - name: otel
```

1. Your metrics provider is specified. The `name` should match what is specified on your `ServiceMeshControlPlane` in the `spec.meshConfig.extensionProviders` field.
2. Your tracing provider is specified. The `name` should match what is specified on your `ServiceMeshControlPlane` in the `spec.meshConfig.extensionProviders` field.

And a `Kiali` resource that looks roughly like this:

```yaml
apiVersion: kiali.io/v1alpha1
kind: Kiali
metadata:
  name: kiali
  namespace: istio-system
spec:
  version: default # 1.
  external_services:
    prometheus: # 2.
      auth:
        type: bearer
        use_kiali_token: true
      thanos_proxy:
        enabled: true
      url: https://thanos-querier.openshift-monitoring.svc.cluster.local:9091
    tracing: # 3.
      enabled: true
      provider: tempo
      use_grpc: false
      internal_url: http://tempo-sample-query-frontend.tempo:3200
      external_url: https://tempo-sample-query-frontend-tempo.apps-crc.testing
    grafana: # 4.
      enabled: false
```

1. You are using the latest version of `Kiali`. As long as you install the 3.0 Service Mesh operator before upgrading Kiali, this version is compatible with both the 2.6 and 3.0 controlplanes.
2. Kiali is configured to use the external prometheus.
3. Kiali is configured to use the external tracing store.
4. Grafana configuration is disabled. Grafana is not supported with OpenShift Service Mesh 3.0.

Now you are ready to migrate. Check the `spec.mode` field on your `ServiceMeshControlPlane` resource to determine if you are running a `MultiTenant` or a `ClusterWide` mesh.

```sh
oc get smcp <smcp-name> -n <smcp-namespace> -o jsonpath='{.spec.mode}'
```

For `MultiTenant` meshes, follow [these instructions](./multi-tenancy/README.md). For `ClusterWide` meshes, follow [these instructions](./cluster-wide/README.md). When the migration is finished, follow [these instructions](./cleaning-2.6/README.md) to remove OpenShift Service Mesh 2.6.

## Post-migration Checklist

After the migration is done, you can optionally verify whether all data plane namespaces are migrated to the newer version of OSSM by checking it in Kiali Mesh page.
More information about the usage of Kiali Mesh page can be found in [this documentation](https://kiali.io/docs/features/istio-component-status/#control-plane-namespace)
