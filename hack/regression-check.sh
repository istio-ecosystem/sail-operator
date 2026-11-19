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

# Fails if a bug fix (a changelog/ fragment with category 'fixed') is about to
# land on, or be released from, a versioned release branch without also being
# present on every higher release branch, so higher branches never regress
# behind lower ones.
#
# Two modes, selected via CHECK_MODE (default 'pr'):
#
#   pr      PR-time gate. Fragments touched by the PR must be present (merged)
#           on every higher release branch and on main. A fragment counts as
#           present on a branch when:
#             - the same fragment file exists on that branch,
#             - a fragment or CHANGELOG entry on that branch references the
#               same issueLink (covers fragments renamed across branches and
#               fragments already consumed by a release cut on that branch),
#             - the branch's CHANGELOG.md contains the fragment's title.
#
#   release Release-time gate. Every pending 'fixed' fragment on the branch
#           being released must already be shipped (present in CHANGELOG.md)
#           on every higher release branch, so a lower release never goes out
#           before the higher branches' releases containing its fixes. main is
#           not checked in this mode: fixes stay as pending fragments there
#           until the next minor cut.
#
# A fragment with 'propagate: false' is skipped in both modes; it marks a fix
# that deliberately stays on its branch.
#
# Required environment variables:
#   BASE_BRANCH - the release branch being targeted (pr mode) or released
#                 (release mode), e.g. release-1.29
#   BASE_SHA    - the PR's base sha (pr mode only)
#   HEAD_SHA    - the PR's head sha (pr mode only)
#   CHECK_MODE  - 'pr' (default) or 'release'

set -euo pipefail

CHECK_MODE="${CHECK_MODE:-pr}"
BASE_BRANCH="${BASE_BRANCH:?BASE_BRANCH environment variable must be set}"

if [[ "${CHECK_MODE}" != "pr" && "${CHECK_MODE}" != "release" ]]; then
  echo "::error::CHECK_MODE must be 'pr' or 'release' (got '${CHECK_MODE}')."
  exit 1
fi

if [[ "${CHECK_MODE}" == "pr" ]]; then
  BASE_SHA="${BASE_SHA:?BASE_SHA environment variable must be set}"
  HEAD_SHA="${HEAD_SHA:?HEAD_SHA environment variable must be set}"
fi

if [[ ! "${BASE_BRANCH}" =~ ^release-([0-9]+)\.([0-9]+)$ ]]; then
  echo "Base branch '${BASE_BRANCH}' is not a versioned release branch; nothing to check."
  exit 0
fi
base_major="${BASH_REMATCH[1]}"
base_minor="${BASH_REMATCH[2]}"

# Fragments subject to the check: those touched by the PR (pr mode) or all
# pending fragments on the branch being released (release mode)
if [[ "${CHECK_MODE}" == "pr" ]]; then
  fragments=$(git diff --name-only --diff-filter=ACMR "${BASE_SHA}...${HEAD_SHA}" -- 'changelog/*.yaml')
else
  fragments=$(git ls-files 'changelog/*.yaml')
fi
if [[ -z "${fragments}" ]]; then
  echo "No changelog fragments to check."
  exit 0
fi

# All release branches with a version higher than the base branch. In pr mode
# main counts as the highest branch; in release mode only release branches are
# checked, because main cuts no patch releases.
higher_branches=()
for ref in $(git for-each-ref --format='%(refname:short)' refs/remotes/origin); do
  branch="${ref#origin/}"
  if [[ "${branch}" =~ ^release-([0-9]+)\.([0-9]+)$ ]]; then
    major="${BASH_REMATCH[1]}"
    minor="${BASH_REMATCH[2]}"
    if (( 10#${major} > 10#${base_major} )) || (( 10#${major} == 10#${base_major} && 10#${minor} > 10#${base_minor} )); then
      higher_branches+=("${branch}")
    fi
  fi
done
if [[ "${CHECK_MODE}" == "pr" ]]; then
  higher_branches+=("main")
fi

if [[ ${#higher_branches[@]} -eq 0 ]]; then
  echo "No higher release branches found; nothing to check."
  exit 0
fi

echo "Checking that bug fixes are present on: ${higher_branches[*]}"

# Extracts a top-level field (e.g. title, issueLink) from a fragment file
fragment_field() { # <file> <field>
  grep -m1 "^${2}:" "${1}" | sed "s/^${2}:[[:space:]]*//" || true
}

# Reports whether a fragment's fix is present on a branch, and why. In release
# mode only a CHANGELOG.md entry (i.e. a fix shipped by a release cut on that
# branch) counts.
fragment_present_on_branch() { # <fragment> <branch>
  local fragment="${1}" branch="${2}"
  local ref="refs/remotes/origin/${branch}"
  local title issue_link
  title="$(fragment_field "${fragment}" title)"
  issue_link="$(fragment_field "${fragment}" issueLink)"

  if [[ "${CHECK_MODE}" == "pr" ]]; then
    if git cat-file -e "${ref}:${fragment}" 2>/dev/null; then
      echo "OK: ${fragment} is present on ${branch} (fragment file)"
      return 0
    fi
    if [[ -n "${issue_link}" ]] && git grep -qF "${issue_link}" "${ref}" -- changelog CHANGELOG.md 2>/dev/null; then
      echo "OK: ${fragment} is present on ${branch} (issueLink reference)"
      return 0
    fi
  else
    if [[ -n "${issue_link}" ]] && git grep -qF "${issue_link}" "${ref}" -- CHANGELOG.md 2>/dev/null; then
      echo "OK: ${fragment} is released on ${branch} (issueLink reference)"
      return 0
    fi
  fi
  if [[ -n "${title}" ]] && git grep -qF -e "- ${title}" "${ref}" -- CHANGELOG.md 2>/dev/null; then
    echo "OK: ${fragment} is present on ${branch} (CHANGELOG entry)"
    return 0
  fi
  return 1
}

errors=false

for fragment in ${fragments}; do
  [[ -f "${fragment}" ]] || continue
  category="$(fragment_field "${fragment}" category)"
  # Only bug fixes are subject to the check; other kinds of change (e.g. new
  # features or version additions) do not need to be present on every branch.
  if [[ "${category}" != "fixed" ]]; then
    echo "Skipping ${fragment}: category '${category}' is not a bug fix."
    continue
  fi
  if [[ "$(fragment_field "${fragment}" propagate)" == "false" ]]; then
    echo "Skipping ${fragment}: marked as 'propagate: false'."
    continue
  fi

  for branch in "${higher_branches[@]}"; do
    if fragment_present_on_branch "${fragment}" "${branch}"; then
      continue
    fi

    if [[ "${CHECK_MODE}" == "pr" ]]; then
      echo "::error file=${fragment}::'${fragment}' is not present on '${branch}'. Merge or cherry-pick this change to '${branch}' (and every branch above ${BASE_BRANCH}) before merging it here. If this change is specific to ${BASE_BRANCH} and should not be propagated, set 'propagate: false' in the fragment or add the 'skip-regression-check' label."
    else
      echo "::error file=${fragment}::'${fragment}' has not been released on '${branch}' yet. Cut the '${branch}' release containing this fix before releasing from ${BASE_BRANCH}. If the fix deliberately stays on ${BASE_BRANCH}, set 'propagate: false' in the fragment."
    fi
    errors=true
  done
done

if [[ "${errors}" == "true" ]]; then
  exit 1
fi

echo "Regression check passed."
