#!/bin/bash
# shellcheck disable=SC1091,SC2001

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
source "${CUR_DIR}"/../validate_semver.sh

GITHUB_TOKEN="${GITHUB_TOKEN:-}"
GIT_USER="${GIT_USER:-}"

GIT_CONFIG_USER_NAME="${GIT_CONFIG_USER_NAME:-}"
GIT_CONFIG_USER_EMAIL="${GIT_CONFIG_USER_EMAIL:-}"

# The OPERATOR_NAME is defined in Makefile
: "${OPERATOR_NAME:?"Missing OPERATOR_NAME variable"}"
: "${OPERATOR_VERSION:?"Missing OPERATOR_VERSION variable"}"
: "${CHANNELS:?"Missing CHANNELS variable"}"
: "${PREVIOUS_VERSION:?"Missing PREVIOUS_VERSION variable"}"

show_help() {
  echo "publish-bundle - raises PR to Operator Hub"
  echo " "
  echo "./publish-bundle.sh [options]"
  echo " "
  echo "Options:"
  echo "-h, --help        shows brief help"
  echo "-d, --dry-run     skips push to GH and PR"
}

dryRun=""

skipInDryRun() {
  if [ -n "${dryRun}" ]; then
    echo "# $*"
  else
    "$@"
  fi
}

while test $# -gt 0; do
  case "$1" in
    -h|--help)
            show_help
            exit 0
            ;;
    -d|--dry-run)
            dryRun=true
            shift
            ;;
    *)
            echo "Unknown param $1"
            exit 1
            ;;
  esac
done


# Validations
validate_semantic_versioning "v${OPERATOR_VERSION}"

if [ -z "${dryRun}" ] && [ -z "${GITHUB_TOKEN}" ]; then
  die "Please provide GITHUB_TOKEN"
fi

TMP_DIR=$(mktemp -d)
trap 'rm -rf "${TMP_DIR}"' EXIT

OPERATOR_HUB=${OPERATOR_HUB:-"community-operators-prod"}
OWNER="${OWNER:-"redhat-openshift-ecosystem"}"
HUB_REPO_URL="${HUB_REPO_URL:-https://github.com/${OWNER}/${OPERATOR_HUB}.git}"
HUB_BASE_BRANCH="${HUB_BASE_BRANCH:-main}"

FORK="${FORK:-openshift-service-mesh}"
FORK_REPO_URL="${FORK_REPO_URL:-https://${GIT_USER}:${GITHUB_TOKEN}@github.com/${FORK}/${OPERATOR_HUB}.git}"

BRANCH=${BRANCH:-"${OPERATOR_NAME}-${OPERATOR_VERSION}"}

git clone --single-branch --depth=1 --branch "${HUB_BASE_BRANCH}" "${HUB_REPO_URL}" "${TMP_DIR}/${OPERATOR_HUB}"

cd "${TMP_DIR}/${OPERATOR_HUB}"
git remote add fork "${FORK_REPO_URL}"
git checkout -b "${BRANCH}"

OPERATORS_DIR="operators/${OPERATOR_NAME}/${OPERATOR_VERSION}/"
BUNDLE_DIR="${CUR_DIR}"/../../bundle
mkdir -p "${OPERATORS_DIR}"
cp -a "${BUNDLE_DIR}"/. "${OPERATORS_DIR}"

# Generate release-config.yaml which is required to update FBC. FBC is only available in community-operators-prod atm
if [ "${OPERATOR_HUB}" = "community-operators-prod" ]
then
  # when publishing a nightly build, we want to get previous build version automatically
  if [[ ${OPERATOR_VERSION} == *"nightly"* ]]
  then
    # expecting there is only one channel in $CHANNELS when pushing nightly builds
    LATEST_VERSION=$(yq '.entries[] | select(.schema == "olm.channel" and .name == '\""${CHANNELS}"\"').entries[-1].name' "${OPERATORS_DIR}../catalog-templates/basic.yaml")
    # there is no entry in the given channel, probably a new channel and first version to be pushed there. Let's use previous nightly channel.
    if [ -z "${LATEST_VERSION}" ]
    then
      PREVIOUS_MINOR=$(echo "${PREVIOUS_VERSION}" | cut -f1,2 -d'.')
      LATEST_VERSION=$(yq '.entries[] | select(.schema == "olm.channel" and .name == '\""${PREVIOUS_MINOR}-nightly"\"').entries[-1].name' "${OPERATORS_DIR}../catalog-templates/basic.yaml")
      if [ -z "${LATEST_VERSION}" ]
      then
        echo "Unable to find previous nightly version. Exiting."
        exit 1
      fi
    fi
  else
    LATEST_VERSION="${OPERATOR_NAME}.v${PREVIOUS_VERSION}"
  fi
  # yaml linter in community-operators-prod CI is expecting a space after a comma
  CHANNELS_SANITIZED=$(echo "${CHANNELS}" | sed 's/, */, /g')
  cat <<EOF > "${OPERATORS_DIR}/release-config.yaml"
catalog_templates:
  - template_name: basic.yaml
    channels: [${CHANNELS_SANITIZED}]
    replaces: ${LATEST_VERSION}
    skipRange: '>=1.0.0 <${OPERATOR_VERSION}'
EOF
fi

if ! git config --global user.name; then
  skipInDryRun git config --global user.name "${GIT_CONFIG_USER_NAME}"
fi

if ! git config --global user.email; then
  skipInDryRun git config --global user.email "${GIT_CONFIG_USER_EMAIL}"
fi

TITLE="operator ${OPERATOR_NAME} (${OPERATOR_VERSION})"
skipInDryRun git add .
skipInDryRun git commit -s -m"${TITLE}"
skipInDryRun git push -f fork "${BRANCH}"

PAYLOAD="${TMP_DIR}/PAYLOAD"

jq -c -n \
  --arg msg "$(cat "${CUR_DIR}"/operatorhub-pr-template.md)" \
  --arg head "${FORK}:${BRANCH}" \
  --arg base "${HUB_BASE_BRANCH}" \
  --arg title "${TITLE}" \
   '{head: $head, base: $base, title: $title, body: $msg }' > "${PAYLOAD}"

if $dryRun; then
  echo -e "${PAYLOAD}\n------------------"
  jq . "${PAYLOAD}"
fi

skipInDryRun curl \
  --fail-with-body \
  -X POST \
  -H "Authorization: token ${GITHUB_TOKEN}" \
  -H "Accept: application/vnd.github.v3+json" \
  https://api.github.com/repos/"${OWNER}/${OPERATOR_HUB}"/pulls \
   --data-binary "@${PAYLOAD}"
