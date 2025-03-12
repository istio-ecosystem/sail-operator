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
    
    # Find all the md files under the docs folder but exclude the one located under the docs/guidelines folder.
    find docs -type f -name "*.md" -not -path "docs/guidelines/*"
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
    SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

    {
      echo "#!/bin/bash"
      echo ""
      echo "# This script was generated from the documentation file $1"
      echo "# Please check the documentation file for more information"
      echo ""
      # Dynamically calculate SCRIPT_DIR at runtime
      echo 'SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"'
      # Source the update-docs-scripts.sh relative to the runtime script directory
      echo 'source "$SCRIPT_DIR/../../hack/update-docs-scripts.sh"'
      echo "# Setup the cluster based in the current env variables"
      echo "setup_env"
      echo "$3"
    } > "$script_file"
    chmod +x "$script_file"
    echo "Generated script: $script_file" >&2
}

# List of prebuilt functions available for insertion.
# Add here any function that is going to be used in more than one script.
PREBUILT_FUNCTIONS=("install_sail_operator" "uninstall_operator" "setup_env" "cleanup_env")

# Define the regex pattern in a variable.
regex='<!--[[:space:]]*prebuilt[[:space:]]+([a-zA-Z0-9_]+)[[:space:]]*-->'

# Process the code block replacing any prebuilt tag with the function name (if available).
function process_prebuilt_tags() {
    local code_block="$1"
    local processed_code=""
    while IFS= read -r line; do
        if [[ $line =~ $regex ]]; then
            local func_name="${BASH_REMATCH[1]}"
            # Check if the extracted function name is in our list.
            if [[ " ${PREBUILT_FUNCTIONS[*]} " =~ " ${func_name} " ]]; then
                processed_code+="${func_name}"$'\n'
            else
                echo "Warning: Prebuilt function '${func_name}' not found in allowed list." >&2
                processed_code+="$line"$'\n'
            fi
        else
            processed_code+="$line"$'\n'
        fi
    done <<< "$code_block"
    echo "$processed_code"
}

##### Pre-build functions to use in the scripts, insert here any function that is going to be used in more than one script
# The function can be inserted in the generated script by adding the tag <!-- prebuilt function_name --> in the documentation file

# Install the Sail Operator
function install_sail_operator() {
    # Build and push operator image
    build_and_push_operator_image

    # Removed this later, only for debugging
    echo "Building and pushing the operator image"
    echo "Hub is set to: ${HUB}"
    echo "Image base is set to: ${IMAGE_BASE}"
    echo "Tag is set to: ${TAG}"
    make -e IMAGE="${HUB}/${IMAGE_BASE}:${TAG}" deploy
    # We need also to wait until the operator pod is running
}

# Uninstall operator
function uninstall_operator() {
    # Running make undeploy will do the work for us
    make undeploy

    # We need to wait for the operator to be uninstalled
    # TODO: Add a wait function to check if the operator is uninstalled
}

# setup_env checks if we are running on OpenShift to setup the environment for the tests
# if OPENSHIFT is set to true we need to add the necessary steps to setup the environment
# If is not set or is false we need to add custom setup steps to create and destroy kind cluster
function setup_env() {
    if [ -n "${OPENSHIFT:-}" ] && [ "${OPENSHIFT}" != "false" ]; then
        # Running the integ-suite-ocp.sh script to setup the environment for OpenShift
        SKIP_TEST=true ./tests/e2e/integ-suite-ocp.sh
    else
        # run the integ-suite-kind.sh script to setup the kind environment to run the tests
        SKIP_TEST=true ./tests/e2e/integ-suite-kind.sh
    fi
}

# cleanup_env checks if we are running on OpenShift to cleanup the environment for the tests
# if OPENSHIFT is set to true we need to add the necessary steps to cleanup the environment
# If is not set or is false we need to add custom cleanup steps to create and destroy kind cluster
function cleanup_env() {
    if [ -n "${OPENSHIFT:-}" ] && [ "${OPENSHIFT}" != "false" ]; then
        echo "Running cleanup for OpenShift"
    else
        echo "Running cleanup for kind"
    fi
}

# Process all documentation files.
get_all_docs | while IFS= read -r doc; do
    echo "Processing document: $doc" >&2
    get_topic_with_tag_in_doc "$doc" | while IFS= read -r topic; do
        echo "Found topic at line: $topic" >&2
        topicName=$(get_topic_name "$doc" "$topic")
        echo "Topic name: $topicName" >&2
        topicEnd=$(get_topic_end "$doc" "$topic")
        if [ -z "$topicEnd" ]; then
            echo "Warning: No end tag found for topic starting at line $topic in $doc" >&2
            continue
        fi
        echo "Topic end at line: $topicEnd" >&2
        codeBlock=$(get_code_block "$doc" "$topic" "$topicEnd")
        echo "Code block extracted from lines $topic to $topicEnd" >&2
        # Replace prebuilt tags if any exist.
        codeBlock=$(process_prebuilt_tags "$codeBlock")
        generate_script "$doc" "$topicName" "$codeBlock"
    done
done

echo "All the scripts were generated successfully"
echo "Please check the folder docs/scripts for the generated scripts"
echo "Please check the documentation file for more information about the topics"
echo "End of the script"