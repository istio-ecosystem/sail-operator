# Reproduce Issue

Generate a comprehensive markdown guide for reproducing a reported issue. The output is a self-contained document that allows contributors to reproduce the issue step-by-step without cloning additional repositories.

## Arguments

- `url` (required): GitHub issue URL (e.g., `https://github.com/istio-ecosystem/sail-operator/issues/1234`)
- `provider` (optional): Installation method - `sail-operator` (default), `helm`, or `istioctl`
- `platform` (optional): Target platform - `kind` (default) or `custom`

## Steps

### 1. Parse Issue Information

Extract issue details from the provided URL:

```bash
# Extract issue number from URL
# Example: https://github.com/istio-ecosystem/sail-operator/issues/1234 -> 1234

# Fetch issue details
gh issue view 1234 --json title,body,labels,url
```

Parse the issue to identify:
- Istio version mentioned
- Deployment profile (sidecar/ambient)
- Multi-cluster requirements
- Platform requirements (OpenShift, KIND, etc.)
- Reproduction steps
- Expected vs actual behavior

Display summary:
```
🔍 Generating reproduction guide for Issue #1234
Title: [issue title]
URL: [issue url]
Provider: sail-operator
Platform: KIND
```

### 2. Determine Configuration

Based on parsed information and arguments, determine:
- **Istio version**: From issue body or default to latest supported
- **Profile**: From issue body or default to sidecar
- **Multi-cluster**: Whether the issue requires multiple clusters
- **Platform**: KIND (with LB) or custom cluster
- **Provider**: sail-operator, helm, or istioctl

Display configuration:
```
📋 Configuration
- Istio Version: v1.24.0
- Profile: sidecar
- Multi-cluster: false
- Platform: KIND
- Provider: sail-operator
```

### 3. Generate Markdown Document

Create a markdown file named `reproduce-issue-1234.md` with the following structure:

#### Header Section

```markdown
# Reproduce Issue #1234: [Issue Title]

**Issue URL**: [issue url]
**Istio Version**: v1.24.0
**Profile**: sidecar
**Provider**: sail-operator
**Platform**: KIND
**Generated**: [timestamp]

## Overview

[Brief description from issue body]

## Expected Behavior

[From issue body]

## Actual Behavior

[From issue body]
```

#### Prerequisites Section

**For KIND platform:**

```markdown
## Prerequisites

### Required Tools

- Docker or Podman
- kubectl
- kind
- [Additional tools based on provider: helm, istioctl]

Verify installation:

\`\`\`bash
docker --version
kubectl version --client
kind --version
[helm version]  # if provider is helm
[istioctl version]  # if provider is istioctl
\`\`\`
```

**For custom platform:**

```markdown
## Prerequisites

### Required Tools

- kubectl configured for your cluster
- Cluster with sufficient resources (minimum 4 CPU, 8GB RAM per node)
- [Additional tools based on provider: helm, istioctl]

Verify cluster access:

\`\`\`bash
kubectl cluster-info
kubectl get nodes
\`\`\`
```

#### Installation Section

**For KIND platform (single cluster):**

```markdown
## Installation

### Setup KIND Cluster with Load Balancer

\`\`\`bash
curl -s https://raw.githubusercontent.com/istio/istio/refs/heads/release-1.26/samples/kind-lb/setupkind.sh | sh -s -- --cluster-name sail-test --ip-space 254
\`\`\`

### Configure kubectl Context

\`\`\`bash
kubectl config use-context kind-sail-test
kubectl cluster-info
\`\`\`
```

**For KIND platform (multi-cluster):**

```markdown
## Installation

### Setup Multi-Cluster KIND Environment

#### Create Clusters

\`\`\`bash
# Create east cluster
curl -s https://raw.githubusercontent.com/istio/istio/refs/heads/release-1.26/samples/kind-lb/setupkind.sh | sh -s -- --cluster-name east --ip-space 254

# Create west cluster
curl -s https://raw.githubusercontent.com/istio/istio/refs/heads/release-1.26/samples/kind-lb/setupkind.sh | sh -s -- --cluster-name west --ip-space 255
\`\`\`

#### Configure Contexts and Aliases

\`\`\`bash
# Set up kubectl aliases
alias keast="kubectl --context kind-east"
alias kwest="kubectl --context kind-west"

# Test connectivity
keast cluster-info
kwest cluster-info
\`\`\`

[If provider is helm, add:]
\`\`\`bash
# Set up helm aliases
alias heast="helm --kube-context kind-east"
alias hwest="helm --kube-context kind-west"
\`\`\`

[If provider is istioctl, add:]
\`\`\`bash
# Set up istioctl aliases
alias ieast="istioctl --context kind-east"
alias iwest="istioctl --context kind-west"
\`\`\`

#### Setup Certificates

\`\`\`bash
# Create certificates directory
mkdir -p certs
cd certs

# Download Istio certificate generation scripts
wget https://raw.githubusercontent.com/istio/istio/release-1.24/tools/certs/common.mk
wget https://raw.githubusercontent.com/istio/istio/release-1.24/tools/certs/Makefile.selfsigned.mk

# Generate root CA
make -f Makefile.selfsigned.mk \
  ROOTCA_CN="Root CA" \
  ROOTCA_ORG=Istio \
  root-ca

# Generate intermediate certificates for east cluster
make -f Makefile.selfsigned.mk \
  INTERMEDIATE_CN="East Intermediate CA" \
  INTERMEDIATE_ORG=Istio \
  east-cacerts

# Generate intermediate certificates for west cluster
make -f Makefile.selfsigned.mk \
  INTERMEDIATE_CN="West Intermediate CA" \
  INTERMEDIATE_ORG=Istio \
  west-cacerts

cd ..
\`\`\`

#### Install Certificates in Clusters

\`\`\`bash
# East cluster
keast create namespace istio-system
keast create secret generic cacerts -n istio-system \
  --from-file=certs/east/ca-cert.pem \
  --from-file=certs/east/ca-key.pem \
  --from-file=certs/east/cert-chain.pem \
  --from-file=certs/east/root-cert.pem

# West cluster
kwest create namespace istio-system
kwest create secret generic cacerts -n istio-system \
  --from-file=certs/west/ca-cert.pem \
  --from-file=certs/west/ca-key.pem \
  --from-file=certs/west/cert-chain.pem \
  --from-file=certs/west/root-cert.pem
\`\`\`
```

**Provider-specific installation:**

**If provider is sail-operator (single cluster):**

```markdown
### Install Sail Operator

\`\`\`bash
# Install Sail Operator CRDs
kubectl apply -f https://github.com/istio-ecosystem/sail-operator/releases/latest/download/install.yaml

# Wait for operator to be ready
kubectl wait --for=condition=Available deployment/sail-operator -n sail-operator --timeout=300s
\`\`\`
```

**If provider is sail-operator (multi-cluster):**

```markdown
### Install Sail Operator

\`\`\`bash
# Install on east cluster
keast apply -f https://github.com/istio-ecosystem/sail-operator/releases/latest/download/install.yaml
keast wait --for=condition=Available deployment/sail-operator -n sail-operator --timeout=300s

# Install on west cluster
kwest apply -f https://github.com/istio-ecosystem/sail-operator/releases/latest/download/install.yaml
kwest wait --for=condition=Available deployment/sail-operator -n sail-operator --timeout=300s
\`\`\`
```

**If provider is helm (single cluster):**

```markdown
### Add Istio Helm Repository

\`\`\`bash
helm repo add istio https://istio-release.storage.googleapis.com/charts
helm repo update
\`\`\`
```

**If provider is helm (multi-cluster):**

```markdown
### Add Istio Helm Repository

\`\`\`bash
helm repo add istio https://istio-release.storage.googleapis.com/charts
helm repo update

# Verify on both clusters
heast version
hwest version
\`\`\`
```

**If provider is istioctl (single cluster):**

```markdown
### Install istioctl

\`\`\`bash
# Download istioctl for the specific version
curl -L https://istio.io/downloadIstio | ISTIO_VERSION=1.24.0 sh -
export PATH=$PWD/istio-1.24.0/bin:$PATH

# Verify installation
istioctl version
\`\`\`
```

**If provider is istioctl (multi-cluster):**

```markdown
### Install istioctl

\`\`\`bash
# Download istioctl for the specific version
curl -L https://istio.io/downloadIstio | ISTIO_VERSION=1.24.0 sh -
export PATH=$PWD/istio-1.24.0/bin:$PATH

# Verify installation on both contexts
ieast version
iwest version
\`\`\`
```

#### Configuration Section

**For sail-operator provider (sidecar mode, single cluster):**

```markdown
## Configuration

### Deploy Istio Control Plane

\`\`\`bash
kubectl apply -f - <<EOF
apiVersion: sailoperator.io/v1alpha1
kind: Istio
metadata:
  name: default
  namespace: istio-system
spec:
  version: v1.24.0
  namespace: istio-system
EOF
\`\`\`

### Wait for Istio to be Ready

\`\`\`bash
kubectl wait --for=condition=Ready istio/default -n istio-system --timeout=300s
kubectl get istio -A
kubectl get istiorevision
\`\`\`
```

**For sail-operator provider (ambient mode, single cluster):**

```markdown
## Configuration

### Deploy Istio Components for Ambient Mode

\`\`\`bash
# Create required namespaces
kubectl create namespace istio-cni
kubectl create namespace ztunnel

# Deploy Istio control plane
kubectl apply -f - <<EOF
apiVersion: sailoperator.io/v1alpha1
kind: Istio
metadata:
  name: default
  namespace: istio-system
spec:
  version: v1.24.0
  namespace: istio-system
  profile: ambient
EOF
\`\`\`

\`\`\`bash
# Deploy CNI
kubectl apply -f - <<EOF
apiVersion: sailoperator.io/v1alpha1
kind: IstioCNI
metadata:
  name: default
  namespace: istio-cni
spec:
  version: v1.24.0
  namespace: istio-cni
EOF
\`\`\`

\`\`\`bash
# Deploy ZTunnel
kubectl apply -f - <<EOF
apiVersion: sailoperator.io/v1alpha1
kind: ZTunnel
metadata:
  name: default
  namespace: ztunnel
spec:
  version: v1.24.0
  namespace: ztunnel
EOF
\`\`\`

### Wait for All Components

\`\`\`bash
kubectl wait --for=condition=Ready istio/default -n istio-system --timeout=300s
kubectl wait --for=condition=Ready istiocni/default -n istio-cni --timeout=300s
kubectl wait --for=condition=Ready ztunnel/default -n ztunnel --timeout=300s
\`\`\`
```

**For sail-operator provider (multi-cluster):**

```markdown
## Configuration

### Deploy Istio on East Cluster

\`\`\`bash
keast apply -f - <<EOF
apiVersion: sailoperator.io/v1alpha1
kind: Istio
metadata:
  name: default
  namespace: istio-system
spec:
  version: v1.24.0
  namespace: istio-system
  values:
    global:
      meshID: mesh1
      multiCluster:
        clusterName: east
      network: east-network
EOF
\`\`\`

### Deploy Istio on West Cluster

\`\`\`bash
kwest apply -f - <<EOF
apiVersion: sailoperator.io/v1alpha1
kind: Istio
metadata:
  name: default
  namespace: istio-system
spec:
  version: v1.24.0
  namespace: istio-system
  values:
    global:
      meshID: mesh1
      multiCluster:
        clusterName: west
      network: west-network
EOF
\`\`\`

### Wait for Istio to be Ready

\`\`\`bash
keast wait --for=condition=Ready istio/default -n istio-system --timeout=300s
kwest wait --for=condition=Ready istio/default -n istio-system --timeout=300s
\`\`\`

### Configure Cross-Cluster Communication

\`\`\`bash
# Create remote secrets for east cluster
keast get secret istio-ca-secret -n istio-system -o yaml | \
  sed 's/namespace: istio-system/namespace: istio-system/g' | \
  kwest apply -f -

# Create remote secrets for west cluster
kwest get secret istio-ca-secret -n istio-system -o yaml | \
  sed 's/namespace: istio-system/namespace: istio-system/g' | \
  keast apply -f -

# Enable cross-cluster endpoint discovery
keast apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: istio-remote-secret-west
  namespace: istio-system
  labels:
    istio/multiCluster: "true"
type: Opaque
stringData:
  west: |
    $(kwest config view --flatten --minify -o jsonpath='{.clusters[0].cluster.server}')
EOF

kwest apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: istio-remote-secret-east
  namespace: istio-system
  labels:
    istio/multiCluster: "true"
type: Opaque
stringData:
  east: |
    $(keast config view --flatten --minify -o jsonpath='{.clusters[0].cluster.server}')
EOF
\`\`\`
```

**For helm provider (sidecar mode, single cluster):**

```markdown
## Configuration

### Install Istio Base Chart

\`\`\`bash
helm install istio-base istio/base \
  --namespace istio-system \
  --create-namespace \
  --version 1.24.0
\`\`\`

### Install Istiod

\`\`\`bash
helm install istiod istio/istiod \
  --namespace istio-system \
  --version 1.24.0 \
  --wait
\`\`\`

### Verify Installation

\`\`\`bash
kubectl get pods -n istio-system
helm list -n istio-system
\`\`\`
```

**For helm provider (ambient mode, single cluster):**

```markdown
## Configuration

### Install Istio Base Chart

\`\`\`bash
helm install istio-base istio/base \
  --namespace istio-system \
  --create-namespace \
  --version 1.24.0
\`\`\`

### Install CNI

\`\`\`bash
helm install istio-cni istio/cni \
  --namespace istio-cni \
  --create-namespace \
  --version 1.24.0 \
  --set profile=ambient \
  --wait
\`\`\`

### Install Istiod

\`\`\`bash
helm install istiod istio/istiod \
  --namespace istio-system \
  --version 1.24.0 \
  --set profile=ambient \
  --wait
\`\`\`

### Install ZTunnel

\`\`\`bash
helm install ztunnel istio/ztunnel \
  --namespace istio-system \
  --version 1.24.0 \
  --wait
\`\`\`

### Verify Installation

\`\`\`bash
kubectl get pods -n istio-system
kubectl get pods -n istio-cni
helm list -n istio-system
\`\`\`
```

**For helm provider (multi-cluster):**

```markdown
## Configuration

### Install Istio on East Cluster

\`\`\`bash
# Install base
heast install istio-base istio/base \
  --namespace istio-system \
  --create-namespace \
  --version 1.24.0

# Install istiod with multi-cluster config
heast install istiod istio/istiod \
  --namespace istio-system \
  --version 1.24.0 \
  --set global.meshID=mesh1 \
  --set global.multiCluster.clusterName=east \
  --set global.network=east-network \
  --wait
\`\`\`

### Install Istio on West Cluster

\`\`\`bash
# Install base
hwest install istio-base istio/base \
  --namespace istio-system \
  --create-namespace \
  --version 1.24.0

# Install istiod with multi-cluster config
hwest install istiod istio/istiod \
  --namespace istio-system \
  --version 1.24.0 \
  --set global.meshID=mesh1 \
  --set global.multiCluster.clusterName=west \
  --set global.network=west-network \
  --wait
\`\`\`

### Configure Cross-Cluster Communication

\`\`\`bash
# Install east-west gateway on east cluster
heast install eastwest-gateway istio/gateway \
  --namespace istio-system \
  --version 1.24.0 \
  --set service.type=LoadBalancer \
  --wait

# Install east-west gateway on west cluster
hwest install eastwest-gateway istio/gateway \
  --namespace istio-system \
  --version 1.24.0 \
  --set service.type=LoadBalancer \
  --wait

# Create remote secrets
keast get secret istio-ca-secret -n istio-system -o yaml | \
  kwest apply -f -

kwest get secret istio-ca-secret -n istio-system -o yaml | \
  keast apply -f -
\`\`\`
```

**For istioctl provider (sidecar mode, single cluster):**

```markdown
## Configuration

### Install Istio

\`\`\`bash
istioctl install --set profile=default -y
\`\`\`

### Verify Installation

\`\`\`bash
kubectl get pods -n istio-system
istioctl verify-install
\`\`\`
```

**For istioctl provider (ambient mode, single cluster):**

```markdown
## Configuration

### Install Istio with Ambient Profile

\`\`\`bash
istioctl install --set profile=ambient -y
\`\`\`

### Verify Installation

\`\`\`bash
kubectl get pods -n istio-system
kubectl get pods -n istio-cni
kubectl get daemonset -n istio-system ztunnel
istioctl verify-install
\`\`\`
```

**For istioctl provider (multi-cluster):**

```markdown
## Configuration

### Install Istio on East Cluster

\`\`\`bash
ieast install --set profile=default \
  --set values.global.meshID=mesh1 \
  --set values.global.multiCluster.clusterName=east \
  --set values.global.network=east-network \
  -y
\`\`\`

### Install Istio on West Cluster

\`\`\`bash
iwest install --set profile=default \
  --set values.global.meshID=mesh1 \
  --set values.global.multiCluster.clusterName=west \
  --set values.global.network=west-network \
  -y
\`\`\`

### Install East-West Gateways

\`\`\`bash
# Generate and apply east-west gateway manifest for east
ieast x create-remote-secret --name=east | kwest apply -f -

# Generate and apply east-west gateway manifest for west
iwest x create-remote-secret --name=west | keast apply -f -
\`\`\`

### Verify Installation

\`\`\`bash
ieast verify-install
iwest verify-install
\`\`\`
```

#### Reproduction Steps Section

```markdown
## Reproduction Steps

[Extract and format reproduction steps from the issue body]

### Step 1: [Description]

\`\`\`bash
# Commands from issue
\`\`\`

### Step 2: [Description]

\`\`\`bash
# Commands from issue
\`\`\`

[Continue for all steps]

## Verification

### Check Current Status

\`\`\`bash
kubectl get all -n istio-system
kubectl get istio,istiorevision -A  # for sail-operator
\`\`\`

### Check Logs

\`\`\`bash
# Operator logs (for sail-operator)
kubectl logs -n sail-operator deployment/sail-operator --tail=50

# Istiod logs
kubectl logs -n istio-system -l app=istiod --tail=50
\`\`\`

### Expected vs Actual

**Expected**: [From issue]

**Actual**: [From issue or "Verify if issue reproduces"]

## Cleanup

[If single cluster:]
\`\`\`bash
kind delete cluster --name sail-test
\`\`\`

[If multi-cluster:]
\`\`\`bash
kind delete cluster --name east
kind delete cluster --name west
rm -rf certs
\`\`\`

[If custom cluster, provider-specific cleanup:]

**Sail Operator:**
\`\`\`bash
kubectl delete istio --all -A
kubectl delete istiorevision --all
kubectl delete istiocni --all -A  # if ambient
kubectl delete ztunnel --all -A   # if ambient
kubectl delete -f https://github.com/istio-ecosystem/sail-operator/releases/latest/download/install.yaml
\`\`\`

**Helm:**
\`\`\`bash
helm uninstall ztunnel -n istio-system  # if ambient
helm uninstall istiod -n istio-system
helm uninstall istio-cni -n istio-cni   # if ambient
helm uninstall istio-base -n istio-system
kubectl delete namespace istio-system istio-cni
\`\`\`

**Istioctl:**
\`\`\`bash
istioctl uninstall --purge -y
kubectl delete namespace istio-system istio-cni
\`\`\`

## Notes

- All commands are self-contained and don't require cloning additional repositories
- For custom platforms, adjust namespace and resource names as needed
- Check operator/istiod logs if issues occur during installation
- Verify all pods are in Running state before proceeding with reproduction steps
```

### 4. Save and Display Document

Save the generated markdown to `reproduce-issue-[number].md` and display summary:

```
✅ Reproduction guide generated successfully!
==========================================

📄 File: reproduce-issue-1234.md
🔍 Issue: #1234 - [title]
📦 Provider: sail-operator
🏗️  Platform: KIND
🌐 Clusters: [single/multi]

The guide includes:
✓ Prerequisites and tool requirements
✓ Complete installation steps
✓ Configuration for [profile] mode
✓ Reproduction steps from the issue
✓ Verification and cleanup instructions

Next steps:
1. Review the generated file
2. Follow the steps to reproduce the issue
3. Report findings in the issue
```

## Error Handling

**Issue not accessible:**
```
❌ Unable to fetch issue details
Ensure you have:
- Valid GitHub token configured (gh auth login)
- Access to the repository
- Correct issue URL

Proceeding with template generation...
```

**Invalid provider:**
```
❌ Invalid provider: [value]
Valid options: sail-operator, helm, istioctl
```

**Invalid platform:**
```
❌ Invalid platform: [value]
Valid options: kind, custom
```

## Examples

### Basic usage with sail-operator on KIND

```bash
/reproduce-issue https://github.com/istio-ecosystem/sail-operator/issues/1234
```

### Using Helm provider

```bash
/reproduce-issue https://github.com/istio-ecosystem/sail-operator/issues/1234 helm
```

### Using istioctl on custom cluster

```bash
/reproduce-issue https://github.com/istio-ecosystem/sail-operator/issues/1234 istioctl custom
```

### All arguments specified

```bash
/reproduce-issue https://github.com/istio-ecosystem/sail-operator/issues/1234 sail-operator kind
```

## Notes

- The generated document is completely self-contained
- All steps use `kubectl apply -f - <<EOF` pattern for inline manifests
- Scripts are embedded directly in code blocks
- Multi-cluster setups include proper aliases for all tools
- Certificate generation uses inline shell scripts
- No need to clone any repositories
- All Istio scripts are fetched via curl from official sources
