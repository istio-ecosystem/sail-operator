# Getting Started with Istio Waypoint in Ambient Mode (Tech Preview)

This section provides a high-level overview and installation procedure for Istio's ambient mode on OpenShift Container Platform (OCP) using OpenShift Service Mesh 3.1.x.

---

## 1. Overview of Istio Ambient Mode

Istio's ambient mode offers a sidecar-less approach to service mesh, simplifying operations and reducing resource consumption. Instead of injecting a sidecar proxy into every application pod, ambient mode utilizes a node-level **ZTunnel proxy** for secure, mTLS-enabled connections and an optional **Waypoint proxy** for advanced L7 functionalities.

### 1.2 Why Use Istio's Ambient Mode?

Istio's ambient mode offers several benefits:

* **Simplified Operations:** Eliminates the need for sidecar injection and management, reducing operational complexity and cognitive load.
* **Reduced Resource Consumption:** By centralizing mTLS and L4 policy enforcement in the `ztunnel`, ambient mode significantly lowers resource overhead per pod.
* **Incremental Adoption:** Allows for gradual adoption of service mesh features. Workloads can join the mesh at L4 for mTLS and basic policy, and then selectively opt-in for L7 features via `waypoint` proxies as needed.
* **Enhanced Security:** Provides a secure, zero-trust network foundation with mTLS by default for all meshed workloads.

**Trade-offs:**

* Ambient mode is a newer architecture and may have different operational considerations compared to the traditional sidecar model.
* L7 features require the deployment of `waypoint` proxies, which add a small amount of overhead for the services that utilize them.

---

## 2. Pre-requisites to Using Waypoint Ambient Mode with OSSM 3

Before installing Istio's ambient mode with OpenShift Service Mesh, ensure the following prerequisites are met:

* **OpenShift Container Platform 4.19+:** This version of OpenShift is required for supported Kubernetes Gateway API CRDs, which are essential for ambient mode functionalities.
* **OpenShift Service Mesh 3.1.0+ operator is installed:** Ensure that the OSSM operator version 3.1.0 or later is installed on your OpenShift cluster.
* **Istio deployed in Ambient Mode:** Refer to OSSM Ambient [initial doc](README.md).

**Pre-existing Service Mesh Installations:**

While the use of properly defined discovery selectors will allow a service mesh to be deployed in ambient mode alongside a service mesh in sidecar mode, this is not a scenario we have thoroughly validated. To avoid potential conflicts, as a technology preview feature, Istio's ambient mode should only be installed on clusters without a pre-existing OpenShift Service Mesh installation.

**Note:**: Istio's ambient mode is completely incompatible with clusters containing the OpenShift Service Mesh 2.6 or earlier versions of the operator and they should not be used together.

---

## 3. Waypoint Proxies

Ambient mesh splits Istio's functionality into two distinct layers, a secure overlay layer 4 (L4) and a Layer 7 (L7). The waypoint proxy is an optional component that is Envoy-based and handles L7 processing for workloads it manages. It acts as a gateway to a resource (a namespace, service or pod). Waypoint proxies are installed, upgraded and scaled independently from applications. They can be configured using the Kubernetes [Gateway API](https://gateway-api.sigs.k8s.io/).

### 3.1 Core features

If your applications require any of the following L7 mesh functions, you will need to use a waypoint proxy in ambient mode:

- **Traffic management:** HTTP routing & load balancing, circuit breaking, rate limiting, fault injection, retries and timeouts
- **Security:** Rich authorization policies based on L7 primitives such as request type or HTTP header
- **Observability**: HTTP metrics, access logging and tracing

### 3.2 Prerequisites

Waypoint proxies are deployed using Kubernetes Gateway resources.  
**Note:** As of OpenShift 4.17, the Kubernetes Gateway API CRDs are not available by default and must be installed to be used. This can be done with the following command:

```bash
kubectl get crd gateways.gateway.networking.k8s.io &> /dev/null || \
  { kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.0/standard-install.yaml; }
```

**Note:** As of OpenShift 4.19, the Kubernetes Gateway API CRDs comes pre-installed and could not be modified by the user.

### 3.3 Set up Istio Ambient Mode Resources and a Sample Application

1. Install the Sail Operator along with Istio in ambient mode using the following [steps](README.md#3-procedure-to-install-istios-ambient-mode). 

2. Deploy the sample Bookinfo applications. The steps can be found [here](README.md#36-about-the-bookinfo-application). 

Before you deploy a waypoint proxy in the application namespace, confirm the namespace is labeled with `istio.io/dataplane-mode: ambient`.

### 3.4 Deploy a Waypoint Proxy

1. Deploy a waypoint proxy in the bookinfo application namespace:

```bash
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
```

2. Apply the Waypoint CR.

```bash
$ oc apply -f waypoint.yaml
```

The `istio.io/waypoint-for: service` label on the Gateway resource specifies that it processes traffic for services, which is the default behavior. The type of traffic a waypoint handles is determined by this label. For more details you can refer to Istio [documentation](https://istio.io/latest/docs/ambient/usage/waypoint/#waypoint-traffic-types).

3. Enroll the bookinfo namespace to use the waypoint:

```bash
$ oc label namespace bookinfo istio.io/use-waypoint=waypoint
```

After a namespace is enrolled to use a waypoint, any requests from any pods using the ambient data plane mode, to any service running in the `bookinfo` namespace, will be routed through the waypoint for L7 processing and policy enforcement. 

If you prefer more granularity than using a waypoint for an entire namespace, you can enroll only a specific service or pod to use a waypoint by labeling the respective service or the pod. When enrolling a pod explicitly, you must also add the `istio.io/waypoint-for: workload` label to the Gateway resource.

### 3.5 Cross-namespace Waypoint

1. By default, a waypoint is usable only within the same namespace, but it also supports cross-namespace usage. The following Gateway allows resources in the `bookinfo` namespace to use `waypoint-foo` from the `foo` namespace:

```bash
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

2. Apply the cross namespace waypoint

```bash
$ oc apply -f cross-ns-waypoint.yaml
```

3. By default, the Istio control plane will look for a waypoint specified using the `istio.io/use-waypoint` label in the same namespace as the resource which the label is applied to. You can add labels `istio.io/use-waypoint-namespace` and `istio.io/use-waypoint` together to start using the cross-namespace waypoint.

```bash
$ oc label namespace bookinfo istio.io/use-waypoint-namespace=foo
$ oc label namespace bookinfo istio.io/use-waypoint=waypoint-foo
```

---

## 4. Layer 7 Features in Ambient Mode

The following section describes the stable features using Gateway API resource `HTTPRoute` and Istio resource `AuthorizationPolicy`. Other L7 features using a waypoint proxy will be discussed when they reach to Beta status. 

### 4.1 Traffic Routing

With a waypoint proxy deployed, you can split traffic between different versions of the bookinfo reviews service. This is useful for testing new features or performing A/B testing.

1. For example, let’s configure traffic routing to send 90% of requests to reviews-v1 and 10% to reviews-v2:

```bash
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
```

2. Apply the traffic routing configuration CR.

```bash
$ oc apply -f traffic-route.yaml
```

If you open the Bookinfo application in your browser and refresh the page multiple times, you'll notice that most requests (90%) go to `reviews-v1`, which don’t have any stars, while a smaller portion (10%) go to `reviews-v2`, which display black stars.

### 4.2 Security Authorization

The `AuthorizationPolicy` resource can be used in both sidecar mode and ambient mode. In ambient mode, authorization policies can either be targeted (for ztunnel enforcement) or attached (for waypoint enforcement). For an authorization policy to be attached to a waypoint it must have a `targetRef` which refers to the waypoint, or a Service which uses that waypoint.

When a waypoint proxy is added to a workload, you may have two possible places where you can enforce L4 policy (L7 policy can only be enforced at the waypoint proxy). Ideally you should attach your policy to the waypoint proxy, because the destination ztunnel will see traffic with the waypoint’s identity, not the source identity once you have introduced a waypoint to the traffic path.

1. For example, let's add a L7 authorization policy that will explicitly allow a curl service to send `GET` requests to the `productpage` service, but perform no other operations:

```bash
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
```

2. Apply the authorization policy CR.

```bash
$ oc apply -f authorization-policy.yaml
```

Note the targetRefs field is used to specify the target service for the authorization policy of a waypoint proxy. 

### 4.3 Security Authentication

Istio’s peer authentication policies, which configure mutual TLS (mTLS) modes, are supported by ztunnel.
The difference of that between sidecar mode and ambient mode is the `DISABLE` mode policies. They will be ignored in ambient mode because ztunnel and HBONE implies the use of mTLS.
More information about Istio's peer authentication behavior can be found [here](https://istio.io/latest/docs/concepts/security/#peer-authentication). 
