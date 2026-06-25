# Architecture

This document describes the high-level architecture of the Sail Operator.

See also [AGENTS.md](AGENTS.md) for development workflow, commands, and contribution guidelines.

## Project Structure

```
sail-operator/
├── api/                    # CRD type definitions
│   ├── v1/                 # Stable API group (Istio, IstioRevision, IstioRevisionTag, IstioCNI)
│   └── v1alpha1/           # Experimental API group (ZTunnel)
├── cmd/                    # Operator binary entry point
│   └── main.go             # Manager setup, controller registration, platform detection
├── controllers/            # Kubernetes controller implementations
│   ├── istio/              # Top-level Istio CR controller
│   ├── istiorevision/      # Revision lifecycle via Helm charts
│   ├── istiorevisiontag/   # Revision traffic routing
│   ├── istiocni/           # CNI plugin controller
│   ├── ztunnel/            # Ambient mesh tunnel controller
│   └── webhook/            # ValidatingWebhookConfiguration controller
├── pkg/                    # Core packages (see "Core Packages" below)
├── resources/              # Embedded Istio Helm charts and profiles, per version
├── chart/                  # Operator's own Helm chart for deployment
├── bundle/                 # OLM bundle (ClusterServiceVersion, metadata)
├── tests/
│   ├── e2e/                # End-to-end tests (Ginkgo, against real clusters)
│   └── integration/        # Integration tests (Ginkgo, envtest)
├── hack/                   # Shell scripts for chart downloads, CRD extraction, patching
├── tools/                  # Dependency update and utility scripts
├── enhancements/           # Sail Enhancement Proposals (SEPs)
├── docs/                   # Documentation
├── common/                 # Shared Makefile infrastructure (from istio/common-files)
├── ossm/                   # OpenShift Service Mesh downstream code
└── licenses/               # Third-party license information
```

## High-Level System Diagram

```
                          ┌─────────────────────────────────────────────┐
                          │              Kubernetes API                 │
                          └───────────────────┬─────────────────────────┘
                                              │
                          ┌───────────────────▼─────────────────────────┐
                          │           Sail Operator (Manager)           │
                          │                                             │
                          │  ┌──────────┐  ┌───────────┐  ┌─────────┐  │
                          │  │  Istio   │  │ IstioCNI  │  │ ZTunnel │  │
                          │  │Controller│  │Controller │  │Controller│  │
                          │  └────┬─────┘  └─────┬─────┘  └────┬────┘  │
                          │       │              │              │       │
                          │  ┌────▼─────────┐    │              │       │
                          │  │IstioRevision │    │              │       │
                          │  │  Controller  │    │              │       │
                          │  └────┬─────────┘    │              │       │
                          │       │              │              │       │
                          │  ┌────▼──────────────▼──────────────▼────┐  │
                          │  │          Helm ChartManager            │  │
                          │  │  (install/upgrade embedded charts)    │  │
                          │  └────┬─────────────────────────────────┘  │
                          │       │                                     │
                          └───────┼─────────────────────────────────────┘
                                  │
                          ┌───────▼─────────────────────────────────────┐
                          │         Deployed Istio Components           │
                          │  (istiod, CNI DaemonSet, ZTunnel, gateways) │
                          └─────────────────────────────────────────────┘
```

The Istio controller watches `Istio` CRs and creates `IstioRevision` resources.
The IstioRevision controller uses the Helm ChartManager to install the actual
Istio control plane components. IstioCNI and ZTunnel controllers manage their
respective components independently through the same ChartManager.

## Core Components

### Entry Point (`cmd/main.go`)

The operator binary. Responsibilities:
- Parse flags (metrics address, config file path, resource directory)
- Detect platform (Kubernetes vs OpenShift) to select the default Helm profile
- Load operator configuration from `/etc/sail-operator/config.properties`
- Initialize embedded or filesystem-based Istio chart resources
- Create the controller-runtime Manager with leader election and secure metrics
- Register all six controllers and start the manager
- On OpenShift: fetch cluster TLS profile and watch for changes (triggers restart on change)

### Controllers (`controllers/`)

Each controller follows the `StandardReconciler` pattern from `pkg/reconciler/`,
which provides common reconciliation flow with finalizer support.

| Controller | CRD | Key Behavior |
|---|---|---|
| `istio` | `Istio` | Validates version, selects update strategy (InPlace or RevisionBased), creates/updates `IstioRevision` |
| `istiorevision` | `IstioRevision` | Runs the values pipeline, calls ChartManager to install Helm releases (base, istiod, gateway) |
| `istiorevisiontag` | `IstioRevisionTag` | Manages MutatingWebhookConfigurations to route sidecar injection between revisions |
| `istiocni` | `IstioCNI` | Deploys the Istio CNI DaemonSet via the cni chart |
| `ztunnel` | `ZTunnel` | Deploys the ZTunnel DaemonSet for ambient mode via the ztunnel chart |
| `webhook` | *(none)* | Manages ValidatingWebhookConfigurations for CRD validation |

### Custom Resource Relationships

```
Istio ──creates──▶ IstioRevision ──installs──▶ Helm releases (base, istiod, gateway)
                        ▲
IstioRevisionTag ───────┘  (points to a revision for traffic routing)

IstioCNI ──installs──▶ cni chart
ZTunnel  ──installs──▶ ztunnel chart
```

- **Sidecar mode** requires: `Istio` (and `IstioCNI` on OpenShift)
- **Ambient mode** requires: `Istio` + `IstioCNI` + `ZTunnel`

### Core Packages (`pkg/`)

| Package | Purpose |
|---|---|
| `helm` | Helm v4 ChartManager: chart loading (`FSLoader`), install/upgrade, post-renderer for label injection |
| `install` | Installation orchestration: CRD management, RBAC setup, Library pattern for operator-in-operator embedding |
| `istiovalues` | Values transformation pipeline: profiles, image digests, FIPS/TLS settings, platform overrides |
| `istioversion` | Version resolution from `versions.yaml`: aliases (e.g. `v1.30-latest` -> `v1.30.1`), EOL tracking |
| `revision` | Revision lifecycle: dependency tracking, old revision pruning, workload migration |
| `reconcile` | Component-specific reconciliation logic for istiod, CNI, and ZTunnel |
| `reconciler` | Generic `StandardReconciler[T]` with finalizer support, used by all controllers |
| `config` | Operator configuration loading, platform detection, TLS profile management |
| `kube` | Kubernetes utilities: finalizer helpers, status patching, resource key formatting |
| `validation` | CRD validation helpers |
| `predicate` | Event filtering predicates for controller watches |
| `watches` | Watch configuration helpers for controllers |
| `scheme` | Kubernetes scheme registration (API types, OpenShift types) |
| `enqueuelogger` | Debug logging wrapper for reconciliation queue events |
| `constants` | Shared constants (label values, annotation keys) |
| `converter` | Configuration conversion utilities |
| `env` | Environment variable reading helpers |
| `errlist` | Error list aggregation |
| `version` | Build-time version information |
| `test` | Shared test utilities |

## Data Flow

### Values Pipeline

When an `IstioRevision` is reconciled, values are assembled through a pipeline
before being passed to Helm:

```
Chart defaults (values.yaml from embedded chart)
        │
        ▼
Profile overlay (default, openshift, demo, ambient, etc.)
        │
        ▼
Image digest injection (from config.properties)
        │
        ▼
User-provided overrides (Istio CR spec.values)
        │
        ▼
Platform-specific settings (OpenShift adjustments)
        │
        ▼
FIPS / TLS profile settings (cipher suites, min TLS version)
        │
        ▼
Final merged values ──▶ Helm install/upgrade
```

### Update Strategies

The `Istio` controller supports two update strategies:

- **InPlace**: Directly updates the existing `IstioRevision` in place
- **RevisionBased**: Creates a new `IstioRevision` alongside the old one,
  uses `IstioRevisionTag` to shift traffic, then prunes old revisions
  after a configurable grace period

## Key Technologies

| Technology | Version | Role |
|---|---|---|
| Go | 1.24+ | Implementation language |
| controller-runtime | v0.24.1 | Kubernetes operator framework |
| Helm | v4.2.0 | Chart-based Istio deployment |
| Istio | v1.28 - v1.31 | Service mesh (managed by this operator) |
| Kubebuilder | - | Project scaffolding and code generation |
| Ginkgo / Gomega | v2.28.1 | BDD testing framework |
| OpenShift API | - | Platform-specific integration (optional) |

## Embedded Resources

Istio Helm charts and profiles are compiled into the operator binary via Go's
`embed.FS` (`resources/resources.go`). Each supported Istio version has its own
directory under `resources/`:

```
resources/
├── resources.go          # //go:embed directive
├── v1.28.8/charts/       # base, cni, gateway, istiod, revisiontags, ztunnel
├── v1.29.x/charts/
├── v1.30.x/charts/
└── v1.31.0-alpha/charts/
```

At runtime, the `--resource-directory` flag can override embedded charts with
a filesystem path, which is useful for development.

## Development and Testing

### Testing Layers

- **Unit tests** (`*_test.go` throughout the codebase): Standard Go testing,
  no cluster required. Run with `make test`.
- **Integration tests** (`tests/integration/`): Use controller-runtime's envtest
  to run against a local API server. Run with `make test.integration`.
- **E2E tests** (`tests/e2e/`): Full cluster tests using Ginkgo, covering
  ambient mode, control plane lifecycle, multicluster, gateway API, cert-manager,
  and more. Run with `make test.e2e.kind` (KIND) or `make test.e2e.ocp` (OpenShift).

### Code Generation

Generated artifacts are produced by `make gen` (or specific sub-targets):

| Generator | Output |
|---|---|
| controller-gen | CRD manifests, RBAC manifests, DeepCopy methods (`zz_generated.deepcopy.go`) |
| Custom tooling | `values_types.gen.go` (Istio Helm values schema from `istio.io/api`) |
| operator-sdk | OLM bundle in `bundle/` |
| crd-ref-docs | API reference documentation |
| `hack/download-charts.sh` | Embedded charts under `resources/` |

## Deployment

The operator is deployed via its own Helm chart (`chart/`):

```
chart/
├── Chart.yaml
├── crds/                 # Operator CRDs (Istio, IstioRevision, etc.)
├── templates/            # Deployment, ServiceAccount, RBAC, etc.
└── values.yaml
```

It can also be deployed as an OLM-managed operator using the bundle in `bundle/`.

Key environment variables:
- `HUB` / `TAG`: Container image registry and tag
- `POD_NAMESPACE`: Operator namespace (auto-detected from service account)
- `HELM_DRIVER`: Helm storage driver override
- `VERSIONS_YAML_FILE`: Custom versions file for downstream vendors

## Security Considerations

- Metrics endpoint serves over HTTPS with authentication and authorization filters
- HTTP/2 is disabled on the metrics server to mitigate stream cancellation CVEs
- On OpenShift, the operator respects the cluster-wide TLS security profile
  (cipher suites, minimum TLS version) and restarts on profile changes
- Leader election prevents multiple active instances
- All commits require signing (`-s` flag)
- Gitleaks pre-commit hook scans for secrets

## Platform Abstraction

The operator detects the platform at startup (`pkg/config/platform.go`):

- **Kubernetes**: Uses the `default` Helm profile
- **OpenShift**: Uses the `openshift` Helm profile, fetches TLS configuration
  from the cluster's `APIServer` resource, and integrates with OpenShift-specific
  APIs (`github.com/openshift/api`, `github.com/openshift/library-go`)

Vendor-specific customization is configuration-driven (no code forks):
custom `versions.yaml` files and `vendor_defaults.yaml` for Helm value overrides.
