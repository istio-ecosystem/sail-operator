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

# This script is used to update the example files in the documentation that are the resulting script to be executed by the runme tool.
# It will copy all the files from the docs/ respository to the TEST_DIR defined folder.
# It will exclude this files: sailoperator.io.md (Add more here if is needed)
# Beside that, after copy the files it will read each one and all the html commented bash code block and it will uncomment them. 
# This will allow us to hide from docs validation steps

# Set working directory to the script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Set the root directory to the parent directory of the script directory
ROOT_DIR="$(cd "$SCRIPT_DIR/../../.." && pwd)"
# Set the documentation directory to the docs/ directory
DOCS_DIR="$ROOT_DIR/docs"
# Set the files to exclude from the copy
EXCLUDE_FILES=(
  "sailoperator.io.md" "guidelines.md"
)
# Directory to store the output files
TEST_DIR="${TEST_DIR:-$(mktemp -d)}"
# Ensure the output directory exists
mkdir -p "$TEST_DIR"
echo "Using TEST_DIR directory to place the temporary docs files: $TEST_DIR"

# Validate that no tag value is used in more than one file in the current documentation
declare -A TAG_USAGE
DUPLICATE_FOUND=0

# Recursively find all markdown files in the output directory
# shellcheck disable=SC2094
while IFS= read -r file; do
  # Temp set to track unique tags in the current file
  declare -A TAGS_IN_CURRENT_FILE=()

  while IFS= read -r line; do
    # Match lines like: ```bash { name=... tag=example }
    if [[ "$line" =~ \`\`\`bash[[:space:]]*\{[[:space:]]*[^}]*tag=([a-zA-Z0-9._-]+)[^}]*\} ]]; then
      tag="${BASH_REMATCH[1]}"
      # Skip if this tag has already been seen in the same file
      if [[ ${TAGS_IN_CURRENT_FILE[$tag]+exists} ]]; then
        continue
      fi
      TAGS_IN_CURRENT_FILE["$tag"]=1

      # Now check if this tag has been used in another file
      if [[ ${TAG_USAGE[$tag]+exists} && "${TAG_USAGE[$tag]}" != "$file" ]]; then
        echo "Duplicate tag 'tag=$tag' found in:"
        echo "  - ${TAG_USAGE[$tag]}"
        echo "  - $file"
        DUPLICATE_FOUND=1
      else
        TAG_USAGE["$tag"]="$file"
      fi
    fi
  done < "$file"

  unset TAGS_IN_CURRENT_FILE
done < <(find "$DOCS_DIR" -type f -name "*.md")

# If duplicates were found, fail the script
if [[ "$DUPLICATE_FOUND" -eq 1 ]]; then
  echo "Duplicate 'tag=' values detected across multiple files. Please ensure all tags are unique per project."
  exit 1
fi

# Create first a list with all the files in the docs/ directory and subdirectories to be copied
# Exclude the files in the EXCLUDE_FILES array and include only the files that contains at least one pattern in their code blocks like this:
# bash { name=
FILES_TO_COPY=()

# Create regex pattern from EXCLUDE_FILES array
EXCLUDE_PATTERN="^($(IFS='|'; echo "${EXCLUDE_FILES[*]}"))$"

# Use find with maxdepth to match your original logic (top-level + 1 subdir level)
while read -r file; do
  basename=$(basename "$file")

  # Skip excluded files
  if [[ "$basename" =~ $EXCLUDE_PATTERN ]]; then
    continue
  fi

  # Add file if it contains the required pattern
  if grep -q "bash { name=" "$file"; then
    FILES_TO_COPY+=("$file")
  fi
done < <(find "$DOCS_DIR" -maxdepth 2 -type f -name "*.md")


echo "Files to copy:"
for file in "${FILES_TO_COPY[@]}"; do
  echo "$file"
done

# Copy the files to the output directory and add the suffix -runme.md to each file
for file in "${FILES_TO_COPY[@]}"; do
    # Get the base name of the file
    base_name=$(basename "$file")
    echo "Copying file: $file to $TEST_DIR/${base_name%.md}-runme.md"
    # Copy the file to the output directory and add the suffix -runme.md to each file
    cp "$file" "$TEST_DIR/${base_name%.md}-runme.md"
done


# Uncomment the html commented bash code block in the files
# Read the files in the test/documentation_tests/ directory
# The code block need to match this pattern:
# <!--
# ```bash
# <code>
# ```
# -->
# If any other pattern is found, it will be ignored. Add more patterns here if needed.
for file in "$TEST_DIR"/*.md; do
  perl -0777 -pe 's/<!--\s*(```bash[^\n]*\n)(.*?)(\n```)\s*-->/$1$2$3/gms' "$file" > "${file}.tmp" && mv "${file}.tmp" "$file"
done

# Finish
echo "All files copied and updated successfully."
