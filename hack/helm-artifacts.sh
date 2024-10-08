#!/bin/bash
# shellcheck disable=SC1091

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

set -euo pipefail

CUR_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
source "${CUR_DIR}"/validate_semver.sh

GITHUB_TOKEN="${GITHUB_TOKEN:-}"

GIT_CONFIG_USER_NAME="${GIT_CONFIG_USER_NAME:-}"
GIT_CONFIG_USER_EMAIL="${GIT_CONFIG_USER_EMAIL:-}"

UPSTREAM_OPERATOR_NAME="${UPSTREAM_OPERATOR_NAME:-"sail-operator"}"
OWNER="${OWNER:-"istio-ecosystem"}"
HUB_REPO_URL="${HUB_REPO_URL:-github.com/${OWNER}/${UPSTREAM_OPERATOR_NAME}}"
HUB_HELM_BRANCH="${HUB_HIVE_BRANCH:-"gh-pages"}"
HUB_HELM_ARTIFACT_URL="https://${HUB_REPO_URL}/releases/download/${OPERATOR_VERSION}"/

: "${OPERATOR_VERSION:?"Missing OPERATOR_VERSION variable"}"

show_help() {
    cat <<EOF

    Publish Sail Operator helm artifacts to gh-pages branch
    It will allow to install Sail Operator via "helm add repo"

    ./publish-helm-artifacts.sh [options]

EOF
}

function update_repo_index() {
    echo "Update Helm repo index with new artifact"
}

function prepare_repo() {
    echo "Prepare repository"

    if [ -z "${GITHUB_TOKEN}" ]; then
      die "Please provide GITHUB_TOKEN"
    fi
    if ! command -v helm &> /dev/null; then
      die "Helm command is missing"
    fi

    TMP_DIR=$(mktemp -d)
    trap 'rm -rf "${TMP_DIR}"' EXIT

    git clone --single-branch --depth=1 --branch "${HUB_HELM_BRANCH}" "https://${GIT_USER}:${GITHUB_TOKEN}@${HUB_REPO_URL}" "${TMP_DIR}/${UPSTREAM_OPERATOR_NAME}"
    cd "${TMP_DIR}/${UPSTREAM_OPERATOR_NAME}"

    if ! git config user.name; then
      git config user.name "${GIT_CONFIG_USER_NAME}"
    fi

    if ! git config user.email; then
      git config user.email "${GIT_CONFIG_USER_EMAIL}"
    fi
}

function fetch_released_artifact() {
    echo "Fetch released helm artifact"

    wget "${HUB_HELM_ARTIFACT_URL}/${UPSTREAM_OPERATOR_NAME}-${OPERATOR_VERSION}.tgz"
}

function update_helm_repo_index() {
    echo "Update index of Helm repo"
    local helm_branch="update_helm_artifact_${OPERATOR_VERSION}"

    git checkout -b "$helm_branch"
    helm repo index --merge index.yaml . --url "${HUB_HELM_ARTIFACT_URL}"
    git add index.yaml
    git commit -m "Add new sail-operator chart release - ${OPERATOR_VERSION}"
    git push origin "$helm_branch"

    PAYLOAD="${TMP_DIR}/PAYLOAD"

    jq -c -n \
      --arg msg "Add new sail-operator chart release - ${OPERATOR_VERSION}" \
      --arg head "${OWNER}:${helm_branch}" \
      --arg base "${HUB_HELM_BRANCH}" \
      --arg title "Helm artifact ${OPERATOR_VERSION}" \
      '{head: $head, base: $base, title: $title, body: $msg }' > "${PAYLOAD}"

    curl --fail-with-body -X POST \
      -H "Authorization: token ${GITHUB_TOKEN}" \
      -H "Accept: application/vnd.github.v3+json" \
      https://api.github.com/repos/"${OWNER}/${UPSTREAM_OPERATOR_NAME}"/pulls \
      --data-binary "@${PAYLOAD}"
}

while test $# -gt 0; do
  case "$1" in
    -h|--help)
        show_help
        exit 0
        ;;
    *)
        echo "Unknown param $1"
        exit 1
        ;;
  esac
done

prepare_repo
fetch_released_artifact
update_helm_repo_index
