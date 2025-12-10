#!/bin/sh

# Copyright Istio Authors

# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This bash file is going to be used to run prebuilt functions that help to test the examples easily
# in the documentation. The main idea is to gather all the functions that are going to be used in the
# documentation tests and put them in this file. This way, we can easily run the tests in the documentation
# and make sure that they are working as expected.
# Please add here any prebuilt functions that are going to be used in the documentation tests. For example:
# - wait for pod
# - wait for deployment
# - check resource status
# - check resource creation

set -eu
## General prebuilt functions
## Place here general functions that are going to be used in the documentation tests
## please do not add any specific functions here, they should be added in the next section

# Logging utilities for test execution
# Global step counter for the current test
STEP_COUNTER=0
# Global test name (set externally by the test script)
TEST_NAME=${TEST_NAME:-}

# Print a test start heading
test_start() {
    test_name="$1"
    STEP_COUNTER=0
    echo ""
    echo "##### Test ${test_name}: start"
    echo ""
}

# Print a test step heading
test_step() {
    step_description="$1"
    STEP_COUNTER=$((STEP_COUNTER + 1))
    echo ""
    if [ -n "$step_description" ]; then
        echo "##### Test ${TEST_NAME}: Step ${STEP_COUNTER}: ${step_description}"
    else
        echo "##### Test ${TEST_NAME}: Step ${STEP_COUNTER}"
    fi
    echo ""
}

# Print a test completion heading
test_end() {
    exit_code="${1:-$?}"
    if [ "$exit_code" -eq 0 ]; then
        status="PASS"
    else
        status="FAIL"
    fi
    echo ""
    echo "##### Test ${TEST_NAME} completed: ${status}"
    echo ""
    return "$exit_code"
}

# Retry a command a number of times with a delay
with_retries() {
    retries=60
    delay=2
    i=1 
    while [ "$i" -le "$retries" ]; do
        if "$@"; then
            return 0
        fi
        sleep "$delay"
        i=$((i + 1))
    done    
    echo "Command failed after $retries retries: $*" >&2
    return 1
}

# Get pod names in a namespace filtered by label (or all pods if no label)
get_pod_names() {
    namespace="$1"
    label="$2"

    if [ -n "$label" ] || [ -z "$label" ]; then
        pods=$(kubectl get pods -n "$namespace" -l "$label" \
            -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null)
    else
        pods=$(kubectl get pods -n "$namespace" \
            -o jsonpath='{range .items[*]}{.metadata.name}{"\n"}{end}' 2>/dev/null)
    fi

    [ -z "$pods" ] && return 1
    echo "$pods"
}

# Wait for a pod to be ready
wait_for_pod_ready() {
    namespace="$1"
    pod_name="$2"
    timeout="2m"
    echo "Waiting for pod $pod_name in namespace $namespace to be ready..."
    kubectl wait --for=condition=ready pod "$pod_name" -n "$namespace" --timeout="$timeout"
}

# Wait for all pods in a namespace to be ready
wait_pods_ready_by_ns() {
    namespace="$1"
    pod_names=$(get_pod_names "$namespace" "")

    [ -z "$pod_names" ] && echo "No pods found in namespace $namespace" && return 1
    echo "Found pod(s) in namespace $namespace: $pod_names"

    for pod_name in $pod_names; do
        wait_for_pod_ready "$namespace" "$pod_name"
    done
}

# Wait for all pods that match labels in a namespace to be ready
wait_pods_by_label() {
    namespace="$1"
    label="$2"
    pod_names=$(get_pod_names "$namespace" "$label")

    [ -z "$pod_names" ] && echo "No matching pods in $namespace for label '$label'" && return 1
    echo "Found pod(s): $pod_names"

    for pod_name in $pod_names; do
        wait_for_pod_ready "$namespace" "$pod_name"
    done
}

# Wait for a resource to reach a given status (e.g., Healthy)
wait_resource_state() {
    namespace="$1"
    resource_name="$2"
    resource_type="$3"
    expected_state="$4"

    echo "Waiting for $resource_type $resource_name in $namespace to be $expected_state..."
    status=$(kubectl get "$resource_type" "$resource_name" -n "$namespace" -o jsonpath='{.status.state}' 2>/dev/null)

    if [ "$status" != "$expected_state" ]; then
        echo "$resource_type $resource_name in $namespace is not $expected_state (current: $status)"
        return 1
    fi

    echo "$resource_type $resource_name in $namespace is $expected_state"
}

# Check that a resource has the expected spec.version
resource_version_equal() {
    resource_type="$1"
    resource_name="$2"
    expected_version="$3"

    version=$(kubectl get "$resource_type" "$resource_name" -o jsonpath='{.spec.version}' 2>/dev/null)

    if [ -z "$version" ]; then
        echo "Version not found for $resource_type $resource_name"
        return 1
    fi

    if [ "$version" != "$expected_version" ]; then
        echo "$resource_type $resource_name version mismatch: got $version, expected $expected_version"
        return 1
    fi

    echo "$resource_type $resource_name has expected version: $version"
}

## Resource-specific helpers

# Wait for Istio CNI readiness
wait_cni_ready() {
    namespace="$1"
    with_retries wait_resource_state "$namespace" "default" "istiocni" "Healthy"
    with_retries wait_pods_by_label "$namespace" "k8s-app=istio-cni-node"
}

# Print Istio CNI info
print_cni_info() {
    kubectl get istiocni
    kubectl get pods -n istio-cni
}

# Wait for Istiod pods to be ready
wait_istio_ready() {
    namespace="$1"
    with_retries wait_pods_by_label "$namespace" "app=istiod"
}

# Print Istio info
print_istio_info() {
    kubectl get istio
    kubectl get pods -n istio-system
    kubectl get istio
    kubectl get istiorevision
    kubectl get istiorevisiontag
}

# Check that all pods in a namespace have the expected Istio version
pods_istio_version_match() {
    namespace="$1"
    expected_version="$2"
    istio_ns="${3:-istio-system}"

    pod_names=$(get_pod_names "$namespace" "")
    [ -z "$pod_names" ] && echo "No pods found in namespace $namespace" && return 1
    echo "Verifying Istio version for pods in $namespace..."

    proxy_status=$(istioctl proxy-status -i "$istio_ns" 2>/dev/null)

    for pod_name in $pod_names; do
        full_name="$pod_name.$namespace"
        version=$(echo "$proxy_status" | awk -v name="$full_name" '$1 == name { print $NF }')

        if [ -z "$version" ]; then
            echo "Proxy status not found for pod $full_name"
            return 1
        fi

        if [ "$version" != "$expected_version" ]; then
            echo "Pod $full_name: got version $version, expected $expected_version"
            return 1
        else
            echo "Pod $full_name is running Istio $version"
        fi
    done

    return 0
}

# Check that the provided istiorevision is in use
istio_active_revision_match() {
    expected_revision="$1"

    active_revision=$(kubectl get istio -o jsonpath='{.items[0].status.activeRevisionName}')

    if [ "$active_revision" != "$expected_revision" ]; then
        echo "Expected active revision $expected_revision, got $active_revision"
        return 1
    fi

    echo "Active revision is $active_revision"
}

# Check that the number of istiorevisions ready match the expected number
istio_revisions_ready_count() {
    expected_count="$1"

    revisions=$(kubectl get istio -o jsonpath='{.items[0].status.revisions.ready}')

    if [ "$revisions" -ne "$expected_count" ]; then
        echo "Expected $expected_count istiorevisions, got $revisions"
        return 1
    fi

    echo "Found $revisions istiorevisions"
}

# Check the number of istiod pods running in the istio-system namespace
istiod_pods_count() {
    expected_count="$1"

    pods_count=$(kubectl get pods -n istio-system -l app=istiod --output json | jq -j '.items | length')

    if [ "$pods_count" -ne "$expected_count" ]; then
        echo "Expected $expected_count istiod pods, got $pods_count"
        return 1
    fi

    echo "Found $pods_count istiod pods in istio-system namespace"
}

# Check the status of a specific istio revision tag
istio_revision_tag_status_equal() {
    expected_status="$1"
    istio_revisiontag_name="$2"

    status=$(kubectl get istiorevisiontag "$istio_revisiontag_name" -o jsonpath='{.status.state}' 2>/dev/null)

    if [ "$status" != "$expected_status" ]; then
        echo "Expected istiorevision tag $istio_revisiontag_name to be $expected_status, got $status"
        return 1
    fi
}

# Check if a revision tag is in use
istio_revision_tag_inuse() {
    inuse="$1"
    istio_revisiontag_name="$2"

    conditions=$(kubectl get istiorevisiontag "$istio_revisiontag_name" -o jsonpath='{.status.conditions}' 2>/dev/null)
    # istiod_revision_tag will be the result of getting InUse status condition in lower case and without any spaces
    istio_revision_tag_insue=$(echo "$conditions" | jq -r '.[] | select(.type == "InUse") | .status' | tr '[:upper:]' '[:lower:]' | xargs)
    if [ "$istio_revision_tag_insue" != "$inuse" ]; then
        echo "Expected istiorevision tag $istio_revisiontag_name to be in use, got $istio_revision_tag_insue"
        return 1
    fi
    echo "Istiod revision tag is in use: $istio_revision_tag_insue"
}

# Wait for rollout to success for all the pods in the given namespace
wait_for_rollout_success() {
    namespace="$1"

    deploy_names=$(kubectl get deploy -n "$namespace" -o name)

    [ -z "$deploy_names" ] && echo "No deployments found in namespace $namespace" && return 1
    echo "Found deployment(s): $deploy_names"

    for deploy in $deploy_names; do
        kubectl rollout status "$deploy" -n "$namespace" --timeout=60s
    done
}