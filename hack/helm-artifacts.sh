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

function show_help() {
    cat <<EOF
    Publish Sail Operator helm artifacts to gh-pages branch
    Usage: ./publish-helm-artifacts.sh [options]
EOF
}

function prepare_repo() {
    echo "Preparing repository..."

    if [ -z "${GITHUB_TOKEN}" ]; then
      die "Please provide GITHUB_TOKEN"
    fi
    if ! command -v helm &> /dev/null; then
      die "Helm command is missing"
    fi

    TMP_DIR=$(mktemp -d)
    trap 'rm -rf "${TMP_DIR}"' EXIT

    git clone --single-branch --depth=1 --branch "${HUB_HELM_BRANCH}" "https://${GITHUB_TOKEN}@${HUB_REPO_URL}" "${TMP_DIR}/${UPSTREAM_OPERATOR_NAME}"
    cd "${TMP_DIR}/${UPSTREAM_OPERATOR_NAME}"

    if ! git config user.name; then
      git config user.name "${GIT_CONFIG_USER_NAME}"
    fi
    if ! git config user.email; then
      git config user.email "${GIT_CONFIG_USER_EMAIL}"
    fi
}

function get_latest_release_version() {
    echo "Fetching latest release version from GitHub API..."

    latest_version=$(curl -s \
      "https://api.github.com/repos/${OWNER}/${UPSTREAM_OPERATOR_NAME}/releases/latest" | \
      jq -r .tag_name)
    echo "Latest release version: ${latest_version}"
}

function check_version_in_index() {
    echo "Checking if version ${latest_version} exists in index.yaml..."

    if grep -q "version: ${latest_version}" index.yaml; then
        echo "Version ${latest_version} is already in index.yaml. Skipping update."
        return 0
    else
        echo "Version ${latest_version} not found in index.yaml. Updating index."
        return 1
    fi
}

function add_version_to_index() {
    echo "Adding new version to Helm repo index..."

    wget "https://${HUB_REPO_URL}/releases/download/${latest_version}/${UPSTREAM_OPERATOR_NAME}-${latest_version}.tgz"
    helm repo index --merge index.yaml . --url "https://${HUB_REPO_URL}/releases/download/${latest_version}/"

    git checkout -b "update_helm_artifact_${latest_version}"
    git add index.yaml
    git commit -m "Add new sail-operator chart release - ${latest_version}"
    git push origin "update_helm_artifact_${latest_version}"

    create_pull_request
}

function create_pull_request {
    echo "Creating pull request..."
    PAYLOAD="${TMP_DIR}/PAYLOAD"

    jq -c -n \
      --arg msg "Add new sail-operator chart release - ${latest_version}" \
      --arg head "${OWNER}:update_helm_artifact_${latest_version}" \
      --arg base "${HUB_HELM_BRANCH}" \
      --arg title "Helm artifact ${latest_version}" \
      '{head: $head, base: $base, title: $title, body: $msg }' > "${PAYLOAD}"

    curl -X POST \
      -H "Authorization: token ${GITHUB_TOKEN}" \
      -H "Accept: application/vnd.github.v3+json" \
      -d @"${PAYLOAD}" \
      "https://api.github.com/repos/${OWNER}/${UPSTREAM_OPERATOR_NAME}/pulls"

    rm "${PAYLOAD}"
}

while test $# -gt 0; do
  case "$1" in
    -h|--help)
        show_help
        exit 0
        ;;
    *)
        echo "Unknown parameter $1"
        exit 1
        ;;
  esac
done

prepare_repo
get_latest_release_version
if ! check_version_in_index; then
  add_version_to_index
fi
