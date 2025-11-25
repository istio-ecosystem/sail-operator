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

VERSIONS_YAML_DIR=${VERSIONS_YAML_DIR:-"pkg/istioversion"}
VERSIONS_YAML_FILE=${VERSIONS_YAML_FILE:-"versions.yaml"}
VERSIONS_YAML_PATH=${VERSIONS_YAML_DIR}/${VERSIONS_YAML_FILE}
HELM_VALUES_FILE=${HELM_VALUES_FILE:-"chart/values.yaml"}

function updateVersionsInIstioTypeComment() {
    selectValues=$(yq '.versions[] | select (.eol != true) | .name | ", \"urn:alm:descriptor:com.tectonic.ui:select:" + . + "\""' "${VERSIONS_YAML_PATH}" | tr -d '\n')
    versionsEnum=$(yq '.versions[].name' "${VERSIONS_YAML_PATH}" | tr '\n' ';' | sed 's/;$//g')
    versions=$(yq '.versions[] | select (.eol != true) | .name' "${VERSIONS_YAML_PATH}" | tr '\n' ',' | sed -e 's/,/, /g' -e 's/, $//g')
    defaultVersion=$(yq '.versions[1].name' "${VERSIONS_YAML_PATH}")

    sed -i -E \
      -e "/\+sail:version/,/Version string/ s/(\/\/ \+operator-sdk:csv:customresourcedefinitions:type=spec,order=1,displayName=\"Istio Version\",xDescriptors=\{.*fieldGroup:General\")[^}]*(})/\1$selectValues}/g" \
      -e "/\+sail:version/,/Version string/ s/(\/\/ \+kubebuilder:validation:Enum=)(.*)/\1$versionsEnum/g" \
      -e "/\+sail:version/,/Version string/ s/(\/\/ \+kubebuilder:default=)(.*)/\1$defaultVersion/g" \
      -e "/\+sail:version/,/Version string/ s/(\/\/ \Must be one of:)(.*)/\1 $versions./g" \
      -e "s/(\+kubebuilder:default=.*version: \")[^\"]*\"/\1$defaultVersion\"/g" \
      api/v1/istio_types.go api/v1/istiocni_types.go
    
    cniSelectValues=$(yq '.versions[] | select (.eol != true) | select(.ref == null) | .name | ", \"urn:alm:descriptor:com.tectonic.ui:select:" + . + "\""' "${VERSIONS_YAML_PATH}" | tr -d '\n')
    cniVersionsEnum=$(yq '.versions[] | select (. | has("ref") | not) | .name' "${VERSIONS_YAML_PATH}" | tr '\n' ';' | sed 's/;$//g')
    cniVersions=$(yq '.versions[] | select (.eol != true) | select(. | has("ref") | not) | .name' "${VERSIONS_YAML_PATH}" | tr '\n' ',' | sed -e 's/,/, /g' -e 's/, $//g')


    sed -i -E \
      -e "/\+sail:version/,/Version string/ s/(\/\/ \+operator-sdk:csv:customresourcedefinitions:type=spec,order=1,displayName=\"Istio Version\",xDescriptors=\{.*fieldGroup:General\")[^}]*(})/\1$cniSelectValues}/g" \
      -e "/\+sail:version/,/Version string/ s/(\/\/ \+kubebuilder:validation:Enum=)(.*)/\1$cniVersionsEnum/g" \
      -e "/\+sail:version/,/Version string/ s/(\/\/ \+kubebuilder:default=)(.*)/\1$defaultVersion/g" \
      -e "/\+sail:version/,/Version string/ s/(\/\/ \Must be one of:)(.*)/\1 $cniVersions./g" \
      -e "s/(\+kubebuilder:default=.*version: \")[^\"]*\"/\1$defaultVersion\"/g" \
      api/v1/istiorevision_types.go

    # Ambient mode in Sail Operator is supported starting with Istio version 1.24+
    # TODO: Once support for versions prior to 1.24 is discontinued, we can merge the ztunnel specific changes below with the other components.
    ztunnelselectValues=$(yq '.versions[] | select(.version >= "1.24.0" or .ref >= "v1.24.0") | ", \"urn:alm:descriptor:com.tectonic.ui:select:" + .name + "\""' "${VERSIONS_YAML_PATH}" | tr -d '\n')
    ztunnelversionsEnum=$(yq '.versions[] | select(.version >= "1.24.0" or .ref >= "v1.24.0") | .name' "${VERSIONS_YAML_PATH}" | tr '\n' ';' | sed 's/;$//g')
    ztunnelversions=$(yq '.versions[] | select(.version >= "1.24.0" or .ref >= "v1.24.0") | .name' "${VERSIONS_YAML_PATH}" | tr '\n' ',' | sed -e 's/,/, /g' -e 's/, $//g')

    sed -i -E \
      -e "/\+sail:version/,/Version string/ s/(\/\/ \+operator-sdk:csv:customresourcedefinitions:type=spec,order=1,displayName=\"Istio Version\",xDescriptors=\{.*fieldGroup:General\")[^}]*(})/\1$ztunnelselectValues}/g" \
      -e "/\+sail:version/,/Version string/ s/(\/\/ \+kubebuilder:validation:Enum=)(.*)/\1$ztunnelversionsEnum/g" \
      -e "/\+sail:version/,/Version string/ s/(\/\/ \+kubebuilder:default=)(.*)/\1$defaultVersion/g" \
      -e "/\+sail:version/,/Version string/ s/(\/\/ \Must be one of:)(.*)/\1 $ztunnelversions./g" \
      -e "s/(\+kubebuilder:default=.*version: \")[^\"]*\"/\1$defaultVersion\"/g" \
      api/v1alpha1/ztunnel_types.go

    sed -i -E \
      -e "/\+sail:version/,/Version string/ s/(\/\/ \+operator-sdk:csv:customresourcedefinitions:type=spec,order=1,displayName=\"Istio Version\",xDescriptors=\{.*fieldGroup:General\")[^}]*(})/\1$ztunnelselectValues}/g" \
      -e "/\+sail:version/,/Version string/ s/(\/\/ \+kubebuilder:validation:Enum=)(.*)/\1$ztunnelversionsEnum/g" \
      -e "/\+sail:version/,/Version string/ s/(\/\/ \+kubebuilder:default=)(.*)/\1$defaultVersion/g" \
      -e "/\+sail:version/,/Version string/ s/(\/\/ \Must be one of:)(.*)/\1 $ztunnelversions./g" \
      -e "s/(\+kubebuilder:default=.*version: \")[^\"]*\"/\1$defaultVersion\"/g" \
      api/v1/ztunnel_types.go
}

function updateVersionsInCSVDescription() {
    tmpFile=$(mktemp)

    # 1. generate version list from versions.yaml into temporary file
    yq '.versions[] | select (.eol != true) | .name' "${VERSIONS_YAML_PATH}" > "$tmpFile"

    # 2. replace the version list in the CSV description
    awk '
        /This version of the operator supports the following Istio versions:/ {
            in_version_list = 1;
            print;
            print "";
            while (getline < "'"$tmpFile"'") print "    - " $0;
            print "";
        }
        /See this page/ {
            if (in_version_list) {
                in_version_list = 0;
            }
        }
        !in_version_list {
            print;
        }
    ' "${HELM_VALUES_FILE}" > "${HELM_VALUES_FILE}.tmp" && mv "${HELM_VALUES_FILE}.tmp" "${HELM_VALUES_FILE}"

    rm "$tmpFile"
}

function updateVersionInSamples() {
    defaultVersion=$(yq '.versions[1].name' "${VERSIONS_YAML_PATH}")

    sed -i -E \
      -e "s/version: .*/version: $defaultVersion/g" \
      chart/samples/istio-sample.yaml chart/samples/istiocni-sample.yaml chart/samples/istio-sample-gw-api.yaml \
      chart/samples/istio-sample-revisionbased.yaml chart/samples/ambient/istiocni-sample.yaml \
      chart/samples/ambient/istio-sample.yaml  chart/samples/ambient/istioztunnel-sample.yaml
}

updateVersionsInIstioTypeComment
updateVersionsInCSVDescription
updateVersionInSamples
