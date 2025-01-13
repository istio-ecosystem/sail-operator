## Creating and Configuring Gateways

The Sail Operator does not deploy Ingress or Egress Gateways. Gateways are not 
part of the control plane. As a security best-practice, Ingress and Egress 
Gateways should be deployed in a different namespace than the namespace that 
contains the control plane.

You can deploy gateways using either the Gateway API or Gateway Injection methods. 


### Option 1: Istio Gateway Injection

Gateway Injection uses the same mechanisms as Istio sidecar injection to create 
a gateway from a `Deployment` resource that is paired with a `Service` resource 
that can be made accessible from outside the cluster. For more information, see 
[Installing Gateways](https://istio.io/latest/docs/setup/additional-setup/gateway/#deploying-a-gateway).

To configure gateway injection with the `bookinfo` application, we have provided 
a [sample gateway configuration](../../chart/samples/ingress-gateway.yaml?raw=1) that should be applied in the namespace 
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
