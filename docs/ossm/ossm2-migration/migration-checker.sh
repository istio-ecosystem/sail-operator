#!/bin/bash
#
# Copyright 2024 Red Hat, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#	http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# This script checks your SMCP and other resources for fields/features that need to be disabled
# before safely migrationg to OSSM 3.0.

set -o pipefail -eu

BLUE='\033[1;34m'
YELLOW='\033[1;33m'
GREEN='\033[1;32m'
BLANK='\033[0m'
WARNING_EMOJI='\u2757'
SPACER="-----------------------"

LATEST_VERSION=v2.6
LATEST_CHART_VERSION=2.6.4
TOTAL_WARNINGS=0

SKIP_PROXY_CHECK=false

# process command line args
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
    --skip-proxy-check)
      SKIP_PROXY_CHECK="${2}"
      shift;shift
      ;;
    -h|--help)
      cat <<HELPMSG
Valid command line arguments:
  --skip-proxy-check
    If 'true', will skip checking proxies for the latest version.
    Default: false
HELPMSG
      exit 1
      ;;
    *)
      echo "ERROR: Unknown argument [$key]. Use --help to see valid arguments."
      exit 1
      ;;
  esac
done

warning() {
  echo -e "${YELLOW}${WARNING_EMOJI}$1${BLANK}"
}

success() {
  echo -e "${GREEN}$1${BLANK}"
}

if ! command -v jq > /dev/null 2>&1
then
    echo "jq must be installed and present in PATH."
    exit 1
fi

if ! command -v oc > /dev/null 2>&1
then
    echo "oc must be installed and present in PATH."
    exit 1
fi

if ! oc whoami > /dev/null 2>&1
then
    echo "Unable to use oc. Ensure your cluster is online and you have logged in with 'oc login'"
    exit 1
fi

check_smcp() {
    local name=$1
    local namespace=$2

    local num_warnings=0

    echo -e "$SPACER\nServiceMeshControlPlane\nName: ${BLUE}$name${BLANK}\nNamespace: ${BLUE}$namespace${BLANK}\n"

    local smcp
    smcp=$(oc get smcp "$name" -n "$namespace" -o json)

    if [ "$(echo "$smcp" | jq -r '.spec.security.manageNetworkPolicy')" != "false" ]; then
        warning "Network Policy is still enabled. Please set '.spec.security.manageNetworkPolicy' = false"
        ((num_warnings+=1))
    fi

    local current_version
    current_version=$(echo "$smcp" | jq -r '.spec.version')
    if [ "$current_version" != "$LATEST_VERSION" ]; then
        warning "Your ServiceMeshControlPlane is not on the latest version. Current version: '$current_version'. Latest version: '$LATEST_VERSION'. Please upgrade your ServiceMeshControlPlane to the latest version."
        ((num_warnings+=1))
    fi

    local current_chart_version
    current_chart_version=$(echo "$smcp" | jq -r '.status.chartVersion')
    if [ "$current_chart_version" != "$LATEST_CHART_VERSION" ]; then
        warning "Your ServiceMeshControlPlane does not have the latest z-stream release. If your ServiceMeshControlPlane is already on the latest version, please ensure your Service Mesh operator is also updated to the latest version. Current version: '$current_chart_version'. Latest version: '$LATEST_CHART_VERSION'."
        ((num_warnings+=1))
    fi

    # Addons
    if [ "$(echo "$smcp" | jq -r '.spec.addons.prometheus.enabled')" != "false" ]; then
        warning "Prometheus addon is still enabled. Please disable the addon by setting '.spec.addons.prometheus.enabled' = false"
        ((num_warnings+=1))
    fi
    
    if [ "$(echo "$smcp" | jq -r '.spec.addons.kiali.enabled')" != "false" ]; then
        warning "Kiali addon is still enabled. Please disable the addon by setting '.spec.addons.kiali.enabled' = false"
        ((num_warnings+=1))
    fi
    
    if [ "$(echo "$smcp" | jq -r '.spec.addons.grafana.enabled')" != "false" ]; then
        warning "Grafana addon is enabled. Grafana is no longer supported with Service Mesh 3.x."
        ((num_warnings+=1))
    fi

    if [ "$(echo "$smcp" | jq -r '.spec.tracing.type')" != "None" ]; then
        warning "Tracing addon is still enabled. Please disable the addon by setting '.spec.tracing.type' = None"
        ((num_warnings+=1))
    fi

    if [ "$(echo "$smcp" | jq -r '.spec.gateways.enabled')" != "false" ]; then
        warning "Gateways are still enabled. Please disable gateways by setting '.spec.gateways.enabled' = false"
        ((num_warnings+=1))
    fi

    # IOR is included in the above check since if this top level gateways field
    # is disabled then IOR is disabled too because there won't be any gateways but
    # we're checking it here to remind users to disable it.
    # Default is 'false' so only log a warning if someone has set it to 'true'.
    if [ "$(echo "$smcp" | jq -r '.spec.gateways.openshiftRoute.enabled')" == "true" ]; then
        warning "IOR is still enabled. Please disable IOR gateways by setting '.spec.gateways.openshiftRoute.enabled' = false"
        ((num_warnings+=1))
    fi

    if [ "$num_warnings" -gt 0 ]; then
        ((TOTAL_WARNINGS += num_warnings))
        echo -e "\n${YELLOW}$num_warnings warnings${BLANK}"
    else
        success "No issues detected with the ServiceMeshControlPlane $name/$namespace."
    fi
}

check_federation() {
    echo -e "Checking for federation resources...\n"
    local num_warnings=0

    if [ "$(oc get exportedservicesets.federation.maistra.io -A -o jsonpath='{.items}' | jq 'length')" != 0 ]; then
        warning "Detected federation resources 'exportedservicesets'. Migrating federation to 3.0 is not supported. Please remove your federation resources."
        ((num_warnings+=1))
    fi

    if [ "$(oc get importedservicesets.federation.maistra.io -A -o jsonpath='{.items}' | jq 'length')" != 0 ]; then
        warning "Detected federation resources 'importedservicesets'. Migrating federation to 3.0 is not supported. Please remove your federation resources."
        ((num_warnings+=1))
    fi

    if [ "$num_warnings" -gt 0 ]; then
        ((TOTAL_WARNINGS += num_warnings))
    else
        success "No federation resources found in the cluster."
    fi

    echo -e "$SPACER"
}

check_proxies_updated() {
    echo -e "Checking proxies are up to date...\n"
    local num_warnings=0
    # Find pods and check each one.
    # Format is name/namespace/version.
    for pod in $(oc get pods -A -l maistra-version -o jsonpath='{range .items[*]}{.metadata.name}/{.metadata.namespace}/{.metadata.labels.maistra-version}{" "}{end}'); do
        IFS="/" read -r name namespace version <<< "$pod"
        # label version format: 2.6.4 --> 2.6
        local sanitized_version
        sanitized_version=$(cut -c1-3 <<< "$version")
        # latest version format: v2.6 --> 2.6
        local sanitized_latest_version
        sanitized_latest_version=$(cut -c2- <<< "$LATEST_VERSION")
        if [ "$sanitized_version" != "$sanitized_latest_version" ]; then
            warning "pod: $name/$namespace is running a proxy at an older version: $sanitized_version Please update your ServiceMeshControlPlane to the latest version: ${LATEST_VERSION} and then restart this workload."
            ((num_warnings+=1))
        fi
    done

    if [ "$num_warnings" -gt 0 ]; then
        ((TOTAL_WARNINGS += num_warnings))
    else
        success "All proxies are on the latest version."
    fi

    echo -e "$SPACER"
}

check_smcps() {
    echo -e "Checking ServiceMeshControlPlanes...\n"
    # Find smcps and check each one.
    # Format is name/namespace.
    for smcp in $(oc get smcp -A -o jsonpath='{range .items[*]}{.metadata.name}/{.metadata.namespace}{" "}{end}'); do
        IFS="/" read -r name namespace <<< "$smcp"
        check_smcp "$name" "$namespace"
    done

    echo -e "$SPACER"
}

check_smcps
check_federation
if [ "$SKIP_PROXY_CHECK" != "true" ]; then
    check_proxies_updated
fi

echo -e "Summary\n"
if [ "$TOTAL_WARNINGS" -eq 0 ]; then
    success "No issues detected. Proceed with the 2.6 --> 3.0 migration."
else
    warning "Detected $TOTAL_WARNINGS issues. Please fix these before proceeding with the migration."
fi
