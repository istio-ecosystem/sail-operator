# Sail Operator Controllers Domain Knowledge

This document provides AI agents with detailed knowledge about the Sail Operator's Kubernetes controllers and their reconciliation logic.

## Controller Architecture

The Sail Operator uses the controller-runtime framework with separate controllers for each Custom Resource:

- **IstioController** - Manages `Istio` resources and their lifecycle
- **IstioRevisionController** - Manages `IstioRevision` resources and Helm deployments
- **IstioCNIController** - Manages `IstioCNI` resources for CNI plugin
- **ZTunnelController** - Manages `ZTunnel` resources for Ambient mesh
- **IstioRevisionTagController** - Manages `IstioRevisionTag` resources for canary deployments
- **WebhookController** - Manages ValidatingAdmissionWebhook for Istio resources

## Controller Reconciliation Patterns

### IstioController (`controllers/istio/istio_controller.go`)

**Primary Responsibilities:**
- Creates and manages `IstioRevision` resources based on `Istio` spec
- Handles update strategies (InPlace vs RevisionBased)
- Manages revision lifecycle and cleanup
- Updates `Istio` status with active revision information

**Reconciliation Flow:**
1. Validate Istio version compatibility
2. Determine if new revision is needed (version change, config change)
3. Create new `IstioRevision` if required
4. Wait for revision to become Ready
5. For InPlace strategy: cleanup old revision
6. For RevisionBased: keep both revisions for canary deployment
7. Update Istio status with active revision

**Key Functions:**
- `getDesiredIstioRevision()` - Calculates desired revision configuration
- `updateRevisionIfNeeded()` - Handles revision updates
- `pruneOldRevisions()` - Cleanup logic for old revisions

### IstioRevisionController (`controllers/istiorevision/istiorevision_controller.go`)

**Primary Responsibilities:**
- Installs/upgrades Istio using Helm charts
- Manages Istio component deployments (istiod, gateways, etc.)
- Handles Helm value validation and templating
- Reports installation status and health

**Reconciliation Flow:**
1. Validate revision version and configuration
2. Fetch Istio Helm charts for specified version
3. Generate Helm values from IstioRevision spec
4. Install/upgrade Helm release
5. Wait for all components to be ready
6. Update IstioRevision status

**Key Functions:**
- `installOrUpdateRevision()` - Helm installation logic
- `validateValues()` - Helm values validation
- `checkComponentHealth()` - Health monitoring

### IstioCNIController (`controllers/istiocni/istiocni_controller.go`)

**Primary Responsibilities:**
- Deploys Istio CNI plugin as DaemonSet
- Manages CNI configuration and RBAC
- Handles OpenShift-specific CNI requirements
- Coordinates with Istio control plane

**Reconciliation Flow:**
1. Validate CNI version compatibility
2. Generate CNI-specific Helm values
3. Install CNI DaemonSet and configuration
4. Verify CNI pods are running on all nodes
5. Update IstioCNI status

### ZTunnelController (`controllers/ztunnel/ztunnel_controller.go`)

**Primary Responsibilities:**
- Deploys ZTunnel DaemonSet for Ambient mesh
- Manages ztunnel configuration and certificates
- Handles node-level ztunnel deployment
- Integrates with Istio CA for certificate management

**Reconciliation Flow:**
1. Verify Ambient mesh prerequisites (CNI + Istio)
2. Generate ZTunnel configuration
3. Deploy ztunnel DaemonSet
4. Configure certificate rotation
5. Update ZTunnel status

### IstioRevisionTagController (`controllers/istiorevisiontag/istiorevisiontag_controller.go`)

**Primary Responsibilities:**
- Creates revision tags for canary deployments
- Manages ValidatingAdmissionWebhook configuration
- Handles tag-to-revision mapping
- Enables traffic shifting between revisions

**Reconciliation Flow:**
1. Validate target IstioRevision exists and is ready
2. Create/update ValidatingAdmissionWebhook with tag
3. Configure webhook to inject tag label
4. Update IstioRevisionTag status

### WebhookController (`controllers/webhook/webhook_controller.go`)

**Primary Responsibilities:**
- Manages ValidatingAdmissionWebhook lifecycle
- Handles webhook certificate rotation
- Coordinates webhook configuration across revisions
- Manages admission webhook for sidecar injection

## Common Controller Patterns

### Status Management
All controllers follow standard Kubernetes status patterns:

```go
// Update condition
condition := metav1.Condition{
    Type:   "Ready",
    Status: metav1.ConditionTrue,
    Reason: "InstallationComplete",
    Message: "Istio installation completed successfully",
}
meta.SetStatusCondition(&resource.Status.Conditions, condition)

// Update status
if err := r.Status().Update(ctx, resource); err != nil {
    return ctrl.Result{}, err
}
```

### Error Handling and Requeuing
Controllers use exponential backoff for error recovery:

```go
// Requeue with exponential backoff
if err != nil {
    return ctrl.Result{RequeueAfter: time.Second * 30}, err
}

// Immediate requeue for configuration changes
if configChanged {
    return ctrl.Result{Requeue: true}, nil
}
```

### Resource Ownership
Controllers establish ownership relationships for garbage collection:

```go
// Set owner reference for child resources
if err := controllerutil.SetControllerReference(parent, child, r.Scheme); err != nil {
    return err
}
```

### Finalizer Management
Controllers use finalizers for cleanup coordination:

```go
// Add finalizer
if !controllerutil.ContainsFinalizer(resource, finalizerName) {
    controllerutil.AddFinalizer(resource, finalizerName)
    return r.Update(ctx, resource)
}

// Remove finalizer after cleanup
controllerutil.RemoveFinalizer(resource, finalizerName)
return r.Update(ctx, resource)
```

## Controller Dependencies

### Startup Order
1. **IstioCNIController** - Deploys CNI plugin first
2. **IstioController/IstioRevisionController** - Deploys control plane
3. **ZTunnelController** - Deploys ztunnel (Ambient only)
4. **IstioRevisionTagController** - Creates revision tags
5. **WebhookController** - Manages admission webhooks

### Inter-Controller Communication
Controllers coordinate through:
- **Status Conditions** - Ready/Error states
- **Labels/Annotations** - Metadata passing
- **Owner References** - Hierarchy relationships
- **Events** - Audit trail and debugging

## Debugging Controllers

### Common Issues
1. **Resource Not Found** - Check owner references and namespace
2. **Permission Denied** - Verify RBAC in `config/rbac/`
3. **Helm Failures** - Check Helm values and chart compatibility
4. **Status Not Updating** - Verify status subresource access

### Debugging Commands
```bash
# Check controller logs
kubectl logs -n sail-operator deployment/sail-operator-controller

# Describe resource status
kubectl describe istio -n istio-system

# Check events
kubectl events --for istio/default -n istio-system

# Debug Helm releases
helm list -n istio-system
helm status <release-name> -n istio-system
```

### Controller Metrics
Controllers expose metrics for monitoring:
- `controller_runtime_reconcile_total` - Reconciliation attempts
- `controller_runtime_reconcile_errors_total` - Reconciliation errors
- `controller_runtime_reconcile_time_seconds` - Reconciliation duration

## Testing Controllers

### Unit Tests
- Mock Kubernetes clients using `controller-runtime/pkg/client/fake`
- Test reconciliation logic with various resource states
- Verify status updates and condition management

### Integration Tests
- Use `envtest` for testing against real Kubernetes API
- Test controller interactions and resource lifecycle
- Verify garbage collection and finalizer behavior

### E2E Tests
- Test complete workflows on real clusters
- Verify Istio functionality after operator deployment
- Test upgrade scenarios and error recovery