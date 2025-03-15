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
ROOT=$(dirname "$(dirname "${SCRIPTPATH}")")

# shellcheck source=common/scripts/kind_provisioner.sh
source "${ROOT}/common/scripts/kind_provisioner.sh"

export IP_FAMILY="${IP_FAMILY:-ipv4}"
export ARTIFACTS="${ARTIFACTS:-$(mktemp -d)}"
export MULTICLUSTER="${MULTICLUSTER:-false}"
export ISTIOCTL="${ISTIOCTL:-${ROOT}/bin/istioctl}"
export KUBECONFIG_DIR="${ARTIFACTS}/kubeconfig"

# Set variable for cluster kind name
export KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-operator-integration-tests}"
if [ "${MULTICLUSTER}" == "true" ]; then
  export KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME}-1"
  export KIND_CLUSTER_NAME_2="${KIND_CLUSTER_NAME}-2"
fi

# Use the local registry instead of the default HUB
export KIND_REGISTRY_PORT="5000"
export KIND_REGISTRY="localhost:${KIND_REGISTRY_PORT}"
export HUB="${KIND_REGISTRY}"
# Workaround make inside make: ovewrite this variable so it is not recomputed in Makefile.core.mk
export IMAGE="${HUB}/${IMAGE_BASE:-sail-operator}:${TAG:-latest}"

# Check that istioctl is present using ${ISTIOCTL}
if ! command -v "${ISTIOCTL}" &> /dev/null; then
  echo "istioctl not found. Please set the ISTIOCTL environment variable to the path of the istioctl binary"
  exit 1
fi


# Run the integration tests
echo "Running integration tests"
if [ "${MULTICLUSTER}" == "true" ]; then
  CLUSTER_TOPOLOGY_CONFIG_FILE="${SCRIPTPATH}/config/multicluster.json"
  load_cluster_topology "${CLUSTER_TOPOLOGY_CONFIG_FILE}"
  export KUBECONFIG="${KUBECONFIG_DIR}/primary"
  export KUBECONFIG2="${KUBECONFIG_DIR}/remote"
  trap cleanup_kind_clusters EXIT
  ARTIFACTS="${ARTIFACTS}" ISTIOCTL="${ISTIOCTL}" ./tests/e2e/common-operator-integ-suite.sh --kind --multicluster
else
  trap "cleanup_kind_cluster ${KIND_CLUSTER_NAME}" EXIT
  ARTIFACTS="${ARTIFACTS}" IP_FAMILY="${IP_FAMILY}" ./tests/e2e/common-operator-integ-suite.sh --kind
fi
