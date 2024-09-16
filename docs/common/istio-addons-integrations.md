## Istio Addons Integrations

Istio can be integrated with other software to provide additional functionality 
(More information can be found in: https://istio.io/latest/docs/ops/integrations/). 
The following addons are for demonstration or development purposes only and 
should not be used in production environments:


### Prometheus

`Prometheus` is an open-source systems monitoring and alerting toolkit. You can 
use `Prometheus` with the Sail Operator to keep an eye on how healthy Istio and 
the apps in the service mesh are, for more information, see [Prometheus](https://istio.io/latest/docs/ops/integrations/prometheus/). 

To install Prometheus, perform the following steps:

1. Deploy `Prometheus`:

    ```sh
    $ oc apply -f https://raw.githubusercontent.com/istio/istio/master/samples/addons/prometheus.yaml
    ```
2. Access to `Prometheus`console:

    * Expose the `Prometheus` service externally:
    
    ```sh
    $ oc expose service prometheus -n istio-system
    ```
    * Get the route of the service and open the URL in the web browser
    
    ```sh
    $ oc get route prometheus -o jsonpath='{.spec.host}' -n istio-system
    ```


### Grafana

`Grafana` is an open-source platform for monitoring and observability. You can 
use `Grafana` with the Sail Operator to configure dashboards for istio, see 
[Grafana](https://istio.io/latest/docs/ops/integrations/grafana/) for more information. 

To install Grafana, perform the following steps:

1. Deploy `Grafana`:
    
    ```sh
    $ oc apply -f https://raw.githubusercontent.com/istio/istio/master/samples/addons/grafana.yaml
    ```

2. Access to `Grafana`console:

    * Expose the `Grafana` service externally
    
    ```sh
    $ oc expose service grafana -n istio-system
    ```
    * Get the route of the service and open the URL in the web browser
    
    ```sh
    $ oc get route grafana -o jsonpath='{.spec.host}' -n istio-system
    ```


### Jaeger

`Jaeger` is an open-source end-to-end distributed tracing system. You can use 
`Jaeger` with the Sail Operator to monitor and troubleshoot transactions in 
complex distributed systems, see [Jaeger](https://istio.io/latest/docs/ops/integrations/jaeger/) for more information. 

To install Jaeger, perform the following steps:

1. Deploy `Jaeger`:
    
    ```sh
    $ oc apply -f https://raw.githubusercontent.com/istio/istio/master/samples/addons/jaeger.yaml
    ```
2. Access to `Jaeger` console:

    * Expose the `Jaeger` service externally:

        ```sh
        $ oc expose svc/tracing -n istio-system
        ```

    * Get the route of the service and open the URL in the web browser

        ```sh
        $ oc get route tracing -o jsonpath='{.spec.host}' -n istio-system
        ```
*Note*: if you want to see some traces you can refresh several times the product 
page of bookinfo app to start generating traces.


### Kiali

`Kiali` is an open-source project that provides a graphical user interface to 
visualize the service mesh topology, see [Kiali](https://istio.io/latest/docs/ops/integrations/kiali/) for more information. 

To install Kiali, perform the following steps:

1. Deploy `Kiali`:
    
    ```sh
    $ oc apply -f https://raw.githubusercontent.com/istio/istio/master/samples/addons/kiali.yaml
    ```

2. Access to `Kiali` console:

    * Expose the `Kiali` service externally:

        ```sh
        $ oc expose service kiali -n istio-system
        ```

    * Get the route of the service and open the URL in the web browser

        ```sh
        $ oc get route kiali -o jsonpath='{.spec.host}' -n istio-system
        ```
