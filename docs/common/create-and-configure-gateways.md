[Return to Project Root](../README.md)

# Table of Contents

- [Creating and Configuring Gateways](#creating-and-configuring-gateways)
    - [Option 1: Istio Gateway Injection](#option-1-istio-gateway-injection)
        - [Ingress Gateway](#ingress-gateway)
        - [Egress Gateway](#egress-gateway)
    - [Option 2: Kubernetes Gateway API](#option-2-kubernetes-gateway-api)

## Creating and Configuring Gateways

[Gateways in Istio](https://istio.io/latest/docs/concepts/traffic-management/#gateways) are used to manage inbound and outbound traffic for the mesh. The Sail Operator does not deploy or manage Gateways. You can deploy a gateway either through [gateway-api](https://istio.io/latest/docs/tasks/traffic-management/ingress/gateway-api/) or through [gateway injection](https://istio.io/latest/docs/setup/additional-setup/gateway/#deploying-a-gateway). As you are following the gateway installation instructions, skip the step to install Istio since this is handled by the Sail Operator.

**Note:** The `IstioOperator` / `istioctl` example is separate from the Sail Operator. Setting `spec.components` or `spec.values.gateways` on your Sail Operator `Istio` resource **will not work**.

### Option 1: Istio Gateway Injection

Gateway Injection uses the same mechanisms as Istio sidecar injection to create
a gateway from a `Deployment` resource that is paired with a `Service` resource
that can be made accessible from outside the cluster (ingress) or to control
outbound traffic from the mesh (egress). For more information, see
[Installing Gateways](https://istio.io/latest/docs/setup/additional-setup/gateway/#deploying-a-gateway).

#### Ingress Gateway

To configure ingress gateway injection with the `bookinfo` application, we have provided
a [sample gateway configuration](../../chart/samples/ingress-gateway.yaml) that should be applied in the namespace
where the application is installed:

1. Create the `istio-ingressgateway` deployment and service:

    ```sh
    $ oc apply -f ingress-gateway.yaml
    ```

2. Configure the `bookinfo` application with the new gateway:

    ```sh
    $ oc apply -f https://raw.githubusercontent.com/istio/istio/master/samples/bookinfo/networking/bookinfo-gateway.yaml
    ```

3. On OpenShift, you can use a [Route](https://docs.openshift.com/container-platform/4.13/networking/routes/route-configuration.html) to expose the gateway externally: 

    ```sh
    $ oc expose service istio-ingressgateway
    ```

4. Finally, obtain the gateway host name and the URL of the product page:

    ```sh
    $ HOST=$(oc get route istio-ingressgateway -o jsonpath='{.spec.host}')
    $ echo http://$HOST/productpage
    ```

Verify that the `productpage` is accessible from a web browser. 

#### Egress Gateway

An egress gateway allows you to control outbound traffic from the service mesh, providing security and monitoring capabilities for external service access. Here's how to configure an egress gateway using gateway injection:

1. Create the `istio-egressgateway` namespace:

    ```sh
    $ oc create namespace istio-egressgateway
    ```

2. Create the `istio-egressgateway` deployment and service using the provided [sample egress gateway configuration](../../chart/samples/egress-gateway.yaml?raw=1):

    ```sh
    $ oc apply -f egress-gateway.yaml -n istio-egressgateway
    ```

3. Configure traffic routing to use the egress gateway. For example, to route traffic to `httpbin.org` through the egress gateway:

    ```yaml
    apiVersion: networking.istio.io/v1beta1
    kind: ServiceEntry
    metadata:
      name: httpbin-ext
    spec:
      hosts:
      - httpbin.org
      ports:
      - number: 80
        name: http
        protocol: HTTP
      location: MESH_EXTERNAL
      resolution: DNS
    ---
    apiVersion: networking.istio.io/v1beta1
    kind: DestinationRule
    metadata:
      name: egressgateway-for-httpbin
    spec:
      host: istio-egressgateway.istio-egressgateway.svc.cluster.local
      subsets:
      - name: httpbin
    ---
    apiVersion: networking.istio.io/v1beta1
    kind: VirtualService
    metadata:
      name: direct-httpbin-through-egress-gateway
    spec:
      hosts:
      - httpbin.org
      gateways:
      - mesh
      http:
      - match:
        - port: 80
        route:
        - destination:
            host: istio-egressgateway.istio-egressgateway.svc.cluster.local
            subset: httpbin
            port:
              number: 80
    ```

   Apply this configuration:

    ```sh
    $ oc apply -f egress-gateway-config.yaml
    ```

4. Test the egress gateway by making a request from a pod in the mesh:

    ```sh
    $ oc exec -it $(oc get pod -l app=productpage -o jsonpath='{.items[0].metadata.name}') -c productpage -- curl -s http://httpbin.org/ip
    ```

**Note:** With gateway injection, the gateway proxy automatically handles the routing configuration for the injected workload. The VirtualService above directs mesh traffic to the egress gateway service, and the gateway proxy forwards it to the external destination. This approach provides centralized egress traffic monitoring, policy enforcement, and security controls for outbound traffic.

### Option 2: Kubernetes Gateway API

Istio includes support for Kubernetes [Gateway API](https://gateway-api.sigs.k8s.io/) and intends to make it 
the default API for [traffic management in the future](https://istio.io/latest/blog/2022/gateway-api-beta/). For more 
information, see Istio's [Kubernetes Gateway API](https://istio.io/latest/docs/tasks/traffic-management/ingress/gateway-api/) page.

As of Kubernetes 1.28 and OpenShift 4.14, the Kubernetes Gateway API CRDs are 
not available by default and must be enabled to be used. This can be done with 
the command:

```sh
$ oc get crd gateways.gateway.networking.k8s.io &> /dev/null ||  { oc kustomize "github.com/kubernetes-sigs/gateway-api/config/crd?ref=v1.0.0" | oc apply -f -; }
```

To configure `bookinfo` with a gateway using `Gateway API`:

1. Create and configure a gateway using a `Gateway` and `HTTPRoute` resource:

    ```sh
    $ oc apply -f https://raw.githubusercontent.com/istio/istio/master/samples/bookinfo/gateway-api/bookinfo-gateway.yaml
    ```

2. Retrieve the host, port and gateway URL:

    ```sh
    $ export INGRESS_HOST=$(oc get gtw bookinfo-gateway -o jsonpath='{.status.addresses[0].value}')
    $ export INGRESS_PORT=$(oc get gtw bookinfo-gateway -o jsonpath='{.spec.listeners[?(@.name=="http")].port}')
    $ export GATEWAY_URL=$INGRESS_HOST:$INGRESS_PORT
    ```

3. Obtain the `productpage` URL and check that you can visit it from a browser:

   ```sh
    $ echo "http://${GATEWAY_URL}/productpage"
    ```
