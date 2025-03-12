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
SKIP_TESTS="${SKIP_TESTS:-false}"
# To run this integration test on OCP cluster it's needed to already have the OCP cluster running and be logged in

# Run the integration tests
echo "Running integration tests"

# Check if KUBECONFIG is set
if [ -z "${KUBECONFIG}" ]; then
    echo "KUBECONFIG is not set. KUBECTL will not be able to connect to the cluster. Exiting."
    exit 1
fi

KUBECONFIG="${KUBECONFIG}" SKIP_TESTS="${SKIP_TESTS}"  ./tests/e2e/common-operator-integ-suite.sh --ocp