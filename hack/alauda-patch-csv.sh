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

VERSIONS_YAML_FILE=${VERSIONS_YAML_FILE:-"versions.yaml"}
VERSIONS_YAML_DIR=${VERSIONS_YAML_DIR:-"pkg/istioversion"}

: "${YQ:=yq}"

## MAIN
if [ $# -ne 2 ]; then
  echo "Usage: $0 <clusterserviceversion_file_path> <values_file_path>"
  exit 1
fi
clusterserviceversion_file_path="$1"
values_file_path="$2"

export RELATED_IMAGES=$(yq -oj '[.deployment.annotations | to_entries[] | {"name": .key, "image": .value}] | sort_by(.name)' "${values_file_path}")
yq -i '.spec.relatedImages = env(RELATED_IMAGES)' "${clusterserviceversion_file_path}"

operator_image="$( ${YQ} '.metadata.annotations.containerImage' "${clusterserviceversion_file_path}" )"
${YQ} -i ".spec.relatedImages |= (. + [{\"name\": \"operator_image\", \"image\": \"${operator_image}\"}])" "${clusterserviceversion_file_path}"
