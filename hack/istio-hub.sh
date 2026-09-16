#!/bin/bash
# shellcheck disable=SC2034 # this file is only meant to be sourced

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

# Shared configuration describing where the Istio operand images come from.
# Sourced by hack/download-charts.sh, which points the charts at the right registry,
# and by hack/mirror-istio-images.sh, which populates the mirror.
#
# Istio stopped publishing to registry.istio.io as of 1.31; those releases only exist
# on Docker Hub, which we can't pull from at runtime. Releases older than that are
# still published to registry.istio.io/release, so nothing has to be mirrored for them.

ISTIO_HUB_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
MAKEFILE=${MAKEFILE:-"$(dirname "${ISTIO_HUB_DIR}")/Makefile.core.mk"}

# getVarFromMakefile prints the default value of a variable declared in Makefile.core.mk,
# so the Makefile remains the single place where it is configured.
# $1: the variable name, e.g. MIRROR_ISTIO_MINORS
function getVarFromMakefile() {
  grep "^${1} ?= " "${MAKEFILE}" | sed "s/^${1} ?= *//"
}

# Istio minor releases that are served from MIRROR_HUB. Defined in Makefile.core.mk.
MIRROR_ISTIO_MINORS=${MIRROR_ISTIO_MINORS:-$(getVarFromMakefile MIRROR_ISTIO_MINORS)}
if [ -z "${MIRROR_ISTIO_MINORS}" ]; then
  echo "could not read MIRROR_ISTIO_MINORS from ${MAKEFILE}" >&2
  exit 1
fi

# Registry the mirrored images are copied from.
SOURCE_HUB=${SOURCE_HUB:-"docker.io/istio"}

# Registry holding the mirrored images.
MIRROR_HUB=${MIRROR_HUB:-"quay.io/sail-dev"}

# Registry holding the images that upstream still publishes itself.
UPSTREAM_HUB=${UPSTREAM_HUB:-"registry.istio.io/release"}

# Operand images referenced by the Istio charts.
ISTIO_IMAGES=(pilot proxyv2 install-cni ztunnel)

# usesMirror returns 0 if the images for the given Istio version are served from
# MIRROR_HUB rather than UPSTREAM_HUB.
# $1: the Istio version, e.g. 1.31.0 or 1.31.0-beta.1
function usesMirror() {
  local version="$1" minor
  for minor in ${MIRROR_ISTIO_MINORS}; do
    if [[ "${version}" == "${minor}."* ]]; then
      return 0
    fi
  done
  return 1
}
