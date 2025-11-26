# Sail Operator Testing Framework Domain Knowledge

This document provides AI agents with detailed knowledge about the Sail Operator's testing framework, methodologies, and best practices.

## Testing Architecture

The Sail Operator uses a multi-layered testing approach:

### Unit Tests
- **Framework**: Standard Go testing (`testing` package)
- **Location**: Alongside source code files (`*_test.go`)
- **Purpose**: Test individual functions and business logic
- **Execution**: `make test`

### Integration Tests
- **Framework**: Ginkgo + Gomega + envtest
- **Location**: `tests/integration/`
- **Purpose**: Test controller behavior against fake Kubernetes API
- **Execution**: `make test.integration`

### End-to-End Tests
- **Framework**: Ginkgo + Gomega + real clusters
- **Location**: `tests/e2e/`
- **Purpose**: Test complete operator functionality on real clusters
- **Execution**: `make test.e2e.kind` or `make test.e2e.ocp`

## Integration Testing Framework

### Setup and Configuration
Integration tests use `envtest` to create a fake Kubernetes control plane:

```go
// Example suite setup
var _ = BeforeSuite(func() {
    testEnv = &envtest.Environment{
        CRDDirectoryPaths: []string{
            filepath.Join("..", "..", "config", "crd", "bases"),
        },
    }

    cfg, err := testEnv.Start()
    Expect(err).NotTo(HaveOccurred())

    k8sClient, err = client.New(cfg, client.Options{
        Scheme: scheme.Scheme,
    })
    Expect(err).NotTo(HaveOccurred())
})
```

### Testing Patterns
Integration tests follow Ginkgo BDD patterns:

```go
var _ = Describe("IstioController", func() {
    Context("when creating an Istio resource", func() {
        BeforeEach(func() {
            // Setup test resources
        })

        When("the resource is valid", func() {
            It("should create an IstioRevision", func() {
                // Test assertions
                Eventually(func() error {
                    return k8sClient.Get(ctx, key, &istioRevision)
                }).Should(Succeed())
            })
        })

        AfterEach(func() {
            // Cleanup test resources
        })
    })
})
```

### Key Testing Utilities
- **envtest.Environment** - Fake Kubernetes API server
- **client.Client** - Kubernetes API client for assertions
- **Eventually/Consistently** - Async operation testing
- **BeforeEach/AfterEach** - Test setup/teardown

## End-to-End Testing Framework

### Test Structure
E2E tests are organized by functionality:

```
tests/e2e/
├── ambient/              # Ambient mesh functionality tests
├── controlplane/         # Control plane installation and update tests
├── dualstack/            # Dual-stack networking tests
├── multicluster/         # Multi-cluster scenarios (primary-remote, multi-primary, external control plane)
├── multicontrolplane/    # Multiple control plane tests
├── operator/             # Operator deployment and installation tests
├── samples/              # Sample application tests
├── setup/                # Test setup utilities
└── util/                 # Shared utilities (cleaner, kubectl, helm, etc.)
```

### Cluster Management
E2E tests support multiple cluster types:

#### KIND Clusters
- Automated setup via `integ-suite-kind.sh`
- Uses upstream Istio provisioning scripts
- Configurable with environment variables

#### OpenShift Clusters
- Uses existing OCP cluster
- Setup via `integ-suite-ocp.sh`
- Requires `oc` login and permissions

### Test Execution Environments

#### Containerized Execution (Default)
```bash
# Run in container (default)
make test.e2e.kind

# Specify container runtime
CONTAINER_CLI=podman make test.e2e.kind
```

#### Local Execution
```bash
# Run locally without container
BUILD_WITH_CONTAINER=0 make test.e2e.kind
```

### Test Configuration Variables

#### Core Configuration
- `SKIP_BUILD=false` - Skip operator image build
- `SKIP_DEPLOY=false` - Skip operator deployment
- `IMAGE=quay.io/sail-dev/sail-operator:latest` - Operator image
- `OCP=true` - Use OpenShift cluster
- `OLM=true` - Deploy via OLM instead of Helm

#### Test Behavior
- `GINKGO_FLAGS` - Pass flags to Ginkgo runner
- `NAMESPACE=sail-operator` - Operator namespace
- `CONTROL_PLANE_NS=istio-system` - Istio namespace
- `EXPECTED_REGISTRY=^docker\.io|^gcr\.io` - Expected image registry
- `KEEP_ON_FAILURE=false` - Keep cluster on test failure

#### Custom Samples
- `CUSTOM_SAMPLES_PATH` - Path to custom kustomize samples
- `HTTPBIN_KUSTOMIZE_PATH` - Custom httpbin samples
- `HELLOWORLD_KUSTOMIZE_PATH` - Custom helloworld samples
- `SLEEP_KUSTOMIZE_PATH` - Custom sleep samples

### Test Labels and Filtering

Tests use labels for categorization and selective execution:

```go
var _ = Describe("Ambient configuration", Label("smoke", "ambient"), func() {
    // Test implementation
})
```

Common labels:
- **smoke** - Basic functionality tests
- **ambient** - Ambient mesh specific tests
- **sidecar** - Sidecar injection tests
- **multicluster** - Multi-cluster scenarios
- **upgrade** - Version upgrade tests

### Test Utilities

#### Resource Management
```go
import "github.com/istio-ecosystem/sail-operator/tests/e2e/util/cleaner"

// Automatic resource cleanup
clr := cleaner.New(cl)
BeforeAll(func(ctx SpecContext) {
    clr.Record(ctx)  // Record initial state
})
AfterAll(func(ctx SpecContext) {
    clr.Cleanup(ctx)  // Remove added resources
})
```

#### kubectl and helm Utilities
```go
import (
    "github.com/istio-ecosystem/sail-operator/tests/e2e/util/kubectl"
    "github.com/istio-ecosystem/sail-operator/tests/e2e/util/helm"
)

// Execute kubectl commands
kubectl.Apply(namespace, yamlContent)
kubectl.Delete(namespace, resourceType, resourceName)

// Helm operations
helm.Install(releaseName, chartPath, namespace, values)
helm.Uninstall(releaseName, namespace)
```

#### Client-based Assertions
```go
// Use Kubernetes client for reliable assertions
Eventually(func() error {
    var istio v1.Istio
    return k8sClient.Get(ctx, client.ObjectKey{
        Name: "default",
        Namespace: "istio-system",
    }, &istio)
}).Should(Succeed())

// Check resource status
Expect(istio.Status.State).To(Equal(v1.IstioReady))
```

## Testing Best Practices

### Unit Test Guidelines
1. **Isolation** - Test individual functions without Kubernetes dependencies
2. **Mocking** - Use interfaces and mocks for external dependencies
3. **Table-driven tests** - Use test tables for multiple scenarios
4. **Error testing** - Test both success and failure cases

```go
func TestCalculateRevisionName(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"valid version", "1.25.0", "default-1-25-0"},
        {"version with patch", "1.25.1", "default-1-25-1"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := calculateRevisionName("default", tt.input)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

### Integration Test Guidelines
1. **Real API behavior** - Test controller interactions with Kubernetes API
2. **Async testing** - Use `Eventually`/`Consistently` for async operations
3. **Resource cleanup** - Always clean up created resources
4. **Status testing** - Verify status conditions and state transitions

### E2E Test Guidelines
1. **User scenarios** - Test realistic user workflows
2. **End-to-end validation** - Verify complete functionality
3. **Resource cleanup** - Use cleaner utility for automatic cleanup
4. **Platform coverage** - Test on different Kubernetes distributions

### Common Testing Patterns

#### Testing Controller Reconciliation
```go
It("should reconcile Istio resource", func() {
    // Create resource
    istio := &v1.Istio{
        ObjectMeta: metav1.ObjectMeta{
            Name: "test-istio",
            Namespace: "istio-system",
        },
        Spec: v1.IstioSpec{
            Version: "1.25.0",
        },
    }
    Expect(k8sClient.Create(ctx, istio)).To(Succeed())

    // Verify reconciliation results
    Eventually(func() error {
        var revision v1.IstioRevision
        return k8sClient.Get(ctx, client.ObjectKey{
            Name: "test-istio-1-25-0",
            Namespace: "istio-system",
        }, &revision)
    }).Should(Succeed())
})
```

#### Testing Status Updates
```go
It("should update status conditions", func() {
    Eventually(func() []metav1.Condition {
        var istio v1.Istio
        if err := k8sClient.Get(ctx, key, &istio); err != nil {
            return nil
        }
        return istio.Status.Conditions
    }).Should(ContainElement(MatchFields(IgnoreExtras, Fields{
        "Type":   Equal("Ready"),
        "Status": Equal(metav1.ConditionTrue),
    })))
})
```

#### Testing Error Conditions
```go
When("invalid configuration is provided", func() {
    It("should report error condition", func() {
        // Create resource with invalid config
        Eventually(func() string {
            var istio v1.Istio
            k8sClient.Get(ctx, key, &istio)
            for _, cond := range istio.Status.Conditions {
                if cond.Type == "ReconcileError" {
                    return cond.Status
                }
            }
            return ""
        }).Should(Equal(metav1.ConditionTrue))
    })
})
```

## Test Execution and CI/CD

### Local Development
```bash
# Run all tests
make test test.integration test.e2e.kind

# Run specific test suites
make test.integration GINKGO_FLAGS="--focus=IstioController"
make test.e2e.kind GINKGO_FLAGS="--label-filter=smoke"

# Debug test failures
KEEP_ON_FAILURE=true make test.e2e.kind

# macOS with Podman (see docs/macos/develop-on-macos.adoc)
CONTAINER_CLI=podman TARGET_OS=linux TARGET_ARCH=arm64 make test.e2e.kind
CONTAINER_CLI=podman DOCKER_GID=0 make test.integration
```

### Continuous Integration
Tests run automatically on:
- **Pull requests** - Unit and integration tests
- **Merge to main** - Full test suite including E2E
- **Release branches** - Extended test matrix across platforms

### Test Reporting
- **JUnit XML** - Generated as `report.xml` for CI integration
- **Coverage reports** - Go coverage for unit/integration tests
- **Test artifacts** - Logs and cluster state on failures

## Platform-Specific Testing Considerations

### macOS Development with Podman
For macOS developers using Podman instead of Docker, refer to `docs/macos/develop-on-macos.adoc` for detailed guidance:

#### Common Issues and Solutions
- **Architecture mismatch**: Use `TARGET_ARCH=arm64` for ARM64 Macs
- **UID conflicts**: Set `DOCKER_GID=0` to avoid permission issues
- **Container runtime**: Always specify `CONTAINER_CLI=podman`
- **KIND compatibility**: Use compatible KIND images for Podman

#### Example Commands
```bash
# E2E tests on macOS with Podman
CONTAINER_CLI=podman TARGET_OS=linux TARGET_ARCH=arm64 make test.e2e.kind

# Integration tests with permission fixes
CONTAINER_CLI=podman DOCKER_GID=0 make test.integration

# Avoid container for specific targets
make -f common/Makefile.common.mk update-common
```

### Container Runtime Considerations
The testing framework supports both Docker and Podman:
- **Default**: Docker (`CONTAINER_CLI=docker`)
- **Alternative**: Podman (`CONTAINER_CLI=podman`)
- **Local execution**: `BUILD_WITH_CONTAINER=0` to run tests without containers

### Cluster Architecture
When testing on different architectures:
- **AMD64**: Default for most CI environments
- **ARM64**: Common for Apple Silicon Macs
- **Mixed environments**: Use `TARGET_ARCH` to specify target architecture