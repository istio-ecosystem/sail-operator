# OpenShift Service Mesh 2.6 Cluster wide --> 3 Migration guide
This guide is for users who are currently running `ClusterWide` OpenShift Service Mesh 2.6 migrating to OpenShift Service Mesh 3.0. You should first read [this document comparing OpenShift Service Mesh 2 vs. OpenShift Service Mesh 3](../../ossm2-vs-ossm3.md) to familiarize yourself with the concepts between the two versions and the differences in managing the workloads and addons.

## Migrating OpenShift Service Mesh 2.6 Cluster wide to OpenShift Service Mesh 3

### Prerequisites
- you have read [OpenShift Service Mesh 2 vs. OpenShift Service Mesh 3](../../ossm2-vs-ossm3.md)
- you have completed all the steps from the [pre-migration checklist](../README.md#pre-migration-checklist)
- you have verified that your 2.6 `ServiceMeshControlPlane` is using `ClusterWide` mode
- Red Hat OpenShift Service Mesh 3 operator is installed
- `IstioCNI` is installed
- `istioctl` is installed

### Procedure
In this example, we'll be using the [bookinfo demo](https://raw.githubusercontent.com/Maistra/istio/maistra-2.6/samples/bookinfo/platform/kube/bookinfo.yaml) but you can follow these same steps with your own workloads.

#### Plan the migration
There will be two cluster wide Istio control planes running during the migration process so it's necessary to plan the migration steps in advance in order to avoid possible conflicts between the two control planes.

There are a few conditions which must be verified to ensure a successful migration:
- both control planes must share the same root certificate

  This can be achieved by installing the 3.0 control plane to the same namespace as 2.6 control plane. The migration procedure below shows how to verify the root cert is shared.
- both 3.0 and 2.6 control planes must have access to all namespaces in a mesh

  During the migration, some proxies will be controlled by the 3.0 control plane while others will still be controlled by the 2.6 control plane. To assure the communication still works, both control planes must be aware of the same set of services. Service discovery is done by `istiod` component which is running in your control plane namespace. You must verify that:
  1. there are no Network Policies blocking the traffic between `istiod` and proxies and vice versa for both control planes

     OpenShift Service Mesh 2.6 is by default managing Network Policies which would be blocking traffic for 3.0 control plane. In the pre-migration checklist, we instruct users to disable this feature but users can still decide to re-create those Network Policies manually. In that case, newly created Network Policies must allow the traffic for both control planes. One example of problematic Network policy is the usage of `maistra.io/member-of` label which will be removed automatically when a dataplane namespace is migrated to 3.0. See Migration of Network Policies documentation for details.

     > **_NOTE:_** Incorrectly configured Network Policies will cause traffic disruptions and might be difficult to debug. Pay increased attention when creating Network Policies.
     <!--TODO: add a link when the doc is ready https://issues.redhat.com/browse/OSSM-8520-->
  1. Ensure that the `discoverySelectors` defined in your OpenShift Service Mesh 3.0 `Istio` resource will match the namespaces that make up your OpenShift Service Mesh 2.6 mesh. You may need to add additional labels onto your OpenShift Service Mesh 2.6 application namespaces to ensure that they are captured by your OpenShift Service Mesh 3.0 `Istio` `discoverySelectors`. See [Scoping the service mesh with DiscoverySelectors](../../create-mesh/README.md)
- only one control plane will try to inject a side car

  This can be achieved by correct use of injection labels. Please see [Installing the Sidecar](../../injection/README.md) for details.
  > **_NOTE:_** Due to specific behavior of OpenShift Service Mesh 2.6 it's necessary to disable 2.6 injector when migrating the data plane namespace. We will use `maistra.io/ignore-namespace: "true"` label in this guide.

In addition to the conditions above, for OpenShift Service Mesh 3.0, you must decide whether to want to use the `istio.io/rev` or `istio-injection` labels to configure sidecar injection. See [Installing the Sidecar](../../injection/README.md) for a full explanation of the two labels for configuring sidecar injection.

Going back to your OpenShift Service Mesh 2.6 installation, the way member selection was configured in the `ServiceMeshMemberRoll` may also impact the choice of injection labels used with OpenShift Service Mesh 3.0:

- by default the `spec.memberSelectors` in your `ServiceMeshMemberRoll` is configured to match `istio-injection=enabled` label and all of your 2.6 data plane namespaces are already labeled with `istio-injection=enabled`

    With this configuration, you can choose if you want to keep using the `istio-injection=enabled` label or switch to using `istio.io/rev` label.
- `spec.memberSelectors` in your `ServiceMeshMemberRoll` is __not__ configured to match `istio-injection=enabled` and your 2.6 data plane namespaces are using some other label

    With this configuration, it will be necessary to add either the `istio.io/rev` or `istio-injection` label during the migration. The label defined in the `spec.memberSelectors` in your `ServiceMeshMemberRoll` will have no effect on injection in OpenShift Service Mesh 3
- [adding projects using label selectors](https://docs.openshift.com/container-platform/4.16/service_mesh/v2x/ossm-create-mesh.html#ossm-about-adding-projects-using-label-selectors_ossm-create-mesh) feature is not used at all and all projects were added to the mesh manually by creating `ServiceMeshMember`

    With this configuration, it will be necessary to add either the `istio.io/rev` or `istio-injection` label during the migration.

Procedures below show different approaches for migration. Only first or second procedure should be used for production environments. Read a description of each procedure to pick one which fits your needs.

#### Migration to 3.0 using istio.io/rev label
In this procedure, we will use a proper canary upgrade with gradual migration of data plane namespaces. This should be followed by users who decided to use `istio.io/rev` label.

##### Create your Istio resource
1. Find a namespace with 2.6 control plane:

    ```sh
    oc get smcp -A
    NAMESPACE      NAME                   READY   STATUS            PROFILES      VERSION   AGE
    istio-system   install-istio-system   6/6     ComponentsReady   ["default"]   2.6.4     115m
    ```
1. Prepare the `Istio` resource yaml named `ossm-3.yaml` to be deployed to the same namespace as the 2.6 control plane:

    Here we are not using any `discoverySelectors` so the control plane will have access to all namespaces. In case you want to define `discoverySelectors`, keep in mind that all data plane namespaces you are planning to migrate from 2.6 must be matched.

    > **_NOTE:_** For simplicity, we are using a minimal example for Istio resource. Read [SMCP to Istio mapping](#TODO) to see how to transform fields used in SMCP resource to fields in Istio resource.

    ```yaml
    apiVersion: sailoperator.io/v1alpha1
    kind: Istio
    metadata:
      name: ossm-3 # the name, updateStrategy and version are significant for injection labels
    spec:
      updateStrategy:
        type: RevisionBased
      namespace: istio-system # 3.0 and 2.6 control planes must run in the same namespace
      version: v1.24.1
    # values:  # uncomment and update according to your tracing/metrics configuration if used
    #   meshConfig:
    #   extensionProviders:
    #     - name: prometheus
    #       prometheus: {}
    #     - name: otel
    #       opentelemetry:
    #         port: 4317
    #         service: otel-collector.opentelemetrycollector-3.svc.cluster.local
    ```
1. Apply the `Istio` resource yaml:

    ```sh
    oc apply -f ossm-3.yaml
    ```
1. Verify that new `istiod` is using existing root certificate:

    ```sh
    oc logs deployments/istiod-ossm-3-v1-24-1 -n istio-system | grep 'Load signing key and cert from existing secret'
    2024-12-18T08:13:53.788959Z	info	pkica	Load signing key and cert from existing secret istio-system/istio-ca-secret
    ```

##### Migrate Workloads
This procedure is not using revision tags but it's recommended to use them for big meshes to avoid re-labeling of namespaces during future 3.y.z upgrades.

1. Update injection labels on the data plane namespace

    Here we're adding two and removing one label:

    1. The `istio.io/rev=ossm-3-v1-24-1` label ensures that 3.0 proxy will be injected to any new or restarted pods in that namespace. In our example, the 3.0 revision is named `ossm-3-v1-24-1`
    1. The `maistra.io/ignore-namespace: "true"` label ensures that 2.6 control plane will stop injecting proxies in this namespace to avoid conflicts between 2.6 and 3.0 side car injectors.
    1. `istio-injection` label takes precedence over `istio.io/rev` label so it must be removed if it exists. Otherwise the `istio-injection=enabled` label would prevent proxy injection.

    ```sh
    oc label ns bookinfo istio.io/rev=ossm-3-v1-24-1 maistra.io/ignore-namespace="true" istio-injection- --overwrite=true
    ```

1. Migrate workloads

    You can now restart the workloads so that the new pods will be injected with the 3.0 proxy.

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

##### Validate Workload Migration
1.  Ensure that expected workloads are managed by the new control plane via `istioctl ps -n bookinfo`

    In case you have restarted just `productpage-v1`, you will see that only `productpage` proxy is upgraded and connected to the new control plane:
    ```sh
    $ istioctl ps -n bookinfo
    NAME                                          CLUSTER        CDS             LDS             EDS             RDS             ECDS         ISTIOD                                           VERSION
    details-v1-7f46897b-d497c.bookinfo            Kubernetes     SYNCED          SYNCED          SYNCED          SYNCED          NOT SENT     istiod-install-istio-system-866b57d668-6lpcr     1.20.8
    productpage-v1-74bfbd4d65-vsxqm.bookinfo      Kubernetes     SYNCED (4s)     SYNCED (4s)     SYNCED (3s)     SYNCED (4s)     IGNORED      istiod-ossm-3-v1-24-1-797bb4d78f-xpchx           1.24.1
    ratings-v1-559b64556-c5ppg.bookinfo           Kubernetes     SYNCED          SYNCED          SYNCED          SYNCED          NOT SENT     istiod-install-istio-system-866b57d668-6lpcr     1.20.8
    reviews-v1-847fb7c54d-qxt5d.bookinfo          Kubernetes     SYNCED          SYNCED          SYNCED          SYNCED          NOT SENT     istiod-install-istio-system-866b57d668-6lpcr     1.20.8
    reviews-v2-5c7ff5b77b-8jbhd.bookinfo          Kubernetes     SYNCED          SYNCED          SYNCED          SYNCED          NOT SENT     istiod-install-istio-system-866b57d668-6lpcr     1.20.8
    reviews-v3-5c5d764c9b-rrx8w.bookinfo          Kubernetes     SYNCED          SYNCED          SYNCED          SYNCED          NOT SENT     istiod-install-istio-system-866b57d668-6lpcr     1.20.8
    ```
  > **_NOTE:_** Even with different versions of the proxies, the communication between services should work normally.

Now you can proceed with the migration of next namespaces.

> [!CAUTION]
> Do not remove `maistra.io/ignore-namespace="true"` label until the 2.6 control plane is uninstalled.

#### Migration to 3.0 using istio-injection=enabled label
In this procedure, we will use a proper canary upgrade with gradual migration of data plane namespaces. This should be followed by users who decided to use `istio-injection=enabled` label or are already using this label on 2.6 data plane namespaces. Compared to the simple Migration of 2.6 installation with istio-injection=enabled label below, it is safe to restart any workloads at any given time during the migration. To achieve this, it requires more manual steps and re-labeling.

##### Create your Istio resource
1. Find a namespace with 2.6 control plane:

    ```sh
    oc get smcp -A
    NAMESPACE      NAME                   READY   STATUS            PROFILES      VERSION   AGE
    istio-system   install-istio-system   6/6     ComponentsReady   ["default"]   2.6.4     115m
    ```
1. Prepare the `Istio` resource yaml named `ossm-3.yaml` to be deployed to the same namespace as the 2.6 control plane:

    Here we are not using any `discoverySelectors` so the control plane will have access to all namespaces. In case you want to define `discoverySelectors`, keep in mind that all data plane namespaces you are planning to migrate from 2.6 must be matched.

    We don't want the new control plane to inject proxies to workloads in namespaces with `istio-injection=enabled` label at this point so we can't use `default` name and we can't create `default` revision tag at this point.

    > **_NOTE:_** For simplicity, we are using a minimal example for Istio resource. Read [SMCP to Istio mapping](#TODO) to see how to transform fields used in SMCP resource to fields in Istio resource.

    ```yaml
    apiVersion: sailoperator.io/v1alpha1
    kind: Istio
    metadata:
      name: ossm-3 # the name, updateStrategy and version are significant for injection labels
    spec:
      updateStrategy:
        type: RevisionBased
      namespace: istio-system # 3.0 and 2.6 control planes must run in the same namespace
      version: v1.24.1
    # values:  # uncomment and update according to your tracing/metrics configuration if used
    #   meshConfig:
    #   extensionProviders:
    #     - name: prometheus
    #       prometheus: {}
    #     - name: otel
    #       opentelemetry:
    #         port: 4317
    #         service: otel-collector.opentelemetrycollector-3.svc.cluster.local
    ```
1. Apply the `Istio` resource yaml:

    ```sh
    oc apply -f ossm-3.yaml
    ```
1. Verify that new `istiod` is using existing root certificate:

    ```sh
    oc logs deployments/istiod-ossm-3-v1-24-1 -n istio-system | grep 'Load signing key and cert from existing secret'
    2024-12-18T08:13:53.788959Z	info	pkica	Load signing key and cert from existing secret istio-system/istio-ca-secret
    ```
##### Migrate Workloads
1. Apply correct labels to the data plane namespace

    Here we're adding two and removing one label:

    1. The `istio.io/rev=ossm-3-v1-24-1` label ensures that 3.0 proxy will be injected to any new or restarted pods in that namespace. In our example, the 3.0 revision is named `ossm-3-v1-24-1`
    1. The `maistra.io/ignore-namespace: "true"` label ensures that 2.6 control plane will stop injecting proxies in this namespace to avoid conflicts between 2.6 and 3.0 side car injectors.
    1. Because we can't create the `default` `IstioRevisionTag` yet. It's necessary to temporarily remove the `istio-injection=enabled` as it would prevent the proxy injection by 3.0 control plane as the `istio-injection` label takes precedence over `istio.io/rev` label.

    ```sh
    oc label ns bookinfo istio.io/rev=ossm-3-v1-24-1 maistra.io/ignore-namespace="true" istio-injection- --overwrite=true
    ```

1. Migrate workloads

    You can now restart the workloads so that the new pods will be injected with the 3.0 proxy.

    This can be done all at once:

    ```sh
    oc rollout restart deployments -n bookinfo
    ```
    or individually:
    ```sh
    oc rollout restart deployments productpage-v1 -n bookinfo
    ```
1. Validation of the workloads can be done the same way as in the previous procedures

> [!CAUTION]
> Before proceeding, it's necessary to finish migration of all remaining namespaces.

> **_NOTE:_** Even with different versions of the proxies, the communication between services should work normally.

##### Create a default revision tag and re-label namespaces
1. Prepare a default revision tag yaml named `rev-tag.yaml`:
    ```yaml
    apiVersion: sailoperator.io/v1alpha1
    kind: IstioRevisionTag
    metadata:
      name: default
    spec:
      targetRef:
        kind: IstioRevision
        name: ossm-3-v1-24-1
    ```
1. Apply the `rev-tag.yaml`:
    ```sh
    oc apply -f rev-tag.yaml
    ```
1. Verify `IstioRevisionTag` status:
    ```sh
    oc get istiorevisiontags
    NAME      STATUS                    IN USE   REVISION        AGE
    default   NotReferencedByAnything   False    ossm-3-v1-24-1  18s
    ```
1. Add `istio-injection=enabled` and remove `istio.io/rev` label:
    ```sh
    oc label ns bookinfo istio-injection=enabled istio.io/rev-
    ```
1. Restart the workloads:
    ```sh
    oc rollout restart deployments -n bookinfo
    ```
1. Verify the `IstioRevisionTag` is in use:
    ```sh
    oc get istiorevisiontags
    NAME      STATUS    IN USE   REVISION        AGE
    default   Healthy   True     ossm-3-v1-24-1  28s
    ```
1. Validation of the workloads can be done the same way as in the previous procedures
1. Repeat steps 4. and 5. for other namespaces

> [!CAUTION]
> Do not remove `maistra.io/ignore-namespace="true"` label until the 2.6 control plane is uninstalled.

#### Simple migration of 2.6 installation with istio-injection=enabled label
It is recommended to use one of the procedures above in production environments.
In this procedure it's expected that all 2.6 data plane namespaces have `istio-injection=enabled` label.
> [!CAUTION]
> This procedure may cause a traffic disruption for workloads which are restarted at unexpected time. See steps below to understand the risk.

##### Create your Istio resource
1. Find a namespace with 2.6 control plane:

    ```sh
    oc get smcp -A
    NAMESPACE      NAME                   READY   STATUS            PROFILES      VERSION   AGE
    istio-system   install-istio-system   6/6     ComponentsReady   ["default"]   2.6.4     115m
    ```
1. Prepare the `Istio` resource yaml named `ossm-3.yaml` to be deployed to the same namespace as the 2.6 control plane:

    Here we are not using any `discoverySelectors` so the control plane will have access to all namespaces. In case you want to define `discoverySelectors`, keep in mind that all data plane namespaces you are planning to migrate from 2.6 must be matched.

    Also note that `default` name with `InPlace` update strategy is used which allows usage of the `istio-injection=enabled` label without a need for a `default` `IstioRevisionTag`. In case you want to use different name or `RevisionBased` update strategy, you will have to configure `default` `IstioRevisionTag`. See procedures above.

    > **_NOTE:_** For simplicity, we are using a minimal example for Istio resource. Read [SMCP to Istio mapping](#TODO) to see how to transform fields used in SMCP resource to fields in Istio resource.

    ```yaml
    apiVersion: sailoperator.io/v1alpha1
    kind: Istio
    metadata:
      name: default # the name and the updateStrategy is significant for injection labels
    spec:
      updateStrategy:
        type: InPlace
      namespace: istio-system # 3.0 and 2.6 control planes must run in the same namespace
      version: v1.24.1
    # values:  # uncomment and update according to your tracing/metrics configuration if used
    #   meshConfig:
    #   extensionProviders:
    #     - name: prometheus
    #       prometheus: {}
    #     - name: otel
    #       opentelemetry:
    #         port: 4317
    #         service: otel-collector.opentelemetrycollector-3.svc.cluster.local
    ```
1. Apply the `Istio` resource yaml:

    > **_NOTE:_** after next step, both 2.6 and 3.0 control planes will try to inject side cars to all pods in namespaces with the `istio-injection=enabled` label and all pods with the `sidecar.istio.io/inject="true"` label if the workloads are restarted. This will cause a traffic disruption. To avoid this problem, workloads should be restarted __only after__ the `maistra.io/ignore-namespace: "true"` label is added (see below).
    ```sh
    oc apply -f ossm-3.yaml
    ```
1. Verify that new `istiod` is using existing root certificate:

    ```sh
    oc logs deployments/istiod -n istio-system | grep 'Load signing key and cert from existing secret'
    2024-12-18T08:13:53.788959Z	info	pkica	Load signing key and cert from existing secret istio-system/istio-ca-secret
    ```
##### Migrate Workloads
1. Add `maistra.io/ignore-namespace: "true"` label to the data plane namespace

    The `maistra.io/ignore-namespace: "true"` label ensures that 2.6 control plane will stop injecting proxies in this namespace to avoid conflicts between 2.6 and 3.0 side car injectors. Without this, the proxy will not start.

      > **_NOTE:_** that once you apply the `maistra.io/ignore-namespace` label, any new or restarted pods in the namespace will be connected to the 3.0 proxy. Workloads running 2.6 proxy are still able communicate with workloads running 3.0 proxy.

    ```sh
    oc label ns bookinfo maistra.io/ignore-namespace="true"
    ```

1. Migrate workloads

    You can now restart the workloads so that the new pods will be injected with the 3.0 proxy.

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

##### Validate Workload Migration
1.  Ensure that expected workloads are managed by the new control plane via `istioctl ps -n bookinfo`

    In case you have restarted just `productpage-v1`, you will see that only `productpage` proxy is upgraded and connected to the new control plane:
    ```sh
    $ istioctl ps -n bookinfo
    NAME                                          CLUSTER        CDS             LDS             EDS             RDS             ECDS         ISTIOD                                           VERSION
    details-v1-7f46897b-d497c.bookinfo            Kubernetes     SYNCED          SYNCED          SYNCED          SYNCED          NOT SENT     istiod-install-istio-system-866b57d668-6lpcr     1.20.8
    productpage-v1-74bfbd4d65-vsxqm.bookinfo      Kubernetes     SYNCED (4s)     SYNCED (4s)     SYNCED (3s)     SYNCED (4s)     IGNORED      istiod-797bb4d78f-xpchx                          1.24.1
    ratings-v1-559b64556-c5ppg.bookinfo           Kubernetes     SYNCED          SYNCED          SYNCED          SYNCED          NOT SENT     istiod-install-istio-system-866b57d668-6lpcr     1.20.8
    reviews-v1-847fb7c54d-qxt5d.bookinfo          Kubernetes     SYNCED          SYNCED          SYNCED          SYNCED          NOT SENT     istiod-install-istio-system-866b57d668-6lpcr     1.20.8
    reviews-v2-5c7ff5b77b-8jbhd.bookinfo          Kubernetes     SYNCED          SYNCED          SYNCED          SYNCED          NOT SENT     istiod-install-istio-system-866b57d668-6lpcr     1.20.8
    reviews-v3-5c5d764c9b-rrx8w.bookinfo          Kubernetes     SYNCED          SYNCED          SYNCED          SYNCED          NOT SENT     istiod-install-istio-system-866b57d668-6lpcr     1.20.8
    ```
    In case you restarted all deployments, all proxies will be upgraded:
    ```sh
    $ istioctl ps -n bookinfo
    NAME                                           CLUSTER        CDS              LDS              EDS             RDS              ECDS        ISTIOD                             VERSION
    details-v1-7b5c68d756-9v9g4.bookinfo           Kubernetes     SYNCED (13s)     SYNCED (13s)     SYNCED (4s)     SYNCED (13s)     IGNORED     istiod-797bb4d78f-xpchx            1.24.1
    productpage-v1-db9bfdbd4-z5c2l.bookinfo        Kubernetes     SYNCED (9s)      SYNCED (9s)      SYNCED (4s)     SYNCED (9s)      IGNORED     istiod-797bb4d78f-xpchx            1.24.1
    ratings-v1-7684d8d8b8-xzrc6.bookinfo           Kubernetes     SYNCED (12s)     SYNCED (12s)     SYNCED (4s)     SYNCED (12s)     IGNORED     istiod-797bb4d78f-xpchx            1.24.1
    reviews-v1-fb4d48bd8-lzvtx.bookinfo            Kubernetes     SYNCED (12s)     SYNCED (12s)     SYNCED (4s)     SYNCED (12s)     IGNORED     istiod-797bb4d78f-xpchx            1.24.1
    reviews-v2-58bcc78ff6-fcrb8.bookinfo           Kubernetes     SYNCED (11s)     SYNCED (11s)     SYNCED (4s)     SYNCED (11s)     IGNORED     istiod-797bb4d78f-xpchx            1.24.1
    reviews-v3-5d56c9c79b-l6gms.bookinfo           Kubernetes     SYNCED (11s)     SYNCED (11s)     SYNCED (4s)     SYNCED (11s)     IGNORED     istiod-797bb4d78f-xpchx            1.24.1
    ```
You can now proceed with the migration of next namespace.

  > **_NOTE:_** Even with different versions of the proxies, the communication between services should work normally.

> [!CAUTION]
> Do not remove `maistra.io/ignore-namespace="true"` label until the 2.6 control plane is uninstalled.

### Cleaning of OpenShift Service Mesh 2.6
Follow [these instructions.](../cleaning-2.6/README.md)