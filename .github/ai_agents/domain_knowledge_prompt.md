# Create Domain Knowledge File

This guide outlines best practices for creating domain knowledge files in the `.github/ai_agents/knowledge/` folder. These files serve as a source of truth for understanding the Sail Operator architecture, patterns, and implementation details.

## Naming Convention

- Use kebab-case for filenames: `domain-knowledge-component-name.md`
- Use descriptive names that clearly indicate the content: `domain-knowledge-api-types.md`, `domain-knowledge-controllers.md`, etc.
- Prefix all files with `domain-knowledge-` for consistency

## Structure

Each domain knowledge file should follow this structure:

1. **Title**: Clear, descriptive title at the top using H1 (`# Title`)
2. **Introduction**: Brief overview of what the document explains
3. **Conceptual Overview**: High-level explanation of the concept or component
4. **Implementation Architecture**: Key components and their relationships
5. **Code Implementation**: Relevant code patterns with examples
6. **Key Interfaces/Models**: Important data structures, interfaces, or protocols
7. **Example Use Cases**: Real-world examples of how the component is used
8. **Best Practices**: Guidelines for working with this component
9. **Related Components**: Links to other domain knowledge files that relate to this one

## Content Guidelines

### 1. Technical Depth

- Balance high-level concepts with implementation details
- Include enough information for new developers to understand the system
- Reference specific file paths for key implementations
- Show actual code examples, not just descriptions

### 2. Code Examples

- Include small, focused code snippets that illustrate key patterns
- Reference file paths above each code example
- Use Go syntax highlighting for Go code examples
- Keep examples concise, focusing on the most important patterns

Example:
```go
// From controllers/istio/istio_controller.go
func (r *IstioReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    log := r.Log.WithValues("istio", req.NamespacedName)

    // Fetch the Istio instance
    istio := &v1.Istio{}
    if err := r.Get(ctx, req.NamespacedName, istio); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }

    return r.reconcile(ctx, istio)
}
```

### 3. Component Relationships

- Clearly explain how components interact with each other
- Use numbered lists or flow descriptions to show sequences
- Explain communication patterns between services

### 4. File References

- Include complete file paths to make it easy to find implementations
- Group related files by functionality or component
- List both interface/contract files and implementations

Example:
```
Sail Operator Istio Custom Resources:
- API Definitions: api/v1/istio_types.go
- Controller Implementation: controllers/istio/istio_controller.go
- Status Management: controllers/istio/istio_status.go
- Helm Integration: pkg/helm/istio_installer.go
- Version Management: pkg/istioversion/version_resolver.go
```

### 5. Best Practices

- Include best practices specific to the component
- Highlight common pitfalls and how to avoid them
- Show code examples of proper usage patterns

## Required Sections for Common Component Types

### API Components (Custom Resources)

- Resource specification structure
- Status field definitions
- Validation patterns
- Controller reconciliation flow
- Example resource manifests

### Controller Components

- Reconciliation patterns
- Error handling strategies
- Status update patterns
- Event recording
- Example controller implementations

### Testing Components

- Test structure patterns
- Mock/fake usage
- Integration test setup
- E2E test scenarios
- Platform-specific considerations (macOS/Podman)

### Helm Integration

- Chart management patterns
- Values processing
- Version compatibility
- Platform customization
- Troubleshooting approaches

### Version Management

- Version resolution logic
- Compatibility matrices
- Upgrade strategies
- Chart selection
- Validation patterns

## Example Domain Knowledge File Structure

```markdown
# Sail Operator [Component Name] Domain Knowledge

Brief introduction explaining the purpose and importance of this component in the Sail Operator.

## Overview

High-level conceptual explanation of how this component works within the Sail Operator ecosystem.

## Architecture

How this component fits into the Sail Operator system:
- Relationship to Istio control plane management
- Integration with Kubernetes operator patterns
- Connection to other Sail Operator components

## Implementation Details

### Key Patterns

Important implementation patterns specific to Sail Operator with code examples.

### Component Flow

Step-by-step explanation of how data/requests flow through the component:
1. User creates/updates Custom Resource
2. Controller receives reconciliation request
3. Component processes the change
4. Istio components are updated/configured
5. Status is reported back

## Key Files

List of critical files with their purposes:
- API definitions in `api/v1/`
- Controller implementations in `controllers/`
- Business logic in `pkg/`
- Tests in `tests/`

## Examples

### Basic Usage
```yaml
apiVersion: sailoperator.io/v1
kind: [ResourceType]
metadata:
  name: example
spec:
  # Component-specific configuration
```

### Advanced Configuration
Real-world examples showing complex scenarios.

## Testing Patterns

### Unit Tests
- Location: `[component]_test.go` files
- Framework: Standard Go testing
- Patterns: Table-driven tests for business logic

### Integration Tests
- Location: `tests/integration/`
- Framework: Ginkgo + Gomega + envtest
- Focus: Controller behavior with fake Kubernetes API

### E2E Tests
- Location: `tests/e2e/`
- Framework: Ginkgo + Gomega + real clusters
- Scenarios: Complete workflow validation

## Best Practices

Guidelines for working with this component effectively:
- Follow Kubernetes operator patterns
- Use controller-runtime best practices
- Implement proper error handling and status reporting
- Ensure compatibility with multiple Istio versions

## Platform Considerations

### macOS Development
- Reference `docs/macos/develop-on-macos.adoc` for Podman setup
- Use `CONTAINER_CLI=podman` for container operations
- Consider architecture differences (ARM64 vs AMD64)

### OpenShift
- Special considerations for OpenShift deployments
- Security context constraints
- Network policy requirements

### Multi-Platform Testing
- KIND cluster setup variations
- Container runtime differences
- Architecture-specific image handling

## Troubleshooting

Common issues and solutions:
- Controller reconciliation failures
- Resource status not updating
- Platform-specific deployment issues

## Related Components

Links to related Sail Operator domain knowledge files:
- [API Types](domain-knowledge-api-types.md)
- [Controllers](domain-knowledge-controllers.md)
- [Testing Framework](domain-knowledge-testing-framework.md)
- [Version Management](domain-knowledge-version-management.md)
```

## Maintenance

- Update domain knowledge files when Sail Operator components change significantly
- Keep code examples current with the latest Sail Operator implementation patterns
- Add new domain knowledge files when new Sail Operator features are added
- Review and update Istio version compatibility information regularly
- Update platform-specific guidance when new platforms are supported

## Sail Operator Specific Considerations

When creating domain knowledge files, include these Sail Operator-specific elements:

### Istio Integration
- Explain how the component relates to Istio control plane management
- Document Helm chart integration patterns
- Reference Istio version compatibility requirements
- Include examples of Istio configuration values

### Kubernetes Operator Patterns
- Follow controller-runtime best practices
- Document Custom Resource specifications
- Explain status field usage and condition patterns
- Include proper RBAC considerations

### Multi-Version Support
- Document how components handle multiple Istio versions
- Explain version resolution and compatibility checking
- Include upgrade and downgrade scenarios

### Platform Support
- **macOS Development**: Reference `docs/macos/develop-on-macos.adoc` for Podman setup
- **OpenShift**: Document OCP-specific requirements (CNI, SCC, etc.)
- **KIND/minikube**: Include local development cluster considerations
- **Multi-architecture**: ARM64 vs AMD64 deployment differences

### Testing Integration
- Reference Sail Operator testing framework patterns
- Include examples from `tests/integration/` and `tests/e2e/`
- Document test environment setup (envtest, KIND, OCP)
- Explain container runtime alternatives (Docker vs Podman)

### Development Workflow
- Reference Sail Operator Makefile targets
- Include examples of local development workflows
- Document debugging approaches specific to operator development
- Explain SEP (Sail Enhancement Proposal) process for API changes