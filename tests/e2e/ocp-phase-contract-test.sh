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

set -o errexit
set -o nounset
set -o pipefail

repo_root=$(cd "$(dirname "$0")/../.." && pwd)
real_make=$(command -v make)
tmp=$(mktemp -d)
trap 'rm -rf "${tmp}"' EXIT

mock_bin="${tmp}/bin"
mock_log="${tmp}/commands.log"
mkdir -p "${mock_bin}"
for command in go make oc kubectl yq helm operator-sdk istioctl; do
  ln -s "${repo_root}/tests/e2e/testdata/ocp-phase-fake-command.sh" "${mock_bin}/${command}"
done

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

assert_log() {
  local pattern=$1
  grep -Eq -- "${pattern}" "${mock_log}" || fail "missing log pattern: ${pattern}"
}

assert_no_log() {
  local pattern=$1
  if grep -Eq -- "${pattern}" "${mock_log}"; then
    fail "unexpected log pattern: ${pattern}"
  fi
}

run_make() {
  local expected_rc=$1
  shift
  : > "${mock_log}"
  set +o errexit
  env -u SKIP_BUILD -u SKIP_DEPLOY \
    PATH="${mock_bin}:${PATH}" \
    MOCK_LOG="${mock_log}" \
    MOCK_TMP="${tmp}" \
    LOCALBIN="${mock_bin}" \
    KUBECONFIG="${tmp}/kubeconfig" \
    HUB=quay.io/test-sail \
    TAG=exact-tag \
    IMAGE_BASE=sail-operator \
    TARGET_ARCH=amd64 \
    CI=true \
    "$@" \
    "${real_make}" --no-print-directory -C "${repo_root}" \
      BUILD_WITH_CONTAINER=0 LOCALBIN="${mock_bin}" "${MAKE_TARGET}"
  local rc=$?
  set -o errexit
  if [ "${rc}" -ne "${expected_rc}" ]; then
    fail "${MAKE_TARGET}: expected rc ${expected_rc}, got ${rc}"
  fi
}

: > "${tmp}/kubeconfig"

# Helm preparation uses the real top-level Make target and common script, but
# controls image build, cluster, and Helm commands. It must publish exact state
# without invoking Ginkgo or cleanup.
state="${tmp}/helm-state.json"
MAKE_TARGET=test.e2e.ocp.prepare \
  run_make 0 E2E_STATE_FILE="${state}" OLM=false MOCK_CREATE_REPORT=false
jq -e '
  .schemaVersion == 1 and .hub == "quay.io/test-sail" and
  .tag == "exact-tag" and .imageBase == "sail-operator" and
  .namespace == "sail-operator" and .olm == "false" and
  .deploymentName == "sail-operator" and .targetArch == "amd64"
' "${state}" >/dev/null
assert_log '^make .*docker-push'
assert_log '^helm install '
assert_no_log '^go run .*ginkgo'
assert_no_log '^helm uninstall '

# OLM preparation must hand off the deployment name derived from the generated
# CSV and preserve a quoted arm64 architecture value in state.
state="${tmp}/olm-state.json"
MAKE_TARGET=test.e2e.ocp.prepare \
  run_make 0 E2E_STATE_FILE="${state}" OLM=true TARGET_ARCH=arm64
jq -e '
  .olm == "true" and .deploymentName == "sailoperator-controller-manager" and
  .targetArch == "arm64"
' "${state}" >/dev/null
assert_log '^make .*bundle .*bundle-build .*bundle-push'
assert_log '^operator-sdk run bundle '
assert_no_log '^go run .*ginkgo'
assert_no_log '^operator-sdk cleanup '

# Preparation failures must not leave valid-looking state behind.
state="${tmp}/failed-state.json"
printf '{"schemaVersion":1}\n' > "${state}"
MAKE_TARGET=test.e2e.ocp.prepare \
  run_make 2 E2E_STATE_FILE="${state}" OLM=false MOCK_NESTED_MAKE_RC=9
[ ! -e "${state}" ] || fail "failed preparation retained stale state"

# Noncanonical and exact-true skip settings are incompatible with preparation.
# For both Helm and OLM, stale valid state must be invalidated before rejection
# and no external build or cluster command may run.
for case_spec in \
  'helm-skip-build-noncanonical:OLM=false:SKIP_BUILD=1' \
  'olm-skip-deploy-noncanonical:OLM=true:SKIP_DEPLOY=1' \
  'helm-skip-build-true:OLM=false:SKIP_BUILD=true' \
  'olm-skip-deploy-true:OLM=true:SKIP_DEPLOY=true'; do
  case_name=${case_spec%%:*}
  case_values=${case_spec#*:}
  olm_value=${case_values%%:*}
  skip_value=${case_values#*:}
  olm=${olm_value#OLM=}
  state="${tmp}/${case_name}.json"
  jq -n --arg olm "${olm}" \
    '{schemaVersion: 1, hub: "stale", tag: "stale", imageBase: "sail-operator", namespace: "sail-operator", olm: $olm, deploymentName: "sail-operator", targetArch: "amd64"}' \
    > "${state}"
  printf '%s\n' '{"schemaVersion":1}' > "${state}.tmp"
  MAKE_TARGET=test.e2e.ocp.prepare \
    run_make 2 E2E_STATE_FILE="${state}" "${olm_value}" "${skip_value}"
  assert_no_log '^(make|oc|kubectl|helm|operator-sdk) '
  [ ! -e "${state}" ] || fail "${case_name} retained stale final state"
  [ ! -e "${state}.tmp" ] || fail "${case_name} retained stale temporary state"
  echo "Validated ${case_name}: rejected before side effects; stale state absent"
done

# Missing, unwritable, and non-invalidatable state destinations fail clearly
# before preparation. The last case proves state cleanup errors are not ignored.
MAKE_TARGET=test.e2e.ocp.prepare run_make 2 OLM=false
assert_no_log '^(make|oc|kubectl|helm|operator-sdk) '
MAKE_TARGET=test.e2e.ocp.prepare \
  run_make 2 E2E_STATE_FILE=/proc/sail-e2e-state.json OLM=false
assert_no_log '^(make|oc|kubectl|helm|operator-sdk) '
state="${tmp}/state-path-is-directory"
mkdir "${state}"
MAKE_TARGET=test.e2e.ocp.prepare \
  run_make 2 E2E_STATE_FILE="${state}" OLM=false
assert_no_log '^(make|oc|kubectl|helm|operator-sdk) '
echo "Validated non-invalidatable state path: preparation failed closed"

# Test-only must never build or deploy. Logical label-filter operators remain a
# single Ginkgo argument, and suite status wins over cleanup failures.
artifacts="${tmp}/test-artifacts"
mkdir -p "${artifacts}"
MAKE_TARGET=test.e2e.ocp.test-only \
  run_make 2 OLM=false ARTIFACTS="${artifacts}" MOCK_CREATE_REPORT=true \
    MOCK_SUITE_RC=7 MOCK_CLEANUP_FAIL=true \
    GINKGO_LABEL_FILTER='arm64 && !disconnected'
assert_no_log '^make .*docker-push'
assert_no_log '^helm install '
assert_log '^go run .*--label-filter=arm64\\ \\&\\&\\ \\!disconnected '
assert_log '^helm uninstall sail-operator '
diagnostic_line=$(grep -n '^oc get deployment' "${mock_log}" | head -1 | cut -d: -f1)
cleanup_line=$(grep -n '^helm uninstall sail-operator ' "${mock_log}" | head -1 | cut -d: -f1)
[ "${diagnostic_line}" -lt "${cleanup_line}" ] || fail "diagnostics ran after cleanup"

# OLM test-only cleanup uses the stable package name and still preserves the
# Ginkgo failure when cleanup also fails.
rm -rf "${artifacts}"
mkdir -p "${artifacts}"
MAKE_TARGET=test.e2e.ocp.test-only \
  run_make 2 OLM=true ARTIFACTS="${artifacts}" MOCK_CREATE_REPORT=true \
    MOCK_SUITE_RC=8 MOCK_CLEANUP_FAIL=true DEPLOYMENT_NAME=csv-controller
assert_log '^operator-sdk cleanup sailoperator .*--delete-all'
assert_no_log '^make .*bundle'

# Make standardizes a failed recipe to status 2, so exercise the script path
# directly as well to prove cleanup failures cannot replace the suite's status.
: > "${mock_log}"
set +o errexit
PATH="${mock_bin}:${PATH}" MOCK_LOG="${mock_log}" MOCK_TMP="${tmp}" \
  LOCALBIN="${mock_bin}" KUBECONFIG="${tmp}/kubeconfig" \
  HUB=quay.io/test-sail TAG=exact-tag IMAGE_BASE=sail-operator \
  TARGET_ARCH=amd64 CI=true OLM=false ARTIFACTS="${artifacts}" \
  MOCK_CREATE_REPORT=true MOCK_SUITE_RC=7 MOCK_CLEANUP_FAIL=true \
  tests/e2e/common-operator-integ-suite.sh --ocp --test-only
direct_rc=$?
set -o errexit
[ "${direct_rc}" -eq 7 ] || fail "cleanup replaced suite rc 7 with ${direct_rc}"

# Signal handling is a finally path: it cleans up exactly once and returns the
# conventional signal status instead of resuming or replacing it with cleanup.
: > "${mock_log}"
set +o errexit
PATH="${mock_bin}:${PATH}" MOCK_LOG="${mock_log}" MOCK_TMP="${tmp}" \
  LOCALBIN="${mock_bin}" KUBECONFIG="${tmp}/kubeconfig" \
  HUB=quay.io/test-sail TAG=exact-tag IMAGE_BASE=sail-operator \
  TARGET_ARCH=amd64 CI=true OLM=false ARTIFACTS="${artifacts}" \
  MOCK_CREATE_REPORT=false MOCK_SUITE_RC=0 MOCK_SIGNAL_PARENT=TERM \
  tests/e2e/common-operator-integ-suite.sh --ocp --test-only
signal_rc=$?
set -o errexit
[ "${signal_rc}" -eq 143 ] || fail "TERM returned ${signal_rc}, expected 143"
cleanup_count=$(grep -c '^helm uninstall sail-operator ' "${mock_log}")
[ "${cleanup_count}" -eq 1 ] || fail "TERM cleanup ran ${cleanup_count} times"

# State-file handoff: preparation state is loaded by test-only when E2E_STATE_FILE
# is set, overriding any caller-supplied HUB/TAG/NAMESPACE/OLM/DEPLOYMENT_NAME.
handoff_state="${tmp}/handoff-state.json"
jq -n '{schemaVersion: 1, hub: "quay.io/handoff-hub", tag: "handoff-tag",
        imageBase: "sail-operator", namespace: "handoff-ns", olm: "false",
        deploymentName: "sail-operator", targetArch: "amd64"}' \
  > "${handoff_state}"
rm -rf "${artifacts}"
mkdir -p "${artifacts}"
: > "${mock_log}"
set +o errexit
PATH="${mock_bin}:${PATH}" MOCK_LOG="${mock_log}" MOCK_TMP="${tmp}" \
  LOCALBIN="${mock_bin}" KUBECONFIG="${tmp}/kubeconfig" \
  CI=true ARTIFACTS="${artifacts}" \
  E2E_STATE_FILE="${handoff_state}" \
  MOCK_CREATE_REPORT=true MOCK_SUITE_RC=0 \
  tests/e2e/common-operator-integ-suite.sh --ocp --test-only
handoff_rc=$?
set -o errexit
[ "${handoff_rc}" -eq 0 ] || fail "state-file handoff: test-only failed with rc ${handoff_rc}"
assert_log '^helm uninstall sail-operator --namespace handoff-ns'
assert_no_log '^make .*docker-push'
assert_no_log '^helm install '
echo "Validated state-file handoff: test-only loaded hub/tag/namespace from preparation state"

# The original combined target retains build, Helm deploy, Ginkgo, and cleanup.
rm -rf "${artifacts}"
mkdir -p "${artifacts}"
MAKE_TARGET=test.e2e.ocp \
  run_make 0 OLM=false ARTIFACTS="${artifacts}" MOCK_CREATE_REPORT=true
assert_log '^make .*docker-push'
assert_log '^helm install '
assert_log '^go run .*ginkgo'
assert_log '^helm uninstall sail-operator '

echo "Sail OCP phase contract tests passed"
