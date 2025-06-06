[Return to Project Root](../README.md)

# Table of Contents

- [Update Strategy](#update-strategy)
  - [InPlace](#inplace)
    - [Example using the InPlace strategy](#example-using-the-inplace-strategy)
    - [Recommendations for InPlace Strategy](#recommendations-for-inplace-strategy)
  - [RevisionBased](#revisionbased)
    - [Example using the RevisionBased strategy](#example-using-the-revisionbased-strategy)
    - [Example using the RevisionBased strategy and an IstioRevisionTag](#example-using-the-revisionbased-strategy-and-an-istiorevisiontag)

# Update Strategy

The Sail Operator supports two update strategies to update the version of the Istio control plane: `InPlace` and `RevisionBased`. The default strategy is `InPlace`.

## InPlace
When the `InPlace` strategy is used, the existing Istio control plane is replaced with a new version. The workload sidecars immediately connect to the new control plane. The workloads therefore don't need to be moved from one control plane instance to another.

### Example using the InPlace strategy

Prerequisites:
* Sail Operator is installed.
* `istioctl` is [installed](../common/install-istioctl-tool.md).

Steps:
1. Create the `istio-system` namespace.

    ```bash { name=create-istio-ns tag=inplace-update}
    kubectl create namespace istio-system
    ```

2. Create the `Istio` resource.

    ```bash { name=create-istio-resource tag=inplace-update}
    cat <<EOF | kubectl apply -f-
    apiVersion: sailoperator.io/v1
    kind: Istio
    metadata:
      name: default
    spec:
      namespace: istio-system
      updateStrategy:
        type: InPlace
      version: v1.25.3
    EOF
    ```
<!-- ```bash { name=validation-wait-istio-pods tag=inplace-update}
    . scripts/prebuilt-func.sh
    wait_istio_ready "istio-system"
    print_istio_info
``` -->
3. Confirm the installation and version of the control plane.

    ```console
    $ kubectl get istio -n istio-system
    NAME      REVISIONS   READY   IN USE   ACTIVE REVISION   STATUS    VERSION   AGE
    default   1           1       0        default           Healthy   v1.25.3   23s
    ```
    Note: `IN USE` field shows as 0, as `Istio` has just been installed and there are no workloads using it.

4. Create namespace `bookinfo` and deploy bookinfo application.

    ```bash { name=deploy-bookinfo tag=inplace-update}
    kubectl create namespace bookinfo
    kubectl label namespace bookinfo istio-injection=enabled
    kubectl apply -n bookinfo -f https://raw.githubusercontent.com/istio/istio/release-1.22/samples/bookinfo/platform/kube/bookinfo.yaml
    ```
    Note: if the `Istio` resource name is other than `default`, you need to set the `istio.io/rev` label to the name of the `Istio` resource instead of adding the `istio-injection=enabled` label.
<!-- ```bash { name=validation-wait-bookinfo tag=inplace-update}
    . scripts/prebuilt-func.sh
    with_retries wait_pods_ready_by_ns "bookinfo"
    kubectl get pods -n bookinfo
    istioctl proxy-status
    with_retries pods_istio_version_match "bookinfo" "1.25.3"
``` -->
5. Review the `Istio` resource after application deployment.

   ```console
   $ kubectl get istio -n istio-system
   NAME      REVISIONS   READY   IN USE   ACTIVE REVISION   STATUS    VERSION   AGE
   default   1           1       1        default           Healthy   v1.25.3   115s
   ```
   Note: `IN USE` field shows as 1, as the namespace label and the injected proxies reference the IstioRevision.

6. Perform the update of the control plane by changing the version in the Istio resource.

    ```bash
    kubectl patch istio default -n istio-system --type='merge' -p '{"spec":{"version":"v1.26.0"}}'
    ```
<!-- ```bash { name=validation-wait-istio-updated tag=inplace-update}
    . scripts/prebuilt-func.sh
    old_pod=$(kubectl get pods -n istio-system -l app=istiod -o name)
    kubectl patch istio default -n istio-system --type='merge' -p '{"spec":{"version":"v1.26.0"}}'
    kubectl wait --for=delete $old_pod -n istio-system --timeout=60s
    wait_istio_ready "istio-system"
    print_istio_info
``` -->

7. Confirm the `Istio` resource version was updated.

    ```console
    $ kubectl get istio -n istio-system
    NAME      REVISIONS   READY   IN USE   ACTIVE REVISION   STATUS    VERSION   AGE
    default   1           1       1        default           Healthy   v1.26.0   4m50s
    ```

8. Delete `bookinfo` pods to trigger sidecar injection with the new version.

    ```bash
    kubectl rollout restart deployment -n bookinfo
    ```
<!-- ```bash { name=validation-wait-bookinfo tag=inplace-update}
    pod_names=$(kubectl get pods -n bookinfo -o name)
    kubectl rollout restart deployment -n bookinfo
    # Wait pod deletion
    for pod in $pod_names; do
        kubectl wait --for=delete $pod -n bookinfo --timeout=60s
    done
    . scripts/prebuilt-func.sh
    with_retries wait_pods_ready_by_ns "bookinfo"
    istioctl proxy-status
``` -->

9. Confirm that the new version is used in the sidecar.

    ```bash { name=print-istio-version tag=inplace-update}
    istioctl proxy-status 
    ```
    The column `VERSION` should match the new control plane version.
<!-- ```bash { name=validation-istio-expected-version tag=inplace-update}
    . scripts/prebuilt-func.sh
    with_retries pods_istio_version_match "bookinfo" "1.26.0"
``` -->

### Recommendations for InPlace Strategy
During `InPlace` updates, the control plane pods are restarted, which may cause temporary service disruptions. To minimize downtime during updates, we recommend configuring the `istiod` deployment with high availability (HA). For more information, please refer to this [guide](../general/istiod-ha.md).

## RevisionBased
When the `RevisionBased` strategy is used, a new Istio control plane instance is created for every change to the `Istio.spec.version` field. The old control plane remains in place until all workloads have been moved to the new control plane instance. This needs to be done by the user by updating the namespace label and restarting all the pods. The old control plane will be deleted after the grace period specified in the `Istio` resource field `spec.updateStrategy.inactiveRevisionDeletionGracePeriodSeconds`.

### Example using the RevisionBased strategy

Prerequisites:
* Sail Operator is installed.
* `istioctl` is [installed](../common/install-istioctl-tool.md).

Steps:

1. Create the `istio-system` namespace.

    ```bash { name=create-ns tag=revision-based-update}
    kubectl create namespace istio-system
    ```

2. Create the `Istio` resource.

    ```bash { name=create-istio tag=revision-based-update}
    cat <<EOF | kubectl apply -f-
    apiVersion: sailoperator.io/v1
    kind: Istio
    metadata:
      name: default
    spec:
      namespace: istio-system
      updateStrategy:
        type: RevisionBased
        inactiveRevisionDeletionGracePeriodSeconds: 30
      version: v1.25.3
    EOF
    ```
<!-- ```bash { name=validation-wait-istio-pods tag=revision-based-update}
    . scripts/prebuilt-func.sh
    wait_istio_ready "istio-system"
    print_istio_info
``` -->

3. Confirm the control plane is installed and is using the desired version.

    ```console
    $ kubectl get istio -n istio-system
    NAME      REVISIONS   READY   IN USE   ACTIVE REVISION   STATUS    VERSION   AGE
    default   1           1       0        default-v1-25-3   Healthy   v1.25.3   52s
    ```
    Note: `IN USE` field shows as 0, as the control plane has just been installed and there are no workloads using it.

4. Get the `IstioRevision` name.

    ```console
    $ kubectl get istiorevision -n istio-system
    NAME              TYPE    READY   STATUS    IN USE   VERSION   AGE
    default-v1-25-3   Local   True    Healthy   False    v1.25.3   3m4s
    ```
    Note: `IstioRevision` name is in the format `<Istio resource name>-<version>`.
<!-- ```bash { name=validation-print-revision tag=revision-based-update}
    kubectl get istiorevision -n istio-system
``` -->

5. Create `bookinfo` namespace and label it with the revision name.

    ```bash  { name=create-bookinfo-ns tag=revision-based-update}
    kubectl create namespace bookinfo
    kubectl label namespace bookinfo istio.io/rev=default-v1-25-3
    ```

6. Deploy bookinfo application.

    ```bash { name=deploy-bookinfo tag=revision-based-update}
    kubectl apply -n bookinfo -f https://raw.githubusercontent.com/istio/istio/release-1.22/samples/bookinfo/platform/kube/bookinfo.yaml
    ```
<!-- ```bash { name=validation-wait-bookinfo tag=revision-based-update}
    . scripts/prebuilt-func.sh
    with_retries wait_pods_ready_by_ns "bookinfo"
    kubectl get pods -n bookinfo
    istioctl proxy-status
    with_retries pods_istio_version_match "bookinfo" "1.25.3"
``` -->
7. Review the `Istio` resource after application deployment.

    ```console
    $ kubectl get istio -n istio-system
    NAME      REVISIONS   READY   IN USE   ACTIVE REVISION   STATUS    VERSION   AGE
    default   1           1       1        default-v1-25-3   Healthy   v1.25.3   5m13s
    ```
    Note: `IN USE` field shows as 1, after application being deployed.
<!-- ```bash { name=validation-istio-in-use tag=revision-based-update}
    . scripts/prebuilt-func.sh
    with_retries istio_active_revision_match "default-v1-25-3"
``` -->
8. Confirm that the proxy version matches the control plane version.

    ```bash
    istioctl proxy-status 
    ```
    The column `VERSION` should match the control plane version.

9. Update the control plane to a new version.

    ```bash
    kubectl patch istio default -n istio-system --type='merge' -p '{"spec":{"version":"v1.26.0"}}'
    ```
<!-- ```bash { name=validation-wait-istio-updated tag=revision-based-update}
    kubectl patch istio default -n istio-system --type='merge' -p '{"spec":{"version":"v1.26.0"}}'
    . scripts/prebuilt-func.sh
    with_retries istiod_pods_count "2"
    wait_istio_ready "istio-system"
    print_istio_info
``` -->
10. Verify the `Istio` and `IstioRevision` resources. There will be a new revision created with the new version.

    ```console
    $ kubectl get istio
    NAME      REVISIONS   READY   IN USE   ACTIVE REVISION   STATUS    VERSION   AGE
    default   2           2       1        default-v1-26-0   Healthy   v1.26.0   9m23s

    $ kubectl get istiorevision
    NAME              TYPE    READY   STATUS    IN USE   VERSION   AGE
    default-v1-25-3   Local   True    Healthy   True     v1.25.3   10m
    default-v1-26-0   Local   True    Healthy   False    v1.26.0   66s
    ```
<!-- ```bash { name=validation-istiorevision tag=revision-based-update}
    . scripts/prebuilt-func.sh
    kubectl get istio
    kubectl get istiorevision -n istio-system
    with_retries istio_active_revision_match "default-v1-26-0"
    with_retries istio_revisions_ready_count "2"
``` -->
11. Confirm there are two control plane pods running, one for each revision.

    ```console
    $ kubectl get pods -n istio-system
    NAME                                      READY   STATUS    RESTARTS   AGE
    istiod-default-v1-25-3-c98fd9675-r7bfw    1/1     Running   0          10m
    istiod-default-v1-26-0-7495cdc7bf-v8t4g   1/1     Running   0          113s
    ```
<!-- ```bash { name=validation-istiod-count tag=revision-based-update}
    . scripts/prebuilt-func.sh
    with_retries istiod_pods_count "2"
``` -->
12. Confirm the proxy sidecar version remains the same:

    ```bash
    istioctl proxy-status 
    ```
    The column `VERSION` should still match the old control plane version.
<!-- ```bash { name=validation-wait-bookinfo tag=revision-based-update}
    . scripts/prebuilt-func.sh
    istioctl proxy-status
    with_retries pods_istio_version_match "bookinfo" "1.25.3"
``` -->
13. Change the label of the `bookinfo` namespace to use the new revision.

    ```bash { name=update-bookinfo-ns-revision tag=revision-based-update}
    kubectl label namespace bookinfo istio.io/rev=default-v1-26-0 --overwrite
    ```
    The existing workload sidecars will continue to run and will remain connected to the old control plane instance. They will not be replaced with a new version until the pods are deleted and recreated.

14. Restart all Deplyments in the `bookinfo` namespace.

    ```bash
    kubectl rollout restart deployment -n bookinfo
    ```
<!-- ```bash { name=validation-wait-bookinfo tag=revision-based-update}
    pod_names=$(kubectl get pods -n bookinfo -o name)
    kubectl rollout restart deployment -n bookinfo
    # Wait pod deletion
    for pod in $pod_names; do
        kubectl wait --for=delete $pod -n bookinfo --timeout=60s
    done
    . scripts/prebuilt-func.sh
    with_retries wait_pods_ready_by_ns "bookinfo"
    kubectl get pods -n bookinfo
    istioctl proxy-status
    with_retries pods_istio_version_match "bookinfo" "1.26.0"
``` -->
15. Confirm the new version is used in the sidecars.

    ```bash
    istioctl proxy-status 
    ```
    The column `VERSION` should match the updated control plane version.

16. Confirm the deletion of the old control plane and IstioRevision.

    ```console
    $ kubectl get pods -n istio-system
    NAME                                      READY   STATUS    RESTARTS   AGE
    istiod-default-v1-26-0-7495cdc7bf-v8t4g   1/1     Running   0          4m40s

    $ kubectl get istio
    NAME      REVISIONS   READY   IN USE   ACTIVE REVISION   STATUS    VERSION   AGE
    default   1           1       1        default-v1-26-0   Healthy   v1.26.0   5m

    $ kubectl get istiorevision
    NAME              TYPE    READY   STATUS    IN USE   VERSION   AGE
    default-v1-26-0   Local   True    Healthy   True     v1.26.0   5m31s
    ```
    The old `IstioRevision` resource and the old control plane will be deleted when the grace period specified in the `Istio` resource field `spec.updateStrategy.inactiveRevisionDeletionGracePeriodSeconds` expires.
<!-- ```bash { name=validation-resources-deletion tag=revision-based-update}
    . scripts/prebuilt-func.sh
    echo "Confirm istiod pod is deleted"
    with_retries istiod_pods_count "1"
    echo "Confirm istiorevision is deleted"
    with_retries istio_revisions_ready_count "1"
    print_istio_info
``` -->
### Example using the RevisionBased strategy and an IstioRevisionTag

Prerequisites:
* Sail Operator is installed.
* `istioctl` is [installed](../common/install-istioctl-tool.md).

Steps:

1. Create the `istio-system` namespace.

    ```bash { name=create-ns tag=istiorevisiontag}
    kubectl create namespace istio-system
    ```

2. Create the `Istio` and `IstioRevisionTag` resources.

    ```bash { name=create-istio-and-revision-tag tag=istiorevisiontag}
    cat <<EOF | kubectl apply -f-
    apiVersion: sailoperator.io/v1
    kind: Istio
    metadata:
      name: default
    spec:
      namespace: istio-system
      updateStrategy:
        type: RevisionBased
        inactiveRevisionDeletionGracePeriodSeconds: 30
      version: v1.25.3
    ---
    apiVersion: sailoperator.io/v1
    kind: IstioRevisionTag
    metadata:
      name: default
    spec:
      targetRef:
        kind: Istio
        name: default
    EOF
    ```
<!-- ```bash { name=validation-wait-istio-pods tag=istiorevisiontag}
    . scripts/prebuilt-func.sh
    wait_istio_ready "istio-system"
    kubectl get pods -n istio-system
``` -->
3. Confirm the control plane is installed and is using the desired version.

    ```console
    $ kubectl get istio
    NAME      REVISIONS   READY   IN USE   ACTIVE REVISION   STATUS    VERSION   AGE
    default   1           1       1        default-v1-25-3   Healthy   v1.25.3   52s
    ```
    Note: `IN USE` field shows as 1, even though no workloads are using the control plane. This is because the `IstioRevisionTag` is referencing it.
<!-- ```bash { name=validation-istio-in-use tag=istiorevisiontag}
    . scripts/prebuilt-func.sh
    with_retries istio_active_revision_match "default-v1-25-3"
``` -->
4. Inspect the `IstioRevisionTag`.

    ```console
    $ kubectl get istiorevisiontags
    NAME      STATUS                    IN USE   REVISION          AGE
    default   NotReferencedByAnything   False    default-v1-25-3   52s
    ```
<!-- ```bash { name=validation-istio-revision-tag tag=istiorevisiontag}
    . scripts/prebuilt-func.sh
    with_retries istio_revision_tag_status_equal "NotReferencedByAnything" "default"
``` -->
    Note: `IN USE` field shows as `False`, as the tag is not referenced by any workloads or namespaces.
5. Create `bookinfo` namespace and label it to mark it for injection.

    ```bash { name=create-bookinfo-ns tag=istiorevisiontag}
    kubectl create namespace bookinfo
    kubectl label namespace bookinfo istio-injection=enabled
    ```

6. Deploy bookinfo application.

    ```bash { name=deploy-bookinfo tag=istiorevisiontag}
    kubectl apply -n bookinfo -f https://raw.githubusercontent.com/istio/istio/release-1.23/samples/bookinfo/platform/kube/bookinfo.yaml
    ```
<!-- ```bash { name=validation-wait-bookinfo tag=istiorevisiontag}
    . scripts/prebuilt-func.sh
    with_retries wait_pods_ready_by_ns "bookinfo"
    kubectl get pods -n bookinfo
    istioctl proxy-status
    with_retries pods_istio_version_match "bookinfo" "1.25.3"
``` -->
7. Review the `IstioRevisionTag` resource after application deployment.

    ```console
    $ kubectl get istiorevisiontag
    NAME      STATUS    IN USE   REVISION          AGE
    default   Healthy   True     default-v1-25-3   2m46s
    ```
    Note: `IN USE` field shows 'True', as the tag is now referenced by both active workloads and the bookinfo namespace.
<!-- ```bash { name=validation-istio-revision-tag-inuse tag=istiorevisiontag}
    . scripts/prebuilt-func.sh
    istioctl proxy-status
    with_retries istio_revision_tag_inuse "true" "default"
``` -->
8. Confirm that the proxy version matches the control plane version.

    ```bash
    istioctl proxy-status 
    ```
    The column `VERSION` should match the control plane version.

9. Update the control plane to a new version.

    ```bash { name=update-istio-version tag=istiorevisiontag}
    kubectl patch istio default -n istio-system --type='merge' -p '{"spec":{"version":"v1.26.0"}}'
    ```

10. Verify the `Istio`, `IstioRevision` and `IstioRevisionTag` resources. There will be a new revision created with the new version.

    ```console
    $ kubectl get istio
    NAME      REVISIONS   READY   IN USE   ACTIVE REVISION   STATUS    VERSION   AGE
    default   2           2       1        default-v1-26-0   Healthy   v1.26.0   9m23s

    $ kubectl get istiorevision
    NAME              TYPE    READY   STATUS    IN USE   VERSION   AGE
    default-v1-25-3   Local   True    Healthy   True     v1.25.3   10m
    default-v1-26-0   Local   True    Healthy   True    v1.26.0   66s

    $ kubectl get istiorevisiontag
    NAME      STATUS    IN USE   REVISION          AGE
    default   Healthy   True     default-v1-26-0   10m44s
    ```
    Now, both our IstioRevisions and the IstioRevisionTag are considered in use. The old revision default-v1-25-3 because it is being used by proxies, the new revision default-v1-26-0 because it is referenced by the tag, and lastly the tag because it is referenced by the bookinfo namespace.

11. Confirm there are two control plane pods running, one for each revision.

    ```console
    $ kubectl get pods -n istio-system
    NAME                                      READY   STATUS    RESTARTS   AGE
    istiod-default-v1-25-3-c98fd9675-r7bfw    1/1     Running   0          10m
    istiod-default-v1-26-0-7495cdc7bf-v8t4g   1/1     Running   0          113s
    ```
<!-- ```bash { name=validation-istiod-running tag=istiorevisiontag}
    . scripts/prebuilt-func.sh
    with_retries istiod_pods_count "2"
    wait_istio_ready "istio-system"
``` -->
12. Confirm the proxy sidecar version remains the same:

    ```bash { name=validation-istio-proxy-version tag=istiorevisiontag}
    istioctl proxy-status 
    ```
    The column `VERSION` should still match the old control plane version.
<!-- ```bash { name=validation-istio-version tag=istiorevisiontag}
    . scripts/prebuilt-func.sh
    with_retries pods_istio_version_match "bookinfo" "1.25.3"
    print_istio_info
``` -->
13. Restart all the Deployments in the `bookinfo` namespace.

    ```bash
    kubectl rollout restart deployment -n bookinfo
    ```

14. Confirm the new version is used in the sidecars. Note that it might take a few seconds for the restarts to complete.

    ```bash
    istioctl proxy-status 
    ```
    The column `VERSION` should match the updated control plane version.
<!-- ```bash { name=validation-istio-version-bookinfo tag=istiorevisiontag}
    pod_names=$(kubectl get pods -n bookinfo -o name)
    kubectl rollout restart deployment -n bookinfo
    # Wait pod deletion
    for pod in $pod_names; do
        kubectl wait --for=delete $pod -n bookinfo --timeout=60s
    done
    . scripts/prebuilt-func.sh
    with_retries wait_pods_ready_by_ns "bookinfo"
    kubectl get pods -n bookinfo
    istioctl proxy-status
    with_retries pods_istio_version_match "bookinfo" "1.26.0"
``` -->
16. Confirm the deletion of the old control plane and IstioRevision.

    ```console
    $ kubectl get pods -n istio-system
    NAME                                      READY   STATUS    RESTARTS   AGE
    istiod-default-v1-26-0-7495cdc7bf-v8t4g   1/1     Running   0          4m40s

    $ kubectl get istio -n istio-system
    NAME      REVISIONS   READY   IN USE   ACTIVE REVISION   STATUS    VERSION   AGE
    default   1           1       1        default-v1-26-0   Healthy   v1.26.0   5m

    $ kubectl get istiorevision -n istio-system
    NAME              TYPE    READY   STATUS    IN USE   VERSION   AGE
    default-v1-26-0   Local   True    Healthy   True     v1.26.0   5m31s
    ```
    The old `IstioRevision` resource and the old control plane will be deleted when the grace period specified in the `Istio` resource field `spec.updateStrategy.inactiveRevisionDeletionGracePeriodSeconds` expires.
<!-- ```bash { name=validation-resources-deletion tag=istiorevisiontag}
    . scripts/prebuilt-func.sh
    echo "Confirm istiod pod is deleted"
    with_retries istiod_pods_count "1"
    echo "Confirm istiorevision is deleted"
    with_retries istio_revisions_ready_count "1"
    print_istio_info
``` -->
