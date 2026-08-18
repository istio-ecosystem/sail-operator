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

# Extracts a single version's section from CHANGELOG.md.
# Usage: ./hack/extract-changelog-section.sh 1.30.0
# Output: the content between ## v1.30.0 and the next ## heading (or EOF).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
CHANGELOG="${REPO_ROOT}/CHANGELOG.md"

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <version>" >&2
  echo "Example: $0 1.30.0" >&2
  exit 1
fi

VERSION="$1"

if [[ ! -f "$CHANGELOG" ]]; then
  echo "Error: CHANGELOG.md not found at ${CHANGELOG}" >&2
  exit 1
fi

# Find the line with ## vVERSION (with or without date suffix)
section_line=$(grep -n "^## v${VERSION}" "$CHANGELOG" | head -1 | cut -d: -f1)
if [[ -z "$section_line" ]]; then
  echo "Error: No section found for v${VERSION} in CHANGELOG.md" >&2
  exit 1
fi

# Find the next ## heading after this section
next_line=$(tail -n +"$((section_line + 1))" "$CHANGELOG" | grep -n '^## ' | head -1 | cut -d: -f1)

if [[ -n "$next_line" ]]; then
  end_line=$((section_line + next_line - 1))
  sed -n "$((section_line + 1)),${end_line}p" "$CHANGELOG"
else
  tail -n +"$((section_line + 1))" "$CHANGELOG"
fi
