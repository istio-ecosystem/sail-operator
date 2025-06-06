[Return to Project Root](../)
*Note*: To add new topics to this documentation, please follow the guidelines in the [guidelines](../docs/guidelines/guidelines.md) doc.

# Table of Contents

- [User Documentation](#user-documentation)
- [Concepts](#concepts)
  - [Istio resource](#istio-resource)
  - [Istiod in HA mode](general/istiod-ha.md#Running–Istiod-in-HA-mode)
    - [Setting up Istiod in HA mode: using fixed replicas](general/istiod-ha.md#Setting-up-Istiod-in-HA-mode:-increasing-replicaCount)
    - [Setting up Istiod in HA mode: using autoscaling](general/istiod-ha.md#Setting-up-Istiod-in-HA-mode:-using-autoscaling)
  - [IstioRevision resource](#istiorevision-resource)
  - [IstioRevisionTag resource](#istiorevisiontag-resource)
  - [IstioCNI resource](#istiocni-resource)
    - [Updating the IstioCNI resource](#updating-the-istiocni-resource)
  - [Resource Status](#resource-status)
    - [InUse Detection](#inuse-detection)
- [API Reference documentation](#api-reference-documentation)
- [Getting Started](general/getting-started.md#getting-started)
  - [Installation on OpenShift](general/getting-started.md#installation-on-openshift)
  - [Installation from Source](general/getting-started.md#installation-from-source)
  - [Migrating from Istio in-cluster Operator](general/getting-started.md#migrating-from-istio-in-cluster-operator)
    - [Setting environments variables for Istiod](general/getting-started.md#setting-environments-variables-for-istiod)
    - [Components field](general/getting-started.md#components-field)
    - [CNI lifecycle management](general/getting-started.md#cni-lifecycle-management)
    - [Converter Script to Migrate Istio in-cluster Operator Configuration to Sail Operator](general/getting-started.md#converter-script-to-migrate-istio-in-cluster-operator-configuration-to-sail-operator)
- [Creating and Configuring Gateways](common/create-and-configure-gateways.md#creating-and-configuring-gateways)
  - [Option 1: Istio Gateway Injection](common/create-and-configure-gateways.md#option-1-istio-gateway-injection)
  - [Option 2: Kubernetes Gateway API](common/create-and-configure-gateways.md#option-2-kubernetes-gateway-api)
- [Update Strategy](update-strategy/update-strategy.md#update-strategy)
  - [InPlace](update-strategy/update-strategy.md#inplace)
    - [Example using the InPlace strategy](update-strategy/update-strategy.md#example-using-the-inplace-strategy)
    - [Recommendations for InPlace strategy](update-strategy/update-strategy.md#recommendations-for-inplace-strategy)
  - [RevisionBased](update-strategy/update-strategy.md#revisionbased)
    - [Example using the RevisionBased strategy](update-strategy/update-strategy.md#example-using-the-revisionbased-strategy)
    - [Example using the RevisionBased strategy and an IstioRevisionTag](update-strategy/update-strategy.md#example-using-the-revisionbased-strategy-and-an-istiorevisiontag)
- [Multiple meshes on a single cluster](deployment-models/multiple-mesh.md#multiple-meshes-on-a-single-cluster)
  - [Prerequisites](deployment-models/multiple-mesh.md#prerequisites)
  - [Installation Steps](deployment-models/multiple-mesh.md#installation-steps)
    - [Deploying the control planes](deployment-models/multiple-mesh.md#deploying-the-control-planes)
    - [Deploying the applications](deployment-models/multiple-mesh.md#deploying-the-applications)
  - [Validation](deployment-models/multiple-mesh.md#validation)
    - [Checking application to control plane mapping](deployment-models/multiple-mesh.md#checking-application-to-control-plane-mapping)
    - [Checking application connectivity](deployment-models/multiple-mesh.md#checking-application-connectivity)
- [Multi-cluster](deployment-models/multicluster.md#multi-cluster)
  - [Prerequisites](deployment-models/multicluster.md#prerequisites)
  - [Common Setup](deployment-models/multicluster.md#common-setup)
  - [Multi-Primary](deployment-models/multicluster.md#multi-primary---multi-network)
  - [Primary-Remote](deployment-models/multicluster.md#primary-remote---multi-network)
  - [External Control Plane](deployment-models/multicluster.md#external-control-plane)
- [Dual-stack Support](dual-stack/dual-stack.md#dual-stack-support)
  - [Prerequisites](dual-stack/dual-stack.md#prerequisites-1)
  - [Installation Steps](dual-stack/dual-stack.md#installation-steps)
  - [Validation](dual-stack/dual-stack.md#validation)
- [Introduction to Istio Ambient mode](common/istio-ambient-mode.md#introduction-to-istio-ambient-mode)
  - [Component version](common/istio-ambient-mode.md#component-version)
  - [Concepts](common/istio-ambient-mode.md#concepts)
    - [ZTunnel resource](common/istio-ambient-mode.md#ztunnel-resource)
    - [API Reference documentation](common/istio-ambient-mode.md#api-reference-documentation)
  - [Core features](common/istio-ambient-mode.md#core-features)
  - [Getting Started](common/istio-ambient-mode.md#getting-started)
  - [Visualize the application using Kiali dashboard](common/istio-ambient-mode.mdvisualize-the-application-using-kiali-dashboard)
  - [Troubleshoot issues](common/istio-ambient-mode.md#troubleshoot-issues)
  - [Cleanup](common/istio-ambient-mode.md#cleanup)
- [Introduction to Istio Waypoint Proxy](common/istio-ambient-waypoint.md#introduction-to-istio-waypoint-proxy)
  - [Core features](common/istio-ambient-waypoint.md#core-features)
  - [Getting Started](common/istio-ambient-waypoint.md#getting-started)
  - [Layer 7 Features in Ambient Mode](common/istio-ambient-waypoint.md#layer-7-features-in-ambient-mode)
  - [Troubleshoot issues](common/istio-ambient-waypoint.md#troubleshoot-issues)
  - [Cleanup](common/istio-ambient-waypoint.md#cleanup)
- [Addons](addons/addons.md#addons)
  - [Deploy Prometheus and Jaeger addons](addons/addons.md#deploy-prometheus-and-jaeger-addons)
  - [Deploy Kiali addon](addons/addons.md#deploy-kiali-addon)
  - [Find the active revision of your Istio instance. In our case it is `test`.](addons/addons.md#find-the-active-revision-of-your-istio-instance-in-our-case-it-is-test)
  - [Deploy Gateway and Bookinfo](addons/addons.md#deploy-gateway-and-bookinfo)
  - [Generate traffic and visualize your mesh](addons/addons.md#generate-traffic-and-visualize-your-mesh)
- [Observability Integrations](addons/observability.md#observability-integrations)
  - [Deploy Prometheus and Jaeger addons](addons/observability.md#deploy-prometheus-and-jaeger-addons)
    - [Scraping metrics using the OpenShift monitoring stack](addons/observability.md#scraping-metrics-using-the-openshift-monitoring-stack)
    - [Configure tracing with OpenShift distributed tracing](addons/observability.md#configure-tracing-with-openshift-distributed-tracing)
    - [Integrating with Kiali](addons/observability.md#integrating-with-kiali)
      - [Integrating Kiali with the OpenShift monitoring stack](addons/observability.md#integrating-kiali-with-the-openshift-monitoring-stack)
      - [Integrating Kiali with OpenShift Distributed Tracing](addons/observability.md#integrating-kiali-with-openshift-distributed-tracing)
- [Uninstalling](general/getting-started.md#uninstalling)
  - [Deleting Istio](general/getting-started.md#deleting-istio)
  - [Deleting IstioCNI](general/getting-started.md#deleting-istiocni)
  - [Deleting the Sail Operator](general/getting-started.md#deleting-the-sail-operator)
  - [Deleting the istio-system and istio-cni Projects](general/getting-started.md#deleting-the-istio-system-and-istiocni-projects)
  - [Decide whether you want to delete the CRDs as well](general/getting-started.md#decide-whether-you-want-to-delete-the-crds-as-well)

# User Documentation
Sail Operator manages the lifecycle of your Istio control planes. Instead of creating a new configuration schema, Sail Operator APIs are built around Istio's helm chart APIs. All installation and configuration options that are exposed by Istio's helm charts are available through the Sail Operator CRDs' `values` fields.

Similar to using Istio's Helm charts, the final set of values used to render the charts is determined by a combination of user-provided values, default chart values, and values from selected profiles. 
These profiles can include the user-defined profile, the platform profile, and the compatibility version profile.
To view the final set of values, inspect the ConfigMap named `values` (or `values-<revision>`) in the namespace where the control plane is installed.

## Concepts

### Istio resource
The `Istio` resource is used to manage your Istio control planes. It is a cluster-wide resource, as the Istio control plane operates in and requires access to the entire cluster. To select a namespace to run the control plane pods in, you can use the `spec.namespace` field. Note that this field is immutable, though: in order to move a control plane to another namespace, you have to remove the Istio resource and recreate it with a different `spec.namespace`. You can access all helm chart options through the `values` field in the `spec`:

```yaml
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  namespace: istio-system
  updateStrategy:
    type: InPlace
  values:
    pilot:
      resources:
        requests:
          cpu: 100m
          memory: 1024Mi
```

Note: If you need a specific Istio version, you can explicitly set it using `spec.version`. If not specified, the Operator will install the latest supported version.

Istio uses a ConfigMap for its global configuration, called the MeshConfig. All of its settings are available through `spec.meshConfig`.

To support canary updates of the control plane, Sail Operator includes support for multiple Istio versions. You can select a version by setting the `version` field in the `spec` to the version you would like to install, prefixed with a `v`. You can then update to a new version just by changing this field. An `vX.Y-latest` alias can be used for the latest z/patch versions of each supported y/minor versions. As per the example above, `v1.26-latest` can be specified in the `version` field. By doing so, the operator will keep the istio version with the latest `z` version of the same `y` version. 

Sail Operator supports two different update strategies for your control planes: `InPlace` and `RevisionBased`. When using `InPlace`, the operator will immediately replace your existing control plane resources with the ones for the new version, whereas `RevisionBased` uses Istio's canary update mechanism by creating a second control plane to which you can migrate your workloads to complete the update.

After creation of an `Istio` resource, the Sail Operator will generate a revision name for it based on the updateStrategy that was chosen, and create a corresponding [`IstioRevision`](#istiorevision-resource).

### IstioRevision resource
The `IstioRevision` is the lowest-level API the Sail Operator provides, and it is usually not created by the user, but by the operator itself. It's schema closely resembles that of the `Istio` resource - but instead of representing the state of a control plane you want to be present in your cluster, it represents a *revision* of that control plane, which is an instance of Istio with a specific version and revision name, and its revision name can be used to add workloads or entire namespaces to the mesh, e.g. by using the `istio.io/rev=<REVISION_NAME>` label. It is also a cluster-wide resource.

You can think of the relationship between the `Istio` and `IstioRevision` resource as similar to the one between Kubernetes' `ReplicaSet` and `Pod`: a `ReplicaSet` can be created by users and results in the automatic creation of `Pods`, which will trigger the instantiation of your containers. Similarly, users create an `Istio` resource which instructs the operator to create a matching `IstioRevision`, which then in turn triggers the creation of the Istio control plane. To do that, the Sail Operator will copy all of your relevant configuration from the `Istio` resource to the `IstioRevision` resource.

### IstioRevisionTag resource
The `IstioRevisionTag` resource represents a *Stable Revision Tag*, which functions as an alias for Istio control plane revisions. With a stable tag `prod`, you can e.g. use the label `istio.io/rev=prod` to inject proxies into your workloads. When you perform an upgrade to a control plane with a new revision name, you can simply update your tag to point to the new revision, instead of having to re-label your workloads and namespaces. Also see the [Stable Revision Tags](https://istio.io/latest/docs/setup/upgrade/canary/#stable-revision-labels) section of Istio's [Canary Upgrades documentation](https://istio.io/latest/docs/setup/upgrade/canary/) for more details.

In Istio, stable revision tags are usually created using `istioctl`, but if you're using the Sail Operator, you can use the `IstioRevisionTag` resource, which comes with an additional feature: instead of just being able to reference an `IstioRevision`, you can also reference an `Istio` resource. When you now update your control plane and the underlying `IstioRevision` changes, the Sail Operator will update your revision tag for you. You only need to restart your deployments to re-inject the new proxies.

```yaml
apiVersion: sailoperator.io/v1
kind: IstioRevisionTag
metadata:
  name: default
spec:
  targetRef:
    kind: Istio   # can be either Istio or IstioRevision
    name: prod    # the name of the Istio/IstioRevision resource
```

As you can see in the YAML above, `IstioRevisionTag` really only has one field in its spec: `targetRef`. With this field, you can reference an `Istio` or `IstioRevision` resource. So after deploying this, you will be able to use both the `istio.io/rev=default` and also `istio-injection=enabled` labels to inject proxies into your workloads. The `istio-injection` label can only be used for revisions and revision tags named `default`, like the `IstioRevisionTag` in the above example.

### IstioCNI resource

The lifecycle of Istio's CNI plugin is managed separately when using Sail Operator. To install it, you can create an `IstioCNI` resource. The `IstioCNI` resource is a cluster-wide resource as it will install a `DaemonSet` that will be operating on all nodes of your cluster.

```yaml
apiVersion: sailoperator.io/v1
kind: IstioCNI
metadata:
  name: default
spec:
  namespace: istio-cni
  values:
    cni:
      cniConfDir: /etc/cni/net.d
      excludeNamespaces:
      - kube-system
```

> [!NOTE]
> If you need a specific Istio version, you can explicitly set it using `spec.version`. If not specified, the Operator will install the latest supported version.

#### Updating the IstioCNI resource
Updates for the `IstioCNI` resource are `Inplace` updates, this means that the `DaemonSet` will be updated with the new version of the CNI plugin once the resource is updated and the `istio-cni-node` pods are going to be replaced with the new version. 
To update the CNI plugin, just change the `version` field to the version you want to install. Just like the `Istio` resource, it also has a `values` field that exposes all of the options provided in the `istio-cni` chart:

1. Create the `IstioCNI` resource.
    ```bash { name=install-cni tag=cni-update}
    kubectl create ns istio-cni
    cat <<EOF | kubectl apply -f-
    apiVersion: sailoperator.io/v1
    kind: IstioCNI
    metadata:
      name: default
    spec:
      version: v1.24.2
      namespace: istio-cni
      values:
        cni:
          cniConfDir: /etc/cni/net.d
          excludeNamespaces:
          - kube-system
    EOF
    ```
```bash { name=validation-wait-cni tag=cni-update}
. scripts/prebuilt-func.sh
wait_cni_ready "istio-cni"
with_retries resource_version_equal "istiocni" "default" "v1.24.2"
```
2. Confirm the installation and version of the CNI plugin.
    ```console
    $ kubectl get istiocni -n istio-cni
    NAME      READY   STATUS    VERSION   AGE
    default   True    Healthy   v1.24.2   91m
    $ kubectl get pods -n istio-cni
    NAME                   READY   STATUS    RESTARTS   AGE
    istio-cni-node-hd9zf   1/1     Running   0          90m
    ```

```bash { name=print-cni tag=cni-update}
. scripts/prebuilt-func.sh
print_cni_info
```

3. Update the CNI plugin version.

    ```bash { name=update-cni tag=cni-update}
    kubectl patch istiocni default -n istio-cni --type='merge' -p '{"spec":{"version":"v1.24.3"}}'
    ```
```bash { name=validation-wait-cni tag=cni-update}
. scripts/prebuilt-func.sh
with_retries resource_version_equal "istiocni" "default" "v1.24.3"
wait_cni_ready "istio-cni"
```
4. Confirm the CNI plugin version was updated.

    ```console
    $ kubectl get istiocni -n istio-cni
    NAME      READY   STATUS    VERSION   AGE
    default   True    Healthy   v1.24.3   93m
    $ kubectl get pods -n istio-cni
    NAME                   READY   STATUS    RESTARTS   AGE
    istio-cni-node-jz4lg   1/1     Running   0          44s
    ```

```bash { name=print-cni tag=cni-update}
. scripts/prebuilt-func.sh
print_cni_info
```
> [!NOTE]
> The CNI plugin at version `1.x` is compatible with `Istio` at version `1.x-1`, `1.x` and `1.x+1`.

### Resource Status
All of the Sail Operator API resources have a `status` subresource that contains information about their current state in the Kubernetes cluster.

#### Conditions
All resources have a `Ready` condition which is set to `true` as soon as all child resource have been created and are deemed Ready by their respective controllers. To see additional conditions for each of the resources, check the [API reference documentation](https://github.com/istio-ecosystem/sail-operator/tree/main/docs/api-reference/sailoperator.io.md).

#### InUse Detection
The Sail Operator uses InUse detection to determine whether an object is referenced. This is currently present on all resources apart from `IstioCNI`. On the `Istio` resource, it is a counter as it only aggregates the `InUse` conditions on its child `IstioRevisions`.

|API               |Type        |Name|Description
|------------------|------------|----|-------------------------------------------
|Istio             |Counter     |Status.Revisions.InUse|Aggregates across all child `IstioRevisions`.
|IstioRevision     |Condition   |Status.Conditions[type="InUse']|Set to `true` if the `IstioRevision` is referenced by a namespace, workload or `IstioRevisionTag`.
|IstioRevisionTag  |Condition   |Status.Conditions[type="InUse']|Set to `true` if the `IstioRevisionTag` is referenced by a namespace or workload.

## API Reference documentation
The Sail Operator API reference documentation can be found [here](api-reference/sailoperator.io.md#api-reference).
