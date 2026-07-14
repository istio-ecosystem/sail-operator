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

# Syncs versioned changelog sections from release branches into main's CHANGELOG.md.
# Idempotent — safe to run repeatedly; only adds sections that don't already exist.
#
# Usage:
#   ./hack/sync-changelog-releases.sh                    # auto-detect release branches
#   ./hack/sync-changelog-releases.sh release-1.30       # sync specific branch

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
CHANGELOG="${REPO_ROOT}/CHANGELOG.md"
REMOTE="${UPSTREAM_REMOTE:-upstream}"

if [[ ! -f "$CHANGELOG" ]]; then
  echo "Error: CHANGELOG.md not found at ${CHANGELOG}" >&2
  exit 1
fi

# Collect existing version headers on main
existing_versions=$(grep -oP '^## v\K[0-9]+\.[0-9]+\.[0-9]+' "$CHANGELOG" || true)

if [[ $# -gt 0 ]]; then
  branches=("$@")
else
  git fetch "$REMOTE" 2>/dev/null || true
  mapfile -t branches < <(git branch -r --list "${REMOTE}/release-*" | sed "s|${REMOTE}/||" | sort -V)
fi

added=0

for branch in "${branches[@]}"; do
  remote_ref="${REMOTE}/${branch}"
  if ! git rev-parse --verify "$remote_ref" &>/dev/null; then
    echo "Skipping ${branch}: not found on remote ${REMOTE}" >&2
    continue
  fi

  remote_changelog=$(git show "${remote_ref}:CHANGELOG.md" 2>/dev/null || true)
  if [[ -z "$remote_changelog" ]]; then
    continue
  fi

  # Extract all versioned section headers from the remote branch
  remote_versions=$(echo "$remote_changelog" | grep -oP '^## v\K[0-9]+\.[0-9]+\.[0-9]+' || true)

  for version in $remote_versions; do
    # Skip if already present on main
    if echo "$existing_versions" | grep -qx "$version"; then
      continue
    fi

    # Extract the full section (from ## vX.Y.Z to the next ## or EOF)
    section=$(echo "$remote_changelog" | sed -n "/^## v${version}/,/^## /{/^## v${version}/p; /^## v${version}/!{/^## /!p}}")
    if [[ -z "$section" ]]; then
      continue
    fi

    # Find the right insertion point: after the last version that sorts higher
    # or after ## Next if this is the newest version
    insert_after=""
    while IFS= read -r existing; do
      if [[ -n "$existing" ]] && printf '%s\n%s\n' "$version" "$existing" | sort -V | head -1 | grep -qx "$existing"; then
        # $existing sorts before $version, so insert before $existing
        break
      fi
      insert_after="$existing"
    done <<< "$(grep -oP '^## v\K[0-9]+\.[0-9]+\.[0-9]+' "$CHANGELOG")"

    if [[ -z "$insert_after" ]]; then
      # Insert after ## Next (this version is newer than all existing)
      # Find the line after ## Next's content (the first ## v line)
      first_version_line=$(grep -n '^## v' "$CHANGELOG" | head -1 | cut -d: -f1)
      if [[ -n "$first_version_line" ]]; then
        {
          head -n "$((first_version_line - 1))" "$CHANGELOG"
          echo "$section"
          echo ""
          tail -n +"$first_version_line" "$CHANGELOG"
        } > "${CHANGELOG}.tmp"
        mv "${CHANGELOG}.tmp" "$CHANGELOG"
      else
        # No versioned sections yet, append at end
        echo "" >> "$CHANGELOG"
        echo "$section" >> "$CHANGELOG"
      fi
    else
      # Insert before the version that sorts lower
      insert_line=$(grep -n "^## v${insert_after}" "$CHANGELOG" | head -1 | cut -d: -f1)
      {
        head -n "$((insert_line - 1))" "$CHANGELOG"
        echo "$section"
        echo ""
        tail -n +"$insert_line" "$CHANGELOG"
      } > "${CHANGELOG}.tmp"
      mv "${CHANGELOG}.tmp" "$CHANGELOG"
    fi

    echo "Added v${version} from ${branch}"
    added=$((added + 1))
    # Refresh existing versions after insertion
    existing_versions=$(grep -oP '^## v\K[0-9]+\.[0-9]+\.[0-9]+' "$CHANGELOG" || true)
  done
done

if [[ $added -eq 0 ]]; then
  echo "No new versions to sync."
else
  echo "Synced ${added} version(s) into CHANGELOG.md."
fi
