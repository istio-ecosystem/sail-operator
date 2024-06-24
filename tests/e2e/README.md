# Sail-operator end-to-end test

This end-to-end test suite utilizes Ginkgo, a testing framework known for its expressive specs (reference: https://onsi.github.io/ginkgo/). The setup for the test run is similar to the upstream Istio integration tests:
* In the case of kind execution, it relies on the upstream script [`kind_provisioner.sh`](https://github.com/istio-ecosystem/sail-operator/blob/main/common/scripts/kind_provisioner.sh) and [`integ-suite-kind.sh`](https://github.com/istio-ecosystem/sail-operator/blob/main/tests/e2e/integ-suite-kind.sh), which are copied from the `github.com/istio/common-files` repository to set up the kind cluster used for the test.
* In the case of OCP execution, it relies on the `integ-suite-ocp.sh` and `common-operator-integ-suite.sh` scripts to setup the OCP cluster to be ready for the test.

## Table of Contents

1. [Overview](#overview)
1. [Writing Tests](#writing-tests)
    1. [Adding a Test Suite](#adding-a-test-suite)
    1. [Sub-Tests](#sub-tests)
    1. [Best practices](#best-practices)
1. [Running Tests](#running-tests)
    1. [Pre-requisites](#Pre-requisites)
    1. [How to Run the test](#How-to-Run-the-test)
    1. [Running the test locally](#Running-the-test-locally)
    1. [Settings for end-to-end test execution](#Settings-for-end-to-end-test-execution)    
    1. [Get test definitions for the end-to-end test](#Get-test-definitions-for-the-end-to-end-test)
1. [Contributing](#contributing)

## Overview
The goal of the framework is to make it as easy as possible to author and run tests. In its simplest
case, just typing `make test.e2e.kind` will run all the tests in the `tests/e2e` directory.

This framework is designed to be flexible and extensible. It is easy to add new test suites and new tests. The idea is to be able to simulate what a real user scenario looks like when using the operator.

### Writing Tests
As was mentioned before, the test suite is based on Ginkgo. The tests are written in a BDD style, which makes them easy to read and write. The test suite is organized in a hierarchical way, with the following structure:

* Describe
    * Context
        * When
            * It
            * Specify
            * BeforeEach
            * AfterEach
        * By

Please refer to the [Ginkgo documentation](https://onsi.github.io/ginkgo/) for more information about the structure of the test suite.

### Adding a Test Suite
To add a new test suite, create a new folder in the `tests/e2e` directory if the test is not related to our current coverage. The file should have the following structure:

* `tests/e2e/operator/operator_test.go`
```go
package operator

import (
    . "github.com/onsi/ginkgo"
)

var _ = Describe("Operator", func() {
    Context("installation", func() {
        BeforeAll(func() {
            // Test setup code here
        })
        When("installed via helm install", func() {
            It("starts successfully", func() {
                // Test code here
            })
            It("deploys all the CRDs", func() {
                // Test code here
            })
        })
        AfterAll(func() {
            // Test teardown code here
        })
    })
})
``` 

* `tests/e2e/operator/operator_suite_test.go`
```go
package operator

import (
    . "github.com/onsi/ginkgo"
    . "github.com/onsi/gomega"
    "testing"
)

func TestOperator(t *testing.T) {
    RegisterFailHandler(Fail)
    // Add here specific setup code for the test suite
    RunSpecs(t, "Operator Suite")
}
```

### Sub-Tests
The test suite can have multiple levels of sub-tests. The `Describe` block is used to group tests together. The `Context` block is used to group tests that share the same context. The `When` block is used to group tests that share the same action. The `It` block is used to define the test itself. So, the test suite can have multiple levels of sub-tests.

For example:
```go
var _ = Describe("Operator", func() {
    Context("installation", func() {
        When("installed via helm install", func() {
            It("starts successfully", func() {
                // Test code here
            })
            It("deploys all the CRDs", func() {
                // Test code here
            })
        })
    })
})
```

### Best practices
* Use the `Context` block to group tests that share the same context. Example:
```go
var _ = Describe("Operator", func() {
    Context("installation", func() {
        When("installed via helm install", func() {
            It("starts successfully", func() {
                // Test code here
            })
            It("deploys all the CRDs", func() {
                // Test code here
            })
        })

        When("installed via olm", func() {
            It("starts successfully", func() {
                // Test code here
            })
            It("deploys all the CRDs", func() {
                // Test code here
            })
        })
    })
})
``` 
* Use the `Describe` block to group tests together. Remember that the `Describe` block is the top-level block in the test suite.
* Use the `When` block to group tests that share the same action. For example:
```go
var _ = Describe("Operator", func() {
    Context("installation", func() {
        When("installed via helm install", func() {
            It("starts successfully", func() {
                // Test code here
            })
            It("deploys all the CRDs", func() {
                // Test code here
            })
            It("the confirguration is the expected", func() {
                // Test code here
            })
        })
    })
})
```
* Use the `BeforeAll` and `AfterAll` blocks to set up and tear down the test Suite.
* Use the `BeforeEach` and `AfterEach` blocks to set up and tear down the test environment. This is useful when you need to set up the environment before each test.
* Use the `It` block to define the test itself. This is where the test code should be placed.
* Use the `Specify` block to define the test itself. Remember that `Specify` is an alias for `It` and can be used interchangeably. Use them according to the context of the test.
* Use `Eventually` to wait for a condition to be met. This is useful when you need to wait for a condition to be met before running the test. Remember that each `It` block should have at least one assertion.
* Use `Expect` to make direct assertions.
* Use `Success` helper to print Success message in the test output.
* Use `kubectl` and `helm` utils to make all the necessary operations in the test that are going to be done by a user. This means that the test should simulate the user behavior when using the operator.
* Use `client` to interact with the Kubernetes API. This is useful when you need to interact with the Kubernetes API directly and not through `kubectl`. This needs to be used to make all the assertions if is possible. We use the client to make the assertions over `kubectl` because it is more reliable and faster, it will not need any complex parsing of the output.

## Running the test
The end-to-end test can be run in two different environments: OCP (OpenShift Container Platform) and KinD (Kubernetes in Docker).

### Pre-requisites

* To perform OCP end-to-end testing, it is essential to have a functional OCP (OpenShift Container Platform) cluster already running. However, when testing against a KinD (Kubernetes in Docker) environment, the KinD cluster will be automatically configured using the provided script.
* The `kubectl` command-line tool is required to interact with the KinD cluster.
* The `helm` command-line tool is required to install the operator using Helm. The Helm chart is used to install the operator.

Specifically for OCP:
* The `oc` command-line tool is required to interact with the OCP cluster.
* Running on OCP cluster requires to being logged in with the `oc` command-line tool.

### How to Run the test

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

### Running the test locally

To run the end-to-end tests without a container, use the following command:

```
$ make BUILD_WITH_CONTAINER=0 test.e2e.kind
```
or
```
$ make BUILD_WITH_CONTAINER=0 test.e2e.ocp
```

### Settings for end-to-end test execution

The following environment variables define the behavior of the test run:

* SKIP_BUILD=false - If set to true, the test will skip the build process and an existing operator image will be used to deploy the operator and run the test. The operator image that is going to be used is defined by the `IMAGE` variable.
* IMAGE=quay.io/maistra-dev/sail-operator:latest - The operator image to be used to deploy the operator and run the test. This is useful when you want to test a specific operator image.
* SKIP_DEPLOY=false - If set to true, the test will skip the deployment of the operator. This is useful when the operator is already deployed in the cluster and you want to run the test only.
* OCP=false - If set to true, the test will be configured to run on an OCP cluster and use the `oc` command to interact with it. If set to false, the test will run in a KinD cluster and use `kubectl`.
* NAMESPACE=sail-operator - The namespace where the operator will be deployed and the test will run.
* CONTROL_PLANE_NS=istio-system - The namespace where the control plane will be deployed.
* DEPLOYMENT_NAME=sail-operator - The name of the operator deployment.
* EXPECTED_REGISTRY=`^docker\.io|^gcr\.io` - Which image registry should the operand images come from. Useful for downstream tests.

### Get test definitions for the end-to-end test

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

## Contributing

Please refer to the [CONTRIBUTING.md](../../CONTRIBUTING.md) file for information about how to get involved. We welcome issues, questions, and pull requests.