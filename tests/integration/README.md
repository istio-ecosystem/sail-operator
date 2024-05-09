# Sail-operator Integration Test

This integration test suite utilizes Ginkgo, a testing framework known for its expressive specs (reference: https://onsi.github.io/ginkgo/). The test suites do not need a running Kubernetes cluster, as it uses the `envtest` package to create a fake Kubernetes client.

## Running the tests

To run the tests, execute the following command:

```shell
make test.integration
```

## Writing tests

To write a new test, create a new file in the `tests/integration` directory. The file should have the following structure:

```go
package integration

import (
    . "github.com/onsi/ginkgo"
    . "github.com/onsi/gomega"
)

var _ = Describe("My test suite", func() {
    Context("My test case", func() {
        It("should do something", func() {
            // Your test code here
        })
    })
})
```

For more information on how to write tests using Ginkgo, refer to the official documentation: https://onsi.github.io/ginkgo/.

## Suite_test.go

The `suite_test.go` file is used to run the test suite. It should not contain any test cases. This file contains the setup and teardown logic for the test suite.


## Best practices

* Keep the test cases small and focused on a single functionality.
* Use the `BeforeEach` and `AfterEach` hooks to set up and tear down the test environment.
* Use the `Describe` and `Context` blocks to group related test cases. For example, you can group test cases that test the same functionality or the same method.
```go
var _ = Describe("resource is created", func() {
    Context("status changes", func() {
		When("Resource becomes ready", func() {
			BeforeAll(func() {
				// Your test code here
			})

			It("marks the resource as ready", func() {
				// Your test code here
			})
		})

		When("Resource becomes not ready", func() {
			BeforeAll(func() {
				// Your test code here
			})

			It("marks the resource as not ready", func() {
				// Your test code here
			})
		})
	})
})
``` 

* Use the `It` block to write the test cases.
* Use the `By` block to provide additional context to the test case.
* Use the `Expect` block to make assertions.
* Use the `Eventually` block to handle asynchronous operations.
* Keep the test cases independent of each other and simple to understand. Remember that the test cases should be self-contained and should not rely on the state of other test cases.
* Use the `Gomega` matchers to make assertions. For more information, refer to the official documentation: https://onsi.github.io/gomega/. And remember that each test case should have at least one assertion.
