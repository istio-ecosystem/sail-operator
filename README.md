# Sail Operator

test

This project is an operator that can be used to manage the installation of an [Istio](https://istio.io) control plane.

## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Deploying the operator
Deploy the operator to the cluster:
```sh
make deploy
```

Alternatively, you can deploy the operator using OLM:

```sh
make deploy-olm
```

Make sure that the `HUB` and `TAG` environment variables point to your container image repository and that the repository is publicly accessible.

### Deploying the Istio Control Plane
Create an instance of the Istio resource to install the Istio Control Plane. 

Use the `istio-sample-kubernetes.yaml` file on vanilla Kubernetes:

```sh
kubectl apply -f chart/samples/istio-sample-kubernetes.yaml
```

Use the `istio-sample-openshift.yaml` file on OpenShift:

```sh
kubectl apply -f chart/samples/istio-sample-openshift.yaml
```

### Deploying the Istio CNI plugin
On OpenShift, you must also deploy the Istio CNI plugin by creating an instance of the IstioCNI resource:

```sh
kubectl apply -f chart/samples/istiocni-sample.yaml
```

### Undeploying the operator
Undeploy the operator from the cluster:

```sh
make undeploy
```

## Contributing
We use GitHub to track all of our bugs and feature requests. Please create a GitHub issue for any new bug or feature request.

### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/),
which provide a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster.

### Test It Out
1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

### Writing Tests
Please try to keep business logic in separate packages that can be independently tested wherever possible, especially if you can avoid the usage of Kubernetes clients. It greatly simplifies testing if we don't need to use envtest everywhere.

E2E and integration tests should use the ginkgo-style BDD testing method, an example can be found in [`tests/integration/api/istio_test.go`](https://github.com/istio-ecosystem/sail-operator/blob/main/tests/integration/api/istio_test.go) for the test code and suite setup in [`tests/integration/api/suite_test.go`](https://github.com/istio-ecosystem/sail-operator/blob/main/tests/integration/api/suite_test.go). Unit tests should use standard golang xUnit-style tests (see [`pkg/kube/finalizers_test.go`](https://github.com/maistra/istio-operator/blob/maistra-3.0/pkg/kube/finalizers_test.go) for an example).

### Integration Tests
Please check the specific instructions for the integration tests in the [integration](https://github.com/istio-ecosystem/sail-operator/blob/main/tests/integration/README.md) directory.

To run the integration tests, you can use the following command:

```sh
make test.integration
```

### End-to-End Tests
Please check the specific instructions for the end-to-end tests in the [e2e](https://github.com/istio-ecosystem/sail-operator/blob/main/tests/e2e/README.md) directory.

To run the end-to-end tests, you can use the following command:

```sh
make test.e2e.kind
```

or

```sh
make test.e2e.ocp
```
