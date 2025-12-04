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
ROOT=$(cd "${SCRIPTPATH}/../../.." && pwd)
GW_API_VERSION=${GW_API_VERSION:-v1.4.1}

# shellcheck source=common/scripts/kind_provisioner.sh
source "${ROOT}/common/scripts/kind_provisioner.sh"

export KIND_REGISTRY_NAME="kind-registry"
export KIND_REGISTRY_PORT="5000"
export KIND_REGISTRY="localhost:${KIND_REGISTRY_PORT}"
export DEFAULT_CLUSTER_YAML="${SCRIPTPATH}/config/default.yaml"
export IP_FAMILY="${IP_FAMILY:-ipv4}"

export KIND_IP_FAMILY="${IP_FAMILY}"
export ARTIFACTS="${ARTIFACTS:-$(mktemp -d)}"
export MULTICLUSTER="${MULTICLUSTER:-false}"

# Allow overriding DEFAULT_KIND_IMAGE from environment by passing as an argument in the setup_kind.sh script.
export KIND_IMAGE="${KIND_IMAGE:-}"

export KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-operator-integration-tests}"
if [ "${MULTICLUSTER}" == "true" ]; then
  export KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME}-1"
  export KIND_CLUSTER_NAME_2="${KIND_CLUSTER_NAME}-2"
fi

function setup_kind_registry() {
  running="$(docker inspect -f '{{.State.Running}}' "${KIND_REGISTRY_NAME}" 2>/dev/null || true)"
  if [[ "${running}" != 'true' ]]; then
      docker run \
        -d --restart=always -p "${KIND_REGISTRY_PORT}:5000" --name "${KIND_REGISTRY_NAME}" \
        gcr.io/istio-testing/registry:2
    docker network connect "kind" "${KIND_REGISTRY_NAME}"
  fi

  for cluster in "$@"; do
    if [ "${MULTICLUSTER}" == "true" ]; then
        export KUBECONFIG="${KUBECONFIG_DIR}/${cluster}"
    else
        kind export kubeconfig --name="${cluster}"
    fi

    for node in $(kind get nodes --name="${cluster}"); do
      kubectl annotate node "${node}" "kind.x-k8s.io/registry=localhost:${KIND_REGISTRY_PORT}" --overwrite;
    done
    unset KUBECONFIG
  done
}

if [ "${MULTICLUSTER}" == "true" ]; then
    CLUSTER_TOPOLOGY_CONFIG_FILE="${SCRIPTPATH}/../setup/config/multicluster.json"
    load_cluster_topology "${CLUSTER_TOPOLOGY_CONFIG_FILE}"
    setup_kind_clusters "${KIND_IMAGE}" ""
    setup_kind_registry "${CLUSTER_NAMES[@]}"

    # Apply Gateway API CRDs which are needed for Multi-Cluster Ambient testing
    for config in "${KUBECONFIGS[@]}"; do
        kubectl apply --kubeconfig="$config" --server-side -f "https://github.com/kubernetes-sigs/gateway-api/releases/download/${GW_API_VERSION}/experimental-install.yaml"
    done

    export KUBECONFIG="${KUBECONFIGS[0]}"
    export KUBECONFIG2="${KUBECONFIGS[1]}"
    echo "Your KinD environment is ready, to use it: export KUBECONFIG=$(IFS=:; echo "${KUBECONFIGS[*]}")"
else
  KUBECONFIG="${ARTIFACTS}/config" setup_kind_cluster "${KIND_CLUSTER_NAME}" "${KIND_IMAGE}" "" "true" "true"
  setup_kind_registry "$KIND_CLUSTER_NAME"
  echo "Your KinD environment is ready, to use it: export KUBECONFIG=${ARTIFACTS}/config"
fi
