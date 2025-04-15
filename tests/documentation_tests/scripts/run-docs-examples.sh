#!/bin/bash

# Copyright Istio Authors

# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -eu -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../../.." && pwd)"
TEST_DIR="$ROOT_DIR/tests/documentation_tests"

export KIND_CLUSTER_NAME="docs-automation"
export IP_FAMILY="ipv4"
export ISTIOCTL="${ROOT_DIR}/bin/istioctl"
export IMAGE_BASE="sail-operator"
export TAG="latest"
export LOCAL_REGISTRY="localhost:5000"
export OCP=false

# Validate that istioctl is installed
if ! command -v istioctl &> /dev/null; then
  echo "istioctl could not be found. Please install it."
  exit 1
fi

# Validate that kubectl is installed
if ! command -v kubectl &> /dev/null; then
  echo "kubectl could not be found. Please install it."
  exit 1
fi

# Discover .md files with bash tags
FILES_TO_CHECK=()
for file in "$TEST_DIR"/*.md; do
  if grep -q "bash { name=" "$file"; then
    FILES_TO_CHECK+=("$file")
  fi
done

# Build list of file-tag pairs
TAGS_LIST=()
for file in "${FILES_TO_CHECK[@]}"; do
  TAGS=$(grep -oP 'bash { name=[^ ]+ tag=\K[^}]+(?=})' "$file" | sort -u)
  for tag in $TAGS; do
    TAGS_LIST+=("$file -t $tag")
  done
done

echo "Tags list:"
for tag in "${TAGS_LIST[@]}"; do
  echo "$tag"
done

# Run each test in its own KIND cluster
for tag in "${TAGS_LIST[@]}"; do
  (
    echo "Setting up cluster for: $tag"

    # Source setup and build scripts to preserve trap and env
    source "${ROOT_DIR}/tests/e2e/setup/setup-kind.sh"

    source "${ROOT_DIR}/tests/e2e/setup/build-and-push-operator.sh"
    build_and_push_operator_image

    # Run the actual doc test
    FILE=$(echo "$tag" | cut -d' ' -f1)
    TAG=$(echo "$tag" | cut -d' ' -f3-)

    echo "Running: runme run --filename $FILE -t $TAG --skip-prompts"
    runme run --filename "$FILE" -t "$TAG" --skip-prompts
  )
done
