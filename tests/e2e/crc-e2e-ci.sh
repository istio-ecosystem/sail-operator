#!/usr/bin/env bash
# Copyright Istio Authors
#
# CI-only entry point for CRC E2E (.github/workflows/crc-e2e-sail.yaml).
# Not intended for local use.

set -euo pipefail

tier="${1:?tier required (smoke|standard|extended)}"
workspace="${WORKSPACE:?WORKSPACE is required}"
build_image="${BUILD_IMAGE:?BUILD_IMAGE is required}"
operator_tag="${OPERATOR_TAG:?OPERATOR_TAG is required}"
artifacts_root="${ARTIFACTS_ROOT:-${workspace}/artifacts}"

mkdir -p "${artifacts_root}"

run_e2e() {
  local name="$1"
  local filter="$2"
  local skip_deploy="$3"
  local skip_cleanup="$4"
  local artifacts_dir="$5"

  mkdir -p "${artifacts_dir}"
  echo "::group::CRC E2E ${name} (filter=${filter})"

  set +e
  docker run --rm \
    --network=host \
    --privileged \
    -v "${workspace}":/work \
    -v "${HOME}/.kube":/root/.kube \
    -v "${artifacts_dir}":/tmp/artifacts \
    -e KUBECONFIG=/root/.kube/config \
    -e HUB=quay.io/sail-dev \
    -e TAG="${operator_tag}" \
    -e CI=true \
    -e BUILD_WITH_CONTAINER=0 \
    -e SKIP_BUILD=true \
    -e SKIP_DEPLOY="${skip_deploy}" \
    -e SKIP_CLEANUP="${skip_cleanup}" \
    -e XDG_CACHE_HOME=/tmp/cache \
    -e ARTIFACTS=/tmp/artifacts \
    -e ISTIOD_MEMORY_REQUEST=128Mi \
    -e CNI_MEMORY_REQUEST=40Mi \
    -e ZTUNNEL_MEMORY_REQUEST=64Mi \
    -e PROXY_MEMORY_REQUEST=64Mi \
    -e E2E_VERSIONS_LIMIT=1 \
    -e OPERATOR_DEPLOY_TIMEOUT=10m \
    -e OPERATOR_MEMORY_REQUEST=32Mi \
    -e OPERATOR_MEMORY_LIMIT=256Mi \
    -e OPERATOR_CPU_REQUEST=10m \
    -e OPERATOR_CPU_LIMIT=200m \
    -e "GINKGO_LABEL_FILTER=${filter}" \
    "${build_image}" \
    bash -c "git config --global --add safe.directory /work && cd /work && make test.e2e.ocp"
  local rc=$?
  set -e

  echo "::endgroup::"
  return "${rc}"
}

cleanup_mesh_between_tiers() {
  local control_plane_ns="${CONTROL_PLANE_NS:-istio-system}"
  local istiocni_ns="${ISTIOCNI_NAMESPACE:-istio-cni}"
  local ztunnel_ns="${ZTUNNEL_NAMESPACE:-ztunnel}"
  local wait_timeout="${CRC_MESH_CLEANUP_TIMEOUT:-300s}"
  local command namespace resource

  echo "::group::CRC E2E mesh cleanup between tiers"

  if command -v oc &>/dev/null && oc status &>/dev/null; then
    command=oc
  elif command -v kubectl &>/dev/null; then
    command=kubectl
  else
    echo "Neither oc nor kubectl found" >&2
    return 1
  fi

  delete_cr() {
    resource="$1"
    if "${command}" get "${resource}" --all-namespaces --no-headers 2>/dev/null | grep -q .; then
      echo "Deleting all ${resource} resources..."
      "${command}" delete "${resource}" --all --all-namespaces \
        --wait=true --timeout="${wait_timeout}" --ignore-not-found
    fi
  }

  delete_namespace() {
    namespace="$1"
    if "${command}" get namespace "${namespace}" &>/dev/null; then
      echo "Deleting namespace ${namespace}..."
      "${command}" delete namespace "${namespace}" \
        --wait=true --timeout="${wait_timeout}" --ignore-not-found
    fi
  }

  echo "CRC tier mesh cleanup (operator preserved)"

  # Drop sample-app namespaces first to free node memory before CR finalizers run.
  for namespace in sleep httpbin; do
    delete_namespace "${namespace}"
  done

  delete_cr istios.sailoperator.io
  delete_cr istiorevisions.sailoperator.io
  delete_cr istiorevisiontags.sailoperator.io
  delete_cr istiocni.sailoperator.io
  delete_cr ztunnel.sailoperator.io

  for namespace in "${control_plane_ns}" "${istiocni_ns}" "${ztunnel_ns}"; do
    delete_namespace "${namespace}"
  done

  echo "CRC tier mesh cleanup complete"
  echo "::endgroup::"
}

filter_smoke='crc && smoke && !tls-profile'
filter_standard='crc && (ambient-validation || ambient-dependency || ambient-targetref) && !ambient-targetref-override'
filter_extended='crc && (crd-ownership || reconciliation) && !library-upgrade'

skip_cleanup=false
if [[ "${tier}" == "standard" || "${tier}" == "extended" ]]; then
  skip_cleanup=true
fi

run_e2e smoke "${filter_smoke}" false "${skip_cleanup}" "${artifacts_root}/smoke" || exit 1

if [[ "${tier}" == "standard" || "${tier}" == "extended" ]]; then
  cleanup_mesh_between_tiers
  run_e2e standard "${filter_standard}" true true "${artifacts_root}/standard" || exit 1
fi

if [[ "${tier}" == "extended" ]]; then
  cleanup_mesh_between_tiers
  run_e2e extended "${filter_extended}" true true "${artifacts_root}/extended" || exit 1
fi
