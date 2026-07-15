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

# Parse arguments first so --help works without environment variables
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

# The OPERATOR_NAME is defined in Makefile
: "${OPERATOR_NAME:?"Missing OPERATOR_NAME variable"}"
: "${OPERATOR_VERSION:?"Missing OPERATOR_VERSION variable"}"
: "${CHANNELS:?"Missing CHANNELS variable"}"

# Check required tools early for clearer failures
command -v curl >/dev/null 2>&1 || { echo "ERROR: curl is required but not found" >&2; exit 1; }
command -v yq >/dev/null 2>&1 || { echo "ERROR: yq is required but not found (https://github.com/mikefarah/yq)" >&2; exit 1; }

# FBC catalog URL - used for auto-detection and validation
FBC_URL="https://raw.githubusercontent.com/redhat-openshift-ecosystem/community-operators-prod/main/operators/${OPERATOR_NAME}/catalog-templates/basic.yaml"

# Extract version components (used for auto-detection and validation)
OPERATOR_BASE_VERSION=$(echo "${OPERATOR_VERSION}" | sed 's/-.*$//')
OPERATOR_PATCH=$(echo "${OPERATOR_BASE_VERSION}" | cut -d. -f3)
OPERATOR_MINOR_VERSION=$(echo "${OPERATOR_BASE_VERSION}" | cut -d. -f1,2)

# Auto-detect PREVIOUS_VERSION from FBC (File-Based Catalog)
# This is used to generate the OLM upgrade graph in FBC for stable releases.
# For nightly releases, the previous version is fetched directly from the FBC catalog using the same approach.
#
# The FBC catalog is the authoritative source of truth for OLM upgrade graphs.
# This queries the 'stable' channel in the community-operators-prod FBC catalog to get the latest released version.
#
# To override: export PREVIOUS_VERSION=X.Y.Z before calling this script
#
if [ -z "${PREVIOUS_VERSION:-}" ]; then
  echo "Auto-detecting PREVIOUS_VERSION from FBC catalog..."

  ALL_STABLE=$(curl -sf "${FBC_URL}" 2>/dev/null | yq -r '.entries[] | select(.schema == "olm.channel" and .name == "stable").entries[].name' 2>/dev/null | grep -v '^null$' || true)

  if [ -z "${ALL_STABLE}" ]; then
    echo "ERROR: Failed to fetch versions from FBC catalog." >&2
    echo "  URL: ${FBC_URL}" >&2
    echo "  Network connection and yq are required for release." >&2
    echo "  To override: export PREVIOUS_VERSION=X.Y.Z" >&2
    exit 1
  fi

  if [[ ${OPERATOR_VERSION} != *"nightly"* ]] && [ "${OPERATOR_PATCH}" != "0" ]; then
    PREVIOUS_VERSION=$(echo "${ALL_STABLE}" | sed "s/^${OPERATOR_NAME}\.v//" | grep "^${OPERATOR_MINOR_VERSION}\." | sort -V | tail -1)
    if [ -z "${PREVIOUS_VERSION}" ]; then
      echo "ERROR: No previous version found in FBC for minor series ${OPERATOR_MINOR_VERSION}." >&2
      echo "  To override: export PREVIOUS_VERSION=X.Y.Z" >&2
      exit 1
    fi
  else
    PREVIOUS_VERSION=$(echo "${ALL_STABLE}" | sed "s/^${OPERATOR_NAME}\.v//" | sort -V | tail -1)
  fi
  echo "Detected PREVIOUS_VERSION: ${PREVIOUS_VERSION}"
else
  echo "Using provided PREVIOUS_VERSION: ${PREVIOUS_VERSION}"
fi

echo "Using PREVIOUS_VERSION: ${PREVIOUS_VERSION} for OLM upgrade graph"

# Validate version ordering
# Skip for nightly builds as they use date-based versioning
if [[ ${OPERATOR_VERSION} != *"nightly"* ]]; then
  # Extract PREVIOUS_VERSION components for validation
  PREVIOUS_BASE_VERSION=$(echo "${PREVIOUS_VERSION}" | sed 's/-.*$//')
  PREVIOUS_MINOR_VERSION=$(echo "${PREVIOUS_BASE_VERSION}" | cut -d. -f1,2)

  # Rule 1: If publishing a new minor version (.0 release), it must be newer than latest
  if [ "${OPERATOR_PATCH}" = "0" ]; then
    # Check if versions are equal
    if [ "${OPERATOR_BASE_VERSION}" = "${PREVIOUS_BASE_VERSION}" ]; then
      echo "ERROR: Cannot publish same version as latest in catalog!" >&2
      echo "  Attempting to publish: ${OPERATOR_VERSION}" >&2
      echo "  Latest in catalog:     ${PREVIOUS_VERSION}" >&2
      exit 1
    fi

    # Check if it's older
    NEWER=$(printf "%s\n%s\n" "${OPERATOR_BASE_VERSION}" "${PREVIOUS_BASE_VERSION}" | sort -V | tail -1)
    if [ "${NEWER}" != "${OPERATOR_BASE_VERSION}" ]; then
      echo "ERROR: Cannot publish older minor version than latest in catalog!" >&2
      echo "  Attempting to publish: ${OPERATOR_VERSION} (minor: ${OPERATOR_MINOR_VERSION})" >&2
      echo "  Latest in catalog:     ${PREVIOUS_VERSION} (minor: ${PREVIOUS_MINOR_VERSION})" >&2
      echo "  New minor versions must be greater than the latest." >&2
      exit 1
    fi

    echo "✓ Version check passed: New minor version ${OPERATOR_VERSION} > ${PREVIOUS_VERSION}"
  else
    # Rule 2: For patch releases, validate the minor series exists in FBC
    # It's OK to publish 1.28.4 even when latest is 1.30.0, as long as 1.28 series exists
    echo "Validating patch release ${OPERATOR_VERSION} for minor series ${OPERATOR_MINOR_VERSION}..."

    # Fetch all versions from FBC stable channel
    ALL_VERSIONS=$(curl -sf "${FBC_URL}" 2>/dev/null | yq ".entries[] | select(.schema == \"olm.channel\" and .name == \"stable\").entries[].name" 2>/dev/null)

    # Check if we successfully fetched data
    if [ -z "${ALL_VERSIONS}" ]; then
      echo "ERROR: Failed to fetch version list from FBC catalog!" >&2
      echo "  URL: ${FBC_URL}" >&2
      echo "  Network connection required for validation." >&2
      exit 1
    fi

    # Check if this minor version exists in the fetched list
    MINOR_EXISTS=$(echo "${ALL_VERSIONS}" | grep -c "^${OPERATOR_NAME}\.v${OPERATOR_MINOR_VERSION}\." || true)

    if [ "${MINOR_EXISTS}" -eq 0 ]; then
      echo "ERROR: Cannot publish patch for non-existent minor version!" >&2
      echo "  Attempting to publish: ${OPERATOR_VERSION}" >&2
      echo "  Minor version ${OPERATOR_MINOR_VERSION} does not exist in FBC stable channel." >&2
      echo "  Available minor versions:" >&2
      echo "${ALL_VERSIONS}" | sed "s/${OPERATOR_NAME}\.v//" | cut -d. -f1,2 | sort -uV | tail -10 >&2
      exit 1
    fi

    echo "✓ Version check passed: Patch release ${OPERATOR_VERSION} for existing minor series ${OPERATOR_MINOR_VERSION}"
  fi
fi

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
