[Return to Project Root](../../README.md)

# Plug in CA Certificates
Istio upstream [documentation](https://istio.io/latest/docs/tasks/security/cert-management/plugin-ca-cert/) is covering how to plug in user generated certificates to be used by the Istio CA. Here, we are documenting how to switch from Istio CA generated self-signed certificates to user generated certificates without any traffic disruptions even with strict mTLS enabled.

## Switching from Istio CA generated self-signed certificates
By default the Istio CA generates a self-signed root certificate and key and uses them to sign the workload certificates. This is not recommended in production but there might be cases where it's used already and we need to switch to better certificate management method. It's a simple [task](https://istio.io/latest/docs/tasks/security/cert-management/plugin-ca-cert/) if we are able to do that during a maintenance window where the traffic disruptions are expected. The same task with a no downtime requirement is more complex.

### Problem with the simple workflow
We can simply add a new `cacerts` secret with our certificates but what does it mean for service to service mTLS traffic? Consider `sleep` and `httpbin` demo applications. Both are sharing the same root of trust which is the Istio CA generated self-signed root certificate:
```bash
$ istioctl proxy-config secret deployment/sleep -n sleep -o json | jq -r '.dynamicActiveSecrets[1].secret.validationContext.trustedCa.inlineBytes' | base64 --decode | openssl x509 -text -noout
Certificate:
    Data:
        Version: 3 (0x2)
        Serial Number:
            b0:3c:9a:d8:3f:ab:c8:a7:12:23:a8:65:d6:5e:76:c8
        Signature Algorithm: sha256WithRSAEncryption
        Issuer: O=cluster.local
        Validity
            Not Before: Jun 19 12:39:46 2025 GMT
            Not After : Jun 17 12:39:46 2035 GMT
        Subject: O=cluster.local
        ...
$ istioctl proxy-config secret deployment/httpbin -n httpbin -o json | jq -r '.dynamicActiveSecrets[1].secret.validationContext.trustedCa.inlineBytes' | base64 --decode | openssl x509 -text -noout
Certificate:
    Data:
        Version: 3 (0x2)
        Serial Number:
            b0:3c:9a:d8:3f:ab:c8:a7:12:23:a8:65:d6:5e:76:c8
        Signature Algorithm: sha256WithRSAEncryption
        Issuer: O=cluster.local
        Validity
            Not Before: Jun 19 12:39:46 2025 GMT
            Not After : Jun 17 12:39:46 2035 GMT
        Subject: O=cluster.local
        ...
```

Now, when you create your new `cacerts` secret with an intermediate certificate signed by a different root certificate, you need to restart all workloads to pick up the change. There will be a time window where one of the workloads is already using a new certificate signed by the new intermediate certificate while other workload is still using old one so the mTLS communication breaks.

### No downtime update
To achieve the no downtime update of the certificates, it's necessary to assure that all workloads at any given time are trusting certificates signed by either the old root or the new intermediate (which is signed by the new root). This can be achieved by enabling Istio's multi root support.

1. Enable the multi root support:
    ```bash
    kubectl patch Istio default --type='merge' --patch-file=istio-patch.yaml
    ```
    Content of `istio-patch.yaml`:
    ```yaml
    apiVersion: sailoperator.io/v1
    kind: Istio
    spec:
      values:
        pilot:
          env:
            ISTIO_MULTIROOT_MESH: "true"
        meshConfig:
          defaultConfig:
            proxyMetadata:
          PROXY_CONFIG_XDS_AGENT: "true"
    ```
1. Prepare new root and intermediate certificates (you should be using trusted root CA for issuing the intermediate certificate). Here we are using tooling from istio repository:
    ```bash
    mkdir -p certs
    pushd certs
    make -f ../tools/certs/Makefile.selfsigned.mk root-ca
    make -f ../tools/certs/Makefile.selfsigned.mk intermediate-cacerts
    ```
1. Create new `cacerts` secrets with old CA certificate, key and chain and combined root certificates:

    Get the certificate and the key from existing Istio CA generated secrets and prepare combined root certificates:
    ```bash
    kubectl get secret istio-ca-secret -n istio-system -o jsonpath={.data.'ca-cert\.pem'} | base64 -d > ca-cert.pem
    kubectl get secret istio-ca-secret -n istio-system -o jsonpath={.data.'ca-key\.pem'} | base64 -d > ca-key.pem
    kubectl get secret istio-ca-secret -n istio-system -o jsonpath={.data.'ca-cert\.pem'} | base64 -d > cert-chain.pem
    kubectl get secret istio-ca-secret -n istio-system -o jsonpath={.data.'ca-cert\.pem'} | base64 -d > combined-root.pem
    cat root-cert.pem >> combined-root.pem
    ```
    Create new `cacerts` secrets:
    ```bash
    kubectl create secret generic cacerts -n istio-system \
        --from-file=ca-cert.pem \
        --from-file=ca-key.pem \
        --from-file=root-cert.pem=combined-root.pem \
        --from-file=cert-chain.pem
    ```
1. Restart istiod to pick up new certificates:
    ```bash
    kubectl rollout restart deployment/istiod -n istio-system
    ```
1. Verify that all workloads are trusting both old and new roots, e.g. for httpbin:
    ```bash
    istioctl proxy-config secret deployment/httpbin -n httpbin -o json | jq -r '.dynamicActiveSecrets[1].secret.validationContext.trustedCa.inlineBytes' | base64 --decode
    ```
    > **_NOTE:_** It might be necessary to restart the workload if you only see one certificate.
1. Update `combined-root.pem` by adding the new root certificate again, this will assure that workload certificates are regenerated after next step as the root certificate is updated:
    ```bash
    cat root-cert.pem >> combined-root.pem
    ```
1. Update `cacerts` secrets to use the new intermediate certificate, key and chain and updated combined root certificates:
    ```bash
    kubectl delete secret cacerts -n istio-system --ignore-not-found && \
    kubectl create secret generic cacerts -n istio-system \
        --from-file=intermediate/ca-cert.pem \
        --from-file=intermediate/ca-key.pem \
        --from-file=root-cert.pem=combined-root.pem \
        --from-file=intermediate/cert-chain.pem
    ```
1. Restart istiod to pick up new certificates:
    ```bash
    kubectl rollout restart deployment/istiod -n istio-system
    ```
1. Verify that workloads certificates have been rotated and issued by the new intermediate CA:
    ```bash
    istioctl proxy-config secret deployment/httpbin -n httpbin -o json | jq -r '.dynamicActiveSecrets[0].secret.tlsCertificate.certificateChain.inlineBytes' | base64 -d  |  openssl x509 -text -noout
    Certificate:
    Data:
        Version: 3 (0x2)
        Serial Number:
            37:dc:72:ad:e1:ae:06:e3:0d:fd:3d:61:bb:37:10:16
        Signature Algorithm: sha256WithRSAEncryption
        Issuer: O=Istio, CN=Intermediate CA, L=intermediate
        Validity
            Not Before: Jun 19 15:54:04 2025 GMT
            Not After : Jun 20 15:56:04 2025 GMT
    ...
    ```
1. Remove old root certificate:
    ```bash
    kubectl delete secret cacerts -n istio-system --ignore-not-found && \
    kubectl create secret generic cacerts -n istio-system \
        --from-file=intermediate/ca-cert.pem \
        --from-file=intermediate/ca-key.pem \
        --from-file=root-cert.pem \
        --from-file=intermediate/cert-chain.pem
    ```
1. Restart istiod to pick up new certificates:
    ```bash
    kubectl rollout restart deployment/istiod -n istio-system
    ```

At this point, rotation of the new intermediate certificate will be much simpler as long as it's issued by the same root CA.