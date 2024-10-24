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

When the service mesh's `IstioRevision` name is "default", it's possible to use following labels on a namespace or a pod to enable sidecar injection:
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

### Example: Enabling sidecar injection
Prerequisites:
- The OpenShift Service Mesh operator has been installed
- An Istio CNI resource has been created

1. Create the `istio-system` namespace:
    ```bash
    oc create ns istio-system
    ```
1. Prepare `default` `istio.yaml`:
    ```yaml
    kind: Istio
    apiVersion: sailoperator.io/v1alpha1
    metadata:
      name: default
    spec:
      namespace: istio-system
      updateStrategy:
        type: InPlace
      version: v1.23.0
    ```
1. Create the `default` Istio CR in `istio-system` namespace:
    ```bash
    oc apply -f istio.yaml
    ```
1. Wait for `Istio` to become ready.
    ```bash
    oc wait --for=condition=Ready istios/default -n istio-system
    ```
1. Deploy the `sleep` app:
    ```bash
    oc apply -f https://raw.githubusercontent.com/istio/istio/release-1.23/samples/sleep/sleep.yaml
    ```
1. Verify both the deployment and pod have a single container:
    ```bash
    oc get deployment -o wide
    NAME    READY   UP-TO-DATE   AVAILABLE   AGE   CONTAINERS   IMAGES            SELECTOR
    sleep   1/1     1            1           16s   sleep        curlimages/curl   app=sleep
    oc get pod -l app=sleep
    NAME                     READY   STATUS    RESTARTS   AGE
    sleep-5577c64d7c-ntn9d   1/1     Running   0          16s
    ```
1. Label the `default` namespace with `istio-injection=enabled`:
    ```bash
    oc label namespace default istio-injection=enabled
    ```
1. Injection occurs at pod creation time. Remove the running pod to be injected with a proxy sidecar. 
    ```bash
    oc delete pod -l app=sleep
    ```
1. Verify a new pod is created with the injected sidecar. The original pod has `1/1 READY` containers, and the pod with injected sidecar has `2/2 READY` containers.
    ```bash
    oc get pod -l app=sleep
    NAME                     READY   STATUS    RESTARTS   AGE
    sleep-5577c64d7c-w9vpk   2/2     Running   0          12s
    ```
1. View the detailed state of the injected pod. You should see the injected `istio-proxy` container.
    ```bash
    oc describe pod -l app=sleep
    ...
    Events:
      Type    Reason          Age   From               Message
      ----    ------          ----  ----               -------
      Normal  Scheduled       50s   default-scheduler  Successfully assigned default/sleep-5577c64d7c-w9vpk to user-rhos-d-1-v8rnx-worker-0-rwjrr
      Normal  AddedInterface  50s   multus             Add eth0 [10.128.2.179/23] from ovn-kubernetes
      Normal  Pulled          50s   kubelet            Container image "registry.redhat.io/openshift-service-mesh-tech-preview/istio-proxyv2-rhel9@sha256:c0170ef9a34869828a5f2fea285a7cda543d99e268f7771e6433c54d6b2cbaf4" already present on machine
      Normal  Created         50s   kubelet            Created container istio-validation
      Normal  Started         50s   kubelet            Started container istio-validation
      Normal  Pulled          50s   kubelet            Container image "curlimages/curl" already present on machine
      Normal  Created         50s   kubelet            Created container sleep
      Normal  Started         50s   kubelet            Started container sleep
      Normal  Pulled          50s   kubelet            Container image "registry.redhat.io/openshift-service-mesh-tech-preview/istio-proxyv2-rhel9@sha256:c0170ef9a34869828a5f2fea285a7cda543d99e268f7771e6433c54d6b2cbaf4" already present on machine
      Normal  Created         50s   kubelet            Created container istio-proxy
      Normal  Started         50s   kubelet            Started container istio-proxy
    ...
    ```
> [!CAUTION]
> Injection using the `istioctl kube-inject` which is not supported by Red Hat OpenShift Service Mesh.
