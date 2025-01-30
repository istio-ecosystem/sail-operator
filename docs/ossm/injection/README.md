## Installing the Sidecar
### Injection
To include workloads as part of the service mesh and to begin using Istio's many features, pods must be injected with a sidecar proxy that will be configured by an Istio control plane.

Sidecar injection can be enabled via labels at the namespace or pod level. That also serves to identify the control plane managing the sidecar proxy(ies). By adding a valid injection label on a `Deployment`, pods created through that deployment will automatically have a sidecar added to them. By adding a valid pod injection label on a namespace, any new pods that are created in that namespace will automatically have a sidecar added to them.

The proxy configuration is injected at pod creation time using an [admission controller](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/). As sidecar injection occurs at the pod-level, you won’t see any change to `Deployment` resources. Instead, you’ll want to check individual pods (via `oc describe`) to see the injected Istio proxy container.

### Identifying the revision name

The correct label used to enable sidecar injection depends on the control plane instance being used. A control plane instance is called a "revision" and is managed by the `IstioRevision` resource. The `Istio` control plane resource creates and manages `IstioRevision` resources, thus users do not typically have to create or modify them. 

When the `Istio` resources's `spec.updateStrategy.type` is set to `InPlace`, the `IstioRevision` will have the same name as the `Istio` resource. When the `Istio` resources's `spec.updateStrategy.type` is set to `RevisionBased`, the `IstioRevision` will have the format `<Istio resource name>-v<version>`.

In most cases, there will be a single `IstioRevision` resource per `Istio` resource. During a revision based upgrade, there may be multiple `IstioRevision` instances present, each representing an independent control plane. 

The available revision names can be checked with the command:

```console
$ oc get istiorevision
NAME              READY   STATUS    IN USE   VERSION   AGE
my-mesh-v1-23-0   True    Healthy   False    v1.23.0   114s
```

### Enabling sidecar injection - "default" revision

When the service mesh's `IstioRevision` name is "default", or if there is an `IstioRevisionTag` with the name `default` that references the `IstioRevision`, it's possible to use following labels on a namespace or a pod to enable sidecar injection:
| Resource | Label | Enabled value | Disabled value |
| --- | --- | --- | --- |
| Namespace | `istio-injection` | `enabled` | `disabled` |
| Pod | `sidecar.istio.io/inject` | `"true"` | `"false"` |

### Enabling sidecar injection - other revisions

When the `IstioRevision` name is not "default", then the specific `IstioRevision` name must be used with the `istio.io/rev` label to map the pod to the desired control plane while enabling sidecar injection. 

For example, with the revision shown above, the following labels would enable sidecar injection:
| Resource | Enabled Label | Disabled Label |
| --- | --- | --- |
| Namespace | `istio.io/rev=my-mesh-v1-23-0` | `istio-injection=disabled` |
| Pod | `istio.io/rev=my-mesh-v1-23-0` | `sidecar.istio.io/inject="false"` |

### Sidecar injection logic

If the `istio-injection` label and the `istio.io/rev` label are both present on the same namespace, the `istio-injection` label (mapping to the "default" revision) will take precedence.

The injector is configured with the following logic:

1. If either label (`istio-injection` or `sidecar.istio.io/inject`) is disabled, the pod is not injected.
2. If either label (`istio-injection` or `sidecar.istio.io/inject` or `istio.io/rev`) is enabled, the pod is injected.

### Sidecar injection examples

The following examples use the [Bookinfo application](https://docs.openshift.com/service-mesh/3.0.0tp1/install/ossm-installing-openshift-service-mesh.html#deploying-book-info_ossm-about-bookinfo-application) to demonstrate different approaches for configuring side car injection. 

> Note: If you have followed the procedure to deploy the Bookinfo application, step 5 added a sidecar injection label to the `bookinfo` namespace, and these steps are not necessary to repeat. 

Prerequisites:
- You have installed the Red Hat OpenShift Service Mesh Operator, created an `Istio` resource, and the Operator has deployed Istio.
- You have created the `IstioCNI` resource, and the Operator has deployed the necessary IstioCNI pods.
- You have created the namespaces that are to be part of the mesh, and they are [discoverable by the Istio control plane](https://docs.openshift.com/service-mesh/3.0.0tp1/install/ossm-installing-openshift-service-mesh.html#ossm-scoping-service-mesh-with-discoveryselectors_ossm-creating-istiocni-resource). 
- (Optional) You have deployed the workloads to be included in the mesh. In the following examples, the [Bookinfo has been deployed](https://docs.redhat.com/en/documentation/red_hat_openshift_service_mesh/3.0.0tp1/html-single/installing/index#ossm-about-bookinfo-application_ossm-discoveryselectors-scope-service-mesh) to the `bookinfo` namespace, but sidecar injection (step 5) has not been configured.

#### Example 1: Enabling sidecar injection with namespace labels

In this example, all workloads within a namespace will be injected with a sidecar proxy. This is the best approach if most of the workloads within a namespace are to be included in the mesh. 

Procedure:

1. Verify the revision name of the Istio control plane:

    ```bash
    $ oc get istiorevision 
    NAME      TYPE    READY   STATUS    IN USE   VERSION   AGE
    default   Local   True    Healthy   False    v1.23.0   4m57s
    ```
    Since the revision name is `default`, we can used the default injection labels and do not need to reference the specific revision name. 

1. For workloads already running in the desired namespace, verify that they show "1/1" containers as "READY", indicating that the pods are currently running without sidecars:

    ```bash
    $ oc get pods -n bookinfo
    NAME                             READY   STATUS    RESTARTS   AGE
    details-v1-65cfcf56f9-gm6v7      1/1     Running   0          4m55s
    productpage-v1-d5789fdfb-8x6bk   1/1     Running   0          4m53s
    ratings-v1-7c9bd4b87f-6v7hg      1/1     Running   0          4m55s
    reviews-v1-6584ddcf65-6wqtw      1/1     Running   0          4m54s
    reviews-v2-6f85cb9b7c-w9l8s      1/1     Running   0          4m54s
    reviews-v3-6f5b775685-mg5n6      1/1     Running   0          4m54s
    ```

1. Apply the injection label to the bookinfo namespace by entering the following command at the CLI:
    ```bash
    $ oc label namespace bookinfo istio-injection=enabled
    namespace/bookinfo labeled
    ```

1. Workloads that were already running when the injection label was added will need to be redeployed for sidecar injection to occur. The following command can be used to perform a rolling update of all workloads in the `bookinfo` namespace:
    ```bash
    oc -n bookinfo rollout restart deployment
    ```

1. Verify that once rolled out, the new pods show "2/2" containers "READY", indicating that the sidecars have been successfully injected:

    ```bash
    $ oc get pods -n bookinfo
    NAME                              READY   STATUS    RESTARTS   AGE
    details-v1-7745f84ff-bpf8f        2/2     Running   0          55s
    productpage-v1-54f48db985-gd5q9   2/2     Running   0          55s
    ratings-v1-5d645c985f-xsw7p       2/2     Running   0          55s
    reviews-v1-bd5f54b8c-zns4v        2/2     Running   0          55s
    reviews-v2-5d7b9dbf97-wbpjr       2/2     Running   0          55s
    reviews-v3-5fccc48c8c-bjktn       2/2     Running   0          55s
    ```

#### Example 1a: Enabling sidecar injection with namespace labels and an `IstioRevisionTag`

If your revision name is not `default` - e.g. because you are using the `RevisionBased` update strategy - you can still use the `istio-injection=enabled` label. To do that, you just have to create an `IstioRevisionTag` with the name `default` that references your `Istio` resource.

Procedure:

1. Find the name of your `Istio` resource:

    ```bash
    $ oc get istio 
    NAME      REVISIONS   READY   IN USE   ACTIVE REVISION   STATUS    VERSION   AGE
    default   1           1       1        default-v1-23-2   Healthy   v1.23.2   4m57s
    ```
    In this case, the `Istio` resource has the name `default`, but the underlying revision is called `default-v1-23-2`.

1. Create the `IstioRevisionTag`:

    ```bash
    $ oc apply -f - <<EOF
    apiVersion: sailoperator.io/v1alpha1
    kind: IstioRevisionTag
    metadata:
      name: default
    spec:
      targetRef:
        kind: Istio
        name: default
    EOF

1. Verify that the `IstioRevisionTag` has been created successfully:

    ```bash
    $ oc get istiorevisiontags.sailoperator.io 
    NAME      STATUS    IN USE   REVISION          AGE
    default   Healthy   True     default-v1-23-2   4m23s
    ```
    As you can see, the new tag is referencing your active revision `default-v1-23-2`.

1. Follow steps of [Example 1](#example-1-enabling-sidecar-injection-with-namespace-labels).

    You are now able to use the `istio-injection=enabled` label as if your revision was called `default`.

#### Example 2: Exclude a workload from the mesh

There may be times when you want to exclude individual workloads from a namespace where all workloads are otherwise injected with sidecars. This continues the previous example to exclude the `details` service from the mesh.

 > Note: This example is for demonstration purposes only, and the bookinfo application requires all workloads to be part of the mesh for it to work.

Procedure: 

1. Open the application’s `Deployment` resource in an editor. In this case, we will exclude the `ratings-v1` service.

1. Modify the `spec.template.metadata.labels` section of your `Deployment` resource to include the appropriate pod injection or revision label to set injection to "false". In this case, `sidecar.istio.io/inject: false`:

    ```yaml
    kind: Deployment
    apiVersion: apps/v1
    metadata:
    name: ratings-v1
    namespace: bookinfo
    labels:
      app: ratings
      version: v1
    spec:
      template:
        metadata:
          labels:
            sidecar.istio.io/inject: 'false'
    ```
    > Note: Adding the label to the `Deployment`'s top level `labels` section will not impact sidecar injection.

Updating the deployment will result in a rollout, where a new `ReplicaSet` is created with updated pod(s).

1. Verify that the updated pod(s) do not contain a sidecar container, and shows "1/1" containers "Running":
    ```bash
    oc get pods -n bookinfo
    NAME                              READY   STATUS    RESTARTS   AGE
    details-v1-6bc7b69776-7f6wz       1/1     Running   0          7s
    productpage-v1-54f48db985-gd5q9   2/2     Running   0          29m
    ratings-v1-5d645c985f-xsw7p       2/2     Running   0          29m
    reviews-v1-bd5f54b8c-zns4v        2/2     Running   0          29m
    reviews-v2-5d7b9dbf97-wbpjr       2/2     Running   0          29m
    reviews-v3-5fccc48c8c-bjktn       2/2     Running   0          29m
    ```

### Example 3: Enabling sidecar injection with pod labels

Rather than including all workloads within a namespace, you can include individual workloads for sidecar injection. This approach is ideal when only a few workloads within a namespace will be part of a service mesh. 

This example also demonstrates the use of a revision label for sidecar injection. In this case, the `Istio` resource has been created with the name "my-mesh". A unique resource `Istio` name is needed when there are multiple Istio control planes present in the same cluster, or a revision based control plane upgrade is in progress.

Procedure:

1. Verify the revision name of the Istio control plane:

    ```console
    $ oc get istiorevision
    NAME      TYPE    READY   STATUS    IN USE   VERSION   AGE
    my-mesh   Local   True    Healthy   False    v1.23.0   47s
    ```
    Since the revision name is `my-mesh`, we must use the a revision label to enable sidecar injection. In this case, `istio.io/rev=my-mesh`.

1. For workloads already running, verify that they show "1/1" containers as "READY", indicating that the pods are currently running without sidecars:

    ```bash
    $ oc get pods -n bookinfo
    NAME                             READY   STATUS    RESTARTS   AGE
    details-v1-65cfcf56f9-gm6v7      1/1     Running   0          4m55s
    productpage-v1-d5789fdfb-8x6bk   1/1     Running   0          4m53s
    ratings-v1-7c9bd4b87f-6v7hg      1/1     Running   0          4m55s
    reviews-v1-6584ddcf65-6wqtw      1/1     Running   0          4m54s
    reviews-v2-6f85cb9b7c-w9l8s      1/1     Running   0          4m54s
    reviews-v3-6f5b775685-mg5n6      1/1     Running   0          4m54s
    ```

1. Open the application’s `Deployment` resource in an editor. In this case, we will update the `ratings-v1` service.

1. Update the `spec.template.metadata.labels` section of your `Deployment` to include the appropriate pod injection or revision label. In this case, `istio.io/rev: my-mesh`:

    ```yaml
    kind: Deployment
    apiVersion: apps/v1
    metadata:
    name: ratings-v1
    namespace: bookinfo
    labels:
      app: ratings
      version: v1
    spec:
      template:
        metadata:
          labels:
            istio.io/rev: my-mesh
    ```

    > Note: Adding the label to the `Deployment`'s top level `labels` section will not impact sidecar injection.

    Updating the deployment will result in a rollout, where a new `ReplicaSet` is created with updated pod(s).

1. Verify that only the `ratings-v1` pod now shows "2/2" containers "READY", indicating that the sidecar has been successfully injected:
    ```
    oc get pods -n bookinfo
    NAME                              READY   STATUS    RESTARTS   AGE
    details-v1-559cd49f6c-b89hw       1/1     Running   0          42m
    productpage-v1-5f48cdcb85-8ppz5   1/1     Running   0          42m
    ratings-v1-848bf79888-krdch       2/2     Running   0          9s
    reviews-v1-6b7444ffbd-7m5wp       1/1     Running   0          42m
    reviews-v2-67876d7b7-9nmw5        1/1     Running   0          42m
    reviews-v3-84b55b667c-x5t8s       1/1     Running   0          42m
    ```

1. Repeat for other workloads that you wish to include in the mesh.


Additional Resources
- [Istio Sidecar injection problems](https://istio.io/latest/docs/ops/common-problems/injection/)