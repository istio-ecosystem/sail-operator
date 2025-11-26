# Sail Operator Version Management Domain Knowledge

This document provides AI agents with detailed knowledge about how the Sail Operator manages Istio versions, compatibility, and upgrade strategies.

## Version Management Architecture

The Sail Operator follows Istio's versioning scheme and maintains compatibility with multiple Istio versions simultaneously.

## Version Configuration

### Primary Version File
**Location**: `pkg/istioversion/versions.yaml`

This file defines all supported Istio versions:

```yaml
versions:
  # Alias pointing to a specific version (the first entry is the default)
  - name: v1.X-latest
    ref: v1.X.Y

  # Full version definition
  - name: v1.X.Y
    version: 1.X.Y
    repo: https://github.com/istio/istio
    commit: 1.X.Y
    charts:
      - https://istio-release.storage.googleapis.com/charts/base-1.X.Y.tgz
      - https://istio-release.storage.googleapis.com/charts/istiod-1.X.Y.tgz
      - https://istio-release.storage.googleapis.com/charts/gateway-1.X.Y.tgz
      - https://istio-release.storage.googleapis.com/charts/cni-1.X.Y.tgz
      - https://istio-release.storage.googleapis.com/charts/ztunnel-1.X.Y.tgz

  # End-of-life version (still valid input, but not installable)
  - name: v1.W-latest
    ref: v1.W.Z
    eol: true
  - name: v1.W.Z
    eol: true
```

**Note:** Check `pkg/istioversion/versions.yaml` in the repository for the current list of supported versions.

### Version Entry Structure
- **name** - Human-readable version identifier (e.g., `v1.X.Y`, `v1.X-latest`)
- **ref** - Reference to another version (for alias entries like `v1.X-latest`)
- **version** - Semantic version (x.y.z)
- **repo** - Source repository URL
- **commit** - Git commit/tag reference
- **branch** - Git branch (for development versions like `master`)
- **charts** - List of Helm chart URLs for this version
- **eol** - Boolean indicating end-of-life (version remains valid but not installable)

### Vendor Customization
The version file can be customized using the `VERSIONS_YAML_FILE` environment variable:

```bash
export VERSIONS_YAML_FILE=custom_versions.yaml
```

This allows downstream vendors to:
- Define custom version sets
- Point to different chart repositories
- Support enterprise or patched Istio versions

## Versioning Policy

### Supported Version Range
- **Current policy**: n-2 versions (the operator supports the current and two previous minor Istio versions)
- **Version alignment**: Operator version matches latest supported Istio version
- **Patch version handling**: Not all Istio patch versions are included
- **EOL versions**: Versions can be marked with `eol: true` to keep them as valid input but not installable

### Version Lifecycle
1. **New Istio release** - Add to versions.yaml with charts
2. **Operator release** - Create corresponding operator version
3. **Deprecation** - Remove oldest version when adding new one
4. **End of support** - Remove from versions.yaml

## Default Version Selection

### Automatic Version Selection
When no version is specified in custom resources:

```go
// Default version selection logic
func getDefaultVersion() string {
    versions := loadSupportedVersions()

    // Find latest stable version
    for _, version := range versions {
        if strings.Contains(version.Name, "latest") {
            return version.Version
        }
    }

    // Fallback to newest version
    return versions[0].Version
}
```

### Version Resolution
The operator resolves versions in this order:
1. **Explicit version** - User-specified in `spec.version`
2. **Default version** - Latest stable from versions.yaml
3. **Fallback** - Operator's built-in default

## Chart Management

### Chart Discovery
Charts are downloaded from URLs defined in versions.yaml:

```go
// Chart download logic
func downloadChart(chartURL string) (*chart.Chart, error) {
    // Check local cache first
    if cached := checkCache(chartURL); cached != nil {
        return cached, nil
    }

    // Download and cache
    chartData, err := httpClient.Get(chartURL)
    if err != nil {
        return nil, err
    }

    chart, err := loader.LoadArchive(chartData)
    if err != nil {
        return nil, err
    }

    cacheChart(chartURL, chart)
    return chart, nil
}
```

### Chart Caching Strategy
- **Local cache** - Charts cached by URL in operator filesystem
- **Cache invalidation** - Cache cleared when versions.yaml changes
- **Air-gapped support** - Pre-populate cache for offline environments

### Chart Validation
Charts undergo validation before use:
- **Schema validation** - Verify chart structure
- **Version compatibility** - Ensure chart matches declared version
- **Dependency checks** - Validate chart dependencies

## Version Compatibility Matrix

### Component Compatibility
Different Istio components have specific compatibility requirements:

| Component | Version Source | Compatibility Notes |
|-----------|---------------|-------------------|
| **Istiod** | Istio version | Core control plane |
| **CNI** | Same as Istio | Must match Istio version |
| **ZTunnel** | Same as Istio | Ambient mesh only |
| **Gateways** | Same as Istio | Optional components |

### Cross-Version Support
The operator supports mixed versions in specific scenarios:
- **Canary deployments** - Multiple Istio revisions
- **Rolling upgrades** - Temporary version coexistence
- **Multi-cluster** - Different versions across clusters

## Upgrade Strategies

### InPlace Upgrade Strategy
Default upgrade method with brief downtime:

```yaml
spec:
  updateStrategy:
    type: InPlace
```

**Process**:
1. Update existing IstioRevision with new version
2. Helm upgrade control plane components
3. Restart workloads if necessary
4. Validate new version health

### RevisionBased Upgrade Strategy
Zero-downtime upgrades using canary deployments:

```yaml
spec:
  updateStrategy:
    type: RevisionBased
```

**Process**:
1. Create new IstioRevision with new version
2. Deploy new control plane alongside existing
3. Use IstioRevisionTag for traffic shifting
4. Remove old revision when upgrade complete

### Upgrade Validation
Before performing upgrades:

```go
func validateUpgrade(fromVersion, toVersion string) error {
    // Check version compatibility
    if !isCompatibleUpgrade(fromVersion, toVersion) {
        return errors.New("incompatible version upgrade")
    }

    // Validate breaking changes
    if hasBreakingChanges(fromVersion, toVersion) {
        return errors.New("breaking changes detected")
    }

    // Check operator support
    if !isSupportedVersion(toVersion) {
        return errors.New("target version not supported")
    }

    return nil
}
```

## Version-Specific Behaviors

### Feature Flags
Different Istio versions may require different configurations:

```go
// Version-specific configuration
func getVersionSpecificConfig(version string) map[string]interface{} {
    config := make(map[string]interface{})

    // Ambient mesh support (1.25+)
    if semver.Compare(version, "1.25.0") >= 0 {
        config["pilot.env.PILOT_ENABLE_AMBIENT"] = true
    }

    // New telemetry API (1.26+)
    if semver.Compare(version, "1.26.0") >= 0 {
        config["telemetry.v2.enabled"] = true
    }

    return config
}
```

### Deprecation Handling
The operator handles deprecated features across versions:

```go
func transformValuesForVersion(values map[string]interface{}, version string) {
    // Remove deprecated fields for newer versions
    if semver.Compare(version, "1.26.0") >= 0 {
        delete(values, "global.legacy")
    }

    // Add required fields for older versions
    if semver.Compare(version, "1.25.0") < 0 {
        values["global.legacy"] = true
    }
}
```

## Version Status and Reporting

### Version Information in Status
Custom resources report version information:

```yaml
status:
  observedGeneration: 1
  state: Healthy
  activeRevisionName: default-1-26-0
  revisions:
  - name: default-1-26-0
    version: 1.26.0
    state: Healthy
  conditions:
  - type: Ready
    status: "True"
    reason: InstallationComplete
```

### Version Events
The operator generates events for version-related activities:

```go
func recordVersionEvent(obj runtime.Object, eventType, reason, message string) {
    r.EventRecorder.Event(obj, eventType, reason, message)
}

// Example usage
recordVersionEvent(istio, "Normal", "VersionUpgrade",
    fmt.Sprintf("Upgrading from %s to %s", oldVersion, newVersion))
```

## Troubleshooting Version Issues

### Common Version Problems
1. **Unsupported version** - Version not in versions.yaml
2. **Chart download failure** - Network issues or invalid URLs
3. **Version mismatch** - Operator/Istio version compatibility
4. **Upgrade incompatibility** - Breaking changes between versions

### Version Debugging Commands

```bash
# Check supported versions
kubectl get cm -n sail-operator sail-operator-versions -o yaml

# Verify current versions
kubectl get istio -o jsonpath='{.items[*].spec.version}'

# Check version-specific events
kubectl events --for istio/default --types Warning,Normal

# Validate chart availability
curl -I https://istio-release.storage.googleapis.com/charts/istiod-1.26.0.tgz
```

### Version Status Inspection

```bash
# Check operator's version awareness
kubectl logs -n sail-operator deployment/sail-operator | grep version

# Verify chart cache
kubectl exec -n sail-operator deployment/sail-operator -- ls /tmp/charts/

# Check version resolution
kubectl describe istio default | grep "Version:"
```

## Custom Version Development

### Adding New Versions
1. **Update versions.yaml** - Add new version entry with charts
2. **Test compatibility** - Verify operator works with new version
3. **Update defaults** - Modify vendor_defaults.yaml if needed
4. **Test upgrades** - Validate upgrade paths from previous versions

### Version Testing
```yaml
# Custom test versions.yaml
versions:
  - name: v1.27-dev
    version: 1.27.0-dev
    repo: https://github.com/istio/istio
    commit: main
    charts:
      - https://storage.googleapis.com/istio-build/dev/charts/istiod-1.27.0-dev.tgz
```

### Development Workflow
```bash
# Test with custom versions
VERSIONS_YAML_FILE=test_versions.yaml make test.e2e.kind

# Validate version support
make test.integration GINKGO_FLAGS="--focus=Version"
```