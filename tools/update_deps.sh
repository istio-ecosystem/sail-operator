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

set -exo pipefail

# Set up a cross-platform sed command.
# On macOS, we use gsed (GNU sed) to have consistent behavior with Linux.
# This requires gsed to be installed on macOS (e.g., via `brew install gnu-sed`).
SED_CMD="sed"
if [[ "$(uname)" == "Darwin" ]]; then
  SED_CMD="gsed"
fi

UPDATE_BRANCH=${UPDATE_BRANCH:-"master"}
# When true, only update to the latest patch version (keeps major.minor version the same)
PIN_MINOR=${PIN_MINOR:-false}
# When true, skip Istio module updates (istio.io/istio and istio.io/client-go), do not add new Istio versions and only update tools
TOOLS_ONLY=${TOOLS_ONLY:-false}

SCRIPTPATH="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOTDIR=$(dirname "${SCRIPTPATH}")
cd "${ROOTDIR}"

# Extract tool versions from Makefile
function getVersionFromMakefile() {
  grep "^${1} ?= " "${ROOTDIR}/Makefile.core.mk" | cut -d'=' -f2 | tr -d ' '
}

# Get current versions from Makefile and set as variables
# Only needed when PIN_MINOR is true (for patch version updates)
if [[ "${PIN_MINOR}" == "true" ]]; then
  OPERATOR_SDK_VERSION=$(getVersionFromMakefile "OPERATOR_SDK_VERSION")
  # shellcheck disable=SC2034
  HELM_VERSION=$(getVersionFromMakefile "HELM_VERSION")
  CONTROLLER_TOOLS_VERSION=$(getVersionFromMakefile "CONTROLLER_TOOLS_VERSION")
  CONTROLLER_RUNTIME_BRANCH=$(getVersionFromMakefile "CONTROLLER_RUNTIME_BRANCH")
  OPM_VERSION=$(getVersionFromMakefile "OPM_VERSION")
  OLM_VERSION=$(getVersionFromMakefile "OLM_VERSION")
  GITLEAKS_VERSION=$(getVersionFromMakefile "GITLEAKS_VERSION")
  RUNME_VERSION=$(getVersionFromMakefile "RUNME_VERSION")
  MISSPELL_VERSION=$(getVersionFromMakefile "MISSPELL_VERSION")
fi


# getLatestVersion gets the latest released version of a github project
# $1 = org/repo
function getLatestVersion() {
  curl -sL "https://api.github.com/repos/${1}/releases/latest" | yq '.tag_name'
}

# getLatestVersionByPrefix gets the latest released version of a github project with a specific version prefix
# $1 = org/repo
# $2 = version prefix
function getLatestVersionByPrefix() {
  curl -sL "https://api.github.com/repos/${1}/releases?per_page=100" | \
    yq -r '.[].tag_name' | \
    grep -E "^v?${2}[.0-9]*$" | \
    sort -V | \
    tail -n 1
}

# getLatestPatchVersion gets the latest patch version for a given major.minor version
# $1 = org/repo
# $2 = current version (e.g., v1.2.3)
function getLatestPatchVersion() {
  local repo=$1
  local current_version=$2

  # Extract major.minor from current version
  # Handle versions with or without 'v' prefix
  local version_no_v=${current_version#v}
  local major_minor=""
  major_minor=$(echo "${version_no_v}" | cut -d'.' -f1,2)

  getLatestVersionByPrefix "$repo" "${major_minor}"
}

# getVersionForUpdate chooses between getLatestVersion and getLatestPatchVersion based on PIN_MINOR
# $1 = org/repo
# $2 = current version (optional, required if PIN_MINOR=true)
function getVersionForUpdate() {
  local repo=$1
  local current_version=$2

  if [[ "${PIN_MINOR}" == "true" ]]; then
    getLatestPatchVersion "${repo}" "${current_version}"
  else
    getLatestVersion "${repo}"
  fi
}

function getReleaseBranch() {
  minor=$(echo "${1}" | cut -f1,2 -d'.')
  echo "release-${minor#*v}"
}

function getLatestVersionFromDockerHub() {
  # $1 = org/repo
  curl -sL "https://hub.docker.com/v2/repositories/${1}/tags/?page_size=100" | jq -r '.results[].name' | sort -V | tail -n 1
}

# Update common files
make update-common

# update build container used in github actions
NEW_IMAGE_MASTER=$(grep IMAGE_VERSION= < common/scripts/setup_env.sh | cut -d= -f2)
"$SED_CMD" -i -e "s|\(gcr.io/istio-testing/build-tools\):master.*|\1:$NEW_IMAGE_MASTER|" .github/workflows/update-deps.yaml

# Update go dependencies
export GO111MODULE=on
if [[ "${TOOLS_ONLY}" != "true" ]]; then
  go get -u "istio.io/istio@${UPDATE_BRANCH}"
  go get -u "istio.io/client-go@${UPDATE_BRANCH}"
  go mod tidy
else
  echo "Skipping Istio module updates (TOOLS_ONLY=true)"
fi

# Update operator-sdk
OPERATOR_SDK_LATEST_VERSION=$(getVersionForUpdate operator-framework/operator-sdk "${OPERATOR_SDK_VERSION}")
"$SED_CMD" -i "s|OPERATOR_SDK_VERSION ?= .*|OPERATOR_SDK_VERSION ?= ${OPERATOR_SDK_LATEST_VERSION}|" "${ROOTDIR}/Makefile.core.mk"
find "${ROOTDIR}/chart/templates/olm/scorecard.yaml" -type f -exec "$SED_CMD" -i "s|quay.io/operator-framework/scorecard-test:.*|quay.io/operator-framework/scorecard-test:${OPERATOR_SDK_LATEST_VERSION}|" {} +

# Update helm (FIXME: pinned to v3 as we don't support helm4 yet, see https://github.com/istio-ecosystem/sail-operator/issues/1371)
HELM_LATEST_VERSION=$(getLatestVersionByPrefix helm/helm v3)
"$SED_CMD" -i "s|HELM_VERSION ?= .*|HELM_VERSION ?= ${HELM_LATEST_VERSION}|" "${ROOTDIR}/Makefile.core.mk"

# Update controller-tools
CONTROLLER_TOOLS_LATEST_VERSION=$(getVersionForUpdate kubernetes-sigs/controller-tools "${CONTROLLER_TOOLS_VERSION}")
"$SED_CMD" -i "s|CONTROLLER_TOOLS_VERSION ?= .*|CONTROLLER_TOOLS_VERSION ?= ${CONTROLLER_TOOLS_LATEST_VERSION}|" "${ROOTDIR}/Makefile.core.mk"

# Update controller-runtime
# Note: For controller-runtime, we use the branch to determine the current version
CONTROLLER_RUNTIME_CURRENT_VERSION="v${CONTROLLER_RUNTIME_BRANCH#release-}.0"
CONTROLLER_RUNTIME_LATEST_VERSION=$(getVersionForUpdate kubernetes-sigs/controller-runtime "${CONTROLLER_RUNTIME_CURRENT_VERSION}")
# FIXME: Do not use `go get -u` until https://github.com/kubernetes/apimachinery/issues/190 is resolved
# go get -u "sigs.k8s.io/controller-runtime@${CONTROLLER_RUNTIME_LATEST_VERSION}"
go get "sigs.k8s.io/controller-runtime@${CONTROLLER_RUNTIME_LATEST_VERSION}"
CONTROLLER_RUNTIME_BRANCH=$(getReleaseBranch "${CONTROLLER_RUNTIME_LATEST_VERSION}")
"$SED_CMD" -i "s|CONTROLLER_RUNTIME_BRANCH ?= .*|CONTROLLER_RUNTIME_BRANCH ?= ${CONTROLLER_RUNTIME_BRANCH}|" "${ROOTDIR}/Makefile.core.mk"

# Update opm
OPM_LATEST_VERSION=$(getVersionForUpdate operator-framework/operator-registry "${OPM_VERSION}")
"$SED_CMD" -i "s|OPM_VERSION ?= .*|OPM_VERSION ?= ${OPM_LATEST_VERSION}|" "${ROOTDIR}/Makefile.core.mk"

# Update olm
OLM_LATEST_VERSION=$(getVersionForUpdate operator-framework/operator-lifecycle-manager "${OLM_VERSION}")
"$SED_CMD" -i "s|OLM_VERSION ?= .*|OLM_VERSION ?= ${OLM_LATEST_VERSION}|" "${ROOTDIR}/Makefile.core.mk"

# Update gateway-api
GW_API_LATEST_VERSION=$(getLatestVersion kubernetes-sigs/gateway-api)
"$SED_CMD" -i "s|GW_API_VERSION=.*|GW_API_VERSION=\${GW_API_VERSION:-${GW_API_LATEST_VERSION}}|" "${ROOTDIR}/tests/e2e/setup/setup-kind.sh"

# Update gitleaks
GITLEAKS_LATEST_VERSION=$(getVersionForUpdate gitleaks/gitleaks "${GITLEAKS_VERSION}")
"$SED_CMD" -i "s|GITLEAKS_VERSION ?= .*|GITLEAKS_VERSION ?= ${GITLEAKS_LATEST_VERSION}|" "${ROOTDIR}/Makefile.core.mk"

# Update runme
# Add 'v' prefix to current version for comparison if it doesn't have one
RUNME_VERSION_WITH_V="v${RUNME_VERSION}"
RUNME_LATEST_VERSION=$(getVersionForUpdate runmedev/runme "${RUNME_VERSION_WITH_V}")
# Remove the leading "v" from the version string for storage in Makefile
RUNME_LATEST_VERSION=${RUNME_LATEST_VERSION#v}
"$SED_CMD" -i "s|RUNME_VERSION ?= .*|RUNME_VERSION ?= ${RUNME_LATEST_VERSION}|" "${ROOTDIR}/Makefile.core.mk"

# Update misspell
MISSPELL_LATEST_VERSION=$(getVersionForUpdate client9/misspell "${MISSPELL_VERSION}")
"$SED_CMD" -i "s|MISSPELL_VERSION ?= .*|MISSPELL_VERSION ?= ${MISSPELL_LATEST_VERSION}|" "${ROOTDIR}/Makefile.core.mk"

# Update KIND_IMAGE. Look for KIND_IMAGE := docker.io in the make file and look on docker.io/kindest/node for the latest version.
KIND_LATEST_VERSION=$(getLatestVersionFromDockerHub kindest/node)
if [[ -n "${KIND_LATEST_VERSION}" ]]; then
  "$SED_CMD" -i "s|KIND_IMAGE := docker.io/kindest/node:.*|KIND_IMAGE := docker.io/kindest/node:${KIND_LATEST_VERSION}|" "${ROOTDIR}/Makefile.core.mk"
else
  echo "No latest version found for kindest/node on Docker Hub. Keeping the existing KIND_IMAGE."
fi

# Update Istio versions in the Documentation files
./hack/update-istio-in-docs.sh

# Regenerate files
if [[ "${TOOLS_ONLY}" != "true" ]]; then
  make update-istio gen
else
  echo "Skipping 'make update-istio' (TOOLS_ONLY=true), running 'make gen' only"
  make gen
fi
