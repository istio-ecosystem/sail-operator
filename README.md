# Sail Operator

The Sail Operator manages the lifecycle of your [Istio](https://istio.io) control plane. It provides custom resources for you to deploy and manage your control plane components.

## User Documentation
This document aims to provide an overview of the project and some information for contributors. For information on how to use the operator, take a look at the [User Documentation](docs/README.md).

## Table of Contents

- [How it works](#how-it-works)
- [Getting Started](#getting-started)
    - [Deploying the operator](#deploying-the-operator)
    - [Deploying the Istio Control Plane](#deploying-the-istio-control-plane)
    - [Undeploying the operator](#undeploying-the-operator)
- [Development](#undeploying-the-operator)
    - [Repository Setup](#repository-setup)
    - [Test It Out](#test-it-out)
    - [Modifying the API definitions](#modifying-the-api-definitions)
    - [Writing Tests](#writing-tests)
    - [Integration Tests](#integration-tests)
    - [End-to-End Tests](#end-to-end-tests)
- [Community Support and Contributing](#community-support-and-contributing)
- [Sail Enhancement Proposal](#sail-enhancement-proposal)
- [Issue management](#issue-management)

## How it works

You manage your control plane through an `Istio` resource.

```yaml
apiVersion: sailoperator.io/v1alpha1
kind: Istio
metadata:
  name: example
spec:
  namespace: istio-system
  version: v1.22.0
```

When you create an `Istio` resource, the sail operator then creates an `IstioRevision` that represents a control plane deployment.

```yaml
apiVersion: sailoperator.io/v1alpha1
kind: IstioRevision
metadata:
  name: example
  ...
spec:
  namespace: istio-system
  version: v1.22.0
status:
  ...
  state: Healthy
```

You can customize your control plane installation through the `Istio` resource using Istio's `Helm` configuration values:

```yaml
apiVersion: sailoperator.io/v1alpha1
kind: Istio
metadata:
  name: example
spec:
  version: v1.20.0
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

Create an instance of the `Istio` resource to install the Istio Control Plane.

Use the `istio-sample-kubernetes.yaml` file on vanilla Kubernetes:

```sh
# Create the istio-system namespace if it does not exist
kubectl create ns istio-system
kubectl apply -f chart/samples/istio-sample-kubernetes.yaml
```

Use the `istio-sample-openshift.yaml` file on OpenShift:

```sh
# Create the istio-system namespace if it does not exist
kubectl create ns istio-system
kubectl apply -f chart/samples/istio-sample-openshift.yaml
```

On OpenShift, you must also deploy the Istio CNI plugin by creating an instance of the `IstioCNI` resource:

```sh
# Create the istio-cni namespace if it does not exist
kubectl create ns istio-cni
kubectl apply -f chart/samples/istiocni-sample.yaml
```

View your control plane:

```sh
kubectl get istio default
```

### Undeploying the operator
Undeploy the operator from the cluster:

```sh
make undeploy
```

## Development

This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/).

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/),
which provide a reconcile function responsible for synchronizing resources until the desired state is reached on the cluster.

### Repository Setup

We're using [gitleaks](https://github.com/gitleaks/gitleaks) to scan the repository for secrets. After cloning, please enable the pre-commit hook by running `make git-hook`. This will make sure that `gitleaks` scans your contributions before you push them to GitHub, avoiding any potential secret leaks.

```sh
make git-hook
```

You will also need to sign off your commits to this repository. This can be done by adding the `-s` flag to your `git commit` command. If you want to automate that for this repository, take a look at `.git/hooks/prepare-commit-msg.sample`, it contains an example to do just that.

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

**Important:** Any API change should be discussed in an [SEP](https://github.com/istio-ecosystem/sail-operator/blob/main/enhancements/SEP1-enhancement-process.md) before being implemented.

If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

The API reference documentation can be found in the [docs](https://github.com/istio-ecosystem/sail-operator/tree/main/docs/api-reference/sailoperator.io.md)

### Writing Tests

Please try to keep business logic in separate packages that can be independently tested wherever possible, especially if you can avoid the usage of Kubernetes clients. It greatly simplifies testing if we don't need to use envtest everywhere.

E2E and integration tests should use the ginkgo-style BDD testing method, an example can be found in [`tests/integration/api/istio_test.go`](https://github.com/istio-ecosystem/sail-operator/blob/main/tests/integration/api/istio_test.go) for the test code and suite setup in [`tests/integration/api/suite_test.go`](https://github.com/istio-ecosystem/sail-operator/blob/main/tests/integration/api/suite_test.go). Unit tests should use standard golang xUnit-style tests (see [`pkg/kube/finalizers_test.go`](https://github.com/istio-ecosystem/sail-operator/blob/main/pkg/kube/finalizers_test.go) for an example).

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

## Community Support and Contributing
Please refer to the [CONTRIBUTING-SAIL-PROJECT.md](https://github.com/istio-ecosystem/sail-operator/blob/main/CONTRIBUTING.md) file for more information on how to contribute to the Sail Operator project. This file contains all the information you need to get started with contributing to the project.

## Sail Enhancement Proposal

SEP documents are used to propose and discuss non-trivial features or epics and any API changes. Please refer to the [SEP1-enhancement-process.md](https://github.com/istio-ecosystem/sail-operator/blob/main/enhancements/SEP1-enhancement-process.md) file for more information on how to create a Sail Enhancement Proposal (SEP) for the Sail Operator project.

SEP documents are stored in the [enhancements](https://github.com/istio-ecosystem/sail-operator/tree/main/enhancements) directory of the Sail Operator repository in Markdown format. If you want to create a SEP, be sure to check out the [SEP template](https://github.com/istio-ecosystem/sail-operator/blob/main/enhancements/SEP0-template.md).

## Issue management
Please refer to the [ISSUE-MANAGEMENT.md](https://github.com/istio-ecosystem/sail-operator/blob/main/ISSUE-MANAGEMENT.md) file for more information on how to report bugs and feature requests to the Sail Operator team.

If you found a bug in Istio, please refer to the [Istio GitHub repository](https://github.com/istio-ecosystem/sail-operator/blob/main/BUGS-AND-FEATURE-REQUESTS.md)
