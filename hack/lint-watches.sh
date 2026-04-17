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

check_watches() {
    # colon-separated paths to source files (controller + watch list files)
    IFS=':' read -r -a sourcePaths <<< "$1"
    shift
    # space-separated list of file path patterns indicating which Helm charts to inspect
    chartPaths="$*"

    # Find kinds in charts
    # shellcheck disable=SC2086
    read -r -a chartKinds <<< "$(grep -rEo "^kind: ([A-Za-z0-9]+)" --no-filename $chartPaths | sed -e 's/^kind: //g' | sort | uniq | tr '\n' ' ')"
    echo "Kinds in charts: ${chartKinds[*]}"

    # Find watched kinds from Watches()/Owns() calls and Object: declarations in watch lists
    local watchedStr=""
    for sp in "${sourcePaths[@]}"; do
        watchedStr+=" $(grep -Eo '(Owns|Watches)\(&(.*)' "$sp" 2>/dev/null | sed 's/.*&[^.]*\.\([^{}]*\).*/\1/' || true)"
        watchedStr+=" $(grep -Eo 'Object:\s*&[^.]*\.[A-Za-z0-9]+' "$sp" 2>/dev/null | sed 's/.*&[^.]*\.\(.*\)/\1/' || true)"
    done
    read -r -a watchedKinds <<< "$(echo "$watchedStr" | tr ' ' '\n' | grep -v '^$' | sort | uniq | tr '\n' ' ')"
    echo "Watched kinds: ${watchedKinds[*]}"

    # Find ignored kinds from all source files. Starting list is all operator CRDs
    local ignoredStr="Istio IstioCNI ZTunnel IstioRevision IstioRevisionTag"
    for sp in "${sourcePaths[@]}"; do
        ignoredStr+=" $(sed -n 's/.*\+lint-watches:ignore:\s*\(\w*\).*/\1/p' "$sp" 2>/dev/null || true)"
    done
    read -r -a ignoredKinds <<< "$(echo "$ignoredStr" | tr ' ' '\n' | grep -v '^$' | sort | uniq | tr '\n' ' ')"
    echo "Ignored kinds: ${ignoredKinds[*]}"

    # Check for missing and unnecessary watches
    local unwatched_kinds=()
    for kind in "${chartKinds[@]}"; do
        # shellcheck disable=SC2076
        if [[ ! " ${watchedKinds[*]} ${ignoredKinds[*]} " =~ " $kind " ]]; then
            unwatched_kinds+=("$kind")
        fi
    done

    local unneeded_watches=()
    for kind in "${watchedKinds[@]}"; do
        # shellcheck disable=SC2076
        if [[ ! " ${chartKinds[*]} ${ignoredKinds[*]} " =~ " $kind " ]]; then
            unneeded_watches+=("$kind")
        fi
    done

    # Print unwatched kinds, if any
    local label="${sourcePaths[*]}"
    if [[ ${#unwatched_kinds[@]} -gt 0 ]]; then
        printf "FAIL: The following kinds aren't watched in %s:\n" "$label"
        for kind in "${unwatched_kinds[@]}"; do
            printf "  - %s\n" "$kind"
        done
        exit 1
    else
        printf "%s watches all kinds found in Helm charts.\n" "$label"
    fi

    # Print unnecessary watches, if any
    if [[ ${#unneeded_watches[@]} -gt 0 ]]; then
        printf "FAIL: The following kinds are watched in %s, but are not present in the charts:\n" "$label"
        for kind in "${unneeded_watches[@]}"; do
            printf "  - %s\n" "$kind"
        done
        exit 1
    else
        printf "%s does not watch any kinds that aren't found in Helm charts.\n" "$label"
    fi
}

check_watches "./controllers/istiorevision/istiorevision_controller.go:./pkg/watches/istiod.go" "./resources/*/charts/istiod ./resources/*/charts/istiod-remote ./resources/*/charts/base"
check_watches "./controllers/istiocni/istiocni_controller.go:./pkg/watches/cni.go" "./resources/*/charts/cni"
check_watches "./controllers/ztunnel/ztunnel_controller.go:./pkg/watches/ztunnel.go" "./resources/*/charts/ztunnel"
