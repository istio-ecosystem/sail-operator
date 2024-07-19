[Return to Project Root](../)

# Table of Contents

- [User Documentation](#user-documentation)
- [Concepts](#concepts)
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
- [Multicluster](#multicluster)
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
  - [Deleting the sail-operator](#deleting-the-sail-operator)
  - [Deleting the istio-system and istio-cni Projects](#deleting-the-istio-system-and-istiocni-projects)
  - [Decide whether you want to delete the CRDs as well](#decide-whether-you-want-to-delete-the-crds-as-well)


# User Documentation
tbd

## Concepts
tbd

## Getting Started

### Installation on OpenShift

#### Installing through the web console

1. In the OpenShift Console, navigate to the OperatorHub by clicking **Operator** -> **Operator Hub** in the left side-pane.

1. Search for "sail".

1. Locate the sail-operator, and click to select it.

1. When the prompt that discusses the community operator appears, click **Continue**, then click **Install**.

1. Use the default installation settings presented, and click **Install** to continue.

1. Click **Operators** -> **Installed Operators** to verify that the sail-operator 
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

    ```yaml
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
    ```

1. Verify that the installation succeeded by inspecting the CSV file.

    ```bash
    $ kubectl get csv -n openshift-operators
    NAME                                     DISPLAY         VERSION                    REPLACES                                 PHASE
    sailoperator.v0.1.0-nightly-2024-06-25   Sail Operator   0.1.0-nightly-2024-06-25   sailoperator.v0.1.0-nightly-2024-06-21   Succeeded
    ```

    `Succeeded` should appear in the sailoperator's `PHASE` column.

### Installation from Source

If you're not using OpenShift or simply want to install from source, follow the [instructions in the Contributor Documentation](../README.md#deploying-the-operator).

## Gateways

The Sail-operator does not manage Gateways. You can deploy a gateway manually either through [gateway-api](https://istio.io/latest/docs/tasks/traffic-management/ingress/gateway-api/) or through [gateway injection](https://istio.io/latest/docs/setup/additional-setup/gateway/#deploying-a-gateway). As you are following the gateway installation instructions, skip the step to install Istio since this is handled by the Sail-operator.

**Note:** The `IstioOperator` / `istioctl` example is separate from the Sail-operator. Setting `spec.components` or `spec.values.gateways` on your Sail-operator `Istio` resource **will not work**.

## Update Strategy

The sail-operator supports two update strategies to update the istio version of the control plane: `InPlace` and `RevisionBased`. The default strategy is `InPlace`.

### InPlace
When the `InPlace` strategy is used, the existing Istio control plane is updated in-place. The workloads therefore don't need to be moved from one control plane instance to another.

#### Example using the InPlace strategy

Prerequisites:
* sail-operator needs to be installed.

Steps:
1. Create the `istio-system` and the `istio-cni` namespaces.

    ```bash
    kubectl create namespace istio-system
    kubectl create namespace istio-cni
    ```

2. Create the `Istio-cni` resource
  
      ```bash
      cat <<EOF | kubectl apply -f -
      apiVersion: operator.istio.io/v1alpha1
      kind: IstioCNI
      metadata:
        name: default
      spec:
        version: v1.21.0
        namespace: istio-cni
      EOF
      ```

3. Create the `Istio` resource

    ```bash
    cat <<EOF | kubectl apply -f-
    apiVersion: operator.istio.io/v1alpha1
    kind: Istio
    metadata:
      name: default
    spec:
      namespace: istio-system
      updateStrategy:
        type: InPlace
        inactiveRevisionDeletionGracePeriodSeconds: 30
      version: v1.21.0
    EOF
    ```

4. Confirm the installation and version of the control plane.

    ```bash
    $ kubectl get istio -n istio-system
    NAME      READY   STATUS    IN USE   VERSION   AGE
    default   True    Healthy   True     v1.21.0   2m
    ```

5. Create namespace `bookinfo` and deploy bookinfo application

    ```bash
    kubectl create namespace bookinfo
    kubectl label namespace bookinfo istio.io/rev=default
    kubectl apply -n bookinfo -f https://raw.githubusercontent.com/istio/istio/release-1.22/samples/bookinfo/platform/kube/bookinfo.yaml
    ```

6. Perform the update of the control plane by changing the version in the Istio resource

    ```bash
    cat <<EOF | kubectl apply -f-
    apiVersion: operator.istio.io/v1alpha1
    kind: Istio
    metadata:
      name: default
    spec:
      namespace: istio-system
      updateStrategy:
        type: InPlace
        inactiveRevisionDeletionGracePeriodSeconds: 30
      version: v1.21.2
    EOF
    ```

7. Confirm the istio resource version was updated

    ```bash
    kubectl get istio -n istio-system
    NAME      REVISIONS   READY   IN USE   ACTIVE REVISION   VERSION   AGE
    default   1           1       1        Healthy           v1.21.2   12m
    ```

8. Delete `bookinfo` pod to trigger sidecar injection with the new version

    ```bash
    kubectl delete pods --all -n bookinfo
    ```

9. Confirm that the new version is used in the sidecar

    ```bash
    kubectl get pod -n bookinfo -l app=details -o yaml | yq '.items[].spec.containers[].image'
    docker.io/istio/examples-bookinfo-details-v1:1.19.1
    docker.io/istio/proxyv2:1.21.2
    ```

### RevisionBased
When the `RevisionBased` strategy is used, a new Istio control plane instance is created for every change to the Istio.spec.version field. The old control plane remains in place until all workloads have been moved to the new control plane instance, then, the old control plane is deleted.

#### Example using the RevisionBased strategy

Prerequisites:
* sail-operator needs to be installed.

Steps:

1. Create the `istio-system` and the `istio-cni` namespaces.

    ```bash
    kubectl create namespace istio-system
    kubectl create namespace istio-cni
    ```

2. Create the `Istio-cni` resource
  
      ```bash
      cat <<EOF | kubectl apply -f -
      apiVersion: operator.istio.io/v1alpha1
      kind: IstioCNI
      metadata:
        name: default
      spec:
        version: v1.21.0
        namespace: istio-cni
      EOF
      ```

3. Create the `Istio` resource

    ```bash
    cat <<EOF | kubectl apply -f-
    apiVersion: operator.istio.io/v1alpha1
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

4. Confirm the control plane is installed and is using the desired version.

    ```bash
    $ kubectl get istio -n istio-system
    NAME      READY   STATUS    IN USE   VERSION   AGE
    default   True    Healthy   True     v1.21.0   2m
    ```

5. Get the `istiorevision` name

    ```bash
    $ kubectl get istiorevision -n istio-system
    NAME              READY   STATUS    IN USE   VERSION   AGE
    default-v1-21-0   True    Healthy   False    v1.21.0   114s
    ```

6. Create `bookinfo` namespace and label it with the revision name

    ```bash
    kubectl create namespace bookinfo
    kubectl label namespace bookinfo istio.io/rev=default-v1-21-0
    ```

7. Deploy bookinfo application

    ```bash
    kubectl apply -n bookinfo -f https://raw.githubusercontent.com/istio/istio/release-1.22/samples/bookinfo/platform/kube/bookinfo.yaml
    ```

8. Confirm proxy sidecar injection version match the control plane version

    ```bash
    kubectl get pod -n bookinfo -l app=details -o yaml | yq '.items[].spec.containers[].image'
    docker.io/istio/examples-bookinfo-details-v1:1.19.1
    docker.io/istio/proxyv2:1.21.0
    ```

9. Update the control plane to a new version

    ```bash
    cat <<EOF | kubectl apply -f-
    apiVersion: operator.istio.io/v1alpha1
    kind: Istio
    metadata:
      name: default
    spec:
      namespace: istio-system
      updateStrategy:
        type: RevisionBased
        inactiveRevisionDeletionGracePeriodSeconds: 30
      version: v1.21.2
    EOF
    ```

10. Verify the `Istio` and `IstioRevision` resources. There will be a new revision created with the new version.

    ```bash
    $ kubectl get istio -n istio-system
      NAME      REVISIONS   READY   IN USE   ACTIVE REVISION   VERSION   AGE
      default   2           2       1        Healthy           v1.21.2   23m
    $ kubectl get istiorevision -n istio-system
      NAME              READY   STATUS    IN USE   VERSION   AGE
      default-v1-21-0   True    Healthy   True     v1.21.0   27m
      default-v1-21-2   True    Healthy   False    v1.21.2   4m45s
    ```

11. Confirm there is two control plane pods running, one for each revision

    ```bash
    $ kubectl get pods -n istio-system
      NAME                                      READY   STATUS    RESTARTS   AGE
      istiod-default-v1-21-0-69d6df7f9c-grm24   1/1     Running   0          28m
      istiod-default-v1-21-2-7c4f4674c5-4g7n7   1/1     Running   0          6m9s
    ```

12. Confirm the proxy sidecar version remains the same:

    ```bash
    $ kubectl get pod -n bookinfo -l app=details -o yaml | yq '.items[].spec.containers[].image'
      docker.io/istio/examples-bookinfo-details-v1:1.19.1
      docker.io/istio/proxyv2:1.21.0
    ```

13. Change the label of the `bookinfo` namespace

    ```bash
    kubectl label namespace bookinfo istio.io/rev=default-v1-21-2 --overwrite
    ```

The control plane used by the pods will be updated to the new version only after all workloads are moved to the new revision, and this will happen after the pods are restarted.

14. Delete all the pods in the `bookinfo` namespace

    ```bash
    kubectl delete pods --all -n bookinfo
    ```

15. Confirm the new version is used in the sidecars

    ```bash
    kubectl get pod -n bookinfo -l app=details -o yaml | yq '.items[].spec.containers[].image'
    docker.io/istio/examples-bookinfo-details-v1:1.19.1
    docker.io/istio/proxyv2:1.21.2
    ```

16. Confirm the old control plane deletion

    ```bash
    kubectl get pods -n istio-system
    NAME                                      READY   STATUS    RESTARTS   AGE
    istiod-default-v1-21-2-7c4f4674c5-4g7n7   1/1     Running   0          94m
    ```

17. Confirm the old revision deletion

    ```bash
    kubectl get istiorevision -n istio-system
    NAME              READY   STATUS    IN USE   VERSION   AGE
    default-v1-21-2   True    Healthy   True     v1.21.2   94m
    ```

## Multicluster
tbd

## Addons

Addons are managed separately from the Sail-operator. You can follow the [istio documentation](https://istio.io/latest/docs/ops/integrations/) for how to install addons. Below is an example of how to install some addons for Istio.

The sample will deploy:

- Prometheus
- Jaeger
- Kiali
- Bookinfo demo app

*Prerequisites*

- Sail operator installed.
- Control Plane installed with Sail.

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
$ kubectl get istiorevisions.operator.istio.io 
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

Create the bookinfo namespace and enable injection.

```sh
kubectl get namespace bookinfo || kubectl create namespace bookinfo
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

Otherwise port forward to the kiali pod directly:

```sh
kubectl port-forward -n istio-system svc/kiali 20001:20001
```

You can view Kiali dashboard at: http://localhost:20001

## Observability Integrations

### Scraping metrics using the OpenShift monitoring stack
The easiest way to get started with production-grade metrics collection is to use OpenShift's user-workload monitoring stack. The following steps assume that you installed Istio into the `istio-system` namespace. Note that these steps are not specific to the `sail-operator`, but describe how to configure user-workload monitoring for Istio in general.

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
Integration with Kiali really depends on how you collect your metrics and traces. Note that Kiali is a separate project which for the purpose of this document we'll expect is installed using the Kiali operator. The steps here are not specific to `sail-operator`, but describe how to configure Kiali for use with Istio in general.

#### Integrating Kiali with the OpenShift monitoring stack
If you followed [Scraping metrics using the OpenShift monitoring stack](#scraping-metrics-using-the-openshift-monitoring-stack), you can setup Kiali to retrieve metrics from there.

*Prerequisites*
* User Workload monitoring is [enabled](https://docs.openshift.com/container-platform/latest/observability/monitoring/enabling-monitoring-for-user-defined-projects.html) and [configured](#scraping-metrics-using-the-openshift-monitoring-stack)
* Kiali Operator is installed

*Steps*
1. Create a ClusterRoleBinding for Kiali so it can view metrics from user-workload monitoring

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
    $ kubectl get istiorevisions.operator.istio.io 
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

### Deleting the sail-operator
1. In the OpenShift Container Platform web console, click **Operators** -> **Installed Operators**.
1. Locate the sail-operator. Click the Options menu, and select **Uninstall Operator**.
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
