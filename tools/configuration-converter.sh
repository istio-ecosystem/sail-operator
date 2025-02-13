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
# Default values
NAMESPACE="istio-system"
VERSION=""
INPUT=""
OUTPUT=""

# Function to show usage
usage() {
    echo "Usage: $0 -i <input> [-o <output>] [-n <namespace>] [-v <version>]"
    echo "  -i <input>      : Input file (required)"
    echo "  -o <output>     : Output file (optional, defaults to input's base directory with '-sail.yaml' suffix)"
    echo "  -n <namespace>  : Namespace (optional, defaults to 'istio-system')"
    echo "  -v <version>    : Version (optional)"
    exit 1
}

# Parse command-line arguments
while [[ $# -gt 0 ]]; do
    case "$1" in
        -i)
            INPUT="$2"
            shift 2
            ;;
        -o)
            OUTPUT="$2"
            shift 2
            ;;
        -n)
            NAMESPACE="$2"
            shift 2
            ;;
        -v)
            VERSION="$2"
            shift 2
            ;;
        *)
            usage
            ;;
    esac
done

# Ensure input file is provided
if [[ -z "$INPUT" ]]; then
    echo "Error: Input file is required."
    usage
fi

# Set default output file if not provided. 
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
    | .spec.version = \"$VERSION\" 
    | .spec.namespace = \"$NAMESPACE\" 
    | del(.spec.values.istio_cni)
    | del(.spec.meshConfig)
    | del(.spec.hub) 
    | del(.spec.tag) 
    | del(.spec.values.gateways)" "$OUTPUT"
}

#Convert boolean values to string if they are under *.env
function boolean_2_string(){
    yq -i -e '(.spec.values.[].env.[] | select(. == true)) |= "true" 
    | (.spec.values.[].env.[] | select(. == false)) |= "false"' "$OUTPUT" 
    yq -i -e 'del(.. | select(tag == "!!seq" and length == 0))' "$OUTPUT"
}

# Note that if there is an entry except spec.components.<component>.enabled: true/false converter will delete them and warn user 
function validate_spec_components(){
    yq -i 'del(.spec.components.[] | keys[] | select(. != "enabled")) | .spec.values *= .spec.components | del (.spec.components)' "$OUTPUT"
    echo "Values only spec.components.<component>.enabled: true/false are supported for conversion. Please see https://github.com/istio-ecosystem/sail-operator/tree/main/docs#components-field"
}

# create output file
cp "$INPUT" "$OUTPUT"

# in-place edit created output file  
add_mandatory_fields
boolean_2_string
if [[ $(yq eval '.spec.components' "$OUTPUT") != "null" ]]; then
    validate_spec_components
fi 

echo "Sail configuration file created with name: $(realpath "$OUTPUT")"