[Return to Project Root](../../README.md)

# Consolidating control planes
It's possible to create multiple control planes in a single [cluster](multiple-mesh.md#multiple-meshes-on-a-single-cluster) using the Sail operator. If you no longer need multiple control planes, it can be beneficial to consolidate them into a single one. This documentation describes how to achieve this without any traffic disruption.

## Prerequisites
By default, each control plane uses a different root certificate, which prevents moving workloads from one mesh to another without breaking mTLS. In this procedure, we expect that each control plane is using an intermediate certificate issued by the same root CA. To learn how to migrate from the default certificate management without any traffic disruption, see [Plug in CA Certificates](../general/plugin-ca.md).

Using the same root certificate for both control planes is not only a requirement for working service to service communication with mTLS but it also avoids endless overwriting of the `istio-ca-root-cert` config map which is created by the istiod in every namespace that matches it's discovery selectors. The following procedure also contains a step verifying this. You should not continue with the procedure unless both control planes are using the same root certificate.

## Set up
We have two Istio control planes named `tenant-a` and `tenant-b` with following configuration:
```yaml
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
...
  name: tenant-a
...
spec:
  namespace: istio-system-a
  updateStrategy:
    inactiveRevisionDeletionGracePeriodSeconds: 30
    type: RevisionBased
  values:
    meshConfig:
      discoverySelectors:
      - matchLabels:
          tenant: tenant-a
  version: v1.24.5
status:
  activeRevisionName: tenant-a-v1-24-5
...
```
```yaml
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
...
  name: tenant-b
...
spec:
  namespace: istio-system-b
  updateStrategy:
    inactiveRevisionDeletionGracePeriodSeconds: 30
    type: RevisionBased
  values:
    meshConfig:
      discoverySelectors:
      - matchLabels:
          tenant: tenant-b
  version: v1.24.5
status:
  activeRevisionName: tenant-b-v1-24-5
...
```

`sleep-a` and `httpbin-a` make up a `tenant-a` mesh and `sleep-b` and `httpbin-b` make up a `tenant-b` mesh. Namespaces are labeled with `istio.io/rev=tenant-a-v1-24-5` or `istio.io/rev=tenant-b-v1-24-5` for injection and `tenant=tenant-a` or `tenant=tenant-b` for service discovery. Strict mTLS is enabled for both meshes.

### Procedure
1. Make sure both control planes are using the same root certificate:
    ```bash
    kubectl get cm istio-ca-root-cert -n httpbin-a -o jsonpath={.data.'root-cert\.pem'}
    kubectl get cm istio-ca-root-cert -n httpbin-b -o jsonpath={.data.'root-cert\.pem'}
    ```
1. Add a new discovery label which will make up consolidated mesh:
    ```bash
    kubectl label namespace sleep-a istio-discovery=enabled
    kubectl label namespace sleep-b istio-discovery=enabled
    kubectl label namespace httpbin-a istio-discovery=enabled
    kubectl label namespace httpbin-b istio-discovery=enabled
    ```
1. Update both Istio resources to use the new discovery label:
    ```yaml
    apiVersion: sailoperator.io/v1
    kind: Istio
    metadata:
    ...
      name: tenant-a
    ...
    spec:
      namespace: istio-system-a
      values:
        meshConfig:
          discoverySelectors:
          - matchLabels:
              istio-discovery: enabled
    ...
    ```
    ```yaml
    apiVersion: sailoperator.io/v1
    kind: Istio
    metadata:
    ...
      name: tenant-b
    ...
    spec:
      namespace: istio-system-b
      values:
        meshConfig:
          discoverySelectors:
          - matchLabels:
              istio-discovery: enabled
    ...
    ```
    > **_NOTE:_** Using the same discovery selectors assures that both control planes will have the same internal service registry (will see the same services).
1. Verify that both control planes can discover services from other mesh:
    ```bash
    istioctl pc endpoint deploy/sleep -n sleep-a | grep httpbin
    10.128.2.169:8080                                       HEALTHY     OK                outbound|8000||httpbin.httpbin-a.svc.cluster.local
    10.131.0.59:8080                                        HEALTHY     OK                outbound|8000||httpbin.httpbin-b.svc.cluster.local
    ```
    ```bash
    istioctl pc endpoint deploy/sleep -n sleep-b | grep httpbin
    10.128.2.169:8080                                       HEALTHY     OK                outbound|8000||httpbin.httpbin-a.svc.cluster.local
    10.131.0.59:8080                                        HEALTHY     OK                outbound|8000||httpbin.httpbin-b.svc.cluster.local
    ```
    > **_NOTE:_** We need to assure that both control planes have the same internal service registry (both see the same services) so we can migrate namespaces one by one without breaking the communication between services from different namespaces.
1. Verify that traffic between meshes works:
    ```bash
    kubectl exec "$(kubectl get pod -l app=sleep -n sleep-a -o jsonpath={.items..metadata.name})" -c sleep -n sleep-a -- curl http://httpbin.httpbin-b:8000/ip -s -o /dev/null -w "%{http_code}\n"
    200
    ```
    ```bash
    kubectl exec "$(kubectl get pod -l app=sleep -n sleep-b -o jsonpath={.items..metadata.name})" -c sleep -n sleep-b -- curl http://httpbin.httpbin-a:8000/ip -s -o /dev/null -w "%{http_code}\n"
    200
    ```
1. Even though the communication works, proxies are still connected to different control planes:
    ```bash
    istioctl ps -i istio-system-a
    NAME                                   CLUSTER        CDS               LDS               EDS               RDS               ECDS        ISTIOD                                      VERSION
    httpbin-5dbb8d6b45-mpgk7.httpbin-a     Kubernetes     SYNCED (7m4s)     SYNCED (7m4s)     SYNCED (7m4s)     SYNCED (7m4s)     IGNORED     istiod-tenant-a-v1-24-5-bd7b4c46b-jfzbz     1.24.5
    sleep-557d554568-nfzx9.sleep-a         Kubernetes     SYNCED (7m4s)     SYNCED (7m4s)     SYNCED (7m4s)     SYNCED (7m4s)     IGNORED     istiod-tenant-a-v1-24-5-bd7b4c46b-jfzbz     1.24.5
    ```
    ```bash
    istioctl ps -i istio-system-b
    NAME                                 CLUSTER        CDS               LDS               EDS               RDS               ECDS        ISTIOD                                      VERSION
    httpbin-fd948f9b-cbzw8.httpbin-b     Kubernetes     SYNCED (7m2s)     SYNCED (7m2s)     SYNCED (7m2s)     SYNCED (7m2s)     IGNORED     istiod-tenant-b-v1-24-5-54dcb986f-5jwfx     1.24.5
    sleep-6888c45d9b-f8445.sleep-b       Kubernetes     SYNCED (7m2s)     SYNCED (7m2s)     SYNCED (7m2s)     SYNCED (7m2s)     IGNORED     istiod-tenant-b-v1-24-5-54dcb986f-5jwfx     1.24.5
    ```
1. Update injection labels to connect proxies to the `tenant-a-v1-24-5` revision:
    ```bash
    kubectl label namespace sleep-b istio.io/rev=tenant-a-v1-24-5 --overwrite
    kubectl label namespace httpbin-b istio.io/rev=tenant-a-v1-24-5 --overwrite
    ```
1. Restart workloads:
    ```bash
    kubectl rollout restart deployment -n sleep-b
    kubectl rollout restart deployment -n httpbin-b
    ```
1. Verify that all proxies are connected to the `tenant-a-v1-24-5` revision:
    ```bash
    istioctl ps -i istio-system-a
    NAME                                   CLUSTER        CDS              LDS              EDS              RDS              ECDS        ISTIOD                                      VERSION
    httpbin-5dbb8d6b45-mpgk7.httpbin-a     Kubernetes     SYNCED (14m)     SYNCED (14m)     SYNCED (42s)     SYNCED (14m)     IGNORED     istiod-tenant-a-v1-24-5-bd7b4c46b-jfzbz     1.24.5
    httpbin-7747d468f8-ngwnx.httpbin-b     Kubernetes     SYNCED (48s)     SYNCED (48s)     SYNCED (42s)     SYNCED (48s)     IGNORED     istiod-tenant-a-v1-24-5-bd7b4c46b-jfzbz     1.24.5
    sleep-557d554568-nfzx9.sleep-a         Kubernetes     SYNCED (14m)     SYNCED (14m)     SYNCED (42s)     SYNCED (14m)     IGNORED     istiod-tenant-a-v1-24-5-bd7b4c46b-jfzbz     1.24.5
    sleep-778f4b5bbd-z2rhk.sleep-b         Kubernetes     SYNCED (52s)     SYNCED (52s)     SYNCED (42s)     SYNCED (52s)     IGNORED     istiod-tenant-a-v1-24-5-bd7b4c46b-jfzbz     1.24.5
    ```
1. Remove the no longer used `tenant-b` control plane:
    ```bash
    kubectl delete istio tenant-b
    ```
1. Remove unused labels:
    ```bash
    kubectl label namespace sleep-a tenant-
    kubectl label namespace sleep-b tenant-
    kubectl label namespace httpbin-a tenant-
    kubectl label namespace httpbin-b tenant-
    ```