#!/bin/bash

# Copyright Istio Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -eux -o pipefail

# Configuration variables
NAMESPACE="${NAMESPACE:-sail-operator}"
CONTROL_PLANE_NS="${CONTROL_PLANE_NS:-istio-system}"
COMMAND=""

# Clean up leftover artifacts from e2e.ocp tests
echo "Cleaning up leftover artifacts from e2e.ocp tests"

# Check if KUBECONFIG is set
if [ -z "${KUBECONFIG}" ]; then
    echo "KUBECONFIG is not set. kubectl will not be able to connect to the cluster. Exiting."
    exit 1
fi

# Detect whether to use kubectl or oc command
detect_command() {
    if command -v oc &> /dev/null && oc status &> /dev/null; then
        COMMAND="oc"
        echo "Using oc command for OpenShift cluster"
    elif command -v kubectl &> /dev/null; then
        COMMAND="kubectl"
        echo "Using kubectl command"
    else
        echo "Neither oc nor kubectl found. Exiting."
        exit 1
    fi
}

# Verify and cleanup Istio custom resources following official undeploy process
verify_and_cleanup_istio_resources() {
    echo "=== Cleaning up Sail Operator custom resources ==="
    
    echo "Checking for Istio resources..."
    if ${COMMAND} get istios.sailoperator.io --all-namespaces --no-headers 2>/dev/null | grep -q .; then
        echo "Removing all Istio resources..."
        ${COMMAND} delete istios.sailoperator.io --all --all-namespaces --wait=true --ignore-not-found
    else
        echo "No Istio resources found"
    fi
    
    echo "Checking for IstioCNI resources..."
    if ${COMMAND} get istiocni.sailoperator.io --all-namespaces --no-headers 2>/dev/null | grep -q .; then
        echo "Removing all IstioCNI resources..."
        ${COMMAND} delete istiocni.sailoperator.io --all --all-namespaces --wait=true --ignore-not-found
    else
        echo "No IstioCNI resources found"
    fi
    
    echo "Checking for ZTunnel resources..."
    if ${COMMAND} get ztunnel.sailoperator.io --all-namespaces --no-headers 2>/dev/null | grep -q .; then
        echo "Removing all ZTunnel resources..."
        ${COMMAND} delete ztunnel.sailoperator.io --all --all-namespaces --wait=true --ignore-not-found
    else
        echo "No ZTunnel resources found"
    fi
}

# Verify and cleanup Sail operator deployment
verify_and_cleanup_operator() {
    echo "=== Cleaning up Sail Operator deployment ==="
    
    echo "Checking for Sail operator in namespace: ${NAMESPACE}..."
    if ${COMMAND} get namespace "${NAMESPACE}" &>/dev/null; then
        echo "Found operator namespace: ${NAMESPACE}"
        
        # Check for Helm release
        if command -v helm &> /dev/null && helm list -n "${NAMESPACE}" | grep -q sail-operator; then
            echo "Uninstalling Sail operator Helm release..."
            helm uninstall sail-operator --namespace "${NAMESPACE}"
        else
            echo "No Helm release found for sail-operator"
        fi
        
        # Check for operator deployment directly
        if ${COMMAND} get deployment sail-operator -n "${NAMESPACE}" &>/dev/null; then
            echo "Removing operator deployment..."
            ${COMMAND} delete deployment sail-operator -n "${NAMESPACE}" --ignore-not-found
        fi
    else
        echo "Operator namespace ${NAMESPACE} not found"
    fi
}

# Cleanup known namespaces
cleanup_namespaces() {
    echo "=== Cleaning up namespaces ==="
    
    for ns in "${NAMESPACE}" "${CONTROL_PLANE_NS}" "istio-cni" "ztunnel"; do
        if ${COMMAND} get namespace "${ns}" &>/dev/null; then
            echo "Removing namespace: ${ns}..."
            ${COMMAND} delete namespace "${ns}" --ignore-not-found --wait=true
        else
            echo "Namespace ${ns} not found (already cleaned up)"
        fi
    done
}

# Cleanup CRDs (existing logic)
cleanup_crds() {
    echo "=== Cleaning up CRDs ==="
    
    echo "Removing Istio CRDs..."
    ${COMMAND} get crds -o name | grep ".*\.istio" | xargs -r -n 1 ${COMMAND} delete || true

    echo "Removing Sail Operator CRDs..."
    ${COMMAND} get crds -o name | grep ".*\.sail" | xargs -r -n 1 ${COMMAND} delete || true
}

# Cleanup cluster-level resources (existing logic)
cleanup_cluster_resources() {
    echo "=== Cleaning up cluster-level resources ==="
    
    echo "Removing cluster role bindings..."
    ${COMMAND} delete clusterrolebinding metrics-reader-rolebinding --ignore-not-found

    echo "Removing cluster roles..."
    ${COMMAND} delete clusterrole metrics-reader --ignore-not-found
}

# Main cleanup flow following official documentation order
main() {
    echo "Starting enhanced cleanup process..."
    
    # 1. Detect command to use
    detect_command
    
    # 2. Delete Sail operator custom resources (gives operator chance to cleanup via finalizers)
    verify_and_cleanup_istio_resources
    
    # 3. Remove operator deployment
    verify_and_cleanup_operator
    
    # 4. Clean up namespaces
    cleanup_namespaces
    
    # 5. Remove CRDs
    cleanup_crds
    
    # 6. Clean up cluster-level resources
    cleanup_cluster_resources
    
    echo "Cleanup completed successfully"
}

# Execute main function
main