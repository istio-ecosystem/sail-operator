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

################################################################################
# Script to update EOL flags in versions.yaml based on upstream Istio support
################################################################################

set -euo pipefail

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
REPO_ROOT="${SCRIPT_DIR}/.."

VERSIONS_FILE="${VERSIONS_YAML_FILE:-${REPO_ROOT}/pkg/istioversion/versions.yaml}"
TEMP_DIR=$(mktemp -d)
SUPPORTED_VERSIONS_FILE="${TEMP_DIR}/supported_versions.txt"
CHANGES_MADE=false

# Cleanup temp directory on exit
trap 'rm -rf "${TEMP_DIR}"' EXIT

# Check if required tools are available
if ! command -v yq &> /dev/null; then
  echo "ERROR: yq is required but not found"
  echo "Install from: https://github.com/mikefarah/yq"
  exit 1
fi

if ! command -v jq &> /dev/null; then
  echo "ERROR: jq is required but not found"
  echo "Install from: https://jqlang.github.io/jq/"
  exit 1
fi

echo "Fetching currently supported Istio versions from endoflife.date API..."

# Fetch supported versions from endoflife.date API (those with EOL date in the future)
curl -s https://endoflife.date/api/istio.json | \
  jq -r --arg today "$(date +%Y-%m-%d)" \
    '.[] | select(.eol > $today) | .cycle' | \
  sort -V > "${SUPPORTED_VERSIONS_FILE}"

if [ ! -s "${SUPPORTED_VERSIONS_FILE}" ]; then
  echo "ERROR: Failed to fetch supported versions from endoflife.date API"
  exit 1
fi

echo "Currently supported Istio versions (major.minor):"
cat "${SUPPORTED_VERSIONS_FILE}"
echo ""

# Function to check if a version is supported
is_version_supported() {
  local version=$1
  # Extract major.minor from version (e.g., v1.28.3 -> 1.28)
  local major_minor
  major_minor=$(echo "$version" | grep -oE '[0-9]+\.[0-9]+' | head -n1)

  if [ -z "$major_minor" ]; then
    # If we can't parse it (e.g., master, alpha versions), consider it supported
    return 0
  fi

  if grep -q "^${major_minor}$" "${SUPPORTED_VERSIONS_FILE}"; then
    return 0  # Supported
  else
    return 1  # Not supported (EOL)
  fi
}

echo "Analyzing versions.yaml and updating EOL flags..."
echo ""

# Get the number of versions
version_count=$(yq '.versions | length' "${VERSIONS_FILE}")

# Iterate through each version
for i in $(seq 0 $((version_count - 1))); do
  version_name=$(yq ".versions[$i].name" "${VERSIONS_FILE}")

  # Remove quotes from version name if present
  version_name=$(echo "$version_name" | tr -d '"')

  # Skip special versions
  if [[ "$version_name" == "master" ]] || [[ "$version_name" == *"alpha"* ]]; then
    continue
  fi

  # Check if version has EOL flag
  has_eol=$(yq ".versions[$i].eol // false" "${VERSIONS_FILE}")

  if is_version_supported "$version_name"; then
    # Version is supported upstream
    if [ "$has_eol" = "true" ]; then
      echo "WARNING: ${version_name} is supported upstream but marked as EOL"
      echo "         Manual intervention may be required to restore metadata"
    fi
  else
    # Version is NOT supported upstream (EOL)
    if [ "$has_eol" != "true" ]; then
      echo "Marking ${version_name} as EOL..."

      # Check if version has a ref field
      has_ref=$(yq ".versions[$i] | has(\"ref\")" "${VERSIONS_FILE}")

      if [ "$has_ref" = "true" ]; then
        ref_value=$(yq ".versions[$i].ref" "${VERSIONS_FILE}")
        # Keep name, ref, and eol (in that order to match existing format)
        yq -i ".versions[$i] = {\"name\": \"${version_name}\", \"ref\": \"${ref_value}\", \"eol\": true}" "${VERSIONS_FILE}"
      else
        # Keep only name and eol
        yq -i ".versions[$i] = {\"name\": \"${version_name}\", \"eol\": true}" "${VERSIONS_FILE}"
      fi

      CHANGES_MADE=true
    fi
  fi
done

echo ""
if [ "$CHANGES_MADE" = true ]; then
  echo "Running 'make gen' to regenerate code..."
  cd "${REPO_ROOT}"
  make gen
  echo ""
  echo "✓ EOL version updates completed successfully!"
  echo ""
  echo "Summary of changes:"
  git diff --stat "${VERSIONS_FILE}" || true
  exit 0
else
  echo "✓ No changes needed. All versions are already up-to-date."
  exit 0
fi
