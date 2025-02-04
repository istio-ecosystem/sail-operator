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

if [ -z "$1" ] || [ -z "$2" ] || [ -z "$3" ]; then
    echo "Usage: $0 <ISTIO_OPERATOR_CONFIG_YAML_WITH_PATH> <ISTIO_NAMESPACE> <ISTIO_VERSION>"
    exit 1
fi

WORKDIR=$(dirname "$1")
ISTIO_CONFIG_FILE=$(basename "$1")
ISTIO_VERSION="$2"
ISTIO_NAMESPACE="$3"
SAIL_CONFIG_FILE="sail-operator-config.yaml"

cp "$WORKDIR"/"$ISTIO_CONFIG_FILE" "$SAIL_CONFIG_FILE" || exit 1

function add_mandatory_fields(){
    yq -i eval ".apiVersion = \"sailoperator.io/v1\" 
    | .kind = \"Istio\" 
    | (select(.spec.meshConfig) | .spec.values.meshConfig) = .spec.meshConfig 
    | (select(.spec.values.istio_cni) | .spec.values.pilot.cni) = .spec.values.istio_cni 
    | .metadata.name = \"default\" 
    | .spec.version = \"$ISTIO_VERSION\" 
    | .spec.namespace = \"$ISTIO_NAMESPACE\" 
    | del(.spec.values.istio_cni)
    | del(.spec.meshConfig)
    | del(.spec.hub) 
    | del(.spec.tag) 
    | del(.spec.values.gateways)" "$SAIL_CONFIG_FILE"
}

#Convert boolean values to string if they are under *.env
function boolean_2_string(){
    yq -i -e '(.spec.values.[].env.[] | select(. == true)) |= "true" 
    | (.spec.values.[].env.[] | select(. == false)) |= "false"' "$SAIL_CONFIG_FILE" 
    yq -i -e 'del(.. | select(tag == "!!seq" and length == 0))' "$SAIL_CONFIG_FILE"
}

# Note that if there is an entry except spec.components.<component>.enabled: true/false converter will delete them and warn user 
function validate_spec_components(){
    yq -i 'del(.spec.components.[] | keys[] | select(. != "enabled")) | .spec.values *= .spec.components | del (.spec.components)' $SAIL_CONFIG_FILE
    echo "Converter can only be used values with spec.components.<component>.enabled: true/false. Please see https://github.com/istio-ecosystem/sail-operator/tree/main/docs#components-field"

}

add_mandatory_fields
boolean_2_string
if [[ $(yq eval '.spec.components' "$SAIL_CONFIG_FILE") != "null" ]]; then
    validate_spec_components
fi 

chmod +x $SAIL_CONFIG_FILE

if ! mv "$SAIL_CONFIG_FILE" "$WORKDIR" 2>&1 | grep -q "are identical"; then
    true  
fi

echo "Sail configuration file created with name: ${SAIL_CONFIG_FILE}"   
