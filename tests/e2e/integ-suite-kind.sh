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

SCRIPTPATH="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOT="$(dirname "$(dirname "${SCRIPTPATH}")")"

export ARTIFACTS="${ARTIFACTS:-$(mktemp -d)}"
export MULTICLUSTER="${MULTICLUSTER:-false}"
export IP_FAMILY="${IP_FAMILY:-ipv4}"
export ISTIOCTL="${ISTIOCTL:-${ROOT}/bin/istioctl}"
export KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-operator-integration-tests}"

function check_prerequisites() {
  if ! command -v "${ISTIOCTL}" &> /dev/null; then
    echo "istioctl not found. Please set ISTIOCTL to the path of istioctl binary"
    exit 1
  fi
}

function run_integration_tests() {
  echo "Running integration tests"
  if [ "${MULTICLUSTER}" == "true" ]; then
    ARTIFACTS="${ARTIFACTS}" ISTIOCTL="${ISTIOCTL}" "${ROOT}/tests/e2e/common-operator-integ-suite.sh" --kind --multicluster
  else
    ARTIFACTS="${ARTIFACTS}" IP_FAMILY="${IP_FAMILY}" "${ROOT}/tests/e2e/common-operator-integ-suite.sh" --kind
  fi
}

check_prerequisites

source "${ROOT}/tests/e2e/setup/setup-kind.sh"

# Use the local registry instead of the default HUB
export HUB="${KIND_REGISTRY}"
# Workaround make inside make: ovewrite this variable so it is not recomputed in Makefile.core.mk
export IMAGE="${HUB}/${IMAGE_BASE:-sail-operator}:${TAG:-latest}"

run_integration_tests
