[Return to Project Root](../)

# Table of Contents

- [User Documentation](#user-documentation)
- [Concepts](#concepts)
  - [Istio resource](#istio-resource)
  - [IstioRevision resource](#istiorevision-resource)
  - [IstioCNI resource](#istiocni-resource)
  - [RemoteIstio resource](#remoteistio-resource)
- [API Reference documentation](#api-reference-documentation)
- [Getting Started](#getting-started)
  - [Installation on OpenShift](#installation-on-openshift)
    - [Installing through the web console](#installing-through-the-web-console)
    - [Installing using the CLI](#installing-using-the-cli)
  - [Installation from Source](#installation-from-source)
- [Gateways](#gateways)
- [Update Strategy](#update-strategy)
  - [InPlace](#inplace)
    - [Example using the InPlace strategy](#example-using-the-inplace-strategy)
  - [RevisionBased](#revisionbased)
    - [Example using the RevisionBased strategy](#example-using-the-revisionbased-strategy)
- [Multi-cluster](#multi-cluster)
- [Addons](#addons)
  - [Deploy Prometheus and Jaeger addons](#deploy-prometheus-and-jaeger-addons)
  - [Deploy Kiali addon](#deploy-kiali-addon)
  - [Deploy Gateway and Bookinfo](#deploy-gateway-and-bookinfo)
  - [Generate traffic and visualize your mesh](#generate-traffic-and-visualize-your-mesh)
- [Observability Integrations](#observability-integrations)
  - [Scraping metrics using the OpenShift monitoring stack](#scraping-metrics-using-the-openshift-monitoring-stack)
  - [Integrating with Kiali](#integrating-with-kiali)
    - [Integrating Kiali with the OpenShift monitoring stack](#integrating-kiali-with-the-openshift-monitoring-stack)
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
The `Istio` resource is used to manage your Istio control planes. It is a cluster-wide resource, as the Istio control plane operates in and requires access to the entire cluster. To select a namespace to run the control plane pods in, you can use the `spec.namespace` field. You can access all helm chart options through the `values` field in the `spec`:

```yaml
apiVersion: sailoperator.io/v1alpha1
kind: Istio
metadata:
  name: default
spec:
  version: v1.22.3
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

To support canary updates of the control plane, Sail Operator includes support for multiple Istio versions. You can select a version by setting the `version` field in the `spec` to the version you would like to install, prefixed with a `v`. You can then update to a new version just by changing this field.

Sail Operator supports two different update strategies for your control planes: `InPlace` and `RevisionBased`. When using `InPlace`, the operator will immediately replace your existing control plane resources with the ones for the new version, whereas `RevisionBased` uses Istio's canary update mechanism by creating a second control plane to which you can migrate your workloads to complete the update.

After creation of an `Istio` resource, the Sail Operator will generate a revision name for it based on the updateStrategy that was chosen, and create a corresponding [`IstioRevision`](#istiorevision-resource).

### IstioRevision resource
The `IstioRevision` is the lowest-level API the Sail Operator provides, and it is usually not created by the user, but by the operator itself. It's schema closely resembles that of the `Istio` resource - but instead of representing the state of a control plane you want to be present in your cluster, it represents a *revision* of that control plane, which is an instance of Istio with a specific version and revision name, and its revision name can be used to add workloads or entire namespaces to the mesh, e.g. by using the `istio.io/rev=<REVISION_NAME>` label. It is also a cluster-wide resource.

You can think of the relationship between the `Istio` and `IstioRevision` resource as similar to the one between Kubernetes' `ReplicaSet` and `Pod`: a `ReplicaSet` can be created by users and results in the automatic creation of `Pods`, which will trigger the instantiation of your containers. Similarly, users create an `Istio` resource which instructs the operator to create a matching `IstioRevision`, which then in turn triggers the creation of the Istio control plane. To do that, the Sail Operator will copy all of your relevant configuration from the `Istio` resource to the `IstioRevision` resource.

### IstioCNI resource
The lifecycle of Istio's CNI plugin is managed separately when using Sail Operator. To install it, you can create an `IstioCNI` resource. The `IstioCNI` resource is a cluster-wide resource as it will install a `DaemonSet` that will be operating on all nodes of your cluster. You can select a version by setting the `spec.version` field, as you can see in the sample below. To update the CNI plugin, just change the `version` field to the version you want to install. Just like the `Istio` resource, it also has a `values` field that exposes all of the options provided in the `istio-cni` chart:

```yaml
apiVersion: sailoperator.io/v1alpha1
kind: IstioCNI
metadata:
  name: default
spec:
  version: v1.22.3
  namespace: istio-cni
  values:
    cni:
      cniConfDir: /etc/cni/net.d
      excludeNamespaces:
      - kube-system
```

### RemoteIstio resource
The `RemoteIstio` resource is used to connect the local cluster to an external Istio control plane. 
When you create a `RemoteIstio` resource, the operator deploys the `istiod-remote` Helm chart. 
Instead of deploying the entire Istio control plane, this chart deploys only the sidecar injector webhook, allowing you to inject the Istio proxy into your workloads and have this proxy managed by the Istio control plane running outside the cluster (typically in another Kubernetes cluster). 

The `RemoteIstio` resource is very similar to the `Istio` resource, with the most notable difference being the `istiodRemote` field in the `values` section, which allows you to configure the address of the remote Istio control plane:

```yaml
apiVersion: sailoperator.io/v1alpha1
kind: RemoteIstio
metadata:
  name: default
spec:
  version: v1.22.3
  namespace: istio-system
  updateStrategy:
    type: InPlace
  values:
    istiodRemote:
      injectionPath: /inject/cluster/cluster2/net/network1
    global:
      remotePilotAddress: 1.2.3.4
```

For more information on how to use the `RemoteIstio` resource, refer to the [multi-cluster](#multi-cluster) section.

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

## Gateways

The Sail Operator does not manage Gateways. You can deploy a gateway manually either through [gateway-api](https://istio.io/latest/docs/tasks/traffic-management/ingress/gateway-api/) or through [gateway injection](https://istio.io/latest/docs/setup/additional-setup/gateway/#deploying-a-gateway). As you are following the gateway installation instructions, skip the step to install Istio since this is handled by the Sail Operator.

**Note:** The `IstioOperator` / `istioctl` example is separate from the Sail Operator. Setting `spec.components` or `spec.values.gateways` on your Sail Operator `Istio` resource **will not work**.

## Update Strategy

The Sail Operator supports two update strategies to update the version of the Istio control plane: `InPlace` and `RevisionBased`. The default strategy is `InPlace`.

### InPlace
When the `InPlace` strategy is used, the existing Istio control plane is replaced with a new version. The workload sidecars immediately connect to the new control plane. The workloads therefore don't need to be moved from one control plane instance to another.

#### Example using the InPlace strategy

Prerequisites:
* Sail Operator is installed.
* `istioctl` is installed.

Steps:
1. Create the `istio-system` namespace.

    ```bash
    kubectl create namespace istio-system
    ```

2. Create the `Istio` resource.

    ```bash
    cat <<EOF | kubectl apply -f-
    apiVersion: sailoperator.io/v1alpha1
    kind: Istio
    metadata:
      name: default
    spec:
      namespace: istio-system
      updateStrategy:
        type: InPlace
      version: v1.21.0
    EOF
    ```

3. Confirm the installation and version of the control plane.

    ```console
    $ kubectl get istio -n istio-system
    NAME      READY   STATUS    IN USE   VERSION   AGE
    default   True    Healthy   True     v1.21.0   2m
    ```

4. Create namespace `bookinfo` and deploy bookinfo application.

    ```bash
    kubectl create namespace bookinfo
    kubectl label namespace bookinfo istio-injection=enabled
    kubectl apply -n bookinfo -f https://raw.githubusercontent.com/istio/istio/release-1.22/samples/bookinfo/platform/kube/bookinfo.yaml
    ```
    Note: if the `Istio` resource name is other than `default`, you need to set the `istio.io/rev` label to the name of the `Istio` resource instead of adding the `istio-injection=enabled` label.

5. Perform the update of the control plane by changing the version in the Istio resource.

    ```bash
    kubectl patch istio default -n istio-system --type='merge' -p '{"spec":{"version":"v1.21.2"}}'
    ```

6. Confirm the `Istio` resource version was updated.

    ```console
    $ kubectl get istio -n istio-system
    NAME      REVISIONS   READY   IN USE   ACTIVE REVISION   VERSION   AGE
    default   1           1       1        Healthy           v1.21.2   12m
    ```

7. Delete `bookinfo` pods to trigger sidecar injection with the new version.

    ```bash
    kubectl rollout restart deployment -n bookinfo
    ```

8. Confirm that the new version is used in the sidecar.

    ```bash
    istioctl proxy-status 
    ```
    The column `VERSION` should match the new control plane version.

### RevisionBased
When the `RevisionBased` strategy is used, a new Istio control plane instance is created for every change to the `Istio.spec.version` field. The old control plane remains in place until all workloads have been moved to the new control plane instance. This needs to be done by the user by updating the namespace label and restarting all the pods. The old control plane will be deleted after the grace period specified in the `Istio` resource field `spec.updateStrategy.inactiveRevisionDeletionGracePeriodSeconds`.

#### Example using the RevisionBased strategy

Prerequisites:
* Sail Operator is installed.
* `istioctl` is installed.

Steps:

1. Create the `istio-system` namespace.

    ```bash
    kubectl create namespace istio-system
    ```

2. Create the `Istio` resource.

    ```bash
    cat <<EOF | kubectl apply -f-
    apiVersion: sailoperator.io/v1alpha1
    kind: Istio
    metadata:
      name: default
    spec:
      namespace: istio-system
      updateStrategy:
        type: RevisionBased
        inactiveRevisionDeletionGracePeriodSeconds: 30
      version: v1.21.0
    EOF
    ```

3. Confirm the control plane is installed and is using the desired version.

    ```console
    $ kubectl get istio -n istio-system
    NAME      READY   STATUS    IN USE   VERSION   AGE
    default   True    Healthy   True     v1.21.0   2m
    ```

4. Get the `IstioRevision` name.

    ```console
    $ kubectl get istiorevision -n istio-system
    NAME              READY   STATUS    IN USE   VERSION   AGE
    default-v1-21-0   True    Healthy   False    v1.21.0   114s
    ```
    Note: `IstioRevision` name is in the format `<Istio resource name>-<version>`.

5. Create `bookinfo` namespace and label it with the revision name.

    ```bash
    kubectl create namespace bookinfo
    kubectl label namespace bookinfo istio.io/rev=default-v1-21-0
    ```

6. Deploy bookinfo application.

    ```bash
    kubectl apply -n bookinfo -f https://raw.githubusercontent.com/istio/istio/release-1.22/samples/bookinfo/platform/kube/bookinfo.yaml
    ```

7. Confirm that the proxy version matches the control plane version.

    ```bash
    istioctl proxy-status 
    ```
    The column `VERSION` should match the control plane version.

8. Update the control plane to a new version.

    ```bash
    kubectl patch istio default -n istio-system --type='merge' -p '{"spec":{"version":"v1.21.2"}}'
    ```

9. Verify the `Istio` and `IstioRevision` resources. There will be a new revision created with the new version.

    ```console
    $ kubectl get istio -n istio-system
    NAME      REVISIONS   READY   IN USE   ACTIVE REVISION   VERSION   AGE
    default   2           2       1        Healthy           v1.21.2   23m

    $ kubectl get istiorevision -n istio-system
    NAME              READY   STATUS    IN USE   VERSION   AGE
    default-v1-21-0   True    Healthy   True     v1.21.0   27m
    default-v1-21-2   True    Healthy   False    v1.21.2   4m45s
    ```

10. Confirm there are two control plane pods running, one for each revision.

    ```console
    $ kubectl get pods -n istio-system
    NAME                                      READY   STATUS    RESTARTS   AGE
    istiod-default-v1-21-0-69d6df7f9c-grm24   1/1     Running   0          28m
    istiod-default-v1-21-2-7c4f4674c5-4g7n7   1/1     Running   0          6m9s
    ```

11. Confirm the proxy sidecar version remains the same:

    ```bash
    istioctl proxy-status 
    ```
    The column `VERSION` should still match the old control plane version.

12. Change the label of the `bookinfo` namespace to use the new revision.

    ```bash
    kubectl label namespace bookinfo istio.io/rev=default-v1-21-2 --overwrite
    ```
    The existing workload sidecars will continue to run and will remain connected to the old control plane instance. They will not be replaced with a new version until the pods are deleted and recreated.

13. Delete all the pods in the `bookinfo` namespace.

    ```bash
    kubectl rollout restart deployment -n bookinfo
    ```

14. Confirm the new version is used in the sidecars.

    ```bash
    istioctl proxy-status 
    ```
    The column `VERSION` should match the updated control plane version.

15. Confirm the old control plane and revision deletion.

    ```console
    $ kubectl get pods -n istio-system
    NAME                                      READY   STATUS    RESTARTS   AGE
    istiod-default-v1-21-2-7c4f4674c5-4g7n7   1/1     Running   0          94m

    $ kubectl get istiorevision -n istio-system
    NAME              READY   STATUS    IN USE   VERSION   AGE
    default-v1-21-2   True    Healthy   True     v1.21.2   94m
    ```
    The old `IstioRevision` resource and the old control plane will be deleted when the grace period specified in the `Istio` resource field `spec.updateStrategy.inactiveRevisionDeletionGracePeriodSeconds` expires.

## Multi-cluster

You can use the Sail Operator and the Sail CRDs to manage a multi-cluster Istio deployment. The following instructions are adapted from the [Istio multi-cluster documentation](https://istio.io/latest/docs/setup/install/multicluster/) to demonstrate how you can setup the various deployment models with Sail. Please familiarize yourself with the different [deployment models](https://istio.io/latest/docs/ops/deployment/deployment-models/) before starting.

*Prerequisites*

Each deployment model requires you to install the Sail Operator and the Sail CRDs to every cluster that is part of the mesh.

- Install [istioctl](https://istio.io/latest/docs/setup/install/istioctl) and have it included in your `$PATH`.
- Two kubernetes clusters with external lb support. (If using kind, `cloud-provider-kind` is running in the background)
- kubeconfig file with a context for each cluster.

### Common Setup

These steps are common to every multi-cluster deployment and should be completed *after* meeting the prerequisites but *before* starting on a specific deployment model.

1. Setup env vars.

    ```sh
    export CTX_CLUSTER1=<cluster1-ctx>
    export CTX_CLUSTER2=<cluster2-ctx>
    export ISTIO_VERSION=1.23.0
    ```

2. Create `istio-system` namespace on each cluster.

    ```sh
    kubectl get ns istio-system --context "${CTX_CLUSTER1}" || kubectl create namespace istio-system --context "${CTX_CLUSTER1}"
    ```

    ```sh
    kubectl get ns istio-system --context "${CTX_CLUSTER2}" || kubectl create namespace istio-system --context "${CTX_CLUSTER2}"
    ```

2. Create shared trust and add intermediate CAs to each cluster.

    If you already have a [shared trust](https://istio.io/latest/docs/setup/install/multicluster/before-you-begin/#configure-trust) for each cluster you can skip this. Otherwise, you can use the instructions below to create a shared trust and push the intermediate CAs into your clusters.

    Create a self signed root CA and intermediate CAs.
    ```sh
    mkdir -p certs
    pushd certs
    curl -fsL -o common.mk "https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/tools/certs/common.mk"
    curl -fsL -o Makefile.selfsigned.mk "https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/tools/certs/Makefile.selfsigned.mk"
    make -f Makefile.selfsigned.mk root-ca
    make -f Makefile.selfsigned.mk east-cacerts
    make -f Makefile.selfsigned.mk west-cacerts
    popd
    ```

    Push the intermediate CAs to each cluster.
    ```sh
    kubectl --context "${CTX_CLUSTER1}" label namespace istio-system topology.istio.io/network=network1
    kubectl get secret -n istio-system --context "${CTX_CLUSTER1}" cacerts || kubectl create secret generic cacerts -n istio-system --context "${CTX_CLUSTER1}" \
      --from-file=certs/east/ca-cert.pem \
      --from-file=certs/east/ca-key.pem \
      --from-file=certs/east/root-cert.pem \
      --from-file=certs/east/cert-chain.pem
    kubectl --context "${CTX_CLUSTER2}" label namespace istio-system topology.istio.io/network=network2
    kubectl get secret -n istio-system --context "${CTX_CLUSTER2}" cacerts || kubectl create secret generic cacerts -n istio-system --context "${CTX_CLUSTER2}" \
      --from-file=certs/west/ca-cert.pem \
      --from-file=certs/west/ca-key.pem \
      --from-file=certs/west/root-cert.pem \
      --from-file=certs/west/cert-chain.pem
    ```

### Multi-Primary - Multi-Network

These instructions install a [multi-primary/multi-network](https://istio.io/latest/docs/setup/install/multicluster/multi-primary_multi-network/) Istio deployment using the Sail Operator and Sail CRDs. **Before you begin**, ensure you complete the [common setup](#common-setup).

You can follow the steps below to install manually or you can run [this script](multicluster/setup-multi-primary.sh) which will setup a local environment for you with kind. Before running the setup script, you must install [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) and [cloud-provider-kind](https://kind.sigs.k8s.io/docs/user/loadbalancer/#installing-cloud-provider-kind) then ensure the `cloud-provider-kind` binary is running in the background.

These installation instructions are adapted from: https://istio.io/latest/docs/setup/install/multicluster/multi-primary_multi-network/. 

1. Create an `Istio` resource on `cluster1`.

    ```sh
    kubectl apply --context "${CTX_CLUSTER1}" -f - <<EOF
    apiVersion: sailoperator.io/v1alpha1
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
    kubectl wait --context "${CTX_CLUSTER1}" --for=condition=Ready istios/default --timeout=3m
    ```

2. Create east-west gateway on `cluster1`.

    ```sh
    kubectl apply --context "${CTX_CLUSTER1}" -f https://raw.githubusercontent.com/istio-ecosystem/sail-operator/main/docs/multicluster/east-west-gateway-net1.yaml
    ```

3. Expose services on `cluster1`.

    ```sh
    kubectl --context "${CTX_CLUSTER1}" apply -n istio-system -f https://raw.githubusercontent.com/istio-ecosystem/sail-operator/main/docs/multicluster/expose-services.yaml
    ```

4. Create `Istio` resource on `cluster2`.

    ```sh
    kubectl apply --context "${CTX_CLUSTER2}" -f - <<EOF
    apiVersion: sailoperator.io/v1alpha1
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
    kubectl wait --context "${CTX_CLUSTER2}" --for=jsonpath='{.status.revisions.ready}'=1 istios/default --timeout=3m
    ```

5. Create east-west gateway on `cluster2`.

    ```sh
    kubectl apply --context "${CTX_CLUSTER2}" -f https://raw.githubusercontent.com/istio-ecosystem/sail-operator/main/docs/multicluster/east-west-gateway-net2.yaml
    ```

6. Expose services on `cluster2`.

    ```sh
    kubectl --context "${CTX_CLUSTER2}" apply -n istio-system -f https://raw.githubusercontent.com/istio-ecosystem/sail-operator/main/docs/multicluster/expose-services.yaml
    ```

7. Install a remote secret in `cluster2` that provides access to the `cluster1` API server.

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

8. Install a remote secret in `cluster1` that provides access to the `cluster2` API server.

    ```sh
    istioctl create-remote-secret \
      --context="${CTX_CLUSTER2}" \
      --name=cluster2 | \
      kubectl apply -f - --context="${CTX_CLUSTER1}"
    ```

    If using kind, first get the `cluster1` controlplane IP and pass the `--server` option to `istioctl create-remote-secret`

    ```sh
    CLUSTER2_CONTAINER_IP=$(kubectl get nodes -l node-role.kubernetes.io/control-plane --context "${CTX_CLUSTER2}" -o jsonpath='{.items[0].status.addresses[?(@.type == "InternalIP")].address}')
    istioctl create-remote-secret \
      --context="${CTX_CLUSTER2}" \
      --name=cluster2 \
      --server="https://${CLUSTER2_CONTAINER_IP}:6443" | \
      kubectl apply -f - --context "${CTX_CLUSTER1}"
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

11. Verify that you see a response from both v1 and v2.

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

### Primary-Remote - Multi-Network

These instructions install a [primary-remote/multi-network](https://istio.io/latest/docs/setup/install/multicluster/primary-remote_multi-network/) Istio deployment using the Sail Operator and Sail CRDs. **Before you begin**, ensure you complete the [common setup](#common-setup).

These installation instructions are adapted from: https://istio.io/latest/docs/setup/install/multicluster/primary-remote_multi-network/.

In this setup there is a Primary cluster (`cluster1`) and a Remote cluster (`cluster2`) which are on separate networks.

1. Create an `Istio` resource on `cluster1`.

    ```sh
    kubectl apply --context "${CTX_CLUSTER1}" -f - <<EOF
    apiVersion: sailoperator.io/v1alpha1
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

5. Create `RemoteIstio` resource on `cluster2`.

    ```sh
    kubectl apply --context "${CTX_CLUSTER2}" -f - <<EOF
    apiVersion: sailoperator.io/v1alpha1
    kind: RemoteIstio
    metadata:
      name: default
    spec:
      version: v${ISTIO_VERSION}
      namespace: istio-system
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
test True    Healthy   True     v1.21.0   119m
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