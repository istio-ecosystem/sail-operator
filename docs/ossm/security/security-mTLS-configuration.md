## About mutual Transport Layer Security (mTLS)

Mutual Transport Layer Security (mTLS) is a protocol that enables two parties to authenticate each other. However, configuring mTLS settings can be confusing and a common source of misconfiguration. First, you need to understand the following Istio resources and concepts[1].

- `PeerAuthentication` is used to configure what type of mTLS traffic the sidecar will **accept**.
Its `PERMISSIVE` mode[2] means plaintext or mTLS traffic will all be accepted by the sidecar. Its `STRICT` mode means only mTLS traffic will be accepted by the sidecar.

- `DestinationRule` is used to configure what type of TLS traffic the sidecar will **send**.
Its `DISABLE` mode will allow the sidecar to send plaintext, while its `SIMPLE`, `MUTUAL`, and `ISTIO_MUTUAL` mode will configure the sidecar to originate a TLS connection.

- `Auto mTLS` means: without any configuration, all inter-mesh traffic will be mTLS encrypted.
This is configured by a global mesh config field `enableAutoMtls` and it is enabled by default.

## Default Settings

Because `auto mTLS` is enabled by default. Traffic **sent** through a sidecar is mTLS encrypted. It doesn't matter which `PeerAuthentication` mode is configured. You can use mTLS without changes to the application or service code. The mTLS is handled entirely between the two sidecar proxies.

On the other hand, `PeerAuthentication` is set to the `PERMISSIVE` mode by default, where the sidecars in Service Mesh **accept** both plain-text traffic and connections that are encrypted using mTLS. This mode provides greater flexibility for the mTLS on-boarding process.

## Enabling strict mTLS mode by namespace

If you need to lock down workloads to only **accept** mTLS traffic, you may apply the following change to enable the `STRICT` mode of `PeerAuthentication`. 

- PeerAuthentication Policy example for a namespace

```
$ oc apply -n <namespace> -f - <<EOF
apiVersion: security.istio.io/v1beta1
kind: PeerAuthentication
metadata:
  name: default
  namespace: <namespace>
spec:
  mtls:
    mode: STRICT
EOF
```

If you manually disabled the `auto mTLS` mesh config field and you are setting `PeerAuthentication` to `STRICT` mode, you also need to create a `DestinationRule` resource with `MUTUAL` or `ISTIO_MUTUAL` mode for your service. The following example enables mTLS to all destination hosts in the `<namespace>`.

- DestinationRule example

```
$ oc apply -n <namespace> -f - <<EOF
apiVersion: networking.istio.io/v1alpha3
kind: DestinationRule
metadata:
  name: enable-mtls
  namespace: <namespace>
spec:
  host: "*.<namespace>.svc.cluster.local"
  trafficPolicy:
   tls:
    mode: ISTIO_MUTUAL
EOF
```
a. Replace <namespace> with the namespace where the service is located.


## Enabling strict mTLS across the whole service mesh

If you need to lock down mTLS for the whole mesh, you may apply the above PeerAuthentication Policy example for the istiod namespace, for example, `istio-system` namespace. Moreover, you also need to apply a `DestinationRule` to disable mTLS when talking to API server, as API server doesn't have sidecar. You should apply similar `DestinationRule` for other services that don't have sidecar in this mode.

- PeerAuthentication Policy example for the whole mesh

```
$ oc apply -n istio-system -f - <<EOF
apiVersion: security.istio.io/v1
kind: PeerAuthentication
metadata:
  name: default
  namespace: istio-system
spec:
  mtls:
    mode: STRICT
---
apiVersion: networking.istio.io/v1beta1
kind: DestinationRule
metadata:
  name: api-server
  namespace: istio-system
spec:
  host: kubernetes.default.svc.cluster.local
  trafficPolicy:
    tls:
      mode: DISABLE
EOF
```

## Setting the minimum and maximum protocol versions

See the Istio documentation ["Istio Workload Minimum TLS Version Configuration"](https://istio.io/latest/docs/tasks/security/tls-configuration/workload-min-tls-version/).

## Validating encryption with Kiali

The Kiali console offers several ways to validate whether or not your applications, services, and workloads have mTLS encryption enabled.

The `Services Detail Overview` page displays a Security icon on the graph edges where at least one request with mTLS enabled is present. Also note that Kiali displays a lock icon in the `Network` section next to ports that are configured for mTLS.

### Additional resources

References: 
- [1] https://istio.io/latest/docs/ops/configuration/traffic-management/tls-configuration/
- [2] https://istio.io/latest/docs/concepts/security/#permissive-mode
