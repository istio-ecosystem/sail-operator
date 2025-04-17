|Status|Authors|Created|
|---|---|---|
|WIP|@dgn,@luksa|2025-04-17|

# Working with Istio's `base` chart

## Overview

Istio contains some singleton objects, most of them contained in its `base` chart. These include:

* the default validation webhook
* the istio-reader ServiceAccount
* the CRDs

## Goals

* define how we manage the lifecycle of the components of the `base` chart
* provide guidance on how to deal with potential other singletons

## Non-goals

```sh

```

## Design

### User Stories

1. As a mesh admin, I want to install Istio's default validation webhook in order to make sure my users don't create broken Istio resources.
1. As a mesh admin, I want to establish trust between multiple clusters using Istio's multicluster topologies without having to re-establish that trust when I upgrade one of the control planes.

### API Changes

* When users create an `IstioRevision` resource named `default`, the Sail Operator will install the default ValidatingWebhookConfiguration and point it to the istiod instance managed by that `IstioRevision`.
* When users create an `IstioRevisionTag` resource named `default`, the Sail Operator will install the the default ValidatingWebhookConfiguration and point it to the istiod instance referenced by that `IstioRevisionTag`.
* Whenever an `IstioRevision` installs istiod, the Sail Operator will also create the corresponding `istio-reader` ServiceAccount. The lifecycle of that ServiceAccount will however not be tied to the IstioRevision: it will not be recreated during an upgrade in order to not break existing multicluster deployments. Instead, its lifecycle should be tied to the `Istio` instance controlling the `IstioRevision`.

### Architecture

## Alternatives Considered

## Implementation Plan

## Test Plan
