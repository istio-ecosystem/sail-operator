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
    # path to the controller implementation
    controllerPath=$1
    shift
    # space-separated list of file path patterns indicating which Helm charts to inspect
    chartPaths="$*"

    # Find kinds in charts
    # shellcheck disable=SC2086
    read -r -a chartKinds <<< "$(grep -rEo "^kind: ([A-Za-z0-9]+)" --no-filename $chartPaths | sed -e 's/^kind: //g' | sort | uniq | tr '\n' ' ')"
    echo "Kinds in charts: ${chartKinds[*]}"

    # Find watched kinds in istiorevision_controller.go
    read -r -a watchedKinds <<< "$(grep -Eo "(Owns|Watches)\\((.*)" "$controllerPath" | sed 's/.*&[^.]*\.\([^{}]*\).*/\1/' | sort | uniq | tr '\n' ' ')"
    echo "Watched kinds: ${watchedKinds[*]}"

    # Find ignored kinds in istiorevision_controller.go
    read -r -a ignoredKinds <<< "$(sed -n 's/.*\+lint-watches:ignore:\s*\(\w*\).*/\1/p' "$controllerPath" | sort | uniq | tr '\n' ' ')"
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
    if [[ ${#unwatched_kinds[@]} -gt 0 ]]; then
        printf "FAIL: The following kinds aren't watched in %s:\n" "$controllerPath"
        for kind in "${unwatched_kinds[@]}"; do
            printf "  - %s\n" "$kind"
        done
        exit 1
    else
        printf "%s watches all kinds found in Helm charts.\n" "$controllerPath"
    fi

    # Print unnecessary watches, if any
    if [[ ${#unneeded_watches[@]} -gt 0 ]]; then
        printf "FAIL: The following kinds are watched in %s, but are not present in the charts:\n" "$controllerPath"
        for kind in "${unneeded_watches[@]}"; do
            printf "  - %s\n" "$kind"
        done
        exit 1
    else
        printf "%s does not watch any kinds that aren't found in Helm charts.\n" "$controllerPath"
    fi
}

check_watches "./controllers/istiorevision/istiorevision_controller.go" "./resources/*/charts/istiod ./resources/*/charts/istiod-remote"
check_watches "./controllers/istiocni/istiocni_controller.go" "./resources/*/charts/cni"
