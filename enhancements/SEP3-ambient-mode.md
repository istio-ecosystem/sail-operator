|Status                                             | Authors      | Created    | 
|---------------------------------------------------|--------------|------------|
|Proposed | @yxun         | 2024-11-19 |

# Istio ambient mode

## Overview

Upstream Istio implements the ambient dataplane mode by splitting its functionality into two distinct layers. At the base, a per-node Layer 4 (L4) ztunnel secure overlay handles routing and zero trust security for traffic. Above that, users can enable Layer 7 (L7) waypoint proxies to get access the full range of Istio features. This layered approach enables incremental adoption of Istio, allowing a seamless transition from no mesh to a secure Layer 4 (L4) overlay, and optionally to full Layer 7 (L7) processing.

The upstream Istio ambient mode can be installed and upgraded from istioctl with profile ambient. This is implemented by the upstream in-cluster IstioOperator resource APIs.

## Goals

- Provide a way for installing Istio with the Ambient profile using the Sail Operator. This involves the ztunnel chart and necessary updates to the IstioCNI and Istiod control plane components.
- Provide a way for managing the ambient mode components life cycle. The implementation should enable in-place upgrades.

## Non-goals

- Uninstallation of only the ambient mode components. Currently, to cleanup the environment, need to compeletely uninstall Istio from a cluster.
- Migration of Istiod control plane and dataplane components between sidecar mode and Ambient dataplane mode (and vice versa).
- Canary upgrade by the revision setting in ambient mode. 

## Design

### User Stories

1. As a Sail Operator user, I want the ability to use a CRD and controller to install and upgrade Istio with the Ambient mode profile.
2. As a Sail Operator user, I want the ability to run Istio ambient mode on both Kubernetes and OpenShift environments.

### API Changes
We will add a new CRD called `ZTunnel` that exposes ztunnel helm chart values from a `spec.values.ztunnel` field.

#### ZTunnel resource
Here's an example YAML for the new resource:
```yaml
apiGroup: sailoperator.io/v1alpha1
kind: ZTunnel
metadata:
  name: default
  namespace: kube-system
spec:
    name: default
    version: v1.24.0    # Defines the version of Istio
    profile: ambient    # The built-in installation configuration profile to use.
    values:
      ztunnel:
        hub: gcr.io/istio-testing
        tag: latest
        image: ztunnel
        istioNamespace: istio-system
        logLevel: info
status:
  observedGeneration: 1
  conditions: []
  state: Healthy
```

In the `spec.values` field, users can specify helm chart values for the ztunnel component. These APIs are mapped from upstream Istio `manifests/charts/ztunnel/values.yaml` file. Users can also specify global values in the `spec.values.global` field. These APIs are the same as the IstioCNI global values APIs.

#### ZTunnel Status

The `status.state` field gives a quick hint as to whether a ztunnel DaemonSet has been reconciled ("Healthy") or if there are any problems with it.

### Architecture

To provide support for Istio Ambient mode using Sail Operator, we will need to design and implement new Sail Operator APIs, CRD and controller(s) for installing Ambient profile. This involves the following changes:

* Istio ztunnel helm charts
* Istio CNI helm values, ClusterRole, ClusterRoleBinding, ConfigMap and DaemonSet updates
* Set Istio discovery Pilot environment variable `PILOT_ENABLE_AMBIENT: "true"`
* Set Istio `meshConfig.defaultConfig.proxyMetadata.ISTIO_META_ENABLE_HBONE: "true"`

Therefore, we'll start from introducing a new Sail Operator CRD called `IstioZtunnel` and map ztunnel helm chart values to the CRD APIs.

### Performance Impact

### Backward Compatibility

### Kubernetes vs OpenShift vs Other Distributions

## Alternatives Considered

### Reuse `IstioCNI`'s type field for specifying a ztunnel component
We could expend the `IstioCNI` resource APIs and `IstioCNISpec` values field for a ztunnel component installation and life-cycle management. It would have the benefit that combines IstioCNI and ztunnel components together because both of them are deployed as a Kubernetes DaemonSet on each node. The disadvantage though is that it would be confusing for users to manage either IstioCNI or ztunnel life-cycle.

## Implementation Plan
v1alpha1
- [] Initial implementation & unit tests
- [] Documentation

## Test Plan
Functionality will be tested in integration tests.
