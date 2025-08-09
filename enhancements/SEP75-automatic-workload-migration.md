| Status | Authors | Created | 
|---|---|---|
| WIP | @dgn | 2025-05-22 |

# Automatic Workload Migration

Tracked in [#75](https://github.com/istio-ecosystem/sail-operator/issues/75).

## Overview

When using Istio's revision-based update strategy, workloads must be migrated from old control plane revisions to new ones after an upgrade. Currently, this migration process is manual, requiring users to update namespace and pod labels and restart deployments to pick up new sidecar proxy versions. This creates operational overhead and potential for errors, especially in environments with many namespaces and workloads.

This enhancement introduces automatic workload migration functionality to the Sail Operator, enabling seamless migration of workloads from old IstioRevisions to the current active revision when `workloadMigration.strategy` is set in the Istio resource's update strategy.

## Goals

* Automatically migrate workloads from old IstioRevisions to the active revision when enabled
* Provide configurable migration behavior including batch sizes, delays, and timeouts
* Zero-downtime migration with proper health checking between batches
* Handle both namespace-level and deployment-level revision targeting
* Support both default revision (`istio-injection=enabled`) and named revisions (`istio.io/rev=<revision>`)

## Non-goals

* Support for migration of workloads in external clusters managed by remote control planes
* Migration of StatefulSets, DaemonSets, or other workload types (focus on Deployments initially)
* Complex scheduling or dependency-aware migration ordering
* Ambient Mesh support - ztunnel does not yet support revisions

## Design

### User Stories

1. **As a platform engineer**, I want workloads to automatically migrate to new Istio versions during control plane upgrades, so that I don't have to manually update namespace labels and restart deployments.

2. **As a cluster operator**, I want to configure the migration behavior (batch sizes, delays) to control the impact on my workloads during upgrades.

3. **As an application owner**, I want my applications to maintain availability during Istio upgrades without manual intervention.

4. **As a platform team**, I want to use stable revision tags while still having workloads automatically migrate to new control plane versions.

### API Changes

#### IstioUpdateStrategy Enhancement

The existing `IstioUpdateStrategy` type is extended with a new `WorkloadMigration` field:

```go
type WorkloadMigrationConfig struct {
    // Defines how workloads should be moved from one control plane instance to another automatically.
    // Defaults to "". If not set, workload migration is disabled.
    // +kubebuilder:default=""
    Strategy WorkloadMigrationStrategy `json:"strategy,omitempty"`

    // Maximum number of deployments to restart concurrently during migration.
    // Defaults to 1.
    // +kubebuilder:default=1
    // +kubebuilder:validation:Minimum=1
    BatchSize *int32 `json:"batchSize,omitempty"`

    // Time to wait between deployment restart batches.
    // Defaults to 30s.
    // +kubebuilder:default="30s"
    DelayBetweenBatches *metav1.Duration `json:"delayBetweenBatches,omitempty"`

    // The highest version that the Sail Operator will update to. Updates to versions later than MaximumVersion
    // will not trigger workload migration.
    // If unset, the operator will only trigger workload migration for new patch versions on the current minor version stream.
    MaximumVersion *string `json:"maximumVersion,omitempty"`

    // Maximum time to wait for a deployment to become ready after restart.
    // Defaults to 5m.
    // +kubebuilder:default="5m"
    ReadinessTimeout *metav1.Duration `json:"readinessTimeout,omitempty"`
}

type WorkloadMigrationStrategy string

const (
	WorkloadMigrationStrategyDisabled WorkloadMigrationStrategy = ""
  WorkloadMigrationStrategyBatched  WorkloadMigrationStrategy = "Batched"
)
```

#### RBAC Permissions

The operator already has the required permission to update `Deployment` resources.

### Architecture

#### Migration Flow

1. **Trigger**: Migration is triggered when:
   - `workloadMigration.strategy` is set to a non-empty value (e.g., "Batched")
   - An Istio resource using the RevisionBased updateStrategy is updated and a new revision becomes the active revision
   - The new version is within the `maximumVersion` constraint (if specified)

2. **Discovery**: The operator discovers workloads using old revisions by:
   - Listing all namespaces and checking their `istio.io/rev` or `istio-injection` labels
   - Listing all deployments and checking their pod template labels
   - Listing all pods and checking their pod annotations to detect injected revision
   - Comparing current annotations against the active revision name

3. **Migration**: Workloads are migrated in two phases:
   - **Phase 1**: Namespace label updates (no restarts required yet)
   - **Phase 2**: Deployment restarts in configurable batches

4. **Validation**: Each batch waits for readiness before proceeding to the next batch

#### WorkloadManager & InUse Detection

The introduction of a `WorkloadManager` is proposed that will handle workload-specific tasks such as label updates and InUse detection. Currently, InUse detection code is spread over the `Istio`/`IstioRevision`/`IstioRevisionTag` controllers. Implementation of this SEP should include a refactoring that moves the code into a common package.

#### Deployment Restart Mechanism

Deployments are restarted using Kubernetes' standard rolling update mechanism:

1. **Label Update**: Pod template labels are updated to reference the new revision, if required - the only exception being when IstioRevisionTags are used
2. **Restart Annotation**: A `kubectl.kubernetes.io/restartedAt` annotation is added to trigger pod replacement
3. **Health Check**: The operator waits for `deployment.Status.ReadyReplicas == deployment.Spec.Replicas`

### Performance Impact

* **Discovery Overhead**: The operator lists all namespaces and deployments once per migration
* **Batch Processing**: Migration impact is controlled through configurable batch sizes and delays
* **Memory Usage**: Minimal additional memory for tracking migration state
* **Network Traffic**: Standard Kubernetes API calls for object updates

### Backward Compatibility

* **Opt-in Feature**: Migration only occurs when `workloadMigration.strategy` is explicitly set to a non-empty value
* **Default Behavior**: Existing behavior is unchanged for users who don't enable the feature
* **API Compatibility**: All new fields are optional with sensible defaults
* **GitOps Support**: GitOps Deployments are supported when using `IstioRevisionTag` or tool-specific instructions to ignore revision label updates

### Kubernetes vs OpenShift vs Other Distributions

No distribution-specific dependencies.

## Alternatives Considered

TBD

## Implementation Plan

### Phase 1: Core Implementation
- [ ] Extend IstioUpdateStrategy API with WorkloadMigrationConfig
- [ ] Implement core migration logic in Istio controller
- [ ] Refactor InUse detection into WorkloadManager
- [ ] Implement namespace label migration
- [ ] Implement deployment restart with batching

### Phase 2: Testing
- [ ] Unit tests for all migration functions
- [ ] Integration tests for migration scenarios
- [ ] E2E tests for end-to-end migration workflows

### Phase 3: Documentation and Validation
- [ ] User documentation updates
- [ ] Example configurations

### Phase 4: Future Enhancements (Optional)
- [ ] Support for StatefulSets and DaemonSets
- [ ] Migration rollback capabilities
- [ ] Advanced scheduling options (maintenance windows)
- [ ] Integration with external monitoring systems

## Test Plan

### Unit Tests

### Integration Tests

### E2E Tests

## Example Configuration

### Basic Automatic Migration
```yaml
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  version: v1.26.0
  updateStrategy:
    type: RevisionBased
    workloadMigration:
      strategy: Batched
```

### Advanced Migration Configuration
```yaml
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  version: v1.26.0
  updateStrategy:
    type: RevisionBased
    workloadMigration:
      strategy: Batched
      batchSize: 5
      delayBetweenBatches: 60s
      readinessTimeout: 10m
```

### Usage with Revision Tags
```yaml
# First, create a revision tag
apiVersion: sailoperator.io/v1
kind: IstioRevisionTag
metadata:
  name: stable
spec:
  targetRef:
    kind: Istio
    name: default

---
# Workloads can use stable revision tag
apiVersion: v1
kind: Namespace
metadata:
  name: production
  labels:
    istio.io/rev: stable
```

When the Istio version is updated, workloads using the `stable` tag will automatically be migrated to the new revision.

## Security Considerations

There's a risk that we break security features configured by the user because they don't work properly in the new version, e.g. deprecated features or custom changes made using `EnvoyFilter`. We can't really mitigate this, so workload migration will always be disabled by default and should be used with caution.

## Change History

* 2025-05-22: Initial SEP created
