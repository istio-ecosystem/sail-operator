# Sail Operator end-to-end test

This end-to-end test suite utilizes Ginkgo, a testing framework known for its expressive specs (reference: https://onsi.github.io/ginkgo/). The setup for the test run is similar to the upstream Istio integration tests:
* In the case of kind execution, it relies on the upstream script [`kind_provisioner.sh`](https://github.com/istio-ecosystem/sail-operator/blob/main/common/scripts/kind_provisioner.sh) and [`integ-suite-kind.sh`](https://github.com/istio-ecosystem/sail-operator/blob/main/tests/e2e/integ-suite-kind.sh), which are copied from the `github.com/istio/common-files` repository to set up the kind cluster used for the test.
* In the case of OCP execution, it relies on the `integ-suite-ocp.sh` and `common-operator-integ-suite.sh` scripts to set up the OCP cluster to be ready for the test.

## Table of Contents

1. [Overview](#overview)
1. [Writing Tests](#writing-tests)
    1. [Adding a Test Suite](#adding-a-test-suite)
    1. [Sub-Tests](#sub-tests)
    1. [Best practices](#best-practices)
1. [Running Tests](#running-the-tests)
    1. [Pre-requisites](#pre-requisites)
    1. [How to Run the test](#how-to-run-the-test)
    1. [Running the test locally](#running-the-test-locally)
    1. [Settings for end-to-end test execution](#settings-for-end-to-end-test-execution)
    1. [Customizing the test run](#customizing-the-test-run)    
    1. [Get test definitions for the end-to-end test](#get-test-definitions-for-the-end-to-end-test)
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

var _ = Describe("Operator", Label("labels-for-the-test"), Ordered, func() {
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
Note: The `Label` function is used to label the test. This is useful when you want to run a specific test or a group of tests. The label can be used to filter the tests when running them. Ordered is used to run the tests in the order they are defined. This is useful when you want to run the tests in a specific order.


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
The test suite can have multiple levels of subtests. The `Describe` block is used to group tests together. The `Context` block is used to group tests that share the same context. The `When` block is used to group tests that share the same action. The `It` block is used to define the test itself. So, the test suite can have multiple levels of subtests.

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
* Add Labels to the tests. This is useful when you want to run a specific test or a group of tests. The label can be used to filter the tests when running them. For example:
```go
var _ = Describe("Ambient configuration ", Label("smoke", "ambient"), Ordered, func() {
    Context("installation", func() {
        // Test code here...
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
* Use `cleaner` to clean up any resources your tests create after they finish. The cleaner records a "snapshot" of the cluster, and removes any resources that weren't recorded. The best way to use it is by adding call in `BeforeAll` and `AfterAll` blocks. For example:
```go
import "github.com/istio-ecosystem/sail-operator/tests/e2e/util/cleaner"

var _ = Describe("Testing with cleanup", Ordered, func() {
    clr := cleaner.New(cl)

    BeforeAll(func(ctx SpecContext) {
        clr.Record(ctx)
        // Any additional set up goes here
    })

    // Tests go here

    AfterAll(func(ctx SpecContext) {
        // Any finalizing logic goes here
        clr.Cleanup(ctx)
    })
})
```
    * You can use multiple cleaners, each with its own state. This is useful if the test does some global set up, e.g. sets up the operator, and then specific tests create further resources which you want cleaned.
    * To clean resources without waiting, and waiting for them later, use `CleanupNoWait` followed by `WaitForDeletion`. This is particularly useful when working with more than one cluster.

## Running the tests
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
make test.e2e.ocp
```

* To run the end-to-end tests in KinD cluster, use the following command:
```
make test.e2e.kind
```

Both targets will run setup first by using `integ-suite-ocp.sh` and `integ-suite-kind.sh` scripts respectively, and then run the end-to-end tests using the `common-operator-integ-suite` script setting different flags for OCP and KinD.

Note: By default, the test runs inside a container because the env var `BUILD_WITH_CONTAINER` default value is 1. Take into account that to be able to run the end-to-end tests in a container, you need to have `docker` or `podman` installed in your machine. To select the container cli you will also need to set the `CONTAINER_CLI` env var to `docker` or `podman` in the `make` command, the default value is `docker`.

#### How to Run a specific subset of tests
To run a specific subset of tests, you can use the `GINKGO_FLAGS` environment variable. For example, to run only the `smoke` tests, you can use the following command:

```
GINKGO_FLAGS="-v --label-filter=smoke" make test.e2e.kind
```
Note: `-v` is to add verbosity to the output. The `--label-filter` flag is used to filter the tests by label. You can use multiple labels separated by commas. Please take a look at the topic [Settings for end to end test execution](#settings-for-end-to-end-test-execution) to see how you can set more customizations for the test run.

### Running the test locally

To run the end-to-end tests without a container, use the following command:

```
make BUILD_WITH_CONTAINER=0 test.e2e.kind
```
or
```
make BUILD_WITH_CONTAINER=0 test.e2e.ocp
```

Note: if you are running the test against a cluster that has a different architecture than the one you are running the test, you will need to set the `TARGET_ARCH` environment variable to the architecture of the cluster. For example, if you are running the test against an ARM64 cluster, you can use the following command:

```
TARGET_ARCH=arm64 make test.e2e.ocp
```

### Settings for end-to-end test execution

The following environment variables define the behavior of the test run:

* SKIP_BUILD=false - If set to true, the test will skip the build process and an existing operator image will be used to deploy the operator and run the test. The operator image that is going to be used is defined by the `IMAGE` variable.
* IMAGE=quay.io/sail-dev/sail-operator:latest - The operator image to be used to deploy the operator and run the test. This is useful when you want to test a specific operator image.
* SKIP_DEPLOY=false - If set to true, the test will skip the deployment of the operator. This is useful when the operator is already deployed in the cluster, and you want to run the test only.
* OCP=false - If set to true, the test will be configured to run on an OCP cluster and use the `oc` command to interact with it. If set to false, the test will run in a KinD cluster and use `kubectl`.
* OLM=false - If set to true, the test will use the OLM (Operator Lifecycle Manager) to install the operator. If set to false, the test will use Helm to install the operator.
* GINKGO_FLAGS - The flags to be passed to the Ginkgo test runner. This is useful when you want to set specific ginkgo flags to be used during the test run. For example, to run only the `smoke` tests, you can use set to `GINKGO_FLAGS="--label-filter=smoke"`. Also, you can set to `GINKGO_FLAGS="-v"` to add verbosity to the output. To understand more about the ginkgo flags, please refer to the [Ginkgo documentation](https://onsi.github.io/ginkgo/).
* NAMESPACE=sail-operator - The namespace where the operator will be deployed and the test will run.
* CONTROL_PLANE_NS=istio-system - The namespace where the control plane will be deployed.
* DEPLOYMENT_NAME=sail-operator - The name of the operator deployment.
* EXPECTED_REGISTRY=`^docker\.io|^gcr\.io` - Which image registry should the operand images come from. Useful for downstream tests.
* KEEP_ON_FAILURE - If set to true, when using a local KIND cluster, don't clean it up when the test fails. This allows to debug the failure.

### Customizing the test run

The test run can be customized by setting the following environment variables:

To change all the sample files used in the test, you can use the following environment variable:
* `SAMPLES_PATH=<path-to-kustomize-sample-base-folder>`. We use kustomize to patch the upstream sample yaml files to use images located in the `quay.io/sail-dev` registry. This is useful when you want to use your own sample files or when you want to use a different version of the sample files. The path should point to the folder where the kustomize files are located. For example, if you have your own sample files in the `tests/e2e/samples/custom` folder, you can set the environment variable as follows:
```
CUSTOM_SAMPLES_PATH=tests/e2e/samples/custom
```

Note: when setting this environment variable, make sure that the folder contains the kustomize files with the same structure as the upstream sample files. This means that the folder should contain:
- helloworld/kustomize.yaml
- sleep/kustomize.yaml
- httpbin/kustomize.yaml
- tcp-echo-dual-stack/kustomize.yaml
- tcp-echo-dual-stack-ipv6/kustomize.yaml
- tcp-echo-dual-stack-ipv4/kustomize.yaml

note that each of the folders have their own kustomize file that will be used to patch the sample files. For example, the `helloworld` folder should contain a `kustomization.yaml` file with a content similar to this:
```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - <path-or-url-to-the-source-of-your-sample-file> # For example: https://raw.githubusercontent.com/istio/istio/master/samples/helloworld/helloworld.yaml
images:
    - name: docker.io/istio/examples-helloworld-v1 # This is the image that will be patched
        newName: quay.io/sail-dev/examples-helloworld-v1 # This is the image that will be used in the test
```

You can also only patch one or more appa and continue using the others samples in the rest of the test. To do this instead of set the env var `CUSTOM_SAMPLES_PATH`, you can set the following environment variables:
- `HTTPBIN_KUSTOMIZE_PATH`
- `HELLOWORLD_KUSTOMIZE_PATH`
- `SLEEP_KUSTOMIZE_PATH`
- `TCP_ECHO_DUAL_STACK_KUSTOMIZE_PATH`
- `TCP_ECHO_IPV4_KUSTOMIZE_PATH`
- `TCP_ECHO_IPV6_KUSTOMIZE_PATH`

Each of the env var will be pointing to the kustomize directory that will be used to patch the sample files.

Note: the default behaviour is to use the kustomize files located un the folder `tests/e2e/samples/`. These kustomize files are used to patch the upstream examples from the master branch of the Istio repository to use images located in the `quay.io/sail-dev` registry.

### Using the e2e framework to test your cluster configuration
The e2e framework can be used to test your cluster configuration. The framework is designed to be flexible and extensible. It is easy to add new test suites and new tests. The idea is to be able to simulate what a real user scenario looks like when using the operator.

To do this, we have a set of test cases under a test group called `smoke`. This test group will run a set of tests that are designed to test the basic functionality of the operator. The tests in this group are designed to be run against a cluster that is already configured and running. The tests will not modify the cluster configuration or the operator configuration. The tests will only check if the operator is working as expected.

Pre-requisites:
* The operator is already installed and running in the cluster.

To run the test group, you can use the following command:

* Run the following command to run the smoke tests:
For running on kind:
```
SKIP_BUILD=true SKIP_DEPLOY=true GINKGO_FLAGS="-v --label-filter=smoke" make test.e2e.kind
```

For running on OCP:
```
SKIP_BUILD=true SKIP_DEPLOY=true GINKGO_FLAGS="-v --label-filter=smoke" make test.e2e.ocp
```

#### Running with specific configuration for the Istio and IstioCNI resource
There might be situations where you want to run tests with a specific configuration for the Istio and IstioCNI resource to match some cluster specific needs. For this, you can modify the `pkg/istiovalues/vendor_default.yaml` file to default `spec.values` for the Istio and IstioCNI resources. For more information and an example go to the [file](../../pkg/istiovalues/vendor_defaults.yaml)

#### Running the testing framework against specific Istio versions
By default, the test framework will run the tests against all the latest patch minor version available for the operator. This is useful when you want to test the operator against all the latest patch minor versions available. The test framework will automatically detect the latest patch minor version available for the operator and run the tests against it by reading the versions located in the `pkg/istioversion/versions.yaml` file.

To avoid this and run the tests against a specific Istio versions, you can create your own `versions.yaml` file and set the `VERSIONS_YAML_FILE` environment variable to point to it. The file should have the following structure:
```yaml
versions:
  - name: v1.26-latest
    ref: v1.26.0
  - name: v1.26.0
    version: 1.26.0
    repo: https://github.com/istio/istio
    commit: 1.26.0
    charts:
      - https://istio-release.storage.googleapis.com/charts/base-1.26.0.tgz
      - https://istio-release.storage.googleapis.com/charts/istiod-1.26.0.tgz
      - https://istio-release.storage.googleapis.com/charts/gateway-1.26.0.tgz
      - https://istio-release.storage.googleapis.com/charts/cni-1.26.0.tgz
      - https://istio-release.storage.googleapis.com/charts/ztunnel-1.26.0.tgz
  - name: v1.25-latest
    ref: v1.25.3
  - name: v1.25.3
    version: 1.25.3
    repo: https://github.com/istio/istio
    commit: 1.25.3
    charts:
      - https://istio-release.storage.googleapis.com/charts/base-1.25.3.tgz
      - https://istio-release.storage.googleapis.com/charts/istiod-1.25.3.tgz
      - https://istio-release.storage.googleapis.com/charts/gateway-1.25.3.tgz
      - https://istio-release.storage.googleapis.com/charts/cni-1.25.3.tgz
      - https://istio-release.storage.googleapis.com/charts/ztunnel-1.25.3.tgz
```
*Important*: avoid adding in the custom file versions that are not available in the `pkg/istioversion/versions.yaml` file. The test framework will not be able to run the tests because the operator does not contains the charts for those versions.

* To run the test framework against a specific Istio version, you can use the following command:
```
VERSIONS_YAML_FILE=custom_versions.yaml SKIP_BUILD=true SKIP_DEPLOY=true GINKGO_FLAGS="-v --label-filter=smoke" make test.e2e.kind
```
Note: The `custom_versions.yaml` file must be placed in the `pkg/istioversion` directory. The test framework uses this file to run tests against the specific Istio versions it defines.

### Understanding the test output
By default, running the test using the make target will generate a report.xml file in the project's root directory. This file contains the test results in JUnit format, for example:
```xml
<?xml version="1.0" encoding="UTF-8"?>
  <testsuites tests="154" disabled="79" errors="0" failures="0" time="588.636386015">
      <testsuite name="Ambient Test Suite" package="/work/tests/e2e/ambient" tests="60" disabled="0" skipped="60" errors="0" failures="0" time="0.020193112" timestamp="2025-05-22T13:07:53">
          <properties>
```
As you can see, the test results are grouped by test suite. The `tests` attribute indicates the number of tests that were run, the `disabled` attribute indicates the number of tests that were skipped, and the `errors` and `failures` attributes indicate the number of tests that failed or had errors. The `time` attribute indicates the total time taken to run the tests.

Also, in the terminal you will be able to see the test results in a human readable format. The test results will be printed to the console with the following format:
```
Ran 82 of 82 Specs in 224.026 seconds
SUCCESS! -- 82 Passed | 0 Failed | 0 Pending | 0 Skipped
PASS

Ginkgo ran 1 suite in 3m46.401610849s
Test Suite Passed
```

In case of failure, the test results will be printed to the console with the following format:
```
Ran 82 of 82 Specs in 224.026 seconds
FAIL! -- 81 Passed | 1 Failed | 0 Pending | 0 Skipped
Ginkgo ran 1 suite in 3m46.401610849s
Test Suite Failed
```

### Get test definitions for the end-to-end test

The end-to-end test suite is defined in the `tests/e2e/operator` directory. If you want to check the test definition without running the test, you can use the following make target:

```
make test.e2e.describe
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
