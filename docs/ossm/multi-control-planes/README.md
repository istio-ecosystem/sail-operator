# Multiple Istio Control Planes in a Single Cluster
By default, the control plane will watch all namespaces within the cluster so two control planes would be conflicting each other resulting in undefined behavior.

To resolve this, Istio provides [discoverySelectors](../create-mesh/README.md#discoveryselectors) which together with control plane revisions enables you to install multiple control planes in a single cluster.

## Prerequisites
- The OpenShift Service Mesh operator has been installed
- An Istio CNI resource has been created
- The `istioctl` binary has been installed on your localhost

## Deploying multiple control planes
The cluster will host two control planes installed in two different system namespaces. The mesh application workloads will run in multiple application-specific namespaces, each namespace associated with one or the other control plane based on revision and discovery selector configurations.

1. Create the first system namespace `usergroup-1`:
    ```bash
    oc create ns usergroup-1
    ```
1. Label the first system namespace:
    ```bash
    oc label ns usergroup-1 usergroup=usergroup-1
    ```
1. Prepare `istio-1.yaml`:
    ```yaml
    kind: Istio
    apiVersion: sailoperator.io/v1alpha1
    metadata:
      name: usergroup-1
    spec:
      namespace: usergroup-1
      values:
        meshConfig:
          discoverySelectors:
            - matchLabels:
                usergroup: usergroup-1
      updateStrategy:
        type: InPlace
      version: v1.23.0
    ```
1. Create `Istio` resource:
    ```bash
    oc apply -f istio-1.yaml
    ```
1. Create the second system namespace `usergroup-2`:
    ```bash
    oc create ns usergroup-2
    ```
1. Label the second system namespace:
    ```bash
    oc label ns usergroup-2 usergroup=usergroup-2
    ```
1. Prepare `istio-2.yaml`:
    ```yaml
    kind: Istio
    apiVersion: sailoperator.io/v1alpha1
    metadata:
      name: usergroup-2
    spec:
      namespace: usergroup-2
      values:
        meshConfig:
          discoverySelectors:
            - matchLabels:
                usergroup: usergroup-2
      updateStrategy:
        type: InPlace
      version: v1.23.0
    ```
1. Create `Istio` resource:
    ```bash
    oc apply -f istio-2.yaml
    ```
1. Deploy a policy for workloads in the `usergroup-1` namespace to only accept mutual TLS traffic `peer-auth-1.yaml`:
    ```yaml
    apiVersion: security.istio.io/v1
    kind: PeerAuthentication
    metadata:
      name: "usergroup-1-peerauth"
      namespace: "usergroup-1"
    spec:
      mtls:
        mode: STRICT
    ```
    ```bash
    oc apply -f peer-auth-1.yaml
    ```
1. Deploy a policy for workloads in the `usergroup-2` namespace to only accept mutual TLS traffic `peer-auth-2.yaml`:
    ```yaml
    apiVersion: security.istio.io/v1
    kind: PeerAuthentication
    metadata:
      name: "usergroup-2-peerauth"
      namespace: "usergroup-2"
    spec:
      mtls:
        mode: STRICT
    ```
    ```bash
    oc apply -f peer-auth-2.yaml
    ```
1. Verify the control planes are deployed and running:
    ```bash
    oc get pods -n usergroup-1
    NAME                                  READY   STATUS    RESTARTS   AGE
    istiod-usergroup-1-747fddfb56-xzpkj   1/1     Running   0          5m1s
    oc get pods -n usergroup-2
    NAME                                  READY   STATUS    RESTARTS   AGE
    istiod-usergroup-2-5b9cbb7669-lwhgv   1/1     Running   0          3m41s
    ```

## Deploy application workloads per usergroup
1. Create three application namespaces:
    ```bash
    oc create ns app-ns-1
    oc create ns app-ns-2
    oc create ns app-ns-3
    ```
1. Label each namespace to associate them with their respective control planes:
    ```bash
    oc label ns app-ns-1 usergroup=usergroup-1 istio.io/rev=usergroup-1
    oc label ns app-ns-2 usergroup=usergroup-2 istio.io/rev=usergroup-2
    oc label ns app-ns-3 usergroup=usergroup-2 istio.io/rev=usergroup-2
    ```
1. Deploy one `sleep` and `httpbin` application per namespace:
    ```bash
    oc apply -f https://raw.githubusercontent.com/istio/istio/release-1.23/samples/sleep/sleep.yaml -n app-ns-1
    oc apply -f https://raw.githubusercontent.com/istio/istio/master/samples/httpbin/httpbin.yaml -n app-ns-1
    oc apply -f https://raw.githubusercontent.com/istio/istio/release-1.23/samples/sleep/sleep.yaml -n app-ns-2
    oc apply -f https://raw.githubusercontent.com/istio/istio/master/samples/httpbin/httpbin.yaml -n app-ns-2
    oc apply -f https://raw.githubusercontent.com/istio/istio/release-1.23/samples/sleep/sleep.yaml -n app-ns-3
    oc apply -f https://raw.githubusercontent.com/istio/istio/master/samples/httpbin/httpbin.yaml -n app-ns-3
    ```
1. Wait a few seconds for the `httpbin` and `sleep` pods to be running with sidecars injected:
    ```bash
    oc get pods -n app-ns-1
    NAME                       READY   STATUS    RESTARTS   AGE
    httpbin-7f56dc944b-kpw2x   2/2     Running   0          2m26s
    sleep-5577c64d7c-b5wd2     2/2     Running   0          91m
    ```
    Repeat this step for other application namespaces (`app-ns-2`, `app-ns-3`).
> [!TIP]
> `oc wait deployment sleep -n app-ns-1` can be used to wait for a deployment to be ready

## Verify the application to control plane mapping
Now that the applications are deployed, you can use the `istioctl ps` command to confirm that the application workloads are managed by their respective control plane, i.e., `app-ns-1` is managed by `usergroup-1`, `app-ns-2` and `app-ns-3` are managed by `usergroup-2`:
```bash
istioctl ps -i usergroup-1
NAME                                  CLUSTER        CDS                LDS                EDS                RDS                ECDS        ISTIOD                                  VERSION
httpbin-7f56dc944b-kpw2x.app-ns-1     Kubernetes     SYNCED (2m23s)     SYNCED (2m23s)     SYNCED (2m23s)     SYNCED (2m23s)     IGNORED     istiod-usergroup-1-747fddfb56-xzpkj     1.23.0
sleep-5577c64d7c-b5wd2.app-ns-1       Kubernetes     SYNCED (66s)       SYNCED (66s)       SYNCED (66s)       SYNCED (66s)       IGNORED     istiod-usergroup-1-747fddfb56-xzpkj     1.23.0
```
```bash
istioctl ps -i usergroup-2
NAME                                  CLUSTER        CDS               LDS               EDS             RDS               ECDS        ISTIOD                                  VERSION
httpbin-7f56dc944b-g4s57.app-ns-3     Kubernetes     SYNCED (2m)       SYNCED (2m)       SYNCED (2m)     SYNCED (2m)       IGNORED     istiod-usergroup-2-5b9cbb7669-lwhgv     1.23.0
httpbin-7f56dc944b-rzwr5.app-ns-2     Kubernetes     SYNCED (2m2s)     SYNCED (2m2s)     SYNCED (2m)     SYNCED (2m2s)     IGNORED     istiod-usergroup-2-5b9cbb7669-lwhgv     1.23.0
sleep-5577c64d7c-wjnxc.app-ns-3       Kubernetes     SYNCED (2m2s)     SYNCED (2m2s)     SYNCED (2m)     SYNCED (2m2s)     IGNORED     istiod-usergroup-2-5b9cbb7669-lwhgv     1.23.0
sleep-5577c64d7c-xk27f.app-ns-2       Kubernetes     SYNCED (2m2s)     SYNCED (2m2s)     SYNCED (2m)     SYNCED (2m2s)     IGNORED     istiod-usergroup-2-5b9cbb7669-lwhgv     1.23.0
```
## Verify the application connectivity is ONLY within the respective usergroup
1. Send a request from the `sleep` pod in `app-ns-1` in `usergroup-1` to the `httpbin` service in `app-ns-2` in `usergroup-2`. The communication should fail:
    ```bash
    oc -n app-ns-1 exec "$(oc -n app-ns-1 get pod -l app=sleep -o jsonpath={.items..metadata.name})" -c sleep -- curl -sIL http://httpbin.app-ns-2.svc.cluster.local:8000
    HTTP/1.1 503 Service Unavailable
    content-length: 95
    content-type: text/plain
    date: Wed, 16 Oct 2024 12:05:37 GMT
    server: envoy
    ```
1. Send a request from the `sleep` pod in `app-ns-2` in `usergroup-2` to the `httpbin` service in `app-ns-3` in `usergroup-2`. The communication should work:
    ```bash
    oc -n app-ns-2 exec "$(oc -n app-ns-2 get pod -l app=sleep -o jsonpath={.items..metadata.name})" -c sleep -- curl -sIL http://httpbin.app-ns-3.svc.cluster.local:8000
    HTTP/1.1 200 OK
    access-control-allow-credentials: true
    access-control-allow-origin: *
    content-security-policy: default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' camo.githubusercontent.com
    content-type: text/html; charset=utf-8
    date: Wed, 16 Oct 2024 12:06:30 GMT
    x-envoy-upstream-service-time: 8
    server: envoy
    transfer-encoding: chunked
    ```