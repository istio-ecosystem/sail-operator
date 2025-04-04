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

# To be able to run docs we need to get all the existing tag inside the documentation that is located in the path `tests/documentation_tests/`
# This is because one md file may contain multiple tags and we need to be able to run all of them separately to avoid conflicts.

# Set working directory to the script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Set the root directory to the parent directory of the script directory
ROOT_DIR="$(cd "$SCRIPT_DIR/../../.." && pwd)"
# Set the test directory to the test/documentation_tests/ directory
TEST_DIR="$ROOT_DIR/tests/documentation_tests"

# Get the list of files in the tests/documentation_tests/ directory and avoid checking subdirectories
FILES_TO_CHECK=()
for file in "$TEST_DIR"/*; do
    if [[ "$file" == *.md ]]; then
      # Only add if the file contains the pattern 'bash { name='
      if grep -q "bash { name=" "$file"; then
        FILES_TO_CHECK+=("$file")
      fi
    fi
done

# Check the tag inside each file in FILES_TO_CHECK and create a list of tags by file
# The expected format will be like this:
# bash { name=step-name tag=tag-name}
TAGS_LIST=()
for file in "${FILES_TO_CHECK[@]}"; do
    # Get the tags from the file
    TAGS=$(grep -oP 'bash { name=[^ ]+ tag=\K[^}]+(?=})' "$file")
    # Remove duplicates
    TAGS=$(echo "$TAGS" | tr ' ' '\n' | sort -u | tr '\n' ' ')
    # Add the tags to the list
    TAGS_LIST+=("$file -t $TAGS")
done

echo "Tags list:"
for tag in "${TAGS_LIST[@]}"; do
    echo "$tag"
done

# Run the command for each tag in TAGS_LIST
for tag in "${TAGS_LIST[@]}"; do
    # Extract the file and tag from the string
    FILE=$(echo "$tag" | cut -d' ' -f1)
    TAG=$(echo "$tag" | cut -d' ' -f3-)

    # Run the command with the file and tag
    echo "Running command for $FILE with tag $TAG"
    runme run --filename $FILE -t $TAG --skip-prompts
done




