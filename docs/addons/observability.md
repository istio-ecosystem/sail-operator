[Return to Project Root](../README.md)

# Table of Contents

- [Observability Integrations](#observability-integrations)
  - [Scraping metrics using the OpenShift monitoring stack](#scraping-metrics-using-the-openshift-monitoring-stack)
  - [Configure tracing with OpenShift distributed tracing](#configure-tracing-with-openshift-distributed-tracing)
  - [Integrating with Kiali](#integrating-with-kiali)
    - [Integrating Kiali with the OpenShift monitoring stack](#integrating-kiali-with-the-openshift-monitoring-stack)
    - [Integrating Kiali with OpenShift Distributed Tracing](#integrating-kiali-with-openshift-distributed-tracing)

## Observability Integrations

### Scraping metrics using the OpenShift monitoring stack
The easiest way to get started with production-grade metrics collection is to use OpenShift's user-workload monitoring stack. The following steps assume that you installed Istio into the `istio-system` namespace. Note that these steps are not specific to the Sail Operator, but describe how to configure user-workload monitoring for Istio in general.

*Prerequisites*
* User Workload monitoring is [enabled](https://docs.openshift.com/container-platform/latest/observability/monitoring/enabling-monitoring-for-user-defined-projects.html)

*Steps*
1. Create a ServiceMonitor for istiod.

    ```yaml
    apiVersion: monitoring.coreos.com/v1
    kind: ServiceMonitor
    metadata:
      name: istiod-monitor
      namespace: istio-system 
    spec:
      targetLabels:
      - app
      selector:
        matchLabels:
          istio: pilot
      endpoints:
      - port: http-monitoring
        interval: 30s
    ```
1. Create a PodMonitor to scrape metrics from the istio-proxy containers. Note that *this resource has to be created in all namespaces where you are running sidecars*.

    ```yaml
    apiVersion: monitoring.coreos.com/v1
    kind: PodMonitor
    metadata:
      name: istio-proxies-monitor
      namespace: istio-system 
    spec:
      selector:
        matchExpressions:
        - key: istio-prometheus-ignore
          operator: DoesNotExist
      podMetricsEndpoints:
      - path: /stats/prometheus
        interval: 30s
        relabelings:
        - action: keep
          sourceLabels: ["__meta_kubernetes_pod_container_name"]
          regex: "istio-proxy"
        - action: keep
          sourceLabels: ["__meta_kubernetes_pod_annotationpresent_prometheus_io_scrape"]
        - action: replace
          regex: (\d+);(([A-Fa-f0-9]{1,4}::?){1,7}[A-Fa-f0-9]{1,4})
          replacement: "[$2]:$1"
          sourceLabels: ["__meta_kubernetes_pod_annotation_prometheus_io_port","__meta_kubernetes_pod_ip"]
          targetLabel: "__address__"
        - action: replace
          regex: (\d+);((([0-9]+?)(\.|$)){4})
          replacement: "$2:$1"
          sourceLabels: ["__meta_kubernetes_pod_annotation_prometheus_io_port","__meta_kubernetes_pod_ip"]
          targetLabel: "__address__"
        - action: labeldrop
          regex: "__meta_kubernetes_pod_label_(.+)"
        - sourceLabels: ["__meta_kubernetes_namespace"]
          action: replace
          targetLabel: namespace
        - sourceLabels: ["__meta_kubernetes_pod_name"]
          action: replace
          targetLabel: pod_name
    ```

Congratulations! You should now be able to see your control plane and data plane metrics in the OpenShift Console. Just go to Observe -> Metrics and try the query `istio_requests_total`.

### Configure tracing with OpenShift distributed tracing
This section describes how to setup Istio with OpenShift Distributed Tracing to send distributed traces.

*Prerequisites*
* A Tempo stack is installed and configured
* An instance of an OpenTelemetry collector is already configured in the istio-system namespace
* An Istio instance is created with the `openshift` profile
* An Istio CNI instance is created with the `openshift` profile

*Steps*
1. Configure Istio to enable tracing and include the OpenTelemetry settings:
    ```yaml
    meshConfig:
      enableTracing: true
      extensionProviders:
      - name: otel-tracing
        opentelemetry:
          port: 4317
          service: otel-collector.istio-system.svc.cluster.local 
    ```
The *service* field is the OpenTelemetry collector service in the `istio-system` namespace.

2. Create an Istio telemetry resource to active the OpenTelemetry tracer
    ```yaml
    apiVersion: telemetry.istio.io/v1
    kind: Telemetry
    metadata:
      name: otel-demo
      namespace: istio-system
    spec:
      tracing:
      - providers:
          - name: otel-tracing
            randomSamplingPercentage: 100
    ```

3. Validate the integration: Generate some traffic

We can [Deploy Bookinfo](addons.md#deploy-gateway-and-bookinfo) and generate some traffic.

4. Validate the integration: See the traces in the UI

```bash
kubectl get routes -n tempo tempo-sample-query-frontend-tempo
```

If you [configure Kiali with OpenShift distributed tracing](#integrating-kiali-with-openshift-distributed-tracing) you can verify from there. 

### Integrating with Kiali
Integration with Kiali really depends on how you collect your metrics and traces. Note that Kiali is a separate project which for the purpose of this document we'll expect is installed using the Kiali operator. The steps here are not specific to Sail Operator, but describe how to configure Kiali for use with Istio in general.

#### Integrating Kiali with the OpenShift monitoring stack
If you followed [Scraping metrics using the OpenShift monitoring stack](#scraping-metrics-using-the-openshift-monitoring-stack), you can set up Kiali to retrieve metrics from there.

*Prerequisites*
* User Workload monitoring is [enabled](https://docs.openshift.com/container-platform/latest/observability/monitoring/enabling-monitoring-for-user-defined-projects.html) and [configured](#scraping-metrics-using-the-openshift-monitoring-stack)
* Kiali Operator is installed

*Steps*
1. Create a ClusterRoleBinding for Kiali, so it can view metrics from user-workload monitoring

    ```yaml
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRoleBinding
    metadata:
      name: kiali-monitoring-rbac
    roleRef:
      apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: cluster-monitoring-view
    subjects:
    - kind: ServiceAccount
      name: kiali-service-account
      namespace: istio-system
    ```
1. Find out the revision name of your Istio instance. In our case it is `test`.
    
    ```console
    kubectl get istiorevisions.sailoperator.io
    NAME   READY   STATUS    IN USE   VERSION   AGE
    test   True    Healthy   True     v1.21.0   119m
    ```
1. Create a Kiali resource and point it to your Istio instance. Make sure to replace `test` with your revision name in the fields `config_map_name`, `istio_sidecar_injector_config_map_name`, `istiod_deployment_name` and `url_service_version`.

    ```yaml
    apiVersion: kiali.io/v1alpha1
    kind: Kiali
    metadata:
      name: kiali-user-workload-monitoring
      namespace: istio-system
    spec:
      external_services:
        istio:
          config_map_name: istio-test
          istio_sidecar_injector_config_map_name: istio-sidecar-injector-test
          istiod_deployment_name: istiod-test
          url_service_version: 'http://istiod-test.istio-system:15014/version'
        prometheus:
          auth:
            type: bearer
            use_kiali_token: true
          thanos_proxy:
            enabled: true
          url: https://thanos-querier.openshift-monitoring.svc.cluster.local:9091
    ```
#### Integrating Kiali with OpenShift Distributed Tracing
This section describes how to setup Kiali with OpenShift Distributed Tracing to read the distributed traces.

*Prerequisites*
* Istio tracing is [Configured with OpenShift distributed tracing](#configure-tracing-with-openshift-distributed-tracing) 

*Steps*
1. Setup Kiali to access traces from the Tempo frontend: 
    ```yaml
    external_services:
      grafana:
        enabled: true
        url: "http://grafana-istio-system.apps-crc.testing/"
      tracing:
        enabled: true
        provider: tempo
        use_grpc: false
        in_cluster_url: http://tempo-sample-query-frontend.tempo:3200
        url: 'https://tempo-sample-query-frontend-tempo.apps-crc.testing'
        tempo_config:
          org_id: "1"
          datasource_uid: "a8d2ef1c-d31c-4de5-a90b-e7bc5252cd00"
    ```

Where: 
* `external_services.grafana` section: Is just needed to see the "View in Tracing" link from the Traces tab
* `external_services.tracing.tempo_config`: Is just needed to see the "View in Tracing" link from the Traces tab and redirect to the proper Tempo datasource

Now, we should be able to see traces from Kiali. For this, you can: 
1. Select a Workload/Service/App
2. Click in the "Traces" tab