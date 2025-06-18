[Return to OSSM Docs](../)

# OpenShift Service Mesh and Cert-Manager Operator Istio-CSR Agent

The cert-manager tool is a solution for X.509 certificate management on Kubernetes. It delivers a unified API to integrate applications with private or public key infrastructure (PKI), such as Vault, Google Cloud Certificate Authority Service, Let’s Encrypt, and other providers.

> [!NOTE]
> The cert-manager Operator must be installed before you create and install your Istio resource.

The cert-manager tool ensures the certificates are valid and up-to-date by attempting to renew certificates at a configured time before they expire.

## About integrating Service Mesh with cert-manager Istio-CSR Agent

> [!NOTE]
> Istio-CSR integration for cert-manager Operator for Red Hat OpenShift is a Technology Preview feature only. Technology Preview features are not supported with Red Hat production service level agreements (SLAs) and might not be functionally complete. Red Hat does not recommend using them in production. These features provide early access to upcoming product features, enabling customers to test functionality and provide feedback during the development process.

The cert-manager Operator for Red Hat OpenShift provides enhanced support for securing workloads and control plane components in Red Hat OpenShift Service Mesh or Istio. This includes support for certificates enabling mutual TLS (mTLS), which are signed, delivered, and renewed using cert-manager issuers. You can secure Istio workloads and control plane components by using Red Hat OpenShift Cert-Manager Operator Istio-CSR agent.

With this Istio-CSR integration, Istio can now obtain certificates from the cert-manager Operator for Red Hat OpenShift, simplifying security and certificate management.

### Prerequisites

- cert-manager Operator for Red Hat OpenShift version 1.15.1 should be installed
- Red Hat Openshift 4.14 or later
- OpenShift Service Mesh Operator 2.6 or later
- `IstioCNI` instance is running in the cluster
- [istioctl](https://istio.io/latest/docs/setup/install/istioctl/) is installed

## Integrating the cert-manager Operator Istio-CSR Agent for Red Hat OpenShift with  

You can integrate cert-manager with your Service Mesh by deploying Istio-CSR agent and then creating an `istio` resource that uses Istio-CSR Agent to process workload and controlplane certificate signing requests. The procedure below creates a self signed `Issuer`.

### Procedure

1.  Create the `istio-system` namespace.

    ```sh
    oc create namespace istio-system
    ```

2.  Patch Cert-Manager Operator to Install Istio-CSR agent 

    Use this procedure to be able to enable the Istio-CSR agent in cert-manager Operator for Red Hat OpenShift.

    ```sh
    oc -n cert-manager-operator patch subscription openshift-cert-manager-operator --type='merge' -p '{"spec":{"config":{"env":[{"name":"UNSUPPORTED_ADDON_FEATURES","value":"IstioCSR=true"}]}}}'
    ```

3. Creating a root CA issuer for the Istio-CSR agent 

    - Create a new project for installing Istio-CSR

      ```sh
      oc new-project istio-csr
      ```

    - Create the `Issuer` object as in the following example:
      > [!NOTE]
      > The selfSigned issuer is intended for demonstration, testing, or proof-of-concept setups only. For production deployments, use a secure and trusted CA

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

4.  Creating the IstioCSR custom resource

    - Use this procedure to create the Istio-CSR agent resource

      _Example `istioCSR.yaml`_

      ```yaml
      apiVersion: operator.openshift.io/v1alpha1
      kind: IstioCSR
      metadata:
        name: default
        namespace: istio-csr
      spec:
        istioCSRConfig:
          certManager:
            issuerRef:
              name: istio-ca
              kind: Issuer
              group: cert-manager.io
          istiodTLSConfig:
            trustDomain: cluster.local
          istio:
            namespace: istio-system
      ```

    - Create `istio-csr`
      ```sh
      oc create -f istioCSR.yaml
      ```

    - Check `istio-csr` deployment is ready.
      ```sh
      oc get deployment -n istio-csr
      ```

5.  Install your `Istio` resource.

    Here we are disabling Istio's built in CA server and instead configuring istiod to forward certificate signing requests to istio-csr which will obtain certificates for both istiod and the mesh workloads from cert-manager. We also mount the istiod tls certificate created by istio-csr into the pod at a known location where it will be read.

    - Create the `Istio` object as in the following example:

      _Example `istio.yaml`_

      ```yaml
      apiVersion: sailoperator.io/v1
      kind: Istio
      metadata:
        name: default
      spec:
        version: v1.24-latest
        namespace: istio-system
        values:
          global:
            caAddress: cert-manager-istio-csr.istio-csr.svc:443
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

    - Create the `apps-1` and `apps-2` namespace.

      ```sh
      oc new-project apps-1
      oc new-project apps-2
      ```

    - Enable istio-injection on namespaces

      ```sh
      oc label namespaces apps-1 istio-injection=enabled
      oc label namespaces apps-2 istio-injection=enabled
      ```

    - Deploy `httpbin` app in namespaces

      ```sh
      oc apply -n apps-1 -f https://raw.githubusercontent.com/openshift-service-mesh/istio/release-1.24/samples/httpbin/httpbin.yaml
      oc apply -n apps-2 -f https://raw.githubusercontent.com/openshift-service-mesh/istio/release-1.24/samples/httpbin/httpbin.yaml
      ```

    - Deploy `sleep` app in namespaces

      ```sh
      oc apply -n apps-1 -f https://raw.githubusercontent.com/openshift-service-mesh/istio/release-1.24/samples/sleep/sleep.yaml
      oc apply -n apps-2 -f https://raw.githubusercontent.com/openshift-service-mesh/istio/release-1.24/samples/sleep/sleep.yaml
      ```

    - Verify created apps have sidecars injected

      ```sh
      oc get pods -n apps-1
      oc get pods -n apps-2
      ```

    - Create a mesh-wide strict mTLS policy:

      > [!NOTE]
      > We are enabling PeerAuthentication with strict mTLS mode to show that the certificates are distributed correctly so the mTLS between workloads works.

      _Example `peer_auth.yaml`_

      ```yaml
      apiVersion: security.istio.io/v1beta1
      kind: PeerAuthentication
      metadata:
        name: default
        namespace: istio-system
      spec:
        mtls:
          mode: STRICT
      ```

    - Apply mTLS policy:

      ```sh
      oc apply -f peer_auth.yaml
      ```

    - Verify that apps-1/sleep can access the apps-2/httpbin service:

      ```sh
      oc -n apps-1 exec "$(oc -n apps-1 get pod -l app=sleep -o jsonpath={.items..metadata.name})" -c sleep -- curl -sIL http://httpbin.apps-2.svc.cluster.local:8000
      ```

      Example output

      ```sh
      HTTP/1.1 200 OK
      access-control-allow-credentials: true
      access-control-allow-origin: *
      content-security-policy: default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' camo.githubusercontent.com
      content-type: text/html; charset=utf-8
      date: Wed, 18 Jun 2025 09:20:55 GMT
      x-envoy-upstream-service-time: 14
      server: envoy
      transfer-encoding: chunked
      ```

    - Verify that apps-2/sleep can access the apps-1/httpbin service:

      ```sh
      oc -n apps-2 exec "$(oc -n apps-2 get pod -l app=sleep -o jsonpath={.items..metadata.name})" -c sleep -- curl -sIL http://httpbin.apps-1.svc.cluster.local:8000
      ```

      Example output

      ```sh
      HTTP/1.1 200 OK
      access-control-allow-credentials: true
      access-control-allow-origin: *
      content-security-policy: default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' camo.githubusercontent.com
      content-type: text/html; charset=utf-8
      date: Wed, 18 Jun 2025 09:21:23 GMT
      x-envoy-upstream-service-time: 16
      server: envoy
      transfer-encoding: chunked
      ```

    - Verify httpbin workload certificate matches what is expected:
      ```sh
      istioctl proxy-config secret -n apps-1 $(oc get pods -n apps-1 -o jsonpath='{.items..metadata.name}' --selector app=httpbin) -o json | jq -r '.dynamicActiveSecrets[0].secret.tlsCertificate.certificateChain.inlineBytes' | base64 --decode | openssl x509 -text -noout
      ```

      Example output

      ```sh
      ...
      Issuer: O = cert-manager + O = cluster.local, CN = istio-ca
      ...
      X509v3 Subject Alternative Name:
      URI:spiffe://cluster.local/ns/apps-1/sa/httpbin
      ```

### Uninstalling Cert-Manager Operator Istio-CSR Agent for Red Hat OpenShift

#### Procedure

  1.  Remove the IstioCSR custom resource by running the following command:

      > [!NOTE]
      > To avoid disrupting any Red Hat OpenShift Service Mesh or Istio components, ensure that no component is referencing the Istio-CSR service or the certificates issued for Istio before removing the following resources.

      ```sh
      oc -n <istio-csr_project_name> delete istiocsrs.operator.openshift.io default
      ```
  2. Remove related resources:
    
      i. List the cluster scoped-resources by running the following command and save the names of the listed resources for later reference:

        ```sh
        oc get clusterrolebindings,clusterroles -l "app=cert-manager-istio-csr,app.kubernetes.io/name=cert-manager-istio-csr"
        ```

      ii. List the resources in Istio-csr deployed namespace by running the following command and save the names of the listed resources for later reference:

        ```sh
        oc get certificate,deployments,services,serviceaccounts -l "app=cert-manager-istio-csr,app.kubernetes.io/name=cert-manager-istio-csr" -n <istio_csr_project_name>
        ```

      iii. List the resources in Red Hat OpenShift Service Mesh or Istio deployed namespaces by running the following command and save the names of the listed resources for later reference:

        ```sh
        oc get roles,rolebindings -l "app=cert-manager-istio-csr,app.kubernetes.io/name=cert-manager-istio-csr" -n <istio_csr_project_name>
        ```

      iv. For each resource listed in previous steps, delete the resource by running the following command:

        ```sh
        oc -n <istio_csr_project_name> delete <resource_type>/<resource_name>
        ```

### Additional resources

For information about how to install the cert-manager Operator for OpenShift Container Platform, see: [Installing the cert-manager Operator for Red Hat OpenShift](https://docs.openshift.com/container-platform/4.16/security/cert_manager_operator/cert-manager-operator-install.html).


For information about how to enable istioCSR agent for OpenShift Container Platform, see: [Integrating the cert-manager Operator for Red Hat OpenShift with Istio-CSR](https://docs.redhat.com/en/documentation/openshift_container_platform/4.16/html/security_and_compliance/cert-manager-operator-for-red-hat-openshift#cert-manager-operator-integrating-istio)