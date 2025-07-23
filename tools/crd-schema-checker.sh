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

# CRD compatibility checker for release branches.
# Compares CRDs between the current release branch and the previous one to catch breaking changes.
# Uses OpenShift's crd-schema-checker to detect issues like removed fields or stricter validation.
# Run 'make crd-schema-checker' first to install the dependency, then 'make lint-crds' to check.

set -euo pipefail

CHECKED_CRDS=0 ERRORS=0 STABLE_ERRORS=0 WARNINGS=0 INFOS=0

# TODO: fix the SSA tags on lists and enable the validator
DISABLED_VALIDATORS="NoBools,NoMaps,ListsMustHaveSSATags"

# Get the latest version from a CRD
getLatestCRDVersion() {
    command -v yq &>/dev/null || { echo "unknown"; return; }
    yq eval '.spec.versions[-1].name' "$1" 2>/dev/null || echo "unknown"
}

# Check if version is stable (not alpha/beta)
isStableVersion() {
    [[ "$1" =~ ^v[0-9]+(\.[0-9]+)*$ ]]
}

# Output result with version info
output_result() {
    local crd_name="$1" version="$2" output="$3"
    local errors=0 warnings=0 infos=0
    echo "$crd_name ($version)"
    if [ -n "${output}" ]; then
        while read -r line; do
            if echo "${line}" | grep -iq "ERROR:"; then
                errors=$((errors + 1))
            elif echo "${line}" | grep -iq "Warning:"; then
                warnings=$((warnings + 1))
            elif echo "${line}" | grep -iq "info:"; then
                infos=$((infos + 1))
            fi
            echo " - $line"
        done <<< "$output"
    fi
    echo "--> ${errors} errors, ${warnings} warnings, ${infos} infos"
    if isStableVersion "${version}"; then
        STABLE_ERRORS=$((STABLE_ERRORS + errors))
    fi
    ERRORS=$((ERRORS + errors))
    WARNINGS=$((WARNINGS + warnings))
    INFOS=$((INFOS + infos))
}

if [[ -x "$(pwd)/bin/crd-schema-checker" ]]; then
    CRD_SCHEMA_CHECKER="$(pwd)/bin/crd-schema-checker"
elif command -v crd-schema-checker &>/dev/null; then
    CRD_SCHEMA_CHECKER="crd-schema-checker"
else
    echo "ERROR: crd-schema-checker not found. Run 'make crd-schema-checker'"
    exit 1
fi

repo_url="https://github.com/istio-ecosystem/sail-operator.git"
[[ -n "${PROW_JOB_ID:-}" && -n "${REPO_OWNER:-}" && -n "${REPO_NAME:-}" ]] && 
    repo_url="https://github.com/${REPO_OWNER}/${REPO_NAME}.git"

temp_dir=$(mktemp -d)
trap 'rm -rf "$temp_dir"' EXIT
local_repo_path=$(pwd)
git clone "$repo_url" "$temp_dir/repo"

# Determine branches to compare
current_branch=$(git rev-parse --abbrev-ref HEAD)

pushd "$temp_dir/repo" >/dev/null

git fetch origin '+refs/heads/release-*:refs/remotes/origin/release-*' || true

if [[ "$current_branch" =~ ^release-[0-9]+\.[0-9]+$ ]]; then
    # we're on a release branch. find the previous release branch
    previous_branch=$(git branch -r | grep -E 'origin/release-[0-9]+\.[0-9]+$' | 
        sed 's|.*origin/||' | sort -V | 
        awk -v target="$current_branch" '$0 == target { print prev; exit } { prev = $0 }')
elif [[ -n "${PREVIOUS_VERSION:-}" ]]; then
    previous_branch="release-$(echo "${PREVIOUS_VERSION}" | cut -f1,2 -d'.')"
else
    echo "Not on a release branch and PREVIOUS_VERSION not set. Skipping."
    exit 0
fi

if [[ -z "$previous_branch" ]]; then
    echo "ERROR: No previous release branch found for $current_branch"
    exit 1
fi

git checkout "$previous_branch"
popd >/dev/null

echo "Checking CRD compatibility: $previous_branch -> $current_branch"

# Extract CRDs from repo_dir $1's, copy to output_dir $2
extract_crds() {
    local repo_dir="$1" output_dir="$2"
    mkdir -p "$output_dir"
    pushd "$repo_dir" > /dev/null
    local files
    mapfile -t files < <(ls bundle/manifests/sailoperator.io*.yaml)
    
    for filepath in "${files[@]}"; do
        [[ -z "$filepath" ]] && continue
        content=$(cat "$filepath")
        if [[ "$content" == *"kind: CustomResourceDefinition"* ]]; then
            file=$(basename "$filepath")
            echo "$content" > "$output_dir/$file"
            echo "$file"
        fi
    done
    popd > /dev/null
}
mapfile -t previous_crds < <(extract_crds "$temp_dir/repo" "$temp_dir/prev")
echo Found "${#previous_crds[@]}" CRDs in "$previous_branch": "${previous_crds[@]}"
mapfile -t current_crds < <(extract_crds "$local_repo_path" "$temp_dir/curr")
echo Found "${#current_crds[@]}" CRDs in "$current_branch": "${current_crds[@]}"

# Create lookup maps
declare -A current_crd_map previous_crd_map
for crd in "${current_crds[@]}"; do
    current_crd_map["$crd"]="$temp_dir/curr/${crd}"
done
for crd in "${previous_crds[@]}"; do
    previous_crd_map["$crd"]="$temp_dir/prev/${crd}"
done

echo "Comparing CRDs..."

# Check existing CRDs for breaking changes
for crd in "${previous_crds[@]}"; do
    if [[ -n "${current_crd_map[$crd]:-}" ]]; then
        set +e
        output=$($CRD_SCHEMA_CHECKER check-manifests \
            --disabled-validators=${DISABLED_VALIDATORS} \
            --existing-crd-filename="${previous_crd_map[$crd]}" \
            --new-crd-filename="${current_crd_map[$crd]}" 2>&1)
        set -e
        
        version=$(getLatestCRDVersion "${current_crd_map[$crd]}")
        output_result "${crd}" "${version}" "${output}"
        CHECKED_CRDS=$((CHECKED_CRDS + 1))
    else
        # CRD was removed
        version=$(getLatestCRDVersion "${previous_crd_map[$crd]}")
        if ! isStableVersion "$version"; then
            echo "WARNING: CRD $crd was removed ($version)"
            WARNINGS=$((WARNINGS + 1))
        else
            echo "ERROR: CRD $crd was removed (${version})"
            ERRORS=$((ERRORS + 1))
        fi
    fi
done

# Check for new CRDs
for crd in "${current_crds[@]}"; do
    [[ -n "${previous_crd_map[$crd]:-}" ]] && continue
    echo "INFO: New CRD added: $crd"
    set +e
    output=$($CRD_SCHEMA_CHECKER check-manifests \
        --disabled-validators=${DISABLED_VALIDATORS} \
        --new-crd-filename="${current_crd_map[$crd]}" 2>&1)
    set -e
    version=$(getLatestCRDVersion "${current_crd_map[$crd]}")
    output_result "${crd}" "${version}" "${output}"
    ((CHECKED_CRDS++))
done

echo
echo "=== Results ==="
echo "Checked $CHECKED_CRDS CRDs: $ERRORS errors ($STABLE_ERRORS errors in stable APIs), $WARNINGS warnings, $INFOS infos"

if [[ $STABLE_ERRORS -gt 0 ]]; then
    echo "FAILED: Breaking changes detected"
    exit 1
else
    echo "PASSED: No breaking changes"
fi 
