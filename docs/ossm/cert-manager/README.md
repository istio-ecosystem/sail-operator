[Return to OSSM Docs](../)

# About integrating Service Mesh with cert-manager and istio-csr

The cert-manager tool is a solution for X.509 certificate management on Kubernetes. It delivers a unified API to integrate applications with private or public key infrastructure (PKI), such as Vault, Google Cloud Certificate Authority Service, Let’s Encrypt, and other providers.

The cert-manager tool ensures the certificates are valid and up-to-date by attempting to renew certificates at a configured time before they expire.

For Istio users, cert-manager also provides integration with istio-csr, which is a certificate authority (CA) server that handles certificate signing requests (CSR) from Istio proxies. The server then delegates signing to cert-manager, which forwards CSRs to the configured CA server.

> [!NOTE]
> Red Hat provides support for integrating with istio-csr and cert-manager. Red Hat does not provide direct support for the istio-csr or the community cert-manager components. The use of community cert-manager shown here is for demonstration purposes only.

## Prerequisites

- One of these versions of cert-manager:
  - cert-manager Operator for Red Hat OpenShift 1.10 or later
  - community cert-manager Operator 1.11 or later
  - cert-manager 1.11 or later
- OpenShift Service Mesh Operator 3.0 or later
- istio-csr 0.6.0 or later
- `IstioCNI` instance is running in the cluster
- [istioctl](https://istio.io/latest/docs/setup/install/istioctl/) is installed
- [jq](https://github.com/jqlang/jq) is installed
- [helm](https://helm.sh/docs/intro/install/) is installed

## Installing cert-manager

You can install the cert-manager tool to manage the lifecycle of TLS certificates and ensure that they are valid and up-to-date. If you are running Istio in your environment, you can also install the istio-csr certificate authority (CA) server, which handles certificate signing requests (CSR) from Istio proxies. The istio-csr CA delegates signing to the cert-manager tool, which delegates to the configured CA.

### Procedure

1.  Create the `istio-system` namespace:

    ```sh
    oc create namespace istio-system
    ```

2.  Create the root cluster issuer:

    ```sh
    oc apply -f - <<EOF
    apiVersion: cert-manager.io/v1
    kind: Issuer
    metadata:
      name: selfsigned
      namespace: istio-system
    spec:
      selfSigned: {}
    ---
    apiVersion: cert-manager.io/v1
    kind: Certificate
    metadata:
      name: istio-ca
      namespace: istio-system
    spec:
      isCA: true
      duration: 87600h # 10 years
      secretName: istio-ca
      commonName: istio-ca
      privateKey:
        algorithm: ECDSA
        size: 256
      subject:
        organizations:
          - cluster.local
          - cert-manager
      issuerRef:
        name: selfsigned
        kind: Issuer
        group: cert-manager.io
    ---
    apiVersion: cert-manager.io/v1
    kind: Issuer
    metadata:
      name: istio-ca
      namespace: istio-system
    spec:
      ca:
        secretName: istio-ca
    EOF
    oc wait --for=condition=Ready certificates/istio-ca -n istio-system
    ```

3.  Export the Root CA to the `cert-manager` namespace:

    ```sh
    oc get -n istio-system secret istio-ca -o jsonpath='{.data.tls\.crt}' | base64 -d > ca.pem
    oc create secret generic -n cert-manager istio-root-ca --from-file=ca.pem=ca.pem
    ```

4.  Install istio-csr:

    Next you will install istio-csr into the `cert-manager` namespace. Depending on which `updateStrategy` (`InPlace` or `RevisionBased`) you will choose for your `Istio` resource, you may need to pass additional options.

    <!-- GitHub alerts cannot be nested within other elements but removing the indentation here messes up the rest of the indentation below. For this reason, using a plain note here instead of a fancy Alert.-->

    **Note:** If your controlplane namespace is not `istio-system`, you will need to update `app.istio.namespace` to match your controlplane namespace.

    `InPlace` strategy installation

    ```sh
    helm repo add jetstack https://charts.jetstack.io --force-update
    helm upgrade cert-manager-istio-csr jetstack/cert-manager-istio-csr \
        --install \
        --namespace cert-manager \
        --wait \
        --set "app.tls.rootCAFile=/var/run/secrets/istio-csr/ca.pem" \
        --set "volumeMounts[0].name=root-ca" \
        --set "volumeMounts[0].mountPath=/var/run/secrets/istio-csr" \
        --set "volumes[0].name=root-ca" \
        --set "volumes[0].secret.secretName=istio-root-ca" \
        --set "app.istio.namespace=istio-system"
    ```

    `RevisionBased` strategy installation

    For the `RevisionBased` strategy, you need to specify all the istio revisions to your [istio-csr deployment](https://github.com/cert-manager/istio-csr/tree/main/deploy/charts/istio-csr#appistiorevisions0--string). You can find the names of your `IstioRevision`s with this command:

    ```sh
    oc get istiorevisions
    ```

    Install `istio-csr`

    ```sh
    helm repo add jetstack https://charts.jetstack.io --force-update
    helm upgrade cert-manager-istio-csr jetstack/cert-manager-istio-csr \
        --install \
        --namespace cert-manager \
        --wait \
        --set "app.tls.rootCAFile=/var/run/secrets/istio-csr/ca.pem" \
        --set "volumeMounts[0].name=root-ca" \
        --set "volumeMounts[0].mountPath=/var/run/secrets/istio-csr" \
        --set "volumes[0].name=root-ca" \
        --set "volumes[0].secret.secretName=istio-root-ca" \
        --set "app.istio.namespace=istio-system" \
        --set "app.istio.revisions={default-v1-23-0}"
    ```

5.  Install your `Istio` resource. Here we are disabling Istio's built in CA server and instead pointing istiod to the istio-csr CA server which will issue certificates for both istiod and user workloads. Additionally we mount the istiod certificate in a known location where it will be read by istiod. Mounting the certificates to a known location is only necessary on OSSM.

    ```sh
    oc apply -f - <<EOF
    apiVersion: sailoperator.io/v1alpha1
    kind: Istio
    metadata:
      name: default
    spec:
      version: v1.23.0
      namespace: istio-system
      values:
        global:
          caAddress: cert-manager-istio-csr.cert-manager.svc:443
        pilot:
          env:
            ENABLE_CA_SERVER: "false"
          volumeMounts:
            - mountPath: /tmp/var/run/secrets/istiod/tls
              name: istio-csr-dns-cert
              readOnly: true
    EOF
    ```

6.  Verification

    Use the sample httpbin service and sleep app to check traffic between the workloads is possible and check the workload certificate of the proxy to verify that the cert-manager tool is installed correctly.

    a. Create the `sample` namespace:

    ```sh
    oc new-project sample
    ```

    b. Find your active `IstioRevision`:

    ```sh
    oc get istiorevisions
    ```

    c. Add the injection label for your active revision to the `sample` namespace:

    ```sh
    oc label namespace sample istio.io/rev=<your-active-revision-name> --overwrite=true
    ```

    d. Deploy the HTTP and sleep apps:

    ```sh
    oc apply -n sample -f https://raw.githubusercontent.com/istio/istio/refs/heads/master/samples/httpbin/httpbin.yaml
    oc apply -n sample -f https://raw.githubusercontent.com/istio/istio/refs/heads/master/samples/sleep/sleep.yaml
    oc rollout status deployment httpbin sleep
    ```

    e. Verify that sleep can access the httpbin service:

    ```sh
    oc exec "$(oc get pod -l app=sleep -n sample \
      -o jsonpath={.items..metadata.name})" -c sleep -n sample -- \
      curl http://httpbin.sample:8000/ip -s -o /dev/null \
      -w "%{http_code}\n"
    ```

    Example output

    ```sh
    200
    ```

    f. Verify `httpbin` workload certificate matches what is expected:

    ```sh
    istioctl proxy-config secret -n sample $(oc get pods -n sample -o jsonpath='{.items..metadata.name}' --selector app=httpbin) -o json | jq -r '.dynamicActiveSecrets[0].secret.tlsCertificate.certificateChain.inlineBytes' | base64 --decode | openssl x509 -text -noout
    ```

    Example output

    ```sh
    ...
    Issuer: O = cert-manager + O = cluster.local, CN = istio-ca
    ...
    X509v3 Subject Alternative Name:
      URI:spiffe://cluster.local/ns/sample/sa/httpbin
    ```

### `RevisionBased` Upgrades

This section only applies to `RevisionBased` deployments.

Because istio-csr requires you to pass all revisions, each time you upgrade your `RevisionBased` controlplane you will need to **first** update your istio-csr deployment with the new revision before you update your `Istio.spec.version`. For example, before upgrading your controlplane from `v1.23.0 --> v1.23.1`, you need to first update your istio-csr deployment with the new revision:

```sh
helm upgrade cert-manager-istio-csr jetstack/cert-manager-istio-csr \
  --install \
  --namespace cert-manager \
  --wait \
  --reuse-values \
  --set "app.istio.revisions={default-v1-23-0,default-v1-23-1}"
```

Then you can update your `Istio.spec.version = v1.23.1`. Once the old revision is no longer in use, you can remove the revision from your istio-csr deployment as well.

```sh
helm upgrade cert-manager-istio-csr jetstack/cert-manager-istio-csr \
  --install \
  --namespace cert-manager \
  --wait \
  --reuse-values \
  --set "app.istio.revisions={default-v1-23-1}"
```

### Additional resources

For information about how to install the cert-manager Operator for OpenShift Container Platform, see: [Installing the cert-manager Operator for Red Hat OpenShift](https://docs.openshift.com/container-platform/4.16/security/cert_manager_operator/cert-manager-operator-install.html).
