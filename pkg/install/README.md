# pkg/install

Library for managing istiod installations without running the Sail Operator.
Designed for embedding in other operators (e.g. OpenShift Ingress) that need
to install and maintain Istio as an internal dependency.

## Usage

```go
lib, err := install.New(kubeConfig, resourceFS, crdFS)  // defaults: QPS=50, Burst=100
notifyCh, err := lib.Start(ctx)

// In controller reconcile:
lib.Apply(install.Options{
    Namespace: "istio-system",
    Version:   "v1.24.3",
    Values:    install.GatewayAPIDefaults("istio-system"),
    ManageCRDs: true,
})

// Read result after notification:
for range notifyCh {
    status := lib.Status()
    // update conditions from status
}

// Teardown:
lib.Uninstall(ctx, "istio-system", "default")
```

## How it works

The Library runs as an independent actor with a simple state model:

1. **Apply** -- consumer sends desired state (version, namespace, values)
2. **Reconcile** -- Library installs/upgrades CRDs and istiod via Helm
3. **Drift detection** -- controller-runtime watches on owned resources and CRDs re-trigger reconciliation on changes
4. **Status** -- consumer reads the reconciliation result

The library delegates heavily to existing Sail Operator infrastructure:
- `pkg/reconcile.IstiodReconciler` for Helm install/uninstall/validate
- `pkg/watches.IstiodWatches` for drift detection watch list
- `pkg/istioversion` for version validation and resolution
- `pkg/istiovalues` for values merging

## Public API

### Constructor

- `New(kubeConfig, resourceFS, crdFS, opts...)` -- creates a Library with Kubernetes clients, Helm chart manager, and CRD manager (defaults: QPS=50, Burst=100; override with `WithQPS()` / `WithBurst()`)
- `FromDirectory(path)` -- creates an `fs.FS` from a filesystem path (alternative to embedded resources)

### Library methods

| Method | Description |
|---|---|
| `Start(ctx)` | Starts the controller manager and drift-detection watches; returns notification channel |
| `Apply(opts)` | Validates and sets desired state; enqueues reconciliation |
| `Stop()` | Cancels the reconciliation loop and waits for the manager to shut down |
| `Enqueue()` | Forces re-reconciliation without changing desired state |
| `Status()` | Returns the latest reconciliation result |
| `Uninstall(ctx, ns, rev)` | Performs Helm uninstall of istiod |

### Types

- **Options** -- install options: `Namespace`, `Version`, `Revision`, `Values`, `ManageCRDs`, `IncludeAllCRDs`, `OverwriteOLMManagedCRD`
- **Status** -- reconciliation result: `CRDState`, `CRDMessage`, `CRDs`, `Installed`, `Version`, `Error`
- **CRDManagementState** -- CRD state: `Unknown`, `Ready`, `NotReady`, `Error`
- **CRDInfo** -- per-CRD state: `Name`, `Managed`, `Ready`

### Helper functions

- `GatewayAPIDefaults(namespace)` -- pre-configured values for Gateway API mode
- `MergeValues(base, overlay)` -- deep-merge two Values structs (overlay wins)
- `ValidateOptions(opts)` -- checks that options are valid
- `LibraryRBACRules()` -- returns RBAC PolicyRules for a consumer's ClusterRole
- `AggregateState(infos)` -- derives overall CRD state from individual CRDInfo entries

## CRD management

The Library manages Istio CRDs when `ManageCRDs` is true (the default in PR #721; explicit on this branch).
CRDs are loaded from the embedded `chart/crds/` filesystem, rendered via the base Helm chart,
and filtered by `PILOT_INCLUDE_RESOURCES` / `PILOT_IGNORE_RESOURCES` when `IncludeAllCRDs` is false.

When an existing CRD has OLM labels (`olm.managed`), it is left alone unless `OverwriteOLMManagedCRD`
is set to true.

## Design: delegation over reimplementation

Unlike a standalone library, this implementation reuses existing Sail Operator packages
to minimize code duplication and maintenance burden:

| Concern | Shared Package | Library Role |
|---------|---------------|--------------|
| Helm install/uninstall | `pkg/reconcile.IstiodReconciler` | Wraps in `installer` struct |
| Watch list | `pkg/watches.IstiodWatches` | Registered via `RegisterOwnedWatches` |
| Event filtering | `pkg/watches.ShouldReconcile` | Used as controller-runtime predicates |
| Version management | `pkg/istioversion` | Delegates validation/resolution |
| Values merging | `pkg/istiovalues.MergeOverwrite` | Called from `MergeValues()` |
| Chart rendering | `pkg/helm` | Used for CRD rendering |

## Files

| File | Purpose |
|---|---|
| `library.go` | Public API, types (`Library`, `Status`, `Options`), constructor |
| `reconciler.go` | Controller-runtime reconciler, controller setup, installer |
| `crds.go` | CRD management: load, filter, classify, install, update |
| `values.go` | `GatewayAPIDefaults()`, `MergeValues()` |
| `images.gen.go` | Image configuration (generated) |
| `rbac.go` | RBAC rules for library consumers |
