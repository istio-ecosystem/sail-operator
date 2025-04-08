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
      --ocp) shift; OCP=true ;;
      --kind) shift; OCP=false ;;
      --multicluster) shift; MULTICLUSTER=true ;;
      --skip-build) shift; SKIP_BUILD=true ;;
      --skip-deploy) shift; SKIP_BUILD=true; SKIP_DEPLOY=true ;;
      --olm) shift; OLM=true ;;
      --describe)
        shift
        DESCRIBE=true ;;
      *) echo "Invalid flag: $1"; exit 1 ;;
    esac
  done
}

  if [ "${DESCRIBE}" == "true" ]; then
    WD=$(dirname "$0")
    while IFS= read -r -d '' file; do
      if [[ $file == *"_test.go" ]]; then
        go run github.com/onsi/ginkgo/v2/ginkgo outline -format indent "${file}"
      fi
    done < <(find "${WD}" -type f -name "*_test.go" -print0)
    exit 0
  fi

initialize_variables() {
  VERSIONS_YAML_FILE=${VERSIONS_YAML_FILE:-"versions.yaml"}
  VERSIONS_YAML_DIR=${VERSIONS_YAML_DIR:-"pkg/istioversions"}
  NAMESPACE="${NAMESPACE:-sail-operator}"
  DEPLOYMENT_NAME="${DEPLOYMENT_NAME:-sail-operator}"
  CONTROL_PLANE_NS="${CONTROL_PLANE_NS:-istio-system}"
  COMMAND="kubectl"
  ARTIFACTS="${ARTIFACTS:-$(mktemp -d)}"
  KUBECONFIG="${KUBECONFIG:-"${ARTIFACTS}/config"}"
  ISTIOCTL="${ISTIOCTL:-"istioctl"}"
  LOCALBIN="${LOCALBIN:-${HOME}/bin}"
  OPERATOR_SDK=${LOCALBIN}/operator-sdk
  IP_FAMILY=${IP_FAMILY:-ipv4}
  ISTIO_MANIFEST="chart/samples/istio-sample.yaml"

  IMAGE_BASE="${IMAGE_BASE:-sail-operator}"
  TAG="${TAG:-latest}"
  HUB="${HUB:-localhost:5000}"

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

  if [ "${OLM}" == "true" ] && [ "${SKIP_DEPLOY}" == "false" ]; then    
    IMAGE_TAG_BASE="${HUB}/${IMAGE_BASE}"
    BUNDLE_IMG="${IMAGE_TAG_BASE}-bundle:v${VERSION}"

    IMAGE="${HUB}/${IMAGE_BASE}:${TAG}" \
    IMAGE_TAG_BASE="${IMAGE_TAG_BASE}" \
    BUNDLE_IMG="${BUNDLE_IMG}" \
    OPENSHIFT_PLATFORM=false \
    make bundle bundle-build bundle-push

    ${OPERATOR_SDK} olm install

    ${COMMAND} create ns "${NAMESPACE}" || true
    ${OPERATOR_SDK} run bundle "${BUNDLE_IMG}" -n "${NAMESPACE}" --skip-tls --timeout 5m || exit 1

    ${COMMAND} wait --for=condition=available deployment/"${DEPLOYMENT_NAME}" -n "${NAMESPACE}" --timeout=5m

    SKIP_DEPLOY=true
  fi
fi

if [ "${OCP}" == "true" ]; then
  HUB="image-registry.openshift-image-registry.svc:5000/sail-operator"
fi

export SKIP_DEPLOY IP_FAMILY ISTIO_MANIFEST NAMESPACE CONTROL_PLANE_NS DEPLOYMENT_NAME MULTICLUSTER ARTIFACTS ISTIO_NAME COMMAND KUBECONFIG ISTIOCTL_PATH

IMAGE="${HUB}/${IMAGE_BASE}:${TAG}" \
go run github.com/onsi/ginkgo/v2/ginkgo -tags e2e \
--timeout 60m --junit-report=report.xml ${GINKGO_FLAGS} "${WD}"/...
