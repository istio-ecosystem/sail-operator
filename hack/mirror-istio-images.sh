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

# Mirrors the Istio operand images, including all their variants, from SOURCE_HUB to MIRROR_HUB.
#
# Istio stopped publishing to registry.istio.io as of 1.31, so from that release on the
# only upstream source is Docker Hub, which we can't pull from at runtime. tools/update_deps.sh
# runs this after adding a new Istio version but before generating the manifests that
# reference it, so the images are always in place before anything points at them.
#
# Takes the path to versions.yaml, followed by one or more Istio minor releases,
# defaulting to MIRROR_ISTIO_MINORS in the Makefile, and mirrors every matching
# version in versions.yaml, e.g.:
#
#   hack/mirror-istio-images.sh pkg/istioversion/versions.yaml 1.31
#
# Requires crane to be authenticated against MIRROR_HUB.

set -euo pipefail

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

# shellcheck source=hack/istio-hub.sh
source "${SCRIPT_DIR}/istio-hub.sh"

if [ $# -eq 0 ]; then
  echo "usage: $(basename "$0") <versions-yaml-path> [minor...]" >&2
  exit 1
fi
VERSIONS_YAML_PATH=$1
shift

# crane binary to use. The Makefile pins the version and installs it into bin/.
CRANE=${CRANE:-crane}

# When true, log what would be copied without copying anything.
DRY_RUN=${DRY_RUN:-false}

# Default to the minor releases configured in the Makefile when none are given.
if [ $# -eq 0 ]; then
  # shellcheck disable=SC2086 # MIRROR_ISTIO_MINORS is a space-separated list
  set -- ${MIRROR_ISTIO_MINORS}
fi

for cmd in yq "${CRANE}"; do
  if ! command -v "${cmd}" &> /dev/null; then
    echo "${cmd} command not found. Please install ${cmd} to run this script."
    exit 1
  fi
done

# versionsForMinor prints the versions in versions.yaml belonging to the given minor
# release. Aliases (which have a ref) have nothing of their own to mirror, EOL versions
# aren't installable, and dev builds (which have a branch) aren't published to SOURCE_HUB.
# $1: the minor release, e.g. 1.31
function versionsForMinor() {
  yq '.versions[]
      | select(has("ref") | not)
      | select(.eol != true)
      | select(has("branch") | not)
      | select(.version == "'"${1}"'.*")
      | .version' "${VERSIONS_YAML_PATH}"
}

# get_digest prints the digest of an image, nothing if the image doesn't exist, and
# returns non-zero for any other crane failure.
# $1: image reference
function get_digest() {
  local image_ref=$1 output
  if output=$("${CRANE}" digest "${image_ref}" 2>&1); then
    echo "${output}"
    return 0
  fi

  # A HEAD 404 can precede a different GET failure, so only the last line decides.
  if [[ "${output##*$'\n'}" =~ MANIFEST_UNKNOWN|NAME_UNKNOWN|"unexpected status code 404" ]]; then
    return 0
  fi

  printf '%s\n' "${output}" >&2
  echo "  ERROR: failed to get digest for ${image_ref}" >&2
  return 1
}

failures=()

for minor in "$@"; do
  versions=$(versionsForMinor "${minor}")
  if [ -z "${versions}" ]; then
    echo "WARNING: no versions found for ${minor} in ${VERSIONS_YAML_PATH}"
    continue
  fi

  for version in ${versions}; do
    echo "mirroring ${version}"
    for image in "${ISTIO_IMAGES[@]}"; do
      for variant in "${ISTIO_IMAGE_VARIANTS[@]}"; do
        src="${SOURCE_HUB}/${image}:${version}${variant}"
        dst="${MIRROR_HUB}/${image}:${version}${variant}"

        src_digest=$(get_digest "${src}") || exit 1
        if [ -z "${src_digest}" ]; then
          # Only the default variant is guaranteed to exist for every image.
          if [ -n "${variant}" ]; then
            echo "  WARNING: ${src} does not exist, skipping"
            continue
          fi
          echo "  ERROR: ${src} does not exist"
          failures+=("${src}")
          continue
        fi

        dst_digest=$(get_digest "${dst}") || exit 1
        if [ "${src_digest}" == "${dst_digest}" ]; then
          echo "  up to date: ${dst}"
          continue
        fi

        echo "  copying ${src} -> ${dst}"
        if [ "${DRY_RUN}" == "true" ]; then
          continue
        fi
        if ! "${CRANE}" copy "${src}" "${dst}"; then
          echo "  ERROR: failed to copy ${src} -> ${dst}"
          failures+=("${src}")
        fi
      done
    done
  done
done

if [ ${#failures[@]} -gt 0 ]; then
  echo "failed to mirror ${#failures[@]} image(s):"
  printf '  %s\n' "${failures[@]}"
  exit 1
fi

echo "all images have been mirrored to ${MIRROR_HUB}"
