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

SCRIPTPATH="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOTDIR=$(dirname "${SCRIPTPATH}")
cd "${ROOTDIR}"

# getLatestVersion gets the latest released version of a github project
# $1 = org/repo
function getLatestVersion() {
  curl -sL "https://api.github.com/repos/${1}/releases/latest" | yq '.tag_name'
}

# getLatestMinorVersion gets the latest released version of a github project with a specific minor version prefix
# $1 = org/repo
# $2 = major version prefix
function getLatestMinorVersion() {
  curl -sL "https://api.github.com/repos/${1}/releases" | yq -p json "[.[] | select(.tag_name | test(\"^$2\\.\")) | .tag_name] | .[0]" 
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
go get -u "istio.io/istio@${UPDATE_BRANCH}"
go get -u "istio.io/client-go@${UPDATE_BRANCH}"
go mod tidy

# Update operator-sdk
OPERATOR_SDK_LATEST_VERSION=$(getLatestVersion operator-framework/operator-sdk)
"$SED_CMD" -i "s|OPERATOR_SDK_VERSION ?= .*|OPERATOR_SDK_VERSION ?= ${OPERATOR_SDK_LATEST_VERSION}|" "${ROOTDIR}/Makefile.core.mk"
find "${ROOTDIR}/chart/templates/olm/scorecard.yaml" -type f -exec "$SED_CMD" -i "s|quay.io/operator-framework/scorecard-test:.*|quay.io/operator-framework/scorecard-test:${OPERATOR_SDK_LATEST_VERSION}|" {} +

# Update helm (FIXME: pinned to v3 as we don't support helm4 yet, see https://github.com/istio-ecosystem/sail-operator/issues/1371)
HELM_LATEST_VERSION=$(getLatestMinorVersion helm/helm v3)
"$SED_CMD" -i "s|HELM_VERSION ?= .*|HELM_VERSION ?= ${HELM_LATEST_VERSION}|" "${ROOTDIR}/Makefile.core.mk"

# Update controller-tools
CONTROLLER_TOOLS_LATEST_VERSION=$(getLatestVersion kubernetes-sigs/controller-tools)
"$SED_CMD" -i "s|CONTROLLER_TOOLS_VERSION ?= .*|CONTROLLER_TOOLS_VERSION ?= ${CONTROLLER_TOOLS_LATEST_VERSION}|" "${ROOTDIR}/Makefile.core.mk"

# Update controller-runtime
CONTROLLER_RUNTIME_LATEST_VERSION=$(getLatestVersion kubernetes-sigs/controller-runtime)
# FIXME: Do not use `go get -u` until https://github.com/kubernetes/apimachinery/issues/190 is resolved
# go get -u "sigs.k8s.io/controller-runtime@${CONTROLLER_RUNTIME_LATEST_VERSION}"
go get "sigs.k8s.io/controller-runtime@${CONTROLLER_RUNTIME_LATEST_VERSION}"
CONTROLLER_RUNTIME_BRANCH=$(getReleaseBranch "${CONTROLLER_RUNTIME_LATEST_VERSION}")
"$SED_CMD" -i "s|CONTROLLER_RUNTIME_BRANCH ?= .*|CONTROLLER_RUNTIME_BRANCH ?= ${CONTROLLER_RUNTIME_BRANCH}|" "${ROOTDIR}/Makefile.core.mk"

# Update opm
OPM_LATEST_VERSION=$(getLatestVersion operator-framework/operator-registry)
"$SED_CMD" -i "s|OPM_VERSION ?= .*|OPM_VERSION ?= ${OPM_LATEST_VERSION}|" "${ROOTDIR}/Makefile.core.mk"

# Update olm
OLM_LATEST_VERSION=$(getLatestVersion operator-framework/operator-lifecycle-manager)
"$SED_CMD" -i "s|OLM_VERSION ?= .*|OLM_VERSION ?= ${OLM_LATEST_VERSION}|" "${ROOTDIR}/Makefile.core.mk"

# Update kube-rbac-proxy
RBAC_PROXY_LATEST_VERSION=$(getLatestVersion brancz/kube-rbac-proxy | cut -d/ -f1)
# Only update it if the newer image is available in the registry
if docker manifest inspect "gcr.io/kubebuilder/kube-rbac-proxy:${RBAC_PROXY_LATEST_VERSION}" >/dev/null 2>/dev/null; then
  "$SED_CMD" -i "s|gcr.io/kubebuilder/kube-rbac-proxy:.*|gcr.io/kubebuilder/kube-rbac-proxy:${RBAC_PROXY_LATEST_VERSION}|" "${ROOTDIR}/chart/values.yaml"
fi

# Update gitleaks
GITLEAKS_VERSION=$(getLatestVersion gitleaks/gitleaks)
"$SED_CMD" -i "s|GITLEAKS_VERSION ?= .*|GITLEAKS_VERSION ?= ${GITLEAKS_VERSION}|" "${ROOTDIR}/Makefile.core.mk"

# Update runme
RUNME_LATEST_VERSION=$(getLatestVersion runmedev/runme)
# Remove the leading "v" from the version string
RUNME_LATEST_VERSION=${RUNME_LATEST_VERSION#v}
"$SED_CMD" -i "s|RUNME_VERSION ?= .*|RUNME_VERSION ?= ${RUNME_LATEST_VERSION}|" "${ROOTDIR}/Makefile.core.mk"

# Update misspell
MISSPELL_LATEST_VERSION=$(getLatestVersion client9/misspell)
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
make update-istio gen
