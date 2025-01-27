#!/bin/bash

# Copyright Istio Authors
#
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

# This script sets up a primary-primary multi-network Istio mesh locally with kind using the Sail operator.
# It is adapted from: https://istio.io/latest/docs/setup/install/multicluster/multi-primary_multi-network/.

while [ $# -gt 0 ]; do
  key="$1"
  case $key in
    -h|--help)
      echo "Usage: setup-multi-primary.sh <istio_version>"
      exit 0
      ;;
  esac
  shift
done

set -euo pipefail

# Check for istioctl binary in path.
if ! command -v istioctl &> /dev/null; then
    echo "istioctl not found in path. Please install istioctl."
    exit 1
fi

OUTPUT_DIR=/tmp/sail-multicluster
[ -d "${OUTPUT_DIR}" ] || mkdir "${OUTPUT_DIR}"

CERTS_DIR="${OUTPUT_DIR}/certs"
ISTIO_VERSION=${1:-"master"}

# Check if istio verison is master then this should be latest otherwise it should jsut be istio version
if [ "${ISTIO_VERSION}" == "master" ]; then
    SAIL_VERSION="latest"
else
    SAIL_VERSION="v${ISTIO_VERSION}"
fi

CTX_CLUSTER1=kind-east
CTX_CLUSTER2=kind-west

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
if [ ! -d "${CERTS_DIR}" ]; then
    mkdir -p "${CERTS_DIR}"
    pushd "${CERTS_DIR}"
    curl -fsL -o common.mk "https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/tools/certs/common.mk"
    curl -fsL -o Makefile.selfsigned.mk "https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/tools/certs/Makefile.selfsigned.mk"
    make -f Makefile.selfsigned.mk root-ca
    make -f Makefile.selfsigned.mk east-cacerts
    make -f Makefile.selfsigned.mk west-cacerts
    popd
fi

# 1. Create kind clusters

kind get clusters | grep east || kind create cluster --name east
kind get clusters | grep west || kind create cluster --name west

# 2. Install sail operator on each cluster:

kubectl config use-context "${CTX_CLUSTER1}"
kubectl get ns sail-operator --context "${CTX_CLUSTER1}" || make -C "${SCRIPT_DIR}/../.." deploy
kubectl config use-context "${CTX_CLUSTER2}"
kubectl get ns sail-operator --context "${CTX_CLUSTER2}" || make -C "${SCRIPT_DIR}/../.." deploy

# 3. Create istio-system namespace on each cluster and configure a common root CA. 

kubectl get ns istio-system --context "${CTX_CLUSTER1}" || kubectl create namespace istio-system --context "${CTX_CLUSTER1}"
kubectl --context "${CTX_CLUSTER1}" label namespace istio-system topology.istio.io/network=network1
kubectl get secret -n istio-system --context "${CTX_CLUSTER1}" cacerts || kubectl create secret generic cacerts -n istio-system --context "${CTX_CLUSTER1}" \
  --from-file=${CERTS_DIR}/east/ca-cert.pem \
  --from-file=${CERTS_DIR}/east/ca-key.pem \
  --from-file=${CERTS_DIR}/east/root-cert.pem \
  --from-file=${CERTS_DIR}/east/cert-chain.pem

kubectl get ns istio-system --context "${CTX_CLUSTER2}" || kubectl create namespace istio-system --context "${CTX_CLUSTER2}"
kubectl --context "${CTX_CLUSTER2}" label namespace istio-system topology.istio.io/network=network2
kubectl get secret -n istio-system --context "${CTX_CLUSTER2}" cacerts || kubectl create secret generic cacerts -n istio-system --context "${CTX_CLUSTER2}" \
  --from-file=${CERTS_DIR}/west/ca-cert.pem \
  --from-file=${CERTS_DIR}/west/ca-key.pem \
  --from-file=${CERTS_DIR}/west/root-cert.pem \
  --from-file=${CERTS_DIR}/west/cert-chain.pem

# 4. Create Sail CR on east

kubectl apply --context "${CTX_CLUSTER1}" -f - <<EOF
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  version: ${SAIL_VERSION}
  namespace: istio-system
  values:
    pilot:
      resources:
        requests:
          cpu: 100m
          memory: 1024Mi
    global:
      meshID: mesh1
      multiCluster:
        clusterName: east
      network: network1
EOF

kubectl wait --context "${CTX_CLUSTER1}" --for=condition=Available deployment/sail-operator -n sail-operator --timeout=3m
kubectl wait --context "${CTX_CLUSTER1}" --for=jsonpath='{.status.revisions.ready}'=1 istios/default --timeout=3m

# 5. Create east-west gateway on east
kubectl apply --context "${CTX_CLUSTER1}" -f "${SCRIPT_DIR}/east-west-gateway-net1.yaml"

# 6. Expose services on east
kubectl --context "${CTX_CLUSTER1}" apply -n istio-system -f "${SCRIPT_DIR}/expose-services.yaml"

# 7. Create Sail CR on west

kubectl apply --context "${CTX_CLUSTER2}" -f - <<EOF
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  version: ${SAIL_VERSION}
  namespace: istio-system
  values:
    pilot:
      resources:
        requests:
          cpu: 100m
          memory: 1024Mi
    global:
      meshID: mesh1
      multiCluster:
        clusterName: west
      network: network2
EOF

kubectl wait --context "${CTX_CLUSTER2}" --for=condition=Available deployment/sail-operator -n sail-operator --timeout=3m
kubectl wait --context "${CTX_CLUSTER2}" --for=jsonpath='{.status.revisions.ready}'=1 istios/default --timeout=3m

# 8. Create east-west gateway on west

kubectl apply --context "${CTX_CLUSTER2}" -f "${SCRIPT_DIR}/east-west-gateway-net2.yaml"

# 9. Expose services on west

kubectl --context "${CTX_CLUSTER2}" apply -n istio-system -f "${SCRIPT_DIR}/expose-services.yaml"

# 10. Install a remote secret in west that provides access to east’s API server.

WEST_CONTAINER_IP=$(kubectl get nodes east-control-plane --context "${CTX_CLUSTER1}" -o jsonpath='{.status.addresses[?(@.type == "InternalIP")].address}')
istioctl create-remote-secret \
  --context="${CTX_CLUSTER1}" \
  --name=east \
  --server="https://${WEST_CONTAINER_IP}:6443" | \
  kubectl apply -f - --context "${CTX_CLUSTER2}"

# 11. Install a remote secret in east that provides access to west’s API server.

EAST_CONTAINER_IP=$(kubectl get nodes west-control-plane --context "${CTX_CLUSTER2}" -o jsonpath='{.status.addresses[?(@.type == "InternalIP")].address}')
istioctl create-remote-secret \
  --context="${CTX_CLUSTER2}" \
  --name=west \
  --server="https://${EAST_CONTAINER_IP}:6443" | \
  kubectl apply -f - --context "${CTX_CLUSTER1}"

# 12. Deploy sample applications and verify that you get a response from both v1 and v2 of the helloworld service.

kubectl get ns sample --context "${CTX_CLUSTER1}" || kubectl create --context="${CTX_CLUSTER1}" namespace sample
kubectl get ns sample --context "${CTX_CLUSTER2}" || kubectl create --context="${CTX_CLUSTER2}" namespace sample
kubectl label --context="${CTX_CLUSTER1}" namespace sample istio-injection=enabled
kubectl label --context="${CTX_CLUSTER2}" namespace sample istio-injection=enabled
kubectl apply --context="${CTX_CLUSTER1}" \
  -f "https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml" \
  -l service=helloworld -n sample
kubectl apply --context="${CTX_CLUSTER2}" \
  -f "https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml" \
  -l service=helloworld -n sample
kubectl apply --context="${CTX_CLUSTER1}" \
  -f "https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml" \
  -l version=v1 -n sample
kubectl apply --context="${CTX_CLUSTER2}" \
  -f "https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml" \
  -l version=v2 -n sample
kubectl apply --context="${CTX_CLUSTER1}" \
  -f "https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/sleep/sleep.yaml" -n sample
kubectl apply --context="${CTX_CLUSTER2}" \
  -f "https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/sleep/sleep.yaml" -n sample

kubectl rollout status --context="${CTX_CLUSTER1}" deployment -n sample
kubectl rollout status --context="${CTX_CLUSTER2}" deployment -n sample

cat <<EOF

Success! Verify your deployment by running these commands separately until you see both a v1 and v2 response from each:

kubectl exec --context="${CTX_CLUSTER1}" -n sample -c sleep "$(kubectl get pod --context="${CTX_CLUSTER1}" -n sample -l app=sleep -o jsonpath='{.items[0].metadata.name}')" -- curl -sS helloworld.sample:5000/hello
kubectl exec --context="${CTX_CLUSTER2}" -n sample -c sleep "$(kubectl get pod --context="${CTX_CLUSTER2}" -n sample -l app=sleep -o jsonpath='{.items[0].metadata.name}')" -- curl -sS helloworld.sample:5000/hello
EOF
