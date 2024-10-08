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

set -e -u -o pipefail

: "${ISTIO_VERSION:=$1}"
: "${ISTIO_REPO:=$2}"
: "${ISTIO_COMMIT:=$3}"
CHART_URLS=("${@:4}")

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
REPO_ROOT=$(dirname "${SCRIPT_DIR}")
MANIFEST_DIR="${REPO_ROOT}/resources/${ISTIO_VERSION}"
CHARTS_DIR="${MANIFEST_DIR}/charts"
PROFILES_DIR="${MANIFEST_DIR}/profiles"

ISTIO_URL="${ISTIO_REPO}/archive/${ISTIO_COMMIT}.tar.gz"
WORK_DIR=$(mktemp -d)
trap 'rm -rf "${WORK_DIR}"' EXIT

function downloadIstioManifests() {
  rm -rf "${CHARTS_DIR}"
  mkdir -p "${CHARTS_DIR}"

  rm -rf "${PROFILES_DIR}"
  mkdir -p "${PROFILES_DIR}"

  pushd "${WORK_DIR}" >/dev/null
  echo "downloading Git archive from ${ISTIO_URL}"
  curl -sSLfO "${ISTIO_URL}"

  ISTIO_FILE="${ISTIO_URL##*/}"
  EXTRACT_DIR="${ISTIO_REPO##*/}-${ISTIO_COMMIT}"

  if [ "${#CHART_URLS[@]}" -gt 0 ]; then
    for url in "${CHART_URLS[@]}"; do
      echo "downloading chart from $url"
      curl -sSLfO "$url"

      file="${url##*/}"

      echo "extracting charts from $file to ${CHARTS_DIR}"
      tar zxf "$file" -C "${CHARTS_DIR}"
    done

    echo "extracting profiles from ${ISTIO_FILE} to ${PROFILES_DIR}"
    tar zxf "${ISTIO_FILE}" "${EXTRACT_DIR}/manifests/profiles"
    echo "copying profiles to ${PROFILES_DIR}"
    cp -rf "${WORK_DIR}"/"${EXTRACT_DIR}"/manifests/profiles/* "${PROFILES_DIR}/"

  else
    echo "extracting charts and profiles from ${ISTIO_FILE} to ${WORK_DIR}/${EXTRACT_DIR}"
    tar zxf "${ISTIO_FILE}" "${EXTRACT_DIR}/manifests/charts" "${EXTRACT_DIR}/manifests/profiles"

    echo "copying charts to ${CHARTS_DIR}"
    cp -rf "${WORK_DIR}"/"${EXTRACT_DIR}"/manifests/charts/base "${CHARTS_DIR}/base"
    cp -rf "${WORK_DIR}"/"${EXTRACT_DIR}"/manifests/charts/gateway "${CHARTS_DIR}/gateway"
    cp -rf "${WORK_DIR}"/"${EXTRACT_DIR}"/manifests/charts/istio-cni "${CHARTS_DIR}/cni"
    cp -rf "${WORK_DIR}"/"${EXTRACT_DIR}"/manifests/charts/istio-control/istio-discovery "${CHARTS_DIR}/istiod"
    cp -rf "${WORK_DIR}"/"${EXTRACT_DIR}"/manifests/charts/istiod-remote "${CHARTS_DIR}/istiod-remote"
    cp -rf "${WORK_DIR}"/"${EXTRACT_DIR}"/manifests/charts/ztunnel "${CHARTS_DIR}/ztunnel"

    echo "copying profiles to ${PROFILES_DIR}"
    cp -rf "${WORK_DIR}"/"${EXTRACT_DIR}"/manifests/profiles/* "${PROFILES_DIR}/"
  fi

  echo
  popd >/dev/null
}

function patchIstioCharts() {
  echo "patching istio charts ${CHARTS_DIR}/cni/templates/clusterrole.yaml "
  # NOTE: everything in the patchIstioCharts should be here only temporarily,
  # until we push the required changes upstream
  sed -i '0,/rules:/s//rules:\
- apiGroups: ["security.openshift.io"] \
  resources: ["securitycontextconstraints"] \
  resourceNames: ["privileged"] \
  verbs: ["use"]/' "${CHARTS_DIR}/cni/templates/clusterrole.yaml"

  # remove CRDs from istiod-remote chart, since they are installed by OLM, not by the operator
  rm -f "${CHARTS_DIR}/istiod-remote/templates/crd-all.gen.yaml"
}

function convertIstioProfiles() {
  # delete the remote profile, because it isn't needed (we have the RemoteIstio resource instead)
  [ -f "${PROFILES_DIR}"/remote.yaml ] && rm "${PROFILES_DIR}"/remote.yaml

  # delete the minimal profile, because it ends up being empty after the conversion
  [ -f "${PROFILES_DIR}"/minimal.yaml ] && rm "${PROFILES_DIR}"/minimal.yaml

  # convert profiles
  for profile in "${PROFILES_DIR}"/*.yaml; do
    yq eval -i '.apiVersion="sailoperator.io/v1alpha1"
      | .kind="Istio"
      | (select(.spec.meshConfig) | .spec.values.meshConfig)=.spec.meshConfig
      | (select(.spec.values.istio_cni) | .spec.values.pilot.cni)=.spec.values.istio_cni
      | del(.spec.values.istio_cni)
      | del(.metadata)
      | del(.spec.components)
      | del(.spec.meshConfig)
      | del(.spec.hub)
      | del(.spec.tag)
      | del(.spec.values.gateways)' "$profile"
  done
}

downloadIstioManifests
patchIstioCharts
convertIstioProfiles