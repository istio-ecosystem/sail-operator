# Post-quantum cryptography in Istio

## Setup environment

1. Install Istio 1.26 - the first version that supports `X25519MLKEM768` algorithm for key exchange:

    ```shell
    curl -L https://istio.io/downloadIstio | ISTIO_VERSION=1.26.0 sh -
    ```

1. Create KinD cluster with MetalLB:

    ```shell
    curl -s https://raw.githubusercontent.com/istio/istio/refs/heads/release-1.26/samples/kind-lb/setupkind.sh | sh -s -- --cluster-name test --worker-nodes 2
    ```

## Demo

1. Install Istio:

   ```shell
   cat <<EOF > istio.yaml
   apiVersion: install.istio.io/v1alpha1
   kind: IstioOperator
   spec:
     meshConfig:
       accessLogFile: /dev/stdout
       tlsDefaults:
         ecdhCurves:
         - X25519MLKEM768
   EOF
   ./istio-1.26.0/bin/istioctl install -f istio.yaml -y
   ```
   
1. [Generate client and server certificates and keys](https://istio.io/latest/docs/tasks/traffic-management/ingress/secure-ingress/#generate-client-and-server-certificates-and-keys).

1. Deploy an ingress gateway:

   ```shell
   kubectl create -n istio-system secret tls httpbin-credential \
     --key=example_certs1/httpbin.example.com.key \
     --cert=example_certs1/httpbin.example.com.crt
   ```
   ```shell
   kubectl apply -f - <<EOF
   apiVersion: networking.istio.io/v1
   kind: Gateway
   metadata:
     name: istio-ingressgateway
   spec:
     selector:
       istio: ingressgateway
     servers:
     - port:
         number: 443
         name: https
         protocol: HTTPS
       tls:
         mode: SIMPLE
         credentialName: httpbin-credential
       hosts:
       - httpbin.example.com
   ---
   apiVersion: networking.istio.io/v1
   kind: VirtualService
   metadata:
     name: httpbin
   spec:
     hosts:
     - "httpbin.example.com"
     gateways:
     - istio-ingressgateway
     http:
     - route:
       - destination:
           port:
             number: 8000
           host: httpbin
   EOF
   ```

1. Deploy httpbin server:

   ```shell
   kubectl label namespace default istio-injection=enabled
   kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.26/samples/httpbin/httpbin.yaml
   ```

1. Send a request to the server from OQS curl:
   
   ```shell
   INGRESS_IP=$(kubectl get svc istio-ingressgateway -n istio-system -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
   docker run \
       --network kind \
       -v ./example_certs1/example.com.crt:/etc/example_certs1/example.com.crt \
       --rm -it openquantumsafe/curl \
       curl -v \
       --curves X25519MLKEM768 \
       --cacert /etc/example_certs1/example.com.crt \
       -H "Host: httpbin.example.com" \
       --resolve "httpbin.example.com:443:$INGRESS_IP" \
       "https://httpbin.example.com:443/status/200"
   ```

## Demo - ambient mode and mesh-wide post-quantum-safe key exchange

1. Remove httpbin with sidecar:

   ```shell
   kubectl label namespace default istio-injection-
   kubectl delete -f https://raw.githubusercontent.com/istio/istio/release-1.26/samples/httpbin/httpbin.yaml
   ```

1. Create secret in default namespace:
    ```shell
    kubectl create secret tls httpbin-credential \
      --key=example_certs1/httpbin.example.com.key \
      --cert=example_certs1/httpbin.example.com.crt
    ```

1. Install Istio:

   ```shell
   cat <<EOF > istio.yaml
   apiVersion: install.istio.io/v1alpha1
   kind: IstioOperator
   spec:
     hub: quay.io/jewertow
     tag: pqc
     profile: ambient
     components:
       ingressGateways:
       - name: istio-ingressgateway
         namespace: default
         enabled: true
     meshConfig:
       accessLogFile: /dev/stdout
     values:
       global:
         variant: ""
       pilot:
         env:
           COMPLIANCE_POLICY: "pqc"
       ztunnel:
         env:
           COMPLIANCE_POLICY: "pqc"
   EOF
   ./istio-1.26.0/bin/istioctl install -f istio.yaml -y
   ```

1. Deploy sidecarless httpbin and sleep:

   ```shell
   kubectl label namespace default istio.io/dataplane-mode=ambient
   kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.26/samples/httpbin/httpbin.yaml
   kubectl apply -f https://raw.githubusercontent.com/istio/istio/release-1.26/samples/sleep/sleep.yaml
   ```

1. Enforce deploying applications on different nodes to test ztunnel to ztunnel traffic:

   ```shell
   kubectl patch deploy httpbin --type='json' -p='[{"op": "add", "path": "/spec/template/spec/nodeName", "value": "test-worker"}]'
   kubectl patch deploy sleep --type='json' -p='[{"op": "add", "path": "/spec/template/spec/nodeName", "value": "test-worker2"}]'
   ```

1. Send mesh-internal and mesh-external requests to httpbin:

   ```shell
   kubectl exec deploy/sleep -- curl -vs httpbin:8000/headers
   ```
   ```shell
   INGRESS_IP=$(kubectl get svc istio-ingressgateway -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
   docker run \
       --network kind \
       -v ./example_certs1/example.com.crt:/etc/example_certs1/example.com.crt \
       --rm -it openquantumsafe/curl \
       curl -v \
       --curves X25519MLKEM768 \
       --cacert /etc/example_certs1/example.com.crt \
       -H "Host: httpbin.example.com" \
       --resolve "httpbin.example.com:443:$INGRESS_IP" \
       "https://httpbin.example.com:443/status/200"
   ```

1. Deploy waypoint in the default namespace and send requests again:

   ```shell
   kubectl get crd gateways.gateway.networking.k8s.io &> /dev/null || \
   kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.3.0-rc.1/standard-install.yaml
   ```
   ```shell
   istioctl waypoint apply -n default --enroll-namespace
   ```
