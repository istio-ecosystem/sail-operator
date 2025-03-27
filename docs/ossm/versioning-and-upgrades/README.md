# Versioning and upgrades
This document describes the versioning and upgrade process for the OSSM 3.x operator and its components.

## Table of Contents
- [Understanding Versioning](#understanding-versioning)
- [Understanding Service Mesh & Istio versions](#understanding-service-mesh--istio-versions)
- [Operator Updates & Channels](#operator-updates--channels)
  - [Stable vs Versioned Operator Channels](#stable-vs-versioned-operator-channels)
  - [Operator Update Process](#operator-update-process)
- [Istio Update Process](#istio-update-process)
  - [Istio control plane update strategies](#istio-control-plane-update-strategies)
    - [About InPlace strategy](#about-inplace-strategy)
      - [Selecting the InPlace strategy](#selecting-the-inplace-strategy)
      - [Updating the Istio control plane with the InPlace strategy](#updating-the-istio-control-plane-with-the-inplace-strategy)
    - [About RevisionBased strategy](#about-revisionbased-strategy)
      - [Selecting the RevisionBased strategy](#selecting-the-revisionbased-strategy)
      - [Updating the Istio control plane with the RevisionBased strategy](#updating-the-istio-control-plane-with-the-revisionbased-strategy)
      - [Updating the Istio control plane with the RevisionBased strategy and IstioRevisionTag](#updating-the-istio-control-plane-with-the-revisionbased-strategy-and-istiorevisiontag)
- [Istio-CNI Update Process](#istio-cni-update-process)
    - [Updating the Istio-CNI control plane](#updating-the-istio-cni-control-plane)

## Understanding Versioning
Red Hat uses semantic versioning for product releases. Semantic Versioning is a 3-component number in the format of X.Y.Z, where:
- X stands for a Major version. Major releases usually denote some sort of breaking change: architectural changes, API changes, schema changes, and similar major updates.
- Y stands for a Minor version. Minor releases contain new features and functionality while maintaining backwards compatibility.
- Z stands for a Patch version (also known as a z-stream release). Patch releases are used to address Common Vulnerabilities and Exposures (CVEs) and release bug fixes. New features and functionality are generally not released as part of a Patch release.

## Understanding Service Mesh & Istio versions

The most current Operator version is 3.0.0. The Operator version number indicates the version of the currently installed Operator, which provides support for the components listed in the OpenShift Service Mesh component table. Additional releases of Istio may be available in the operator to facilitate upgrades, but only the latest version of Istio for any given operator version is supported. To understand the Istio version that is supported by a given OSSM Operator version, refer to the OpenShift Service Mesh [supported versions table](https://docs.redhat.com/en/documentation/red_hat_openshift_service_mesh/3.0/html/release_notes/ossm-release-notes-support-tables-assembly#service-mesh-product-supported-versions_ossm-release-notes-support-tables-assembly).

This table shows the supported versions of Istio for each Operator version:
| Operator Version | Istio Version |
|------------------|---------------|
| 3.0.0            | 1.24.3        |
<!--- update the table above with the latest supported Istio version for the Operator version --->

## Operator Updates & Channels
The Operator Lifecycle Manager (OLM) provides a way to manage Operators and their associated services. OLM uses the concept of channels to manage the distribution of updates to Operators. Channels are a way to group related updates together. 

In order to keep your Service Mesh patched with the latest security fixes, bug fixes, and software updates, you must keep your OSSM operator updated. The operator upgrade procedure depends on the configured operator channel and approval strategy. The Operator Lifecycle Manager (OLM) provides the following channels for the OSSM operator:

- Stable: This channel contains the latest stable release of the OSSM operator. 
- Versioned: This channel contains the latest versioned release of a specific OSSM operator version.

### Stable vs Versioned Operator Channels
Using the “stable” channel enables installation of the most recent version of the Red Hat OpenShift Service Mesh 3 Operator and the latest supported version of Istio. When a new release of the operator is available—whether it’s a minor update or just a patch— you will be able to upgrade to the new operator version and the corresponding updated Istio version. The “stable” operator channel should be used when you want to stay on the most current version of service mesh.

In contrast, the “stable-X.Y” channel enables the installation of only the operator version X.Y and subsequent X.Y.Z patch releases. For example, the “stable-3.0” channel will make available the latest OpenShift Service Mesh 3.0 patch release of the operator - for example. “3.0.2”. When a new patch release of the operator is available, you will be able to upgrade to the newer patch version of the operator. To upgrade to a later minor release of OpenShift Service Mesh, you will need to manually update the operator channel. A versioned operator channel should be used when you want to remain on the same minor release and only make patch updates.

The update strategy is located in the Install Operator section under the section update approval, the default value for the update strategy is Automatic. 

### Operator Update Process
The operator will automatically be upgraded to the latest available version, based on the selected channel (Check previous topic to understand operator channels), if the “Automatic” (default) approval strategy is set. If a “Manual” approval strategy was selected, OLM will create an update request, which a cluster administrator must then manually approve to update the operator to the new version.

Take into account that the operator update process will not automatically update the Istio control plane unless you set the version of the `Istio` resource to an alias, for example: `vX.Y-latest`, and the `updateStrategy` is set to `InPlace`, this will trigger a control plane update when a new version is available in the operator. By default, the operator will not update the Istio control plane unless the `Istio` resource is updated with a new Istio version.

## Istio Update Process
Once the operator has been successfully updated, the Istio control plane must be updated to the latest supported version. The `Istio` resource configuration determines how the upgrade will be carried out, including what manual steps will need to be taken versus what is performed automatically after the operator is upgraded.

The `Istio` resource configuration includes the following fields that are relevant to the upgrade process:
- `spec.version`
The version of Istio to be installed. The value of this field takes the form “vX.Y.Z”, where “X.Y.Z” is the desired Istio release. For example, if Istio “1.24.3” is desired, the field should specify “v1.24.3”. This field can be also set to an alias, for example, `vX.Y-latest`, to automatically update to the latest supported version of Istio under that minor version, with this the operator will ensure that the control plane is kept up to date with the latest available patch release of Istio.

To ensure the operator always has access to the latest available patch releases, its approval strategy should be set to “Automatic”. Once the operator is updated, if a version of the form “vX.Y-latest” is specified, it will initiate an update of the Istio control plane based on the configured `updateStrategy`.

- `spec.updateStrategy`
The strategy to use when updating the Istio control plane. The available strategies are `InPlace` and `RevisionBased`. The `InPlace` strategy will update the entire control plane at once as soon the new version is being set. The `RevisionBased` strategy will create new `IstioRevision` while keeping old `IstioRevision` in place allowing gradual update for better control. For more information check the next sections.

### Istio control plane update strategies
The `Istio` resource configuration includes the `spec.updateStrategy` field, which determines how the Istio control plane will be updated. When the operator observes a change in the Istio `spec.version` field or if a new minor release becomes available and a `vX.Y-latest` version alias is configured, an upgrade procedure will be initiated. The available strategies are `InPlace` and `RevisionBased`.

When the `InPlace` strategy is used, the existing Istio control plane is updated and restarted in-place. This means that there is only one instance of the Istio control plane present during the upgrade process and workloads therefore do not need to be moved to the updated control plane instance. The update process is completed by restarting application workloads and gateways to update the Envoy proxies.

While the `InPlace` strategy is the simpler and more efficient update strategy, there is a small possibility of application traffic interruption in the event that a workload pod updates, restarts or scales while the Istio control plane is restarting. This can be mitigated by running multiple replicas of the Istio control plane (Istiod).

When the `RevisionBased` strategy is used, a new Istio control plane instance is created for every change to the `.spec.version` field. The old control plane remains in place until all workloads have been moved to the new control plane instance. Workloads are moved to the new control plane by updating either the `istio.io/rev` labels or using the `IstioRevisionTag` resource, followed by a restart. 

While the `RevisionBased` strategy adds additional steps and requires multiple instances of the Istio control plane to run in parallel during the upgrade procedure, it allows a subset of workloads to be migrated to the updated control plane such that they can be validated before migrating the remaining workloads. This is particularly useful for migrating large meshes containing mission critical workloads. 

### About InPlace strategy
The `InPlace` strategy runs only one revision of the control plane at all times. When you perform an InPlace update, all of the workloads immediately connect to the new version of the control plane. In order to ensure compatibility between the sidecars and the control plane you cannot upgrade by more than one minor version at a time.

#### Selecting the InPlace strategy
To select the `InPlace` strategy, set the `spec.updateStrategy` field in the `Istio` resource to `InPlace`. The following example shows how to set the `updateStrategy` field to `InPlace`:

```yaml
kind: Istio
spec:
  updateStrategy:
    type: InPlace
```

You can set this value when you first create the resource or you can edit the resource later. If you choose to edit the resource after it is created, make the change prior to updating the Istio control plane. For more information about switching between strategies, see [Switching between update strategies](TODO).

When `Istio` is configured to use the `InPlace` strategy, the `IstioRevision` resource that the Operator creates always has the same name as the `Istio` resource.

##### Updating the Istio control plane with the InPlace strategy
When updating `Istio` using the `InPlace` strategy, you can only increment by one minor version at a time. If you want to update by more than one minor version, then you must increment the version and restart the workloads after each update. This ensures that the sidecar version is compatible with the control plane version. After all the workloads are restarted, the update process is complete.

Prerequisites:
- You are logged in to OpenShift Container Platform as `cluster-admin`.
- You have installed the Red Hat OpenShift Service Mesh Operator 3, and deployed `Istio`.
- `istioctl` is installed on your local machine. For more information check the [documentation](https://github.com/openshift-service-mesh/sail-operator/blob/fa8134936066e116eccd1b3059fb124c20cdf98a/docs/ossm/istioctl/README.md).
- Running control plane with the `InPlace` strategy running. For this example we are using the `Istio` resource in the `istio-system` namespace with the name `default`:
```yaml
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  namespace: istio-system
  version: v1.24.2
  updateStrategy:
    type: InPlace
```
- You have installed `IstioCNI` with the desired version. For this example we are using the `IstioCNI` resource in the `istio-cni` namespace with the name `default`:
```yaml
apiVersion: sailoperator.io/v1
kind: IstioCNI
metadata:
  name: default
spec:
  version: v1.24.2
  namespace: istio-cni
```
- Application workloads running in the cluster. For this example we are using the `bookinfo` application in the `bookinfo` namespace. Note that the `bookinfo` application namespace is labeled with the `istio-injection=enabled` label:
`$ oc label namespace bookinfo istio-injection=enabled`

To update the Istio control plane with the `InPlace` strategy, follow these steps:
- Edit the `Istio` resource to the desired version. For example, to update to Istio `1.24.3`, set the `spec.version` field to `v1.24.3`.
`$ oc patch istio default -n istio-system --type='merge' -p '{"spec":{"version":"v1.24.3"}}'`

The Service Mesh Operator deploys a new version of the control plane that replaces the old version of the control plane. The sidecars automatically reconnect to the new control plane.
- Confirm that the new version of the control plane is running and ready by entering the following command
`$ oc get istio default`
You should see the new version of the control plane running and ready.
```console
NAME      REVISIONS   READY   IN USE   ACTIVE REVISION   STATUS    VERSION   AGE
default   1           1       1        default           Healthy   v1.24.3   4m50s
```
- Restart the application workloads so that the new version of the sidecar gets injected by entering the following command:
`$ oc rollout restart deployment -n bookinfo`
- Confirm that the application workloads are running and ready by entering the following command:
`$ oc get pods -n bookinfo`
- Confirm that the new version of the sidecar is running by entering the following command:
`$ istioctl proxy-status`
The column `VERSION` should match the new control plane version.

### About RevisionBased strategy
When the RevisionBased strategy is used, a new Istio control plane instance is created for every change to the `Istio` `.spec.version` field. The old control plane remains in place until all workloads have been moved to the new control plane instance, making canary upgrades possible. This needs to be done by the user by updating the namespace label and restarting all the pods. The old control plane will be deleted after the grace period specified in the `Istio` resource field `spec.updateStrategy.inactiveRevisionDeletionGracePeriodSeconds`. The RevisionBased strategy also allows you to update by more than one minor version.

#### Selecting the RevisionBased strategy
To deploy `Istio` with the `RevisionBased` strategy, create the `Istio` resource with the following `spec.updateStrategy` value:
```yaml
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  namespace: istio-system
  version: v1.24.3
  updateStrategy:
    type: RevisionBased
```

After you select the strategy for the `Istio` resource, the Operator creates a new `IstioRevision` resource with the name `<istio resource name>-<version>`. To attach the workloads to a control plane deployed by using the `RevisionBased` strategy, you must set the `istio.io/rev` namespace label to the name of the `IstioRevision`. Alternatively, you can apply the label to the workload pods to attach them to the new control plane.

##### Updating the Istio control plane with the RevisionBased strategy
When updating `Istio` using the `RevisionBased` strategy, you can update by more than one minor version at a time. The Operator creates a new `IstioRevision` resource for each change to the `.spec.version` field. The Operator then creates a new control plane instance for each `IstioRevision` resource. The workloads are attached to the new control plane instance by setting the `istio.io/rev` namespace label to the name of the `IstioRevision` resource and restarting the workloads.

Prerequisites:
- You are logged in to OpenShift Container Platform as `cluster-admin`.
- You have installed the Red Hat OpenShift Service Mesh Operator 3, and deployed `Istio` with the `RevisionBased` strategy. For this example we are using the `Istio` resource in the `istio-system` namespace with the name `default`:
```yaml
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  namespace: istio-system
  version: v1.24.2
  updateStrategy:
    type: RevisionBased
```
- You have installed `IstioCNI` with the desired version. For this example we are using the `IstioCNI` resource in the `istio-cni` namespace with the name `default`:
```yaml
apiVersion: sailoperator.io/v1
kind: IstioCNI
metadata:
  name: default
spec:
  version: v1.24.2
  namespace: istio-cni
```
- Application workloads running in the cluster. For this example we are using the `bookinfo` application in the `bookinfo` namespace. For this example the `bookinfo` application is already labeled with the `istio.io/rev` label with the label `istio.io/rev=default-v1-24-2`
`$ oc label namespace bookinfo istio.io/rev=default-v1-24-2`
- `istioctl` is installed on your local machine. For more information check the [documentation](https://github.com/openshift-service-mesh/sail-operator/blob/fa8134936066e116eccd1b3059fb124c20cdf98a/docs/ossm/istioctl/README.md)

To update the Istio control plane with the `RevisionBased` strategy, follow these steps:
- Edit the `Istio` resource to the desired version. For example, to update to Istio `1.24.3`, set the `spec.version` field to `v1.24.3`.
`$ oc patch istio default -n istio-system --type='merge' -p '{"spec":{"version":"v1.24.3"}}'`
The Service Mesh Operator deploys a new version of the control plane alongside the old version of the control plane. The sidecars remain connected to the old control plane.
- Confirm that both revisions of the control plane are running and ready:
`$ oc get istiorevisions`
You should see two `IstioRevision` resources, one for the old control plane and one for the new control plane. 
```console
NAME              TYPE    READY   STATUS    IN USE   VERSION   AGE
default-v1-24-2   Local   True    Healthy   True     v1.24.2   10m
default-v1-24-4   Local   True    Healthy   False    v1.24.3   66s
```
`default-v1-24-2` is the revision for old control plane and `default-v1-24-4` is revision for the new control plane. The `IN USE` column indicates which revision is currently serving traffic or being used by any resource in the cluster.
- Confirm there are two control plane pods running, one for each revision.
`$ oc get pods -n istio-system`
You should see two control plane pods, one for each revision.
```console
NAME                                      READY   STATUS    RESTARTS   AGE
istiod-default-v1-24-2-c98fd9675-r7bfw    1/1     Running   0          10m
istiod-default-v1-24-3-7495cdc7bf-v8t4g   1/1     Running   0          113s
```
- Confirm that the proxy sidecars are still connected to the old control plane by entering the following command:
`$ istioctl proxy-status`
The column `VERSION` should match the old control plane version.
- Move the workloads to the new control plane by updating the `istio.io/rev` label on the application namespace or the pods to the revision name of the new control plane.
`$ oc label namespace bookinfo istio.io/rev=default-v1-24-3 --overwrite`
- Restart the application workloads so that the new version of the sidecar gets injected by entering the following command:
`$ oc rollout restart deployment -n bookinfo`
- Confirm that the application workloads are running and ready by entering the following command:
`$ oc get pods -n bookinfo`
- Confirm that the new version of the sidecar is running by entering the following command:
`$ istioctl proxy-status`
The column `VERSION` should match the new control plane version.
- Confirm the deletion of the old control plane by entering the following command:
`$ oc get istiorevisions`
You should see only the new `IstioRevision` resource.
```console
NAME              TYPE    READY   STATUS    IN USE   VERSION   AGE
default-v1-24-3   Local   True    Healthy   True     v1.24.3   2m
```
The old `IstioRevision` resource and the old control plane will be deleted when the grace period specified in the `Istio` resource field `spec.updateStrategy.inactiveRevisionDeletionGracePeriodSeconds` expires, the default value is 30 seconds.

##### Updating the Istio control plane with the RevisionBased strategy and IstioRevisionTag
When updating `Istio` using the `RevisionBased` strategy, you can create an `IstioRevisionTag` resource to tag a specific `IstioRevision` resource. The `IstioRevisionTag` resource allows you to attach workloads to a specific `IstioRevision` resource without changing the `istio.io/rev` label on the namespace or the pods. The `IstioRevisionTag` resource is useful when you want to perform a canary upgrade or when you want to attach a specific workload to a specific control plane instance.

Prerequisites:
- You are logged in to OpenShift Container Platform as `cluster-admin`.
- You have installed the Red Hat OpenShift Service Mesh Operator 3, and deployed `Istio` with the `RevisionBased` strategy. For this example we are using the `Istio` resource in the `istio-system` namespace with the name `default`:
```yaml
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  namespace: istio-system
  updateStrategy:
    type: RevisionBased
  version: v1.24.2
```
- You have created an `IstioRevisionTag` resource. For this example we are using the `IstioRevisionTag` resource in the with the following configuration:
```yaml
apiVersion: sailoperator.io/v1
kind: IstioRevisionTag
metadata:
  name: default
spec:
  targetRef:
    kind: Istio
    name: default
```
See that the `targetRef` field is referencing the `Istio` resource that you want to tag. For this example the `IstioRevisionTag` reference to the `Istio` resource with the name `default`.
- You have installed `IstioCNI` with the desired version. For this example we are using the `IstioCNI` resource in the `istio-cni` namespace with the name `default`:
```yaml
apiVersion: sailoperator.io/v1
kind: IstioCNI
metadata:
  name: default
spec:
  version: v1.24.2
  namespace: istio-cni
```
- Application workloads running in the cluster. For this example we are using the `bookinfo` application in the `bookinfo` namespace. For this example the `bookinfo` application namespace is labeled with the `istio-injection=enabled` label.
`$ oc label namespace bookinfo istio-injection=enabled`
To update the Istio control plane with the `RevisionBased` strategy and `IstioRevisionTag`, follow these steps:
- Update the `Istio` resource to the desired version. For example, to update to Istio `1.24.3`, set the `spec.version` field to `v1.24.3`.
`$ oc patch istio default -n istio-system --type='merge' -p '{"spec":{"version":"v1.24.3"}}'`
The Service Mesh Operator deploys a new version of the control plane alongside the old version of the control plane. The sidecars remain connected to the old control plane.
- Confirm that both revisions of the control plane are running and ready:
`$ oc get istiorevisions`
You should see two `IstioRevision` resources, one for the old control plane and one for the new control plane. 
```console
NAME              TYPE    READY   STATUS    IN USE   VERSION   AGE
default-v1-24-2   Local   True    Healthy   True     v1.24.2   10m
default-v1-24-3   Local   True    Healthy   True     v1.24.3   66s
```
- Verify that `IstioRevisionTag` `InUse` field is set to `true`:
`$ oc get istiorevisiontags`
```console
NAME      STATUS    IN USE   REVISION          AGE
default   Healthy   True     default-v1-24-3   10m44s
```
Now, both our `IstioRevisions` and the `IstioRevisionTag` are considered in use. The old revision `default-v1-24-2` because it is being used by proxies, the new revision `default-v1-24-3` because it is referenced by the tag, and lastly the `IstioRevisionTag` is considered in use because it is referenced by the bookinfo namespace.

When the `IstioRevisionTag` references an `Istio` resource, it automatically tracks the active `IstioRevision` associated with that `Istio` instance. This means whenever the underlying Istio control plane is updated to a new revision, the `IstioRevisionTag` updates automatically to reference this new revision. The Sail Operator manages this synchronization, ensuring that your namespaces and workloads referencing the tag remain up-to-date without manual intervention.

- Confirm the proxy sidecar version remains the same:
`$ istioctl proxy-status`
The column `VERSION` should match the old control plane version.
- Restart the application workloads so that the new version of the sidecar gets injected by entering the following command:
`$ oc rollout restart deployment -n bookinfo`
- Confirm that the application workloads are running and ready by entering the following command:
`$ oc get pods -n bookinfo`
- Confirm that the new version of the sidecar is running by entering the following command:
`$ istioctl proxy-status`
The column `VERSION` should match the new control plane version.
- Confirm the deletion of the old control plane by entering the following command:
`$ oc get istiorevisions`
You should see only the new `IstioRevision` resource.
```console
NAME              TYPE    READY   STATUS    IN USE   VERSION   AGE
default-v1-24-3   Local   True    Healthy   True     v1.24.3   5m31s
```
The old `IstioRevision` resource and the old control plane will be deleted when the grace period specified in the `Istio` resource field `spec.updateStrategy.inactiveRevisionDeletionGracePeriodSeconds` expires, the default value is 30 seconds.

## Istio-CNI Update Process
Updates to the `IstioCNI` resource are performed as `Inplace` updates. This means the `DaemonSet` is updated with the new version of the CNI plugin when the resource changes, automatically replacing the existing istio-cni-node pods. The `IstioCNI` resource configuration field relevant to the upgrade process is:
- `spec.version`: The version of the CNI plugin to be installed. The value of this field takes the form “vX.Y.Z”, where “X.Y.Z” is the desired CNI plugin release. For example, if CNI plugin “1.24.3” is desired, the field should specify “v1.24.3”.

To update the CNI plugin, just change the `spec.version` field to the version you want to install. Just like the `Istio` resource, it also has a `values` field that exposes all of the options provided in the `istio-cni` chart.

### Updating the Istio-CNI resource version
When updating `IstioCNI`, keep in mind that the CNI plugin version `1.x` is compatible with Istio versions `1.x-1`, `1.x`, and `1.x+1`. You can upgrade the CNI plugin independently to the latest version without updating Istio, but you can't upgrade Istio to a version incompatible with the installed CNI plugin.

For example, If your Istio control plane is at `1.24.3` and the CNI plugin is at `1.24.2`, you can safely update the CNI plugin to `1.24.3`. However, you can't upgrade `Istio` to `1.24.4` without first updating the CNI plugin.

Prerequisites:
- You are logged in to OpenShift Container Platform as `cluster-admin`.
- You have installed the Red Hat OpenShift Service Mesh Operator 3, and deployed `IstioCNI`. For this example we are using the `IstioCNI` resource in the `istio-cni` namespace with the name `default`:
```yaml
apiVersion: sailoperator.io/v1
kind: IstioCNI
metadata:
  name: default
spec:
  version: v1.24.2
  namespace: istio-cni
```

To update the Istio-CNI control plane, follow these steps:
- Edit the `IstioCNI` resource to the desired version. For example, to update to IstioCNI `1.24.3`, set the `spec.version` field to `v1.24.3`.
`$ oc patch istiocni default -n istio-cni --type='merge' -p '{"spec":{"version":"v1.24.3"}}'` 
The Service Mesh Operator deploys a new version of the CNI plugin that replaces the old version of the CNI plugin. The `istio-cni-node` pods automatically reconnect to the new CNI plugin. The update process is complete.
- Confirm that the new version of the CNI plugin is running and ready by entering the following command
`$ oc get istiocni default`
You should see the new version of the CNI plugin running and ready.
```console
NAME      READY   STATUS    VERSION   AGE
default   True    Healthy   v1.24.3   91m
```
