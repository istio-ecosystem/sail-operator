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

CHECKED_CRDS=0
ERRORS=0
WARNINGS=0

# Function to check if a CRD has any alpha/beta versions
is_alpha_beta_crd() {
    local crd_file="$1"
    # Look for version names that contain alpha or beta (e.g., "name: v1alpha1", "name: v1beta1")
    grep -q -E "name:[[:space:]]*v[0-9]+(alpha|beta)" "$crd_file"
}

[[ $# -gt 0 && ("$1" == "--help" || "$1" == "-h") ]] && {
    echo "Usage: $0 [--help|-h]"
    echo "Checks CRD backwards compatibility on release branches."
    exit 0
}

# Check for crd-schema-checker
if [[ -x "./bin/crd-schema-checker" ]]; then
    CRD_SCHEMA_CHECKER="./bin/crd-schema-checker"
elif command -v crd-schema-checker &> /dev/null; then
    CRD_SCHEMA_CHECKER="crd-schema-checker"
else
    echo "ERROR: crd-schema-checker not found. Run 'make crd-schema-checker'"
    exit 1
fi

current_branch=$(git rev-parse --abbrev-ref HEAD)

# prow uses shallow-clones
prow=false
if git rev-parse --is-shallow-repository | grep -q true; then
  git fetch --unshallow origin
  prow=true
fi

# Only run on release branches
[[ "$current_branch" =~ ^release-[0-9]+\.[0-9]+$ ]] || {
    echo "Not on a release branch (current: $current_branch). Attempting PREVIOUS_VERSION env var."
    VERSION=$(echo "${PREVIOUS_VERSION}" | cut -f1,2 -d'.')
    current_branch="release-${VERSION}"
    echo "Detected PREVIOUS_VERSION=${PREVIOUS_VERSION}. Using ${current_branch} as latest release branch."
}

# Extract version and find previous release
read -r current_major current_minor <<< "$(echo "$current_branch" | sed 's/release-//' | tr '.' ' ')"

previous_branch=$(git branch -a | grep -E 'release-[0-9]+\.[0-9]+$' | \
    sed 's/.*release-/release-/' | sort -V | uniq | \
    awk -v target="release-${current_major}.${current_minor}" '
    $0 == target { print prev; exit }
    { prev = $0 }
    ')

[[ -n "$previous_branch" ]] || {
    echo "ERROR: No previous release branch found"
    exit 1
}

echo "Checking CRD compatibility: $previous_branch -> $current_branch"

# Setup temp dirs
temp_dir=$(mktemp -d)
trap 'rm -rf $temp_dir' EXIT
previous_dir="$temp_dir/previous"
current_dir="$temp_dir/current"
mkdir -p "$previous_dir" "$current_dir"

extract_crds() {
    local branch="$1" output_dir="$2"
    if [[ "${prow}" == "true" ]]; then
      git fetch origin "${branch}"
    fi
    while IFS= read -r file; do
        [[ -z "$file" ]] && continue
        content=$(git show "$branch:bundle/manifests/$file" 2>/dev/null)
        [[ -z "$content" ]] && continue
        if [[ "$content" == *"CustomResourceDefinition"* ]]; then
            crd_name=$(echo "$content" | grep "name:" | head -1 | sed 's/.*name:[[:space:]]*//' | tr -d '"'"'"' ')
            if [[ -n "$crd_name" && "$crd_name" == *"sailoperator.io" ]]; then
                echo "$content" > "$output_dir/${crd_name}.yaml"
                echo "$output_dir/${crd_name}.yaml"
            fi
        fi
    done < <(git ls-tree --name-only -r "$branch:bundle/manifests" 2>/dev/null | grep -E '\.(yaml|yml)$')
}

echo "Extracting CRDs..."
mapfile -t previous_crds < <(extract_crds "$previous_branch" "$previous_dir")
mapfile -t current_crds < <(extract_crds "$current_branch" "$current_dir")

# Create lookup map for current CRDs
declare -A current_crd_map
for crd_file in "${current_crds[@]}"; do
    [[ -n "$crd_file" ]] && {
        crd_name=$(basename "$crd_file" .yaml)
        current_crd_map["$crd_name"]="$crd_file"
    }
done

echo "Comparing CRDs..."

# Compare existing CRDs
for previous_crd in "${previous_crds[@]}"; do
    [[ -z "$previous_crd" ]] && continue
    
    crd_name=$(basename "$previous_crd" .yaml)
    
    if [[ -n "${current_crd_map[$crd_name]:-}" ]]; then
        set +e
        output=$($CRD_SCHEMA_CHECKER check-manifests \
            --disabled-validators=NoBools,NoMaps \
            --existing-crd-filename="$previous_crd" \
            --new-crd-filename="${current_crd_map[$crd_name]}" 2>&1)
        exit_code=$?
        set -e
        
        if [[ $exit_code -eq 0 ]]; then
            echo "OK: $crd_name"
        else
            # Check if this is an alpha/beta CRD
            if is_alpha_beta_crd "$previous_crd"; then
                echo "WARNING: $crd_name (alpha/beta CRD)"
                if echo "$output" | grep -q "ERROR:"; then
                    echo "$output" | grep "ERROR:" | sed 's/ERROR:[[:space:]]*/  /'
                else
                    echo "  $output"
                fi
                WARNINGS=$((WARNINGS + 1))
            else
                echo "ERROR: $crd_name"
                if echo "$output" | grep -q "ERROR:"; then
                    echo "$output" | grep "ERROR:" | sed 's/ERROR:[[:space:]]*/  /'
                else
                    echo "  $output"
                fi
                ERRORS=$((ERRORS + 1))
            fi
        fi
        CHECKED_CRDS=$((CHECKED_CRDS + 1))
    else
        # Check if the removed CRD was alpha/beta
        if is_alpha_beta_crd "$previous_crd"; then
            echo "WARNING: CRD $crd_name was removed (alpha/beta CRD)"
            WARNINGS=$((WARNINGS + 1))
        else
            echo "ERROR: CRD $crd_name was removed"
            ERRORS=$((ERRORS + 1))
        fi
    fi
done

# Check for new CRDs
for current_crd in "${current_crds[@]}"; do
    [[ -z "$current_crd" ]] && continue
    
    crd_name=$(basename "$current_crd" .yaml)
    found=false
    
    for previous_crd in "${previous_crds[@]}"; do
        [[ -n "$previous_crd" ]] && [[ "$crd_name" == "$(basename "$previous_crd" .yaml)" ]] && {
            found=true
            break
        }
    done
    
    [[ "$found" == false ]] && {
        echo "INFO: New CRD added: $crd_name"
        WARNINGS=$((WARNINGS + 1))
    }
done

echo
echo "=== Results ==="
echo "Checked $CHECKED_CRDS CRDs: $ERRORS errors, $WARNINGS warnings"

if [[ $ERRORS -gt 0 ]]; then
    echo "FAILED: Breaking changes detected"
    exit 1
else
    echo "PASSED: No breaking changes"
fi 