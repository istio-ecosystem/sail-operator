[![unit-test](https://github.com/istio-ecosystem/sail-operator/actions/workflows/unit-tests.yaml/badge.svg)](https://github.com/istio-ecosystem/sail-operator/actions/workflows/unit-tests.yaml)
[![integration test badge](https://github.com/istio-ecosystem/sail-operator/actions/workflows/integration-tests.yaml/badge.svg)](https://github.com/istio-ecosystem/sail-operator/actions/workflows/integration-tests.yaml)
[![update-deps badge](https://github.com/istio-ecosystem/sail-operator/actions/workflows/update-deps.yaml/badge.svg)](https://github.com/istio-ecosystem/sail-operator/actions/workflows/update-deps.yaml)
[![nightly-images badge](https://github.com/istio-ecosystem/sail-operator/actions/workflows/nightly-images.yaml/badge.svg)](https://github.com/istio-ecosystem/sail-operator/actions/workflows/nightly-images.yaml)

# Sail Operator

The Sail Operator manages the lifecycle of your [Istio](https://istio.io) control plane. It provides custom resources for you to deploy and manage your control plane components.

## Contributor Call

We have a weekly call every Thursday at 4PM CET / 10AM EST to discuss current progress and future plans. To add it to your Google Calendar, you can [subscribe to the Sail Operator calendar](https://calendar.google.com/calendar/u/0?cid=MDRhNzBkZjUwNmI5ZjFlMTAyYmUzZDhiZTFlNDA3ZjRlMjcwZjAzNmY4NDFkZTA1MmYzYzczYjk3OTU4ZGI2MUBncm91cC5jYWxlbmRhci5nb29nbGUuY29t).

## User Documentation
This document aims to provide an overview of the project and some information for contributors. For information on how to use the operator, take a look at the [User Documentation](docs/README.adoc).

## Table of Contents

- [How it works](#how-it-works)
- [Getting Started](#getting-started)
    - [Deploying the operator from source](#deploying-the-operator-from-source)
    - [Deploying the operator by using Helm charts](#deploying-the-operator-by-using-helm-charts)
    - [Deploying the Istio Control Plane](#deploying-the-istio-control-plane)
    - [Undeploying the operator](#undeploying-the-operator)
- [Development](#development)
    - [Repository Setup](#repository-setup)
    - [Test It Out](#test-it-out)
    - [Modifying the API definitions](#modifying-the-api-definitions)
    - [Writing Tests](#writing-tests)
    - [Integration Tests](#integration-tests)
    - [End-to-End Tests](#end-to-end-tests)
    - [Vendor-specific changes](#vendor-specific-changes)
    - [Developing on macOS](#developing-on-macos)
- [Release process](#release-process)
- [Versioning and Support Policy](#versioning-and-support-policy)
- [Community Support and Contributing](#community-support-and-contributing)
- [Sail Enhancement Proposal](#sail-enhancement-proposal)
- [Issue management](#issue-management)

## How it works

You manage your control plane through an `Istio` resource.

```yaml
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: example
spec:
  namespace: istio-system
```

**Note:** You can explicitly specify the version using the `spec.version` field. If not specified, the default version supported by the Operator will be used.

When you create an `Istio` resource, the sail operator then creates an `IstioRevision` that represents a control plane deployment.

```yaml
apiVersion: sailoperator.io/v1
kind: IstioRevision
metadata:
  name: example
  ...
spec:
  namespace: istio-system
status:
  ...
  state: Healthy
```

You can customize your control plane installation through the `Istio` resource using Istio's `Helm` configuration values:

```yaml
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: example
spec:
  namespace: istio-system
  values:
    global:
      variant: debug
      logging:
        level: "all:debug"
    meshConfig:
      trustDomain: example.com
      trustDomainAliases:
      - example.net
```

## Getting Started

You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.  
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Quick start using a local KIND cluster

For a quick start using a local cluster run:

```sh
make cluster
```

This deploys the cluster and preloads the latest operator image built from source to it.
You can then follow up by deploying the operator and using it locally.

Alternatively, you can deploy a local multi-cluster setup by running:

```sh
make cluster MULTICLUSTER=true
```

### Deploying the operator from source

Deploy the operator to the cluster:

```sh
make deploy
```

Alternatively, you can deploy the operator using OLM:

```sh
make deploy-olm
```

Make sure that the `HUB` and `TAG` environment variables point to your container image repository and that the repository is publicly accessible.

When deploying on a local KIND cluster, make sure to use the local image registry:

```sh
export HUB=localhost:5000
```

### Deploying the operator by using Helm charts

Deploy operator with Helm by using the following [guide](chart/README.md)

### Deploying the Istio Control Plane

#### Create namespaces

Create namespace for `Istio` Control Plane.

```sh
kubectl create namespace istio-system
```

For Ambient mode or use on Openshift, create a namespace for `IstioCNI` resource.

```sh
kubectl create namespace istio-cni
```

For Ambient mode, create a namespace for `ZTunnel` resource.

```sh
kubectl create namespace ztunnel
```

#### Deploy a Sidecar mode

Create an instance of the `Istio` resource to install the Istio Control Plane.

```sh
kubectl apply -f chart/samples/istio-sample.yaml
```

On OpenShift, you must also deploy the Istio CNI plugin by creating an instance of the `IstioCNI` resource:

```sh
kubectl apply -f chart/samples/istiocni-sample.yaml
```

View your control plane:

```sh
kubectl get istio default
```

#### Deploy an Ambient mode

Create an instance of the `Istio` Ambient resource to install the Istio Ambient Control Plane.

```sh
kubectl apply -f chart/samples/ambient/istio-sample.yaml
```

Create an instance of the `IstioCNI` resource to install the Istio CNI plugin.

```sh
kubectl apply -f chart/samples/ambient/istiocni-sample.yaml
```

Create an instal of the `ZTunnel` resource to install the ZTunnel plugin.

```sh
kubectl apply -f chart/samples/ambient/istioztunnel-sample.yaml
```

View your control plane:

```sh
kubectl get istio default
kubectl get istiocni default
kubectl get ztunnel default
```

**Note** - The version can be specified by modifying the `version` field within `Istio` and `IstioCNI` manifests.  
For other deployment options, refer to the [docs](docs) directory.

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

**Important:** Any API change should be discussed in an [SEP](enhancements/SEP1-enhancement-process.md) before being implemented.

If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

The API reference documentation can be found in the [docs](https://github.com/istio-ecosystem/sail-operator/tree/main/docs/api-reference/sailoperator.io.md)

### Writing Tests

Please try to keep business logic in separate packages that can be independently tested wherever possible, especially if you can avoid the usage of Kubernetes clients. It greatly simplifies testing if we don't need to use envtest everywhere.

E2E and integration tests should use the ginkgo-style BDD testing method, an example can be found in [`tests/integration/api/istio_test.go`](tests/integration/api/istio_test.go) for the test code and suite setup in [`tests/integration/api/suite_test.go`](tests/integration/api/suite_test.go). Unit tests should use standard golang xUnit-style tests (see [`pkg/kube/finalizers_test.go`](pkg/kube/finalizers_test.go) for an example).

### Integration Tests

Please check the specific instructions for the integration tests in the [integration](tests/integration/README.md) directory.

To run the integration tests, you can use the following command:

```sh
make test.integration
```

### End-to-End Tests

Please check the specific instructions for the end-to-end tests in the [e2e](tests/e2e/README.md) directory.

To run the end-to-end tests, you can use the following command:

```sh
make test.e2e.kind
```

or

```sh
make test.e2e.ocp
```

### Vendor-specific changes

As you might know, the Sail Operator project serves as the community upstream for the Red Hat OpenShift Service Mesh 3 operator. To accomodate any vendor-specific changes, we have a few places in the code base that allow for vendors to make downstream changes to the project with minimal conflicts. As a rule, these vendor-specific modifications should not include code changes, and they should be vendor-agnostic, ie if any other vendor wants to use them, they should be flexible enough to allow for doing that.

### Developing on macOS

There are some considerations that you need to take into account while trying to develop, debug and work on macOS and specially if you are using Podman instead on Docker. Please take a look into this [documentation](/docs/macos/develop-on-macos.md) for macOS specifics.

#### versions.yaml

The name of the versions.yaml file can be overwritten using the VERSIONS_YAML_FILE environment variable. This way, downstream vendors can point to a custom list of supported versions. Note that this file should be located in the `pkg/istioversion` directory, with the default value being `versions.yaml`.

#### vendor_defaults.yaml

By modifying `pkg/istiovalues/vendor_defaults.yaml`, vendors can change some defaults for the helm values. Note that these are defaults, not overrides, so user input will always take precendence.

## Release process

Please refer to the [RELEASE-PROCESS.md](RELEASE-PROCESS.md) file for more information on how the Sail Operator release process works.

## Versioning and Support Policy

Versioning for the Sail Operator will follow Istio versioning. When there's a new version of Istio released, there will be a corresponding release of the Sail Operator. The latest version of Istio can be installed using the latest version of the Sail Operator. For example, when Istio 1.25 is released, there will be a corresponding 1.25 release of the Sail Operator that supports installing Istio 1.25.

The Sail Operator will support n-2 releases of Istio. If you install the 1.25 Sail Operator, the Operator can install Istio 1.23-1.25.

Not all Istio patch versions will be included in Sail Operator releases. Some may be skipped. Also, the Sail Operator patch version will not correspond to the Istio patch version. For example, the 1.25.0 Sail Operator may only support installing Istio 1.25.1 but not Istio 1.25.0.

When an Istio release is out of support, the corresponding Sail Operator release will be out of support as well.

> [!NOTE]
> The first stable 1.0 release did not follow this versioning strategy but subsequent releases will.

## Community Support and Contributing
Please refer to the [CONTRIBUTING-SAIL-PROJECT.md](CONTRIBUTING.md) file for more information on how to contribute to the Sail Operator project. This file contains all the information you need to get started with contributing to the project.

## AI Agents for Development
If you're using AI coding assistants like Claude, GitHub Copilot, or Cursor, check out our [AI Agents Guide](docs/ai/ai-agents-guide.adoc) for information on how to configure these tools to understand Sail Operator patterns and best practices.

## Sail Enhancement Proposal

SEP documents are used to propose and discuss non-trivial features or epics and any API changes. Please refer to the [SEP1-enhancement-process.md](enhancements/SEP1-enhancement-process.md) file for more information on how to create a Sail Enhancement Proposal (SEP) for the Sail Operator project.

SEP documents are stored in the [enhancements](enhancements) directory of the Sail Operator repository in Markdown format. If you want to create a SEP, be sure to check out the [SEP template](enhancements/SEP0-template.md).

## Issue management
Please refer to the [ISSUE-MANAGEMENT.md](ISSUE-MANAGEMENT.md) file for more information on how to report bugs and feature requests to the Sail Operator team.

If you found a bug in Istio, please refer to the [Istio GitHub repository](BUGS-AND-FEATURE-REQUESTS.md)
