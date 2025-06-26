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
  OLM=${OLM:-false}
  DESCRIBE=false
  MULTICLUSTER=false
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
    if [ "${OCP}" == "true" ]; then
      echo "Skipping operator deployment using OLM on OCP clusters due to certificate issues with the internal registry."
      exit 1
    fi
  fi
}

initialize_variables() {
  VERSIONS_YAML_FILE=${VERSIONS_YAML_FILE:-"versions.yaml"}
  VERSIONS_YAML_DIR=${VERSIONS_YAML_DIR:-"pkg/istioversions"}
  NAMESPACE="${NAMESPACE:-sail-operator}"
  DEPLOYMENT_NAME="${DEPLOYMENT_NAME:-sail-operator}"
  CONTROL_PLANE_NS="${CONTROL_PLANE_NS:-istio-system}"
  COMMAND="kubectl"
  ARTIFACTS="${ARTIFACTS:-$(mktemp -d)}"
  KUBECONFIG="${KUBECONFIG:-"${ARTIFACTS}/config"}"
  ISTIOCTL_PATH="${ISTIOCTL:-"istioctl"}"
  LOCALBIN="${LOCALBIN:-${HOME}/bin}"
  OPERATOR_SDK=${LOCALBIN}/operator-sdk
  IP_FAMILY=${IP_FAMILY:-ipv4}
  ISTIO_MANIFEST="chart/samples/istio-sample.yaml"

  # export to be sure that the variables are available in the subshell
  export IMAGE_BASE="${IMAGE_BASE:-sail-operator}"
  export TAG="${TAG:-latest}"
  export HUB="${HUB:-localhost:5000}"

  echo "Setting Istio manifest file: ${ISTIO_MANIFEST}"
  ISTIO_NAME=$(yq eval '.metadata.name' "${WD}/../../$ISTIO_MANIFEST")

  if [ "${OCP}" == "true" ]; then COMMAND="oc"; fi
}

# Main script flow
check_arguments "$@"
parse_flags "$@"
initialize_variables

# Export necessary vars
export COMMAND OCP HUB IMAGE_BASE TAG NAMESPACE

if [ "${SKIP_BUILD}" == "false" ]; then
  "${WD}/setup/build-and-push-operator.sh"

  if [ "${OCP}" = "true" ]; then
    # This is a workaround when pulling the image from internal registry
    # To avoid errors of certificates meanwhile we are pulling the operator image from the internal registry
    # We need to set image $HUB to a fixed known value after the push
    # This value always will be equal to the svc url of the internal registry
    HUB="image-registry.openshift-image-registry.svc:5000/istio-images"
    echo "Using internal registry: ${HUB}"

    # Workaround for OCP helm operator installation issues:
    # To avoid any cleanup issues, after we build and push the image we check if the namespace exists and delete it if it does.
    # The test logic already handles the namespace creation and deletion during the test run. 
    if ${COMMAND} get ns "${NAMESPACE}" &>/dev/null; then
      echo "Namespace ${NAMESPACE} already exists. Deleting it to avoid conflicts."
      ${COMMAND} delete ns "${NAMESPACE}"
    fi
  fi
  # If OLM is enabled, deploy the operator using OLM
  # We are skipping the deploy via OLM test on OCP because the workaround to avoid the certificate issue is not working.
  # Jira ticket related to the limitation: https://issues.redhat.com/browse/OSSM-7993
  if [ "${OLM}" == "true" ] && [ "${SKIP_DEPLOY}" == "false" ] && [ "${MULTICLUSTER}" == "false" ]; then    
    IMAGE_TAG_BASE="${HUB}/${IMAGE_BASE}"
    BUNDLE_IMG="${IMAGE_TAG_BASE}-bundle:v${VERSION}"

    IMAGE="${HUB}/${IMAGE_BASE}:${TAG}" \
    IMAGE_TAG_BASE="${IMAGE_TAG_BASE}" \
    BUNDLE_IMG="${BUNDLE_IMG}" \
    OPENSHIFT_PLATFORM=false \
    make bundle bundle-build bundle-push

    # Install OLM in the cluster because it's not available by default in kind.
    OLM_INSTALL_ARGS=""
    if [ "${OLM_VERSION}" != "" ]; then
      OLM_INSTALL_ARGS+="--version ${OLM_VERSION}"
    fi

    # Ensure kubeconfig is set to the kind cluster
    kind export kubeconfig --name="${KIND_CLUSTER_NAME}"
    # shellcheck disable=SC2086
    ${OPERATOR_SDK} olm install ${OLM_INSTALL_ARGS}

    # Wait for for the CatalogSource to be CatalogSource.status.connectionState.lastObservedState == READY
    ${COMMAND} wait catalogsource operatorhubio-catalog -n olm --for 'jsonpath={.status.connectionState.lastObservedState}=READY' --timeout=5m

    ${COMMAND} create ns "${NAMESPACE}" || true
    ${OPERATOR_SDK} run bundle "${BUNDLE_IMG}" -n "${NAMESPACE}" --skip-tls --timeout 5m || exit 1

    ${COMMAND} wait --for=condition=available deployment/"${DEPLOYMENT_NAME}" -n "${NAMESPACE}" --timeout=5m

    SKIP_DEPLOY=true
  fi
fi

export SKIP_DEPLOY IP_FAMILY ISTIO_MANIFEST NAMESPACE CONTROL_PLANE_NS DEPLOYMENT_NAME MULTICLUSTER ARTIFACTS ISTIO_NAME COMMAND KUBECONFIG ISTIOCTL_PATH

# shellcheck disable=SC2086
IMAGE="${HUB}/${IMAGE_BASE}:${TAG}" \
go run github.com/onsi/ginkgo/v2/ginkgo -tags e2e \
--timeout 60m --junit-report=report.xml ${GINKGO_FLAGS} "${WD}"/...
