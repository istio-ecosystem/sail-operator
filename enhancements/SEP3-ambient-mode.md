|Status                                             | Authors      | Created    | 
|---------------------------------------------------|--------------|------------|
|WIP | @yxun         | 2024-11-12 |

# Istio ambient mode

## Overview

Upstream Istio implements the ambient mode by splitting its functionality into two distinct layers. At the base, a per-node Layer 4(L4) ztunnel secure overlay handles routing and zero trust security for traffic. Above that, users can enable L7 waypoint proxies to get access the full range of Istio features.

The upstream Istio ambient mode can be installed and upgraded from istioctl with profile ambient. This is implemented by the upstream in-cluster IstioOperator resource APIs.

## Goals

- Provide a way for installing the ambient mode component profile and charts in Sail Operator. This includes the ztunnel component chart and binary, Istio CNI changes and related meshConfig in pilot.
- Provide a way for managing the ambient mode components life cycle. The implementation should enable in-place upgrades.

## Non-goals

- Uninstallation of only the ambient mode components. Currently, to cleanup the environment, need to compeletely uninstall Istio from a cluster.
- Istio Sidecar mode migration to ambient mode.
- Canary upgrade by the revision setting in ambient mode. 

## Design

### User Stories

1. As a user of Sail Operator, I want to be able to use a CRD and controller for installing and upgrading Istio ambient mode profile and components.
2. As a user of Sail Operator, I want to be able to run Istio ambient mode on a subset of OpenShift environments.

### API Changes

### Architecture

### Performance Impact

### Backward Compatibility

### Kubernetes vs OpenShift vs Other Distributions

## Alternatives Considered
Other approaches that have been discussed and discarded during or before the creation of the SEP. Should include the reasons why they have not been chosen.

## Implementation Plan
In the beginning, this should give a rough overview of the work required to implement the SEP. Later on when the SEP has been accepted, this should list the epics that have been created to track the work.

## Test Plan
When and how can this be tested? We'll want to automate testing as much as possible, so we need to start about testability early.

## Change History (only required when making changes after SEP has been accepted)
* 2024-07-09 Fixed a typo
