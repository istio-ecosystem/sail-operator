[Return to Project Root](../)

# Table of Contents

- [User Documentation](#user-documentation)
- [Concepts](#concepts)
  - [Istio resource](#istio-resource)
  - [IstioRevision resource](#istiorevision-resource)
  - [IstioRevisionTag resource](#istiorevisiontag-resource)
  - [IstioCNI resource](#istiocni-resource)
  - [Resource Status](#resource-status)
    - [InUse Detection](#inuse-detection)
- [API Reference documentation](#api-reference-documentation)
- [Getting Started](#getting-started)
  - [Installation on OpenShift](#installation-on-openshift)
    - [Installing through the web console](#installing-through-the-web-console)
    - [Installing using the CLI](#installing-using-the-cli)
  - [Installation from Source](#installation-from-source)
- [Migrating from Istio in-cluster Operator](#migrating-from-istio-in-cluster-operator)
- [Gateways](#gateways)
- [Update Strategy](#update-strategy)
  - [InPlace](#inplace)
    - [Example using the InPlace strategy](#example-using-the-inplace-strategy)
  - [RevisionBased](#revisionbased)
    - [Example using the RevisionBased strategy](#example-using-the-revisionbased-strategy)
    - [Example using the RevisionBased strategy and an IstioRevisionTag](#example-using-the-revisionbased-strategy-and-an-istiorevisiontag)
- [Multiple meshes on a single cluster](#multiple-meshes-on-a-single-cluster)
  - [Prerequisites](#prerequisites)
  - [Installation Steps](#installation-steps)
    - [Deploying the control planes](#deploying-the-control-planes)
    - [Deploying the applications](#deploying-the-applications)
  - [Validation](#validation)
    - [Checking application to control plane mapping](#checking-application-to-control-plane-mapping)
    - [Checking application connectivity](#checking-application-connectivity)
- [Multi-cluster](#multi-cluster)
  - [Prerequisites](#prerequisites)
  - [Common Setup](#common-setup)
  - [Multi-Primary](#multi-primary---multi-network)
  - [Primary-Remote](#primary-remote---multi-network)
  - [External Control Plane](#external-control-plane)
- [Dual-stack Support](#dual-stack-support)
  - [Prerequisites](#prerequisites-1)
  - [Installation Steps](#installation-steps)
  - [Validation](#validation)
- [Ambient mode](common/istio-ambient-mode.md)
- [Addons](#addons)
  - [Deploy Prometheus and Jaeger addons](#deploy-prometheus-and-jaeger-addons)
  - [Deploy Kiali addon](#deploy-kiali-addon)
  - [Deploy Gateway and Bookinfo](#deploy-gateway-and-bookinfo)
  - [Generate traffic and visualize your mesh](#generate-traffic-and-visualize-your-mesh)
- [Observability Integrations](#observability-integrations)
  - [Scraping metrics using the OpenShift monitoring stack](#scraping-metrics-using-the-openshift-monitoring-stack)
  - [Configure Tracing with OpenShift distributed tracing](#configure-tracing-with-openshift-distributed-tracing)
  - [Integrating with Kiali](#integrating-with-kiali)
    - [Integrating Kiali with the OpenShift monitoring stack](#integrating-kiali-with-the-openshift-monitoring-stack)
    - [Integrating Kiali with OpenShift distributed tracing](#integrating-kiali-with-openshift-distributed-tracing)
- [Uninstalling](#uninstalling)
  - [Deleting Istio](#deleting-istio)
  - [Deleting IstioCNI](#deleting-istiocni)
  - [Deleting the Sail Operator](#deleting-the-sail-operator)
  - [Deleting the istio-system and istio-cni Projects](#deleting-the-istio-system-and-istiocni-projects)
  - [Decide whether you want to delete the CRDs as well](#decide-whether-you-want-to-delete-the-crds-as-well)


# User Documentation
Sail Operator manages the lifecycle of your Istio control planes. Instead of creating a new configuration schema, Sail Operator APIs are built around Istio's helm chart APIs. All installation and configuration options that are exposed by Istio's helm charts are available through the Sail Operator CRDs' `values` fields.

## Concepts

### Istio resource
The `Istio` resource is used to manage your Istio control planes. It is a cluster-wide resource, as the Istio control plane operates in and requires access to the entire cluster. To select a namespace to run the control plane pods in, you can use the `spec.namespace` field. Note that this field is immutable, though: in order to move a control plane to another namespace, you have to remove the Istio resource and recreate it with a different `spec.namespace`. You can access all helm chart options through the `values` field in the `spec`:

```yaml
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  version: v1.23.2
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

Istio uses a ConfigMap for its global configuration, called the MeshConfig. All of its settings are available through `spec.meshConfig`.

To support canary updates of the control plane, Sail Operator includes support for multiple Istio versions. You can select a version by setting the `version` field in the `spec` to the version you would like to install, prefixed with a `v`. You can then update to a new version just by changing this field.

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
The lifecycle of Istio's CNI plugin is managed separately when using Sail Operator. To install it, you can create an `IstioCNI` resource. The `IstioCNI` resource is a cluster-wide resource as it will install a `DaemonSet` that will be operating on all nodes of your cluster. You can select a version by setting the `spec.version` field, as you can see in the sample below. To update the CNI plugin, just change the `version` field to the version you want to install. Just like the `Istio` resource, it also has a `values` field that exposes all of the options provided in the `istio-cni` chart:

```yaml
apiVersion: sailoperator.io/v1
kind: IstioCNI
metadata:
  name: default
spec:
  version: v1.23.2
  namespace: istio-cni
  values:
    cni:
      cniConfDir: /etc/cni/net.d
      excludeNamespaces:
      - kube-system
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
The Sail Operator API reference documentation can be found [here](https://github.com/istio-ecosystem/sail-operator/tree/main/docs/api-reference/sailoperator.io.md).

## Getting Started

### Installation on OpenShift

#### Installing through the web console

1. In the OpenShift Console, navigate to the OperatorHub by clicking **Operator** -> **Operator Hub** in the left side-pane.

1. Search for "sail".

1. Locate the Sail Operator, and click to select it.

1. When the prompt that discusses the community operator appears, click **Continue**, then click **Install**.

1. Use the default installation settings presented, and click **Install** to continue.

1. Click **Operators** -> **Installed Operators** to verify that the Sail Operator 
is installed. `Succeeded` should appear in the **Status** column.

#### Installing using the CLI

*Prerequisites*

* You have access to the cluster as a user with the `cluster-admin` cluster role.

*Steps*

1. Create the `openshift-operators` namespace (if it does not already exist).

    ```bash
    $ kubectl create namespace openshift-operators
    ```

1. Create the `Subscription` object with the desired `spec.channel`.

   ```bash
   kubectl apply -f - <<EOF
       apiVersion: operators.coreos.com/v1alpha1
       kind: Subscription
       metadata:
         name: sailoperator
         namespace: openshift-operators
       spec:
         channel: "0.1-nightly"
         installPlanApproval: Automatic
         name: sailoperator
         source: community-operators
         sourceNamespace: openshift-marketplace
   EOF
   ```

1. Verify that the installation succeeded by inspecting the CSV status.

    ```bash
    $ kubectl get csv -n openshift-operators
    NAME                                     DISPLAY         VERSION                    REPLACES                                 PHASE
    sailoperator.v0.1.0-nightly-2024-06-25   Sail Operator   0.1.0-nightly-2024-06-25   sailoperator.v0.1.0-nightly-2024-06-21   Succeeded
    ```

    `Succeeded` should appear in the sailoperator CSV `PHASE` column.

### Installation from Source

If you're not using OpenShift or simply want to install from source, follow the [instructions in the Contributor Documentation](../README.md#deploying-the-operator).

## Migrating from Istio in-cluster Operator

If you're planning to migrate from the [now-deprecated Istio in-cluster operator](https://istio.io/latest/blog/2024/in-cluster-operator-deprecation-announcement/) to the Sail Operator, you will have to make some adjustments to your Kubernetes Resources. While direct usage of the IstioOperator resource is not possible with the Sail Operator, you can very easily transfer all your settings to the respective Sail Operator APIs. As shown in the [Concepts](#concepts) section, every API resource has a `spec.values` field which accepts the same input as the `IstioOperator`'s `spec.values` field. Also, the [Istio resource](#istio-resource) provides a `spec.meshConfig` field, just like IstioOperator does.

Another important distinction between the two operators is that Sail Operator can manage and install different versions of Istio and its components, whereas the in-cluster operator always installs the version of Istio that it was released with. This makes managing control plane upgrades much easier, as the operator update is disconnected from the control plane update.

So for a simple Istio deployment, the transition will be very easy:

```yaml
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
spec:
  meshConfig:
    accessLogFile: /dev/stdout
  values:
    pilot:
      traceSampling: 0.1
```

becomes

```yaml
apiVersion: sailoperator.io/v1
kind: Istio
spec:
  meshConfig:
    accessLogFile: /dev/stdout
  values:
    pilot:
      traceSampling: 0.1
  version: v1.23.2
```

Note that the only field that was added is the `spec.version` field. There are a few situations however where the APIs are different and require different approaches to achieve the same outcome.

### Environment variables

In Sail Operator, all `.env` fields are `map[string]string` instead of `struct{}`, so you have to be careful with values such as `true` or `false` - they need to be in quotes in order to pass the type checks!

That means the following YAML

```yaml
apiVersion: install.istio.io/v1alpha1
kind: IstioOperator
metadata:
  name: default
spec:
  values:
    global:
      istiod:
        enableAnalysis: true
    pilot:
      env:
        PILOT_ENABLE_STATUS: true
```

becomes

```yaml
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  values:
    global:
      istiod:
        enableAnalysis: true
    pilot:
      env:
        PILOT_ENABLE_STATUS: "true"
  version: v1.23.0
  namespace: istio-system
```

Note the quotes around the value of `spec.values.pilot.env.PILOT_ENABLE_STATUS`. Without them, Kubernetes would reject the YAML as it expects a value of type `string` but receives a `boolean`.

### components field

Sail Operator's Istio resource does not have a `spec.components` field. Instead, you can enable and disable components directly by setting `spec.values.<component>.enabled: true/false`. Other functionality exposed through `spec.components` like the k8s overlays is not currently available.

### CNI

The CNI plugin's lifecycle is managed separately from the control plane. You will have to create a [IstioCNI resource](#istiocni-resource) to use CNI.

## Gateways

[Gateways in Istio](https://istio.io/latest/docs/concepts/traffic-management/#gateways) are used to manage inbound and outbound traffic for the mesh. The Sail Operator does not deploy or manage Gateways. You can deploy a gateway either through [gateway-api](https://istio.io/latest/docs/tasks/traffic-management/ingress/gateway-api/) or through [gateway injection](https://istio.io/latest/docs/setup/additional-setup/gateway/#deploying-a-gateway). As you are following the gateway installation instructions, skip the step to install Istio since this is handled by the Sail Operator.

**Note:** The `IstioOperator` / `istioctl` example is separate from the Sail Operator. Setting `spec.components` or `spec.values.gateways` on your Sail Operator `Istio` resource **will not work**.

For examples installing Gateways on OpenShift, see the [Gateways](common/create-and-configure-gateways.md) page.

## Update Strategy

The Sail Operator supports two update strategies to update the version of the Istio control plane: `InPlace` and `RevisionBased`. The default strategy is `InPlace`.

### InPlace
When the `InPlace` strategy is used, the existing Istio control plane is replaced with a new version. The workload sidecars immediately connect to the new control plane. The workloads therefore don't need to be moved from one control plane instance to another.

#### Example using the InPlace strategy

Prerequisites:
* Sail Operator is installed.
* `istioctl` is [installed](common/install-istioctl-tool.md).

Steps:
1. Create the `istio-system` namespace.

    ```bash
    kubectl create namespace istio-system
    ```

2. Create the `Istio` resource.

    ```bash
    cat <<EOF | kubectl apply -f-
    apiVersion: sailoperator.io/v1
    kind: Istio
    metadata:
      name: default
    spec:
      namespace: istio-system
      updateStrategy:
        type: InPlace
      version: v1.22.5
    EOF
    ```

3. Confirm the installation and version of the control plane.

    ```console
    $ kubectl get istio -n istio-system
    NAME      REVISIONS   READY   IN USE   ACTIVE REVISION   STATUS    VERSION   AGE
    default   1           1       0        default           Healthy   v1.22.5   23s
    ```
    Note: `IN USE` field shows as 0, as `Istio` has just been installed and there are no workloads using it.

4. Create namespace `bookinfo` and deploy bookinfo application.

    ```bash
    kubectl create namespace bookinfo
    kubectl label namespace bookinfo istio-injection=enabled
    kubectl apply -n bookinfo -f https://raw.githubusercontent.com/istio/istio/release-1.22/samples/bookinfo/platform/kube/bookinfo.yaml
    ```
    Note: if the `Istio` resource name is other than `default`, you need to set the `istio.io/rev` label to the name of the `Istio` resource instead of adding the `istio-injection=enabled` label.

5. Review the `Istio` resource after application deployment.

   ```console
   $ kubectl get istio -n istio-system
   NAME      REVISIONS   READY   IN USE   ACTIVE REVISION   STATUS    VERSION   AGE
   default   1           1       1        default           Healthy   v1.22.5   115s
   ```
   Note: `IN USE` field shows as 1, as the namespace label and the injected proxies reference the IstioRevision.

6. Perform the update of the control plane by changing the version in the Istio resource.

    ```bash
    kubectl patch istio default -n istio-system --type='merge' -p '{"spec":{"version":"v1.23.2"}}'
    ```

7. Confirm the `Istio` resource version was updated.

    ```console
    $ kubectl get istio -n istio-system
    NAME      REVISIONS   READY   IN USE   ACTIVE REVISION   STATUS    VERSION   AGE
    default   1           1       1        default           Healthy   v1.23.2   4m50s
    ```

8. Delete `bookinfo` pods to trigger sidecar injection with the new version.

    ```bash
    kubectl rollout restart deployment -n bookinfo
    ```

9. Confirm that the new version is used in the sidecar.

    ```bash
    istioctl proxy-status 
    ```
    The column `VERSION` should match the new control plane version.

### RevisionBased
When the `RevisionBased` strategy is used, a new Istio control plane instance is created for every change to the `Istio.spec.version` field. The old control plane remains in place until all workloads have been moved to the new control plane instance. This needs to be done by the user by updating the namespace label and restarting all the pods. The old control plane will be deleted after the grace period specified in the `Istio` resource field `spec.updateStrategy.inactiveRevisionDeletionGracePeriodSeconds`.

#### Example using the RevisionBased strategy

Prerequisites:
* Sail Operator is installed.
* `istioctl` is [installed](common/install-istioctl-tool.md).

Steps:

1. Create the `istio-system` namespace.

    ```bash
    kubectl create namespace istio-system
    ```

2. Create the `Istio` resource.

    ```bash
    cat <<EOF | kubectl apply -f-
    apiVersion: sailoperator.io/v1
    kind: Istio
    metadata:
      name: default
    spec:
      namespace: istio-system
      updateStrategy:
        type: RevisionBased
        inactiveRevisionDeletionGracePeriodSeconds: 30
      version: v1.22.5
    EOF
    ```

3. Confirm the control plane is installed and is using the desired version.

    ```console
    $ kubectl get istio -n istio-system
    NAME      REVISIONS   READY   IN USE   ACTIVE REVISION   STATUS    VERSION   AGE
    default   1           1       0        default-v1-22-5   Healthy   v1.22.5   52s
    ```
    Note: `IN USE` field shows as 0, as the control plane has just been installed and there are no workloads using it.

4. Get the `IstioRevision` name.

    ```console
    $ kubectl get istiorevision -n istio-system
    NAME              TYPE    READY   STATUS    IN USE   VERSION   AGE
    default-v1-22-5   Local   True    Healthy   False    v1.22.5   3m4s
    ```
    Note: `IstioRevision` name is in the format `<Istio resource name>-<version>`.

5. Create `bookinfo` namespace and label it with the revision name.

    ```bash
    kubectl create namespace bookinfo
    kubectl label namespace bookinfo istio.io/rev=default-v1-22-5
    ```

6. Deploy bookinfo application.

    ```bash
    kubectl apply -n bookinfo -f https://raw.githubusercontent.com/istio/istio/release-1.22/samples/bookinfo/platform/kube/bookinfo.yaml
    ```

7. Review the `Istio` resource after application deployment.

    ```console
    $ kubectl get istio -n istio-system
    NAME      REVISIONS   READY   IN USE   ACTIVE REVISION   STATUS    VERSION   AGE
    default   1           1       1        default-v1-22-5   Healthy   v1.22.5   5m13s
    ```
    Note: `IN USE` field shows as 1, after application being deployed.

8. Confirm that the proxy version matches the control plane version.

    ```bash
    istioctl proxy-status 
    ```
    The column `VERSION` should match the control plane version.

9. Update the control plane to a new version.

    ```bash
    kubectl patch istio default -n istio-system --type='merge' -p '{"spec":{"version":"v1.23.2"}}'
    ```

10. Verify the `Istio` and `IstioRevision` resources. There will be a new revision created with the new version.

    ```console
    $ kubectl get istio
    NAME      REVISIONS   READY   IN USE   ACTIVE REVISION   STATUS    VERSION   AGE
    default   2           2       1        default-v1-23-2   Healthy   v1.23.2   9m23s

    $ kubectl get istiorevision
    NAME              TYPE    READY   STATUS    IN USE   VERSION   AGE
    default-v1-22-5   Local   True    Healthy   True     v1.22.5   10m
    default-v1-23-2   Local   True    Healthy   False    v1.23.2   66s
    ```

11. Confirm there are two control plane pods running, one for each revision.

    ```console
    $ kubectl get pods -n istio-system
    NAME                                      READY   STATUS    RESTARTS   AGE
    istiod-default-v1-22-5-c98fd9675-r7bfw    1/1     Running   0          10m
    istiod-default-v1-23-2-7495cdc7bf-v8t4g   1/1     Running   0          113s
    ```

12. Confirm the proxy sidecar version remains the same:

    ```bash
    istioctl proxy-status 
    ```
    The column `VERSION` should still match the old control plane version.

13. Change the label of the `bookinfo` namespace to use the new revision.

    ```bash
    kubectl label namespace bookinfo istio.io/rev=default-v1-23-2 --overwrite
    ```
    The existing workload sidecars will continue to run and will remain connected to the old control plane instance. They will not be replaced with a new version until the pods are deleted and recreated.

14. Restart all Deplyments in the `bookinfo` namespace.

    ```bash
    kubectl rollout restart deployment -n bookinfo
    ```

15. Confirm the new version is used in the sidecars.

    ```bash
    istioctl proxy-status 
    ```
    The column `VERSION` should match the updated control plane version.

16. Confirm the deletion of the old control plane and IstioRevision.

    ```console
    $ kubectl get pods -n istio-system
    NAME                                      READY   STATUS    RESTARTS   AGE
    istiod-default-v1-23-2-7495cdc7bf-v8t4g   1/1     Running   0          4m40s

    $ kubectl get istio
    NAME      REVISIONS   READY   IN USE   ACTIVE REVISION   STATUS    VERSION   AGE
    default   1           1       1        default-v1-23-2   Healthy   v1.23.2   5m

    $ kubectl get istiorevision
    NAME              TYPE    READY   STATUS    IN USE   VERSION   AGE
    default-v1-23-2   Local   True    Healthy   True     v1.23.2   5m31s
    ```
    The old `IstioRevision` resource and the old control plane will be deleted when the grace period specified in the `Istio` resource field `spec.updateStrategy.inactiveRevisionDeletionGracePeriodSeconds` expires.

#### Example using the RevisionBased strategy and an IstioRevisionTag

Prerequisites:
* Sail Operator is installed.
* `istioctl` is [installed](common/install-istioctl-tool.md).

Steps:

1. Create the `istio-system` namespace.

    ```bash
    kubectl create namespace istio-system
    ```

2. Create the `Istio` and `IstioRevisionTag` resources.

    ```bash
    cat <<EOF | kubectl apply -f-
    apiVersion: sailoperator.io/v1
    kind: Istio
    metadata:
      name: default
    spec:
      namespace: istio-system
      updateStrategy:
        type: RevisionBased
        inactiveRevisionDeletionGracePeriodSeconds: 30
      version: v1.23.3
    ---
    apiVersion: sailoperator.io/v1
    kind: IstioRevisionTag
    metadata:
      name: default
    spec:
      targetRef:
        kind: Istio
        name: default
    EOF
    ```

3. Confirm the control plane is installed and is using the desired version.

    ```console
    $ kubectl get istio
    NAME      REVISIONS   READY   IN USE   ACTIVE REVISION   STATUS    VERSION   AGE
    default   1           1       1        default-v1-23-3   Healthy   v1.22.5   52s
    ```
    Note: `IN USE` field shows as 1, even though no workloads are using the control plane. This is because the `IstioRevisionTag` is referencing it.

4. Inspect the `IstioRevisionTag`.

    ```console
    $ kubectl get istiorevisiontags
    NAME      STATUS                    IN USE   REVISION          AGE
    default   NotReferencedByAnything   False    default-v1-23-3   52s
    ```

5. Create `bookinfo` namespace and label it to mark it for injection.

    ```bash
    kubectl create namespace bookinfo
    kubectl label namespace bookinfo istio-injection=enabled
    ```

6. Deploy bookinfo application.

    ```bash
    kubectl apply -n bookinfo -f https://raw.githubusercontent.com/istio/istio/release-1.23/samples/bookinfo/platform/kube/bookinfo.yaml
    ```

7. Review the `IstioRevisionTag` resource after application deployment.

    ```console
    $ kubectl get istiorevisiontag
    NAME      STATUS    IN USE   REVISION          AGE
    default   Healthy   True     default-v1-23-3   2m46s
    ```
    Note: `IN USE` field shows 'True', as the tag is now referenced by both active workloads and the bookinfo namespace.

8. Confirm that the proxy version matches the control plane version.

    ```bash
    istioctl proxy-status 
    ```
    The column `VERSION` should match the control plane version.

9. Update the control plane to a new version.

    ```bash
    kubectl patch istio default -n istio-system --type='merge' -p '{"spec":{"version":"v1.24.1"}}'
    ```

10. Verify the `Istio`, `IstioRevision` and `IstioRevisionTag` resources. There will be a new revision created with the new version.

    ```console
    $ kubectl get istio
    NAME      REVISIONS   READY   IN USE   ACTIVE REVISION   STATUS    VERSION   AGE
    default   2           2       1        default-v1-24-1   Healthy   v1.24.1   9m23s

    $ kubectl get istiorevision
    NAME              TYPE    READY   STATUS    IN USE   VERSION   AGE
    default-v1-23-3   Local   True    Healthy   True     v1.23.3   10m
    default-v1-24-1   Local   True    Healthy   True    v1.24.1   66s

    $ kubectl get istiorevisiontag
    NAME      STATUS    IN USE   REVISION          AGE
    default   Healthy   True     default-v1-24-1   10m44s
    ```
    Now, both our IstioRevisions and the IstioRevisionTag are considered in use. The old revision default-v1-23-3 because it is being used by proxies, the new revision default-v1-24-1 because it is referenced by the tag, and lastly the tag because it is referenced by the bookinfo namespace.

11. Confirm there are two control plane pods running, one for each revision.

    ```console
    $ kubectl get pods -n istio-system
    NAME                                      READY   STATUS    RESTARTS   AGE
    istiod-default-v1-23-3-c98fd9675-r7bfw    1/1     Running   0          10m
    istiod-default-v1-24-1-7495cdc7bf-v8t4g   1/1     Running   0          113s
    ```

12. Confirm the proxy sidecar version remains the same:

    ```bash
    istioctl proxy-status 
    ```
    The column `VERSION` should still match the old control plane version.

13. Restart all the Deployments in the `bookinfo` namespace.

    ```bash
    kubectl rollout restart deployment -n bookinfo
    ```

14. Confirm the new version is used in the sidecars. Note that it might take a few seconds for the restarts to complete.

    ```bash
    istioctl proxy-status 
    ```
    The column `VERSION` should match the updated control plane version.

16. Confirm the deletion of the old control plane and IstioRevision.

    ```console
    $ kubectl get pods -n istio-system
    NAME                                      READY   STATUS    RESTARTS   AGE
    istiod-default-v1-24-1-7495cdc7bf-v8t4g   1/1     Running   0          4m40s

    $ kubectl get istio -n istio-system
    NAME      REVISIONS   READY   IN USE   ACTIVE REVISION   STATUS    VERSION   AGE
    default   1           1       1        default-v1-24-1   Healthy   v1.24.1   5m

    $ kubectl get istiorevision -n istio-system
    NAME              TYPE    READY   STATUS    IN USE   VERSION   AGE
    default-v1-24-1   Local   True    Healthy   True     v1.24.1   5m31s
    ```
    The old `IstioRevision` resource and the old control plane will be deleted when the grace period specified in the `Istio` resource field `spec.updateStrategy.inactiveRevisionDeletionGracePeriodSeconds` expires.

## Multiple meshes on a single cluster

The Sail Operator supports running multiple meshes on a single cluster and associating each workload with a specific mesh. 
Each mesh is managed by a separate control plane.

Applications are installed in multiple namespaces, and each namespace is associated with one of the control planes through its labels.
The `istio.io/rev` label determines which control plane injects the sidecar proxy into the application pods.
Additional namespace labels determine whether the control plane discovers and manages the resources in the namespace. 
A control plane will discover and manage only those namespaces that match the discovery selectors configured on the control plane.
Additionally, discovery selectors determine which control plane creates the `istio-ca-root-cert` ConfigMap in which namespace.

Currently, discovery selectors in multiple control planes must be configured so that they don't overlap (i.e. the discovery selectors of two control planes don't match the same namespace).
Each control plane must be deployed in a separate Kubernetes namespace.

This guide explains how to set up two meshes: `mesh1` and `mesh2` in namespaces `istio-system1` and `istio-system2`, respectively, and three application namespaces: `app1`, `app2a`, and `app2b`.
Mesh 1 will manage namespace `app1`, and Mesh 2 will manage namespaces `app2a` and `app2b`.
Because each mesh will use its own root certificate authority and configured to use a peer authentication policy with the `STRICT` mTLS mode, the communication between the two meshes will not be allowed. 

### Prerequisites

- Install [istioctl](common/install-istioctl-tool.md).
- Kubernetes 1.23 cluster.
- kubeconfig file with a context for the Kubernetes cluster.
- Install the Sail Operator and the Sail CRDs to the cluster.

### Installation Steps

#### Deploying the control planes

1. Create the system namespace `istio-system1` and deploy the `mesh1` control plane in it.
   ```sh
   $ kubectl create namespace istio-system1
   $ kubectl label ns istio-system1 mesh=mesh1
   $ kubectl apply -f - <<EOF
   apiVersion: sailoperator.io/v1
   kind: Istio
   metadata:
     name: mesh1
   spec:
     namespace: istio-system1
     version: v1.24.0
     values:
       meshConfig:
         discoverySelectors:
         - matchLabels:
             mesh: mesh1
   EOF
   ```
   
2. Create the system namespace `istio-system2` and deploy the `mesh2` control plane in it.
   ```sh
   $ kubectl create namespace istio-system2
   $ kubectl label ns istio-system2 mesh=mesh2
   $ kubectl apply -f - <<EOF
   apiVersion: sailoperator.io/v1
   kind: Istio
   metadata:
     name: mesh2
   spec:
     namespace: istio-system2
     version: v1.24.0
     values:
       meshConfig:
         discoverySelectors:
         - matchLabels:
             mesh: mesh2
   EOF
   ```

3. Create a peer authentication policy that only allows mTLS communication within each mesh.
   ```sh
   $ kubectl apply -f - <<EOF
   apiVersion: security.istio.io/v1
   kind: PeerAuthentication
   metadata:
     name: default
     namespace: istio-system1
   spec:
     mtls:
       mode: STRICT
   EOF
   
   $ kubectl apply -f - <<EOF
   apiVersion: security.istio.io/v1
   kind: PeerAuthentication
   metadata:
     name: default
     namespace: istio-system2
   spec:
     mtls:
       mode: STRICT
   EOF
   ```  

#### Verifying the control planes

1. Check the labels on the control plane namespaces:
   ```sh
   $ kubectl get ns -l mesh -L mesh
   NAME            STATUS   AGE    MESH
   istio-system1   Active   106s   mesh1
   istio-system2   Active   105s   mesh2
   ```

2. Check the control planes are `Healthy`:
   ```sh
   $ kubectl get istios
   NAME    REVISIONS   READY   IN USE   ACTIVE REVISION   STATUS    VERSION   AGE
   mesh1   1           1       0        mesh1             Healthy   v1.24.0   84s
   mesh2   1           1       0        mesh2             Healthy   v1.24.0   77s
   ```

3. Confirm that the validation and mutation webhook configurations exist for both meshes:
   ```sh
   $ kubectl get validatingwebhookconfigurations
   NAME                                  WEBHOOKS   AGE
   istio-validator-mesh1-istio-system1   1          2m45s
   istio-validator-mesh2-istio-system2   1          2m38s

   $ kubectl get mutatingwebhookconfigurations
   NAME                                         WEBHOOKS   AGE
   istio-sidecar-injector-mesh1-istio-system1   2          5m55s
   istio-sidecar-injector-mesh2-istio-system2   2          5m48s
   ```

#### Deploying the applications

1. Create three application namespaces:
   ```sh
   $ kubectl create ns app1 
   $ kubectl create ns app2a 
   $ kubectl create ns app2b
   ```

2. Label each namespace to enable discovery by the corresponding control plane:
   ```sh
   $ kubectl label ns app1 mesh=mesh1
   $ kubectl label ns app2a mesh=mesh2
   $ kubectl label ns app2b mesh=mesh2
   ```

3. Label each namespace to enable injection by the corresponding control plane:
   ```sh
   $ kubectl label ns app1 istio.io/rev=mesh1
   $ kubectl label ns app2a istio.io/rev=mesh2
   $ kubectl label ns app2b istio.io/rev=mesh2
   ```

4. Deploy the `curl` and `httpbin` sample applications in each namespace:
   ```sh
   $ kubectl -n app1 apply -f https://raw.githubusercontent.com/istio/istio/refs/heads/master/samples/curl/curl.yaml 
   $ kubectl -n app1 apply -f https://raw.githubusercontent.com/istio/istio/refs/heads/master/samples/httpbin/httpbin.yaml 

   $ kubectl -n app2a apply -f https://raw.githubusercontent.com/istio/istio/refs/heads/master/samples/curl/curl.yaml 
   $ kubectl -n app2a apply -f https://raw.githubusercontent.com/istio/istio/refs/heads/master/samples/httpbin/httpbin.yaml 
   
   $ kubectl -n app2b apply -f https://raw.githubusercontent.com/istio/istio/refs/heads/master/samples/curl/curl.yaml 
   $ kubectl -n app2b apply -f https://raw.githubusercontent.com/istio/istio/refs/heads/master/samples/httpbin/httpbin.yaml 
   ```

5. Confirm that a sidecar has been injected into each of the application pods. The value `2/2` should be displayed in the `READY` column for each pod, as in the following example:
   ```sh
   $ kubectl get pods -n app1
   NAME                       READY   STATUS    RESTARTS   AGE
   curl-5b549b49b8-mg7nl      2/2     Running   0          102s
   httpbin-7b549f7859-h6hnk   2/2     Running   0          89s

   $ kubectl get pods -n app2a
   NAME                       READY   STATUS    RESTARTS   AGE
   curl-5b549b49b8-2hlvm      2/2     Running   0          2m3s
   httpbin-7b549f7859-bgblg   2/2     Running   0          110s

   $ kubectl get pods -n app2b
   NAME                       READY   STATUS    RESTARTS   AGE
   curl-5b549b49b8-xnzzk      2/2     Running   0          2m9s
   httpbin-7b549f7859-7k5gf   2/2     Running   0          118s
   ```

### Validation

#### Checking application to control plane mapping

Use the `istioctl ps` command to confirm that the application pods are connected to the correct control plane. 

The `curl` and `httpbin` pods in namespace `app1` should be connected to the control plane in namespace `istio-system1`, as shown in the following example (note the `.app1` suffix in the `NAME` column):

```sh
$ istioctl ps -i istio-system1
NAME                              CLUSTER        CDS                LDS                EDS                RDS                ECDS        ISTIOD                            VERSION
curl-5b549b49b8-mg7nl.app1        Kubernetes     SYNCED (4m40s)     SYNCED (4m40s)     SYNCED (4m31s)     SYNCED (4m40s)     IGNORED     istiod-mesh1-5df45b97dd-tf2wl     1.24.0
httpbin-7b549f7859-h6hnk.app1     Kubernetes     SYNCED (4m31s)     SYNCED (4m31s)     SYNCED (4m31s)     SYNCED (4m31s)     IGNORED     istiod-mesh1-5df45b97dd-tf2wl     1.24.0
```

The pods in namespaces `app2a` and `app2b` should be connected to the control plane in namespace `istio-system2`:

```sh
$ istioctl ps -i istio-system2
NAME                               CLUSTER        CDS                LDS                EDS                RDS                ECDS        ISTIOD                            VERSION
curl-5b549b49b8-2hlvm.app2a        Kubernetes     SYNCED (4m37s)     SYNCED (4m37s)     SYNCED (4m31s)     SYNCED (4m37s)     IGNORED     istiod-mesh2-59f6b874fb-mzxqw     1.24.0
curl-5b549b49b8-xnzzk.app2b        Kubernetes     SYNCED (4m37s)     SYNCED (4m37s)     SYNCED (4m31s)     SYNCED (4m37s)     IGNORED     istiod-mesh2-59f6b874fb-mzxqw     1.24.0
httpbin-7b549f7859-7k5gf.app2b     Kubernetes     SYNCED (4m31s)     SYNCED (4m31s)     SYNCED (4m31s)     SYNCED (4m31s)     IGNORED     istiod-mesh2-59f6b874fb-mzxqw     1.24.0
httpbin-7b549f7859-bgblg.app2a     Kubernetes     SYNCED (4m32s)     SYNCED (4m32s)     SYNCED (4m31s)     SYNCED (4m32s)     IGNORED     istiod-mesh2-59f6b874fb-mzxqw     1.24.0
```

#### Checking application connectivity

As both meshes are configured to use the `STRICT` mTLS peer authentication mode, the applications in namespace `app1` should not be able to communicate with the applications in namespaces `app2a` and `app2b`, and vice versa.
To test whether the `curl` pod in namespace `app2a` can connect to the `httpbin` service in namespace `app1`, run the following commands:

```sh
$ kubectl -n app2a exec deploy/curl -c curl -- curl -sIL http://httpbin.app1:8000
HTTP/1.1 503 Service Unavailable
content-length: 95
content-type: text/plain
date: Fri, 29 Nov 2024 08:58:28 GMT
server: envoy
```

As expected, the response indicates that the connection was not successful. 
In contrast, the same pod should be able to connect to the `httpbin` service in namespace `app2b`, because they are part of the same mesh:

```sh
$ kubectl -n app2a exec deploy/curl -c curl -- curl -sIL http://httpbin.app2b:8000
HTTP/1.1 200 OK
access-control-allow-credentials: true
access-control-allow-origin: *
content-security-policy: default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' camo.githubusercontent.com
content-type: text/html; charset=utf-8
date: Fri, 29 Nov 2024 08:57:52 GMT
x-envoy-upstream-service-time: 0
server: envoy
transfer-encoding: chunked
```

### Cleanup

To clean up the resources created in this guide, delete the `Istio` resources and the namespaces:

```sh
$ kubectl delete istio mesh1 mesh2
$ kubectl delete ns istio-system1 istio-system2 app1 app2a app2b
```


## Multi-cluster

You can use the Sail Operator and the Sail CRDs to manage a multi-cluster Istio deployment. The following instructions are adapted from the [Istio multi-cluster documentation](https://istio.io/latest/docs/setup/install/multicluster/) to demonstrate how you can setup the various deployment models with Sail. Please familiarize yourself with the different [deployment models](https://istio.io/latest/docs/ops/deployment/deployment-models/) before starting.

### Prerequisites

- Install [istioctl](common/install-istioctl-tool.md).
- Two kubernetes clusters with external lb support. (If using kind, `cloud-provider-kind` is running in the background)
- kubeconfig file with a context for each cluster.
- Install the Sail Operator and the Sail CRDs to every cluster.

### Common Setup

These steps are common to every multi-cluster deployment and should be completed *after* meeting the prerequisites but *before* starting on a specific deployment model.

1. Setup environment variables.

    ```sh
    export CTX_CLUSTER1=<cluster1-ctx>
    export CTX_CLUSTER2=<cluster2-ctx>
    export ISTIO_VERSION=1.23.2
    ```

2. Create `istio-system` namespace on each cluster.

    ```sh
    kubectl get ns istio-system --context "${CTX_CLUSTER1}" || kubectl create namespace istio-system --context "${CTX_CLUSTER1}"
    kubectl get ns istio-system --context "${CTX_CLUSTER2}" || kubectl create namespace istio-system --context "${CTX_CLUSTER2}"
    ```

4. Create a shared root certificate.

    If you have [established trust](https://istio.io/latest/docs/setup/install/multicluster/before-you-begin/#configure-trust) between your clusters already you can skip this and the following steps.

    ```sh
    openssl genrsa -out root-key.pem 4096
    cat <<EOF > root-ca.conf
    [ req ]
    encrypt_key = no
    prompt = no
    utf8 = yes
    default_md = sha256
    default_bits = 4096
    req_extensions = req_ext
    x509_extensions = req_ext
    distinguished_name = req_dn
    [ req_ext ]
    subjectKeyIdentifier = hash
    basicConstraints = critical, CA:true
    keyUsage = critical, digitalSignature, nonRepudiation, keyEncipherment, keyCertSign
    [ req_dn ]
    O = Istio
    CN = Root CA
    EOF

    openssl req -sha256 -new -key root-key.pem \
      -config root-ca.conf \
      -out root-cert.csr

    openssl x509 -req -sha256 -days 3650 \
      -signkey root-key.pem \
      -extensions req_ext -extfile root-ca.conf \
      -in root-cert.csr \
      -out root-cert.pem
    ```
5. Create intermediate certiciates.

    ```sh
    for cluster in west east; do
      mkdir $cluster

      openssl genrsa -out ${cluster}/ca-key.pem 4096
      cat <<EOF > ${cluster}/intermediate.conf
    [ req ]
    encrypt_key = no
    prompt = no
    utf8 = yes
    default_md = sha256
    default_bits = 4096
    req_extensions = req_ext
    x509_extensions = req_ext
    distinguished_name = req_dn
    [ req_ext ]
    subjectKeyIdentifier = hash
    basicConstraints = critical, CA:true, pathlen:0
    keyUsage = critical, digitalSignature, nonRepudiation, keyEncipherment, keyCertSign
    subjectAltName=@san
    [ san ]
    DNS.1 = istiod.istio-system.svc
    [ req_dn ]
    O = Istio
    CN = Intermediate CA
    L = $cluster
    EOF

      openssl req -new -config ${cluster}/intermediate.conf \
        -key ${cluster}/ca-key.pem \
        -out ${cluster}/cluster-ca.csr

      openssl x509 -req -sha256 -days 3650 \
        -CA root-cert.pem \
        -CAkey root-key.pem -CAcreateserial \
        -extensions req_ext -extfile ${cluster}/intermediate.conf \
        -in ${cluster}/cluster-ca.csr \
        -out ${cluster}/ca-cert.pem

      cat ${cluster}/ca-cert.pem root-cert.pem \
        > ${cluster}/cert-chain.pem
      cp root-cert.pem ${cluster}
    done
    ```

6. Push the intermediate CAs to each cluster.
    ```sh
    kubectl --context "${CTX_CLUSTER1}" label namespace istio-system topology.istio.io/network=network1
    kubectl get secret -n istio-system --context "${CTX_CLUSTER1}" cacerts || kubectl create secret generic cacerts -n istio-system --context "${CTX_CLUSTER1}" \
      --from-file=east/ca-cert.pem \
      --from-file=east/ca-key.pem \
      --from-file=east/root-cert.pem \
      --from-file=east/cert-chain.pem
    kubectl --context "${CTX_CLUSTER2}" label namespace istio-system topology.istio.io/network=network2
    kubectl get secret -n istio-system --context "${CTX_CLUSTER2}" cacerts || kubectl create secret generic cacerts -n istio-system --context "${CTX_CLUSTER2}" \
      --from-file=west/ca-cert.pem \
      --from-file=west/ca-key.pem \
      --from-file=west/root-cert.pem \
      --from-file=west/cert-chain.pem
    ```

### Multi-Primary - Multi-Network

These instructions install a [multi-primary/multi-network](https://istio.io/latest/docs/setup/install/multicluster/multi-primary_multi-network/) Istio deployment using the Sail Operator and Sail CRDs. **Before you begin**, ensure you complete the [common setup](#common-setup).

You can follow the steps below to install manually or you can run [this script](multicluster/setup-multi-primary.sh) which will setup a local environment for you with kind. Before running the setup script, you must install [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) and [cloud-provider-kind](https://kind.sigs.k8s.io/docs/user/loadbalancer/#installing-cloud-provider-kind) then ensure the `cloud-provider-kind` binary is running in the background.

These installation instructions are adapted from: https://istio.io/latest/docs/setup/install/multicluster/multi-primary_multi-network/. 

1. Create an `Istio` resource on `cluster1`.

    ```sh
    kubectl apply --context "${CTX_CLUSTER1}" -f - <<EOF
    apiVersion: sailoperator.io/v1
    kind: Istio
    metadata:
      name: default
    spec:
      version: v${ISTIO_VERSION}
      namespace: istio-system
      values:
        global:
          meshID: mesh1
          multiCluster:
            clusterName: cluster1
          network: network1
    EOF
    ```
  
2. Wait for the control plane to become ready.

    ```sh
    kubectl wait --context "${CTX_CLUSTER1}" --for=condition=Ready istios/default --timeout=3m
    ```

3. Create east-west gateway on `cluster1`.

    ```sh
    kubectl apply --context "${CTX_CLUSTER1}" -f https://raw.githubusercontent.com/istio-ecosystem/sail-operator/main/docs/multicluster/east-west-gateway-net1.yaml
    ```

4. Expose services on `cluster1`.

    ```sh
    kubectl --context "${CTX_CLUSTER1}" apply -n istio-system -f https://raw.githubusercontent.com/istio-ecosystem/sail-operator/main/docs/multicluster/expose-services.yaml
    ```

5. Create `Istio` resource on `cluster2`.

    ```sh
    kubectl apply --context "${CTX_CLUSTER2}" -f - <<EOF
    apiVersion: sailoperator.io/v1
    kind: Istio
    metadata:
      name: default
    spec:
      version: v${ISTIO_VERSION}
      namespace: istio-system
      values:
        global:
          meshID: mesh1
          multiCluster:
            clusterName: cluster2
          network: network2
    EOF
    ```

6. Wait for the control plane to become ready.

    ```sh
    kubectl wait --context "${CTX_CLUSTER2}" --for=jsonpath='{.status.revisions.ready}'=1 istios/default --timeout=3m
    ```

7. Create east-west gateway on `cluster2`.

    ```sh
    kubectl apply --context "${CTX_CLUSTER2}" -f https://raw.githubusercontent.com/istio-ecosystem/sail-operator/main/docs/multicluster/east-west-gateway-net2.yaml
    ```

8. Expose services on `cluster2`.

    ```sh
    kubectl --context "${CTX_CLUSTER2}" apply -n istio-system -f https://raw.githubusercontent.com/istio-ecosystem/sail-operator/main/docs/multicluster/expose-services.yaml
    ```

9. Install a remote secret in `cluster2` that provides access to the `cluster1` API server.

    ```sh
    istioctl create-remote-secret \
      --context="${CTX_CLUSTER1}" \
      --name=cluster1 | \
      kubectl apply -f - --context="${CTX_CLUSTER2}"
    ```

    **If using kind**, first get the `cluster1` controlplane ip and pass the `--server` option to `istioctl create-remote-secret`.

    ```sh
    CLUSTER1_CONTAINER_IP=$(kubectl get nodes -l node-role.kubernetes.io/control-plane --context "${CTX_CLUSTER1}" -o jsonpath='{.items[0].status.addresses[?(@.type == "InternalIP")].address}')
    istioctl create-remote-secret \
      --context="${CTX_CLUSTER1}" \
      --name=cluster1 \
      --server="https://${CLUSTER1_CONTAINER_IP}:6443" | \
      kubectl apply -f - --context "${CTX_CLUSTER2}"
    ```

10. Install a remote secret in `cluster1` that provides access to the `cluster2` API server.

    ```sh
    istioctl create-remote-secret \
      --context="${CTX_CLUSTER2}" \
      --name=cluster2 | \
      kubectl apply -f - --context="${CTX_CLUSTER1}"
    ```

    **If using kind**, first get the `cluster1` controlplane IP and pass the `--server` option to `istioctl create-remote-secret`

    ```sh
    CLUSTER2_CONTAINER_IP=$(kubectl get nodes -l node-role.kubernetes.io/control-plane --context "${CTX_CLUSTER2}" -o jsonpath='{.items[0].status.addresses[?(@.type == "InternalIP")].address}')
    istioctl create-remote-secret \
      --context="${CTX_CLUSTER2}" \
      --name=cluster2 \
      --server="https://${CLUSTER2_CONTAINER_IP}:6443" | \
      kubectl apply -f - --context "${CTX_CLUSTER1}"
    ```

11. Create sample application namespaces in each cluster.

    ```sh
    kubectl get ns sample --context "${CTX_CLUSTER1}" || kubectl create --context="${CTX_CLUSTER1}" namespace sample
    kubectl label --context="${CTX_CLUSTER1}" namespace sample istio-injection=enabled
    kubectl get ns sample --context "${CTX_CLUSTER2}" || kubectl create --context="${CTX_CLUSTER2}" namespace sample
    kubectl label --context="${CTX_CLUSTER2}" namespace sample istio-injection=enabled
    ```

12. Deploy sample applications in `cluster1`.

    ```sh
    kubectl apply --context="${CTX_CLUSTER1}" \
      -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml \
      -l service=helloworld -n sample
    kubectl apply --context="${CTX_CLUSTER1}" \
      -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml \
      -l version=v1 -n sample
    kubectl apply --context="${CTX_CLUSTER1}" \
      -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/sleep/sleep.yaml -n sample
    ```

13. Deploy sample applications in `cluster2`.

    ```sh
    kubectl apply --context="${CTX_CLUSTER2}" \
      -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml \
      -l service=helloworld -n sample
    kubectl apply --context="${CTX_CLUSTER2}" \
      -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml \
      -l version=v2 -n sample
    kubectl apply --context="${CTX_CLUSTER2}" \
      -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/sleep/sleep.yaml -n sample
    ```

14. Wait for the sample applications to be ready.
    ```sh
    kubectl --context="${CTX_CLUSTER1}" wait --for condition=available -n sample deployment/helloworld-v1
    kubectl --context="${CTX_CLUSTER2}" wait --for condition=available -n sample deployment/helloworld-v2
    kubectl --context="${CTX_CLUSTER1}" wait --for condition=available -n sample deployment/sleep
    kubectl --context="${CTX_CLUSTER2}" wait --for condition=available -n sample deployment/sleep
    ```

15. From `cluster1`, send 10 requests to the helloworld service. Verify that you see responses from both v1 and v2.

    ```sh
    for i in {0..9}; do
      kubectl exec --context="${CTX_CLUSTER1}" -n sample -c sleep \
        "$(kubectl get pod --context="${CTX_CLUSTER1}" -n sample -l \
        app=sleep -o jsonpath='{.items[0].metadata.name}')" \
        -- curl -sS helloworld.sample:5000/hello;
    done
    ```

16. From `cluster2`, send another 10 requests to the helloworld service. Verify that you see responses from both v1 and v2.

    ```sh
    for i in {0..9}; do
      kubectl exec --context="${CTX_CLUSTER2}" -n sample -c sleep \
        "$(kubectl get pod --context="${CTX_CLUSTER2}" -n sample -l \
        app=sleep -o jsonpath='{.items[0].metadata.name}')" \
        -- curl -sS helloworld.sample:5000/hello;
    done
    ```

17. Cleanup

    ```sh
    kubectl delete istios default --context="${CTX_CLUSTER1}"
    kubectl delete ns istio-system --context="${CTX_CLUSTER1}" 
    kubectl delete ns sample --context="${CTX_CLUSTER1}"
    kubectl delete istios default --context="${CTX_CLUSTER2}"
    kubectl delete ns istio-system --context="${CTX_CLUSTER2}" 
    kubectl delete ns sample --context="${CTX_CLUSTER2}"
    ```

### Primary-Remote - Multi-Network

These instructions install a [primary-remote/multi-network](https://istio.io/latest/docs/setup/install/multicluster/primary-remote_multi-network/) Istio deployment using the Sail Operator and Sail CRDs. **Before you begin**, ensure you complete the [common setup](#common-setup).

These installation instructions are adapted from: https://istio.io/latest/docs/setup/install/multicluster/primary-remote_multi-network/.

In this setup there is a Primary cluster (`cluster1`) and a Remote cluster (`cluster2`) which are on separate networks.

1. Create an `Istio` resource on `cluster1`.

    ```sh
    kubectl apply --context "${CTX_CLUSTER1}" -f - <<EOF
    apiVersion: sailoperator.io/v1
    kind: Istio
    metadata:
      name: default
    spec:
      version: v${ISTIO_VERSION}
      namespace: istio-system
      values:
        pilot:
          env:
            EXTERNAL_ISTIOD: "true"
        global:
          meshID: mesh1
          multiCluster:
            clusterName: cluster1
          network: network1
    EOF
    kubectl wait --context "${CTX_CLUSTER1}" --for=jsonpath='{.status.revisions.ready}'=1 istios/default --timeout=3m
    ```

2. Create east-west gateway on `cluster1`.

    ```sh
    kubectl apply --context "${CTX_CLUSTER1}" -f https://raw.githubusercontent.com/istio-ecosystem/sail-operator/main/docs/multicluster/east-west-gateway-net1.yaml
    ```
  
3. Expose istiod on `cluster1`.

    ```sh
    kubectl apply --context "${CTX_CLUSTER1}" -f https://raw.githubusercontent.com/istio-ecosystem/sail-operator/main/docs/multicluster/expose-istiod.yaml
    ```

4. Expose services on `cluster1` and `cluster2`.

    ```sh
    kubectl --context "${CTX_CLUSTER1}" apply -n istio-system -f https://raw.githubusercontent.com/istio-ecosystem/sail-operator/main/docs/multicluster/expose-services.yaml
    ```

5. Create an `Istio` on `cluster2` with the `remote` profile.

    ```sh
    kubectl apply --context "${CTX_CLUSTER2}" -f - <<EOF
    apiVersion: sailoperator.io/v1
    kind: Istio
    metadata:
      name: default
    spec:
      version: v${ISTIO_VERSION}
      namespace: istio-system
      profile: remote
      values:
        istiodRemote:
          injectionPath: /inject/cluster/remote/net/network2
        global:
          remotePilotAddress: $(kubectl --context="${CTX_CLUSTER1}" -n istio-system get svc istio-eastwestgateway -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
    EOF
    ```

6. Set the controlplane cluster and network for `cluster2`.

    ```sh
    kubectl --context="${CTX_CLUSTER2}" annotate namespace istio-system topology.istio.io/controlPlaneClusters=cluster1
    kubectl --context="${CTX_CLUSTER2}" label namespace istio-system topology.istio.io/network=network2
    ```

7. Install a remote secret on `cluster1` that provides access to the `cluster2` API server.

    ```sh
    istioctl create-remote-secret \
      --context="${CTX_CLUSTER2}" \
      --name=remote | \
      kubectl apply -f - --context="${CTX_CLUSTER1}"
    ```

    If using kind, first get the `cluster2` controlplane ip and pass the `--server` option to `istioctl create-remote-secret`

    ```sh
    REMOTE_CONTAINER_IP=$(kubectl get nodes -l node-role.kubernetes.io/control-plane --context "${CTX_CLUSTER2}" -o jsonpath='{.items[0].status.addresses[?(@.type == "InternalIP")].address}')
    istioctl create-remote-secret \
      --context="${CTX_CLUSTER2}" \
      --name=remote \
      --server="https://${REMOTE_CONTAINER_IP}:6443" | \
      kubectl apply -f - --context "${CTX_CLUSTER1}"
    ```

8. Install east-west gateway in `cluster2`.

    ```sh
    kubectl apply --context "${CTX_CLUSTER2}" -f https://raw.githubusercontent.com/istio-ecosystem/sail-operator/main/docs/multicluster/east-west-gateway-net2.yaml
    ```

9. Deploy sample applications to `cluster1`.

    ```sh
    kubectl get ns sample --context "${CTX_CLUSTER1}" || kubectl create --context="${CTX_CLUSTER1}" namespace sample
    kubectl label --context="${CTX_CLUSTER1}" namespace sample istio-injection=enabled
    kubectl apply --context="${CTX_CLUSTER1}" \
      -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml \
      -l service=helloworld -n sample
    kubectl apply --context="${CTX_CLUSTER1}" \
      -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml \
      -l version=v1 -n sample
    kubectl apply --context="${CTX_CLUSTER1}" \
      -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/sleep/sleep.yaml -n sample
    ```

10. Deploy sample applications to `cluster2`.

    ```sh
    kubectl get ns sample --context "${CTX_CLUSTER2}" || kubectl create --context="${CTX_CLUSTER2}" namespace sample
    kubectl label --context="${CTX_CLUSTER2}" namespace sample istio-injection=enabled
    kubectl apply --context="${CTX_CLUSTER2}" \
      -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml \
      -l service=helloworld -n sample
    kubectl apply --context="${CTX_CLUSTER2}" \
      -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml \
      -l version=v2 -n sample
    kubectl apply --context="${CTX_CLUSTER2}" \
      -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/sleep/sleep.yaml -n sample
    ```

11. Verify that you see a response from both v1 and v2 on `cluster1`.

    `cluster1` responds with v1 and v2
    ```sh
    kubectl exec --context="${CTX_CLUSTER1}" -n sample -c sleep \
        "$(kubectl get pod --context="${CTX_CLUSTER1}" -n sample -l \
        app=sleep -o jsonpath='{.items[0].metadata.name}')" \
        -- curl -sS helloworld.sample:5000/hello
    ```

    `cluster2` responds with v1 and v2
    ```sh
    kubectl exec --context="${CTX_CLUSTER2}" -n sample -c sleep \
        "$(kubectl get pod --context="${CTX_CLUSTER2}" -n sample -l \
        app=sleep -o jsonpath='{.items[0].metadata.name}')" \
        -- curl -sS helloworld.sample:5000/hello
    ```

12. Cleanup

    ```sh
    kubectl delete istios default --context="${CTX_CLUSTER1}"
    kubectl delete ns istio-system --context="${CTX_CLUSTER1}" 
    kubectl delete ns sample --context="${CTX_CLUSTER1}"
    kubectl delete istios default --context="${CTX_CLUSTER2}"
    kubectl delete ns istio-system --context="${CTX_CLUSTER2}" 
    kubectl delete ns sample --context="${CTX_CLUSTER2}"
    ```

### External Control Plane

These instructions install an [external control plane](https://istio.io/latest/docs/setup/install/external-controlplane/) Istio deployment using the Sail Operator and Sail CRDs. **Before you begin**, ensure you meet the requirements of the [common setup](#common-setup) and complete **only** the "Setup env vars" step. Unlike other Multi-Cluster deployments, you won't be creating a common CA in this setup.

These installation instructions are adapted from [Istio's external control plane documentation](https://istio.io/latest/docs/setup/install/external-controlplane/) and are intended to be run in a development environment, such as `kind`, rather than in production.

In this setup there is an external control plane cluster (`cluster1`) and a remote cluster (`cluster2`) which are on separate networks.

1. Create an `Istio` resource on `cluster1` to manage the ingress gateways for the external control plane.

    ```sh
    kubectl create namespace istio-system --context "${CTX_CLUSTER1}"
    kubectl apply --context "${CTX_CLUSTER1}" -f - <<EOF
    apiVersion: sailoperator.io/v1
    kind: Istio
    metadata:
      name: default
    spec:
      version: v${ISTIO_VERSION}
      namespace: istio-system
      global:
        network: network1
    EOF
    kubectl wait --context "${CTX_CLUSTER1}" --for=condition=Ready istios/default --timeout=3m
    ```

2. Create the ingress gateway for the external control plane.

    ```sh
    kubectl --context "${CTX_CLUSTER1}" apply -f https://raw.githubusercontent.com/istio-ecosystem/sail-operator/main/docs/multicluster/controlplane-gateway.yaml
    kubectl --context "${CTX_CLUSTER1}" wait '--for=jsonpath={.status.loadBalancer.ingress[].ip}' --timeout=30s svc istio-ingressgateway -n istio-system
    ```

3. Configure your environment to expose the ingress gateway.

    **Note:** these instructions are intended to be executed in a test environment. For production environments, please refer to: https://istio.io/latest/docs/setup/install/external-controlplane/#set-up-a-gateway-in-the-external-cluster and https://istio.io/latest/docs/tasks/traffic-management/ingress/secure-ingress/#configure-a-tls-ingress-gateway-for-a-single-host for setting up a secure ingress gateway.

    ```sh
    export EXTERNAL_ISTIOD_ADDR=$(kubectl -n istio-system --context="${CTX_CLUSTER1}" get svc istio-ingressgateway -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
    ```

4. Create the `external-istiod` namespace and `Istio` resource in `cluster2`.

    ```sh
    kubectl create namespace external-istiod --context="${CTX_CLUSTER2}"
    kubectl apply --context "${CTX_CLUSTER2}" -f - <<EOF
    apiVersion: sailoperator.io/v1
    kind: Istio
    metadata:
      name: external-istiod
    spec:
      version: v${ISTIO_VERSION}
      namespace: external-istiod
      profile: remote
      values:
        defaultRevision: external-istiod
        global:
          istioNamespace: external-istiod
          remotePilotAddress: ${EXTERNAL_ISTIOD_ADDR}
          configCluster: true
        pilot:
          configMap: true
        istiodRemote:
          injectionPath: /inject/cluster/cluster2/net/network1
    EOF
    ```

5. Create the `external-istiod` namespace on `cluster1`.

    ```sh
    kubectl create namespace external-istiod --context="${CTX_CLUSTER1}"
    ```

6. Create the remote-cluster-secret on `cluster1` so that the `external-istiod` can access the remote cluster.

    ```sh
    kubectl create sa istiod-service-account -n external-istiod --context="${CTX_CLUSTER1}"
    REMOTE_NODE_IP=$(kubectl get nodes -l node-role.kubernetes.io/control-plane --context "${CTX_CLUSTER2}" -o jsonpath='{.items[0].status.addresses[?(@.type == "InternalIP")].address}')
    istioctl create-remote-secret \
      --context="${CTX_CLUSTER2}" \
      --type=config \
      --namespace=external-istiod \
      --service-account=istiod-external-istiod \
      --create-service-account=false \
      --server="https://${REMOTE_NODE_IP}:6443" | \
      kubectl apply -f - --context "${CTX_CLUSTER1}"
    ```

7. Create the `Istio` resource on the external control plane cluster. This will manage both Istio configuration and proxies on the remote cluster.

    ```sh
    kubectl apply --context "${CTX_CLUSTER1}" -f - <<EOF
    apiVersion: sailoperator.io/v1
    kind: Istio
    metadata:
      name: external-istiod
    spec:
      namespace: external-istiod
      profile: empty
      values:
        meshConfig:
          rootNamespace: external-istiod
          defaultConfig:
            discoveryAddress: $EXTERNAL_ISTIOD_ADDR:15012
        pilot:
          enabled: true
          volumes:
            - name: config-volume
              configMap:
                name: istio-external-istiod
            - name: inject-volume
              configMap:
                name: istio-sidecar-injector-external-istiod
          volumeMounts:
            - name: config-volume
              mountPath: /etc/istio/config
            - name: inject-volume
              mountPath: /var/lib/istio/inject
          env:
            INJECTION_WEBHOOK_CONFIG_NAME: "istio-sidecar-injector-external-istiod-external-istiod"
            VALIDATION_WEBHOOK_CONFIG_NAME: "istio-validator-external-istiod-external-istiod"
            EXTERNAL_ISTIOD: "true"
            LOCAL_CLUSTER_SECRET_WATCHER: "true"
            CLUSTER_ID: cluster2
            SHARED_MESH_CONFIG: istio
        global:
          caAddress: $EXTERNAL_ISTIOD_ADDR:15012
          istioNamespace: external-istiod
          operatorManageWebhooks: true
          configValidation: false
          meshID: mesh1
          multiCluster:
            clusterName: cluster2
          network: network1
    EOF
    kubectl wait --context "${CTX_CLUSTER1}" --for=condition=Ready istios/external-istiod --timeout=3m
    ```

8. Create the Istio resources to route traffic from the ingress gateway to the external control plane.

    ```sh
    kubectl apply --context "${CTX_CLUSTER1}" -f - <<EOF
    apiVersion: networking.istio.io/v1
    kind: Gateway
    metadata:
      name: external-istiod-gw
      namespace: external-istiod
    spec:
      selector:
        istio: ingressgateway
      servers:
        - port:
            number: 15012
            protocol: tls
            name: tls-XDS
          tls:
            mode: PASSTHROUGH
          hosts:
          - "*"
        - port:
            number: 15017
            protocol: tls
            name: tls-WEBHOOK
          tls:
            mode: PASSTHROUGH
          hosts:
          - "*"
    ---
    apiVersion: networking.istio.io/v1
    kind: VirtualService
    metadata:
      name: external-istiod-vs
      namespace: external-istiod
    spec:
        hosts:
        - "*"
        gateways:
        - external-istiod-gw
        tls:
        - match:
          - port: 15012
            sniHosts:
            - "*"
          route:
          - destination:
              host: istiod-external-istiod.external-istiod.svc.cluster.local
              port:
                number: 15012
        - match:
          - port: 15017
            sniHosts:
            - "*"
          route:
          - destination:
              host: istiod-external-istiod.external-istiod.svc.cluster.local
              port:
                number: 443
    EOF
    ```

9. Wait for the `Istio` resource to be ready:

    ```sh
    kubectl wait --context="${CTX_CLUSTER2}" --for=condition=Ready istios/external-istiod --timeout=3m
    ```

10. Create the `sample` namespace on the remote cluster and label it to enable injection.

    ```sh
    kubectl create --context="${CTX_CLUSTER2}" namespace sample
    kubectl label --context="${CTX_CLUSTER2}" namespace sample istio.io/rev=external-istiod
    ```

11. Deploy the `sleep` and `helloworld` applications to the `sample` namespace.

    ```sh
    kubectl apply -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml -l service=helloworld -n sample --context="${CTX_CLUSTER2}"
    kubectl apply -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml -l version=v1 -n sample --context="${CTX_CLUSTER2}"
    kubectl apply -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/sleep/sleep.yaml -n sample --context="${CTX_CLUSTER2}"

    ```

12. Verify the pods in the `sample` namespace have a sidecar injected.

    ```sh
    kubectl get pod -n sample --context="${CTX_CLUSTER2}"
    ```
    You should see `2/2` pods for each application in the `sample` namespace.
    ```
    NAME                             READY   STATUS    RESTARTS   AGE
    helloworld-v1-6d65866976-jb6qc   2/2     Running   0          49m
    sleep-5fcd8fd6c8-mg8n2           2/2     Running   0          49m
    ```

13. Verify you can send a request to `helloworld` through the `sleep` app on the Remote cluster.

    ```sh
    kubectl exec --context="${CTX_CLUSTER2}" -n sample -c sleep "$(kubectl get pod --context="${CTX_CLUSTER2}" -n sample -l app=sleep -o jsonpath='{.items[0].metadata.name}')" -- curl -sS helloworld.sample:5000/hello
    ```
    You should see a response from the `helloworld` app.
    ```sh
    Hello version: v1, instance: helloworld-v1-6d65866976-jb6qc
    ```

14. Deploy an ingress gateway to the Remote cluster and verify you can reach `helloworld` externally.

    Install the gateway-api CRDs.
    ```sh
    kubectl get crd gateways.gateway.networking.k8s.io --context="${CTX_CLUSTER2}" &> /dev/null || \
    { kubectl kustomize "github.com/kubernetes-sigs/gateway-api/config/crd?ref=v1.1.0" | kubectl apply -f - --context="${CTX_CLUSTER2}"; }
    ```

    Expose `helloworld` through the ingress gateway.
    ```sh
    kubectl apply -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/gateway-api/helloworld-gateway.yaml -n sample --context="${CTX_CLUSTER2}"
    kubectl -n sample --context="${CTX_CLUSTER2}" wait --for=condition=programmed gtw helloworld-gateway
    ```

    Confirm you can access the `helloworld` application through the ingress gateway created in the Remote cluster.
    ```sh
    curl -s "http://$(kubectl -n sample --context="${CTX_CLUSTER2}" get gtw helloworld-gateway -o jsonpath='{.status.addresses[0].value}'):80/hello"
    ```
    You should see a response from the `helloworld` application:
    ```sh
    Hello version: v1, instance: helloworld-v1-6d65866976-jb6qc
    ```

15. Cleanup

    ```sh
    kubectl delete istios default --context="${CTX_CLUSTER1}"
    kubectl delete ns istio-system --context="${CTX_CLUSTER1}"
    kubectl delete istios external-istiod --context="${CTX_CLUSTER1}"
    kubectl delete ns external-istiod --context="${CTX_CLUSTER1}"
    kubectl delete istios external-istiod --context="${CTX_CLUSTER2}"
    kubectl delete ns external-istiod --context="${CTX_CLUSTER2}"
    kubectl delete ns sample --context="${CTX_CLUSTER2}"
    ```

## Dual-stack Support

Kubernetes supports dual-stack networking as a stable feature starting from
[v1.23](https://kubernetes.io/docs/concepts/services-networking/dual-stack/), allowing clusters to handle both
IPv4 and IPv6 traffic. With many cloud providers also beginning to offer dual-stack Kubernetes clusters, it's easier
than ever to run services that function across both address types. Istio introduced dual-stack as an experimental
feature in version 1.17, and promoted it to [Alpha](https://istio.io/latest/news/releases/1.24.x/announcing-1.24/change-notes/) in
version 1.24. With Istio in dual-stack mode, services can communicate over both IPv4 and IPv6 endpoints, which helps
organizations transition to IPv6 while still maintaining compatibility with their existing IPv4 infrastructure.

When Kubernetes is configured for dual-stack, it automatically assigns an IPv4 and an IPv6 address to each pod,
enabling them to communicate over both IP families. For services, however, you can control how they behave using
the `ipFamilyPolicy` setting.

Service.Spec.ipFamilyPolicy can take the following values
- SingleStack: Only one IP family is configured for the service, which can be either IPv4 or IPv6.
- PreferDualStack: Both IPv4 and IPv6 cluster IPs are assigned to the Service when dual-stack is enabled.
                   However, if dual-stack is not enabled or supported, it falls back to singleStack behavior.
- RequireDualStack: The service will be created only if both IPv4 and IPv6 addresses can be assigned.

This allows you to specify the type of service, providing flexibility in managing your network configuration.
For more details, you can refer to the Kubernetes [documentation](https://kubernetes.io/docs/concepts/services-networking/dual-stack/#services).

### Prerequisites

- Kubernetes 1.23 or later configured with dual-stack support.
- Sail Operator is installed.

### Installation Steps

You can use any existing Kind cluster that supports dual-stack networking or, alternatively, install one using the following command.

```sh
kind create cluster --name istio-ds --config - <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
 ipFamily: dual
EOF
```

Note: If you installed the KinD cluster using the command above, install the [Sail Operator](#getting-started) before proceeding with the next steps.

1. Create the `Istio` resource with dual-stack configuration.

   ```sh
   kubectl get ns istio-system || kubectl create namespace istio-system
   kubectl apply -f - <<EOF
   apiVersion: sailoperator.io/v1
   kind: Istio
   metadata:
     name: default
   spec:
     values:
       meshConfig:
         defaultConfig:
           proxyMetadata:
             ISTIO_DUAL_STACK: "true"
       pilot:
         ipFamilyPolicy: RequireDualStack
         env:
           ISTIO_DUAL_STACK: "true"
     version: v1.23.2
     namespace: istio-system
   EOF
   kubectl wait --for=jsonpath='{.status.revisions.ready}'=1 istios/default --timeout=3m
   ```

2. If running on OpenShift platform, create the IstioCNI resource as well.

   ```sh
   kubectl get ns istio-cni || kubectl create namespace istio-cni
   kubectl apply -f - <<EOF
   apiVersion: sailoperator.io/v1
   kind: IstioCNI
   metadata:
     name: default
   spec:
     version: v1.23.2
     namespace: istio-cni
   EOF
   kubectl wait --for=condition=Ready pod -n istio-cni -l k8s-app=istio-cni-node --timeout=60s
   ```

### Validation

1. Create the following namespaces, each hosting the tcp-echo service with the specified configuration.

   - dual-stack: which includes a tcp-echo service that listens on both IPv4 and IPv6 address.
   - ipv4: which includes a tcp-echo service listening only on IPv4 address.
   - ipv6: which includes a tcp-echo service listening only on IPv6 address.

   ```sh
   kubectl get ns dual-stack || kubectl create namespace dual-stack
   kubectl get ns ipv4 || kubectl create namespace ipv4
   kubectl get ns ipv6 ||  kubectl create namespace ipv6
   kubectl get ns sleep || kubectl create namespace sleep
   ```

2. Label the namespaces for sidecar injection.
   ```sh
   kubectl label --overwrite namespace dual-stack istio-injection=enabled
   kubectl label --overwrite namespace ipv4 istio-injection=enabled
   kubectl label --overwrite namespace ipv6 istio-injection=enabled
   kubectl label --overwrite namespace sleep istio-injection=enabled
   ```

3. Deploy the pods and services in their respective namespaces.
   ```sh
   kubectl apply -n dual-stack -f https://raw.githubusercontent.com/istio/istio/release-1.23/samples/tcp-echo/tcp-echo-dual-stack.yaml
   kubectl apply -n ipv4 -f https://raw.githubusercontent.com/istio/istio/release-1.23/samples/tcp-echo/tcp-echo-ipv4.yaml
   kubectl apply -n ipv6 -f https://raw.githubusercontent.com/istio/istio/release-1.23/samples/tcp-echo/tcp-echo-ipv6.yaml
   kubectl apply -n sleep -f https://raw.githubusercontent.com/istio/istio/release-1.23/samples/sleep/sleep.yaml
   kubectl wait --for=condition=Ready pod -n sleep -l app=sleep --timeout=60s
   ```

4. Ensure that the tcp-echo service in the dual-stack namespace is configured with `ipFamilyPolicy` of RequireDualStack.
   ```console
   kubectl get service tcp-echo -n dual-stack -o=jsonpath='{.spec.ipFamilyPolicy}'
   RequireDualStack
   ```

5. Verify that sleep pod is able to reach the dual-stack pods.
   ```console
   kubectl exec -n sleep "$(kubectl get pod -n sleep -l app=sleep -o jsonpath='{.items[0].metadata.name}')" -- sh -c "echo dualstack | nc tcp-echo.dual-stack 9000"
   hello dualstack
   ```

6. Similarly verify that sleep pod is able to reach both ipv4 pods as well as ipv6 pods.
   ```console
   kubectl exec -n sleep "$(kubectl get pod -n sleep -l app=sleep -o jsonpath='{.items[0].metadata.name}')" -- sh -c "echo ipv4 | nc tcp-echo.ipv4 9000"
   hello ipv4
   ```

   ```console
   kubectl exec -n sleep "$(kubectl get pod -n sleep -l app=sleep -o jsonpath='{.items[0].metadata.name}')" -- sh -c "echo ipv6 | nc tcp-echo.ipv6 9000"
   hello ipv6
   ```

7. Cleanup
   ```sh
   kubectl delete istios default
   kubectl delete ns istio-system
   kubectl delete istiocni default
   kubectl delete ns istio-cni
   kubectl delete ns dual-stack ipv4 ipv6 sleep
   ```

## Addons

Addons are managed separately from the Sail Operator. You can follow the [istio documentation](https://istio.io/latest/docs/ops/integrations/) for how to install addons. Below is an example of how to install some addons for Istio.

The sample will deploy:

- Prometheus
- Jaeger
- Kiali
- Bookinfo demo app

*Prerequisites*

- Sail operator is installed.
- Control Plane is installed via the Sail Operator.

### Deploy Prometheus and Jaeger addons

```sh
kubectl apply -f https://raw.githubusercontent.com/istio/istio/master/samples/addons/prometheus.yaml
kubectl apply -f https://raw.githubusercontent.com/istio/istio/master/samples/addons/jaeger.yaml
```

### Deploy Kiali addon

Install the kiali operator.

You can install the kiali operator through OLM if running on Openshift, otherwise you can use helm:

```sh
helm install --namespace kiali-operator --create-namespace kiali-operator kiali/kiali-operator
```

Find out the revision name of your Istio instance. In our case it is `test`.
    
```bash
$ kubectl get istiorevisions.sailoperator.io 
NAME   READY   STATUS    IN USE   VERSION   AGE
test   True    Healthy   True     v1.21.0   119m
```

Create a Kiali resource and point it to your Istio instance. Make sure to replace `test` with your revision name in the fields `config_map_name`, `istio_sidecar_injector_config_map_name`, `istiod_deployment_name` and `url_service_version`.

```sh
kubectl apply -f - <<EOF
apiVersion: kiali.io/v1alpha1
kind: Kiali
metadata:
  name: kiali
  namespace: istio-system
spec:
  external_services:
    grafana:
      enabled: false
    istio:
      component_status:
        enabled: false
      config_map_name: istio-test
      istio_sidecar_injector_config_map_name: istio-sidecar-injector-test
      istiod_deployment_name: istiod-test
      url_service_version: 'http://istiod-test.istio-system:15014/version'
EOF
```

### Deploy Gateway and Bookinfo

Create the bookinfo namespace (if it doesn't already exist) and enable injection.

```sh
kubectl create namespace bookinfo
kubectl label namespace bookinfo istio.io/rev=test
```

Install Bookinfo demo app.

```sh
kubectl apply -n bookinfo -f https://raw.githubusercontent.com/istio/istio/master/samples/bookinfo/platform/kube/bookinfo.yaml
kubectl apply -n bookinfo -f https://raw.githubusercontent.com/istio/istio/master/samples/bookinfo/platform/kube/bookinfo-versions.yaml
```

Install gateway API CRDs if they are not already installed.

```sh
kubectl get crd gateways.gateway.networking.k8s.io &> /dev/null || \
  { kubectl kustomize "github.com/kubernetes-sigs/gateway-api/config/crd?ref=v1.1.0" | kubectl apply -f -; }
```

Create bookinfo gateway.

```sh
kubectl apply -n bookinfo -f https://raw.githubusercontent.com/istio/istio/master/samples/bookinfo/gateway-api/bookinfo-gateway.yaml
kubectl wait -n bookinfo --for=condition=programmed gtw bookinfo-gateway
```

### Generate traffic and visualize your mesh

Send traffic to the productpage service. Note that this command will run until cancelled.

```sh
export INGRESS_HOST=$(kubectl get gtw bookinfo-gateway -n bookinfo -o jsonpath='{.status.addresses[0].value}')
export INGRESS_PORT=$(kubectl get gtw bookinfo-gateway -n bookinfo -o jsonpath='{.spec.listeners[?(@.name=="http")].port}')
export GATEWAY_URL=$INGRESS_HOST:$INGRESS_PORT
watch curl http://${GATEWAY_URL}/productpage &> /dev/null
```

In a separate terminal, open Kiali to visualize your mesh.

If using Openshift, open the Kiali route:

```sh
echo https://$(kubectl get routes -n istio-system kiali -o jsonpath='{.spec.host}')
```

Otherwise, port forward to the kiali pod directly:

```sh
kubectl port-forward -n istio-system svc/kiali 20001:20001
```

You can view Kiali dashboard at: http://localhost:20001

## Observability Integrations

### Scraping metrics using the OpenShift monitoring stack
The easiest way to get started with production-grade metrics collection is to use OpenShift's user-workload monitoring stack. The following steps assume that you installed Istio into the `istio-system` namespace. Note that these steps are not specific to the Sail Operator, but describe how to configure user-workload monitoring for Istio in general.

*Prerequisites*
* User Workload monitoring is [enabled](https://docs.openshift.com/container-platform/latest/observability/monitoring/enabling-monitoring-for-user-defined-projects.html)

*Steps*
1. Create a ServiceMonitor for istiod.

    ```yaml
    apiVersion: monitoring.coreos.com/v1
    kind: ServiceMonitor
    metadata:
      name: istiod-monitor
      namespace: istio-system 
    spec:
      targetLabels:
      - app
      selector:
        matchLabels:
          istio: pilot
      endpoints:
      - port: http-monitoring
        interval: 30s
    ```
1. Create a PodMonitor to scrape metrics from the istio-proxy containers. Note that *this resource has to be created in all namespaces where you are running sidecars*.

    ```yaml
    apiVersion: monitoring.coreos.com/v1
    kind: PodMonitor
    metadata:
      name: istio-proxies-monitor
      namespace: istio-system 
    spec:
      selector:
        matchExpressions:
        - key: istio-prometheus-ignore
          operator: DoesNotExist
      podMetricsEndpoints:
      - path: /stats/prometheus
        interval: 30s
        relabelings:
        - action: keep
          sourceLabels: ["__meta_kubernetes_pod_container_name"]
          regex: "istio-proxy"
        - action: keep
          sourceLabels: ["__meta_kubernetes_pod_annotationpresent_prometheus_io_scrape"]
        - action: replace
          regex: (\d+);(([A-Fa-f0-9]{1,4}::?){1,7}[A-Fa-f0-9]{1,4})
          replacement: "[$2]:$1"
          sourceLabels: ["__meta_kubernetes_pod_annotation_prometheus_io_port","__meta_kubernetes_pod_ip"]
          targetLabel: "__address__"
        - action: replace
          regex: (\d+);((([0-9]+?)(\.|$)){4})
          replacement: "$2:$1"
          sourceLabels: ["__meta_kubernetes_pod_annotation_prometheus_io_port","__meta_kubernetes_pod_ip"]
          targetLabel: "__address__"
        - action: labeldrop
          regex: "__meta_kubernetes_pod_label_(.+)"
        - sourceLabels: ["__meta_kubernetes_namespace"]
          action: replace
          targetLabel: namespace
        - sourceLabels: ["__meta_kubernetes_pod_name"]
          action: replace
          targetLabel: pod_name
    ```

Congratulations! You should now be able to see your control plane and data plane metrics in the OpenShift Console. Just go to Observe -> Metrics and try the query `istio_requests_total`.

### Configure tracing with OpenShift distributed tracing
This section describes how to setup Istio with OpenShift Distributed Tracing to send distributed traces.

*Prerequisites*
* A Tempo stack is installed and configured
* An instance of an OpenTelemetry collector is already configured in the istio-system namespace
* An Istio instance is created with the `openshift` profile
* An Istio CNI instance is created with the `openshift` profile

*Steps*
1. Configure Istio to enable tracing and include the OpenTelemetry settings:
    ```yaml
    meshConfig:
      enableTracing: true
      extensionProviders:
      - name: otel-tracing
        opentelemetry:
          port: 4317
          service: otel-collector.istio-system.svc.cluster.local 
    ```
The *service* field is the OpenTelemetry collector service in the `istio-system` namespace.

2. Create an Istio telemetry resource to active the OpenTelemetry tracer
    ```yaml
    apiVersion: telemetry.istio.io/v1
    kind: Telemetry
    metadata:
      name: otel-demo
      namespace: istio-system
    spec:
      tracing:
      - providers:
          - name: otel-tracing
            randomSamplingPercentage: 100
    ```

3. Validate the integration: Generate some traffic

We can [Deploy Bookinfo](#deploy-gateway-and-bookinfo) and generate some traffic.

4. Validate the integration: See the traces in the UI

```sh
kubectl get routes -n tempo tempo-sample-query-frontend-tempo
```

If you [configure Kiali with OpenShift distributed tracing](#integrating-kiali-with-openshift-distributed-tracing) you can verify from there. 

### Integrating with Kiali
Integration with Kiali really depends on how you collect your metrics and traces. Note that Kiali is a separate project which for the purpose of this document we'll expect is installed using the Kiali operator. The steps here are not specific to Sail Operator, but describe how to configure Kiali for use with Istio in general.

#### Integrating Kiali with the OpenShift monitoring stack
If you followed [Scraping metrics using the OpenShift monitoring stack](#scraping-metrics-using-the-openshift-monitoring-stack), you can set up Kiali to retrieve metrics from there.

*Prerequisites*
* User Workload monitoring is [enabled](https://docs.openshift.com/container-platform/latest/observability/monitoring/enabling-monitoring-for-user-defined-projects.html) and [configured](#scraping-metrics-using-the-openshift-monitoring-stack)
* Kiali Operator is installed

*Steps*
1. Create a ClusterRoleBinding for Kiali, so it can view metrics from user-workload monitoring

    ```yaml
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRoleBinding
    metadata:
      name: kiali-monitoring-rbac
    roleRef:
      apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: cluster-monitoring-view
    subjects:
    - kind: ServiceAccount
      name: kiali-service-account
      namespace: istio-system
    ```
1. Find out the revision name of your Istio instance. In our case it is `test`.
    
    ```bash
    $ kubectl get istiorevisions.sailoperator.io 
    NAME   READY   STATUS    IN USE   VERSION   AGE
    test   True    Healthy   True     v1.21.0   119m
    ```
1. Create a Kiali resource and point it to your Istio instance. Make sure to replace `test` with your revision name in the fields `config_map_name`, `istio_sidecar_injector_config_map_name`, `istiod_deployment_name` and `url_service_version`.

    ```yaml
    apiVersion: kiali.io/v1alpha1
    kind: Kiali
    metadata:
      name: kiali-user-workload-monitoring
      namespace: istio-system
    spec:
      external_services:
        istio:
          config_map_name: istio-test
          istio_sidecar_injector_config_map_name: istio-sidecar-injector-test
          istiod_deployment_name: istiod-test
          url_service_version: 'http://istiod-test.istio-system:15014/version'
        prometheus:
          auth:
            type: bearer
            use_kiali_token: true
          thanos_proxy:
            enabled: true
          url: https://thanos-querier.openshift-monitoring.svc.cluster.local:9091
    ```
#### Integrating Kiali with OpenShift Distributed Tracing
This section describes how to setup Kiali with OpenShift Distributed Tracing to read the distributed traces.

*Prerequisites*
* Istio tracing is [Configured with OpenShift distributed tracing](#configure-tracing-with-openshift-distributed-tracing) 

*Steps*
1. Setup Kiali to access traces from the Tempo frontend: 
    ```yaml
    external_services:
      grafana:
        enabled: true
        url: "http://grafana-istio-system.apps-crc.testing/"
      tracing:
        enabled: true
        provider: tempo
        use_grpc: false
        in_cluster_url: http://tempo-sample-query-frontend.tempo:3200
        url: 'https://tempo-sample-query-frontend-tempo.apps-crc.testing'
        tempo_config:
          org_id: "1"
          datasource_uid: "a8d2ef1c-d31c-4de5-a90b-e7bc5252cd00"
    ```

Where: 
* `external_services.grafana` section: Is just needed to see the "View in Tracing" link from the Traces tab
* `external_services.tracing.tempo_config`: Is just needed to see the "View in Tracing" link from the Traces tab and redirect to the proper Tempo datasource

Now, we should be able to see traces from Kiali. For this, you can: 
1. Select a Workload/Service/App
2. Click in the "Traces" tab

## Uninstalling

### Deleting Istio
1. In the OpenShift Container Platform web console, click **Operators** -> **Installed Operators**.
1. Click **Istio** in the **Provided APIs** column.
1. Click the Options menu, and select **Delete Istio**.
1. At the prompt to confirm the action, click **Delete**.

### Deleting IstioCNI
1. In the OpenShift Container Platform web console, click **Operators** -> **Installed Operators**.
1. Click **IstioCNI** in the **Provided APIs** column.
1. Click the Options menu, and select **Delete IstioCNI**.
1. At the prompt to confirm the action, click **Delete**.

### Deleting the Sail Operator
1. In the OpenShift Container Platform web console, click **Operators** -> **Installed Operators**.
1. Locate the Sail Operator. Click the Options menu, and select **Uninstall Operator**.
1. At the prompt to confirm the action, click **Uninstall**.

### Deleting the istio-system and istio-cni Projects
1. In the OpenShift Container Platform web console, click  **Home** -> **Projects**.
1. Locate the name of the project and click the Options menu.
1. Click **Delete Project**.
1. At the prompt to confirm the action, enter the name of the project.
1. Click **Delete**.

### Decide whether you want to delete the CRDs as well
OLM leaves this [decision](https://olm.operatorframework.io/docs/tasks/uninstall-operator/#step-4-deciding-whether-or-not-to-delete-the-crds-and-apiservices) to the users.
If you want to delete the Istio CRDs, you can use the following command.
```bash
$ kubectl get crds -oname | grep istio.io | xargs kubectl delete
```
