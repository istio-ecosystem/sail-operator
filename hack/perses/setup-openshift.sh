#!/usr/bin/env bash
# Copyright Istio Authors
#
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

set -euo pipefail

# Sets up an OpenShift cluster for validating Sail Operator Perses dashboards.
#
# Prerequisites (manual):
#   - Logged into the cluster (oc login)
#   - Cluster Observability Operator (COO) installed from OperatorHub
#
# Usage:
#   hack/perses/setup-openshift.sh [all|uwm|perses|operator|istio|monitoring|bookinfo|validate]
#
# Environment:
#   OPERATOR_NAMESPACE  Namespace where Sail runs (default: sail-operator)
#   ISTIO_VERSION       Istio/Sail version for Istio + IstioCNI (default: v1.31.0)
#   HUB / TAG           Image for make deploy (optional; used when step=operator|all)
#   SKIP_OPERATOR_BUILD If set to 1, deploy without rebuilding (default: 0)

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
MANIFESTS="${SCRIPT_DIR}/manifests"

OPERATOR_NAMESPACE="${OPERATOR_NAMESPACE:-sail-operator}"
ISTIO_VERSION="${ISTIO_VERSION:-v1.31.0}"
SKIP_OPERATOR_BUILD="${SKIP_OPERATOR_BUILD:-0}"
OC="${OC:-oc}"
STEP="${1:-all}"

log() { echo "==> $*"; }

require_oc() {
  if ! command -v "${OC}" >/dev/null 2>&1; then
    echo "oc not found; set OC=/path/to/oc or install the OpenShift CLI" >&2
    exit 1
  fi
  if ! "${OC}" whoami >/dev/null 2>&1; then
    echo "Not logged into a cluster. Run: oc login ..." >&2
    exit 1
  fi
}

apply_file() {
  local file="$1"
  log "Applying ${file#"${REPO_ROOT}/"}"
  "${OC}" apply -f "${file}"
}

apply_operator_ns_manifest() {
  local file="$1"
  if [[ "${OPERATOR_NAMESPACE}" == "sail-operator" ]]; then
    apply_file "${file}"
    return
  fi
  log "Applying ${file#"${REPO_ROOT}/"} (namespace=${OPERATOR_NAMESPACE})"
  sed "s/namespace: sail-operator/namespace: ${OPERATOR_NAMESPACE}/g" "${file}" | "${OC}" apply -f -
}

step_uwm() {
  log "Enabling User Workload Monitoring"
  apply_file "${MANIFESTS}/cluster-monitoring-config.yaml"
}

step_perses() {
  log "Enabling Perses via COO UIPlugin"
  apply_file "${MANIFESTS}/uiplugin-monitoring.yaml"
  log "Waiting for PersesDashboard CRD"
  "${OC}" wait --for=condition=Established crd/persesdashboards.perses.dev --timeout=5m
  log "Creating PersesDatasource in ${OPERATOR_NAMESPACE}"
  "${OC}" get ns "${OPERATOR_NAMESPACE}" >/dev/null 2>&1 || "${OC}" create ns "${OPERATOR_NAMESPACE}"
  apply_operator_ns_manifest "${MANIFESTS}/perses-datasource.yaml"
}

step_operator() {
  log "Deploying Sail Operator to ${OPERATOR_NAMESPACE}"
  cd "${REPO_ROOT}"
  if [[ "${SKIP_OPERATOR_BUILD}" != "1" ]]; then
    log "Building and deploying (HUB=${HUB:-<default>} TAG=${TAG:-<default>})"
    make deploy
  else
    log "SKIP_OPERATOR_BUILD=1; running make deploy only (image must already exist)"
    make deploy
  fi
}

step_istio() {
  log "Installing Istio ${ISTIO_VERSION} and IstioCNI"
  "${OC}" get ns istio-system >/dev/null 2>&1 || "${OC}" create ns istio-system
  "${OC}" get ns istio-cni >/dev/null 2>&1 || "${OC}" create ns istio-cni
  # shellcheck disable=SC2016
  sed "s/\${ISTIO_VERSION}/${ISTIO_VERSION}/g" "${MANIFESTS}/istio-and-cni.yaml.tmpl" | "${OC}" apply -f -
  log "Waiting for Istio to become ready"
  "${OC}" wait --for=condition=Ready istios/default --timeout=5m || true
}

step_monitoring() {
  log "Creating ServiceMonitor / PodMonitor / Telemetry"
  apply_file "${MANIFESTS}/istiod-service-monitor.yaml"
  apply_file "${MANIFESTS}/istio-pod-monitor.yaml"
  apply_file "${MANIFESTS}/telemetry-prometheus.yaml"
}

step_bookinfo() {
  log "Installing bookinfo sample"
  "${OC}" get ns bookinfo >/dev/null 2>&1 || "${OC}" create ns bookinfo
  "${OC}" label ns bookinfo istio.io/rev=default --overwrite
  "${OC}" apply -n bookinfo -f https://raw.githubusercontent.com/istio/istio/master/samples/bookinfo/platform/kube/bookinfo.yaml
  "${OC}" apply -n bookinfo -f https://raw.githubusercontent.com/istio/istio/master/samples/bookinfo/platform/kube/bookinfo-versions.yaml
  apply_file "${MANIFESTS}/bookinfo-pod-monitor.yaml"

  if ! "${OC}" get crd gateways.gateway.networking.k8s.io >/dev/null 2>&1; then
    log "Installing Gateway API CRDs"
    "${OC}" kustomize "github.com/kubernetes-sigs/gateway-api/config/crd?ref=v1.1.0" | "${OC}" apply -f -
  fi
  "${OC}" apply -n bookinfo -f https://raw.githubusercontent.com/istio/istio/master/samples/bookinfo/gateway-api/bookinfo-gateway.yaml
  "${OC}" wait -n bookinfo --for=condition=programmed gtw/bookinfo-gateway --timeout=5m || true

  if ! "${OC}" get route productpage -n bookinfo >/dev/null 2>&1; then
    "${OC}" expose svc/productpage -n bookinfo || true
  fi
  "${OC}" get route productpage -n bookinfo || true
}

step_validate() {
  log "Validating Perses dashboards in ${OPERATOR_NAMESPACE}"
  "${OC}" get crd persesdashboards.perses.dev
  "${OC}" get persesdatasource prometheus-datasource -n "${OPERATOR_NAMESPACE}"
  echo
  "${OC}" get persesdashboards -n "${OPERATOR_NAMESPACE}"
  echo
  local expected=(
    istio-control-plane
    istio-mesh-dashboard
    istio-performance
    istio-service-dashboard
    istio-workload-dashboard
    istio-ztunnel-dashboard
  )
  local missing=0
  for name in "${expected[@]}"; do
    if ! "${OC}" get persesdashboard "${name}" -n "${OPERATOR_NAMESPACE}" >/dev/null 2>&1; then
      echo "MISSING: ${name}"
      missing=1
    else
      echo "OK: ${name}"
    fi
  done
  echo
  log "Recent Sail Operator logs (perses)"
  "${OC}" logs -n "${OPERATOR_NAMESPACE}" deploy/sail-operator --tail=100 2>/dev/null | grep -i perses || true
  if [[ "${missing}" -ne 0 ]]; then
    echo "Validation failed: not all dashboards are present" >&2
    exit 1
  fi
  log "Dashboards present. Generate bookinfo traffic and check Observe → Monitoring (Perses) in the OpenShift console."
}

usage() {
  cat <<EOF
Usage: $0 [all|uwm|perses|operator|istio|monitoring|bookinfo|validate]

Order for 'all':
  1. uwm         Enable User Workload Monitoring
  2. operator    Build/deploy Sail Operator
  3. perses      UIPlugin + PersesDatasource in OPERATOR_NAMESPACE
  4. istio       Istio + IstioCNI (ISTIO_VERSION)
  5. monitoring  ServiceMonitor / PodMonitors / Telemetry
  6. bookinfo    Sample app + gateway + route
  7. validate    Check CRD, datasource, and 6 dashboards

Prereq: oc login + Cluster Observability Operator installed.
EOF
}

main() {
  require_oc
  case "${STEP}" in
    all)
      step_uwm
      step_operator
      step_perses
      step_istio
      step_monitoring
      step_bookinfo
      step_validate
      ;;
    uwm) step_uwm ;;
    perses) step_perses ;;
    operator) step_operator ;;
    istio) step_istio ;;
    monitoring) step_monitoring ;;
    bookinfo) step_bookinfo ;;
    validate) step_validate ;;
    -h|--help|help) usage ;;
    *)
      echo "Unknown step: ${STEP}" >&2
      usage
      exit 1
      ;;
  esac
}

main
