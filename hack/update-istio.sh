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

SLEEP_TIME=10
VERSIONS_YAML_FILE=${VERSIONS_YAML_FILE:-"pkg/istioversions/versions.yaml"}

# Add a new entry in versions.yaml file.
# First argument is the new version (e.g. 1.22.5)
# Second argument is the old version (e.g. 1.22.4)
#
# The new entry will be placed immediately before the old one
function add_stable_version() {
    echo "Adding new stable version: ${1}"
    # we want to add the istiod-remote chart only for 1.23
    istiod_remote_line=""
    if [[ ${1} == 1.23.* ]]
    then
      istiod_remote_line="\"https://istio-release.storage.googleapis.com/charts/istiod-remote-${1}.tgz\","
    fi
    template=$(cat <<-END
{
  "name": "v${1}",
  "version": "${1}",
  "repo": "https://github.com/istio/istio",
  "commit": "${1}",
  "charts": [
    "https://istio-release.storage.googleapis.com/charts/base-${1}.tgz",
    "https://istio-release.storage.googleapis.com/charts/istiod-${1}.tgz",
    ${istiod_remote_line}
    "https://istio-release.storage.googleapis.com/charts/gateway-${1}.tgz",
    "https://istio-release.storage.googleapis.com/charts/cni-${1}.tgz",
    "https://istio-release.storage.googleapis.com/charts/ztunnel-${1}.tgz"
    ]
}
END
    )

    # Insert the new key above the old one (https://stackoverflow.com/questions/74368503/is-it-possible-to-insert-an-element-into-a-middle-of-array-in-yaml-using-yq)
    # shellcheck disable=SC2016
    yq -i '.versions |=  (
        (.[] | select(.name == "v'"${2}"'") | key) as $pos |
        .[:$pos] +
        ['"${template}"'] +
        .[$pos:])' "${VERSIONS_YAML_FILE}"
}

# Given an input with potentially several major.minor versions, list only the latest one
# e.g.: For the input below:
#   1.23.0
#   1.22.1
#   1.22.0
#   1.21.1
#   1.21.0
#
# Output is:
#   1.23.0
#   1.22.1
#   1.21.1
function list_only_latest() {
    local current tmp=""
    while read -r input; do
        IFS="." read -r -a version <<< "${input}"
        current="${version[0]}.${version[1]}"
        if [[ "${current}" != "${tmp}" ]]; then
            echo "${input}"
            tmp=${current}
        fi
    done
}

function update_stable() {
    all_releases=$(curl -sL "https://api.github.com/repos/istio/istio/releases" | yq '.[].tag_name' -oy)
    supported_versions=$(yq '.versions[] | .name' "${VERSIONS_YAML_FILE}" | grep -v latest | list_only_latest)

    # For each supported version, look for a greater version in the all_releases list
    for version in ${supported_versions}; do
        version="${version:1}" # remove 'v' prefix, e.g. v1.21.0 => 1.21.0
        IFS="." read -r -a version_array <<< "${version}" # split version into an array for major, minor and patch
        latest_release=$(echo "${all_releases}" | grep "${version_array[0]}.${version_array[1]}." | head -1) # get the latest release for "major.minor"
        if [[ "${version}" != "${latest_release}" ]]; then
            add_stable_version "${latest_release}" "${version}"
        fi
    done
}

function update_latest() {
    COMMIT=$(yq '.versions[] | select(.name == "latest") | "git ls-remote --heads " + .repo + ".git " + .branch + " | cut -f 1"' "${VERSIONS_YAML_FILE}" | sh)
    CURRENT=$(yq '.versions[] | select(.name == "latest") | .commit' "${VERSIONS_YAML_FILE}")

    if [ "${COMMIT}" == "${CURRENT}" ]; then
        echo "${VERSIONS_YAML_FILE} is already up-to-date with latest commit ${COMMIT}."
        return
    fi

    echo Updating version 'latest' to commit "${COMMIT}"
    echo "Verifying the artifacts are available on GCS, this might take a while - you can abort the wait with CTRL+C"

    URL="https://storage.googleapis.com/istio-build/dev/${COMMIT}"
    until curl --output /dev/null --silent --head --fail "${URL}"; do
        echo -n '.'
        sleep ${SLEEP_TIME}
    done
    echo

    VERSION=$(curl -sSfL "${URL}")
    echo Version: "${VERSION}"

    yq -i '
        (.versions[] | select(.name == "latest") | .version) = "'"${VERSION}"'" |
        (.versions[] | select(.name == "latest") | .commit) = "'"${COMMIT}"'" |
        (.versions[] | select(.name == "latest") | .charts) = [
            "https://storage.googleapis.com/istio-build/dev/'"${VERSION}"'/helm/base-'"${VERSION}"'.tgz",
            "https://storage.googleapis.com/istio-build/dev/'"${VERSION}"'/helm/cni-'"${VERSION}"'.tgz",
            "https://storage.googleapis.com/istio-build/dev/'"${VERSION}"'/helm/gateway-'"${VERSION}"'.tgz",
            "https://storage.googleapis.com/istio-build/dev/'"${VERSION}"'/helm/istiod-'"${VERSION}"'.tgz",
            "https://storage.googleapis.com/istio-build/dev/'"${VERSION}"'/helm/ztunnel-'"${VERSION}"'.tgz"
        ]' "${VERSIONS_YAML_FILE}"
}

update_stable
update_latest
