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

# This script is used to convert istio configuration to sail operator configuration.
# In the end of the execution new yaml file will be created with "sail-ISTIO_CONFIG_YAML" name.
# Usage: ./configuration-converter.sh ISTIO_CONFIG_YAML_WITH_PATH, example:  ./configuration-converter.sh sample_config.yaml"
set -e

# Function to show usage
usage() {
    echo "Usage: $0 <input> [output] [-n <namespace>] [-v <version>]"
    echo "  <input>         : Input file (required)"
    echo "  [output]        : Output file (optional, defaults to input's file name with '-sail.yaml' suffix)"
    echo "  -n <namespace>  : Namespace (optional, defaults to 'istio-system')"
    echo "  -v <version>    : Istio Version (optional)"
    exit 1
}

# Initialize variables
INPUT=""
OUTPUT=""
NAMESPACE="istio-system"
VERSION=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        -n)
            if [[ -z "$2" || "$2" == -* ]]; then
                echo "Error: -n requires a non-empty argument."
                exit 1
            fi
            NAMESPACE="$2"
            shift 2
            ;;
        -v)
            if [[ -z "$2" || "$2" == -* ]]; then
                echo "Error: -v requires a non-empty argument."
                exit 1
            fi
            VERSION="$2"
            shift 2
            ;;
        *)
            if [[ -z "$INPUT" ]]; then
                INPUT="$1"  # First positional argument is the INPUT
            elif [[ -z "$OUTPUT" ]]; then
                OUTPUT="$1"  # Second positional argument is the OUTPUT
            fi
            shift
            ;;
    esac
done

# Ensure the input file is provided and is valid
if [[ -z "$INPUT" || ! -f "$INPUT" ]]; then
    echo "Error: Input file is missing or invalid."
    usage
fi

# If OUTPUT is not specified, generate a default output file name
if [[ -z "$OUTPUT" ]]; then
    OUTPUT="$(dirname "$INPUT")/$(basename "$INPUT" .yaml)-sail.yaml"
elif [[ -d "$OUTPUT" ]]; then
    echo "Error: OUTPUT must be a file, not a directory."
    exit 1
fi

if ! command -v yq &>/dev/null; then
    echo "Error: 'yq' is not installed. Please install it before running the script."
    exit 1
fi

function add_mandatory_fields(){
    yq -i eval ".apiVersion = \"sailoperator.io/v1\" 
    | .kind = \"Istio\" 
    | (select(.spec.meshConfig) | .spec.values.meshConfig) = .spec.meshConfig 
    | (select(.spec.values.istio_cni) | .spec.values.pilot.cni) = .spec.values.istio_cni
    | .metadata.name = \"default\" 
    | .spec.namespace = \"$NAMESPACE\" 
    | del(.spec.values.istio_cni)
    | del(.spec.meshConfig)
    | del(.spec.hub) 
    | del(.spec.tag) 
    | del(.spec.values.gateways)" "$OUTPUT" 
    
    # If VERSION is not empty, add .spec.version
    if [[ -n "$VERSION" ]]; then
        yq -i ".spec.version = \"$VERSION\"" "$OUTPUT"  
    fi

    # Ensure meshConfig and accessLogFile exist
    yq -i eval '.spec.values.meshConfig |= (. // {}) | 
    .spec.values.meshConfig.accessLogFile |= (select(. != "" ) // "/dev/stdout")' "$OUTPUT"
}

function boolean_2_string(){
    #Convert boolean values to string if they are under *.env
    yq -i -e '
    (.spec.values.[].env.[] | select(. == true)) |= "true" |
    (.spec.values.[].env.[] | select(. == false)) |= "false" |
    del(.. | select(length == 0))' "$OUTPUT"
}

# Note that if there is an entry except spec.components.<component>.enabled: true/false converter will delete them and warn user 
function validate_spec_components(){
    if [[ $(yq eval '.spec.components' "$OUTPUT") != "null" ]]; then
        yq -i 'with(.spec.components[]; del( .[] | select(key != "enabled") )) | .spec.values *= .spec.components | del(.spec.components)' "$OUTPUT"
        echo "Only values in the format spec.components.<component>.enabled: true/false are supported for conversion. For more details, refer to the documentation: https://github.com/istio-ecosystem/sail-operator/tree/main/docs#components-field"
    fi
}

# create output file
cp "$INPUT" "$OUTPUT"

# in-place edit created output file  
add_mandatory_fields
boolean_2_string
validate_spec_components

echo "Sail configuration file created with name: $(realpath "$OUTPUT")"

