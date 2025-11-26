# Sail Operator API Types Domain Knowledge

This document provides AI agents with detailed knowledge about the Sail Operator's Custom Resource Definitions (CRDs) and API types.

## Core Custom Resources

### Istio Resource
The primary resource for managing Istio control plane deployments.

**Key Fields:**
- `spec.version` - Istio version to install (defaults to operator's default version)
- `spec.namespace` - Target namespace for control plane (default: `istio-system`, immutable)
- `spec.profile` - Built-in installation profile (e.g., `default`, `ambient`, `openshift`)
- `spec.values` - Helm values for customizing Istio installation
- `spec.updateStrategy.type` - Update strategy: `InPlace` (default) or `RevisionBased`
- `spec.updateStrategy.inactiveRevisionDeletionGracePeriodSeconds` - Seconds before removing inactive revision (default: 30)
- `spec.updateStrategy.updateWorkloads` - Automatically move workloads to new revision (default: false)

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
- `spec.version` - CNI plugin version (must match Istio version)
- `spec.namespace` - CNI installation namespace (default: `istio-cni`, immutable)
- `spec.profile` - Built-in installation profile
- `spec.values` - CNI-specific Helm values

**Note:** The resource name must be `default` (validated by CRD).

### ZTunnel Resource
Manages ZTunnel workloads for Istio Ambient mesh mode.

**Key Fields:**
- `spec.version` - ZTunnel version (must match Istio version)
- `spec.namespace` - ZTunnel namespace (default: `ztunnel`)
- `spec.values` - ZTunnel configuration values

**Note:** The resource name must be `default` (validated by CRD). ZTunnel was promoted to v1 API; a v1alpha1 version still exists for backwards compatibility.

### IstioRevisionTag Resource
Creates revision tags for canary deployments and traffic shifting. References an Istio or IstioRevision object and serves as an alias for sidecar injection.

**Key Fields:**
- `spec.targetRef.kind` - Kind of target resource (`Istio` or `IstioRevision`)
- `spec.targetRef.name` - Name of the target resource

**Status Fields:**
- `status.istioRevision` - Name of the referenced IstioRevision
- `status.istiodNamespace` - Namespace of the corresponding Istiod instance

## Common Patterns

### Profile Configuration
Istio and IstioCNI resources support profiles for predefined configuration sets:
- `ambient` - Ambient mesh configuration
- `default` - Default Istio configuration (always applied)
- `demo` - Demo configuration with additional features
- `empty` - Empty profile
- `openshift` - OpenShift-specific configuration (auto-applied on OpenShift)
- `openshift-ambient` - OpenShift with Ambient mesh
- `preview` - Preview features
- `remote` - Remote cluster configuration
- `stable` - Stable production configuration

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
# 1. CNI (required for Ambient)
apiVersion: sailoperator.io/v1
kind: IstioCNI
metadata:
  name: default
spec:
  namespace: istio-cni
  profile: ambient

# 2. Control Plane
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  namespace: istio-system
  profile: ambient

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
# Main Istio resource with RevisionBased strategy
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  namespace: istio-system
  updateStrategy:
    type: RevisionBased

# Revision tag pointing to the Istio resource
apiVersion: sailoperator.io/v1
kind: IstioRevisionTag
metadata:
  name: stable
spec:
  targetRef:
    kind: Istio
    name: default
```