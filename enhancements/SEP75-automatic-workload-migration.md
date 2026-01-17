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
* Provide optional version boundaries to control which updates trigger automatic migration (no limits by default, configurable to restrict patch/minor versions)
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

#### Deprecation / Removal of `updateStrategy.updateWorkloads` field

We accidentally included this field in our v1 CRD even though it never had any functionality. It doesn't really follow our existing API standards so we won't use it for this functionality going forward, and will remove it instead. As it was never functional to begin with, removing it should cause no harm.

#### IstioUpdateStrategy Enhancement

The existing `IstioUpdateStrategy` type is extended with a new `WorkloadMigration` field:

```go
type WorkloadMigrationConfig struct {
    // Defines how workloads should be moved from one control plane instance to another automatically.
    // Defaults to "". If not set, workload migration is disabled.
    // +kubebuilder:default=""
    Strategy WorkloadMigrationStrategy `json:"strategy,omitempty"`

    // Batched configures the batched migration strategy.
    // Only applicable when Strategy is "Batched".
    // +optional
    Batched *BatchedMigrationConfig `json:"batched,omitempty"`
}

type BatchedMigrationConfig struct {
    // Maximum number of deployments to restart concurrently during migration.
    // Defaults to 1.
    // +kubebuilder:default=1
    // +kubebuilder:validation:Minimum=1
    BatchSize *int32 `json:"batchSize,omitempty"`

    // Time to wait between deployment restart batches.
    // Defaults to 30s.
    // +kubebuilder:default="30s"
    DelayBetweenBatches *metav1.Duration `json:"delayBetweenBatches,omitempty"`

    // The highest version that the Sail Operator will automatically migrate workloads to.
    // This acts as a safety boundary to prevent automatic migration beyond a specific version.
    //
    // Behavior:
    // - If unset (default): Workloads will automatically migrate to any new version without limits.
    //   This includes patch, minor, and major version updates.
    // - If set: Workloads will only migrate to versions up to and including MaxVersion.
    //   Updates beyond this version will not trigger automatic migration.
    //
    // Examples:
    // - MaxVersion: unset -> migrate to any version (default behavior)
    // - MaxVersion: "1.24.999" -> migrate within 1.24.x only
    // - MaxVersion: "1.26.0" -> migrate up to and including 1.26.0
    // - MaxVersion: "1.999.0" -> migrate to any 1.x version, stop at 2.0
    //
    // +optional
    MaxVersion *string `json:"maxVersion,omitempty"`

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
   - The new version is within the `maxVersion` constraint (if specified, otherwise all versions trigger migration)

2. **Discovery**: The operator discovers workloads using old revisions by:
   - Listing all namespaces and checking their `istio.io/rev` or `istio-injection` labels
   - Listing all deployments and checking their pod template labels
   - Listing all pods and checking their pod annotations to detect injected revision
   - Comparing current annotations against the active revision name

3. **Migration**: Workloads are migrated in two phases:
   - **Phase 1**: Namespace label updates (no restarts required yet)
     - if the label is pointing to an `IstioRevisionTag` that is updated as part of the `Istio` update, this is a no-op.
     - if there is no label on the namespace, but only on the workload itself, the workload's label will be updated during the restart
   - **Phase 2**: Deployment restarts in configurable batches

4. **Validation**: Each batch waits for readiness before proceeding to the next batch

#### MaxVersion Decision Logic

The `maxVersion` field controls which version updates trigger automatic workload migration. The operator uses semantic version comparison to determine whether to migrate workloads.

**Decision Matrix:**

| Current Version | New Version | MaxVersion | Migration Triggered? | Rationale |
|----------------|-------------|----------------|---------------------|-----------|
| 1.24.1 | 1.24.2 | (unset) | ✅ Yes | No limits - any version update migrates |
| 1.24.1 | 1.25.0 | (unset) | ✅ Yes | No limits - any version update migrates |
| 1.24.1 | 2.0.0 | (unset) | ✅ Yes | No limits - even major versions migrate |
| 1.24.1 | 1.24.5 | "1.24.999" | ✅ Yes | New version is within maximum boundary |
| 1.24.1 | 1.25.0 | "1.24.999" | ❌ No | New version exceeds maximum boundary |
| 1.24.1 | 1.26.0 | "1.25.0" | ❌ No | New version exceeds maximum boundary |
| 1.24.1 | 1.25.3 | "1.26.0" | ✅ Yes | New version is within maximum boundary |

**Common Usage Patterns:**

1. **Unrestricted (Default)**: Don't set `batched.maxVersion`
   ```yaml
   workloadMigration:
     strategy: Batched
     # batched.maxVersion unset - migrate to any version
   ```
   - Automatic migration for all version updates (patch, minor, major)
   - Simplest configuration with full automation
   - Best for development/staging or environments with robust automated testing

2. **Patch-only Updates**: Set `batched.maxVersion` to limit minor version changes
   ```yaml
   workloadMigration:
     strategy: Batched
     batched:
       maxVersion: "1.24.999"  # Stay within 1.24.x
   ```
   - Automatic migration only for patch releases (1.24.1 → 1.24.2)
   - Requires manual update to `maxVersion` when ready for 1.25
   - Best for production environments with strict change control

3. **Controlled Minor Upgrades**: Set `batched.maxVersion` to specific target version
   ```yaml
   workloadMigration:
     strategy: Batched
     batched:
       maxVersion: "1.26.0"  # Update this when ready for 1.27
   ```
   - Migrate through 1.25.x and 1.26.x automatically
   - Requires updating `maxVersion` when ready for 1.27
   - Best for staged rollouts with validation between minor versions

**Implementation Note:** Version comparison uses semantic versioning rules. When `maxVersion` is set, the operator compares `spec.version` against `maxVersion` using standard semver comparison (`<=`). When unset, all versions are accepted.

#### Status Reporting

Migration progress and failures are reported through the Istio resource status.

##### Status Structure

The `Istio` resource status is extended with a `WorkloadMigrationStatus` field:

```go
type IstioStatus struct {
    // ... existing fields ...
    
    // WorkloadMigration reports the status of automatic workload migration.
    // Only present when workloadMigration.strategy is enabled.
    // +optional
    WorkloadMigration *WorkloadMigrationStatus `json:"workloadMigration,omitempty"`
}

type WorkloadMigrationStatus struct {
    // State represents the current state of the migration process.
    // +kubebuilder:validation:Enum=Idle;InProgress;Completed;Failed
    State WorkloadMigrationState `json:"state"`
    
    // TargetRevision is the revision that workloads are being migrated to.
    // +optional
    TargetRevision string `json:"targetRevision,omitempty"`
    
    // TotalWorkloads is the total number of workloads discovered for migration.
    // +optional
    TotalWorkloads int32 `json:"totalWorkloads,omitempty"`
    
    // MigratedWorkloads is the number of workloads successfully migrated.
    // +optional
    MigratedWorkloads int32 `json:"migratedWorkloads,omitempty"`
    
    // FailedWorkloads is the number of workloads that failed to migrate.
    // +optional
    FailedWorkloads int32 `json:"failedWorkloads,omitempty"`
    
    // Failures contains details about workloads that failed to migrate.
    // Limited to the most recent 10 failures to avoid status bloat.
    // +optional
    Failures []WorkloadMigrationFailure `json:"failures,omitempty"`
    
    // StartTime is when the current migration started.
    // +optional
    StartTime *metav1.Time `json:"startTime,omitempty"`
    
    // CompletionTime is when the migration completed (successfully or with failures).
    // +optional
    CompletionTime *metav1.Time `json:"completionTime,omitempty"`
    
    // Batched contains status specific to the batched migration strategy.
    // Only present when strategy is "Batched".
    // +optional
    Batched *BatchedMigrationStatus `json:"batched,omitempty"`
}

type BatchedMigrationStatus struct {
    // CurrentBatch is the current batch being processed (1-indexed).
    // +optional
    CurrentBatch int32 `json:"currentBatch,omitempty"`
    
    // TotalBatches is the total number of batches to process.
    // +optional
    TotalBatches int32 `json:"totalBatches,omitempty"`
}

type WorkloadMigrationState string

const (
    WorkloadMigrationStateIdle       WorkloadMigrationState = "Idle"
    WorkloadMigrationStateInProgress WorkloadMigrationState = "InProgress"
    WorkloadMigrationStateCompleted  WorkloadMigrationState = "Completed"
    WorkloadMigrationStateFailed     WorkloadMigrationState = "Failed"
)

type WorkloadMigrationFailure struct {
    // Namespace of the failed workload.
    Namespace string `json:"namespace"`
    
    // Name of the failed workload.
    Name string `json:"name"`
    
    // Kind of the failed workload (e.g., "Deployment").
    Kind string `json:"kind"`
    
    // Reason for the failure.
    Reason string `json:"reason"`
    
    // Timestamp when the failure occurred.
    Timestamp metav1.Time `json:"timestamp"`
}
```

##### Status Examples

**Migration in progress:**
```yaml
status:
  workloadMigration:
    state: InProgress
    targetRevision: default-v1-26-0
    totalWorkloads: 47
    migratedWorkloads: 23
    failedWorkloads: 0
    startTime: "2025-10-21T10:30:00Z"
    batched:
      currentBatch: 5
      totalBatches: 10
```

**Migration completed successfully:**
```yaml
status:
  workloadMigration:
    state: Completed
    targetRevision: default-v1-26-0
    totalWorkloads: 47
    migratedWorkloads: 47
    failedWorkloads: 0
    startTime: "2025-10-21T10:30:00Z"
    completionTime: "2025-10-21T10:45:23Z"
    batched:
      currentBatch: 10
      totalBatches: 10
```

**Migration completed with failures:**
```yaml
status:
  workloadMigration:
    state: Failed
    targetRevision: default-v1-26-0
    totalWorkloads: 47
    migratedWorkloads: 44
    failedWorkloads: 3
    failures:
      - namespace: production
        name: legacy-app
        kind: Deployment
        reason: "Readiness timeout exceeded after 5m0s"
        timestamp: "2025-10-21T10:35:12Z"
      - namespace: staging
        name: flaky-service
        kind: Deployment
        reason: "Deployment failed to reach ready state: 2/3 replicas ready"
        timestamp: "2025-10-21T10:38:45Z"
      - namespace: production
        name: critical-app
        kind: Deployment
        reason: "Readiness timeout exceeded after 5m0s"
        timestamp: "2025-10-21T10:42:30Z"
    startTime: "2025-10-21T10:30:00Z"
    completionTime: "2025-10-21T10:45:23Z"
    batched:
      currentBatch: 8
      totalBatches: 10
```

##### Status Update Frequency

- Status is updated at the start and end of each batch
- Status counters are updated as each workload completes (success or failure)
- The `Failures` array in status is limited to 10 most recent entries to prevent unbounded growth

#### WorkloadManager & InUse Detection

The introduction of a `WorkloadManager` is proposed that will handle workload-specific tasks such as label updates and InUse detection. Currently, InUse detection code is spread over the `Istio`/`IstioRevision`/`IstioRevisionTag` controllers. Implementation of this SEP should include a refactoring that moves the code into a common package.

#### Deployment Restart Mechanism

Deployments are restarted using Kubernetes' standard rolling update mechanism:

1. **Label Update**: Pod template labels are updated to reference the new revision, if required - the only exception being when `IstioRevisionTag`s are used, and the referenced `Istio` resource is the one being updated
2. **Restart Annotation**: If a pod label change was not required, a `kubectl.kubernetes.io/restartedAt` annotation is added to trigger pod replacement
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

## Test Plan

The following should serve as an inspiration as to which use cases should be tested, rather than an exact plan to follow. Depending on the specifics of the implementation, it might turn out that some of these scenarios are better-suited to be tested in another testing stage than what is listed here.

### Unit Tests

**API Validation and Defaults:**
- Validate `workloadMigration.strategy` enum values
- Default values applied correctly (`batchSize=1`, `delayBetweenBatches=30s`, `readinessTimeout=5m`)
- Invalid configurations rejected (e.g., `batchSize < 1`)
- API serialization/deserialization of all new types

**Workload Discovery:**
- List namespaces with `istio.io/rev` label matching old revision
- List namespaces with `istio-injection=enabled` for default revision
- List deployments with pod template labels matching old revision
- List deployments by inspecting pod annotations for injected revision
- Exclude namespaces/deployments already on target revision
- Handle empty clusters (no workloads found)

**Version Comparison Logic:**
- Semantic version parsing and comparison
- Version boundary checks for all scenarios in decision matrix
- `maxVersion` unset allows all versions
- `maxVersion="1.24.999"` blocks 1.25.0 but allows 1.24.x
- Handle malformed version strings gracefully

**Batch Calculation:**
- Calculate correct number of batches given total workloads and batch size
- Handle edge cases (0 workloads, workloads < batch size, workloads % batch size != 0)
- Respect batch size configuration

**Status Updates:**
- State transitions (Idle → InProgress → Completed/Failed)
- Counter updates (totalWorkloads, migratedWorkloads, failedWorkloads)
- Batch progress tracking (currentBatch, totalBatches)
- Failure array limited to 10 most recent entries
- Timestamps set correctly (startTime, completionTime)

**Migration Logic:**
- Namespace label updates (add/update `istio.io/rev` or `istio-injection`)
- Deployment pod template label updates
- Restart annotation added (`kubectl.kubernetes.io/restartedAt`)
- Skip label updates when using IstioRevisionTag

### Integration Tests

**Basic Migration Flow:**
- Enable migration with `strategy: Batched`
- Verify namespaces with old revision labels are updated
- Verify deployments are restarted with correct annotations
- Verify status reflects migration state

**Batching Behavior:**
- Configure `batchSize=3` with 10 deployments
- Verify deployments processed in groups of 3
- Verify delay between batches is respected
- Verify status shows current/total batch progress

**Readiness Checking:**
- Deployment becomes ready within timeout → migration proceeds
- Deployment doesn't become ready → timeout occurs, marked as failed
- Failed deployment doesn't block subsequent batches
- Status captures failure details correctly

**Status Reporting:**
- Status created when migration starts
- Status updated after each batch
- Status shows Completed when all succeed
- Status shows Failed when any workload fails
- Failure details captured with namespace, name, reason, timestamp

**Version Boundaries:**
- Migration triggered when version ≤ maxVersion
- Migration skipped when version > maxVersion
- Migration always triggered when maxVersion unset
- Handle version comparison edge cases

**Multiple Revisions:**
- Migrate workloads from multiple old revisions to single active revision
- Handle mix of namespaces using `istio.io/rev` and `istio-injection=enabled`
- Handle workloads with deployment-level revision overrides

**IstioRevisionTag Support:**
- Workloads using revision tag migrate when Istio version changes
- Revision tag target updated before workload migration
- Namespace labels remain pointing to tag, not direct revision

**Edge Cases:**
- No workloads to migrate → status shows Completed immediately
- All namespaces already on target revision → no-op
- Deployment deleted during migration → handle gracefully
- Istio resource deleted during migration → migration stops
- Controller restart during migration → resume or restart migration

### E2E Tests

**End-to-End Basic Migration:**
- Install Istio v1.24.0 with RevisionBased update strategy
- Deploy sample workloads (httpbin, sleep) in multiple namespaces
- Update Istio to v1.25.0 with `workloadMigration.strategy: Batched`
- Verify all workloads migrate to new revision
- Verify workloads remain healthy and traffic flows correctly
- Verify old control plane can be cleaned up

**Failure Recovery:**
- Deploy workload with PodDisruptionBudget preventing restart
- Deploy normal workloads
- Upgrade Istio with automatic migration
- Verify problematic workload times out but others migrate successfully
- Verify status reports failure with correct details
- Verify Events emitted for failure

**Zero-Downtime Migration:**
- Deploy service with continuous traffic (load generator)
- Enable automatic migration with conservative settings
- Upgrade Istio version
- Monitor service availability during migration
- Verify no failed requests (within acceptable threshold)
- Verify gradual rollout maintains availability

**Performance at Scale:**
- Deploy 100 namespaces with 200 total deployments
- Upgrade Istio with automatic migration
- Measure migration completion time
- Verify operator memory usage remains stable
- Verify Kubernetes API rate limits not exceeded

## Example Configuration

### Basic Automatic Migration (Default - No Limits)
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
      # batched.maxVersion is unset - workloads will migrate to any version automatically
```

### Patch-only Updates (Production with Strict Change Control)
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
      batched:
        maxVersion: "1.26.999"  # Only migrate within 1.26.x, stop at 1.27
        batchSize: 3  # Conservative batching for production
        delayBetweenBatches: 2m
        readinessTimeout: 10m
```

### Controlled Minor Version Upgrades (Staged Rollout)
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
      batched:
        maxVersion: "1.26.0"  # Auto-migrate up to 1.26.x, then requires manual update
        batchSize: 5
        delayBetweenBatches: 60s
        readinessTimeout: 10m
```

## Security Considerations

There's a risk that we break security features configured by the user because they don't work properly in the new version, e.g. deprecated features or custom changes made using `EnvoyFilter`. We can't really mitigate this, so workload migration will always be disabled by default and should be used with caution.

## Future Enhancements

### Per-Workload Annotations

While the initial implementation provides cluster-wide and namespace-level migration configuration through the `Istio` resource, some users may need to override migration behavior on a per-workload basis. This could be achieved through annotations on Deployment resources.

**Potential annotations:**
- `sailoperator.io/readiness-timeout: 10s`: Override the readiness timeout for specific workloads
- `sailoperator.io/skip-migration: true`: Exclude specific workloads from automatic migration
- `sailoperator.io/migration-batch: 3`: Assign workload to a specific migration priority (integer, lower values migrate first)

### Failure Policy

The initial implementation will continue processing all batches even if individual workloads fail to migrate, reporting all failures at the end. Some users may want different behavior when migrations fail. We could implement a `failurePolicy` field that allows users to specify what the operator should do in case a workload does not become ready.

**Potential `failurePolicy` values:**
* `ContinueOnError`: Continue migrating all workloads, report failures at end
* `FailFast`: Stop immediately on first failure

### Kubernetes Events

The operator emits Kubernetes events for significant migration steps and all failures.

## Implementation Plan

### Iteration 1: Basic Automatic Migration
Users can enable automatic migration with `strategy: Batched` and workloads migrate automatically during upgrades.

- [ ] Add API: `workloadMigration.strategy: Batched` (with hardcoded defaults)
- [ ] Implement workload discovery (namespaces and deployments using old revisions)
- [ ] Implement namespace label migration
- [ ] Implement serial deployment restarts (one at a time, no batching yet)
- [ ] Wait for deployment readiness before proceeding
- [ ] Unit and integration tests
- [ ] Basic documentation with single example

### Iteration 2: Batching and Status Visibility
Users can control migration speed and monitor progress through status.

- [ ] Add configurable `batchSize` and `delayBetweenBatches` to API
- [ ] Implement batch processing logic
- [ ] Add `WorkloadMigrationStatus` to track progress (state, counters, batch info)
- [ ] Unit and integration tests for batching
- [ ] Documentation for batch configuration and monitoring

### Iteration 3: Safety and Error Handling
Users can safely run migrations in production with visibility into failures.

- [ ] Add configurable `readinessTimeout` to API
- [ ] Implement readiness check and timeout handling per deployment
- [ ] Track and report failures in `WorkloadMigrationStatus.failures`
- [ ] Unit and integration tests for failure scenarios
- [ ] E2E tests for complete migration workflows
- [ ] Troubleshooting documentation

### Iteration 4: Version Control
Users can restrict automatic migration to patch updates or specific version ranges.

- [ ] Add `maxVersion` field to API
- [ ] Implement semantic version comparison
- [ ] Gate migration trigger based on version boundaries
- [ ] Unit and integration tests for version logic
- [ ] Documentation with patch-only and controlled upgrade examples

## Change History

* 2025-10-22: Updated `maxVersion` field, added some potential future enhancements, added status reporting, moved most fields under `batched` fields for extensibility, rewrote implementation plan, added testing scenarios
* 2025-05-22: Initial SEP created
