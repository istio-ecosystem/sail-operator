[Return to Project Root](../../README.md)

# Plug in CA Certificates
Istio [documentation](https://istio.io/latest/docs/tasks/security/cert-management/plugin-ca-cert/) is covering how to plug in user generated certificates to be used by the Istio CA but it's not describing a use case where it's necessary to switch from Istio CA generated self-signed certificates to user generated certificates without any traffic disruptions even with strict mTLS enabled. This missing use case is covered here.

## Switching from Istio CA generated self-signed certificates
By default the Istio CA generates a self-signed root certificate and key and uses them to sign the workload certificates. Having the root CA's private key in the cluster is not recommended in production but there might be cases where this default configuration is used already and we need to switch to better certificate management method. It's a simple [task](https://istio.io/latest/docs/tasks/security/cert-management/plugin-ca-cert/) to be done during a maintenance window where the traffic disruptions are allowed. If the same task must be done outside of the maintenance window without any traffic disruptions, the procedure is more complex.

### Cause of the traffic disruptions
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

Now, when you create your new `cacerts` secret with an intermediate certificate signed by a different root certificate, you need to restart all workloads to pick up the change. There will be a time window where one of the workloads is already using a new certificate signed by the new intermediate certificate while the other workload is still using the old one, so the mTLS communication breaks, causing traffic disruption.

### Avoiding traffic disruptions
To achieve the no-downtime update of the certificates, it's necessary to ensure that all workloads at any given time are trusting certificates signed by either the old root or the new intermediate (which is signed by the new root). This can be achieved by enabling Istio's multi-root support.

1. Enable the multi root support:

    Prepare`istio-patch.yaml`:
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
    > **_NOTE:_** Visit Istio documentation for details about `ISTIO_MULTIROOT_MESH` and `PROXY_CONFIG_XDS_AGENT`.

    Patch the Istio resource:
    ```bash
    kubectl patch Istio default --type='merge' --patch-file=istio-patch.yaml
    ```
1. Prepare new root and intermediate certificates (you should be using trusted root CA for issuing the intermediate certificate). Here we are using [tooling](https://github.com/istio/istio/tree/master/tools/certs) from the istio repository:
    ```bash
    mkdir -p certs
    pushd certs
    make -f ../tools/certs/Makefile.selfsigned.mk root-ca
    make -f ../tools/certs/Makefile.selfsigned.mk intermediate-cacerts
    ```
1. Create new `cacerts` secrets with old CA certificate, key and chain and new combined root certificates:
    > **_NOTE:_** It's necessary to assure that all workloads trust both old and new root certificates before updating the certificate used for signing workload certificates to avoid traffic disruptions.

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
    -----BEGIN CERTIFICATE-----
    MIIC/TCCAeWgAwIBAgIRAOJUkqyDi0j/BlG8jizlmucwDQYJKoZIhvcNAQELBQAw
    GDEWMBQGA1UEChMNY2x1c3Rlci5sb2NhbDAeFw0yNTA2MjQxMjA4NDNaFw0zNTA2
    MjIxMjA4NDNaMBgxFjAUBgNVBAoTDWNsdXN0ZXIubG9jYWwwggEiMA0GCSqGSIb3
    DQEBAQUAA4IBDwAwggEKAoIBAQD+VPnSrL8JcESAaQT8xewSqacNfhDOpBT36HgR
    UFx1TFPR+dw4uZDlFW+ANOffE2HGVj9sXhA69p51xfISdOYeneZRzd68k6mjZkXV
    0kXB6wf52T/T0NRkprq+17g5jgxbXEu+yvfeEUbL3GLx6NJCkgzHH3zaqBf0nZDX
    tfVM14/uep2rGXIRf3/hnwO3qff0uRVLJebE/9lV6cOE1pbUPU4qPA7NEgiFqzzp
    ap2FL1MoXa2ptYJ0kX7ZCobXDbOD5IIrFWC+MI2dDLL409EjIv5R22An4TiVV0Qx
    oGkvdC5CXYrDes37jJsIdpMxzFBWeESxTd+w8bxXJiPzKOlTAgMBAAGjQjBAMA4G
    A1UdDwEB/wQEAwICBDAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBTJjPo79+xn
    WXWG+MSAf5i1nEOdBDANBgkqhkiG9w0BAQsFAAOCAQEAuOUF+zT90k4180bObsTS
    QeRAKBp+A9tRIqHSt7kg4QSJFz+KeoQ1CResuquydVtFwJ84ulfATqL6IbfzUWiF
    nWgNlQ/fVvW3MS1/0ZjA6qHr5LJABu8ouwsOqo9tWJifKYl6cD7InoKgViLGssL0
    guQzV+mJ8TY8s8RhtB5H5ZQ9nm9/c6Qy4RuoECf9e3PfY/hwNgLXcHIWgBinxYrt
    6N5/96gZ77nUDtbI4qBuHxiGZ0rxcGFJ+/fJTUbKV+QKuF16GRxURUfoyJ5iL9Si
    AnmwFWYxglgunft9xqW6tg/+0v8J9hcO1uxe3M0LXj4xh5BUCAtOuGaPcE1uHBtI
    qQ==
    -----END CERTIFICATE-----
    -----BEGIN CERTIFICATE-----
    MIIFFDCCAvygAwIBAgIULu/YsgYLAcQ1kPc8kyzkDEGjhS8wDQYJKoZIhvcNAQEL
    BQAwIjEOMAwGA1UECgwFSXN0aW8xEDAOBgNVBAMMB1Jvb3QgQ0EwHhcNMjUwNjI0
    MTIxMTA3WhcNMzUwNjIyMTIxMTA3WjAiMQ4wDAYDVQQKDAVJc3RpbzEQMA4GA1UE
    AwwHUm9vdCBDQTCCAiIwDQYJKoZIhvcNAQEBBQADggIPADCCAgoCggIBAJaj/7VE
    AdTGAJoylinnqNzuKKV2ZRV6yqFhMeVknRWl4nGOuJp58sQPO0DXG2uxv1Oi6hKo
    Q8A2uL3ReQVt60VqrVvoFKFFaBnicnJ9XWOzZWx07uz7PoBc9llj+azUuSrTOWF5
    wxtQ1RHM/v2fPyzoNQMwj6Xohggh1JboFUW09IRXmoDW/HNVuFdoDtlk47ZAeI7S
    9z3yHMhTlOJ1tDrQqQgh2booBfm8DhoDtdIkFCjG9kKj9nB2Wz4hM160fneAlg5m
    aP0TZSECfWq3I0QCadXmveUth6jvU+0TI54O/O6/w/Tm9Sd0VuswoKkxFAH+PgJF
    /8FifH3BWi0dmLRBSPVBlJiUloFtXeZAsYGjHVlz2hs0R1cL8D0STJwWgLTQGnak
    CY9j7S/3CwGKMfuCxxDbFDhCcEoFDC4kO6CyU7GXNN8DZhZSBIjXF5Gj1Ua93Co/
    lmISOxVrFNCEdDODFLEe1dgffUn0m4kWUWaQzbsLWqFQFx1YZs0FjQ61Ap6Y8QjR
    edhmTGROCZRm9y4HrHRAZJ2poIfXOSJgkyfu/o7kvkO/zhamYKNbmBMJGvlw7JdS
    waMp4I5kFNql27AAFJVG1lyFGagr7fi7wDsY8ohRB5V/mFV1Hu06Ukz03Z+s5+hj
    6c2mPxoO5c/hY7QVt8G2gvYkvRpek2iI1IFHAgMBAAGjQjBAMB0GA1UdDgQWBBTB
    thIKcqmem8YGPAkkqvkUdptflDAPBgNVHRMBAf8EBTADAQH/MA4GA1UdDwEB/wQE
    AwIC5DANBgkqhkiG9w0BAQsFAAOCAgEAdE95pB1JOlmZkR9WEXb8F81FESti/z2V
    nKkAQsYui39UK3jK93cMRg2axxLH/3hXxLJcVNZ/iV5aTNhL9naatui3dMz0zLBk
    2CduGwctlBooJzOa4c2jUbhpdycyIjsHFd6l9ezrWY/JOf1oLwwjNwPa1AO+VOt+
    ZC4tf4j/O0Q+6ThhGQfZVr0X6UN/jWV89Wpo00QsyACwcn2izbx9o25KSGioNJeS
    ZcwpgbW1jzASSEUeklqyc1gfZgxM7HyHC+GUV/QSfJugUB4glyUZzpz6gTZWL6N5
    aq5xkQBSUAP8nOmy4aIAEEx4clL03iq62xbwamzjtET5M5NqRIPc2V2cZqQhs0TJ
    iiGT98SBu2IydDGPXI/rruujShrIhmJ9WwiaPBdHBnSQQ+AkeDvA3AOcgFmy6Mbs
    HfJ5vvwxtPYLc8VPNGWKlu+Jbknea+N5izpdSca+TqfqQ+QwVpcbAGgplT5CqmHU
    Ap0ytVizhMxJpMMDU1GZ2C90SCX9N9hnD/Who/Py1BfbjEvBD9TuNdQ14cRWHDmU
    Xmyv/zsOhCBskS7bnQNLqhBUS4JMvSDCb0CUMEzmGzJDCGXOTeYs2d1mcNTvDkLS
    Hgv1jKTfpRXP4pMFOGGMY9XC3OYK/TtVhDAyrWewREMNQTtBKSEj2S6R5rT5MD02
    ir4ltxRVyHM=
    -----END CERTIFICATE-----
    ```
    > **_NOTE:_** It might be necessary to restart the workload if you only see one certificate.
1. Update `combined-root.pem` by adding the new root certificate again. Using updated `root-cert.pem` will trigger a rotation of workload certificates even without a need to restart the workloads:
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