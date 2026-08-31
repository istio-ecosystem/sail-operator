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

filter_smoke='crc && smoke && !tls-profile'
filter_standard='crc && (ambient-validation || ambient-dependency || ambient-targetref)'
filter_extended='crc && (crd-ownership || reconciliation)'

keep_operator=false
skip_cleanup=false
if [[ "${tier}" == "standard" || "${tier}" == "extended" ]]; then
  keep_operator=true
  skip_cleanup=true
fi

run_e2e smoke "${filter_smoke}" false "${skip_cleanup}" "${artifacts_root}/smoke" || exit 1

if [[ "${tier}" == "standard" || "${tier}" == "extended" ]]; then
  run_e2e standard "${filter_standard}" true true "${artifacts_root}/standard" || exit 1
fi

if [[ "${tier}" == "extended" ]]; then
  run_e2e extended "${filter_extended}" true true "${artifacts_root}/extended" || exit 1
fi
