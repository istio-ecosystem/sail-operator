# Scoping the service mesh with DiscoverySelectors
This page describes how the service mesh control plane discovers and observes cluster resources and how to manage this scope.

A service mesh will include a workload that:
1. Has been discovered by the control plane
1. Has been [injected with a Envoy proxy sidecar](../injection/README.md)

By default, the control plane will watch ("discover") all namespaces within the cluster, meaning that:
- Each proxy instance will receive configuration for all namespaces. This includes information also about workloads that are not enrolled in the mesh.
- Any workload with the appropriate pod or namespace injection label will be injected with a proxy sidecar.

This may not be desirable in a shared cluster. You may want to limit the scope of the service mesh to only a portion of your cluster. This is particularly important if you plan to have [multiple service meshes within the same cluster](./multi-control-planes/README.md).

### DiscoverySelectors
Discovery selectors provide a mechanism for the mesh administrator to limit the control plane's visibility to a defined set of namespaces. This is done through a Kubernetes [label selector](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors), which defines criteria for which namespaces will be visible to the control plane. Any namespaces not matching are ignored by the control plane entirely.

> **_NOTE:_** Istiod will always open a watch to OpenShift for all namespaces. However, discovery selectors will ignore objects that are not selected very early in its processing, minimizing costs.

> **_NOTE:_** `discoverySelectors` is not a security boundary. Istiod will continue to have access to all namespaces even when you have configured your `discoverySelectors`.

 #### Using DiscoverySelectors
The `discoverySelectors` field accepts an array of Kubernetes [selectors](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#resources-that-support-set-based-requirements). The exact type is `[]LabelSelector`, as defined [here](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#resources-that-support-set-based-requirements), allowing both simple selectors and set-based selectors. These selectors apply to labels on namespaces.

You can configure each label selector for a variety of use cases, including but not limited to:

- Arbitrary label names/values, for example, all namespaces with label `istio-discovery=enabled`
- A list of namespace labels using set-based selectors which carries OR semantics, for example, all namespaces with label `istio-discovery=enabled` OR `region=us-east1`
- Inclusion and/or exclusion of namespaces, for example, all namespaces with label `istio-discovery=enabled` AND label key `app` equal to `helloworld`

#### Using Discovery Selectors to Scope of a Service Mesh
Assuming you know which namespaces to include as part of the service mesh, as a mesh administrator, you can configure `discoverySelectors` at installation time or post-installation by adding your desired discovery selectors to Istio’s MeshConfig resource. For example, you can configure Istio to discover only the namespaces that have the label `istio-discovery=enabled`.

##### Prerequisites
- The OpenShift Service Mesh operator has been installed
- An Istio CNI resource has been created
- The `istioctl` binary has been installed on your localhost

1. Create the `istio-system` system namespace:
    ```bash
    oc create ns istio-system
    ```
1. Label the `istio-system` system namespace:
    ```bash
    oc label ns istio-system istio-discovery=enabled
    ```
1. Prepare `istio.yaml` with `discoverySelectors` configured:
    ```yaml
    kind: Istio
    apiVersion: sailoperator.io/v1alpha1
    metadata:
      name: default
    spec:
      namespace: istio-system
      values:
        meshConfig:
          discoverySelectors:
            - matchLabels:
                istio-discovery: enabled
      updateStrategy:
        type: InPlace
      version: v1.23.0
    ```
1. Apply the Istio CR:
    ```bash
    oc apply -f istio.yaml
    ```
1. Create first application namespace:
    ```bash
    oc create ns app-ns-1
    ```
1. Create second application namespace:
    ```bash
    oc create ns app-ns-2
    ```
1. Label first application namespace to be matched by defined `discoverySelectors` and enable sidecar injection:
    ```bash
    oc label ns app-ns-1 istio-discovery=enabled istio-injection=enabled
    ```
1. Deploy the sleep application to the first namespaces:
    ```bash
    oc apply -f https://raw.githubusercontent.com/istio/istio/release-1.23/samples/sleep/sleep.yaml -n app-ns-1
    ```
1. Deploy the sleep application to the second namespaces:
    ```bash
    oc apply -f https://raw.githubusercontent.com/istio/istio/release-1.23/samples/sleep/sleep.yaml -n app-ns-2
    ```
1. Verify that you don't see any endpoints from the second namespace:
    ```bash
    istioctl pc endpoint deploy/sleep -n app-ns-1
    ENDPOINT                                                STATUS      OUTLIER CHECK     CLUSTER
    10.128.2.197:15010                                      HEALTHY     OK                outbound|15010||istiod.istio-system.svc.cluster.local
    10.128.2.197:15012                                      HEALTHY     OK                outbound|15012||istiod.istio-system.svc.cluster.local
    10.128.2.197:15014                                      HEALTHY     OK                outbound|15014||istiod.istio-system.svc.cluster.local
    10.128.2.197:15017                                      HEALTHY     OK                outbound|443||istiod.istio-system.svc.cluster.local
    10.131.0.32:80                                          HEALTHY     OK                outbound|80||sleep.app-ns-1.svc.cluster.local
    127.0.0.1:15000                                         HEALTHY     OK                prometheus_stats
    127.0.0.1:15020                                         HEALTHY     OK                agent
    unix://./etc/istio/proxy/XDS                            HEALTHY     OK                xds-grpc
    unix://./var/run/secrets/workload-spiffe-uds/socket     HEALTHY     OK                sds-grpc
    ```
1. Label second application namespace to be matched by defined `discoverySelectors` and enable sidecar injection:
    ```bash
    oc label ns app-ns-2 istio-discovery=enabled
    ```
1. Verify that after labeling second namespace it also appears on the list of discovered endpoints:
    ```bash
    istioctl pc endpoint deploy/sleep -n app-ns-1
    ENDPOINT                                                STATUS      OUTLIER CHECK     CLUSTER
    10.128.2.197:15010                                      HEALTHY     OK                outbound|15010||istiod.istio-system.svc.cluster.local
    10.128.2.197:15012                                      HEALTHY     OK                outbound|15012||istiod.istio-system.svc.cluster.local
    10.128.2.197:15014                                      HEALTHY     OK                outbound|15014||istiod.istio-system.svc.cluster.local
    10.128.2.197:15017                                      HEALTHY     OK                outbound|443||istiod.istio-system.svc.cluster.local
    10.131.0.32:80                                          HEALTHY     OK                outbound|80||sleep.app-ns-1.svc.cluster.local
    10.131.0.33:80                                          HEALTHY     OK                outbound|80||sleep.app-ns-2.svc.cluster.local
    127.0.0.1:15000                                         HEALTHY     OK                prometheus_stats
    127.0.0.1:15020                                         HEALTHY     OK                agent
    unix://./etc/istio/proxy/XDS                            HEALTHY     OK                xds-grpc
    unix://./var/run/secrets/workload-spiffe-uds/socket     HEALTHY     OK                sds-grpc
    ```

See [Multiple Istio Control Planes in a Single Cluster](../multi-control-planes/README.md) for another example of `discoverySelectors` usage.

### Next Steps: Sidecar injection
As described earlier, in addition to the control plane discovering the namespaces to be included in the mesh, workloads must  be [injected with a sidecar proxy](../injection/README.md) to be included in the service mesh.
