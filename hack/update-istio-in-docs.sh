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

YAML_FILE="pkg/istioversion/versions.yaml"

SED_CMD="sed"
if [[ "$(uname)" == "Darwin" ]]; then
  SED_CMD="gsed"
fi

# Check if yq is installed
if ! command -v yq &> /dev/null
then
    echo "Error: 'yq' command not found." >&2
    echo "Please install yq (e.g., 'brew install yq' or 'sudo snap install yq')." >&2
    exit 1
fi

echo "Extracting the latest two 'version' values from $YAML_FILE..."

LATEST_VERSIONS=$(yq '.versions[] | select(.version) | .version' "$YAML_FILE" | head -n 2)
if [ -z "$LATEST_VERSIONS" ]; then
    echo "No versions with a 'version' property found."
    exit 1
fi

echo "$LATEST_VERSIONS"

# Check all the ascii docs in docs/ and update the istio version variables
# LATEST_VERSIONS[0] is latest and LATEST_VERSIONS[1] is latest minus one
LATEST_VERSION=$(echo "$LATEST_VERSIONS" | sed -n '1p')
LATEST_VERSION_REVISION_FORMAT=$(echo "$LATEST_VERSION" | tr '.' '-')
LATEST_TAG="v${LATEST_VERSION%.*}-latest"
LATEST_RELEASE_NAME="release-${LATEST_VERSION%.*}"
LATEST_MINUS_ONE_VERSION=$(echo "$LATEST_VERSIONS" | sed -n '2p')
LATEST_MINUS_ONE_VERSION_REVISION_FORMAT=$(echo "$LATEST_MINUS_ONE_VERSION" | tr '.' '-')
echo "The versions to update are:"
echo "LATEST_VERSION: $LATEST_VERSION"
echo "LATEST_VERSION_REVISION_FORMAT: $LATEST_VERSION_REVISION_FORMAT"
echo "LATEST_TAG: $LATEST_TAG"
echo "LATEST_RELEASE_NAME: $LATEST_RELEASE_NAME"
echo "LATEST_MINUS_ONE_VERSION: $LATEST_MINUS_ONE_VERSION"
echo "LATEST_MINUS_ONE_VERSION_REVISION_FORMAT: $LATEST_MINUS_ONE_VERSION_REVISION_FORMAT"

for file in docs/**/*.adoc; do
    echo "Updating file: $file"
    "$SED_CMD" -i -E "s/(:istio_latest_version: ).*/\1$LATEST_VERSION/" "$file"
    "$SED_CMD" -i -E "s/(:istio_latest_version_revision_format: ).*/\1$LATEST_VERSION_REVISION_FORMAT/" "$file"
    "$SED_CMD" -i -E "s/(:istio_latest_tag: ).*/\1$LATEST_TAG/" "$file"
    "$SED_CMD" -i -E "s/(:istio_release_name: ).*/\1$LATEST_RELEASE_NAME/" "$file"
    "$SED_CMD" -i -E "s/(:istio_latest_minus_one_version: ).*/\1$LATEST_MINUS_ONE_VERSION/" "$file"
    "$SED_CMD" -i -E "s/(:istio_latest_minus_one_version_revision_format: ).*/\1$LATEST_MINUS_ONE_VERSION_REVISION_FORMAT/" "$file"
done
echo "Documentation files updated successfully."