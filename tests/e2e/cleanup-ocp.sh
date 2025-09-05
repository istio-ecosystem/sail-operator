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

# Clean up leftover artifacts from e2e.ocp tests
echo "Cleaning up leftover artifacts from e2e.ocp tests"

# Check if KUBECONFIG is set
if [ -z "${KUBECONFIG}" ]; then
    echo "KUBECONFIG is not set. kubectl will not be able to connect to the cluster. Exiting."
    exit 1
fi

# Clean up cluster-level resources that may be left over from e2e tests
echo "Removing cluster role bindings..."
kubectl delete clusterrolebinding metrics-reader-rolebinding --ignore-not-found

echo "Removing cluster roles..."
kubectl delete clusterrole metrics-reader --ignore-not-found

echo "Removing Istio CRDs..."
kubectl get crds -o name | grep ".*\.istio" | xargs -r -n 1 kubectl delete || true

echo "Removing Sail Operator CRDs..."
kubectl get crds -o name | grep ".*\.sail" | xargs -r -n 1 kubectl delete || true

echo "Cleanup completed successfully"