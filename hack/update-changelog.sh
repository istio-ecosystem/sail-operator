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

# Collects changelog fragment files from changelog/ into a versioned section
# in CHANGELOG.md, then deletes the fragments.
#
# Usage:
#   ./hack/update-changelog.sh           # auto-detect version from Makefile.core.mk
#   ./hack/update-changelog.sh 1.30.1    # specify version explicitly

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
CHANGELOG="${REPO_ROOT}/CHANGELOG.md"
FRAGMENT_DIR="${REPO_ROOT}/changelog"

if [[ ! -f "$CHANGELOG" ]]; then
  echo "Error: CHANGELOG.md not found at ${CHANGELOG}" >&2
  exit 1
fi

if [[ $# -ge 1 ]]; then
  VERSION="$1"
else
  VERSION=$(grep '^VERSION ' "${REPO_ROOT}/Makefile.core.mk" | head -1 | sed 's/.*?= *//')
  if [[ -z "$VERSION" ]]; then
    echo "Error: Could not detect VERSION from Makefile.core.mk. Pass version as argument." >&2
    exit 1
  fi
fi

DATE=$(date +%Y-%m-%d)

# Collect all fragment files
fragments=()
for f in "${FRAGMENT_DIR}"/*.yaml; do
  [[ -f "$f" ]] && fragments+=("$f")
done

if [[ ${#fragments[@]} -eq 0 ]]; then
  echo "Warning: No changelog fragments found in ${FRAGMENT_DIR}/. Nothing to collect." >&2
  exit 0
fi

# Build the versioned section, grouped by category
section="## v${VERSION} - ${DATE}"

for category in added changed fixed removed; do
  entries=""
  for f in "${fragments[@]}"; do
    fcat=$(grep '^category:' "$f" | sed 's/^category:[[:space:]]*//')
    if [[ "$fcat" != "$category" ]]; then
      continue
    fi

    title=$(grep '^title:' "$f" | sed 's/^title:[[:space:]]*//')
    issue_link=$(grep '^issueLink:' "$f" | sed 's/^issueLink:[[:space:]]*//' || true)

    # Extract description (multi-line block scalar after "description: |" or single line)
    description=""
    if grep -q '^description:' "$f"; then
      desc_line=$(grep '^description:' "$f")
      if echo "$desc_line" | grep -q '^description:[[:space:]]*|'; then
        # Block scalar: read indented lines after "description: |"
        description=$(sed -n '/^description:/,/^[^ ]/{ /^description:/d; /^[^ ]/d; p; }' "$f" | sed 's/^  //')
      else
        # Inline value
        description="${desc_line#description:}"
        description="${description#"${description%%[![:space:]]*}"}"
      fi
    fi

    # Format entry
    entry="- ${title}"
    if [[ -n "$issue_link" ]]; then
      # Extract issue number from URL
      issue_num=$(echo "$issue_link" | grep -oP '/issues/\K[0-9]+' || true)
      if [[ -n "$issue_num" ]]; then
        entry="${entry} ([#${issue_num}](${issue_link}))"
      fi
    fi

    if [[ -n "$description" ]]; then
      # Indent description lines with 2 spaces
      indented=$(echo "$description" | sed '/^[[:space:]]*$/d' | sed 's/^/  /')
      entry="${entry}"$'\n'"${indented}"
    fi

    if [[ -n "$entries" ]]; then
      entries="${entries}"$'\n\n'"${entry}"
    else
      entries="${entry}"
    fi
  done

  if [[ -n "$entries" ]]; then
    # Capitalize category name for heading
    heading="${category^}"
    section="${section}"$'\n\n'"### ${heading}"$'\n'"${entries}"
  fi
done

# Insert the new section before the first ## v line in CHANGELOG.md
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

# Delete collected fragments (keep .gitkeep)
for f in "${fragments[@]}"; do
  rm "$f"
done

echo "Collected ${#fragments[@]} fragment(s) into '## v${VERSION} - ${DATE}' in CHANGELOG.md."
