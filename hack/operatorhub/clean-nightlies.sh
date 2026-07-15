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

set -euo pipefail

# This script creates a PR to https://github.com/redhat-openshift-ecosystem/community-operators-prod
# It removes nightly releases which are older than 2 weeks.
#

CUR_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"

GITHUB_TOKEN="${GITHUB_TOKEN:-}"
GIT_USER="${GIT_USER:-}"

GIT_CONFIG_USER_NAME="${GIT_CONFIG_USER_NAME:-}"
GIT_CONFIG_USER_EMAIL="${GIT_CONFIG_USER_EMAIL:-}"

TMP_DIR=$(mktemp -d)
trap 'rm -rf "${TMP_DIR}"' EXIT

OPERATOR_HUB=${OPERATOR_HUB:-"community-operators-prod"}
OWNER="${OWNER:-"redhat-openshift-ecosystem"}"
HUB_REPO_URL="${HUB_REPO_URL:-https://github.com/${OWNER}/${OPERATOR_HUB}.git}"

FORK="${FORK:-maistra}"
FORK_REPO_URL="${FORK_REPO_URL:-https://${GIT_USER}:${GITHUB_TOKEN}@github.com/${FORK}/${OPERATOR_HUB}.git}"

BRANCH="sail-operator-$(date '+%Y-%m-%d')"

git clone --single-branch --depth=1 --branch main "${HUB_REPO_URL}" "${TMP_DIR}/${OPERATOR_HUB}"

cd "${TMP_DIR}/${OPERATOR_HUB}"
git remote add fork "${FORK_REPO_URL}"
git checkout -b "${BRANCH}"

cd "operators/sailoperator"
# iterate over all directories with 'nightly' in the name
for dir_name in *nightly*
do
  # skip directories which are newer than 2 weeks
  if [[ "${dir_name: -10}" < $(date -d "2 weeks ago" '+%Y-%m-%d') ]]
  then
    echo "Removing ${dir_name}"
    rm -rf "${dir_name}"
    # remove bundle entries from catalog template
    yq 'del(.entries[].entries[] | select(.name |contains("'"${dir_name}"'")))'  -i catalog-templates/basic.yaml
    yq 'del(.entries[] | select(.image |contains("'"${dir_name}"'")))' -i catalog-templates/basic.yaml
  fi
done

# remove empty channels https://github.com/istio-ecosystem/sail-operator/issues/1192
yq 'del(.entries[] | select(.entries | select(length == 0)))'  -i catalog-templates/basic.yaml

# regenerate catalogs and validate them
make catalogs
make validate-catalogs

if ! git config --global user.name; then
  git config --global user.name "${GIT_CONFIG_USER_NAME}"
fi

if ! git config --global user.email; then
  git config --global user.email "${GIT_CONFIG_USER_EMAIL}"
fi

TITLE="Cleaning nightly builds of Sail operator"
cd "${TMP_DIR}/${OPERATOR_HUB}"
git add .
git commit -s -m"${TITLE}"
git push -f fork "${BRANCH}"

PAYLOAD="${TMP_DIR}/PAYLOAD"

jq -c -n \
  --arg msg "$(cat "${CUR_DIR}"/operatorhub-pr-template.md)" \
  --arg head "${FORK}:${BRANCH}" \
  --arg base main \
  --arg title "${TITLE}" \
   '{head: $head, base: $base, title: $title, body: $msg }' > "${PAYLOAD}"

curl \
  --fail-with-body \
  -X POST \
  -H "Authorization: token ${GITHUB_TOKEN}" \
  -H "Accept: application/vnd.github.v3+json" \
  https://api.github.com/repos/"${OWNER}/${OPERATOR_HUB}"/pulls \
   --data-binary "@${PAYLOAD}"
