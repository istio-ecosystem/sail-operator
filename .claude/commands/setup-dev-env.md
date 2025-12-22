# Setup Development Environment

Set up a complete local development environment with KIND cluster, Sail Operator, and Istio (sidecar or ambient mode).

## Arguments

- `profile` (optional): Deployment profile - either `sidecar` (default) or `ambient`

## Steps

### 1. Validate Profile Argument

If a profile argument is provided, validate it is either "sidecar" or "ambient". If invalid, show error and stop.

If no profile is provided, default to "sidecar".

Display the profile being used:
```
🚀 Setting up Sail Operator Development Environment
Profile: [sidecar|ambient]
```

### 2. Create KIND Cluster

Run the following command to create a KIND cluster with local registry:
```bash
make cluster
```

Explain what this does:
- Creates local KIND cluster
- Sets up registry at localhost:5000
- Configures networking for Istio
- Preloads operator image
- Generates kubeconfig in /tmp directory

**IMPORTANT:** After the cluster is created, the kubeconfig file path will be displayed in the output. Look for a line like:
```
KUBECONFIG=/tmp/tmp.XXXXXXXXXX/config
```

Extract this path from the make cluster output and export it for all subsequent commands.

Run:
```bash
export KUBECONFIG=/tmp/tmp.XXXXXXXXXX/config  # Use the actual path from output
```

After exporting KUBECONFIG, verify the cluster with:
```bash
kubectl cluster-info
kubectl get nodes
```

Expected output:
- Kubernetes control plane running
- 1 node in Ready state

Show: `[1/8] ✅ KIND cluster created and kubeconfig exported`

### 3. Install MetalLB

**IMPORTANT:** Ensure KUBECONFIG is still exported from Step 2.

MetalLB is a load balancer implementation for bare metal Kubernetes clusters. It allows LoadBalancer-type services to receive external IPs in the KIND cluster.

Install MetalLB:
```bash
kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.14.9/config/manifests/metallb-native.yaml
```

Wait for MetalLB to be ready:
```bash
kubectl wait --for=condition=ready pod -l app=metallb -n metallb-system --timeout=2m
```

Verify MetalLB components are running:
```bash
kubectl get pods -n metallb-system
```

Expected output:
- controller pod in Running state (1/1)
- speaker pods in Running state (1/1) - one per node

Configure MetalLB with an IP address pool. First, get the KIND Docker network subnet:
```bash
docker network inspect kind | grep -o '"Subnet": "[^"]*"' | awk -F'"' '{print $4}'
```

Create an IPAddressPool and L2Advertisement for MetalLB:
```bash
kubectl apply -f - <<EOF
apiVersion: metallb.io/v1beta1
kind: IPAddressPool
metadata:
  name: example
  namespace: metallb-system
spec:
  addresses:
  - 172.18.255.200-172.18.255.250
---
apiVersion: metallb.io/v1beta1
kind: L2Advertisement
metadata:
  name: empty
  namespace: metallb-system
EOF
```

Note: The IP range 172.18.255.200-172.18.255.250 is typically safe for KIND's default network (172.18.0.0/16). If your KIND cluster uses a different network, adjust the range accordingly.

Verify the configuration:
```bash
kubectl get ipaddresspool -n metallb-system
kubectl get l2advertisement -n metallb-system
```

Expected output:
- IPAddressPool named "example" with address range configured
- L2Advertisement named "empty"

Show: `[2/8] ✅ MetalLB installed and configured`

### 4. Build and Push Operator Image

**IMPORTANT:** Ensure KUBECONFIG is still exported from Step 2.

Set the HUB and TAG environment variables to use local KIND registry with 'local' tag:
```bash
export HUB=localhost:5000
export TAG=local
```

Build the operator binary, create Docker image, and push to local registry:
```bash
make build docker-build docker-push
```

Explain what this does:
- `make build`: Compiles the operator binary from current source code
- `make docker-build`: Creates Docker image with the compiled binary (tagged as 'local')
- `make docker-push`: Pushes image to localhost:5000 (KIND registry)
- Final image: localhost:5000/sail-operator:local

Verify the image was pushed:
```bash
docker images | grep localhost:5000/sail-operator
```

Expected: Image with tag sail-operator:local listed

Show: `[3/9] ✅ Operator binary built and image pushed to KIND registry`

### 5. Deploy Sail Operator to Cluster

Deploy the operator using the locally built image:
```bash
make deploy
```

Explain what this does:
- Deploys operator to sail-operator namespace
- Uses the image from localhost:5000 (HUB variable)
- Creates necessary RBAC and service accounts

After running, wait for operator to be ready:
```bash
kubectl wait --for=condition=Ready pod -l control-plane=sail-operator -n sail-operator --timeout=3m
```

Verify operator status:
```bash
kubectl get pods -n sail-operator
kubectl get deployment -n sail-operator -o jsonpath='{.items[0].spec.template.spec.containers[0].image}'
```

Expected:
- Operator pod in Running state (1/1)
- Image shows localhost:5000/sail-operator:local

Show: `[4/9] ✅ Sail Operator deployed (using local image)`

### 6. Deploy Istio Control Plane

**If profile is "sidecar":**

Run:
```bash
make deploy-istio
```

This creates istio-system namespace and deploys the Istio CR.

Wait for Istio to be ready:
```bash
kubectl wait --for=condition=Ready istio/default -n istio-system --timeout=5m
```

Verify:
```bash
kubectl get istio -n istio-system
kubectl get istiorevision
kubectl get pods -n istio-system -l app=istiod
```

Show: `[5/9] ✅ Istio control plane deployed (sidecar mode)`

**If profile is "ambient":**

Run:
```bash
make deploy-istio-with-ambient
```

This creates istio-system, istio-cni, and ztunnel namespaces, and deploys Istio, IstioCNI, and ZTunnel CRs.

Wait for all components:
```bash
kubectl wait --for=condition=Ready istio/default -n istio-system --timeout=5m
kubectl wait --for=condition=Ready istiocni/default -n istio-cni --timeout=3m
kubectl wait --for=condition=Ready ztunnel/default -n ztunnel --timeout=3m
```

Verify:
```bash
kubectl get istio -n istio-system
kubectl get istiocni -n istio-cni
kubectl get ztunnel -n ztunnel
kubectl get pods -n istio-system -l app=istiod
kubectl get pods -n istio-cni
kubectl get pods -n ztunnel
```

Show: `[5/9] ✅ Istio control plane deployed (ambient mode with CNI and ZTunnel)`

### 7. Create Sample Namespace

**If profile is "sidecar":**

Run:
```bash
kubectl create namespace sample
kubectl label namespace sample istio-injection=enabled
```

Show: `[6/9] ✅ Sample namespace created with sidecar injection label`

**If profile is "ambient":**

Run:
```bash
kubectl create namespace sample
kubectl label namespace sample istio.io/dataplane-mode=ambient
```

Show: `[6/9] ✅ Sample namespace created with ambient mode label`

### 8. Deploy Sample Applications

Run the following commands to deploy sleep and httpbin using local kustomization files:
```bash
kubectl apply -n sample -k tests/e2e/samples/sleep
kubectl apply -n sample -k tests/e2e/samples/httpbin
```

Explain what this does:
- Uses kustomization files from tests/e2e/samples/
- Applies sleep and httpbin with image overrides
- sleep: Uses quay.io/curl/curl (not docker.io/curlimages/curl)
- httpbin: Uses quay.io/sail-dev/go-httpbin (not docker.io/mccutchen/go-httpbin)

Wait for pods to be ready:
```bash
kubectl wait --for=condition=Ready pod -l app=sleep -n sample --timeout=2m
kubectl wait --for=condition=Ready pod -l app=httpbin -n sample --timeout=2m
```

Check pod status:
```bash
kubectl get pods -n sample
```

**If profile is "sidecar":**
- Verify each pod shows 2/2 containers (app + sidecar)

**If profile is "ambient":**
- Verify each pod shows 1/1 containers (no sidecar, using ztunnel)

Show: `[7/9] ✅ Sample applications deployed (sleep and httpbin)`

### 9. Test Connectivity

Run the following command to test HTTP connectivity from sleep to httpbin:
```bash
kubectl exec -n sample deploy/sleep -- curl -s http://httpbin.sample:8000/headers
```

Check the output:
- Should receive JSON response with HTTP headers

Test response code:
```bash
kubectl exec -n sample deploy/sleep -- curl -s -o /dev/null -w "%{http_code}\n" http://httpbin.sample:8000/status/200
```

Expected output: `200`

**For Sidecar Mode Only:**

Verify mTLS by checking for X-Forwarded-Client-Cert header:
```bash
kubectl exec -n sample deploy/sleep -- curl -s http://httpbin.sample:8000/headers | grep X-Forwarded-Client-Cert
```

If `X-Forwarded-Client-Cert` is present, mTLS is active at L7 (via sidecar).

**For Ambient Mode:**

Note: X-Forwarded-Client-Cert header will NOT be present in ambient mode without a waypoint proxy. This is expected behavior:
- Ambient mode uses L4 mTLS via ztunnel
- L7 headers like X-Forwarded-Client-Cert require a waypoint proxy
- The 200 response confirms connectivity is working through ztunnel
- mTLS is active at L4 (transparent to application)

Show:
- Sidecar mode: `[8/9] ✅ Connectivity test passed - mTLS active (L7 via sidecar)`
- Ambient mode: `[8/9] ✅ Connectivity test passed - mTLS active (L4 via ztunnel)`

### 10. Display Final Summary

Gather all status information and display a comprehensive summary.

**For Sidecar Mode:**

```
✅ Development Environment Setup Complete!
==========================================

📦 CLUSTER
   - Type: KIND
   - Status: Running
   - Nodes: 1 Ready

⚖️  METALLB
   - Namespace: metallb-system
   - Status: Running
   - Controller: Running (1/1)
   - Speaker: Running (1/1)
   - IP Pool: 172.18.255.200-172.18.255.250

🎛️  SAIL OPERATOR
   - Namespace: sail-operator
   - Status: Running (1/1)
   - Version: [show detected version]

🌐 ISTIO CONTROL PLANE
   - Profile: Sidecar
   - Namespace: istio-system
   - Status: Ready
   - Revision: [show revision name]
   - istiod: Running (1/1)

🧪 SAMPLE APPLICATIONS
   - Namespace: sample
   - Label: istio-injection=enabled
   - sleep: Running (2/2) - with sidecar
   - httpbin: Running (2/2) - with sidecar
   - Connectivity: ✅ Tested (200 OK)
   - mTLS: ✅ Active (L7 via sidecar, X-Forwarded-Client-Cert present)

📋 SUMMARY
   - ✅ KIND cluster created
   - ✅ MetalLB installed and configured
   - ✅ Sail Operator deployed
   - ✅ Istio control plane ready
   - ✅ Sample apps deployed with sidecars
   - ✅ Connectivity test passed
   - ✅ mTLS working (L7 via sidecar)
   - ✅ Environment fully operational

🧪 NEXT STEPS
1. Test additional endpoints:
   kubectl exec -n sample deploy/sleep -- curl http://httpbin.sample:8000/get

2. Deploy bookinfo sample:
   kubectl label namespace default istio-injection=enabled
   kubectl apply -n default -f https://raw.githubusercontent.com/istio/istio/master/samples/bookinfo/platform/kube/bookinfo.yaml

3. Configure ingress gateway:
   See docs/common/create-and-configure-gateways.adoc

📚 DOCUMENTATION
   - Installation: README.md
   - Sidecar injection: docs/common/install-bookinfo-app.adoc
   - Gateways: docs/common/create-and-configure-gateways.adoc
```

**For Ambient Mode:**

```
✅ Development Environment Setup Complete!
==========================================

📦 CLUSTER
   - Type: KIND
   - Status: Running
   - Nodes: 1 Ready

⚖️  METALLB
   - Namespace: metallb-system
   - Status: Running
   - Controller: Running (1/1)
   - Speaker: Running (1/1)
   - IP Pool: 172.18.255.200-172.18.255.250

🎛️  SAIL OPERATOR
   - Namespace: sail-operator
   - Status: Running (1/1)
   - Image: localhost:5000/sail-operator:local (local build)
   - Version: [show detected version]

🌐 ISTIO CONTROL PLANE (AMBIENT MODE)
   - Profile: Ambient
   - Namespace: istio-system
   - Status: Ready
   - Revision: [show revision name]
   - istiod: Running (1/1)

🔌 ISTIO CNI
   - Namespace: istio-cni
   - Status: Ready
   - DaemonSet: Running ([show count] nodes)

🔒 ZTUNNEL
   - Namespace: ztunnel
   - Status: Ready
   - DaemonSet: Running ([show count] nodes)

🧪 SAMPLE APPLICATIONS
   - Namespace: sample
   - Label: istio.io/dataplane-mode=ambient
   - sleep: Running (1/1) - no sidecar
   - httpbin: Running (1/1) - no sidecar
   - Connectivity: ✅ Tested (200 OK)
   - mTLS: ✅ Active (L4 via ztunnel)
   - Traffic: Flows through ztunnel (no L7 headers without waypoint)

📋 SUMMARY
   - ✅ KIND cluster created
   - ✅ MetalLB installed and configured
   - ✅ Sail Operator deployed
   - ✅ IstioCNI daemonset running
   - ✅ Istio control plane ready
   - ✅ ZTunnel daemonset running
   - ✅ Sample apps deployed (ambient mode)
   - ✅ Connectivity test passed
   - ✅ mTLS working (L4 via ztunnel)
   - ✅ Ambient mode fully operational

🧪 NEXT STEPS
1. Test additional endpoints:
   kubectl exec -n sample deploy/sleep -- curl http://httpbin.sample:8000/get

2. Deploy waypoint for L7 features:
   istioctl waypoint apply -n sample
   kubectl label namespace sample istio.io/use-waypoint=waypoint

3. Deploy bookinfo sample:
   kubectl label namespace default istio.io/dataplane-mode=ambient
   kubectl apply -n default -f https://raw.githubusercontent.com/istio/istio/master/samples/bookinfo/platform/kube/bookinfo.yaml

4. Check ztunnel workloads:
   istioctl ztunnel-config workloads -n ztunnel

📚 DOCUMENTATION
   - Ambient mode: docs/common/istio-ambient-mode.adoc
   - Waypoints: docs/common/istio-ambient-waypoint.adoc
   - Migration guide: docs/migrate-from-sidecar-to-ambient/migration.adoc
```

Show: `[9/9] ✅ Setup complete - environment verified and ready`

## Error Handling

If any command fails at any step:

1. **Stop immediately** and show the error
2. **Display the failed command** and its output
3. **Provide troubleshooting guidance** based on the failure type
4. **Suggest recovery steps**

### Common Errors:

**KIND cluster creation failed:**
```
❌ Failed at Step 1: KIND cluster creation

Error: [show error message]

Troubleshooting:
1. Ensure Docker/Podman is running
2. Check if port 5000 is available
3. Try: kind delete cluster && make cluster
```

**MetalLB installation failed:**
```
❌ Failed at Step 3: MetalLB installation

Error: [show error message]

Troubleshooting:
1. Check if metallb-system namespace was created:
   kubectl get namespace metallb-system
2. Check MetalLB pod logs:
   kubectl logs -n metallb-system -l app=metallb
3. Verify network connectivity:
   kubectl get pods -n metallb-system
4. Retry installation:
   kubectl delete namespace metallb-system
   kubectl apply -f https://raw.githubusercontent.com/metallb/metallb/v0.14.9/config/manifests/metallb-native.yaml
```

**Image build/push failed:**
```
❌ Failed at Step 4: Building operator image

Error: [show error message]

Troubleshooting:
1. Ensure HUB is set: echo $HUB (should be localhost:5000)
2. Check KIND registry is running: docker ps | grep kind-registry
3. Verify you can push: docker push localhost:5000/test:latest
4. Retry: make docker-build docker-push
```

**Operator deployment failed:**
```
❌ Failed at Step 5: Operator deployment

Check operator logs:
kubectl logs -n sail-operator deployment/sail-operator --tail=100

Common fixes:
1. Verify CRDs: kubectl get crd | grep sailoperator.io
2. If missing: make install
3. Retry: make deploy
```

**Istio not ready:**
```
❌ Failed at Step 6: Istio control plane

Check Istio status:
kubectl describe istio/default -n istio-system

Check istiod:
kubectl get pods -n istio-system
kubectl logs -n istio-system -l app=istiod
```

**Sample apps not ready:**
```
❌ Failed at Step 7 or 8: Sample applications

Check pod status:
kubectl get pods -n sample
kubectl describe pod -n sample -l app=sleep
kubectl describe pod -n sample -l app=httpbin

Verify namespace label:
kubectl get namespace sample --show-labels
```

**Connectivity test failed:**
```
❌ Failed at Step 9: Connectivity test

Test result: [show curl output or error]

Troubleshooting:
1. Check if httpbin is running:
   kubectl get pods -n sample -l app=httpbin

2. Check if sleep is running:
   kubectl get pods -n sample -l app=sleep

3. Check service exists:
   kubectl get svc -n sample httpbin

4. Check logs:
   kubectl logs -n sample deploy/sleep
   kubectl logs -n sample deploy/httpbin
```

## Cleanup

To tear down the environment, run:
```bash
kind delete cluster
```

This deletes the entire KIND cluster including all deployed resources.

To keep the cluster but remove resources:
```bash
kubectl delete namespace sample
kubectl delete istio --all --all-namespaces
kubectl delete istiocni --all --all-namespaces  # if ambient mode
kubectl delete ztunnel --all --all-namespaces   # if ambient mode
```

## Notes

- All commands should be executed in the project root directory
- Wait for each component to be fully ready before moving to next step
- Verify connectivity test succeeds to ensure complete working environment
- For ambient mode, verify pods show 1/1 containers (no sidecar)
- For sidecar mode, verify pods show 2/2 containers (app + sidecar)

## Environment Variables

This command requires setting two critical environment variables:

### 1. KUBECONFIG

The KIND cluster creates a kubeconfig file in /tmp directory. After running `make cluster`:

1. **Capture the kubeconfig path** from the output (looks like: `KUBECONFIG=/tmp/tmp.XXXXXXXXXX/config`)
2. **Export it immediately:**
   ```bash
   export KUBECONFIG=/tmp/tmp.XXXXXXXXXX/config
   ```
3. **Verify it's set:**
   ```bash
   echo $KUBECONFIG
   kubectl cluster-info
   ```
4. **Keep it exported** for the entire setup process

**Alternative:** All kubectl commands can be run with `--kubeconfig` flag, but exporting once is simpler.

### 2. HUB (Registry Location)

To use the locally built operator image:

1. **Export HUB and TAG:**
   ```bash
   export HUB=localhost:5000
   export TAG=local
   ```
2. **Keep them exported** - make commands use these to build and deploy
3. **Verify:**
   ```bash
   echo $HUB   # Should show: localhost:5000
   echo $TAG   # Should show: local
   ```

**Why these are needed:**
- `make build` compiles the operator binary
- `make docker-build` creates image as $HUB/sail-operator:$TAG
- `make docker-push` pushes to $HUB/sail-operator:$TAG
- `make deploy` deploys from $HUB/sail-operator:$TAG
- Setting HUB=localhost:5000 and TAG=local creates: localhost:5000/sail-operator:local
