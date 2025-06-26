# Third-party CNI integration with OpenShift Service Mesh 3

This document provides guidance on how to deploy and validate OpenShift Service Mesh 3.x with a third-party CNI on Red Hat OpenShift, using the Sail Operator e2e testing framework. It includes steps to configure the environment and run tests to ensure compatibility and functionality with the selected CNI.

> [!Note]
> The responsibility for testing and declaring support for OpenShift Service Mesh (OSSM) with 3rd party CNIs remains with vendors supporting those 3rd party CNIs (e.g. Tigera for Calico, Isovalent/Cisco for Cilium, etc). This documentation is for these vendors to use as part of their testing to become a “certified CNI plugin” for OpenShift. The OpenShift Service Mesh team will only ever test/support OSSM on Red Hat supported CNIs (e.g. OVN-K, SDN). By default, OCP uses OVN-K as the default CNI.

*Key Point*: Calico serves as the Container Network Interface (CNI) in the following example. The document presumes a fully configured cluster with an actively running third-party CNI.

## Global Prerequisites

These prerequisites apply to **all test scenarios** covered in this document:

* OCP cluster running with a third-party CNI (e.g., Calico).

```console
$ oc get tigerastatus
NAME        AVAILABLE   PROGRESSING   DEGRADED   SINCE
apiserver   True        False         False      23m
calico      True        False         False      14m
goldmane    True        False         False      23m
ippools     True        False         False      15m
whisker     True        False         False      24m
```

```console
$ oc get nodes
NAME                          STATUS   ROLES                  AGE   VERSION
ip-10-0-25-190.ec2.internal   Ready    control-plane,master   84m   v1.31.8
ip-10-0-43-214.ec2.internal   Ready    control-plane,master   83m   v1.31.8
ip-10-0-48-121.ec2.internal   Ready    worker                 77m   v1.31.8
ip-10-0-85-125.ec2.internal   Ready    worker                 77m   v1.31.8
ip-10-0-9-228.ec2.internal    Ready    worker                 73m   v1.31.8
ip-10-0-91-204.ec2.internal   Ready    control-plane,master   83m   v1.31.8
```

```console
$ oc get pods -n calico-system
NAME                                       READY   STATUS    RESTARTS   AGE
calico-kube-controllers-6968f55b5f-pkrv9   1/1     Running   0          85m
calico-node-5kwrh                          1/1     Running   0          85m
calico-node-cxntv                          1/1     Running   0          85m
calico-node-f4b7l                          1/1     Running   0          79m
calico-node-hln4k                          1/1     Running   0          76m
calico-node-rw65s                          1/1     Running   0          85m
calico-node-trj56                          1/1     Running   0          79m
calico-typha-6c65ff74f4-r992t              1/1     Running   0          85m
calico-typha-6c65ff74f4-vsn4v              1/1     Running   0          79m
calico-typha-6c65ff74f4-wnprq              1/1     Running   0          85m
csi-node-driver-652qk                      2/2     Running   0          85m
csi-node-driver-gx8bq                      2/2     Running   0          76m
csi-node-driver-hrzt4                      2/2     Running   0          85m
csi-node-driver-kxfhl                      2/2     Running   0          85m
csi-node-driver-pdp5h                      2/2     Running   0          79m
csi-node-driver-x82rg                      2/2     Running   0          79m
goldmane-ff655769-jssz9                    1/1     Running   0          85m
whisker-5f848c455b-fs6j5                   2/2     Running   0          85m
```

* OpenShift Service Mesh Operator installed and running on the cluster.

```console
$ oc get pods -n openshift-operators
NAME                                     READY   STATUS    RESTARTS   AGE
servicemesh-operator3-86b7ffc8fb-bcsqh   1/1     Running   0          51m
```

*Note:* validate the CNI configuration before starting to run the test, for example: be sure that the CNI configuration file is present in the `/etc/cni/multus/net.d` directory.

## CNI third-party test with default configuration for OSSM resources.

The default configuration for Istio and IstioCNI resources are:

```console
cni:
  cniBinDir: /var/lib/cni/bin
  cniConfDir: /etc/cni/multus/net.d
  chained: false
  cniConfFileName: "istio-cni.conf"
  provider: "multus"
pilot:
  cni:
    enabled: true
    provider: "multus"
```
As you can see, the default configuration uses Multus as the CNI provider.

### Prerequisites

* Multus CNI installed and configured on the cluster.

```console
$ oc get pods -n openshift-multus 
NAME                                           READY   STATUS    RESTARTS   AGE
multus-9lhkk                                   1/1     Running   0          85m
multus-additional-cni-plugins-28f4p            1/1     Running   0          79m
multus-additional-cni-plugins-b75fr            1/1     Running   0          85m
multus-additional-cni-plugins-khldk            1/1     Running   0          76m
multus-additional-cni-plugins-ls4fj            1/1     Running   0          85m
multus-additional-cni-plugins-q7cgj            1/1     Running   0          79m
multus-additional-cni-plugins-w5vjp            1/1     Running   0          85m
multus-admission-controller-849c469dd5-md9zp   2/2     Running   0          83m
multus-admission-controller-849c469dd5-nwx8n   2/2     Running   0          83m
multus-c8jkf                                   1/1     Running   0          79m
multus-gbb99                                   1/1     Running   0          85m
multus-h7twd                                   1/1     Running   0          79m
multus-rbp6b                                   1/1     Running   0          85m
multus-zt6wh                                   1/1     Running   0          76m
network-metrics-daemon-4kp67                   2/2     Running   0          85m
network-metrics-daemon-4vg24                   2/2     Running   0          85m
network-metrics-daemon-clx5s                   2/2     Running   0          79m
network-metrics-daemon-js96f                   2/2     Running   0          85m
network-metrics-daemon-tqlw8                   2/2     Running   0          76m
network-metrics-daemon-vggbb                   2/2     Running   0          79m
```

### Running e2e framework from Sail Operator
Using the e2e testing framework will run a subset of tests that will install and validate the OSSM configuration over the cluster.

*Notes:*
* For more information about the testing framework please refer to the upstream [documentation](https://github.com/istio-ecosystem/sail-operator/tree/main/tests/e2e).
* [Here](https://github.com/istio-ecosystem/sail-operator/tree/main/tests/e2e#using-the-e2e-framework-to-test-your-cluster-configuration) you will find specific information about running the framework against your specific cluster configuration.

#### Steps to run the e2e tests

1. Clone the Sail Operator repository:

```console
git clone https://github.com/istio-ecosystem/sail-operator
cd sail-operator
```

2. Run the make target to run the e2e tests:

Note: Before running the test please check the current supported versions of the current OSSM version that you want to test, this versions need to be added to a custom versions file following this [documentation](https://github.com/istio-ecosystem/sail-operator/blob/main/tests/e2e/README.md#running-the-testing-framework-against-specific-istio-versions).

```console
SKIP_DEPLOY=true SKIP_BUILD=true DEPLOYMENT_NAME=servicemesh-operator3 NAMESPACE=openshift-operators EXPECTED_REGISTRY="^registry.redhat.io" GINKGO_FLAGS="-v VERSIONS_YAML_FILE=custom_versions.yaml --label-filter=smoke" make test.e2e.ocp
```
Note:
* The above command sets several environment variables to control the behavior of the e2e tests:
  - `SKIP_DEPLOY`: Skips the deployment of the operator. Your already have the operator running on the cluster.
  - `SKIP_BUILD`: Skips the build and push of the operator image to the cluster. The test are going to use the running operator from the cluster.
  - `DEPLOYMENT_NAME`: Matches the default name of the deployment for OSSM3.
  - `NAMESPACE`: Sets the namespace where the operator is being deployed.
  - `EXPECTED_REGISTRY`: Sets the expected registry for image validation.
  - `GINKGO_FLAGS`: Enables verbose output and filters tests by label. For this test we are going to run smoke test suite.
  - `VERSIONS_YAML_FILE`: Specifies the custom versions file to use for the test run. This file should contain the versions of Istio and IstioCNI that you want to test against and are available in the current OSSM operator version.

3. Validate the test results:
If the test run ends successfully, you will see at the end of the test execution: “Test Suite Passed”. For example
```console
Ginkgo ran 6 suites in 5m9.079600163s
Test Suite Passed
```

Note that if the test run fails, you will see this message: "Test Suite Failed". More information about reading the test results [here](https://github.com/istio-ecosystem/sail-operator/tree/main/tests/e2e#understanding-the-test-output).

Additionally, the test run generates an XML file with the complete report. To understand and check the output, you can check the file and look for the second line to get the general result of the test run:
```console
<?xml version="1.0" encoding="UTF-8"?>
  <testsuites tests="67" disabled="12" errors="0" failures="0" time="306.809194592">
```
The `tests` attribute indicates the total number of tests run, `disabled` indicates the number of tests that were skipped, `errors` indicates the number of tests that encountered errors, `failures` indicates the number of tests that failed, and `time` indicates the total time taken for the test run. If you see that failures are greater than 0, you will need to check the entire output of the test run to see the exact step where a test failed, these steps are inside the same XML file.

#### Manual validation using default settings of Istio and IstioCNI
Besides the test execution of the e2e framework, you can also do some manual steps validations using bookinfo to ensure that the CNI configuration is correct.

1. Install manually Istio and IstioCNI resources using the default configuration:
Please refer to the [documentation](https://docs.redhat.com/en/documentation/red_hat_openshift_service_mesh/3.0/html/installing/ossm-installing-service-mesh#about-istio-deployment_ossm-about-deploying-istio-using-service-mesh-operator) to install the Istio and IstioCNI resources using the default configuration.

2. Install bookinfo application:
```console
oc create namespace bookinfo
oc label namespace bookinfo istio-injection=enabled
oc apply -n bookinfo -f https://raw.githubusercontent.com/istio/istio/release-1.24/samples/bookinfo/platform/kube/bookinfo.yaml
```

3. Validate bookinfo application:
```console
oc get pods -n bookinfo
NAME                             READY   STATUS    RESTARTS   AGE
details-v1-696f7cfbcb-4xcsg      2/2     Running   0          3m52s
productpage-v1-94cdccd8f-tk9vv   2/2     Running   0          3m48s
ratings-v1-649c95cf4d-qrqcm      2/2     Running   0          3m51s
reviews-v1-6469d4f55b-gkjm2      2/2     Running   0          3m50s
reviews-v2-56b87cb468-2cj5w      2/2     Running   0          3m49s
reviews-v3-7bdb75f6fb-lfbq6      2/2     Running   0          3m49s
```

4. Validate istio-cni pod logs:
```console
COMMIT
# Completed on Tue May 20 18:03:02 2025
2025-05-20T18:03:02.028824Z info cni-plugin ============= End iptables configuration for reviews-v2-56b87cb468-2cj5w =============
2025-05-20T18:03:03.180714Z info cni-plugin ============= Start iptables configuration for productpage-v1-94cdccd8f-tk9vv =============
2025-05-20T18:03:03.196098Z info cni-plugin Istio iptables environment:
ENVOY_PORT=
INBOUND_CAPTURE_PORT=
ISTIO_INBOUND_INTERCEPTION_MODE=
ISTIO_INBOUND_TPROXY_ROUTE_TABLE=
ISTIO_INBOUND_PORTS=
ISTIO_OUTBOUND_PORTS=
ISTIO_LOCAL_EXCLUDE_PORTS=
ISTIO_EXCLUDE_INTERFACES=
ISTIO_SERVICE_CIDR=
ISTIO_SERVICE_EXCLUDE_CIDR=
ISTIO_META_DNS_CAPTURE=
INVALID_DROP=
2025-05-20T18:03:03.196141Z info cni-plugin Istio iptables variables:
PROXY_PORT=15001
PROXY_INBOUND_CAPTURE_PORT=15006
PROXY_TUNNEL_PORT=15008
PROXY_UID=1001119999
PROXY_GID=1001119999
INBOUND_INTERCEPTION_MODE=REDIRECT
INBOUND_TPROXY_MARK=1337
INBOUND_TPROXY_ROUTE_TABLE=133
INBOUND_PORTS_INCLUDE=*
INBOUND_PORTS_EXCLUDE=15020,15021,15090
```

As you can see in the logs:
`2025-05-20T18:03:03.180714Z info cni-plugin ============= Start iptables configuration for productpage-v1-94cdccd8f-tk9vv =============`
This line confirms that the Istio CNI plugin was invoked and started its setup process for this specific productpage pod. Also, checking the init container logs in the productpage pods shows:
```console
2025-05-20T18:02:59.773965Z info Starting iptables validation. This check verifies that iptables rules are properly established for the network.
2025-05-20T18:02:59.774017Z info Listening on 127.0.0.1:15001
2025-05-20T18:02:59.774304Z info Listening on 127.0.0.1:15006
2025-05-20T18:02:59.774599Z info Local addr 127.0.0.1:15006
2025-05-20T18:02:59.774643Z info Original addr 127.0.0.1: 15002
2025-05-20T18:02:59.774724Z info Validation passed, iptables rules established
```
This confirms that the purpose of this init container is to verify the iptables setup, not to create it. This is the hallmark of the Istio CNI plugin being active. istio-validation container confirmed that the necessary iptables rules were already in place in the pod's network namespace when it started. This means the Istio CNI plugin successfully applied them earlier in the pod's lifecycle

## CNI third-party test with chained equal true and using provider default

### Prerequisites

* Get the directory and path of the CNI configuration. To achieve this, validate where are located the cni binaries and where is located the cni configuration file. This is going to be needed to place the custom configuration in the resources. You can use the following commands from one of cluster nodes to validate the CNI configuration and binaries:

```console 
$ find / -iname 10-calico.conflist
/run/multus/cni/net.d/10-calico.conflist
$ ls -l /host/var/lib/cni/bin/ |grep calico
-rwxr-xr-x. 1 root root 76781691 May 21 08:18 calico
-rwxr-xr-x. 1 root root 76781691 May 21 08:18 calico-ipam
$ ls -l /host/var/lib/cni/bin/ |grep multus
-rwxr-xr-x. 1 root root  2530555 May 21 08:18 install_multus
-rwxr-xr-x. 1 root root 54064704 May 21 08:18 multus
-rwxr-xr-x. 1 root root 58529952 May 21 08:18 multus-daemon
-rwxr-xr-x. 1 root root 45518520 May 21 08:18 multus-shim
```
As you can see, the CNI configuration file is located in `/run/multus/cni/net.d/10-calico.conflist` and the CNI binaries are located in `/host/var/lib/cni/bin/` in this example.

### Validation steps
To validate the integration of OpenShift Service Mesh with a third-party CNI you can follow these steps:

1. Install Istio and IstioCNI resources with chained equal true and using provider default:

From the information about the CNI configuration that we gather before we now know the specific configuration for the Istio and IstioCNI resources that we need to use for this validation. The configuration is as follows for our example:

```yaml
apiVersion: sailoperator.io/v1
kind: IstioCNI
  name: default
spec:
  namespace: istio-cni
  values:
    cni:
      chained: true
      cniBinDir: /var/lib/cni/bin
      cniConfDir: /run/multus/cni/net.d
      cniConfFileName: 10-calico.conflist
      provider: default
  version: v1.24.5
```
The configuration set for the resource IstioCNI is as follows:
* *chained*: true
* *cniBinDir*: "/var/lib/cni/bin". Matching the directory from the previous search
* *cniConfDir*: "/run/multus/cni/net.d". Matching the directory where the config file from calico is located
* *cniConfFileName*: "10-calico.conflist". Matching the existing file name of the configuration file
* *provider*: default. 

```yaml
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  namespace: istio-system
  updateStrategy:
    inactiveRevisionDeletionGracePeriodSeconds: 30
    type: InPlace
  values:
    pilot:
      cni:
        enabled: true
        provider: default
  version: v1.24.5
```
The configuration set for the resource Istio is as follows:
* *spec.values.pilot.cni.enabled*: true
* *spec.values.pilot.cni.provider*: default

Once both resources are created you can check that the resource pods are running:

```console
$ oc get pods -n istio-cni
NAME                   READY   STATUS    RESTARTS   AGE
istio-cni-node-kfws7   1/1     Running   0          139m
istio-cni-node-kgx6s   1/1     Running   0          139m
istio-cni-node-lz4kk   1/1     Running   0          139m
istio-cni-node-p4j74   1/1     Running   0          139m
istio-cni-node-p592d   1/1     Running   0          139m
istio-cni-node-qzvlh   1/1     Running   0          139m
$ oc get pods -n istio-system
NAME                      READY   STATUS    RESTARTS   AGE
istiod-75fb5f5bc5-w2m2j   1/1     Running   0          134m
```

2. Install bookinfo application:

```console
$ oc create namespace bookinfo
namespace/bookinfo created
$ oc label namespace bookinfo istio-injection=enabled
namespace/bookinfo labeled
$ oc apply -n bookinfo -f https://raw.githubusercontent.com/istio/istio/release-1.24/samples/bookinfo/platform/kube/bookinfo.yaml
```

3. Validate bookinfo application:

```console
$ oc get pods -n bookinfo
NAME                             READY   STATUS    RESTARTS   AGE
details-v1-696f7cfbcb-xhctt      2/2     Running   0          132m
productpage-v1-94cdccd8f-qpqtb   2/2     Running   0          132m
ratings-v1-649c95cf4d-wclnb      2/2     Running   0          132m
reviews-v1-6469d4f55b-pg6pn      2/2     Running   0          132m
reviews-v2-56b87cb468-rwzks      2/2     Running   0          132m
reviews-v3-7bdb75f6fb-wn5f5      2/2     Running   0          132m
```

4. Validate istio-cni pod logs:

```console
2025-05-21T14:25:10.778858Z info cni-plugin ============= End iptables configuration for details-v1-696f7cfbcb-xhctt =============
2025-05-21T14:25:12.832700Z info cni-plugin ============= Start iptables configuration for reviews-v2-56b87cb468-rwzks =============
2025-05-21T14:25:12.841418Z info cni-plugin Istio iptables environment:
...
```
This confirms that the Istio CNI plugin was invoked and started its setup process for this specific application pods.

5. Validate we can reach the bookinfo application:

You can use the following command to validate that you can reach the bookinfo application from inside the cluster (for example, from the productpage pod):
```console
# curl details.bookinfo:9080/details/1
{"id":1,"author":"William Shakespeare","year":1595,"type":"paperback","pages":200,"publisher":"PublisherA","language":"English","ISBN-10":"1234567890","ISBN-13":"123-1234567890"}
```

6. Accesing to the application from outside the cluster:

To access the bookinfo application from outside the cluster, you can follow the upstream specific [documentation](https://istio.io/latest/docs/examples/bookinfo/#:~:text=Create%20a%20gateway%20for%20the%20Bookinfo%20application%3A). In this documentation you will find the steps to create a gateway and a virtual service to expose the bookinfo application or use Gateway API to expose the application. 

Note: the e2e testing framework can be run also for custom configuration but to achieve this you will need to follow this [documentation](https://github.com/istio-ecosystem/sail-operator/tree/main/tests/e2e#running-with-specific-configuration-for-the-istio-and-istiocni-resource).

## Chained CNI vs. Standalone CNI

The chained parameter in the Istio CNI configuration dictates how the Istio CNI plugin integrates with other CNI plugins.

* chained: false (Standalone CNI)
When chained is false (the default), the Istio CNI plugin operates independently for Istio's traffic redirection. It applies iptables rules, but a primary CNI (e.g., Calico) handles core network setup (IP addressing, routing). This mode typically relies on Multus CNI to orchestrate the execution of both the primary CNI and the Istio CNI during pod network setup. The istio-init container then validates that the iptables rules were successfully applied by the CNI plugin.

* chained: true (Chained CNI)
When chained is true, the Istio CNI plugin is integrated directly into the primary CNI's configuration. After the primary CNI completes its setup, it explicitly chains to the Istio CNI plugin to apply the iptables rules for traffic redirection. This requires modifying the primary CNI's configuration file to include the Istio CNI. The istio-init container's role remains validation.