[Return to Project Root](../README.md)

# Table of Contents
- [Multiple meshes on a single cluster](#multiple-meshes-on-a-single-cluster)
  - [Prerequisites](#prerequisites)
  - [Installation Steps](#installation-steps)
    - [Deploying the control planes](#deploying-the-control-planes)
    - [Verifying the control planes](#verifying-the-control-planes)
    - [Deploying the applications](#deploying-the-applications)
  - [Validation](#validation)
    - [Checking application to control plane mapping](#checking-application-to-control-plane-mapping)
    - [Checking application connectivity](#checking-application-connectivity)
  - [Cleanup](#cleanup)

## Multiple meshes on a single cluster

The Sail Operator supports running multiple meshes on a single cluster and associating each workload with a specific mesh. 
Each mesh is managed by a separate control plane.

Applications are installed in multiple namespaces, and each namespace is associated with one of the control planes through its labels.
The `istio.io/rev` label determines which control plane injects the sidecar proxy into the application pods.
Additional namespace labels determine whether the control plane discovers and manages the resources in the namespace. 
A control plane will discover and manage only those namespaces that match the discovery selectors configured on the control plane.
Additionally, discovery selectors determine which control plane creates the `istio-ca-root-cert` ConfigMap in which namespace.

Currently, discovery selectors in multiple control planes must be configured so that they don't overlap (i.e. the discovery selectors of two control planes don't match the same namespace).
Each control plane must be deployed in a separate Kubernetes namespace.

This guide explains how to set up two meshes: `mesh1` and `mesh2` in namespaces `istio-system1` and `istio-system2`, respectively, and three application namespaces: `app1`, `app2a`, and `app2b`.
Mesh 1 will manage namespace `app1`, and Mesh 2 will manage namespaces `app2a` and `app2b`.
Because each mesh will use its own root certificate authority and configured to use a peer authentication policy with the `STRICT` mTLS mode, the communication between the two meshes will not be allowed. 

### Prerequisites

- Install [istioctl](../common/install-istioctl-tool.md).
- Kubernetes 1.23 cluster.
- kubeconfig file with a context for the Kubernetes cluster.
- Install the Sail Operator and the Sail CRDs to the cluster.

### Installation Steps

#### Deploying the control planes

1. Create the system namespace `istio-system1` and deploy the `mesh1` control plane in it.
   ```bash { name=deploy-mesh1 tag=multiple-meshes}
   kubectl create namespace istio-system1
   kubectl label ns istio-system1 mesh=mesh1
   kubectl apply -f - <<EOF
   apiVersion: sailoperator.io/v1
   kind: Istio
   metadata:
     name: mesh1
   spec:
     namespace: istio-system1
     version: v1.24.0
     values:
       meshConfig:
         discoverySelectors:
         - matchLabels:
             mesh: mesh1
   EOF
   ```
<!-- ```bash { name=validation-wait-istio-pods tag=multiple-meshes}
    . scripts/prebuilt-func.sh
    wait_istio_ready "istio-system1"
    kubectl get pods -n istio-system1
``` -->
2. Create the system namespace `istio-system2` and deploy the `mesh2` control plane in it.
   ```bash { name=deploy-mesh2 tag=multiple-meshes}
   kubectl create namespace istio-system2
   kubectl label ns istio-system2 mesh=mesh2
   kubectl apply -f - <<EOF
   apiVersion: sailoperator.io/v1
   kind: Istio
   metadata:
     name: mesh2
   spec:
     namespace: istio-system2
     version: v1.24.0
     values:
       meshConfig:
         discoverySelectors:
         - matchLabels:
             mesh: mesh2
   EOF
   ```
<!-- ```bash { name=validation-wait-istio-pods tag=multiple-meshes}
    . scripts/prebuilt-func.sh
    wait_istio_ready "istio-system2"
    kubectl get pods -n istio-system2
``` -->
3. Create a peer authentication policy that only allows mTLS communication within each mesh.
   ```bash { name=peer-authentication tag=multiple-meshes}
   kubectl apply -f - <<EOF
   apiVersion: security.istio.io/v1
   kind: PeerAuthentication
   metadata:
     name: default
     namespace: istio-system1
   spec:
     mtls:
       mode: STRICT
   EOF
   
   kubectl apply -f - <<EOF
   apiVersion: security.istio.io/v1
   kind: PeerAuthentication
   metadata:
     name: default
     namespace: istio-system2
   spec:
     mtls:
       mode: STRICT
   EOF
   ```  

#### Verifying the control planes

1. Check the labels on the control plane namespaces:
   ```console
   $ kubectl get ns -l mesh -L mesh
   NAME            STATUS   AGE    MESH
   istio-system1   Active   106s   mesh1
   istio-system2   Active   105s   mesh2
   ```
<!-- ```bash { name=validation-control-planes-ns tag=multiple-meshes}
   kubectl get ns -l mesh -L mesh
   kubectl get pods -n istio-system1
   kubectl get pods -n istio-system2
``` -->
2. Check the control planes are `Healthy`:
   ```console
   $ kubectl get istios
   NAME    REVISIONS   READY   IN USE   ACTIVE REVISION   STATUS    VERSION   AGE
   mesh1   1           1       0        mesh1             Healthy   v1.24.0   84s
   mesh2   1           1       0        mesh2             Healthy   v1.24.0   77s
   ```
<!-- ```bash { name=validation-istios tag=multiple-meshes}
   kubectl get istios
``` -->
3. Confirm that the validation and mutation webhook configurations exist for both meshes:
   ```console
   $ kubectl get validatingwebhookconfigurations
   NAME                                  WEBHOOKS   AGE
   istio-validator-mesh1-istio-system1   1          2m45s
   istio-validator-mesh2-istio-system2   1          2m38s

   $ kubectl get mutatingwebhookconfigurations
   NAME                                         WEBHOOKS   AGE
   istio-sidecar-injector-mesh1-istio-system1   2          5m55s
   istio-sidecar-injector-mesh2-istio-system2   2          5m48s
   ```
<!-- ```bash { name=validation-webhook-configs tag=multiple-meshes}
   kubectl get validatingwebhookconfigurations
   kubectl get mutatingwebhookconfigurations
``` -->
#### Deploying the applications

1. Create three application namespaces:
   ```bash { name=create-app-namespaces tag=multiple-meshes}
   kubectl create ns app1 
   kubectl create ns app2a 
   kubectl create ns app2b
   ```

2. Label each namespace to enable discovery by the corresponding control plane:
   ```bash { name=label-app-namespaces tag=multiple-meshes}
   kubectl label ns app1 mesh=mesh1
   kubectl label ns app2a mesh=mesh2
   kubectl label ns app2b mesh=mesh2
   ```

3. Label each namespace to enable injection by the corresponding control plane:
   ```bash { name=label-rev-app-namespaces tag=multiple-meshes}
   kubectl label ns app1 istio.io/rev=mesh1
   kubectl label ns app2a istio.io/rev=mesh2
   kubectl label ns app2b istio.io/rev=mesh2
   ```

4. Deploy the `curl` and `httpbin` sample applications in each namespace:
   ```bash { name=deploy-apps tag=multiple-meshes}
   # Deploy curl and httpbin in app1
   kubectl -n app1 apply -f https://raw.githubusercontent.com/istio/istio/refs/heads/master/samples/curl/curl.yaml 
   kubectl -n app1 apply -f https://raw.githubusercontent.com/istio/istio/refs/heads/master/samples/httpbin/httpbin.yaml 
   # Deploy curl and httpbin in app2a and app2b
   kubectl -n app2a apply -f https://raw.githubusercontent.com/istio/istio/refs/heads/master/samples/curl/curl.yaml 
   kubectl -n app2a apply -f https://raw.githubusercontent.com/istio/istio/refs/heads/master/samples/httpbin/httpbin.yaml 
   # Deploy curl and httpbin in app2b
   kubectl -n app2b apply -f https://raw.githubusercontent.com/istio/istio/refs/heads/master/samples/curl/curl.yaml 
   kubectl -n app2b apply -f https://raw.githubusercontent.com/istio/istio/refs/heads/master/samples/httpbin/httpbin.yaml 
   ```
<!-- ```bash { name=validation-app-deployed tag=multiple-meshes}
    . scripts/prebuilt-func.sh
    with_retries wait_pods_ready_by_ns "app1"
    kubectl get pods -n app1
    with_retries pods_istio_version_match "app1" "1.24.0" "istio-system1"
    with_retries wait_pods_ready_by_ns "app2a"
    kubectl get pods -n app2a
    with_retries pods_istio_version_match "app2a" "1.24.0" "istio-system2"
     with_retries wait_pods_ready_by_ns "app2b"
    kubectl get pods -n app2b
    with_retries pods_istio_version_match "app2b" "1.24.0" "istio-system2"
``` -->
5. Confirm that a sidecar has been injected into each of the application pods. The value `2/2` should be displayed in the `READY` column for each pod, as in the following example:
   ```console
   $ kubectl get pods -n app1
   NAME                       READY   STATUS    RESTARTS   AGE
   curl-5b549b49b8-mg7nl      2/2     Running   0          102s
   httpbin-7b549f7859-h6hnk   2/2     Running   0          89s

   $ kubectl get pods -n app2a
   NAME                       READY   STATUS    RESTARTS   AGE
   curl-5b549b49b8-2hlvm      2/2     Running   0          2m3s
   httpbin-7b549f7859-bgblg   2/2     Running   0          110s

   $ kubectl get pods -n app2b
   NAME                       READY   STATUS    RESTARTS   AGE
   curl-5b549b49b8-xnzzk      2/2     Running   0          2m9s
   httpbin-7b549f7859-7k5gf   2/2     Running   0          118s
   ```

### Validation

#### Checking application to control plane mapping

Use the `istioctl ps` command to confirm that the application pods are connected to the correct control plane. 

The `curl` and `httpbin` pods in namespace `app1` should be connected to the control plane in namespace `istio-system1`, as shown in the following example (note the `.app1` suffix in the `NAME` column):

```console
$ istioctl ps -i istio-system1
NAME                              CLUSTER        CDS                LDS                EDS                RDS                ECDS        ISTIOD                            VERSION
curl-5b549b49b8-mg7nl.app1        Kubernetes     SYNCED (4m40s)     SYNCED (4m40s)     SYNCED (4m31s)     SYNCED (4m40s)     IGNORED     istiod-mesh1-5df45b97dd-tf2wl     1.24.0
httpbin-7b549f7859-h6hnk.app1     Kubernetes     SYNCED (4m31s)     SYNCED (4m31s)     SYNCED (4m31s)     SYNCED (4m31s)     IGNORED     istiod-mesh1-5df45b97dd-tf2wl     1.24.0
```

The pods in namespaces `app2a` and `app2b` should be connected to the control plane in namespace `istio-system2`:

```console
$ istioctl ps -i istio-system2
NAME                               CLUSTER        CDS                LDS                EDS                RDS                ECDS        ISTIOD                            VERSION
curl-5b549b49b8-2hlvm.app2a        Kubernetes     SYNCED (4m37s)     SYNCED (4m37s)     SYNCED (4m31s)     SYNCED (4m37s)     IGNORED     istiod-mesh2-59f6b874fb-mzxqw     1.24.0
curl-5b549b49b8-xnzzk.app2b        Kubernetes     SYNCED (4m37s)     SYNCED (4m37s)     SYNCED (4m31s)     SYNCED (4m37s)     IGNORED     istiod-mesh2-59f6b874fb-mzxqw     1.24.0
httpbin-7b549f7859-7k5gf.app2b     Kubernetes     SYNCED (4m31s)     SYNCED (4m31s)     SYNCED (4m31s)     SYNCED (4m31s)     IGNORED     istiod-mesh2-59f6b874fb-mzxqw     1.24.0
httpbin-7b549f7859-bgblg.app2a     Kubernetes     SYNCED (4m32s)     SYNCED (4m32s)     SYNCED (4m31s)     SYNCED (4m32s)     IGNORED     istiod-mesh2-59f6b874fb-mzxqw     1.24.0
```
<!-- ```bash { name=validation-print-ps tag=multiple-meshes}
    istioctl ps -i istio-system1
    istioctl ps -i istio-system2
``` -->
#### Checking application connectivity

As both meshes are configured to use the `STRICT` mTLS peer authentication mode, the applications in namespace `app1` should not be able to communicate with the applications in namespaces `app2a` and `app2b`, and vice versa.
To test whether the `curl` pod in namespace `app2a` can connect to the `httpbin` service in namespace `app1`, run the following commands:

```console
$ kubectl -n app2a exec deploy/curl -c curl -- curl -sIL http://httpbin.app1:8000
HTTP/1.1 503 Service Unavailable
content-length: 95
content-type: text/plain
date: Fri, 29 Nov 2024 08:58:28 GMT
server: envoy
```
<!-- ```bash { name=validation-curl-app1 tag=multiple-meshes}
    response=$(kubectl -n app2a exec deploy/curl -c curl --   curl -s -o /dev/null -w "%{http_code}" http://httpbin.app1:8000)
    echo $response
    if [ "$response" -eq 503 ]; then
        echo "Connection to httpbin.app1:8000 failed as expected"
    else
        echo "Connection to httpbin.app1:8000 succeeded unexpectedly"
        exit 1
    fi
``` -->
As expected, the response indicates that the connection was not successful. 
In contrast, the same pod should be able to connect to the `httpbin` service in namespace `app2b`, because they are part of the same mesh:

```console
$ kubectl -n app2a exec deploy/curl -c curl -- curl -sIL http://httpbin.app2b:8000
HTTP/1.1 200 OK
access-control-allow-credentials: true
access-control-allow-origin: *
content-security-policy: default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' camo.githubusercontent.com
content-type: text/html; charset=utf-8
date: Fri, 29 Nov 2024 08:57:52 GMT
x-envoy-upstream-service-time: 0
server: envoy
transfer-encoding: chunked
```
<!-- ```bash { name=validation-curl-app2a tag=multiple-meshes}
    response=$(kubectl -n app2a exec deploy/curl -c curl -- curl -s -o /dev/null -w "%{http_code}" http://httpbin.app2b:8000)
    echo $response
    if [ "$response" -eq 503 ]; then
        echo "Connection to httpbin.app1:8000 failed unexpectedly"
        exit 1
    else
        echo "Connection to httpbin.app1:8000 succeeded as expected"
    fi
``` -->
### Cleanup

To clean up the resources created in this guide, delete the `Istio` resources and the namespaces:

```bash
   kubectl delete istio mesh1 mesh2
   kubectl delete ns istio-system1 istio-system2 app1 app2a app2b
```
