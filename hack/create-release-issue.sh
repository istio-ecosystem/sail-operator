#!/usr/bin/env bash

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

# Create (or dry-run) a "Release Sail Operator <VERSION>" checklist issue.
#
# Used by .github/workflows/create-release-issue.yaml and /add-istio-version.
#
# Usage:
#   hack/create-release-issue.sh --pr-url URL [--pr-title TITLE] [--version X.Y.Z] [--dry-run]
#
# Environment:
#   GH_TOKEN / GITHUB_TOKEN  — required for gh
#   GITHUB_REPOSITORY         — default repo (owner/name); overridable with --repo

set -euo pipefail

usage() {
  cat <<'EOF'
Usage: hack/create-release-issue.sh [options]

Options:
  --pr-url URL         PR URL to embed in the checklist (required)
  --pr-title TITLE     If set, skip unless the title matches
                       "Automator: Update dependencies in ...@release-"
  --version X.Y.Z      Override VERSION (default: parse Makefile.core.mk)
  --base-branch NAME   PR base branch (e.g. release-1.31). Required unless --force.
                       Skips unless this is the highest release-X.Y branch (active OLM line).
  --force              Create even when --base-branch is not the latest release-* branch
  --repo OWNER/NAME    GitHub repo (default: $GITHUB_REPOSITORY or istio-ecosystem/sail-operator)
  --template PATH      Checklist body template (default: .github/ISSUE_TEMPLATE/release-checklist.md)
  --dry-run            Print actions only; do not create an issue
  -h, --help           Show this help
EOF
}

REPO="${GITHUB_REPOSITORY:-istio-ecosystem/sail-operator}"
PR_URL=""
PR_TITLE=""
VERSION=""
BASE_BRANCH=""
FORCE=false
DRY_RUN=false
TEMPLATE=".github/ISSUE_TEMPLATE/release-checklist.md"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --pr-url)
      PR_URL="${2:-}"
      shift 2
      ;;
    --pr-title)
      PR_TITLE="${2:-}"
      shift 2
      ;;
    --version)
      VERSION="${2:-}"
      shift 2
      ;;
    --base-branch)
      BASE_BRANCH="${2:-}"
      shift 2
      ;;
    --force)
      FORCE=true
      shift
      ;;
    --repo)
      REPO="${2:-}"
      shift 2
      ;;
    --template)
      TEMPLATE="${2:-}"
      shift 2
      ;;
    --dry-run)
      DRY_RUN=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ -z "${PR_URL}" ]]; then
  echo "--pr-url is required" >&2
  usage >&2
  exit 1
fi

# Prefer GH_TOKEN; fall back to GITHUB_TOKEN (Actions default).
if [[ -z "${GH_TOKEN:-}" && -n "${GITHUB_TOKEN:-}" ]]; then
  export GH_TOKEN="${GITHUB_TOKEN}"
fi

# Require the "Automator: Update dependencies in <org>/<repo>@release-X.Y" title
# of the daily update-deps automation, which adds new Istio versions on release
# branches. Skips every other PR title, including "Add Istio ..." PRs (their
# checklist, if needed, is opened manually or via workflow_dispatch).
if [[ -n "${PR_TITLE}" ]]; then
  if ! grep -qE '^Automator: Update dependencies in .+@release-' <<< "${PR_TITLE}"; then
    echo "PR title does not match 'Automator: Update dependencies in ...@release-'; skipping: ${PR_TITLE}"
    exit 0
  fi
fi

# The checklist is only needed when the PR actually added Istio versions, i.e.
# changed pkg/istioversion/versions.yaml. Most "Automator: Update dependencies"
# PRs only bump libraries/tools and must not open one.
# Note: use `gh pr diff --name-only`, not `gh pr view --json files`; the latter
# caps at 100 files and these PRs regularly change several hundred, which would
# silently hide versions.yaml and skip the checklist.
PR_FILES=$(gh pr diff "${PR_URL}" --repo "${REPO}" --name-only)
if ! grep -qx "pkg/istioversion/versions.yaml" <<< "${PR_FILES}"; then
  echo "PR did not change pkg/istioversion/versions.yaml; no Istio version was added; skipping."
  exit 0
fi

# Only the active OLM line (highest release-X.Y) gets a checklist. Older
# release-* Add Istio backports do not open Release Sail Operator issues.
latest_release_branch() {
  # Prefer matching-refs over listing every branch. Only y-stream names
  # (release-X.Y), not patch-level release-X.Y.Z if any exist.
  gh api "repos/${REPO}/git/matching-refs/heads/release-" --jq '.[].ref' \
    | sed 's|^refs/heads/||' \
    | grep -E '^release-[0-9]+\.[0-9]+$' \
    | sort -V \
    | tail -1
}

if [[ "${FORCE}" != true ]]; then
  if [[ -z "${BASE_BRANCH}" ]]; then
    echo "--base-branch is required (or pass --force)" >&2
    usage >&2
    exit 1
  fi
  LATEST_BRANCH=$(latest_release_branch)
  if [[ -z "${LATEST_BRANCH}" ]]; then
    echo "Could not determine latest release-X.Y branch in ${REPO}" >&2
    exit 1
  fi
  echo "Latest release branch: ${LATEST_BRANCH}"
  if [[ "${BASE_BRANCH}" != "${LATEST_BRANCH}" ]]; then
    echo "Base branch ${BASE_BRANCH} is not the latest (${LATEST_BRANCH}); skipping release checklist."
    exit 0
  fi
fi

if [[ -z "${VERSION}" ]]; then
  if [[ ! -f Makefile.core.mk ]]; then
    echo "Makefile.core.mk not found; pass --version or run from repo root" >&2
    exit 1
  fi
  VERSION=$(grep -E '^VERSION \?=' Makefile.core.mk | head -1 | sed -E 's/^VERSION \?=[[:space:]]*//')
fi
if [[ -z "${VERSION}" ]]; then
  echo "Could not parse VERSION from Makefile.core.mk" >&2
  exit 1
fi
echo "Parsed VERSION=${VERSION}"

# Print "<number>\t<url>" of the first issue with exactly the given title and state.
find_issue() {
  local title="$1" state="$2"
  gh issue list \
    --repo "${REPO}" \
    --state "${state}" \
    --search "in:title \"${title}\"" \
    --json number,title,url \
    --jq ".[] | select(.title == \"${title}\") | \"\(.number)\t\(.url)\"" \
    | head -1
}

# VERSION in Makefile.core.mk is the source of truth. On a release branch it holds
# the last released version, because the operator VERSION is only bumped by the
# release preparation workflow. The one exception is a release in progress: the
# prep PR already bumped VERSION and its checklist is still open. Then the new Istio
# versions fold into that release instead of opening a checklist for VERSION+1.
IN_PROGRESS=$(find_issue "Release Sail Operator ${VERSION}" open)
if [[ -n "${IN_PROGRESS}" ]]; then
  IN_PROGRESS_NUM="${IN_PROGRESS%%$'\t'*}"
  IN_PROGRESS_URL="${IN_PROGRESS#*$'\t'}"
  echo "Release ${VERSION} is in progress (#${IN_PROGRESS_NUM} is open); folding new Istio versions into it."
  if [[ "${DRY_RUN}" == true ]]; then
    echo "DRY_RUN=true — would comment on ${IN_PROGRESS_URL}"
  else
    gh issue comment "${IN_PROGRESS_NUM}" --repo "${REPO}" \
      --body "More Istio versions were added in ${PR_URL} while this release is in progress. Re-run the release preparation workflow (with \`version: ${VERSION}\`) to pin them and include them in this release."
  fi
  exit 0
fi

TITLE_VERSION=$(echo "${VERSION}" | awk -F. '{print $1"."$2"."$3+1}')
TITLE="Release Sail Operator ${TITLE_VERSION}"
# Dedupe across all states so a closed checklist for the same VERSION is not
# recreated on re-runs; reopen the existing issue if a redo is needed.
EXISTING=$(find_issue "${TITLE}" all)
if [[ -n "${EXISTING}" ]]; then
  EXISTING_NUM="${EXISTING%%$'\t'*}"
  EXISTING_URL="${EXISTING#*$'\t'}"
  echo "Issue #${EXISTING_NUM} already exists with title \"${TITLE}\"; skipping."
  echo "Existing: ${EXISTING_URL}"
  exit 0
fi

if [[ ! -f "${TEMPLATE}" ]]; then
  echo "Template not found: ${TEMPLATE}" >&2
  exit 1
fi

BODY_FILE=$(mktemp)
trap 'rm -f "${BODY_FILE}"' EXIT
# Drop optional YAML frontmatter; substitute the placeholders (PR URL, repo URL so
# workflow links point to the repo the issue is created in, and the release branch).
awk '
  NR == 1 && /^---$/ { fm = 1; next }
  fm && /^---$/ { fm = 0; next }
  fm { next }
  { print }
' "${TEMPLATE}" | sed -e "s|ADD_ISTIO_PR_URL|${PR_URL}|g" \
  -e "s|REPO_URL|https://github.com/${REPO}|g" \
  -e "s|RELEASE_BRANCH|${BASE_BRANCH:-release-X.Y}|g" > "${BODY_FILE}"

LABELS=("automated" "release")

if [[ "${DRY_RUN}" == true ]]; then
  echo "DRY_RUN=true — would create issue:"
  echo "  repo:   ${REPO}"
  echo "  title:  ${TITLE}"
  echo "  pr:     ${PR_URL}"
  echo "  labels: ${LABELS[*]}"
  echo "  body:"
  sed 's/^/    /' "${BODY_FILE}"
  exit 0
fi

# Ensure the labels exist (forks used for testing may have neither). Matches
# validate_ztunnel_values / brokenlinks, which attach workflow-specific labels.
gh label create automated \
  --repo "${REPO}" \
  --description "Created by automation" \
  --color "EDEDED" >/dev/null 2>&1 || true
gh label create release \
  --repo "${REPO}" \
  --description "Release checklist / process" \
  --color "0E8A16" >/dev/null 2>&1 || true

label_args=()
for label in "${LABELS[@]}"; do
  label_args+=(--label "${label}")
done

ISSUE_URL=$(gh issue create \
  --repo "${REPO}" \
  --title "${TITLE}" \
  --body-file "${BODY_FILE}" \
  "${label_args[@]}")
echo "Created ${ISSUE_URL} for PR: ${PR_TITLE:-${PR_URL}}"
