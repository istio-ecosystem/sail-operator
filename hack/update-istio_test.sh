#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=hack/update-istio.sh
source "${SCRIPT_DIR}/update-istio.sh"
VERSIONS_YAML_PATH=$(mktemp)
trap 'rm -f "${VERSIONS_YAML_PATH}"' EXIT

curl() {
    case "$*" in
        *page=1)
            printf '[{"tag_name":"1.31.1"}'
            for release in {1..99}; do
                printf ',{"tag_name":"9.0.%s"}' "${release}"
            done
            printf ']\n'
            ;;
        *page=2)
            printf '[{"tag_name":"1.31.10"}]\n'
            ;;
        *)
            echo "Unexpected releases URL: $*" >&2
            return 1
            ;;
    esac
}

releases=$(get_all_releases)
release_count=$(wc -l <<< "${releases}")
[[ "${release_count}" -eq 101 ]]
[[ "$(get_latest_release_for_minor "${releases}" 1 31)" == "1.31.10" ]]
[[ -z "$(get_latest_release_for_minor "${releases}" 1 29)" ]]

printf '%s\n' 'versions:' '  - name: "v1.29.5"' '    version: "1.29.5"' > "${VERSIONS_YAML_PATH}"
update_stable

echo "update-istio release lookup tests passed"