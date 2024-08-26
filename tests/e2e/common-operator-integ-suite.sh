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

  if [ "${SKIP_BUILD}" == "true" ]; then
    echo "Skipping build"
  fi

  if [ "${SKIP_DEPLOY}" == "true" ]; then
    echo "Skipping deploy"
  fi

  if [ "${OLM}" == "true" ]; then
    echo "OLM deployment enabled"
    if [ "${OCP}" == "true" ]; then
      echo "OLM deploy test is being skipped on OCP due errors with the workaround to avoid the certificate issue"
      exit 1
    fi
  fi
}

initialize_variables() {
  WD=$(dirname "$0")
  WD=$(cd "${WD}" || exit; pwd)

  VERSIONS_YAML_FILE=${VERSIONS_YAML_FILE:-"versions.yaml"}
  NAMESPACE="${NAMESPACE:-sail-operator}"
  DEPLOYMENT_NAME="${DEPLOYMENT_NAME:-sail-operator}"
  CONTROL_PLANE_NS="${CONTROL_PLANE_NS:-istio-system}"
  COMMAND="kubectl"
  ARTIFACTS="${ARTIFACTS:-$(mktemp -d)}"
  KUBECONFIG="${KUBECONFIG:-"${ARTIFACTS}/config"}"
  # Set the location to install dependencies
  LOCALBIN="${LOCALBIN:-$(pwd)/bin}"

  # Create the directory if it doesn't exist
  mkdir -p "${LOCALBIN}"

  # Define the path for the operator-sdk binary
  OPERATOR_SDK="${LOCALBIN}/operator-sdk"

  if [ "${OCP}" == "true" ]; then
    COMMAND="oc"
  fi

  echo "Using command: ${COMMAND}"

  if [ "${OCP}" == "true" ]; then
    ISTIO_MANIFEST="chart/samples/istio-sample-openshift.yaml"
  else
    ISTIO_MANIFEST="chart/samples/istio-sample-kubernetes.yaml"
  fi

  echo "Setting Istio manifest file: ${ISTIO_MANIFEST}"
  ISTIO_NAME=$(yq eval '.metadata.name' "${WD}/../../$ISTIO_MANIFEST")

  TIMEOUT="3m"

  VERBOSE=${VERBOSE:-"false"}
}

get_internal_registry() {
  # Validate that the internal registry is running in the OCP Cluster, configure the variable to be used in the make target. 
  # If there is no internal registry, the test can't be executed targeting to the internal registry

  # Check if the registry pods are running
  ${COMMAND} get pods -n openshift-image-registry --no-headers | grep -v "Running\|Completed" && echo "It looks like the OCP image registry is not deployed or Running. This tests scenario requires it. Aborting." && exit 1

  # Check if default route already exist
  if [ -z "$(${COMMAND} get route default-route -n openshift-image-registry -o name)" ]; then
    echo "Route default-route does not exist, patching DefaultRoute to true on Image Registry."
    ${COMMAND} patch configs.imageregistry.operator.openshift.io/cluster --patch '{"spec":{"defaultRoute":true}}' --type=merge
  
    timeout --foreground -v -s SIGHUP -k ${TIMEOUT} ${TIMEOUT} bash --verbose -c \
      "until ${COMMAND} get route default-route -n openshift-image-registry &> /dev/null; do sleep 5; done && echo 'The 'default-route' has been created.'"
  fi

  # Get the registry route
  URL=$(${COMMAND} get route default-route -n openshift-image-registry --template='{{ .spec.host }}')
  # Hub will be equal to the route url/project-name(NameSpace) 
  export HUB="${URL}/${NAMESPACE}"
  echo "Internal registry URL: ${HUB}"

  # Create namespace where operator will be located
  # This is needed because the roles are created in the namespace where the operator is deployed
  ${COMMAND} create namespace "${NAMESPACE}" || true

  # Adding roles to avoid the need to be authenticated to push images to the internal registry
  # Using envsubst to replace the variable NAMESPACE in the yaml file
  envsubst < "${WD}/config/role-bindings.yaml" | ${COMMAND} apply -f -

  # Login to the internal registry when running on CRC
  # Take into count that you will need to add before the registry URL as Insecure registry in "/etc/docker/daemon.json"
  if [[ ${URL} == *".apps-crc.testing"* ]]; then
    echo "Executing Docker login to the internal registry"
    if ! oc whoami -t | docker login -u "$(${COMMAND} whoami)" --password-stdin "${URL}"; then
      echo "***** Error: Failed to log in to Docker registry."
      echo "***** Check the error and if is related to 'tls: failed to verify certificate' please add the registry URL as Insecure registry in '/etc/docker/daemon.json'"
      exit 1
    fi
  fi
}

build_and_push_operator_image() {
  # Build and push docker image
  # Notes: to be able to build and push to the local registry we need to set these variables to be used in the Makefile
  # IMAGE ?= ${HUB}/${IMAGE_BASE}:${TAG}, so we need to pass hub, image_base, and tag to be able to build and push the image
  echo "Building and pushing image"
  echo "Image base: ${IMAGE_BASE}"
  echo " Tag: ${TAG}"

  # Check the current architecture to build the image for the same architecture
  # For now we are only building for arm64 and x86_64 because z and p are not supported by the operator yet.
  DOCKER_BUILD_FLAGS=""
  TARGET_ARCH=$(uname -m)

  if [[ "$TARGET_ARCH" == "aarch64" || "$TARGET_ARCH" == "arm64" ]]; then
      echo "Running on arm64 architecture"
      TARGET_ARCH="arm64"
      DOCKER_BUILD_FLAGS="--platform=linux/${TARGET_ARCH}"
  elif [[ "$TARGET_ARCH" == "x86_64" || "$TARGET_ARCH" == "amd64" ]]; then
      echo "Running on x86_64 architecture"
      TARGET_ARCH="amd64"
  else
      echo "Unsupported architecture: ${TARGET_ARCH}"
      exit 1
  fi

  # running docker build inside another container layer causes issues with bind mounts
  BUILD_WITH_CONTAINER=0 DOCKER_BUILD_FLAGS=${DOCKER_BUILD_FLAGS} IMAGE=${HUB}/${IMAGE_BASE}:${TAG} TARGET_ARCH=${TARGET_ARCH} make docker-push
}

# PRE SETUP: Get arguments and initialize variables
check_arguments "$@"
parse_flags "$@"
initialize_variables

if [ "${SKIP_BUILD}" == "false" ]; then
  # SETUP
  if [ "${OCP}" == "true" ]; then
    # Internal Registry is only available in OCP clusters
    get_internal_registry
  fi

  build_and_push_operator_image

  # If OLM is enabled, deploy the operator using OLM
  if [ "${OLM}" == "true" ] && [ "${SKIP_DEPLOY}" == "false" ]; then
    # Set the platform to Kubernetes by default.
    # We are skipping the deploy via OLM test on OCP because the workaround to avoid the certificate issue is not working.
    # Jora ticket related to the limitation: https://issues.redhat.com/browse/OSSM-7993
    HELM_PLATFORM="kubernetes"
    
    # Install OLM in the cluster because it's not available by default in kind.
    ${OPERATOR_SDK} olm install

    # Set image-related variables
    IMAGE_TAG_BASE="${HUB}/${IMAGE_BASE}"
    BUNDLE_IMG="${IMAGE_TAG_BASE}-bundle:v${VERSION}"

    # Deploy the operator using OLM
    IMAGE="${HUB}/${IMAGE_BASE}:${TAG}" \
    IMAGE_TAG_BASE="${IMAGE_TAG_BASE}" \
    BUNDLE_IMG="${BUNDLE_IMG}" \
    HELM_PLATFORM="${HELM_PLATFORM}" \
    make bundle bundle-build bundle-push

    # Create operator namespace
    ${COMMAND} create ns "${NAMESPACE}" || echo "Creation of namespace ${NAMESPACE} failed with the message: $?"
    # Deploy the operator using OLM
    ${OPERATOR_SDK} run bundle "${BUNDLE_IMG}" -n "${NAMESPACE}" --skip-tls

    # Wait for the operator to be ready
    ${COMMAND} wait --for=condition=available deployment/"${DEPLOYMENT_NAME}" -n "${NAMESPACE}" --timeout=5m

    # Set SKIP_DEPLOY to true to avoid deploying the operator again
    SKIP_DEPLOY=true
  fi
fi

if [ "${OCP}" == "true" ]; then
  # This is a workaround
  # To avoid errors of certificates meanwhile we are pulling the operator image from the internal registry
  # We need to set image $HUB to a fixed known value after the push
  # This value always will be equal to the svc url of the internal registry
  HUB="image-registry.openshift-image-registry.svc:5000/sail-operator"
fi

# Run the go test passing the env variables defined that are going to be used in the operator tests
# shellcheck disable=SC2086
IMAGE="${HUB}/${IMAGE_BASE}:${TAG}" SKIP_DEPLOY="${SKIP_DEPLOY}" OCP="${OCP}" ISTIO_MANIFEST="${ISTIO_MANIFEST}" \
NAMESPACE="${NAMESPACE}" CONTROL_PLANE_NS="${CONTROL_PLANE_NS}" DEPLOYMENT_NAME="${DEPLOYMENT_NAME}" \
ISTIO_NAME="${ISTIO_NAME}" COMMAND="${COMMAND}" VERSIONS_YAML_FILE="${VERSIONS_YAML_FILE}" KUBECONFIG="${KUBECONFIG}" \
go run github.com/onsi/ginkgo/v2/ginkgo -tags e2e --timeout 30m --junit-report=report.xml ${GINKGO_FLAGS} "${WD}"/...
