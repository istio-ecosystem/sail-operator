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

: "${ISTIO_VERSION_NAME:=$1}"
: "${ISTIO_VERSION:=$2}"
: "${ISTIO_REPO:=$3}"
: "${ISTIO_COMMIT:=$4}"
CHART_URLS=("${@:5}")

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
REPO_ROOT=$(dirname "${SCRIPT_DIR}")
MANIFEST_DIR="${REPO_ROOT}/resources/${ISTIO_VERSION_NAME}"
CHARTS_DIR="${MANIFEST_DIR}/charts"
PROFILES_DIR="${MANIFEST_DIR}/profiles"

ISTIO_URL="${ISTIO_REPO}/archive/${ISTIO_COMMIT}.tar.gz"
WORK_DIR=$(mktemp -d)
FORCE_DOWNLOADS=${FORCE_DOWNLOADS:=false}
trap 'rm -rf "${WORK_DIR}"' EXIT

function downloadRequired() {
  commit_file="${MANIFEST_DIR}/commit"
  if [ ! -f "${commit_file}" ]; then
    return 0
  fi
  if [ "$ISTIO_COMMIT" != "$(cat "${commit_file}")" ]; then
    return 0
  fi

  if [ "${#CHART_URLS[@]}" -gt 0 ]; then
    for url in "${CHART_URLS[@]}"; do
      file="${url##*/}"
      etag_file="${MANIFEST_DIR}/$file.etag"
      if [ ! -f "${etag_file}" ]; then
        return 0
      fi
      current=$(curl -I "$url" 2>/dev/null | awk -F': ' '/^etag:/ {print $2}' | tr -d "\"")
      if [ "$current" != "$(cat "${etag_file}")" ]; then
        return 0
      fi
    done
  fi
  return 1
}


function downloadIstioManifests() {
  rm -rf "${CHARTS_DIR}"
  mkdir -p "${CHARTS_DIR}"

  rm -rf "${PROFILES_DIR}"
  mkdir -p "${PROFILES_DIR}"

  pushd "${WORK_DIR}" >/dev/null
  commit_file="${MANIFEST_DIR}/commit"  
  echo "writing commit for Git archive to ${ISTIO_COMMIT}"
  echo "${ISTIO_COMMIT}" > "${commit_file}"

  echo "downloading Git archive from ${ISTIO_URL}"
  curl -sSfLO "${ISTIO_URL}"

  ISTIO_FILE="${ISTIO_URL##*/}"
  EXTRACT_DIR="${ISTIO_REPO##*/}-${ISTIO_COMMIT}"

  if [ "${#CHART_URLS[@]}" -gt 0 ]; then
    for url in "${CHART_URLS[@]}"; do
      file="${url##*/}"
      etag_file="${MANIFEST_DIR}/$file.etag"
      echo "downloading chart from $url"
      curl -LfO -D - "$url" 2>/dev/null | awk -F': ' '/^etag:/ {print $2}' | tr -d "\"" > "$etag_file"

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

  # add permissions for CNI to use the privileged SCC. This has been added upstream in 1.23.0,
  # so this can be removed once we remove support for versions <1.23
  sed -i '0,/rules:/s//rules:\
- apiGroups: ["security.openshift.io"] \
  resources: ["securitycontextconstraints"] \
  resourceNames: ["privileged"] \
  verbs: ["use"]/' "${CHARTS_DIR}/cni/templates/clusterrole.yaml"

  # remove CRDs from base chart, since they are installed by OLM, not by the operator
  rm -f "${CHARTS_DIR}/base/templates/crds.yaml"
  # <v1.24.0
  rm -rf "${CHARTS_DIR}/base/crds/"
  # >=v1.24.0
  rm -f "${CHARTS_DIR}/base/files/crd-all.gen.yaml"

  # TODO: remove this once we remove support for 1.23
  # remove CRDs from istiod-remote chart, since they are installed by OLM, not by the operator
  rm -f "${CHARTS_DIR}/istiod-remote/templates/crd-all.gen.yaml"

  # remove install.operator.istio.io/owning-resource label from all charts
  sed -i '/install.operator.istio.io\/owning-resource/d' "${CHARTS_DIR}"/*/templates/*.yaml

  # add values ConfigMap template if it doesn't exist
  if [ ! -f "${CHARTS_DIR}/istiod/templates/configmap-values.yaml" ]; then
    cat <<EOF > "${CHARTS_DIR}/istiod/templates/configmap-values.yaml"
apiVersion: v1
kind: ConfigMap
metadata:
  name: values{{- if not (eq .Values.revision "") }}-{{ .Values.revision }}{{- end }}
  namespace: {{ .Release.Namespace }}
  annotations:
    kubernetes.io/description: This ConfigMap contains the Helm values used during chart rendering. This ConfigMap is rendered for debugging purposes and external tooling; modifying these values has no effect.
  labels:
    istio.io/rev: {{ .Values.revision | default "default" | quote }}
    install.operator.istio.io/owning-resource: {{ .Values.ownerName | default "unknown" }}
    operator.istio.io/component: "Pilot"
    release: {{ .Release.Name }}
    app.kubernetes.io/name: "istiod"
    {{- include "istio.labels" . | nindent 4 }}
data:
  original-values: |-
{{ .Values._original | toPrettyJson | indent 4 }}
{{- \$_ := unset $.Values "_original" }}
  merged-values: |-
{{ .Values | toPrettyJson | indent 4 }}
EOF

    # remove "include istio.labels" line if istio.labels is not defined in zzz_profile.yaml
    if ! grep -q 'define "istio.labels"' "${CHARTS_DIR}/istiod/templates/zzz_profile.yaml"; then
      sed -i '/istio.labels/d' "${CHARTS_DIR}/istiod/templates/configmap-values.yaml"
    fi
  fi

  # shellcheck disable=SC2016
  if ! grep -q '$x := set $.Values "_original" (deepCopy $.Values)' "${CHARTS_DIR}/istiod/templates/zzz_profile.yaml"; then
    # shellcheck disable=SC2016
    sed -i '/mustMergeOverwrite \$defaults \$.Values/i {{- $x := set $.Values "_original" (deepCopy $.Values) }}' "${CHARTS_DIR}/istiod/templates/zzz_profile.yaml"
  fi

  if versionIsBelow "1.27.1"; then
    # patch the Gateway chart to comply with the new JSON schema validation introduced upstream
    # This is necessary for all versions <1.27.1
    # TODO: remove once we remove support for 1.27.0
    jq '
      del(.type, .additionalProperties) |
      ."$defs".values.additionalProperties = false |
      ."$defs".values.properties."_internal_defaults_do_not_set" = {"type": "object"}
    ' "${CHARTS_DIR}/gateway/values.schema.json" > temp.json && mv temp.json "${CHARTS_DIR}/gateway/values.schema.json"
  fi
}

# The charts use docker.io as the default registry, but this leads to issues
# because of Docker Hub's rate limiting. This function modifies the hub field
# in all charts to use gcr.io/istio-release instead of docker.io/istio.
# gcr.io also contains the official images for Istio and they ar an exact match.
function replaceDockerHubWithGcrIo() {
  echo "replacing docker.io/istio with gcr.io/istio-release in all charts"

  find "${CHARTS_DIR}" -name values.yaml -exec sed -i 's/hub: docker.io\/istio/hub: gcr.io\/istio-release/g' {} \;
}

function convertIstioProfiles() {
  # delete the minimal profile, because it ends up being empty after the conversion
  [ -f "${PROFILES_DIR}"/minimal.yaml ] && rm "${PROFILES_DIR}"/minimal.yaml
  # convert profiles
  for profile in "${PROFILES_DIR}"/*.yaml; do
    yq eval -i '.apiVersion="sailoperator.io/v1"
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

function createRevisionTagChart() {
  mkdir -p "${CHARTS_DIR}/revisiontags/templates"
  echo "apiVersion: v2
appVersion: ${ISTIO_VERSION}
description: Helm chart for istio revision tags
name: revisiontags
sources:
- https://github.com/istio-ecosystem/sail-operator
version: 0.1.0
" > "${CHARTS_DIR}/revisiontags/Chart.yaml"
  cp "${CHARTS_DIR}/istiod/values.yaml" "${CHARTS_DIR}/revisiontags/values.yaml"
  if [ -e "${CHARTS_DIR}/istiod/templates/revision-tags.yaml" ]; then
    cp "${CHARTS_DIR}/istiod/templates/revision-tags.yaml" "${CHARTS_DIR}/revisiontags/templates/revision-tags.yaml"
  else
    cp "${CHARTS_DIR}/istiod/templates/revision-tags-mwc.yaml" "${CHARTS_DIR}/revisiontags/templates/revision-tags-mwc.yaml"
    cp "${CHARTS_DIR}/istiod/templates/revision-tags-svc.yaml" "${CHARTS_DIR}/revisiontags/templates/revision-tags-svc.yaml"
  fi
  cp "${CHARTS_DIR}/istiod/templates/zzz_profile.yaml" "${CHARTS_DIR}/revisiontags/templates/zzz_profile.yaml"

  mkdir -p "${CHARTS_DIR}/revisiontags/files"
  cp "${CHARTS_DIR}"/istiod/files/profile-*.yaml "${CHARTS_DIR}/revisiontags/files"
}

function versionIsBelow() {
  local comparedVersion="$1"
  [[ "$(printf '%s\n' "$ISTIO_VERSION" "${comparedVersion}" | sort -V | head -n1)" == "$ISTIO_VERSION" ]] && [[ "$ISTIO_VERSION" != "${comparedVersion}" ]]
}

if ! downloadRequired && [ "${FORCE_DOWNLOADS}" != "true" ] ; then
  echo "${ISTIO_VERSION_NAME} charts are up to date. Skipping downloads"
  exit 0
fi
downloadIstioManifests
patchIstioCharts
replaceDockerHubWithGcrIo
convertIstioProfiles
createRevisionTagChart
