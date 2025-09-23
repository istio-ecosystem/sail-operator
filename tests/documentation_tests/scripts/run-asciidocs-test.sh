#!/bin/bash

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

set -eu -o pipefail

# shellcheck disable=SC2155
export SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../../.." && pwd)"

export KIND_CLUSTER_NAME="docs-automation"
export IP_FAMILY="ipv4"
export ISTIOCTL="${ROOT_DIR}/bin/istioctl"
export IMAGE_BASE="sail-operator"
export TAG="latest"
export LOCAL_REGISTRY="localhost:5000"
export OCP=false
export KIND_REGISTRY_NAME="kind-registry"
export KIND_REGISTRY_PORT="5000"
export KIND_REGISTRY="localhost:${KIND_REGISTRY_PORT}"
# Use the local registry instead of the default HUB
export HUB="${KIND_REGISTRY}"
# Workaround make inside make: ovewrite this variable so it is not recomputed in Makefile.core.mk
export IMAGE="${HUB}/${IMAGE_BASE}:${TAG}"
export ARTIFACTS="${ARTIFACTS:-$(mktemp -d)}"
export TEST_DIR="${ARTIFACTS}/docs.test"
export KUBECONFIG="${KUBECONFIG:-"${ARTIFACTS}/config"}"
export HELM_TEMPL_DEF_FLAGS="--include-crds --values chart/values.yaml"
export DOCS_DIR="${ROOT_DIR}/docs"

# Check that TEST_DIR exist, if not create the directory
if [[ ! -d "$TEST_DIR" ]]; then
  echo "Creating TEST_DIR directory: $TEST_DIR"
  mkdir -p "$TEST_DIR"
else
  echo "Using existing TEST_DIR directory: $TEST_DIR"
fi

# If os is Darwin set a different KIND_IMAGE= docker.io/kindest/node:v1.33.2
if [[ "$(uname)" == "Darwin" ]]; then
  export KIND_IMAGE="docker.io/kindest/node:v1.33.2"
fi

# Validate that istioctl is installed
if ! command -v istioctl &> /dev/null; then
  echo "istioctl could not be found. Please install it."
  exit 1
fi

# Validate that kubectl is installed
if ! command -v kubectl &> /dev/null; then
  echo "kubectl could not be found. Please install it."
  exit 1
fi

function create_test_files() {
    # Get all the .adoc files in the DOCS_DIR and subdirectories
    FILES_TO_CHECK=()
    while IFS= read -r -d '' file; do
      # Skip guidelines.adoc. This file contains the guidelines and we want to avoid parsing this file.
      if [[ "$(basename "$file")" == "guidelines.adoc" ]]; then
        continue
      fi
      FILES_TO_CHECK+=("$file")
    done < <(find "$DOCS_DIR" -name "*.adoc" -print0)
    

    # From the list of files get the test list by looking for all the conditional blocks
    # Create a TEST_KV with the filename and the test name
    # Example: ifdef::cni-update-test[] endif::[] from the file README.adoc
    # TEST_KV: README.adoc cni-update-test
    TEST_KV=()
    for file in "${FILES_TO_CHECK[@]}"; do
        while IFS= read -r line; do
            if [[ $line =~ ^ifdef::(.*)\[\] ]]; then
                TEST_KV+=("$file:${BASH_REMATCH[1]}")
            fi
        done < "$file"
    done

    # Delete duplicates
    local temp_array=()
    while IFS= read -r kv; do
        temp_array+=("$kv")
    done < <(printf "%s\n" "${TEST_KV[@]}" | sort -u)
    TEST_KV=("${temp_array[@]}")

    echo "Tests list: ${TEST_KV[*]}"

    # From each Test name in the list create a corresponding test file
    for test_kv in "${TEST_KV[@]}"; do
        IFS=: read -r file test_name <<< "$test_kv"
        test_file="${TEST_DIR}/${test_name}.sh"
        echo "Creating test file: $test_file"
        touch "$test_file"

        echo "Print test file contents:"
        echo "=========================="
        cat "$test_file"
        echo "=========================="
        # Add required steps to the test file
        {
          echo "#!/bin/bash"
          echo "set -eu -o pipefail"
          echo "echo \"Running test: $test_name\""
          echo "# Source to prebuilt functions"
          echo ". \$SCRIPT_DIR/prebuilt-func.sh"
        } > "$test_file"
        # Extract content in document order: both ifdef blocks and named code blocks matching the test name
        # First pass: extract content from named code blocks and ifdef blocks in order
        awk -v test_name="$test_name" '
        # Handle ifdef blocks
        $0 ~ "ifdef::" test_name "\\[\\]" {
            in_ifdef = 1
            next
        }
        /^endif::\[\]/ && in_ifdef {
            in_ifdef = 0
            next
        }
        in_ifdef {
            print
            next
        }
        
        # Handle named code blocks
        $0 ~ "\\[source,.*,name=\"" test_name "\"\\]" {
            found_named_block = 1
            next
        }
        found_named_block && /^----$/ {
            if (in_named_code_block) {
                in_named_code_block = 0
                found_named_block = 0
            } else {
                in_named_code_block = 1
            }
            next
        }
        found_named_block && in_named_code_block {
            print
        }
        ' "$file" >> "$test_file"

        # Replace variables defined in the .adoc file
        while IFS= read -r line; do
            if [[ $line =~ ^:([a-zA-Z_][a-zA-Z0-9_]*):[:space:]*(.*)$ ]]; then
                var_name="${BASH_REMATCH[1]}"
                var_value="${BASH_REMATCH[2]// /}"  # Remove leading/trailing spaces
                echo "Replacing variable: $var_name with value: $var_value"
                sed "s|{$var_name}|$var_value|g" "$test_file" > "$test_file.tmp" && mv "$test_file.tmp" "$test_file"
            fi
        done < "$file"
    done

    cat "$TEST_DIR"/*.sh
}

# Run the tests on a separate cluster for all given test files
function run_tests() {
  (
    echo "Setting up cluster: $KIND_CLUSTER_NAME to run AsciiDoc tests"
    kind delete cluster --name "$KIND_CLUSTER_NAME" > /dev/null || true

    # Source setup and build scripts to preserve trap and env
    source "${ROOT_DIR}/tests/e2e/setup/setup-kind.sh"

    # Build and push the operator image from source
    source "${ROOT_DIR}/tests/e2e/setup/build-and-push-operator.sh"
    build_and_push_operator_image

    # Ensure kubeconfig is set to the current cluster
    # TODO: check why KUBECONFIG is not properly set
    kind export kubeconfig --name="${KIND_CLUSTER_NAME}"

    # Deploy the sail operator
    kubectl create ns sail-operator || echo "namespace sail-operator already exists"
    # shellcheck disable=SC2086
    helm template chart chart ${HELM_TEMPL_DEF_FLAGS} --set image="${IMAGE}" --namespace sail-operator | kubectl apply --server-side=true -f -
    kubectl wait --for=condition=available --timeout=600s deployment/sail-operator -n sail-operator

    # Record all namespaces before each test, to restore to this point
    declare -A namespaces
    for ns in $(kubectl get namespaces -o name); do
      namespaces["$ns"]=1
    done

    # Get list of test files to run (passed as parameters)
    test_files=("$@")
    
    for test_file in "${test_files[@]}"; do
      test_name=$(basename "$test_file" .sh)
      echo "*** Running test: $test_name ***"
      
      # Make test file executable and run it
      chmod +x "$test_file"
      "$test_file"
      
      echo "*** Test completed: $test_name ***"

      # Save some time by avoiding cleanup for last test, as the cluster will be deleted anyway.
      [ "$test_file" != "${test_files[-1]}" ] || break

      # Clean up any of our cluster-wide custom resources
      for crd in $(cat chart/crds/sailoperator.io_*.yaml | yq -rN 'select(.kind == "CustomResourceDefinition" and .spec.scope == "Cluster") | .metadata.name'); do
        for cr in $(kubectl get "$crd" -o name 2>/dev/null || true); do
          kubectl delete "$cr" 2>/dev/null || true
        done
      done

      # Clean up any new namespaces created after the cluster was deployed.
      for ns in $(kubectl get namespaces -o name); do
        if [ -z "${namespaces[$ns]-}" ]; then
          kubectl delete "$ns"
          kubectl wait --for=delete "$ns" --timeout=3m
        fi
      done
    done
  )
}

# Create test files first
create_test_files

# Check that test files were created
if ! find "$TEST_DIR" -maxdepth 1 -name "*.sh" -not -name "prebuilt-func.sh"; then
  echo "No test files found in the test directory: $TEST_DIR"
  exit 1
fi

# Separate dual stack tests from regular tests
regular_tests=()
dual_stack_tests=()

for test_file in "$TEST_DIR"/*.sh; do
  test_name=$(basename "$test_file" .sh)
  if [[ "$test_name" == *"dual-stack"* ]]; then
    dual_stack_tests+=("$test_file")
  else
    regular_tests+=("$test_file")
  fi
done

echo "Regular tests: ${regular_tests[*]}"
if [[ ${#dual_stack_tests[@]} -gt 0 ]]; then
  echo "Dual stack tests: ${dual_stack_tests[*]}"
else
  echo "Dual stack tests: none"
fi

# Run regular tests on a single cluster
if [[ ${#regular_tests[@]} -gt 0 ]]; then
  echo "Running regular tests..."
  run_tests "${regular_tests[@]}"
fi

# Run dual stack tests on their own cluster, since they need to be deployed with support for dual stack
if [[ ${#dual_stack_tests[@]} -gt 0 ]]; then
  echo "Running dual stack tests with IP_FAMILY=dual..."
  IP_FAMILY="dual" run_tests "${dual_stack_tests[@]}"
fi
