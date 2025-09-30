# Sail Operator Agent Instructions

This document provides AI coding agents with project-specific context for the Sail Operator, which manages the lifecycle of Istio control planes on Kubernetes.

## Project Overview

The Sail Operator is a Kubernetes operator built to manage Istio service mesh deployments. It provides custom resources (`Istio`, `IstioRevision`, `IstioCNI`, `ZTunnel`) to deploy and manage control plane components.

## Setup Commands

```bash
# Install CRDs into the cluster
make install

# Deploy operator to cluster
make deploy

# Run operator locally (for development)
make run

# Create local KIND cluster with operator
make cluster

# Create multi-cluster setup
make cluster MULTICLUSTER=true
```

## Development Workflow

```bash
# Run unit tests
make test

# Run integration tests
make test.integration

# Run e2e tests on KIND
make test.e2e.kind

# Run e2e tests on OpenShift
make test.e2e.ocp

# Lint and format code
make lint

# Build operator image
make build

# macOS with Podman (see docs/macos/develop-on-macos.adoc)
CONTAINER_CLI=podman make build
CONTAINER_CLI=podman DOCKER_GID=0 make deploy
```

## Code Style and Conventions

- **Language**: Go 1.24+ with modules
- **Framework**: Kubebuilder with controller-runtime
- **Testing**: Ginkgo/Gomega for E2E/integration tests, standard Go testing for unit tests
- **Commit signing**: Required with `-s` flag
- **API changes**: Must be discussed in SEP (Sail Enhancement Proposal) first

## Key Directories

- `api/` - Custom Resource Definitions (CRDs)
- `pkg/` - Core business logic packages
- `controllers/` - Kubernetes controllers
- `tests/integration/` - Integration test suites
- `tests/e2e/` - End-to-end test suites
- `chart/` - Helm charts and samples
- `enhancements/` - SEP (Sail Enhancement Proposal) documents
- `docs/` - Documentation files
- `resources/` - Resource manifests for each supported Istio version
- `hack/` - Development scripts and tools

## Custom Resources

### Primary Resources
- **Istio**: Main resource representing a control plane
- **IstioRevision**: Represents a specific control plane deployment
- **IstioRevisionTag**: Tags for managing active revisions
- **IstioCNI**: CNI plugin configuration (required for OpenShift/Ambient)
- **ZTunnel**: Ambient mesh tunnel configuration

### Resource Relationships
- `Istio` creates and manages `IstioRevision`
- `IstioRevisionTag` points to a `Istio` resource
- Ambient mode requires `Istio` + `ZTunnel` (+ `IstioCNI` on OpenShift)
- Sidecar mode requires `Istio` (+ `IstioCNI` on OpenShift)

## Key Configuration Files

- `pkg/istioversion/versions.yaml` - Supported Istio versions (configurable via VERSIONS_YAML_FILE)
- `pkg/istiovalues/vendor_defaults.yaml` - Vendor-specific Helm value defaults
- `go.mod` - Go dependencies and replacements

## Testing Patterns

- **Unit tests**: Use standard Go testing, avoid Kubernetes clients when possible
- **Integration tests**: Use Ginkgo/Gomega with envtest
- **E2E tests**: Use Ginkgo/Gomega against real clusters
- **Test isolation**: Keep business logic in testable packages separate from controllers

## Common Operations

### Adding New API Fields
1. Modify types in `api/`
2. Run `make gen` to generate CRDs
3. Update controllers in `controllers/`
4. Add tests
5. Create SEP for significant changes

### Debugging
- Use `make run` to run operator locally
- Check controller logs for reconciliation issues
- Use `kubectl describe` on custom resources for events

## Vendor Considerations

The project supports downstream vendors with custom configurations:
- Vendor-specific changes should be configuration-based, not code changes
- Use `VERSIONS_YAML_FILE` environment variable for custom version files
- Modify `vendor_defaults.yaml` for vendor-specific Helm defaults

## Versioning Policy

- Sail Operator versions follow Istio versioning
- Supports n-2 Istio releases (e.g., Sail 1.27 supports Istio 1.25-1.27)
- Not all Istio patch versions are included in Sail releases

## Security

- Never commit secrets or credentials
- Use gitleaks pre-commit hook for secret scanning
- Follow Kubernetes security best practices
- Sign all commits with `-s` flag
- Add comments to all git commits (e.g., -m "Fix typo in README" -m "Fixes #123")

## Troubleshooting

- **CRD issues**: Ensure `make install` was run
- **Image pull errors**: Verify HUB/TAG environment variables
- **Local development**: Use `export HUB=localhost:5000` for KIND clusters and KIND registry
- **Controller errors**: Check logs and resource events
- **macOS with Podman**: See `docs/macos/develop-on-macos.adoc` for platform-specific guidance

## Integration with Istio

The operator deploys Istio using Helm charts and follows Istio's configuration patterns:
- Uses official Istio Helm charts
- Supports all standard Istio configuration via `values` field
- Manages Istio lifecycle (install, upgrade, uninstall)
- Handles revision-based upgrades

## Domain Knowledge

For detailed technical knowledge about specific areas of the Sail Operator, refer to these domain-specific documents:

### API and Resource Management
- **[API Types and CRDs](.github/ai_agents/knowledge/domain-knowledge-api-types.md)** - Detailed knowledge about Custom Resource Definitions, API types, validation rules, and resource relationships
- **[Controllers Architecture](.github/ai_agents/knowledge/domain-knowledge-controllers.md)** - Controller reconciliation patterns, error handling, debugging, and inter-controller communication

### Development and Operations
- **[Helm Integration](.github/ai_agents/knowledge/domain-knowledge-helm-integration.md)** - Chart management, values processing, platform customization, and troubleshooting
- **[Testing Framework](.github/ai_agents/knowledge/domain-knowledge-testing-framework.md)** - Unit/integration/E2E testing methodologies, utilities, and best practices
- **[Version Management](.github/ai_agents/knowledge/domain-knowledge-version-management.md)** - Version compatibility, upgrade strategies, chart management, and troubleshooting

### Creating New Domain Knowledge
- **[Domain Knowledge Creation Guide](.github/ai_agents/domain_knowledge_prompt.md)** - Template and guidelines for creating new domain knowledge files

Each domain knowledge file provides deep, technical details that complement the high-level guidance in this document. Use them for specific implementation questions and detailed understanding of system behavior.