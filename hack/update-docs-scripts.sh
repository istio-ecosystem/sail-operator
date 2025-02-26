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

# Important information regarding this script:
# This scripts is used to automatically generate bash script from documentation files under the folder /docs
# The generated bash script will be used to test the documentation steps examples to monitoring basically changes over the API that could break the documentation
# Also check if the documentation is up to date with the code
# Resulting script are going to be saved under /docs/scripts
# The script will scan all the files under /docs and generate a bash script for topics with the tag "<!-- generate-docs-test-init  script_name -->" and "<!-- generate-docs-test-end -->" inside the topics that wanted to be tested
# Each topic inside each document will follow the naming pattern: <document_name>_<topic_name>.sh
# Please check documentation guidelines for more information about how to use the tag "generate-docs-test" and How to write documentation

set -euo pipefail

# Function to get all documentation files.
function get_all_docs() {
    # Log message to stderr to avoid interfering with command output.
    echo "Getting all the documentation files" >&2
    find docs -type f -name "*.md"
}

function get_topic_with_tag_in_doc() {
    # Get all line numbers that contain the comment tag "generate-docs-test-init" in the document.
    # "$1": documentation file.
    grep -n -E "generate-docs-test-init" "$1" | cut -d: -f1 || true
}

function get_topic_name() {
    # Extract and sanitize the topic name from the topic line.
    # Expected format: <!-- generate-docs-test-init Installing_the_operator_using_the_cli-->
    # "$1": file, "$2": line number.
    sed -n "${2}p" "$1" \
      | sed -E 's/<!--[[:space:]]*generate-docs-test-init[[:space:]]+(.+)[[:space:]]*-->/\1/' \
      | sed -e 's/ /_/g'
}

function get_topic_end() {
    # Get the first line number after $2 that contains the end tag.
    # Expected end tag format: <!-- generate-docs-test-end -->
    # "$1": file, "$2": starting line number.
    local start_line="$2"
    grep -n "generate-docs-test-end" "$1" \
      | cut -d: -f1 \
      | awk -v start="$start_line" '$1 > start {print; exit}'
}

function get_topic_content() {
    # Extract the topic content from the topic line.
    # "$1": file, "$2": line number.
    sed -n "${2}p" "$1" \
      | sed -E 's/<!--[[:space:]]*generate-docs-test-init[[:space:]]+(.+)[[:space:]]*-->/\1/'
}


function get_code_block() {
    # Extract the code block from the topic.
    # "$1": file, "$2": start line, "$3": end line.
    sed -n "${2},${3}p" "$1" \
      | sed -e 's/^/# /' \
      | sed -e '/```bash/,/```/ s/^# //' \
      | sed -e '/```sh/,/```/ s/^# //' \
      | sed -e '/```shell/,/```/ s/^# //' \
      | sed -e 's/```bash//g' \
      | sed -e 's/```sh//g' \
      | sed -e 's/```shell//g' \
      | sed -e 's/```//g' \
      | sed -e 's/^ *\$ //' \
      | sed -e 's/^ *//'
}

function generate_script() {
    # Generate the script file.
    # "$1": documentation file, "$2": topic name, "$3": code block.
    local script_dir="docs/scripts"
    mkdir -p "$script_dir"
    local script_file="$script_dir/$2.sh"

    {
      echo "#!/bin/bash"
      echo ""
      echo "# This script was generated from the documentation file $1"
      echo "# Please check the documentation file for more information"
      echo ""
      echo "$3"
    } > "$script_file"
    chmod +x "$script_file"
}

# Process all documentation files.
get_all_docs | while IFS= read -r doc; do
    get_topic_with_tag_in_doc "$doc" | while IFS= read -r topic; do
        topicName=$(get_topic_name "$doc" "$topic")
        topicContent=$(get_topic_content "$doc" "$topic")
        topicEnd=$(get_topic_end "$doc" "$topic")
        if [ -z "$topicEnd" ]; then
            echo "Warning: No end tag found for topic starting at line $topic in $doc" >&2
            continue
        fi
        codeBlock=$(get_code_block "$doc" "$topic" "$topicEnd")
        generate_script "$doc" "$topicName" "$codeBlock"
    done
done

echo "All the scripts were generated successfully"
echo "Please check the folder docs/scripts for the generated scripts"
echo "Please check the documentation file for more information about the topics"
echo "End of the script"