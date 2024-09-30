[Return to OSSM Docs](../)

# Cert Manager and istio-csr Integration

Below are instructions for integrating cert-manager with OpenShift Service Mesh 3. It largely follows the [cert-manager istio-csr documentation](https://cert-manager.io/docs/usage/istio-csr/) but you will need to adjust a few settings based on which `updateStrategy` you are using.

## Common Setup

These steps are the same for each `updateStrategy`.

1. Install the [cert-manager Operator for Red Hat OpenShift](https://docs.redhat.com/en/documentation/openshift_container_platform/4.16/html/security_and_compliance/cert-manager-operator-for-red-hat-openshift#cert-manager-operator-install).

2. Create the `istio-system` namespace for the root cert and for your `Istio` resource.

   ```sh
   oc create namespace istio-system
   ```

3. Create a Self Signed certificate. Note you should adapt this example based on what PKI you are using e.g. using a vault `Issuer` instead of a self signed `Issuer`.

   ```sh
   oc apply -f https://raw.githubusercontent.com/cert-manager/website/7f5b2be9dd67831574b9bde2407bed4a920b691c/content/docs/tutorials/istio-csr/example/example-issuer.yaml
   ```

4. Export the root CA to the `cert-manager` namespace.

   ```sh
   oc get -n istio-system secret istio-ca -o jsonpath='{.data.tls\.crt}' | base64 -d > ca.pem
   oc create secret generic -n cert-manager istio-root-ca --from-file=ca.pem=ca.pem
   ```

## InPlace Strategy

1. Install [istio-csr](https://cert-manager.io/docs/usage/istio-csr).

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

   Note: If your controlplane namespace is not `istio-system`, you will need to update `app.istio.namespace` to match your controlplane namespace.

2. Install `Istio` controlplane in the `istio-system` namespace.

   ```sh
   oc apply -f - <<EOF
   apiVersion: sailoperator.io/v1alpha1
   kind: Istio
   metadata:
     name: default
   spec:
     version: v1.23.0
     namespace: istio-system
     updateStrategy:
       type: InPlace
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

3. [Verify your deployment](#verify) is configured correctly.

## RevisionBased Strategy

For the `RevisionBased` strategy, you need to specify all the istio revisions in your [istio-csr deployment](https://github.com/cert-manager/istio-csr/tree/main/deploy/charts/istio-csr#appistiorevisions0--string).

1. Install [istio-csr](https://cert-manager.io/docs/usage/istio-csr).

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

   Note: If your controlplane namespace is not `istio-system`, you will need to update `app.istio.namespace` to match your controlplane namespace.

2. Install `Istio` controlplane in the `istio-system` namespace.

   ```sh
   oc apply -f - <<EOF
   apiVersion: sailoperator.io/v1alpha1
   kind: Istio
   metadata:
     name: default
   spec:
     version: v1.23.0
     namespace: istio-system
     updateStrategy:
       type: RevisionBased
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

3. [Verify your deployment](#verify) is configured correctly.

### Verify

1. Deploy a sample application.

   Create the `sample` namespace.

   ```sh
   oc create namespace sample
   ```

   For the `RevisionBased` strategy, label your namespace with `istio.io/rev=default-v1-23-0`.

   ```sh
   oc label namespace sample istio.io/rev=default-v1-23-0
   ```

   For the `InPlace` strategy, label your namespace with `istio.io/rev=default`.

   ```sh
   oc label namespace sample istio.io/rev=default
   ```

   Deploy the `httpbin` application.

   ```sh
   oc apply -n sample -f https://raw.githubusercontent.com/istio/istio/release-1.23/samples/httpbin/httpbin.yaml
   ```

2. Ensure httpbin pod is Running.

   ```sh
   oc get pods -n sample
   ```

   ```sh
   NAME                       READY   STATUS    RESTARTS   AGE
   httpbin-67854dd9b5-b7c2q   2/2     Running   0          110s
   ```

3. Use `istioctl` to ensure httpbin workload certificate matches what is expected.

   ```sh
   istioctl proxy-config secret -n sample $(oc get pods -n sample -o jsonpath='{.items..metadata.name}' --selector app=httpbin) -o json | jq -r '.dynamicActiveSecrets[0].secret.tlsCertificate.certificateChain.inlineBytes' | base64 --decode | openssl x509 -text -noout
   ```

### Upgrades - RevisionBased

Because istio-csr requires you to pass all revisions, each time you upgrade your `RevsionBased` controlplane you will need to **first** update your istio-csr deployment with the new revision before you update your `Istio.spec.version`.

```sh
helm upgrade cert-manager-istio-csr jetstack/cert-manager-istio-csr \
  --install \
  --namespace cert-manager \
  --wait \
  --reuse-values \
  --set "app.istio.revisions={default-v1-23-0,default-v1-23-1}"
```

Once the old revision is no longer in use, you can remove the revision from istio-csr as well.

```sh
helm upgrade cert-manager-istio-csr jetstack/cert-manager-istio-csr \
  --install \
  --namespace cert-manager \
  --wait \
  --reuse-values \
  --set "app.istio.revisions={default-v1-23-1}"
```
