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

WD=$(dirname "$0")
WD=$(cd "${WD}" || exit; pwd)

# The following variables must already be set and exported from the caller:
# COMMAND, HUB, IMAGE_BASE, TAG, NAMESPACE

# OCP-specific registry logic
get_internal_registry() {
  ${COMMAND} get pods -n openshift-image-registry --no-headers | grep -v "Running\|Completed" && \
    echo "OCP image registry is not running. Aborting." && exit 1

  if [ -z "$(${COMMAND} get route default-route -n openshift-image-registry -o name)" ]; then
    echo "Creating default route for image registry..."
    ${COMMAND} patch configs.imageregistry.operator.openshift.io/cluster \
      --patch '{"spec":{"defaultRoute":true}}' --type=merge
  
    timeout 3m bash -c \
      "until ${COMMAND} get route default-route -n openshift-image-registry &>/dev/null; do sleep 5; done"
  fi

  URL=$(${COMMAND} get route default-route -n openshift-image-registry --template='{{ .spec.host }}')
  export HUB="${URL}/${NAMESPACE}"
  echo "Registry URL: ${HUB}"

  ${COMMAND} create namespace "${NAMESPACE}" || true
  envsubst < "${WD}/config/role-bindings.yaml" | ${COMMAND} apply -f -

  if [[ ${URL} == *".apps-crc.testing"* ]]; then
    echo "Logging into internal registry..."
    if ! oc whoami -t | docker login -u "$(${COMMAND} whoami)" --password-stdin "${URL}"; then
      echo "Failed to log in to Docker registry."
      exit 1
    fi
  fi
}

build_and_push_operator_image() {
  echo "Building and pushing operator image: ${HUB}/${IMAGE_BASE}:${TAG}"

  DOCKER_BUILD_FLAGS=""
  TARGET_ARCH=${TARGET_ARCH:-$(uname -m)}

  if [[ "$TARGET_ARCH" == "aarch64" || "$TARGET_ARCH" == "arm64" ]]; then
      TARGET_ARCH="arm64"
      DOCKER_BUILD_FLAGS="--platform=linux/${TARGET_ARCH}"
  elif [[ "$TARGET_ARCH" == "x86_64" || "$TARGET_ARCH" == "amd64" ]]; then
      TARGET_ARCH="amd64"
      DOCKER_BUILD_FLAGS="--platform=linux/${TARGET_ARCH}"
  else
      echo "Unsupported architecture: ${TARGET_ARCH}"
      exit 1
  fi

  echo "Building for architecture: ${TARGET_ARCH}"
  echo "Docker build flags: ${DOCKER_BUILD_FLAGS}"
  echo "Using image base: ${HUB}/${IMAGE_BASE}:${TAG}"

  BUILD_WITH_CONTAINER=0 \
    DOCKER_BUILD_FLAGS=${DOCKER_BUILD_FLAGS} \
    IMAGE=${HUB}/${IMAGE_BASE}:${TAG} \
    TARGET_ARCH=${TARGET_ARCH} \
    make docker-push
}

# Main logic
if [ "${OCP}" == "true" ]; then
  get_internal_registry
fi

build_and_push_operator_image
