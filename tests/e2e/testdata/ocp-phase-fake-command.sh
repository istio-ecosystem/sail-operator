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

set -o nounset
set -o pipefail

name=$(basename "$0")
{
  printf '%s' "${name}"
  printf ' %q' "$@"
  printf '\n'
} >> "${MOCK_LOG}"

case "${name}" in
  go)
    if [ "${1:-}" = env ]; then
      case "${2:-}" in
        GOBIN) printf '\n' ;;
        GOPATH) printf '%s\n' "${MOCK_TMP}/gopath" ;;
        GOOS) printf 'linux\n' ;;
        GOARCH) printf 'amd64\n' ;;
      esac
      exit 0
    fi
    if [ -n "${MOCK_SIGNAL_PARENT:-}" ]; then
      kill -s "${MOCK_SIGNAL_PARENT}" "${PPID}"
    fi
    if [ "${MOCK_CREATE_REPORT:-false}" = true ]; then
      mkdir -p "${ARTIFACTS}"
      printf '<testsuite/>\n' > "${ARTIFACTS}/report.xml"
    fi
    exit "${MOCK_SUITE_RC:-0}"
    ;;
  make)
    exit "${MOCK_NESTED_MAKE_RC:-0}"
    ;;
  oc|kubectl)
    if [ "${1:-}" = get ] && [ "${2:-}" = clusteroperator ]; then
      printf '{"items":[]}\n'
      exit 0
    fi
    if [ "${1:-}" = get ] && [ "${2:-}" = ns ]; then
      exit 1
    fi
    if [ "${MOCK_CLEANUP_FAIL:-false}" = true ] &&
      [ "${1:-}" = delete ] && [ "${2:-}" = namespace ]; then
      exit 23
    fi
    exit 0
    ;;
  yq)
    if [ "${2:-}" = .spec.install.spec.deployments\[0\].name ]; then
      printf 'sailoperator-controller-manager\n'
    else
      printf 'istio-sample\n'
    fi
    ;;
  helm)
    if [ "${1:-}" = version ]; then
      printf 'version.BuildInfo{Version:"v4.3.0"}\n'
      exit 0
    fi
    if [ "${MOCK_CLEANUP_FAIL:-false}" = true ] && [ "${1:-}" = uninstall ]; then
      exit 24
    fi
    exit 0
    ;;
  operator-sdk)
    if [ "${1:-}" = version ]; then
      printf 'operator-sdk version: "v1.42.3"\n'
      exit 0
    fi
    if [ "${MOCK_CLEANUP_FAIL:-false}" = true ] && [ "${1:-}" = cleanup ]; then
      exit 25
    fi
    exit 0
    ;;
  istioctl)
    exit 0
    ;;
esac
