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

# To be able to run this script, it's needed to pass the flag --ocp or --kind
set -eu -o pipefail

WD=$(dirname "$0")
WD=$(cd "${WD}" || exit; pwd)

check_arguments() {
  if [ $# -eq 0 ]; then
    echo "No arguments provided"
    exit 1
  fi
}

parse_flags() {
  SKIP_BUILD=${SKIP_BUILD:-false}
  SKIP_DEPLOY=${SKIP_DEPLOY:-false}
  SKIP_CLEANUP=${SKIP_CLEANUP:-false}
  OLM=${OLM:-false}
  PREPARE_ONLY=false
  TEST_ONLY=false
  DESCRIBE=false
  MULTICLUSTER=${MULTICLUSTER:-false}
  while [ $# -gt 0 ]; do
    case "$1" in
      --ocp)
        shift
        OCP=true
        ;;
      --kind)
        shift
        OCP=false
        ;;
      --multicluster)
        shift
        MULTICLUSTER=true
        ;;
      --skip-build)
        shift
        SKIP_BUILD=true
        ;;
      --skip-deploy)
        shift
        # no point building if we don't deploy
        SKIP_BUILD=true
        SKIP_DEPLOY=true
        ;;
      --olm)
        shift
        OLM=true
        ;;
      --prepare-only)
        shift
        PREPARE_ONLY=true
        ;;
      --test-only)
        shift
        TEST_ONLY=true
        SKIP_BUILD=true
        SKIP_DEPLOY=true
        ;;
      --describe)
        shift
        DESCRIBE=true
        ;;
      *)
        echo "Invalid flag: $1"
        exit 1
        ;;
    esac
  done

  if [ "${PREPARE_ONLY}" == "true" ] && [ "${TEST_ONLY}" == "true" ]; then
    echo "--prepare-only and --test-only are mutually exclusive" >&2
    exit 1
  fi

  if [ "${PREPARE_ONLY}" == "true" ]; then
    # Invalidate any state from an earlier attempt before rejecting incompatible
    # skip settings. Otherwise a failed preparation can leave a valid-looking
    # handoff for the consumer phase.
    validate_preparation_state_path
    if [ "${SKIP_BUILD}" != "false" ]; then
      echo "--prepare-only requires SKIP_BUILD=false; refusing to publish unprepared state" >&2
      exit 1
    fi
    if [ "${SKIP_DEPLOY}" != "false" ]; then
      echo "--prepare-only requires SKIP_DEPLOY=false; refusing to publish unprepared state" >&2
      exit 1
    fi
  fi

  if [ "${DESCRIBE}" == "true" ]; then
    WD=$(dirname "$0")
    while IFS= read -r -d '' file; do
      if [[ $file == *"_test.go" ]]; then
        go run github.com/onsi/ginkgo/v2/ginkgo outline -format indent "${file}"
      fi
    done < <(find "${WD}" -type f -name "*_test.go" -print0)
    exit 0
  fi

  if [ "${OCP}" == "true" ]; then
    echo "Running on OCP"
  else
    echo "Running on kind"
  fi

  if [ "${MULTICLUSTER}" == "true" ]; then
    echo "Running on multicluster"
  fi

  if [ "${SKIP_BUILD}" == "true" ]; then
    echo "Skipping build"
  fi

  if [ "${SKIP_DEPLOY}" == "true" ]; then
    echo "Skipping deploy"
  fi

  if [ "${OLM}" == "true" ]; then
    echo "OLM deployment enabled"
  fi

  if [ "${PREPARE_ONLY}" == "true" ]; then
    echo "Preparation-only mode enabled"
  elif [ "${TEST_ONLY}" == "true" ]; then
    echo "Test-only mode enabled"
  fi
}

validate_preparation_state_path() {
  if [ -z "${E2E_STATE_FILE:-}" ]; then
    echo "E2E_STATE_FILE must be set in preparation-only mode" >&2
    return 1
  fi

  local state_dir
  local state_probe
  state_dir=$(dirname "${E2E_STATE_FILE}")
  state_probe="${E2E_STATE_FILE}.probe.$$"
  if ! mkdir -p "${state_dir}"; then
    echo "Cannot create preparation state directory ${state_dir}" >&2
    return 1
  fi
  if ! rm -f "${E2E_STATE_FILE}" "${E2E_STATE_FILE}.tmp"; then
    echo "Cannot invalidate previous preparation state ${E2E_STATE_FILE}" >&2
    return 1
  fi
  if [ -e "${E2E_STATE_FILE}" ] || [ -e "${E2E_STATE_FILE}.tmp" ]; then
    echo "Previous preparation state still exists at ${E2E_STATE_FILE}" >&2
    return 1
  fi
  if ! (umask 077 && : > "${state_probe}"); then
    echo "Cannot write preparation state in ${state_dir}" >&2
    return 1
  fi
  if ! rm -f "${state_probe}"; then
    echo "Cannot remove preparation state probe ${state_probe}" >&2
    return 1
  fi
}

initialize_variables() {
  VERSIONS_YAML_FILE=${VERSIONS_YAML_FILE:-"versions.yaml"}
  VERSIONS_YAML_DIR=${VERSIONS_YAML_DIR:-"pkg/istioversion"}
  NAMESPACE="${NAMESPACE:-sail-operator}"
  DEPLOYMENT_NAME="${DEPLOYMENT_NAME:-sail-operator}"
  OPERATOR_NAME="${OPERATOR_NAME:-sailoperator}"
  CONTROL_PLANE_NS="${CONTROL_PLANE_NS:-istio-system}"
  COMMAND="kubectl"
  ARTIFACTS="${ARTIFACTS:-$(mktemp -d)}"
  KUBECONFIG="${KUBECONFIG:-"${ARTIFACTS}/config"}"
  ISTIOCTL_PATH="${ISTIOCTL:-"istioctl"}"
  LOCALBIN="${LOCALBIN:-${HOME}/bin}"
  OPERATOR_SDK=${LOCALBIN}/operator-sdk
  IP_FAMILY=${IP_FAMILY:-ipv4}
  ISTIO_MANIFEST="chart/samples/istio-sample.yaml"
  CI=${CI:-"false"}
  USE_INTERNAL_REGISTRY=${USE_INTERNAL_REGISTRY:-"false"}
  FIPS_CLUSTER=${FIPS_CLUSTER:-"false"}
  # In Prow CI, HEAD is a temporary merge commit not visible in the PR history.
  # PULL_PULL_SHA holds the actual PR head commit — use it when available.
  if [ -n "${PULL_PULL_SHA:-}" ]; then
    COMMIT_HASH="${PULL_PULL_SHA:0:8}"
  else
    COMMIT_HASH=$(git rev-parse --short HEAD)
  fi
  echo "DEBUG commit hash resolution:"
  echo "  PULL_PULL_SHA: ${PULL_PULL_SHA:-not set}"
  echo "  git HEAD (full): $(git rev-parse HEAD)"
  echo "  COMMIT_HASH resolved to: ${COMMIT_HASH}"

  # Debug logging and fallback for GINKGO_FLAGS
  echo "CI environment: ${CI}"
  echo "GINKGO_FLAGS received: '${GINKGO_FLAGS:-}'"

  # Fallback: Generate GINKGO_FLAGS if empty and CI=true
  if [ -z "${GINKGO_FLAGS:-}" ] && [ "${CI}" == "true" ]; then
    GINKGO_FLAGS="--no-color"
    echo "Generated GINKGO_FLAGS fallback: '${GINKGO_FLAGS}'"
  fi

  # export to be sure that the variables are available in the subshell
  export IMAGE_BASE="${IMAGE_BASE:-sail-operator}"
  export TAG="${TAG:-latest}"
  export HUB="${HUB:-localhost:5000}"

  # Handle OCP registry scenarios
  # Note: Makefile.core.mk sets HUB=quay.io/sail-dev and TAG=1.29-latest by default
  if [ "${OCP}" == "true" ] && [ "${TEST_ONLY}" != "true" ]; then
    # Debug output for troubleshooting
    echo "DEBUG: CI='${CI}', HUB='${HUB}'"

    if [ "${CI}" == "true" ] && [ "${HUB}" == "quay.io/sail-dev" ]; then
      # Scenario 2: CI mode with default HUB -> use external registry with proper CI tag
      echo "CI mode detected for OCP, using external registry ${HUB}"
      export USE_INTERNAL_REGISTRY="false"
      # Use PR_NUMBER and commit hash to identify the image, avoid race conditions in CI when multiple runs are pushing to the same default tag
      # Use TARGET_ARCH to differentiate tags for different architectures in CI
      if [ -n "${PR_NUMBER:-}" ]; then
        TAG="pr-${PR_NUMBER}-${COMMIT_HASH}-${TARGET_ARCH}"
        export TAG
        echo "Using PR-based tag: ${TAG}"
      else
        TAG="ci-test-${COMMIT_HASH}-${TARGET_ARCH}"
        export TAG
        echo "Using commit-based tag: ${TAG}"
      fi
    elif [ "${CI}" == "true" ]; then
      # Additional CI mode check - handle CI mode regardless of HUB value
      echo "CI mode detected for OCP with custom HUB (${HUB}), using external registry"
      export USE_INTERNAL_REGISTRY="false"
    elif [ "${HUB}" != "quay.io/sail-dev" ]; then
      # Scenario 3: Custom registry provided by user
      echo "Using custom registry: ${HUB}"
      export USE_INTERNAL_REGISTRY="false"
    else
      # Scenario 1: Local development -> use internal OCP registry
      echo "Local development mode, will use OCP internal registry"
      export USE_INTERNAL_REGISTRY="true"
    fi
  elif [ "${OCP}" == "true" ]; then
    # In test-only mode HUB and TAG describe the image deployed by the separate
    # preparation phase. Do not derive a new architecture-dependent tag here.
    export USE_INTERNAL_REGISTRY="false"
    echo "Using prepared operator image ${HUB}/${IMAGE_BASE}:${TAG}"
  fi

  echo "Setting Istio manifest file: ${ISTIO_MANIFEST}"
  ISTIO_NAME=$(yq eval '.metadata.name' "${WD}/../../$ISTIO_MANIFEST")

  if [ "${OCP}" == "true" ]; then COMMAND="oc"; fi
}

check_cluster_operators() {
  # This function is only relevant for OCP clusters
  if [ "${OCP}" != "true" ]; then
    echo "Skipping ClusterOperator check on non-OCP cluster."
    return 0
  fi

  # Check if jq is installed
  if ! command -v jq &> /dev/null; then
    echo "ERROR: jq is required for the cluster operator health check. Please install jq."
    exit 1
  fi

  local timeout_seconds=600
  echo "Validating OpenShift cluster operators are stable..."
  local end_time=$(( $(date +%s) + timeout_seconds ))

  while [ "$(date +%s)" -lt $end_time ]; do
    # This command uses jq to count operators that are not Available, or are Progressing, or are Degraded.
    # A healthy cluster should have a count of 0.
    local unstable_operators
    unstable_operators=$(oc get clusteroperator -o json | jq '[.items[] | select(.status.conditions[] | (.type == "Available" and .status == "False") or (.type == "Progressing" and .status == "True") or (.type == "Degraded" and .status == "True"))] | length')

    if [[ $unstable_operators -eq 0 ]]; then
      echo "All cluster operators are stable."
      return 0
    fi

    echo -n "."
    sleep 15
  done

  echo "ERROR: Timeout reached. Not all cluster operators are stable."
  oc get clusteroperator # Print the final status for debugging
  exit 1
}

install_operator() {
  echo "Installing sail-operator (KUBECONFIG=${KUBECONFIG})"
  "${COMMAND}" create namespace "${NAMESPACE}"

  # Optional resource overrides for constrained CI (e.g. Openshift Local on ubuntu-latest).
  # Defaults stay in chart/values.yaml; set OPERATOR_*_REQUEST / OPERATOR_*_LIMIT to tune.
  local helm_sets=(
    --set "image=${HUB}/${IMAGE_BASE}:${TAG}"
    --set "operatorLogLevel=3"
  )
  if [ -n "${OPERATOR_MEMORY_REQUEST:-}" ]; then
    helm_sets+=(--set "operator.resources.requests.memory=${OPERATOR_MEMORY_REQUEST}")
  fi
  if [ -n "${OPERATOR_MEMORY_LIMIT:-}" ]; then
    helm_sets+=(--set "operator.resources.limits.memory=${OPERATOR_MEMORY_LIMIT}")
  fi
  if [ -n "${OPERATOR_CPU_REQUEST:-}" ]; then
    helm_sets+=(--set "operator.resources.requests.cpu=${OPERATOR_CPU_REQUEST}")
  fi
  if [ -n "${OPERATOR_CPU_LIMIT:-}" ]; then
    helm_sets+=(--set "operator.resources.limits.cpu=${OPERATOR_CPU_LIMIT}")
  fi

  echo "Helm install overrides: ${helm_sets[*]}"
  helm install sail-operator "${SOURCE_DIR}"/chart --namespace "${NAMESPACE}" "${helm_sets[@]}"
}

await_operator() {
  echo "Awaiting operator deployment on (KUBECONFIG=${KUBECONFIG})"
  local name="${DEPLOYMENT_NAME}"
  local timeout="${OPERATOR_DEPLOY_TIMEOUT:-5m}"
  if [ "${OLM}" == "true" ]; then
    local csv_name
    local csv_file
    csv_file=$(find "${WD}/../../bundle/manifests/" -name "*.clusterserviceversion.yaml" | head -1)
    csv_name=$(yq eval '.spec.install.spec.deployments[0].name' "${csv_file}" 2>/dev/null || true)
    if [ -n "${csv_name}" ]; then
      echo "OLM mode: using deployment name from bundle CSV: ${csv_name}"
      name="${csv_name}"
      DEPLOYMENT_NAME="${csv_name}"
    fi
  fi
  if ! "${COMMAND}" wait --for=condition=available deployment/"${name}" -n "${NAMESPACE}" --timeout="${timeout}"; then
    # If the deployment is not available within the timeout, dump relevant information for diagnosis
    # We need this because it's executed before e2e test run so we will not have any debug information if fails the deployment.
    echo "ERROR: operator deployment/${name} not Available within ${timeout}; dumping status for diagnosis"
    "${COMMAND}" get deployment,pods -n "${NAMESPACE}" -o wide || true
    "${COMMAND}" describe deployment/"${name}" -n "${NAMESPACE}" || true
    "${COMMAND}" describe pods -n "${NAMESPACE}" || true
    "${COMMAND}" get events -n "${NAMESPACE}" --sort-by='.lastTimestamp' || true
    return 1
  fi
}

# shellcheck disable=SC2317,SC2329  # Function is invoked indirectly via trap
uninstall_operator() {
  echo "Uninstalling sail-operator (KUBECONFIG=${KUBECONFIG})"
  helm uninstall sail-operator --namespace "${NAMESPACE}"
  "${COMMAND}" delete namespace "${NAMESPACE}"
}

# shellcheck disable=SC2317,SC2329  # Function is invoked indirectly via cleanup.
uninstall_olm_operator() {
  echo "Uninstalling OLM-managed sail-operator (KUBECONFIG=${KUBECONFIG})"
  "${OPERATOR_SDK}" cleanup "${OPERATOR_NAME}" --delete-all -n "${NAMESPACE}"
  "${COMMAND}" delete namespace "${NAMESPACE}" --ignore-not-found
}

write_preparation_state() {
  local state_tmp
  state_tmp="${E2E_STATE_FILE}.tmp"
  jq -n \
    --arg hub "${HUB}" \
    --arg tag "${TAG}" \
    --arg imageBase "${IMAGE_BASE}" \
    --arg namespace "${NAMESPACE}" \
    --arg olm "${OLM}" \
    --arg deploymentName "${DEPLOYMENT_NAME}" \
    --arg targetArch "${TARGET_ARCH:-}" \
    '{schemaVersion: 1, hub: $hub, tag: $tag, imageBase: $imageBase, namespace: $namespace, olm: $olm, deploymentName: $deploymentName, targetArch: $targetArch}' \
    > "${state_tmp}"
  mv "${state_tmp}" "${E2E_STATE_FILE}"
  echo "Preparation state written to ${E2E_STATE_FILE}"
}

read_preparation_state() {
  if [ -z "${E2E_STATE_FILE:-}" ]; then
    return 0
  fi
  if [ ! -f "${E2E_STATE_FILE}" ]; then
    echo "E2E_STATE_FILE is set to ${E2E_STATE_FILE} but the file does not exist" >&2
    exit 1
  fi
  local schema_version
  schema_version=$(jq -r '.schemaVersion // empty' "${E2E_STATE_FILE}")
  if [ "${schema_version}" != "1" ]; then
    echo "Unsupported preparation state schema version: '${schema_version}' in ${E2E_STATE_FILE}" >&2
    exit 1
  fi
  HUB=$(jq -r '.hub' "${E2E_STATE_FILE}")
  TAG=$(jq -r '.tag' "${E2E_STATE_FILE}")
  IMAGE_BASE=$(jq -r '.imageBase' "${E2E_STATE_FILE}")
  NAMESPACE=$(jq -r '.namespace' "${E2E_STATE_FILE}")
  OLM=$(jq -r '.olm' "${E2E_STATE_FILE}")
  DEPLOYMENT_NAME=$(jq -r '.deploymentName' "${E2E_STATE_FILE}")
  local target_arch
  target_arch=$(jq -r '.targetArch // empty' "${E2E_STATE_FILE}")
  if [ -n "${target_arch}" ]; then
    TARGET_ARCH="${target_arch}"
  fi
  echo "Preparation state loaded from ${E2E_STATE_FILE}"
}

gather_operator_diagnostics() {
  echo "Gathering operator diagnostics before cleanup"
  "${COMMAND}" get deployment,pods -n "${NAMESPACE}" -o wide || true
  "${COMMAND}" get events -n "${NAMESPACE}" --sort-by='.lastTimestamp' || true
}

# Ensure cleanup always runs and that the original test exit code is preserved
# shellcheck disable=SC2317,SC2329  # Function is invoked indirectly via trap
cleanup() {
  # Do not let cleanup errors affect the final exit code
  set +e
  if [ "${SKIP_CLEANUP}" == "true" ] || [ "${PREPARE_ONLY}" == "true" ]; then
    echo "Skipping operator cleanup"
  elif [ "${TEST_ONLY}" == "true" ] && [ "${OLM}" == "true" ]; then
    uninstall_olm_operator || true
  elif [ "${OLM}" != "true" ] && { [ "${SKIP_DEPLOY}" != "true" ] || [ "${TEST_ONLY}" == "true" ]; }; then
    if [ "${MULTICLUSTER}" == true ]; then
      KUBECONFIG="${KUBECONFIG}" uninstall_operator || true
      # shellcheck disable=SC2153  # KUBECONFIG2 is set by multicluster setup scripts
      KUBECONFIG="${KUBECONFIG2}" uninstall_operator || true
    else
      uninstall_operator || true
    fi
  fi
  echo "JUnit report: ${ARTIFACTS}/report.xml"
}

# shellcheck disable=SC2317  # Function is invoked indirectly via signal traps.
handle_signal() {
  local signal_number=$1
  trap - EXIT INT TERM
  cleanup
  exit "$((128 + signal_number))"
}

trap cleanup EXIT
trap 'handle_signal 2' INT
trap 'handle_signal 15' TERM

# Main script flow
check_arguments "$@"
parse_flags "$@"
if [ "${TEST_ONLY}" == "true" ]; then
  read_preparation_state
fi
initialize_variables

# Export necessary vars
export COMMAND OCP HUB IMAGE_BASE TAG NAMESPACE USE_INTERNAL_REGISTRY

if [ "${SKIP_BUILD}" == "false" ]; then
  "${WD}/setup/build-and-push-operator.sh"

  if [ "${OCP}" = "true" ]; then
    # This is a workaround when pulling the image from internal registry
    # To avoid errors of certificates meanwhile we are pulling the operator image from the internal registry
    # We need to set image $HUB to a fixed known value after the push
    # Convert from route URL to service URL format for image pulling
    if [[ "${HUB}" == *"/istio-images" ]]; then
      HUB="image-registry.openshift-image-registry.svc:5000/istio-images"
      echo "Using internal registry service URL: ${HUB}"
    else
      echo "Using external registry: ${HUB}"
    fi

    # Workaround for OCP helm operator installation issues:
    # To avoid any cleanup issues, after we build and push the image we check if the namespace exists and delete it if it does.
    # The test logic already handles the namespace creation and deletion during the test run. 
    if ${COMMAND} get ns "${NAMESPACE}" &>/dev/null; then
      echo "Namespace ${NAMESPACE} already exists. Deleting it to avoid conflicts."
      ${COMMAND} delete ns "${NAMESPACE}"
    fi
  fi
  # If OLM is enabled, deploy the operator using OLM
  # If PR_NUMBER is set we will tag the BUNDLE_IMG with the PR number and commit hash to avoid conflicts.
  if [ "${OLM}" == "true" ] && [ "${SKIP_DEPLOY}" == "false" ] && [ "${MULTICLUSTER}" == "false" ]; then
    IMAGE_TAG_BASE="${HUB}/${IMAGE_BASE}"
    if [ "${CI}" == "true" ]; then
      if [ -n "${PR_NUMBER:-}" ]; then
        BUNDLE_IMG="${IMAGE_TAG_BASE}-bundle:pr-${PR_NUMBER}-${COMMIT_HASH}-${TARGET_ARCH}"
      else
        BUNDLE_IMG="${IMAGE_TAG_BASE}-bundle:ci-test-${COMMIT_HASH}-${TARGET_ARCH}"
      fi
    else
      BUNDLE_IMG="${IMAGE_TAG_BASE}-bundle:ci-test-${COMMIT_HASH}-${TARGET_ARCH}"
    fi

    IMAGE="${HUB}/${IMAGE_BASE}:${TAG}" \
    IMAGE_TAG_BASE="${IMAGE_TAG_BASE}" \
    BUNDLE_IMG="${BUNDLE_IMG}" \
    OCP="${OCP}" \
    make bundle bundle-build bundle-push

    if [ "${OCP}" == "false" ]; then
      # Install OLM in the cluster because it's not available by default in kind.
      OLM_INSTALL_ARGS=""
      if [ "${OLM_VERSION}" != "" ]; then
        OLM_INSTALL_ARGS+="--version ${OLM_VERSION}"
      fi

      # Ensure kubeconfig is set to the kind cluster
      kind export kubeconfig --name="${KIND_CLUSTER_NAME}"
      # shellcheck disable=SC2086
      ${OPERATOR_SDK} olm install ${OLM_INSTALL_ARGS}

      ${COMMAND} wait catalogsource operatorhubio-catalog -n olm --for 'jsonpath={.status.connectionState.lastObservedState}=READY' --timeout=5m
    else
      # On OCP, wait for different CatalogSources as operatorhubio-catalog might not exist
      ${COMMAND} wait catalogsource redhat-operators -n openshift-marketplace --for 'jsonpath={.status.connectionState.lastObservedState}=READY' --timeout=5m || true
    fi

    ${COMMAND} create ns "${NAMESPACE}" || true
    # OLM registry pods (registry-grpc, registry-grpc-init) don't satisfy the
    # restricted:latest PodSecurity policy that OCP 4.23 enforces by default on new namespaces.
    if [ "${OCP}" == "true" ]; then
      ${COMMAND} label namespace "${NAMESPACE}" \
        pod-security.kubernetes.io/enforce=privileged \
        pod-security.kubernetes.io/warn=privileged \
        pod-security.kubernetes.io/audit=privileged --overwrite || true
    fi
    ${OPERATOR_SDK} run bundle "${BUNDLE_IMG}" -n "${NAMESPACE}" --skip-tls --timeout 5m || exit 1

    await_operator

    SKIP_DEPLOY=true
  fi
fi

export SKIP_DEPLOY IP_FAMILY ISTIO_MANIFEST NAMESPACE CONTROL_PLANE_NS DEPLOYMENT_NAME OPERATOR_NAME MULTICLUSTER ARTIFACTS ISTIO_NAME COMMAND KUBECONFIG ISTIOCTL_PATH SKIP_CLEANUP GINKGO_FLAGS FIPS_CLUSTER

if [ "${OLM}" != "true" ] && [ "${SKIP_DEPLOY}" != "true" ]; then
  # shellcheck disable=SC2153
  if [ "${MULTICLUSTER}" == true ]; then
    KUBECONFIG="${KUBECONFIG}" install_operator
    KUBECONFIG="${KUBECONFIG2}" install_operator
    KUBECONFIG="${KUBECONFIG}" await_operator
    KUBECONFIG="${KUBECONFIG2}" await_operator
  else
    install_operator
    await_operator
  fi
fi

# Check that all cluster operators are stable before running the tests. This only applies to OCP clusters.
# This is to avoid test failures due to cluster instability.
check_cluster_operators

if [ "${PREPARE_ONLY}" == "true" ]; then
  write_preparation_state
  echo "Preparation completed; Ginkgo was not started"
  exit 0
fi

set +eu
# Disable errexit to avoid failing the test run before generating the report.xml,
# and disable nounset so that empty LABEL_FILTER_ARGS array expansion is safe on
# all bash versions (bash 3.2 treats empty array[@] as unbound under nounset).
# Capture the test exit code and allow cleanup via trap to run

# GINKGO_LABEL_FILTER provides a dedicated, quoted-safe way to pass a Ginkgo
# label-filter expression so that logical operators (&&, ||, !) are not
# interpreted by the shell. It is ignored when GINKGO_FLAGS already carries a
# --label-filter to preserve backward compatibility with existing callers.
LABEL_FILTER_ARGS=()
if [ -n "${GINKGO_LABEL_FILTER:-}" ] && [[ "${GINKGO_FLAGS:-}" != *"--label-filter"* ]]; then
  LABEL_FILTER_ARGS=("--label-filter=${GINKGO_LABEL_FILTER}")
fi

# shellcheck disable=SC2086,SC2068
IMAGE="${HUB}/${IMAGE_BASE}:${TAG}" \
go run github.com/onsi/ginkgo/v2/ginkgo -tags e2e \
--timeout 60m --junit-report="${ARTIFACTS}/report.xml" ${GINKGO_FLAGS:-} "${LABEL_FILTER_ARGS[@]}" "${WD}"/...
TEST_EXIT_CODE=$?

if [ "${TEST_EXIT_CODE}" -ne 0 ]; then
  gather_operator_diagnostics
fi

exit "${TEST_EXIT_CODE}"
