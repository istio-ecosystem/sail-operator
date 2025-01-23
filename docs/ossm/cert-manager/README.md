[Return to OSSM Docs](../)

# About integrating Service Mesh with cert-manager and istio-csr

The cert-manager tool is a solution for X.509 certificate management on Kubernetes. It delivers a unified API to integrate applications with private or public key infrastructure (PKI), such as Vault, Google Cloud Certificate Authority Service, Let’s Encrypt, and other providers.

The cert-manager tool ensures the certificates are valid and up-to-date by attempting to renew certificates at a configured time before they expire.

For Istio users, cert-manager provides integration with Istio through an external agent called istio-csr. istio-csr handles certificate signing requests (CSR) from Istio proxies and the controlplane by verifying the identity of the workload and then creating a CSR through cert-manager for the workload. cert-manager then creates a CSR to the configured CA Issuer which then signs the certificate.

> [!NOTE]
> Red Hat provides support for integrating with istio-csr and cert-manager. Red Hat does not provide direct support for the istio-csr or the community cert-manager components. The use of community cert-manager shown here is for demonstration purposes only.

## Prerequisites

- One of these versions of cert-manager:
  - cert-manager Operator for Red Hat OpenShift 1.10 or later
  - community cert-manager Operator 1.11 or later
  - cert-manager 1.11 or later
- OpenShift Service Mesh Operator 3.0 or later
- `IstioCNI` instance is running in the cluster
- [istioctl](https://istio.io/latest/docs/setup/install/istioctl/) is installed
- [jq](https://github.com/jqlang/jq) is installed
- [helm](https://helm.sh/docs/intro/install/) is installed

## Integrating cert-manager with Service Mesh

You can integrate cert-manager with your Service Mesh by deploying istio-csr and then creating an `Istio` resource that uses istio-csr to process workload and controlplane certificate signing requests. The procedure below creates a self signed `Issuer`, but any other `Issuer` can be used instead.

### Procedure

1.  Create the `istio-system` namespace.

    ```sh
    oc create namespace istio-system
    ```

2.  Create the root issuer.

    - Create the `Issuer` object as in the following example:

      _Example `issuer.yaml`_

      ```yaml
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
      ```

    - Create the objects by using the following command.

      ```sh
      oc apply -f issuer.yaml
      ```

    - Wait for the `istio-ca` certificate to become ready.
      ```sh
      oc wait --for=condition=Ready certificates/istio-ca -n istio-system
      ```

3.  Copy the `istio-ca` certificate to the `cert-manager` namespace.

    Here we are copying our `istio-ca` certificate to the `cert-manager` namespace where it can be used by istio-csr.

    - Copy the secret to a local file.

      ```sh
      oc get -n istio-system secret istio-ca -o jsonpath='{.data.tls\.crt}' | base64 -d > ca.pem
      ```

    - Create a secret from the local cert file in the `cert-manager` namespace.
      ```sh
      oc create secret generic -n cert-manager istio-root-ca --from-file=ca.pem=ca.pem
      ```

4.  Install istio-csr.

    Next you will install istio-csr into the `cert-manager` namespace. Depending on which `updateStrategy` (`InPlace` or `RevisionBased`) you will choose for your `Istio` resource, you may need to pass additional options.

    <!-- GitHub alerts cannot be nested within other elements but removing the indentation here messes up the rest of the indentation below. For this reason, using a plain note here instead of a fancy Alert.-->

    **Note:** If your controlplane namespace is not `istio-system`, you will need to update `app.istio.namespace` to match your controlplane namespace.

    `InPlace` strategy installation

    - Add the jetstack charts to your local helm repo.

      ```sh
      helm repo add jetstack https://charts.jetstack.io --force-update
      ```

    - Install the istio-csr chart.
      ```sh
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

    For the `RevisionBased` strategy, you need to specify all the istio revisions to your [istio-csr deployment](https://github.com/cert-manager/istio-csr/tree/main/deploy/charts/istio-csr#appistiorevisions0--string).

    - Add the jetstack charts to your local helm repo.

      ```sh
      helm repo add jetstack https://charts.jetstack.io --force-update
      ```

    - Install the istio-csr chart with your revision name. Revision names will be of the form `<istio-name><istio-version-with-dashes>` e.g. `default-v1-23-0`.
      ```sh
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

5.  Install your `Istio` resource.

    Here we are disabling Istio's built in CA server and instead configuring istiod to forward certificate signing requests to istio-csr which will obtain certificates for both istiod and the mesh workloads from cert-manager. We also mount the istiod tls certificate created by istio-csr into the pod at a known location where it will be read.

    - Create the `Istio` object as in the following example:

      _Example `istio.yaml`_

      ```yaml
      apiVersion: sailoperator.io/v1alpha1
      kind: Istio
      metadata:
        name: default
      spec:
        version: v1.24.1
        namespace: istio-system
        values:
          global:
            caAddress: cert-manager-istio-csr.cert-manager.svc:443
          pilot:
            env:
              ENABLE_CA_SERVER: "false"
      ```

    - Create the `Istio` resource by using the following command.

      ```sh
      oc apply -f istio.yaml
      ```

    - Wait for `Istio` to become ready.
      ```sh
      oc wait --for=condition=Ready istios/default -n istio-system
      ```

6.  Verification

    Use the sample httpbin service and sleep app to check traffic between the workloads is possible and check the workload certificate of the proxy to verify that the cert-manager tool is installed correctly.

    - Create the `sample` namespace.

      ```sh
      oc new-project sample
      ```

    - Find your active `IstioRevision`.

      ```sh
      oc get istios default -o jsonpath='{.status.activeRevisionName}'
      ```

    - Add the injection label for your active revision to the `sample` namespace.

      ```sh
      oc label namespace sample istio.io/rev=<your-active-revision-name> --overwrite=true
      ```

    - Deploy the sample `httpbin` app.

      ```sh
      oc apply -n sample -f https://raw.githubusercontent.com/istio/istio/refs/heads/master/samples/httpbin/httpbin.yaml
      ```

    - Deploy the sample `sleep` app.

      ```sh
      oc apply -n sample -f https://raw.githubusercontent.com/istio/istio/refs/heads/master/samples/sleep/sleep.yaml
      ```

    - Wait for both apps to become ready.

      ```sh
      oc rollout status -n sample deployment httpbin sleep
      ```

    - Verify that sleep can access the httpbin service:

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

    - Verify `httpbin` workload certificate matches what is expected:

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
