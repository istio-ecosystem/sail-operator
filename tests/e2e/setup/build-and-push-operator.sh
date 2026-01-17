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
echo "Starting build and push of operator image..."
echo "HUB: ${HUB}"
echo "IMAGE_BASE: ${IMAGE_BASE}"
echo "TAG: ${TAG}"

build_and_push_operator_image