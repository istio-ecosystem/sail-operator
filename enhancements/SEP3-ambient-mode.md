|Status                                             | Authors      | Created    | 
|---------------------------------------------------|--------------|------------|
|Proposed | @yxun         | 2024-11-19 |

# Istio ambient mode

## Overview

Upstream Istio implements the ambient mode by splitting its functionality into two distinct layers. At the base, a per-node Layer 4(L4) ztunnel secure overlay handles routing and zero trust security for traffic. Above that, users can enable L7 waypoint proxies to get access the full range of Istio features.

The upstream Istio ambient mode can be installed and upgraded from istioctl with profile ambient. This is implemented by the upstream in-cluster IstioOperator resource APIs.

## Goals

- Provide a way for installing the ambient mode component profile and charts in Sail Operator. This includes the ztunnel component chart and binary, Istio CNI changes and related meshConfig in pilot.
- Provide a way for managing the ambient mode components life cycle. The implementation should enable in-place upgrades.

## Non-goals

- Uninstallation of only the ambient mode components. Currently, to cleanup the environment, need to compeletely uninstall Istio from a cluster.
- Istio Sidecar mode migration to ambient mode.
- Canary upgrade by the revision setting in ambient mode. 

## Design

### User Stories

1. As a user of Sail Operator, I want to be able to use a CRD and controller for installing and upgrading Istio ambient mode profile and components.
2. As a user of Sail Operator, I want to be able to run Istio ambient mode on a subset of OpenShift environments.

### API Changes
We will add a new CRD called `IstioZtunnel` that exposes ztunnel helm chart values from a `spec.values.ztunnel` field.

#### IstioZtunnel resource
Here's an example YAML for the new resource:
```yaml
apiGroup: sailoperator.io/v1alpha1
kind: IstioZtunnel
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

#### IstioZtunnel Status

The `status.state` field gives a quick hint as to whether a ztunnel DaemonSet has been reconcile ("Healthy") or if there are any problems with it.

### Architecture

To provide a developer preview of Istio Ambient mode using Sail Operator, we will need to design and implement new Sail Operator APIs, CRD and controller(s) for installing Istio Ambient mode profile and helm charts. This involves the following component helm charts and values:

* Istio ztunnel helm charts
* Istio CNI helm values, ClusterRole, ClusterRoleBinding, ConfigMap and DaemonSet updates
* Set Istio discovery Pilot environment variable `PILOT_ENABLE_AMBIENT: "true"`
* Set Istio `meshConfig.defaultConfig.proxyMetadata.ISTIO_META_ENABLE_HBONE: "true"`

Therefore, we'll start from introducing a new Sail Operator CRD called `IstioZtunnel` and mapping ztunnel helm chart values to the CRD APIs.

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
