# Sail Operator end-to-end test

This end-to-end test suite utilizes Ginkgo, a testing framework known for its expressive specs (reference: https://onsi.github.io/ginkgo/). The setup for the test run is similar to the upstream Istio integration tests:
* In the case of kind execution, it relies on the upstream script `kind_provisioner.sh` and `integ-suite-kind.sh`, which are copied from the `github.com/istio/common-files` repository to set up the kind cluster used for the test.
* In the case of OCP execution, it relies on the `inter-suite-ocp.sh` and `common-operator-integ-suite` scripts to setup the the OCP cluster to be ready for the test.

## Pre-requisites

* To perform OCP end-to-end testing, it is essential to have a functional OCP (OpenShift Container Platform) cluster already running. However, when testing against a KinD (Kubernetes in Docker) environment, the KinD cluster will be automatically configured using the provided script.

## How to Run the test

* To run the end-to-end tests in OCP cluster, use the following command:
```
$ make test.e2e.ocp
```

* To run the end-to-end tests in KinD cluster, use the following command:
```
$ make test.e2e.kind
```

Both targets will run setup first by using `integ-suite-ocp.sh` and `integ-suite-kind.sh` scripts respectively, and then run the end-to-end tests using the `common-operator-integ-suite` script setting different flags for OCP and KinD.

Note: By default, the test runs inside a container because the env var `BUILD_WITH_CONTAINER` default value is 1. Take into account that to be able to run the end-to-end tests in a container, you need to have `docker` or `podman` installed in your machine. To select the container cli you will also need to set the `CONTAINER_CLI` env var to `docker` or `podman` in the `make` command, the default value is `docker`.

## Running the test locally

To run the end-to-end tests without a container, use the following command:

```
$ make BUILD_WITH_CONTAINER=0 test.e2e.kind
```
or
```
$ make BUILD_WITH_CONTAINER=0 test.2e2.ocp
```

## Settings for end-to-end test execution

The following environment variables define the behavior of the test run:

* SKIP_BUILD=false - If set to true, the test will skip the build process and an existing operator image will be used to deploy the operator and run the test. The operator image that is going to be used is defined by the `IMAGE` variable.
* IMAGE=quay.io/maistra-dev/sail-operator:latest - The operator image to be used to deploy the operator and run the test. This is useful when you want to test a specific operator image.
* SKIP_DEPLOY=false - If set to true, the test will skip the deployment of the operator. This is useful when the operator is already deployed in the cluster and you want to run the test only.
* OCP=false - If set to true, the test will be configured to run on an OCP cluster and use the `oc` command to interact with it. If set to false, the test will run in a KinD cluster and use `kubectl`.
* NAMESPACE=sail-operator - The namespace where the operator will be deployed and the test will run.
* CONTROL_PLANE_NS=istio-system - The namespace where the control plane will be deployed.
* DEPLOYMENT_NAME=sail-operator - The name of the operator deployment.

## Get test definitions for the end-to-end test

The end-to-end test suite is defined in the `tests/e2e/operator` directory. If you want to check the test definition without running the test, you can use the following make target:

```
$ make test.e2e.describe
```

When you run this target, the test definitions will be printed to the console with format `indent`. For example:
    
```
Name,Text,Start,End,Spec,Focused,Pending,Labels
Describe,Operator,882,7688,false,false,false,""
    Describe,installation,2039,3282,false,false,false,""
        When,installed via helm install,2327,3278,false,false,false,""
            It,starts successfully,2608,3042,true,false,false,""
            It,deploys all the CRDs,3047,3273,true,false,false,""
    Describe,installation and unistallation of the istio resource,3285,7473,false,false,false,""
        Context,undefined,3499,7042,false,false,false,""
            When,the resource is created,3713,6569,false,false,false,""
                Specify,successfully,3759,3973,true,false,false,""
                It,updates the Istio resource status to Reconcilied and Ready,3980,4709,true,false,false,""
                It,deploys istiod,4716,4964,true,false,false,""
                It,deploys correct istiod image tag according to the version in the Istio CR,4971,5392,true,false,false,""
                It,deploys the CNI DaemonSet when running on OpenShift,5399,6220,true,false,false,""
                It,doesn't continuously reconcile the istio resource,6227,6562,true,false,false,""
            When,the Istio CR is deleted,6575,7036,false,false,false,""
                BeforeEach,,6621,6765,false,false,false,""
                It,removes everything from the namespace,6772,7029,true,false,false,""
        By,Cleaning up the namespace,7071,7102,false,false,false,""
    By,Cleaning up the operator deployment,7496,7537,false,false,false,""
```

This can be used to show the actual coverage of the test suite.