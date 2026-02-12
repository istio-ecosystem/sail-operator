#!/bin/bash

# Copyright Istio Authors

# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -eux -o pipefail

SCRIPTPATH="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOT="$(dirname "${SCRIPTPATH}")"

# OCP environment variable can be set to "true" to run tests against an existing OCP cluster instead of a Kind cluster
OCP="${OCP:-false}"

if [[ "${OCP}" == "true" ]]; then
    echo "Running scorecard tests against existing OCP cluster"

    # Check if KUBECONFIG is set
    if [ -z "${KUBECONFIG:-}" ]; then
        echo "KUBECONFIG is not set. oc will not be able to connect to the cluster. Exiting."
        exit 1
    fi

    # Verify we can connect to the cluster
    if ! oc cluster-info > /dev/null 2>&1; then
        echo "Cannot connect to OpenShift cluster. Check your KUBECONFIG and cluster access."
        exit 1
    fi

    echo "Connected to cluster: $(oc config current-context)"

else
    echo "Running scorecard tests against Kind cluster"

    # shellcheck source=common/scripts/kind_provisioner.sh
    source "${ROOT}/common/scripts/kind_provisioner.sh"

    # Create a temporary kubeconfig
    KUBECONFIG="$(mktemp)"
    export KUBECONFIG

    # Create the kind cluster
    export KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-kind}"
    export DEFAULT_CLUSTER_YAML="${ROOT}/tests/e2e/setup/config/default.yaml"
    export ARTIFACTS="${ARTIFACTS:-$(mktemp -d)}"
    export IP_FAMILY="${IP_FAMILY:-ipv4}"
    setup_kind_cluster "${KIND_CLUSTER_NAME}" "" "" "true" "true"

    kind export kubeconfig --name="${KIND_CLUSTER_NAME}"
fi

# Determine namespace - use scorecard-test for OCP to avoid conflicts with any existing namespaces, and default for Kind since it's a fresh cluster
NAMESPACE="${SCORECARD_NAMESPACE:-default}"
if [[ "${OCP}" == "true" ]]; then
    NAMESPACE="${SCORECARD_NAMESPACE:-scorecard-test}"
fi
# Create namespace if it doesn't exist
oc create namespace "${NAMESPACE}" || true

# Run the test
OPERATOR_SDK="${OPERATOR_SDK:-operator-sdk}"
echo "Running scorecard tests in namespace: ${NAMESPACE}"
${OPERATOR_SDK} scorecard --kubeconfig="${KUBECONFIG}" --namespace="${NAMESPACE}" bundle
