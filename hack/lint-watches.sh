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

# Validates that the static watch lists in pkg/watches/ match the resource
# kinds produced by the Helm charts in resources/. Each watch list file
# contains typed object prototypes (e.g. &corev1.ConfigMap{}) whose Go type
# name matches the Kubernetes Kind.

check_watchlist() {
    # path to the watch list Go file (e.g. pkg/watches/istiod.go)
    watchlistPath=$1
    shift
    # space-separated list of file path patterns indicating which Helm charts to inspect
    chartPaths="$*"

    echo "--- Checking $watchlistPath ---"

    # Find kinds in charts
    # shellcheck disable=SC2086
    read -r -a chartKinds <<< "$(grep -rEo "^kind: ([A-Za-z0-9]+)" --no-filename $chartPaths | sed -e 's/^kind: //g' | sort | uniq | tr '\n' ' ')"
    echo "Kinds in charts: ${chartKinds[*]}"

    # Extract kinds from the watch list by parsing typed object prototypes (e.g. &corev1.ConfigMap{})
    read -r -a watchlistKinds <<< "$(grep -Eo '&\w+\.\w+\{\}' "$watchlistPath" | sed 's/&[a-zA-Z0-9]*\.\([A-Za-z0-9]*\){}/\1/' | sort | uniq | tr '\n' ' ')"
    echo "Watch list kinds: ${watchlistKinds[*]}"

    # Find ignored kinds
    local ignoredKinds=()
    if grep -q '+lint-watches:ignore' "$watchlistPath"; then
        read -r -a ignoredKinds <<< "$(sed -n 's/.*\+lint-watches:ignore:\s*\(\w*\).*/\1/p' "$watchlistPath" | sort | uniq | tr '\n' ' ')"
        echo "Ignored kinds: ${ignoredKinds[*]}"
    fi

    # Check for chart kinds missing from the watch list
    local missing_kinds=()
    for kind in "${chartKinds[@]}"; do
        # shellcheck disable=SC2076
        if [[ ! " ${watchlistKinds[*]} ${ignoredKinds[*]} " =~ " $kind " ]]; then
            missing_kinds+=("$kind")
        fi
    done

    # Check for watch list entries not in charts
    local extra_kinds=()
    for kind in "${watchlistKinds[@]}"; do
        # shellcheck disable=SC2076
        if [[ ! " ${chartKinds[*]} ${ignoredKinds[*]} " =~ " $kind " ]]; then
            extra_kinds+=("$kind")
        fi
    done

    if [[ ${#missing_kinds[@]} -gt 0 ]]; then
        printf "FAIL: The following chart kinds are missing from %s:\n" "$watchlistPath"
        for kind in "${missing_kinds[@]}"; do
            printf "  - %s\n" "$kind"
        done
        exit 1
    else
        printf "%s covers all kinds found in Helm charts.\n" "$watchlistPath"
    fi

    if [[ ${#extra_kinds[@]} -gt 0 ]]; then
        printf "FAIL: The following kinds in %s are not present in the charts:\n" "$watchlistPath"
        for kind in "${extra_kinds[@]}"; do
            printf "  - %s\n" "$kind"
        done
        exit 1
    else
        printf "%s does not contain kinds that aren't found in Helm charts.\n" "$watchlistPath"
    fi
    echo ""
}

check_watchlist "./pkg/watches/istiod.go" "./resources/*/charts/istiod ./resources/*/charts/base"
check_watchlist "./pkg/watches/cni.go" "./resources/*/charts/cni"
check_watchlist "./pkg/watches/ztunnel.go" "./resources/*/charts/ztunnel"
