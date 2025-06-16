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

export SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../../.." && pwd)"
FOCUS_DOC_TAGS="${FOCUS_DOC_TAGS:-}"

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

# Ensure TEST_DIR exists, if not create it
mkdir -p "$TEST_DIR"

# Run the update-docs-examples.sh script to update the documentation files into the artifacts directory. By default ARTIFACTS is set to a temporary directory.
"${ROOT_DIR}/tests/documentation_tests/scripts/update-docs-examples.sh"

# Check that .md files were copied to the artifacts directory. If there is no files, then exit with an error.
if ! find "$TEST_DIR" -maxdepth 1 -name "*.md"; then
  echo "No .md files found in the artifacts directory: $TEST_DIR"
  exit 1
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

# Discover .md files with bash tags
FILES_TO_CHECK=()
for file in "$TEST_DIR"/*.md; do
  if grep -q "bash { name=" "$file"; then
    FILES_TO_CHECK+=("$file")
  fi
done

# Build a list of file-tag pairs, isolating 'dual-stack' since it requires special treatment
TAGS_LIST=()
dual_stack_tag=""

for file in "${FILES_TO_CHECK[@]}"; do
  TAGS=$(grep -oP 'tag=\K[^} ]+' "$file" | sort -u)
  for tag in $TAGS; do
    if [[ -n "$FOCUS_DOC_TAGS" && "$tag" != "$FOCUS_DOC_TAGS" ]]; then
      continue
    fi

    if [[ "$tag" == "dual-stack" ]]; then
      dual_stack_tag="$file -t $tag"
      continue
    fi

    TAGS_LIST+=("$file -t $tag")
  done
done

echo "Tags list:"
for tag in "${TAGS_LIST[@]}"; do
  echo "$tag"
done
echo "$dual_stack_tag"

# Run the tests on a separate cluster for all given tags
function run_tests() {
  (
    echo "Setting up cluster: $KIND_CLUSTER_NAME to run tests for tags: $*"
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

    for tag in "$@"; do
      FILE=$(echo "$tag" | cut -d' ' -f1)
      RUNME_TAG=$(echo "$tag" | cut -d' ' -f3-)

      echo "*** Testing '$RUNME_TAG' in file '$file' *** "
      runme run --filename "$FILE" -t "$RUNME_TAG" --skip-prompts
      echo "*** Testing concluded for '$RUNME_TAG' in file '$file' *** "

      # Save some time by avoiding cleanup for last tag, as the cluster will be deleted anyway.
      [ "$tag" != "${!#}" ] || break

      # Clean up any of our cluster-wide custom resources
      for crd in $(cat chart/crds/sailoperator.io_*.yaml | yq -rN 'select(.kind == "CustomResourceDefinition" and .spec.scope == "Cluster") | .metadata.name'); do
        for cr in $(kubectl get "$crd" -o name); do
          kubectl delete "$cr"
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

# Run tests on a single cluster for all tags that we found
run_tests "${TAGS_LIST[@]}"

# Run dual stack tests on it's own cluster, since it needs to be deployed with support for dual stack
if [[ -n "$dual_stack_tag" ]]; then
  IP_FAMILY="dual" run_tests "$dual_stack_tag"
fi
