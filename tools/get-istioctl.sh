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

# Script to download the istioctl binary either from a direct URL or by extracting it from a container image in a registry.
# Supports downloading from a specified URL or locating the latest image tag matching a pattern in a container registry.

usage() {
  echo "Usage: $0 [--from-url] <url> | [--from-container-image] <namespace/repository> <tag_pattern>"
  echo ""
  echo "Download istioctl from a URL or extract it from a container image matching a tag pattern"
  echo ""
  echo "Options:"
  echo "  --from-url              Download istioctl binary from a URL"
  echo "  --from-container-image  Extract istioctl binary from container image"
  echo ""
  echo "Arguments:"
  echo "  url                   URL to download istioctl binary from"
  echo "  namespace/repository  Container repository in format namespace/repository"
  echo "  tag pattern           Specific tag pattern to query"
  echo ""
  echo "Examples:"
  echo "  $0  "
  echo "  $0 --from-url https://developers.redhat.com/content-gateway/file/pub/cgw/servicemesh/1.24.6/istioctl-1.24.6-linux-amd64.tar.gz"
  echo "  $0 --from-container-image quay.io redhat-user-workloads/service-mesh-tenant/istioctl-3-1 on-push"
  exit 1
}

ISTIOCTL_VERSION="${ISTIOCTL_VERSION:-"1.26.2"}"
ISTIOCTL_URL="${ISTIOCTL_URL:-}"
CONTAINER_CLI="${CONTAINER_CLI:-docker}"
TARGET_OS="${TARGET_OS:-"$(go env GOOS)"}"
TARGET_ARCH="${TARGET_ARCH:-"$(go env GOARCH)"}"
LOCALBIN="${LOCALBIN:-"/tmp/bin"}"

# get_from_url downloads and extracts the istioctl binary from a provided URL.
get_from_url() {
  local url="$1"
  
  echo "Fetching istioctl from $url"
  curl -fsL "$url" -o /tmp/istioctl.tar.gz || {
    echo "Download failed! Please check the URL and ISTIOCTL_VERSION."
    exit 1
  }
  tar -xzf /tmp/istioctl.tar.gz -C /tmp || {
    echo "Extraction failed!"
    exit 1
  }

  mkdir -p "${LOCALBIN}"
  mv /tmp/"$(tar tf /tmp/istioctl.tar.gz | tail -n 1)" "${LOCALBIN}"
  trap "rm -f /tmp/istioctl.tar.gz" EXIT
  echo "istioctl extracted to ${LOCALBIN}"
}

# get_from_container_image extracts the istioctl binary from a container image by finding the latest tag matching a given pattern in a container registry
get_from_container_image() {
  local registry="${1:?"Registry is required"}"
  local repository="${2:?"Repository is required"}"
  local tag_pattern="${3:?"Tag pattern is required"}"

  local base_url="https://$registry/api/v1/repository"
  local url="$base_url/$repository/tag/?onlyActiveTags=true&limit=10000"
  local response
  local latest_tag

  # Validate repository format (should be namespace/anything)
  if [[ ! "$repository" =~ ^[^/]+/.+$ ]]; then
    echo "Error: Repository must be in format 'namespace/repository'"
    usage
  fi

  echo "curl -s -f $url"
  if ! response=$(curl -s -f "$url" 2>/dev/null); then
    echo "Error: Failed to fetch information for tag pattern '$tag_pattern' in repository '$registry/$repository'"
    echo "Please check that the repository and tag pattern exist and are publicly accessible"
    exit 1
  fi

  # Check if jq is available for JSON parsing
  if command -v jq >/dev/null 2>&1; then
    latest_tag=$(echo "$response" | jq -r '.tags | sort_by(.last_modified) | reverse | .[] | select(.name | test("'"$tag_pattern"'")?) | .name' | head -n 1)
    
    if [ -n "$latest_tag" ]; then
      echo "Extracting istioctl from container image $registry/$repository:$latest_tag"
      
      if ${CONTAINER_CLI} ps -a --format '{{.Names}}' | grep -q "^${latest_tag}\$"; then
        delete_container_image "$latest_tag"
      fi

      ${CONTAINER_CLI} run -d --name "$latest_tag" "$registry/$repository:$latest_tag" /bin/bash && \
      ${CONTAINER_CLI} cp "$latest_tag":/releases/istioctl-"$ISTIOCTL_VERSION"-"$TARGET_OS"-"$TARGET_ARCH".tar.gz /tmp/istioctl.tar.gz
      
      delete_container_image "$latest_tag"

      tar -xzf /tmp/istioctl.tar.gz -C /tmp || { \
        echo "Extraction failed!"; \
        exit 1; \
      }

      mv /tmp/"$(tar tf /tmp/istioctl.tar.gz | tail -n 1)" "${LOCALBIN}"
      echo "istioctl extracted to ${LOCALBIN}"

    else
      echo "Error: No tag matching pattern '$tag_pattern' found in repository '$registry/$repository'"
      exit 1
    fi
  else
    echo "Note: Install 'jq' for better formatted output"
    echo "$response"
  fi
}

delete_container_image() {
  local container_name="${1}"
  echo "Deleting container image $container_name"
  ${CONTAINER_CLI} stop "$container_name" >/dev/null 2>&1 && ${CONTAINER_CLI} rm -f "$container_name" >/dev/null 2>&1
}

# Parse command line arguments
FROM_URL=false
FROM_CONTAINER_IMAGE=false
ARGS=()

while [[ $# -gt 0 ]]; do
  case $1 in
    --from-url)
      FROM_URL=true
      shift
      ;;

    --from-container-image)
      FROM_CONTAINER_IMAGE=true
      shift
      ;;
    -h|--help)
      usage
      ;;
    *)
      ARGS+=("$1")
      shift
      ;;
  esac
done

set -- "${ARGS[@]}"

# Call the appropriate function based on the flag
if [ "$FROM_CONTAINER_IMAGE" = true ]; then
  ISTIOCTL_CONTAINER_IMAGE_REGISTRY="${1:?"ISTIOCTL_CONTAINER_IMAGE_REGISTRY is required"}"
  ISTIOCTL_CONTAINER_IMAGE_REPOSITORY="${2:?"ISTIOCTL_CONTAINER_IMAGE_REPOSITORY is required"}"
  ISTIOCTL_CONTAINER_IMAGE_TAG_PATTERN="${3:?"ISTIOCTL_CONTAINER_IMAGE_TAG_PATTERN is required"}"
  get_from_container_image "$ISTIOCTL_CONTAINER_IMAGE_REGISTRY" "$ISTIOCTL_CONTAINER_IMAGE_REPOSITORY" "$ISTIOCTL_CONTAINER_IMAGE_TAG_PATTERN"
fi

if [ "$FROM_URL" = true ]; then
  ISTIOCTL_URL="$1"
  get_from_url "$ISTIOCTL_URL"
fi