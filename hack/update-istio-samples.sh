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

# Base directory containing the sample application folders
SAMPLES_DIR="tests/e2e/samples"
# Quay repository where images will be copied
HUB="${HUB:-quay.io/sail-dev}"

# Array to store unique images that need to be copied
declare -a IMAGES_TO_COPY

# Iterate over all subdirectories in the samples directory
for dir in "$SAMPLES_DIR"/*/; do
    # Skip if it's not a directory
    [[ ! -d "$dir" ]] && continue

    name=$(basename "$dir")
    kustomization_file="${dir}/kustomization.yaml"

    echo "Processing sample: $name"

    # Check if the kustomization.yaml file exists
    if [[ ! -f "$kustomization_file" ]]; then
        echo "kustomization.yaml not found in $dir. Skipping."
        continue
    fi

    # Extract the upstream URL from the 'resources' field
    url=$(grep -oE '^\s*-\s*https://[^[:space:]]+' "$kustomization_file" | sed 's/^\s*-\s*//')

    if [[ -z "$url" ]]; then
        echo "No upstream URL found in $kustomization_file. Skipping."
        continue
    fi

    echo "Reading images from upstream URL: $url"

    # Read the URL content in memory and extract the image names
    while read -r original; do
        echo "  Found image to copy: $original"
        # Skip curlimages/curl
        [[ "$original" == curlimages/curl* ]] && continue
        IMAGES_TO_COPY["$original"]=1
    done < <(curl -fsSL "$url" | grep -oE 'image:\s*docker.io/(istio|mccutchen)/[^[:space:]]+' | sed -E 's/image:\s*//')
done

# Copy images from Docker Hub to Quay if any were found
if [ ${#IMAGES_TO_COPY[@]} -eq 0 ]; then
    echo "No images found to copy."
    exit 0
fi

echo "Copying images to $QUAY_HUB using crane..."
# Requirements: crane must be installed, and you must be logged into quay.io with write permissions.
for src_image in "${IMAGES_TO_COPY[@]}"; do
    # Extract the image name and tag (e.g., from 'docker.io/istio/foo:tag' to 'foo:tag')
    image_name_tag=$(echo "$src_image" | sed -E 's|docker.io/[^/]+/||')
    dst_image="${QUAY_HUB}/${image_name_tag}"

    echo "Copying $src_image -> $dst_image"
    crane copy "$src_image" "$dst_image"
done

echo "All required images have been copied to $QUAY_HUB."