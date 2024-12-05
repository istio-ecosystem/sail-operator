# OpenShift Service Mesh 2.6 migration to 3.0

This document details how to migrate from 2.6 to OpenShift Service Mesh 3.0.

## Pre-migration Checklist

Before you begin to migrate your controlplane from OpenShift Service Mesh 2.6 to 3.0, ensure you have done the following:

- Upgrade your 2.6 OpenShift Service Mesh Operator to the latest release.
- Upgrade your `ServiceMeshControlPlane` version to the latest OpenShift Service Mesh release.
- Disable the following features on your `ServiceMeshControlPlane`. These fields are unsupported in 3.0 and must be disabled prior to migration.
    <!-- TODO: create a separate page for each of these bullet points describing how to migrate off the SMCP managed version. -->
    <!-- TODO: revisit this list when: https://issues.redhat.com/browse/OSSM-8299 is completed. -->
  - Network Policy management: `spec.security.manageNetworkPolicy=false`. If you wish to keep the Network Policies created by the 2.6 `ServiceMeshControlPlane`, you will need to recreate and manage these manually.
  - Disabled addons:
    - Prometheus: `spec.addons.prometheus.enabled=false`
    - Kiali: `spec.addons.kiali.enabled=false`
    - Grafana: `spec.addons.grafana.enabled=false`
    - Tracing: `spec.tracing.type=None`
  - IOR is disabled.
  - Default ingress/egress gateways are disabled.

Now you are ready to migrate. Check the `spec.mode` field on your `ServiceMeshControlPlane` resource to determine if you are running a `MultiTenant` or a `ClusterWide` mesh.

```sh
oc get smcp <smcp-name> -n <smcp-namespace> -o jsonpath='{.spec.mode}'
```

For `MultiTenant` meshes, follow [these instructions](./multi-tenancy/README.md). For `ClusterWide` meshes, follow [these instructions](#TODO).
