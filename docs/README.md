[Return to Project Root](../)

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

## Multicluster

Currently, only the [multi-primary](https://istio.io/latest/docs/setup/install/multicluster/multi-primary) deployment model is supported with the Sail-operator. Support for primary-remote and external-control-plane deployments is in progress.

Each deployment model requires you to install the Sail-operator and its CRDs to every cluster that is part of the mesh.

### Multi-primary - multi-network

*Prerequisites*

- [istioctl](https://istio.io/latest/docs/setup/install/istioctl)
- Two clusters with external lb support. (If using kind, `cloud-provider-kind` is running in the background)
- Sail operator and Sail CRDs installed on each cluster.

These installation instructions are adapted from: https://istio.io/latest/docs/setup/install/multicluster/multi-primary_multi-network/.
You can follow the steps below to install manually or you can run [this script](multicluster/setup-multi-primary.sh) which will setup a local environment for you with kind. Before running the setup script, you must install [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) and [cloud-provider-kind](https://kind.sigs.k8s.io/docs/user/loadbalancer/#installing-cloud-provider-kind) then ensure the `cloud-provider-kind` binary is running in the background.

1. Setup env vars

    ```sh
    export CTX_CLUSTER1=<cluster1-ctx>
    export CTX_CLUSTER2=<cluster2-ctx>
    export ISTIO_VERSION=1.22.0
    ```

2. Create `istio-system` namespace on each cluster

    ```sh
    kubectl get ns istio-system --context "${CTX_CLUSTER1}" || kubectl create namespace istio-system --context "${CTX_CLUSTER1}"
    ```

    ```sh
    kubectl get ns istio-system --context "${CTX_CLUSTER2}" || kubectl create namespace istio-system --context "${CTX_CLUSTER2}"
    ```

3. Create shared trust and add intermediate CAs to each cluster.

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

4. Create Sail CR on east

    ```sh
    kubectl apply --context "${CTX_CLUSTER1}" -f - <<EOF
    apiVersion: operator.istio.io/v1alpha1
    kind: Istio
    metadata:
      name: default
    spec:
      version: v${ISTIO_VERSION}
      namespace: istio-system
      values:
        pilot:
          resources:
            requests:
              cpu: 100m
              memory: 1024Mi
        global:
          meshID: mesh1
          multiCluster:
            clusterName: east
          network: network1
    EOF
    kubectl wait --context "${CTX_CLUSTER1}" --for=jsonpath='{.status.revisions.ready}'=1 istios/default --timeout=3m
    ```

5. Create east-west gateway on east

    ```sh
    curl -fsL "https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/multicluster/gen-eastwest-gateway.sh" | \
      bash -s -- --mesh mesh1 --cluster east --network network1 | \
      istioctl manifest generate -f - | \
      kubectl apply --context "${CTX_CLUSTER1}" -f -
    ```

6. Expose services on east

    ```sh
    kubectl --context "${CTX_CLUSTER1}" apply -n istio-system -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/multicluster/expose-services.yaml
    ```

7. Create Sail CR on west

    ```sh
    kubectl apply --context "${CTX_CLUSTER2}" -f - <<EOF
    apiVersion: operator.istio.io/v1alpha1
    kind: Istio
    metadata:
      name: default
    spec:
      version: v${ISTIO_VERSION}
      namespace: istio-system
      values:
        pilot:
          resources:
            requests:
              cpu: 100m
              memory: 1024Mi
        global:
          meshID: mesh1
          multiCluster:
            clusterName: west
          network: network2
    EOF
    kubectl wait --context "${CTX_CLUSTER2}" --for=jsonpath='{.status.revisions.ready}'=1 istios/default --timeout=3m
    ```

8. Create east-west gateway on west

    ```sh
    curl -fsL "https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/multicluster/gen-eastwest-gateway.sh" | \
      bash -s -- --mesh mesh1 --cluster west --network network2 | \
      istioctl manifest generate -f - | \
      kubectl apply --context "${CTX_CLUSTER2}" -f -
    ```

9. Expose services on west

    ```sh
    kubectl --context "${CTX_CLUSTER2}" apply -n istio-system -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/multicluster/expose-services.yaml
    ```

10. Install a remote secret in west that provides access to east’s API server.

    ```sh
    istioctl create-remote-secret \
      --context="${CTX_CLUSTER1}" \
      --name=cluster1 | \
      kubectl apply -f - --context="${CTX_CLUSTER2}"
    ```

    If using kind, first get the east controlplane ip and pass the `--server` option to `istioctl create-remote-secret`

    ```sh
    WEST_CONTAINER_IP=$(kubectl get nodes east-control-plane --context "${CTX_CLUSTER1}" -o jsonpath='{.status.addresses[?(@.type == "InternalIP")].address}')
    istioctl create-remote-secret \
      --context="${CTX_CLUSTER1}" \
      --name=east \
      --server="https://${WEST_CONTAINER_IP}:6443" | \
      kubectl apply -f - --context "${CTX_CLUSTER2}"
    ```

11. Install a remote secret in east that provides access to west’s API server.

    ```sh
    istioctl create-remote-secret \
      --context="${CTX_CLUSTER1}" \
      --name=cluster1 | \
      kubectl apply -f - --context="${CTX_CLUSTER2}"
    ```

    If using kind, first get the east controlplane ip and pass the `--server` option to `istioctl create-remote-secret`

    ```sh
    EAST_CONTAINER_IP=$(kubectl get nodes west-control-plane --context "${CTX_CLUSTER2}" -o jsonpath='{.status.addresses[?(@.type == "InternalIP")].address}')
    istioctl create-remote-secret \
      --context="${CTX_CLUSTER2}" \
      --name=west \
      --server="https://${EAST_CONTAINER_IP}:6443" | \
      kubectl apply -f - --context "${CTX_CLUSTER1}"
    ```

12. Deploy sample applications to east cluster.

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

13. Deploy sample applications to west cluster.

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

14. Verify that you see a response from both v1 and v2.

    east cluster responds with v1 and v2
    ```sh
    kubectl exec --context="${CTX_CLUSTER1}" -n sample -c sleep \
        "$(kubectl get pod --context="${CTX_CLUSTER1}" -n sample -l \
        app=sleep -o jsonpath='{.items[0].metadata.name}')" \
        -- curl -sS helloworld.sample:5000/hello
    ```

    west cluster responds with v1 and v2
    ```sh
    kubectl exec --context="${CTX_CLUSTER2}" -n sample -c sleep \
        "$(kubectl get pod --context="${CTX_CLUSTER2}" -n sample -l \
        app=sleep -o jsonpath='{.items[0].metadata.name}')" \
        -- curl -sS helloworld.sample:5000/hello
    ```

15. Cleanup

    ```sh
    kubectl delete istios default --context="${CTX_CLUSTER1}"
    kubectl delete ns istio-system --context="${CTX_CLUSTER1}" 
    kubectl delete ns sample --context="${CTX_CLUSTER1}"
    kubectl delete istios default --context="${CTX_CLUSTER2}"
    kubectl delete ns istio-system --context="${CTX_CLUSTER2}" 
    kubectl delete ns sample --context="${CTX_CLUSTER2}"
    ```


## Examples
tbd

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
