# Sail Operator API Types Domain Knowledge

This document provides AI agents with detailed knowledge about the Sail Operator's Custom Resource Definitions (CRDs) and API types.

## Core Custom Resources

### Istio Resource
The primary resource for managing Istio control plane deployments.

**Key Fields:**
- `spec.version` - Istio version to install (defaults to operator's supported version)
- `spec.namespace` - Target namespace for control plane (typically `istio-system`)
- `spec.values` - Helm values for customizing Istio installation
- `spec.updateStrategy` - Controls how updates are performed (`InPlace` or `RevisionBased`)

**Status Fields:**
- `status.state` - Current state: `Healthy`, `Installing`, `Updating`, `Error`, etc.
- `status.activeRevisionName` - Name of the active IstioRevision
- `status.revisions` - Summary of all managed revisions

### IstioRevision Resource
Represents a specific deployment of Istio control plane components.

**Key Fields:**
- `spec.version` - Exact Istio version for this revision
- `spec.namespace` - Installation namespace
- `spec.values` - Helm configuration values

**Status Fields:**
- `status.state` - Revision state: `Installing`, `Healthy`, `Failed`, etc.
- `status.conditions` - Detailed condition information

### IstioCNI Resource
Manages the Istio CNI plugin (required for OpenShift and Ambient mesh).

**Key Fields:**
- `spec.version` - CNI plugin version
- `spec.namespace` - CNI installation namespace (typically `istio-cni`)
- `spec.values` - CNI-specific Helm values

### ZTunnel Resource (Alpha)
Manages ZTunnel workloads for Istio Ambient mesh mode.

**Key Fields:**
- `spec.version` - ZTunnel version
- `spec.namespace` - ZTunnel namespace (typically `ztunnel`)
- `spec.values` - ZTunnel configuration values

### IstioRevisionTag Resource
Creates revision tags for canary deployments and traffic shifting.

**Key Fields:**
- `spec.targetRef` - Reference to target IstioRevision
- `spec.tag` - Tag name (e.g., `canary`, `stable`)

## Common Patterns

### Values Configuration
All resources support Helm values via the `values` field:

```yaml
spec:
  values:
    global:
      variant: distroless
      logging:
        level: "all:info"
    meshConfig:
      trustDomain: cluster.local
      defaultConfig:
        proxyStatsMatcher:
          inclusionRegexps:
            - ".*outlier_detection.*"
```

### Version Management
- Versions are specified as semantic versions (e.g., `1.25.0`)
- If no version specified, uses operator's default supported version
- Version compatibility is enforced by the operator

### Resource Relationships
1. `Istio` → creates/manages → `IstioRevision`
2. `IstioRevisionTag` → references → `Istio`
3. Ambient mode requires: `Istio` + `IstioCNI` + `ZTunnel`
4. Sidecar mode requires: `Istio` (+ `IstioCNI` on OpenShift)

## Status Conditions

All resources use standard Kubernetes condition patterns:

**Common Condition Types:**
- `Ready` - Resource is ready and operational
- `Reconciled` - Last reconciliation was successful
- `ReconcileError` - Reconciliation encountered errors

**Condition Status Values:**
- `True` - Condition is active/successful
- `False` - Condition is inactive/failed
- `Unknown` - Condition state is uncertain

## Update Strategies

### InPlace Strategy
- Updates existing revision in-place
- Faster but with brief control plane downtime
- Default strategy

### RevisionBased Strategy
- Creates new revision alongside existing one
- Enables canary deployments and zero-downtime updates
- Requires manual traffic shifting via IstioRevisionTag

## Validation Rules

### Version Constraints
- Must use supported Istio versions (defined in `versions.yaml`)
- CNI and ZTunnel versions must be compatible with Istio version
- Revision names must be unique within namespace

### Namespace Requirements
- Control plane namespace must exist before creating Istio resource
- CNI deployed to `istio-cni` namespace
- ZTunnel deployed to `ztunnel` namespace

### Resource Dependencies
- IstioCNI must be deployed before Istio in Ambient mode
- ZTunnel requires both Istio and IstioCNI to be ready
- Removing Istio resource removes all associated IstioRevisions

## Generated Types

The `api/v1/values_types.gen.go` file contains auto-generated types from Istio's Helm values schema:

- **Values** - Root Helm values structure
- **GlobalConfig** - Global Istio configuration
- **PilotConfig** - Istiod (Pilot) specific configuration
- **ProxyConfig** - Sidecar proxy configuration
- **MeshConfig** - Service mesh configuration

These types ensure type-safe access to all Istio configuration options.

## Common Configuration Examples

### Basic Istio Installation
```yaml
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  namespace: istio-system
```

### Ambient Mesh Setup
```yaml
# 1. CNI
apiVersion: sailoperator.io/v1alpha1
kind: IstioCNI
metadata:
  name: default
spec:
  namespace: istio-cni

# 2. Control Plane
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  namespace: istio-system
  values:
    pilot:
      env:
        PILOT_ENABLE_AMBIENT: true

# 3. ZTunnel
apiVersion: sailoperator.io/v1
kind: ZTunnel
metadata:
  name: default
spec:
  namespace: ztunnel
```

### Revision-Based Canary Deployment
```yaml
# Main revision
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: stable
spec:
  namespace: istio-system
  updateStrategy:
    type: RevisionBased

# Canary revision tag
apiVersion: sailoperator.io/v1
kind: IstioRevisionTag
metadata:
  name: canary
spec:
  targetRef:
    kind: Istio
    name: default
```