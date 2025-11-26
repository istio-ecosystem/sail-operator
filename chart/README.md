# Deploy Sail Operator by using Helm charts

Follow this guide to install and configure Sail Operator by using [Helm](https://helm.sh/docs/)

## Prerequisites

Kubernetes:
* You have deployed a cluster on Kubernetes platform 1.27 or later.
* You are logged in to the Kubernetes cluster with admin permissions level user.

OpenShift:
* You have deployed a cluster on OpenShift Container Platform 4.14 or later.  
* You are logged in to the OpenShift Container Platform web console as a user with the `cluster-admin` role.

[Install the Helm client](https://helm.sh/docs/intro/install/), version 3.6 or above.

## Prepare the Helm charts

```sh
helm repo add sail-operator https://istio-ecosystem.github.io/sail-operator
helm repo update
```

## Installation steps

List available releases

```sh
helm search repo sail-operator --versions
```

This section describes the procedure to install `Sail Operator` using Helm. The general syntax for helm installation is:

```sh
helm install <release-name> <chart> --version <release-version> --create-namespace --namespace <namespace> [--set <other_parameters>]
```

The variables specified in the command are as follows:
* `<release-name>` - A name to identify and manage the Helm chart once installed.
* `<chart>` - A path to a packaged chart, a path to an unpacked chart directory or a URL.
* `<namespace>` - The namespace in which the chart is to be installed.
* `<release-version>` - A version number of the release.

Default configuration values can be changed using one or more `--set <parameter>=<value>` arguments. Alternatively, you can specify several parameters in a custom values file using the `--values <file>` argument.

1. Create the namespace, `sail-operator`, for the Sail Operator components:

    ```sh
    kubectl create namespace sail-operator
    ```

**Note** - This step could be skipped by using the `--create-namespace` argument in step 2.

2. Install the Sail Operator base charts which will manage all the Custom Resource Definitions(CRDs) to be able to deploy the Istio control plane:

    ```sh
    helm install sail-operator sail-operator/sail-operator --version 1.26.3 --namespace sail-operator
    ```

3. Validate the CRD installation with the `helm ls` command:

    ```sh
    helm ls -n sail-operator

    NAME  	      NAMESPACE    	REVISION	UPDATED                                	STATUS  	CHART               	APP VERSION
    sail-operator	sail-operator	1       	2025-08-20 16:14:12.210759174 +0300 IDT	deployed	sail-operator-1.26.3	1.26.3
    ```

4. Get the status of the installed helm chart to ensure it is deployed:

    ```bash
    helm status sail-operator -n sail-operator

    NAME: sail-operator
    LAST DEPLOYED: Wed Aug 20 16:14:12 2025
    NAMESPACE: sail-operator
    STATUS: deployed
    REVISION: 1
    TEST SUITE: None
    ```

5. Check `sail-operator` deployment is successfully installed and its pods are running:

    ```sh
    kubectl -n sail-operator get deployment --output wide

    NAME            READY   UP-TO-DATE   AVAILABLE   AGE     CONTAINERS      IMAGES                                  SELECTOR
    sail-operator   1/1     1            1           4h37m   sail-operator   quay.io/sail-dev/sail-operator:1.26.3   app.kubernetes.io/created-by=sailoperator,app.kubernetes.io/part-of=sailoperator,control-plane=sail-operator

    kubectl -n sail-operator get pods -o wide

    NAME                             READY   STATUS    RESTARTS   AGE     IP           NODE                                       NOMINATED NODE   READINESS GATES
    sail-operator-79c7d9ffb9-bwfdf   1/1     Running   0          4h38m   10.244.0.5   operator-integration-tests-control-plane   <none>           <none>
    ```

## Deploying Istio

For Istio deployment refer to the following section - [deploy](../README.md#deploying-the-istio-control-plane).

### Customizing Istio configuration

The `spec.values` field of the `Istio` and `IstioCNI` resource can be used to customize Istio and Istio CNI plugin configuration using Istio's `Helm` configuration values.

An example configuration:

    ```yaml
    apiVersion: sailoperator.io/v1
    kind: Istio
    metadata:
      name: example
    spec:
      version: v1.26.3
    values:
      global:
      mtls:
        enabled: true
      trustDomainAliases:
        - example.net
      meshConfig:
        trustDomain: example.com
        trustDomainAliases:
          - example.net
    ```

For a list of available configuration for the `spec.values` field, run the following command:

```sh
kubectl explain istio.spec.values
```

For the `IstioCNI` resource, replace `istio` with `istiocni` in the command above.

Alternatively, refer to [Istio's artifacthub chart documentation](https://artifacthub.io/packages/search?org=istio&sort=relevance&page=1) for:

- [Base parameters](https://artifacthub.io/packages/helm/istio-official/base?modal=values)
- [Istiod parameters](https://artifacthub.io/packages/helm/istio-official/istiod?modal=values)
- [Gateway parameters](https://artifacthub.io/packages/helm/istio-official/gateway?modal=values)
- [CNI parameters](https://artifacthub.io/packages/helm/istio-official/cni?modal=values)
- [ZTunnel parameters](https://artifacthub.io/packages/helm/istio-official/ztunnel?modal=values)

## Installing the istioctl tool

The `istioctl` tool is a configuration command line utility that allows service 
operators to debug and diagnose Istio service mesh deployments.

For installation steps, refer to the following [link](../docs/common/install-istioctl-tool.md).

## Installing the Bookinfo Application

You can use the `bookinfo` example application to explore service mesh features. 
Using the `bookinfo` application, you can easily confirm that requests from a 
web browser pass through the mesh and reach the application.

For installation steps, refer to the following [link](../docs/common/install-bookinfo-app.md).

## Creating and Configuring Gateways

The Sail Operator does not deploy Ingress or Egress Gateways. Gateways are not 
part of the control plane. As a security best-practice, Ingress and Egress 
Gateways should be deployed in a different namespace than the namespace that 
contains the control plane.

You can deploy gateways using either the Gateway API or Gateway Injection methods. 

For installation steps, refer to the following [link](../docs/common/create-and-configure-gateways.md).

## Istio Addons Integrations

Istio can be integrated with other software to provide additional functionality 
(More information can be found in: https://istio.io/latest/docs/ops/integrations/). 
The following addons are for demonstration or development purposes only and 
should not be used in production environments:

For installation steps, refer to the following [link](../docs/common/istio-addons-integrations.md).


## Undeploying Istio and the Sail Operator

### Deleting Istio

```sh
kubectl -n istio-system delete istio default
```

### Deleting IstioCNI (in Ambient mode or OpenShift cluster platform)

```sh
kubectl -n istio-cni delete istiocni default
```

### Deleting ZTunnel (in Ambient mode)

```sh
kubectl -n ztunnel delete ztunnel default
```

### Uninstall the Sail Operator using Helm

```sh
helm uninstall sail-operator --namespace sail-operator
```
 
### Deleting the Project namespaces

```sh
kubectl delete namespace istio-system
kubectl delete namespace istio-cni
kubectl delete namespace ztunnel
kubectl delete namespace sail-operator
```
