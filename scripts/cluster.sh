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
ROOT="$(dirname "${SCRIPTPATH}")"

# shellcheck source=common/scripts/kind_provisioner.sh
source "${ROOT}/common/scripts/kind_provisioner.sh"

export ARTIFACTS="${ARTIFACTS:-$(mktemp -d)}"
export DEFAULT_CLUSTER_YAML="${SCRIPTPATH}/config/default.yaml"
export IP_FAMILY="${IP_FAMILY:-ipv4}"
export MULTICLUSTER="${MULTICLUSTER:-false}"
export SKIP_CLEANUP="true"

# We run a local-registry in a docker container that KinD nodes pull from
export KIND_REGISTRY_NAME="kind-registry"
export KIND_REGISTRY_PORT="5000"

# See https://github.com/istio-ecosystem/sail-operator/issues/558
export KIND_IP_FAMILY="${IP_FAMILY}"

# Set variable to exclude kind clusters from kubectl annotations.
# You need to set kind clusters names separated by comma
export KIND_EXCLUDE_CLUSTERS="${KIND_EXCLUDE_CLUSTERS:-}"

# Set variable for cluster kind name
export KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-sail-operator}"
if [ "${MULTICLUSTER}" == "true" ]; then
  export KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME}-1"
  export KIND_CLUSTER_NAME_2="${KIND_CLUSTER_NAME}-2"
fi

# Copied from Istio: https://github.com/istio/istio/blob/861abfbc050c5be41154054853fe70336a851ce9/prow/lib.sh#L149
function setup_kind_registry() {
  # create a registry container if it not running already
  running="$(docker inspect -f '{{.State.Running}}' "${KIND_REGISTRY_NAME}" 2>/dev/null || true)"
  if [[ "${running}" != 'true' ]]; then
      docker run \
        -d --restart=always -p "${KIND_REGISTRY_PORT}:5000" --name "${KIND_REGISTRY_NAME}" \
        gcr.io/istio-testing/registry:2

    # Allow kind nodes to reach the registry
    docker network connect "kind" "${KIND_REGISTRY_NAME}"
  fi

  # https://docs.tilt.dev/choosing_clusters.html#discovering-the-registry
  for cluster in $(kind get clusters); do
    # TODO get context/config from existing variables
    # Avoid adding the registry to excluded clusters. Use when you have multiple clusters running.
    if [[ "${KIND_EXCLUDE_CLUSTERS}" == *"${cluster}"* ]]; then
      continue
    fi

    kind export kubeconfig --name="${cluster}"
    for node in $(kind get nodes --name="${cluster}"); do
      kubectl annotate node "${node}" "kind.x-k8s.io/registry=localhost:${KIND_REGISTRY_PORT}" --overwrite;
    done
  done
}

if [ "${MULTICLUSTER}" == "true" ]; then
  CLUSTER_TOPOLOGY_CONFIG_FILE="${SCRIPTPATH}/config/multicluster.json"
  load_cluster_topology "${CLUSTER_TOPOLOGY_CONFIG_FILE}"
  setup_kind_clusters "" ""
else
  KUBECONFIG="${ARTIFACTS}/config" setup_kind_cluster "${KIND_CLUSTER_NAME}" "" "" "true" "false"
fi

setup_kind_registry
