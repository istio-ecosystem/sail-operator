# OpenShift Service Mesh 2 --> 3 Multi-tenancy Migration guide

This guide is for users who are currently running `MultiTenant` OpenShift Service Mesh 2.6 migrating to OpenShift Service Mesh 3.0. You should first read [this document comparing OpenShift Service Mesh 2 vs. OpenShift Service Mesh 3](../../ossm2-vs-ossm3.md) to familiarize yourself with the concepts between the two versions and the differences in manging `MultiTenant` workloads. Specifically the [Scoping the Mesh section](../../ossm2-vs-ossm3.md#scoping-of-the-mesh-discovery-selectors-and-labels-replace-servicemeshmemberroll-and-servicemeshmember) is important for migrating from OpenShift Service Mesh 2 to OpenShift Service Mesh 3.

## Migrating OpenShift Service Mesh 2 Multi-Tenant to OpenShift Service Mesh 3

### Prerequisites

- OSSM2 operator is installed
- OSSM3 operator is installed
- `IstioCNI` is installed
- `istioctl` is installed
- MultiTenant `ServiceMeshControlPlane`

### Procedure

In this example, we'll be using the [bookinfo demo](https://raw.githubusercontent.com/Maistra/istio/maistra-2.6/samples/bookinfo/platform/kube/bookinfo.yaml) but you can follow these same steps with your own workloads.

Before you begin, please ensure that you have completed all the steps in the [pre-migration checklist](../README.md#pre-migration-checklist).

<!-- Developer instructions for testing with a fresh 2.6 install.

1. Create SMCP namespace

```sh
oc create ns istio-system-tenant-a
```

1. Create SMCP

   ```yaml
   apiVersion: maistra.io/v2
   kind: ServiceMeshControlPlane
   metadata:
     name: basic
     namespace: istio-system-tenant-a
   spec:
     security:
       manageNetworkPolicy: false
       dataPlane:
         mtls: true
     addons:
       grafana:
         enabled: false
       kiali:
         enabled: false
       prometheus:
         enabled: false
     gateways:
       openshiftRoute:
         enabled: false
     mode: MultiTenant
     policy:
       type: Istiod
     profiles:
       - default
     telemetry:
       type: Istiod
     tracing:
       type: None
     version: v2.6
   ```

1. Create bookinfo namespace

   ```sh
   oc create ns bookinfo
   ```

1. Add bookinfo to the SMMR

   ```yaml
   apiVersion: maistra.io/v1
   kind: ServiceMeshMemberRoll
   metadata:
     name: default
     namespace: istio-system-tenant-a
   spec:
     members:
       - bookinfo
   ```

1. Deploy bookinfo

   ```sh
   oc apply -n bookinfo -f https://raw.githubusercontent.com/Maistra/istio/maistra-2.6/samples/bookinfo/platform/kube/bookinfo.yaml
   ```

1. Ensure pods are all healthy and you see `2/2` pods indicating a sidecar was injected.

   ```sh
   oc get pods -n bookinfo
   ```

   Example output

   ```sh
   NAME                             READY   STATUS    RESTARTS   AGE
   details-v1-75cb5b97b4-5c6nm      2/2     Running   0          6h13m
   productpage-v1-899d756d8-ch424   2/2     Running   0          6h10m
   ratings-v1-58757c649b-8bdg4      2/2     Running   0          6h13m
   reviews-v1-6878c868b6-42kw7      2/2     Running   0          6h13m
   reviews-v2-6c8bd45654-jpt76      2/2     Running   0          6h13m
   reviews-v3-57997d6ccd-j6pmh      2/2     Running   0          6h13m
   ``` -->

#### Install OpenShift Service Mesh 3.0

1. Create your `Istio` resource.

   Here we are setting `discoverySelectors` on our `Istio` resource. In 3.0, controlplanes by default watch the entire cluster and when managing multiple controlplanes on a single cluster, you must narrow the scope of each controlplane by setting `discoverySelectors`. In this example, we use the label `tenant` but you can use any label or combination of labels that you choose.

   > **_NOTE:_** For simplicity, we are using a minimal example for Istio resource. Read [SMCP to Istio mapping](#TODO) to see how to transform fields used in SMCP resource to fields in Istio resource.

   ```yaml
   apiVersion: sailoperator.io/v1alpha1
   kind: Istio
   metadata:
     name: istio-tenant-a
   spec:
     namespace: istio-system-tenant-a
     version: v1.23.0
     values:
       meshConfig:
         discoverySelectors:
           - matchLabels:
               tenant: tenant-a
   #     extensionProviders:  # uncomment and update according to your tracing/metrics configuration if used
   #       - name: prometheus
   #         prometheus: {}
   #       - name: otel
   #         opentelemetry:
   #           port: 4317
   #           service: otel-collector.opentelemetrycollector-3.svc.cluster.local
   ```

> [!WARNING]
> It is important your `Istio` resource's `spec.namespace` field is the **same** namespace as your `ServiceMeshControlPlane`. If you set your `Istio` resource's `spec.namespace` field to a different namespace than your `ServiceMeshControlPlane`, the migration will not work properly. In this example, we assume that your `ServiceMeshControlPlane` is found in the `istio-system-tenant-a` namespace.

2. Add your `tenant` label to each one of your dataplane namespaces.

   With 2.6, we enrolled namespaces into the mesh by adding them to the Service Mesh Member Roll resource. In 3.0, you must label each one of your dataplane namespaces with this label. For every namespace in your Service Mesh Member Roll, add your tenant label to the namespace.

   ```sh
   oc label ns bookinfo tenant=tenant-a
   ```

   Now we are ready to migrate our workloads from our 2.6 controlplane to our 3.0 controlplane.

#### Migrate Workloads

1. Find the current `IstioRevision` for your Service Mesh 3.0 controlplane.

   ```sh
   oc get istios istio-tenant-a
   NAME             REVISIONS   READY   IN USE   ACTIVE REVISION   STATUS    VERSION   AGE
   istio-tenant-a   1           1       0        istio-tenant-a    Healthy   v1.24.1   30s
   ```

   Copy the `ACTIVE REVISION`. You will use this as your `istio.io/rev` label in the next step. In this case, our revision label is `istio-tenant-a`. The naming format of your revisions will change depending on which upgrade strategy you choose for your `Istio` instance but the correct revision label will show under `ACTIVE REVISION`.

1. Update injection labels on the dataplane namespace.

   Here we're adding two labels to the namespace:

   1. The `istio.io/rev: istio-tenant-a` label which ensures that any new pods that get created in that namespace will connect to the 3.0 proxy.
   2. The `maistra.io/ignore-namespace: "true"` label which will disable sidecar injection for 2.6 proxies in the namespace. This ensures that 2.6 will stop injecting proxies in this namespace and any new proxies will be injected by the 3.0 controlplane. Without this, the 2.6 injection webhook will try to inject the pod and it will connect to the 2.6 proxy as well as refuse to start since it will have the 2.6 cni annotation.

   **Note:** that once you apply the `maistra.io/ignore-namespace` label, any new pod that gets created in the namespace will be connected to the 3.0 proxy. Workloads will still be able to communicate with each other though regardless of which controlplane they are connected to.

   ```sh
   oc label ns bookinfo istio.io/rev=istio-tenant-a maistra.io/ignore-namespace="true" --overwrite=true
   ```

1. `curl` the productpage pod in `bookinfo` to ensure proxies can still communicate with one another.

   ```sh
   oc exec -it -n bookinfo deployments/productpage-v1 -c istio-proxy -- curl localhost:9080/productpage
   ```

   You should see

   ```html
   ...
           <p>Absolutely fun and entertaining. The play lacks thematic depth when compared to other plays by Shakespeare.</p>
           <small>Reviewer2</small>


           <font color="black">
             <!-- full stars: -->

             <span class="glyphicon glyphicon-star"></span>

             <span class="glyphicon glyphicon-star"></span>

             <span class="glyphicon glyphicon-star"></span>

             <span class="glyphicon glyphicon-star"></span>

             <!-- empty stars: -->

             <span class="glyphicon glyphicon-star-empty"></span>

           </font>


         </blockquote>

         <dl>
           <dt>Reviews served by:</dt>
           <u>reviews-v2-6dd458b5db-frrlb</u>

         </dl>
   ...
   ```
   
1. Migrate Gateways

   Following the same labeling scheme used for your workload namespace, follow the gateway migration [guide](./../gateway-migration.md).

1. Migrate workloads.

   You can now restart the workloads so that the new pod will be injected with the 3.0 proxy.

   This can be done all at once:

   ```sh
   oc rollout restart deployments -n bookinfo
   ```

   or individually:

   ```sh
   oc rollout restart deployments productpage-v1 -n bookinfo
   ```

1. Wait for the productpage app to restart.

   ```sh
   oc rollout status deployment productpage-v1 -n bookinfo
   ```

#### Validate Workload Migration

1. Ensure the productpage app is connected to the new controlplane

   You can see which proxies are still connected to the 2.6 controlplane with `istioctl`. Here `basic` should be the name of your `ServiceMeshControlPlane`:

   ```sh
   istioctl ps --istioNamespace istio-system-tenant-a --revision basic
   ```

   Example response:

   ```sh
   NAME                                              CLUSTER        CDS        LDS        EDS        RDS          ECDS         ISTIOD                            VERSION
   details-v1-7b49464bc-zr7nr.bookinfo               Kubernetes     SYNCED     SYNCED     SYNCED     SYNCED       NOT SENT     istiod-basic-6c9f8d9894-sh6lx     1.20.8
   ratings-v1-d6f449f59-9rds2.bookinfo               Kubernetes     SYNCED     SYNCED     SYNCED     SYNCED       NOT SENT     istiod-basic-6c9f8d9894-sh6lx     1.20.8
   reviews-v1-686cd989df-9x59z.bookinfo              Kubernetes     SYNCED     SYNCED     SYNCED     SYNCED       NOT SENT     istiod-basic-6c9f8d9894-sh6lx     1.20.8
   reviews-v2-785b8b48fc-l7xkj.bookinfo              Kubernetes     SYNCED     SYNCED     SYNCED     SYNCED       NOT SENT     istiod-basic-6c9f8d9894-sh6lx     1.20.8
   reviews-v3-67889ffd49-7bhxn.bookinfo              Kubernetes     SYNCED     SYNCED     SYNCED     SYNCED       NOT SENT     istiod-basic-6c9f8d9894-sh6lx     1.20.8
   ```

   And which proxies have been migrated to the new 3.0 controlplane:

   ```sh
   istioctl ps --istioNamespace istio-system-tenant-a --revision istio-tenant-a
   ```

   Example response:

   ```sh
   NAME                                         CLUSTER        CDS        LDS        EDS        RDS        ECDS     ISTIOD                      VERSION
   productpage-v1-7745c5cc94-wpvth.bookinfo     Kubernetes     SYNCED     SYNCED     SYNCED     SYNCED              istiod-5bbf98dccf-n8566     1.23.0
   ```

1. Ensure the `bookinfo` application is still working correctly.

   ```sh
   oc exec -it -n bookinfo deployments/productpage-v1 -c istio-proxy -- curl localhost:9080/productpage
   ```

   Example response:

   ```html
   ...
           <p>Absolutely fun and entertaining. The play lacks thematic depth when compared to other plays by Shakespeare.</p>
           <small>Reviewer2</small>


           <font color="black">
             <!-- full stars: -->

             <span class="glyphicon glyphicon-star"></span>

             <span class="glyphicon glyphicon-star"></span>

             <span class="glyphicon glyphicon-star"></span>

             <span class="glyphicon glyphicon-star"></span>

             <!-- empty stars: -->

             <span class="glyphicon glyphicon-star-empty"></span>

           </font>


         </blockquote>

         <dl>
           <dt>Reviews served by:</dt>
           <u>reviews-v2-6dd458b5db-frrlb</u>

         </dl>
   ...
   ```
### Cleaning of OpenShift Service Mesh 2.6
Follow [these instructions.](../cleaning-2.6/README.md)