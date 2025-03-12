#!/bin/bash

# This script was generated from the documentation file docs/README.md
# Please check the documentation file for more information

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "$SCRIPT_DIR/update-docs-scripts.sh"
# <!-- generate-docs-test-init Update_Strategy_In_Place-->
# When the `InPlace` strategy is used, the existing Istio control plane is replaced with a new version. The workload sidecars immediately connect to the new control plane. The workloads therefore don't need to be moved from one control plane instance to another.
# 
# #### Example using the InPlace strategy
# 
# Prerequisites:
# * Sail Operator is installed.
install_sail_operator
# * `istioctl` is [installed](common/install-istioctl-tool.md).
# 
# Steps:
# 1. Create the `istio-system` namespace.
# 

kubectl create namespace istio-system

# 
# 2. Create the `Istio` resource.
# 

cat <<EOF | kubectl apply -f-
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
name: default
spec:
namespace: istio-system
updateStrategy:
type: InPlace
version: v1.22.5
EOF

# 
# 3. Confirm the installation and version of the control plane.
# 
#     console
#     $ kubectl get istio -n istio-system
#     NAME      REVISIONS   READY   IN USE   ACTIVE REVISION   STATUS    VERSION   AGE
#     default   1           1       0        default           Healthy   v1.22.5   23s
#     
#     Note: `IN USE` field shows as 0, as `Istio` has just been installed and there are no workloads using it.
# 
# 4. Create namespace `bookinfo` and deploy bookinfo application.
# 

kubectl create namespace bookinfo
kubectl label namespace bookinfo istio-injection=enabled
kubectl apply -n bookinfo -f https://raw.githubusercontent.com/istio/istio/release-1.22/samples/bookinfo/platform/kube/bookinfo.yaml

#     Note: if the `Istio` resource name is other than `default`, you need to set the `istio.io/rev` label to the name of the `Istio` resource instead of adding the `istio-injection=enabled` label.
# 
# 5. Review the `Istio` resource after application deployment.
# 
#    console
#    $ kubectl get istio -n istio-system
#    NAME      REVISIONS   READY   IN USE   ACTIVE REVISION   STATUS    VERSION   AGE
#    default   1           1       1        default           Healthy   v1.22.5   115s
#    
#    Note: `IN USE` field shows as 1, as the namespace label and the injected proxies reference the IstioRevision.
# 
# 6. Perform the update of the control plane by changing the version in the Istio resource.
# 

kubectl patch istio default -n istio-system --type='merge' -p '{"spec":{"version":"v1.23.2"}}'

# 
# 7. Confirm the `Istio` resource version was updated.
# 
#     console
#     $ kubectl get istio -n istio-system
#     NAME      REVISIONS   READY   IN USE   ACTIVE REVISION   STATUS    VERSION   AGE
#     default   1           1       1        default           Healthy   v1.23.2   4m50s
#     
# 
# 8. Delete `bookinfo` pods to trigger sidecar injection with the new version.
# 

kubectl rollout restart deployment -n bookinfo

# 
# 9. Confirm that the new version is used in the sidecar.
# 

istioctl proxy-status 

#     The column `VERSION` should match the new control plane version.
# <!-- generate-docs-test-end -->
