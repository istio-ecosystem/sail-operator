# Sail Operator Helm Integration Domain Knowledge

This document provides AI agents with detailed knowledge about how the Sail Operator integrates with Istio Helm charts for deployment and configuration management.

## Helm Integration Overview

The Sail Operator uses Istio's official Helm charts to deploy and manage Istio components. Instead of embedding charts, it dynamically downloads them from Istio's release repositories.

## Chart Management

### Chart Discovery
Charts are downloaded from Istio's GitHub releases:
- **Base Chart**: `istio/manifests/charts/base`
- **Istiod Chart**: `istio/manifests/charts/istio-control/istio-discovery`
- **CNI Chart**: `istio/manifests/charts/istio-cni`
- **ZTunnel Chart**: `istio/manifests/charts/ztunnel`
- **Gateway Charts**: `istio/manifests/charts/gateways/*`

### Chart Caching
- Charts are cached locally to avoid repeated downloads
- Cache invalidated when Istio version changes
- Supports air-gapped environments with pre-cached charts

## Values Processing

### Values Hierarchy
The operator merges values from multiple sources (highest to lowest priority):

1. **User-provided values** - `spec.values` in custom resources
2. **Vendor defaults** - `pkg/istiovalues/vendor_defaults.yaml`
3. **Operator defaults** - Built-in safe defaults
4. **Chart defaults** - Default values from Helm charts

### Values Transformation
Values undergo several transformations:

```go
// Example transformation pipeline
rawValues := getUserValues(resource.Spec.Values)
vendorDefaults := loadVendorDefaults()
operatorDefaults := getOperatorDefaults()

// Merge with precedence
finalValues := merge(operatorDefaults, vendorDefaults, rawValues)

// Apply transformations
transformedValues := applyTransformations(finalValues, resource)
```

### Common Value Transformations
- **Namespace injection** - Automatically set target namespaces
- **Revision naming** - Generate consistent revision names
- **Resource limits** - Apply operator-managed resource constraints
- **Security contexts** - Inject security policies for specific platforms

## Helm Release Management

### Release Naming Convention
- **Istio**: `istio-<revision-name>`
- **CNI**: `istio-cni-<resource-name>`
- **ZTunnel**: `ztunnel-<resource-name>`

### Release Lifecycle
1. **Pre-install validation** - Validate values and dependencies
2. **Install/Upgrade** - Deploy charts using Helm SDK
3. **Post-install verification** - Verify component readiness
4. **Status tracking** - Monitor release health
5. **Cleanup** - Remove releases when resources are deleted

### Helm SDK Usage
The operator uses Helm's Go SDK for chart operations:

```go
// Example Helm install
cfg := &action.Configuration{}
client := action.NewInstall(cfg)
client.ReleaseName = releaseName
client.Namespace = targetNamespace
client.CreateNamespace = true

release, err := client.Run(chart, values)
```

## Chart Customization

### Platform-Specific Adjustments
Charts are customized for different Kubernetes platforms:

#### OpenShift
- **Security Context Constraints** - Apply OpenShift SCC policies
- **Route vs Ingress** - Use OpenShift Routes instead of Ingress
- **Network Policies** - Apply OpenShift-specific network policies

#### GKE/EKS/AKS
- **Cloud provider integrations** - Configure load balancers
- **Node selectors** - Platform-specific node targeting
- **Storage classes** - Use cloud-native storage

### Resource Customization
Common resource customizations:

```yaml
# Resource limits and requests
resources:
  limits:
    cpu: 500m
    memory: 2Gi
  requests:
    cpu: 100m
    memory: 128Mi

# Node affinity and tolerations
nodeSelector:
  node-role.kubernetes.io/control-plane: ""
tolerations:
- key: CriticalAddonsOnly
  operator: Exists
```

## Values Schema Validation

### Generated Types
The `api/v1/values_types.gen.go` file contains Go types generated from Istio's Helm values schema, providing:
- **Type safety** - Compile-time validation of configuration
- **IntelliSense** - IDE support for configuration
- **Documentation** - Embedded field documentation

### Runtime Validation
Values are validated at runtime:
1. **Schema validation** - Against Istio's JSON schema
2. **Compatibility checks** - Version-specific validation
3. **Platform validation** - Platform-specific requirements
4. **Security validation** - Security policy compliance

## Common Configuration Patterns

### Basic Control Plane
```yaml
spec:
  values:
    global:
      meshID: mesh1
      network: network1
    pilot:
      resources:
        requests:
          cpu: 100m
          memory: 128Mi
```

### Multi-Cluster Configuration
```yaml
spec:
  values:
    global:
      meshID: mesh1
      network: network1
      remotePilotAddress: pilot.istio-system.svc.cluster.local
    istiodRemote:
      enabled: true
```

### Ambient Mesh Configuration
```yaml
# Istio
spec:
  values:
    pilot:
      env:
        PILOT_ENABLE_AMBIENT: true
    global:
      waypoint:
        affinity: {}

# CNI
spec:
  values:
    cni:
      ambient:
        enabled: true

# ZTunnel
spec:
  values:
    ztunnel:
      image: istio/ztunnel
```

### Production Hardening
```yaml
spec:
  values:
    global:
      proxy:
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 2000m
            memory: 1024Mi
    pilot:
      resources:
        requests:
          cpu: 500m
          memory: 2048Mi
      env:
        PILOT_ENABLE_WORKLOAD_ENTRY_AUTOREGISTRATION: false
        PILOT_ENABLE_CROSS_CLUSTER_WORKLOAD_ENTRY: false
```

## Troubleshooting Helm Integration

### Common Issues
1. **Chart download failures** - Network connectivity or version issues
2. **Values validation errors** - Schema violations or incompatible values
3. **Release conflicts** - Conflicting Helm releases in same namespace
4. **Resource quotas** - Insufficient cluster resources

### Debugging Commands
```bash
# Check Helm releases
helm list -A | grep istio

# Debug release status
helm status istio-base -n istio-system

# Get release values
helm get values istio-base -n istio-system

# Check release history
helm history istio-base -n istio-system
```

### Values Debugging
```bash
# Export effective values
kubectl get istio default -o jsonpath='{.spec.values}' | jq .

# Check operator logs for values processing
kubectl logs -n sail-operator deployment/sail-operator-controller | grep values
```

## Version Compatibility

### Supported Versions
The operator maintains compatibility with multiple Istio versions:
- Defined in `pkg/istioversion/versions.yaml`
- Supports n-2 versioning policy
- Automatic compatibility validation

### Chart Compatibility Matrix
Different Istio versions may have breaking changes in chart structure:
- **Values schema changes** - New/removed configuration options
- **Resource definitions** - Changed Kubernetes resources
- **Default behaviors** - Modified default configurations

The operator handles these through version-specific transformation logic.