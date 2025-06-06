[Return to Project Root](../README.md)

# Table of Contents

- [Addons](#addons)
  - [Deploy Prometheus and Jaeger addons](#deploy-prometheus-and-jaeger-addons)
  - [Deploy Kiali addon](#deploy-kiali-addon)
  - [Find the active revision of your Istio instance. In our case it is `test`.](#find-the-active-revision-of-your-istio-instance-in-our-case-it-is-test)
  - [Deploy Gateway and Bookinfo](#deploy-gateway-and-bookinfo)
  - [Generate traffic and visualize your mesh](#generate-traffic-and-visualize-your-mesh)

## Addons

Addons are managed separately from the Sail Operator. You can follow the [istio documentation](https://istio.io/latest/docs/ops/integrations/) for how to install addons. Below is an example of how to install some addons for Istio.

The sample will deploy:

- Prometheus
- Jaeger
- Kiali
- Bookinfo demo app

*Prerequisites*

- Sail operator is installed.
- Control Plane is installed via the Sail Operator.

### Deploy Prometheus and Jaeger addons

```bash
kubectl apply -f https://raw.githubusercontent.com/istio/istio/master/samples/addons/prometheus.yaml
kubectl apply -f https://raw.githubusercontent.com/istio/istio/master/samples/addons/jaeger.yaml
```

### Deploy Kiali addon

Install the kiali operator.

You can install the kiali operator through OLM if running on Openshift, otherwise you can use helm:

```bash
helm install --namespace kiali-operator --create-namespace kiali-operator kiali/kiali-operator
```

Create a Kiali resource. We're enabling tracing and disabling grafana here since tracing is disabled by default and grafana is not part of this example.

```bash
kubectl apply -f - <<EOF
apiVersion: kiali.io/v1alpha1
kind: Kiali
metadata:
  name: kiali
  namespace: istio-system
spec:
  external_services:
    tracing:
      enabled: true
    grafana:
      enabled: false
EOF
```

### Find the active revision of your Istio instance.
In our case it is `test` the istio resource name.

```console
$ kubectl get istios
NAME      REVISIONS   READY   IN USE   ACTIVE REVISION   STATUS    VERSION   AGE
test      1           1       0        test              Healthy   v1.24.3   8m10s
```

### Deploy Gateway and Bookinfo

Create the bookinfo namespace (if it doesn't already exist) and enable injection.

```bash
kubectl create namespace bookinfo
kubectl label namespace bookinfo istio.io/rev=test
```

Install Bookinfo demo app.

```bash
kubectl apply -n bookinfo -f https://raw.githubusercontent.com/istio/istio/master/samples/bookinfo/platform/kube/bookinfo.yaml
kubectl apply -n bookinfo -f https://raw.githubusercontent.com/istio/istio/master/samples/bookinfo/platform/kube/bookinfo-versions.yaml
```

Install gateway API CRDs if they are not already installed.

```bash
kubectl get crd gateways.gateway.networking.k8s.io &> /dev/null || \
  { kubectl kustomize "github.com/kubernetes-sigs/gateway-api/config/crd?ref=v1.1.0" | kubectl apply -f -; }
```

Create bookinfo gateway.

```bash
kubectl apply -n bookinfo -f https://raw.githubusercontent.com/istio/istio/master/samples/bookinfo/gateway-api/bookinfo-gateway.yaml
kubectl wait -n bookinfo --for=condition=programmed gtw bookinfo-gateway
```

### Generate traffic and visualize your mesh

Send traffic to the productpage service. Note that this command will run until canceled.

```bash
export INGRESS_HOST=$(kubectl get gtw bookinfo-gateway -n bookinfo -o jsonpath='{.status.addresses[0].value}')
export INGRESS_PORT=$(kubectl get gtw bookinfo-gateway -n bookinfo -o jsonpath='{.spec.listeners[?(@.name=="http")].port}')
export GATEWAY_URL=$INGRESS_HOST:$INGRESS_PORT
watch curl http://${GATEWAY_URL}/productpage &> /dev/null
```

In a separate terminal, open Kiali to visualize your mesh.

If using Openshift, open the Kiali route:

```bash
echo https://$(kubectl get routes -n istio-system kiali -o jsonpath='{.spec.host}')
```

Otherwise, port forward to the kiali pod directly:

```bash
kubectl port-forward -n istio-system svc/kiali 20001:20001
```

You can view Kiali dashboard at: http://localhost:20001
