[Return to Project Root](../README.md)

# Table of Contents

- [Multi-cluster](#multi-cluster)
  - [Prerequisites](#prerequisites)
  - [Common Setup](#common-setup)
  - [Multi-Primary - Multi-Network](#multi-primary---multi-network)
  - [Primary-Remote - Multi-Network](#primary-remote---multi-network)
  - [External Control Plane](#external-control-plane)

## Multi-cluster

You can use the Sail Operator and the Sail CRDs to manage a multi-cluster Istio deployment. The following instructions are adapted from the [Istio multi-cluster documentation](https://istio.io/latest/docs/setup/install/multicluster/) to demonstrate how you can setup the various deployment models with Sail. Please familiarize yourself with the different [deployment models](https://istio.io/latest/docs/ops/deployment/deployment-models/) before starting.

### Prerequisites

- Install [istioctl](../common/install-istioctl-tool.md).
- Two kubernetes clusters with external lb support. (If using kind, `cloud-provider-kind` is running in the background)
- kubeconfig file with a context for each cluster.
- Install the Sail Operator and the Sail CRDs to every cluster.

### Common Setup

These steps are common to every multi-cluster deployment and should be completed *after* meeting the prerequisites but *before* starting on a specific deployment model.

1. Setup environment variables.

    ```bash
    export CTX_CLUSTER1=<cluster1-ctx>
    export CTX_CLUSTER2=<cluster2-ctx>
    export ISTIO_VERSION=1.26.0
    ```

2. Create `istio-system` namespace on each cluster.

    ```bash
    kubectl get ns istio-system --context "${CTX_CLUSTER1}" || kubectl create namespace istio-system --context "${CTX_CLUSTER1}"
    kubectl get ns istio-system --context "${CTX_CLUSTER2}" || kubectl create namespace istio-system --context "${CTX_CLUSTER2}"
    ```

4. Create a shared root certificate.

    If you have [established trust](https://istio.io/latest/docs/setup/install/multicluster/before-you-begin/#configure-trust) between your clusters already you can skip this and the following steps.

    ```bash
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
5. Create intermediate certificates.

    ```bash
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
    ```bash
    kubectl get secret -n istio-system --context "${CTX_CLUSTER1}" cacerts || kubectl create secret generic cacerts -n istio-system --context "${CTX_CLUSTER1}" \
      --from-file=east/ca-cert.pem \
      --from-file=east/ca-key.pem \
      --from-file=east/root-cert.pem \
      --from-file=east/cert-chain.pem
    kubectl get secret -n istio-system --context "${CTX_CLUSTER2}" cacerts || kubectl create secret generic cacerts -n istio-system --context "${CTX_CLUSTER2}" \
      --from-file=west/ca-cert.pem \
      --from-file=west/ca-key.pem \
      --from-file=west/root-cert.pem \
      --from-file=west/cert-chain.pem
    ```

### Multi-Primary - Multi-Network

These instructions install a [multi-primary/multi-network](https://istio.io/latest/docs/setup/install/multicluster/multi-primary_multi-network/) Istio deployment using the Sail Operator and Sail CRDs. **Before you begin**, ensure you complete the [common setup](#common-setup).

You can follow the steps below to install manually or you can run [this script](resources/setup-multi-primary.sh) which will setup a local environment for you with kind. Before running the setup script, you must install [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) and [cloud-provider-kind](https://kind.sigs.k8s.io/docs/user/loadbalancer/#installing-cloud-provider-kind) then ensure the `cloud-provider-kind` binary is running in the background.

These installation instructions are adapted from: https://istio.io/latest/docs/setup/install/multicluster/multi-primary_multi-network/. 

1. Create an `Istio` resource on `cluster1`.

    ```bash
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

    ```bash
    kubectl wait --context "${CTX_CLUSTER1}" --for=condition=Ready istios/default --timeout=3m
    ```

3. Create east-west gateway on `cluster1`.

    ```bash
    kubectl apply --context "${CTX_CLUSTER1}" -f https://raw.githubusercontent.com/istio-ecosystem/sail-operator/main/docs/deployment-models/resources/east-west-gateway-net1.yaml
    ```

4. Expose services on `cluster1`.

    ```bash
    kubectl --context "${CTX_CLUSTER1}" apply -n istio-system -f https://raw.githubusercontent.com/istio-ecosystem/sail-operator/main/docs/deployment-models/resources/expose-services.yaml
    ```

5. Create `Istio` resource on `cluster2`.

    ```bash
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

    ```bash
    kubectl wait --context "${CTX_CLUSTER2}" --for=jsonpath='{.status.revisions.ready}'=1 istios/default --timeout=3m
    ```

7. Create east-west gateway on `cluster2`.

    ```bash
    kubectl apply --context "${CTX_CLUSTER2}" -f https://raw.githubusercontent.com/istio-ecosystem/sail-operator/main/docs/deployment-models/resources/east-west-gateway-net2.yaml
    ```

8. Expose services on `cluster2`.

    ```bash
    kubectl --context "${CTX_CLUSTER2}" apply -n istio-system -f https://raw.githubusercontent.com/istio-ecosystem/sail-operator/main/docs/deployment-models/resources/expose-services.yaml
    ```

9. Install a remote secret in `cluster2` that provides access to the `cluster1` API server.

    ```bash
    istioctl create-remote-secret \
      --context="${CTX_CLUSTER1}" \
      --name=cluster1 | \
      kubectl apply -f - --context="${CTX_CLUSTER2}"
    ```

    **If using kind**, first get the `cluster1` controlplane ip and pass the `--server` option to `istioctl create-remote-secret`.

    ```bash
    CLUSTER1_CONTAINER_IP=$(kubectl get nodes -l node-role.kubernetes.io/control-plane --context "${CTX_CLUSTER1}" -o jsonpath='{.items[0].status.addresses[?(@.type == "InternalIP")].address}')
    istioctl create-remote-secret \
      --context="${CTX_CLUSTER1}" \
      --name=cluster1 \
      --server="https://${CLUSTER1_CONTAINER_IP}:6443" | \
      kubectl apply -f - --context "${CTX_CLUSTER2}"
    ```

10. Install a remote secret in `cluster1` that provides access to the `cluster2` API server.

    ```bash
    istioctl create-remote-secret \
      --context="${CTX_CLUSTER2}" \
      --name=cluster2 | \
      kubectl apply -f - --context="${CTX_CLUSTER1}"
    ```

    **If using kind**, first get the `cluster1` controlplane IP and pass the `--server` option to `istioctl create-remote-secret`

    ```bash
    CLUSTER2_CONTAINER_IP=$(kubectl get nodes -l node-role.kubernetes.io/control-plane --context "${CTX_CLUSTER2}" -o jsonpath='{.items[0].status.addresses[?(@.type == "InternalIP")].address}')
    istioctl create-remote-secret \
      --context="${CTX_CLUSTER2}" \
      --name=cluster2 \
      --server="https://${CLUSTER2_CONTAINER_IP}:6443" | \
      kubectl apply -f - --context "${CTX_CLUSTER1}"
    ```

11. Create sample application namespaces in each cluster.

    ```bash
    kubectl get ns sample --context "${CTX_CLUSTER1}" || kubectl create --context="${CTX_CLUSTER1}" namespace sample
    kubectl label --context="${CTX_CLUSTER1}" namespace sample istio-injection=enabled
    kubectl get ns sample --context "${CTX_CLUSTER2}" || kubectl create --context="${CTX_CLUSTER2}" namespace sample
    kubectl label --context="${CTX_CLUSTER2}" namespace sample istio-injection=enabled
    ```

12. Deploy sample applications in `cluster1`.

    ```bash
    kubectl apply --context="${CTX_CLUSTER1}" \
      -f "https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml" \
      -l service=helloworld -n sample
    kubectl apply --context="${CTX_CLUSTER1}" \
      -f "https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml" \
      -l version=v1 -n sample
    kubectl apply --context="${CTX_CLUSTER1}" \
      -f "https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/sleep/sleep.yaml" -n sample
    ```

13. Deploy sample applications in `cluster2`.

    ```bash
    kubectl apply --context="${CTX_CLUSTER2}" \
      -f "https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml" \
      -l service=helloworld -n sample
    kubectl apply --context="${CTX_CLUSTER2}" \
      -f "https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml" \
      -l version=v2 -n sample
    kubectl apply --context="${CTX_CLUSTER2}" \
      -f "https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/sleep/sleep.yaml" -n sample
    ```

14. Wait for the sample applications to be ready.
    ```bash
    kubectl --context="${CTX_CLUSTER1}" wait --for condition=available -n sample deployment/helloworld-v1
    kubectl --context="${CTX_CLUSTER2}" wait --for condition=available -n sample deployment/helloworld-v2
    kubectl --context="${CTX_CLUSTER1}" wait --for condition=available -n sample deployment/sleep
    kubectl --context="${CTX_CLUSTER2}" wait --for condition=available -n sample deployment/sleep
    ```

15. From `cluster1`, send 10 requests to the helloworld service. Verify that you see responses from both v1 and v2.

    ```bash
    for i in {0..9}; do
      kubectl exec --context="${CTX_CLUSTER1}" -n sample -c sleep \
        "$(kubectl get pod --context="${CTX_CLUSTER1}" -n sample -l \
        app=sleep -o jsonpath='{.items[0].metadata.name}')" \
        -- curl -sS helloworld.sample:5000/hello;
    done
    ```

16. From `cluster2`, send another 10 requests to the helloworld service. Verify that you see responses from both v1 and v2.

    ```bash
    for i in {0..9}; do
      kubectl exec --context="${CTX_CLUSTER2}" -n sample -c sleep \
        "$(kubectl get pod --context="${CTX_CLUSTER2}" -n sample -l \
        app=sleep -o jsonpath='{.items[0].metadata.name}')" \
        -- curl -sS helloworld.sample:5000/hello;
    done
    ```

17. Cleanup

    ```bash
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

    ```bash
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

    ```bash
    kubectl apply --context "${CTX_CLUSTER1}" -f https://raw.githubusercontent.com/istio-ecosystem/sail-operator/main/docs/deployment-models/resources/east-west-gateway-net1.yaml
    ```
  
3. Expose istiod on `cluster1`.

    ```bash
    kubectl apply --context "${CTX_CLUSTER1}" -f https://raw.githubusercontent.com/istio-ecosystem/sail-operator/main/docs/deployment-models/resources/expose-istiod.yaml
    ```

4. Expose services on `cluster1` and `cluster2`.

    ```bash
    kubectl --context "${CTX_CLUSTER1}" apply -n istio-system -f https://raw.githubusercontent.com/istio-ecosystem/sail-operator/main/docs/deployment-models/resources/expose-services.yaml
    ```

5. Create an `Istio` on `cluster2` with the `remote` profile.

    ```bash
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

6. Set the controlplane cluster for `cluster2`.

    ```bash
    kubectl --context="${CTX_CLUSTER2}" annotate namespace istio-system topology.istio.io/controlPlaneClusters=cluster1
    ```

7. Install a remote secret on `cluster1` that provides access to the `cluster2` API server.

    ```bash
    istioctl create-remote-secret \
      --context="${CTX_CLUSTER2}" \
      --name=remote | \
      kubectl apply -f - --context="${CTX_CLUSTER1}"
    ```

    If using kind, first get the `cluster2` controlplane ip and pass the `--server` option to `istioctl create-remote-secret`

    ```bash
    REMOTE_CONTAINER_IP=$(kubectl get nodes -l node-role.kubernetes.io/control-plane --context "${CTX_CLUSTER2}" -o jsonpath='{.items[0].status.addresses[?(@.type == "InternalIP")].address}')
    istioctl create-remote-secret \
      --context="${CTX_CLUSTER2}" \
      --name=remote \
      --server="https://${REMOTE_CONTAINER_IP}:6443" | \
      kubectl apply -f - --context "${CTX_CLUSTER1}"
    ```

8. Install east-west gateway in `cluster2`.

    ```bash
    kubectl apply --context "${CTX_CLUSTER2}" -f https://raw.githubusercontent.com/istio-ecosystem/sail-operator/main/docs/deployment-models/resources/east-west-gateway-net2.yaml
    ```

9. Deploy sample applications to `cluster1`.

    ```bash
    kubectl get ns sample --context "${CTX_CLUSTER1}" || kubectl create --context="${CTX_CLUSTER1}" namespace sample
    kubectl label --context="${CTX_CLUSTER1}" namespace sample istio-injection=enabled
    kubectl apply --context="${CTX_CLUSTER1}" \
      -f "https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml" \
      -l service=helloworld -n sample
    kubectl apply --context="${CTX_CLUSTER1}" \
      -f "https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml" \
      -l version=v1 -n sample
    kubectl apply --context="${CTX_CLUSTER1}" \
      -f "https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/sleep/sleep.yaml" -n sample
    ```

10. Deploy sample applications to `cluster2`.

    ```bash
    kubectl get ns sample --context "${CTX_CLUSTER2}" || kubectl create --context="${CTX_CLUSTER2}" namespace sample
    kubectl label --context="${CTX_CLUSTER2}" namespace sample istio-injection=enabled
    kubectl apply --context="${CTX_CLUSTER2}" \
      -f "https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml" \
      -l service=helloworld -n sample
    kubectl apply --context="${CTX_CLUSTER2}" \
      -f "https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml" \
      -l version=v2 -n sample
    kubectl apply --context="${CTX_CLUSTER2}" \
      -f "https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/sleep/sleep.yaml" -n sample
    ```

11. Verify that you see a response from both v1 and v2 on `cluster1`.

    `cluster1` responds with v1 and v2
    ```bash
    kubectl exec --context="${CTX_CLUSTER1}" -n sample -c sleep \
        "$(kubectl get pod --context="${CTX_CLUSTER1}" -n sample -l \
        app=sleep -o jsonpath='{.items[0].metadata.name}')" \
        -- curl -sS helloworld.sample:5000/hello
    ```

    `cluster2` responds with v1 and v2
    ```bash
    kubectl exec --context="${CTX_CLUSTER2}" -n sample -c sleep \
        "$(kubectl get pod --context="${CTX_CLUSTER2}" -n sample -l \
        app=sleep -o jsonpath='{.items[0].metadata.name}')" \
        -- curl -sS helloworld.sample:5000/hello
    ```

12. Cleanup

    ```bash
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

    ```bash
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

    ```bash
    kubectl --context "${CTX_CLUSTER1}" apply -f https://raw.githubusercontent.com/istio-ecosystem/sail-operator/main/docs/deployment-models/resources/controlplane-gateway.yaml
    kubectl --context "${CTX_CLUSTER1}" wait '--for=jsonpath={.status.loadBalancer.ingress[].ip}' --timeout=30s svc istio-ingressgateway -n istio-system
    ```

3. Configure your environment to expose the ingress gateway.

    **Note:** these instructions are intended to be executed in a test environment. For production environments, please refer to: https://istio.io/latest/docs/setup/install/external-controlplane/#set-up-a-gateway-in-the-external-cluster and https://istio.io/latest/docs/tasks/traffic-management/ingress/secure-ingress/#configure-a-tls-ingress-gateway-for-a-single-host for setting up a secure ingress gateway.

    ```bash
    export EXTERNAL_ISTIOD_ADDR=$(kubectl -n istio-system --context="${CTX_CLUSTER1}" get svc istio-ingressgateway -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
    ```

4. Create the `external-istiod` namespace and `Istio` resource in `cluster2`.

    ```bash
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

    ```bash
    kubectl create namespace external-istiod --context="${CTX_CLUSTER1}"
    ```

6. Create the remote-cluster-secret on `cluster1` so that the `external-istiod` can access the remote cluster.

    ```bash
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

    ```bash
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

8. Create the `Gateway` and `VirtualService` resources to route traffic from the ingress gateway to the external control plane.

    ```bash
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

    ```bash
    kubectl wait --context="${CTX_CLUSTER2}" --for=condition=Ready istios/external-istiod --timeout=3m
    ```

10. Create the `sample` namespace on the remote cluster and label it to enable injection.

    ```bash
    kubectl create --context="${CTX_CLUSTER2}" namespace sample
    kubectl label --context="${CTX_CLUSTER2}" namespace sample istio.io/rev=external-istiod
    ```

11. Deploy the `sleep` and `helloworld` applications to the `sample` namespace.

    ```bash
    kubectl apply -f "https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml" -l service=helloworld -n sample --context="${CTX_CLUSTER2}"
    kubectl apply -f "https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml" -l version=v1 -n sample --context="${CTX_CLUSTER2}"
    kubectl apply -f "https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/sleep/sleep.yaml" -n sample --context="${CTX_CLUSTER2}"
    ```

12. Verify the pods in the `sample` namespace have a sidecar injected.

    ```bash
    kubectl get pod -n sample --context="${CTX_CLUSTER2}"
    ```
    You should see `2/2` pods for each application in the `sample` namespace.
    ```
    NAME                             READY   STATUS    RESTARTS   AGE
    helloworld-v1-6d65866976-jb6qc   2/2     Running   0          49m
    sleep-5fcd8fd6c8-mg8n2           2/2     Running   0          49m
    ```

13. Verify you can send a request to `helloworld` through the `sleep` app on the Remote cluster.

    ```bash
    kubectl exec --context="${CTX_CLUSTER2}" -n sample -c sleep "$(kubectl get pod --context="${CTX_CLUSTER2}" -n sample -l app=sleep -o jsonpath='{.items[0].metadata.name}')" -- curl -sS helloworld.sample:5000/hello
    ```
    You should see a response from the `helloworld` app.
    ```bash
    Hello version: v1, instance: helloworld-v1-6d65866976-jb6qc
    ```

14. Deploy an ingress gateway to the Remote cluster and verify you can reach `helloworld` externally.

    Install the gateway-api CRDs.
    ```bash
    kubectl get crd gateways.gateway.networking.k8s.io --context="${CTX_CLUSTER2}" &> /dev/null || \
    { kubectl kustomize "github.com/kubernetes-sigs/gateway-api/config/crd?ref=v1.1.0" | kubectl apply -f - --context="${CTX_CLUSTER2}"; }
    ```

    Expose `helloworld` through the ingress gateway.
    ```bash
    kubectl apply -f "https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/gateway-api/helloworld-gateway.yaml" -n sample --context="${CTX_CLUSTER2}"
    kubectl -n sample --context="${CTX_CLUSTER2}" wait --for=condition=programmed gtw helloworld-gateway
    ```

    Confirm you can access the `helloworld` application through the ingress gateway created in the Remote cluster.
    ```bash
    curl -s "http://$(kubectl -n sample --context="${CTX_CLUSTER2}" get gtw helloworld-gateway -o jsonpath='{.status.addresses[0].value}'):80/hello"
    ```
    You should see a response from the `helloworld` application:
    ```bash
    Hello version: v1, instance: helloworld-v1-6d65866976-jb6qc
    ```

15. Cleanup

    ```bash
    kubectl delete istios default --context="${CTX_CLUSTER1}"
    kubectl delete ns istio-system --context="${CTX_CLUSTER1}"
    kubectl delete istios external-istiod --context="${CTX_CLUSTER1}"
    kubectl delete ns external-istiod --context="${CTX_CLUSTER1}"
    kubectl delete istios external-istiod --context="${CTX_CLUSTER2}"
    kubectl delete ns external-istiod --context="${CTX_CLUSTER2}"
    kubectl delete ns sample --context="${CTX_CLUSTER2}"
    ```
