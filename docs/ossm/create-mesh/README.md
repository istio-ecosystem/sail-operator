# Scoping the service mesh with DiscoverySelectors

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

#### Using Discovery Selectors to Scope a Service Mesh
Assuming you know which namespaces to include as part of the service mesh, as a mesh administrator, you can configure `discoverySelectors` at installation time or post-installation by adding your desired discovery selectors to Istio’s MeshConfig resource. 

For example, you can configure Istio to discover only the namespaces that have the label `istio-discovery=enabled`.

##### Prerequisites
- The OpenShift Service Mesh operator has been installed
- An Istio CNI resource has been created

1. Add a label to the namespace containing the Istio control plane, for example, the `istio-system` system namespace:
    ```bash
    oc label namespace istio-system istio-discovery=enabled
    ```
1. Modify the `Istio` control plane resource to include a `discoverySelectors` section with the same label, for example:
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
    ```

1. Apply the Istio CR:
    ```bash
    oc apply -f istio.yaml
    ```
1. You then must ensure that all namespaces that will contain workloads that are to be part of the service mesh have both the `discoverySelector` label and, if desired, the appropriate Istio injection label. For example, for the `bookinfo` application, you can apply both labels as follows:
    ```bash
    oc label namespace bookinfo istio-discovery=enabled istio-injection=enabled
    ```
In addition to limiting the scope of a single service mesh, `discoverySelectors` also play a critical role in limiting the scope of control plane when [multiple Istio control planes are to be deployed within a single cluster](../multi-control-planes/README.md).

### Next Steps: Sidecar injection
As described earlier, in addition to the control plane discovering the namespaces to be included in the mesh, workloads must  be [injected with a sidecar proxy](../injection/README.md) to be included in the service mesh.
