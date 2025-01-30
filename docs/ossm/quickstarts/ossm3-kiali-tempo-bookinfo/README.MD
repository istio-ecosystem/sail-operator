# Quickstart: OSSM3 with Kiali, Tempo, Bookinfo
This quickstart guide provides step-by-step instructions on how to set up OSSM3 with Kiali, Tempo, Open Telemetry, and Bookinfo app.
By the end of this quickstart, you will have installed OSSM3, where tracing information is collected by Open Telemetry Collector and Tempo, and monitoring is managed by an in-cluster monitoring stack. The Bookinfo sample application will be included in the service mesh, with a traffic generator sending one request per second to simualte traffic. Additionally, the Kiali UI and OSSMC plugin will be set up to provide a graphical overview.

## Prerequisites
- The OpenShift Service Mesh 3, Kiali, Tempo, Red Hat build of OpenTelemetry operators have been installed (you can install it by `./installOperators.sh` script which installs the particular operator versions (see subscriptions.yaml))
- The cluster that has available Persistent Volumes or supports dynamic provisioning storage (for installing MiniO)

## What is located where
The quickstart 
  * installs MiniO and Tempo to `tracing-system` namespace
  * installs OpenTelemetryCollector to `opentelemetrycollector` namespace
  * installs OSSM3 (Istio CR) with Kiali and OSSMC to `istio-system` namespace
  * installs IstioCNI to `istio-cni` namespace
  * installs Istio ingress gateway to `istio-ingress` namespace
  * installs bookinfo app with traffic generator in `bookinfo` namespace

## Shortcut to the end
To skip all the following steps and set everything up automatically (e.g., for demo purposes), simply run the prepared `./install_ossm3_demo.sh` script which will perform all steps automatically.

## Steps
All required YAML resources are in the `./resources` folder.
For a more detailed description about what is set and why, see OpenShift Service Mesh documentation.

Set up Tempo and OpenTelemetryCollector
------------
```bash
oc new-project tracing-system
```
First, set up MiniO storage which is used by Tempo to store data (or you can use S3 storage, see Tempo documentation)
```bash
oc apply -f ./resources/TempoOtel/minio.yaml -n tracing-system
oc wait --for condition=Available deployment/minio --timeout 150s -n tracing-system
```
Then, set up Tempo CR
```bash
oc apply -f ./resources/TempoOtel/tempo.yaml -n tracing-system
oc wait --for condition=Ready TempoStack/sample --timeout 150s -n tracing-system
oc wait --for condition=Available deployment/tempo-sample-compactor --timeout 150s -n tracing-system
```
Expose Jaeger UI route which will be used in the Kiali CR later
```bash
oc expose svc tempo-sample-query-frontend --port=jaeger-ui --name=tracing-ui -n tracing-system
```
Next, set up OpenTelemetryCollector
```bash
oc new-project opentelemetrycollector
oc apply -f ./resources/TempoOtel/opentelemetrycollector.yaml -n opentelemetrycollector
oc wait --for condition=Available deployment/otel-collector --timeout 60s -n opentelemetrycollector
```

Set up OSSM3
------------
```bash
oc new-project istio-system
```
First, install Istio custom resource
> **_NOTE:_**  In this example, the `.spec.version` is missing so the istio version is automatically set by OSSM operator. You can specify the version manually, but it must be one that is supported by the operator; otherwise, a validation error will occur.
```bash
oc apply -f ./resources/OSSM3/istiocr.yaml  -n istio-system
oc wait --for condition=Ready istio/default --timeout 60s  -n istio-system
```
> **_NOTE:_** When you name your IstioCR differently as `default`, you need to use `istio.io/rev=<istioCR_NAME>` label instead of `istio-injection=enabled` in the all next steps of this example. 
> 
> Also, you will need to update values `config_map_name`, `istio_sidecar_injector_config_map_name`, `istiod_deployment_name`, `url_service_version` in the Kiali CR with `-<istioCR_NAME>` suffix.
> 
> e.g. when Istio cr name is `default`:
> ```
>      config_map_name: istio
>      istio_sidecar_injector_config_map_name: istio-sidecar-injector
>      istiod_deployment_name: istiod
>      url_service_version: 'http://istiod.istio-system:15014/version'
>```
> e.g. when Istio cr name is `main` (or something else):
> ```
>      config_map_name: istio-main
>      istio_sidecar_injector_config_map_name: istio-sidecar-injector-main
>      istiod_deployment_name: istiod-main
>      url_service_version: 'http://istiod-main.istio-system:15014/version'
>```

Then, set up Telemetry resource to enable tracers defined in Istio custom resource
```bash
oc apply -f ./resources/TempoOtel/istioTelemetry.yaml  -n istio-system
```
The opentelemetrycollector namespace needs to be added as a member of the mesh
```bash
oc label namespace opentelemetrycollector istio-injection=enabled
```

Then, install IstioCNI
> **_NOTE:_**  In this example, the `.spec.version` is missing so the istio version is automatically set by OSSM operator. the `.spec.version` is missing so the istio version is automatically set by OSSM operator. You can specify the version manually, but it must be one that is supported by the operator.
```bash
oc new-project istio-cni
oc apply -f ./resources/OSSM3/istioCni.yaml -n istio-cni
oc wait --for condition=Ready istiocni/default --timeout 60s -n istio-cni
```

Set up the ingress gateway in a different namespace as istio-system.
Add that namespace as a member of the mesh.
```bash
oc new-project istio-ingress
oc label namespace istio-ingress istio-injection=enabled
oc apply -f ./resources/OSSM3/istioIngressGateway.yaml  -n istio-ingress
oc wait --for condition=Available deployment/istio-ingressgateway --timeout 60s -n istio-ingress
```
Expose Istio ingress route which will be used in the bookinfo traffic generator later (and via that URL, we will be accessing to the bookinfo app)
```bash
oc expose svc istio-ingressgateway --port=http2 --name=istio-ingressgateway -n istio-ingress
```

Set up OCP user monitoring workflow
------------
First, OCP user monitoring needs to be enabled
```bash
oc apply -f ./resources/Monitoring/ocpUserMonitoring.yaml
```
Then, create service monitor and pod monitor for istio namespaces
```bash
oc apply -f ./resources/Monitoring/serviceMonitor.yaml -n istio-system
oc apply -f ./resources/Monitoring/podMonitor.yaml -n istio-system
oc apply -f ./resources/Monitoring/podMonitor.yaml -n istio-ingress
```

Set up Kiali
------------
Create cluster role binding for kiali to be able to read ocp monitoring
```bash
oc apply -f ./resources/Kiali/kialiCrb.yaml -n istio-system
```
Set up Kiali CR. The URL for Jaeger UI (which was exposed earlier) needs to be set to Kiali CR in `.spec.external_services.tracing.url`
> **_NOTE:_**  In this example, the `.spec.version` is missing so the istio version is automatically set by Kiali operator. You can specify the version manually, but it must be one that is supported by the operator; otherwise, an error will appear in events on the Kiali resource.
```bash
export TRACING_INGRESS_ROUTE="http://$(oc get -n tracing-system route tracing-ui -o jsonpath='{.spec.host}')"
cat ./resources/Kiali/kialiCr.yaml | JAEGERROUTE="${TRACING_INGRESS_ROUTE}" envsubst | oc -n istio-system apply -f -
oc wait --for condition=Successful kiali/kiali --timeout 150s -n istio-system 
```
Increase timeout for the Kiali ui route in OCP since big queries for spans can take longer
```bash
oc annotate route kiali haproxy.router.openshift.io/timeout=60s -n istio-system
```
Optionally, OSSMC plugin can be installed as well
> **_NOTE:_**  In this example, the `.spec.version` is missing so the istio version is automatically set by Kiali operator. You can specify the version manually, but it must be one that is supported by the operator and the version needs to be **the same as Kiali CR**.
```bash
oc apply -f ./resources/Kiali/kialiOssmcCr.yaml -n istio-system
oc wait -n istio-system --for=condition=Successful OSSMConsole ossmconsole --timeout 120s
```

Set up BookInfo
------------
Create bookinfo namespace and add that namespace as a member of the mesh
```bash
oc new-project bookinfo
oc label namespace bookinfo istio-injection=enabled
```
Create pod monitor for bookinfo namespaces
```bash
oc apply -f ./resources/Monitoring/podMonitor.yaml -n bookinfo
```
> **_NOTE(shortcut):_**  It takes some time till pod monitor shows in Metrics targets, you can check it in OCP console Observe->Targets. The Kiali UI will not show the metrics till the targets are ready.
 
Install the Bookinfo app (the bookinfo resources are from `release-1.23` istio release branch)
```bash
oc apply -f ./resources/Bookinfo/bookinfo.yaml -n bookinfo
oc apply -f ./resources/Bookinfo/bookinfo-gateway.yaml -n bookinfo
oc wait --for=condition=Ready pods --all -n bookinfo --timeout 60s
```

Optionally, install a traffic generator for booking app which every second generates a request to simulate traffic
```bash
export INGRESSHOST=$(oc get route istio-ingressgateway -n istio-ingress -o=jsonpath='{.spec.host}')
cat ./resources/Bookinfo/traffic-generator-configmap.yaml | ROUTE="http://${INGRESSHOST}/productpage" envsubst | oc -n bookinfo apply -f - 
oc apply -f ./resources/Bookinfo/traffic-generator.yaml -n bookinfo
```

Test that everything works correctly
------------
Now, everything should be set.
Check the Bookinfo app via the ingress route
```bash
INGRESSHOST=$(oc get route istio-ingressgateway -n istio-ingress -o=jsonpath='{.spec.host}')
echo "http://${INGRESSHOST}/productpage"
```
Check Kiali UI
```bash
KIALI_HOST=$(oc get route kiali -n istio-system -o=jsonpath='{.spec.host}')
echo "https://${KIALI_HOST}"
```
You can check all namespaces that all pods running correctly:
```bash
oc get pods -n tracing-system
oc get pods -n opentelemetrycollector
oc get pods -n istio-system
oc get pods -n istio-cni
oc get pods -n istio-ingress
oc get pods -n bookinfo
```
Output (the number of istio-cni pods is equals to the number of OCP nodes):
```bash
NAME                                           READY   STATUS    RESTARTS   AGE
minio-6f8c5c79-fmjpd                           1/1     Running   0          10m
tempo-sample-compactor-dcffd76dc-7mnll         1/1     Running   0          10m
tempo-sample-distributor-7dbbf4b5d7-xw5w5      1/1     Running   0          10m
tempo-sample-ingester-0                        1/1     Running   0          10m
tempo-sample-querier-7bbcc6dd9b-gtl4q          1/1     Running   0          10m
tempo-sample-query-frontend-5885fff6bf-cklc5   2/2     Running   0          10m
NAME                              READY   STATUS    RESTARTS   AGE
otel-collector-77b6b4b58d-dwk6q   1/1     Running   0          9m23s
NAME                           READY   STATUS    RESTARTS   AGE
istiod-6847b886d5-s8vz8        1/1     Running   0          9m8s
kiali-6b7dbdf67b-cczm5         1/1     Running   0          7m56s
ossmconsole-7b64979c75-f9fbf   1/1     Running   0          7m22s
NAME                   READY   STATUS    RESTARTS   AGE
istio-cni-node-8h4mr   1/1     Running   0          8m44s
istio-cni-node-qvmw4   1/1     Running   0          8m44s
istio-cni-node-vpv9v   1/1     Running   0          8m44s
istio-cni-node-wml9b   1/1     Running   0          8m44s
istio-cni-node-x8np2   1/1     Running   0          8m44s
NAME                                    READY   STATUS    RESTARTS   AGE
istio-ingressgateway-7f8878b6b4-6k8tj   1/1     Running   0          8m19s
istio-ingressgateway-7f8878b6b4-f5744   1/1     Running   0          8m36s
NAME                             READY   STATUS    RESTARTS   AGE
details-v1-65cfcf56f9-72k5p      2/2     Running   0          3m4s
kiali-traffic-generator-cblht    2/2     Running   0          77s
productpage-v1-d5789fdfb-rlkhl   2/2     Running   0          3m
ratings-v1-7c9bd4b87f-5qmmp      2/2     Running   0          3m3s
reviews-v1-6584ddcf65-mhd75      2/2     Running   0          3m2s
reviews-v2-6f85cb9b7c-q8mc2      2/2     Running   0          3m2s
reviews-v3-6f5b775685-ctb65      2/2     Running   0          3m1s
```