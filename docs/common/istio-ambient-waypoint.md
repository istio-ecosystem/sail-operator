
## Introduction to Istio Waypoint Proxy

Ambient mesh splits Istio's functionality into two distinct layers, a secure overlay layer 4 (L4) and a Layer 7 (L7). The waypoint proxy is an optional component that is Envoy-based and handles L7 processing for workloads it manages. It acts as a gateway to a resource (a namespace, service or pod). Waypoint proxies are installed, upgraded and scaled independently from applications. They can be configured using the Kubernetes [Gateway API](https://gateway-api.sigs.k8s.io/).

### Core features

If your applications require any of the following L7 mesh functions, you will need to use a waypoint proxy in ambient mode:

- Traffic management: HTTP routing & load balancing, circuit breaking, rate limiting, fault injection, retries and timeouts
- Security: Rich authorization policies based on L7 primitives such as request type or HTTP header
- Observability: HTTP metrics, access logging and tracing

## Getting Started

*Prerequisites*

Waypoint proxies are deployed using Kubernetes Gateway resources. As of Kubernetes 1.30 and OpenShift 4.17, the Kubernetes Gateway API CRDs are not available by default and must be installed to be used. This can be done with the following command:

```sh
kubectl get crd gateways.gateway.networking.k8s.io &> /dev/null || \
  { kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.0/standard-install.yaml; }
```

### Set up Istio Ambient Mode Resources and a Sample Application

1. Install the Sail Operator along with Istio in ambient mode using the [following](https://github.com/istio-ecosystem/sail-operator/blob/main/docs/common/istio-ambient-mode.md#installation-on-openshift) steps. 

2. Deploy the sample Bookinfo applications. The steps can be found [here](https://github.com/istio-ecosystem/sail-operator/blob/main/docs/common/istio-ambient-mode.md#deploy-a-sample-application). 

Before you deploy a waypoint proxy in the application namespace, confirm the namespace is labeled with `istio.io/dataplane-mode: ambient`.

### Deploy a Waypoint Proxy

1. Deploy a waypoint proxy in the bookinfo namespace:

```sh
kubectl apply -f - <<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  labels:
    istio.io/waypoint-for: service
  name: waypoint
  namespace: bookinfo
spec:
  gatewayClassName: istio-waypoint
  listeners:
  - name: mesh
    port: 15008
    protocol: HBONE
EOF
```

The Gateway resource is labeled with `istio.io/waypoint-for: service`, indicating the waypoint can process traffic for services, which is the default waypoint traffic type.

The type of traffic that will be redirected to the waypoint is determined by the `istio.io/waypoint-for` label on the Gateway object. More detailed information can be found [here](https://istio.io/latest/docs/ambient/usage/waypoint/#waypoint-traffic-types).

2. Enroll the bookinfo namespace to use the waypoint:

```sh
kubectl label ns bookinfo istio.io/use-waypoint=waypoint
```

After a namespace is enrolled to use a waypoint, any requests from any pods using the ambient data plane mode, to any service running in that namespace, will be routed through the waypoint for L7 processing and policy enforcement. 

If you prefer more granularity than using a waypoint for an entire namespace, you can enroll only a specific service or pod to use a waypoint. When you explicitly enroll a pod, you also need to use the `istio.io/waypoint-for: workload` label in the Gateway resource.

#### Cross-namespace Waypoint

Alternatively, you can use waypoints in different namespaces. The following `Gateway` would allow resources in the `bookinfo` namespace to use the `waypoint-foo` in the `foo` namespace:

```yaml
kind: Gateway
metadata:
  name: waypoint-foo
  namespace: foo
spec:
  gatewayClassName: istio-waypoint
  listeners:
  - name: mesh
    port: 15008
    protocol: HBONE
    allowedRoutes:
      namespaces:
        from: Selector
        selector:
          matchLabels:
            kubernetes.io/metadata.name: bookinfo
```

By default, the Istio control plane will look for a waypoint specified using the `istio.io/use-waypoint` label in the same namespace as the resource which the label is applied to. You can add labels `istio.io/use-waypoint-namespace` and `istio.io/use-waypoint` together to start using the cross-namespace waypoint.

```sh
kubectl label ns bookinfo istio.io/use-waypoint-namespace=foo
kubectl label ns bookinfo istio.io/use-waypoint=waypoint-foo
```

## Layer 7 Features in Ambient Mode

The following section describes the stable features using Gateway API resource `HTTPRoute` and Istio resource `AuthorizationPolicy`. Other L7 features using a waypoint proxy will be discussed when they reach to Beta status. 

### Traffic Routing

With a waypoint proxy deployed, you can split traffic between different versions of the bookinfo reviews service. This is useful for testing new features or performing A/B testing.

For example, let’s configure traffic routing to send 90% of requests to reviews v1 and 10% to reviews v2:

```sh
kubectl apply -f - <<EOF
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: reviews
  namespace: bookinfo
spec:
  parentRefs:
  - group: ""
    kind: Service
    name: reviews
    port: 9080
  rules:
  - backendRefs:
    - name: reviews-v1
      port: 9080
      weight: 90
    - name: reviews-v2
      port: 9080
      weight: 10
EOF
```

If you open the Bookinfo application in your browser and refresh the page multiple times. Notice the requests from the reviews-v1 don’t have any stars, while the requests from reviews-v2 have black stars.

### Security Authorization

The `AuthorizationPolicy` resource can be used in both sidecar mode and ambient mode. In ambient mode, authorization policies can either be targeted (for ztunnel enforcement) or attached (for waypoint enforcement). For an authorization policy to be attached to a waypoint it must have a `targetRef` which refers to the waypoint, or a Service which uses that waypoint.

When a waypoint proxy is added to a workload, you may have two possible places where you can enforce L4 policy. (L7 policy can only be enforced at the waypoint proxy.) Ideally you should attach your policy to the waypoint proxy. Because the destination ztunnel will see traffic with the waypoint’s identity, not the source identity once you have introduced a waypoint to the traffic path.

For example, let's add a L7 authorization policy that will explicitly allow a curl service to send `GET` requests to the `productpage` service, but perform no other operations:

```sh
kubectl apply -f - <<EOF
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: productpage-waypoint
  namespace: bookinfo
spec:
  targetRefs:
  - kind: Service
    group: ""
    name: productpage
  action: ALLOW
  rules:
  - from:
    - source:
        principals:
        - cluster.local/ns/default/sa/curl
    to:
    - operation:
        methods: ["GET"]
EOF
```

Note the targetRefs field is used to specify the target service for the authorization policy of a waypoint proxy. 

### Security Authentication

Istio’s peer authentication policies, which configure mutual TLS (mTLS) modes, are supported by ztunnel.
The difference of that between sidecar mode and ambient mode is the `DISABLE` mode policies. They will be ignored in ambient mode because ztunnel and HBONE implies the use of mTLS.
More information about Istio's peer authentication behavior can be found [here](https://istio.io/latest/docs/concepts/security/#peer-authentication). 

## Troubleshoot issues

A troubleshooting guide can be reviewed from the upstream documentation, [Troubleshoot issues with waypoints](https://istio.io/latest/docs/ambient/usage/troubleshoot-waypoint/).

You can download an `istioctl` binary and run those diagnostic commands. For example, to determine which waypoint is implementing the L7 configuration for the bookinfo sevices:

```console
$ istioctl -n ztunnel ztunnel-config service

NAMESPACE    SERVICE NAME           SERVICE VIP    WAYPOINT ENDPOINTS
bookinfo     details                172.30.16.130  waypoint 1/1
bookinfo     details-v1             172.30.27.112  waypoint 1/1
bookinfo     productpage            172.30.131.128 waypoint 1/1
bookinfo     productpage-v1         172.30.22.77   waypoint 1/1
bookinfo     ratings                172.30.52.59   waypoint 1/1
bookinfo     ratings-v1             172.30.196.247 waypoint 1/1
bookinfo     reviews                172.30.48.169  waypoint 3/3
bookinfo     reviews-v1             172.30.153.9   waypoint 1/1
bookinfo     reviews-v2             172.30.11.38   waypoint 1/1
bookinfo     reviews-v3             172.30.166.227 waypoint 1/1
bookinfo     waypoint               172.30.27.85   None     1/1
istio-system istiod                 172.30.202.223 None     1/1
```

If the value for the pod's waypoint column isn't correct, verify your pod is labeled with `istio.io/use-waypoint` and the label's value is the name of a waypoint that can process the traffic. 

You can check the waypoint's proxy status via the `istioctl proxy-status` command:

```console
$ istioctl proxy-status

NAME                               CLUSTER    CDS          LDS          EDS          RDS     ECDS     ISTIOD                     VERSION
waypoint-cd5b58869-f4jf5.bookinfo Kubernetes SYNCED (17m) SYNCED (17m) SYNCED (17m) IGNORED IGNORED istiod-995b64576-qv22g     1.24.0
```

You can check the envoy configuration for the waypoint via the istioctl proxy-config command, which shows all the information related to the waypoint such as clusters, endpoints, listeners, routes and secrets:

```console
$ istioctl proxy-config all deploy/waypoint
```

## Cleanup

#### Remove Istio and Gateway Resources

```sh
kubectl delete -n bookinfo AuthorizationPolicy productpage-waypoint
kubectl delete -n bookinfo HTTPRoute reviews
kubectl delete -n bookinfo Gateway waypoint
```

#### Remove the namespace from the ambient data plane

```sh
kubectl label namespace bookinfo istio.io/dataplane-mode-
```

#### Remove the sample application

```sh
kubectl delete -n bookinfo -f https://raw.githubusercontent.com/istio/istio/release-1.24/samples/bookinfo/platform/kube/bookinfo.yaml
kubectl delete -n bookinfo -f https://raw.githubusercontent.com/istio/istio/release-1.24/samples/bookinfo/platform/kube/bookinfo-versions.yaml
kubectl delete -n bookinfo -f https://raw.githubusercontent.com/istio/istio/release-1.24/samples/bookinfo/gateway-api/bookinfo-gateway.yaml
```

#### Remove the Kubernetes Gateway API CRDs

```sh
kubectl delete -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.0/standard-install.yaml
```
