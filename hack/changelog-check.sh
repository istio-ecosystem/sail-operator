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

# Checks that at least one valid changelog fragment was added under changelog/
# for the diff between BASE_SHA and HEAD.

set -euo pipefail

BASE_SHA="${BASE_SHA:?BASE_SHA environment variable must be set}"

# Check that at least one .yaml file was added in changelog/
added_fragments=$(git diff --name-only --diff-filter=A "${BASE_SHA}...HEAD" -- 'changelog/*.yaml')
if [[ -z "$added_fragments" ]]; then
  echo "::error::No changelog fragment found. Add a YAML file under 'changelog/' (see changelog/README.md for format), or add the 'skip-changelog' label to opt out."
  exit 1
fi

# Validate each added fragment
errors=false
valid_categories="added changed fixed removed"

for fragment in $added_fragments; do
  echo "Validating ${fragment}..."

  # Extract fields using yq
  category=$(yq eval '.category // ""' "$fragment")
  title=$(yq eval '.title // ""' "$fragment")
  issue_link=$(yq eval '.issueLink // ""' "$fragment")

  # Validate category
  if [[ -z "$category" ]]; then
    echo "::error file=${fragment}::Missing 'category' field. Must be one of: ${valid_categories}"
    errors=true
  elif ! echo "$valid_categories" | grep -qw "$category"; then
    echo "::error file=${fragment}::Invalid category '${category}'. Must be one of: ${valid_categories}"
    errors=true
  fi

  # Validate title
  if [[ -z "$title" ]]; then
    echo "::error file=${fragment}::Missing 'title' field."
    errors=true
  fi

  # Validate issueLink for fixed entries
  if [[ "$category" == "fixed" && -z "$issue_link" ]]; then
    echo "::error file=${fragment}::Fixed entries must include an 'issueLink' field with a GitHub issue URL."
    errors=true
  fi
  if [[ -n "$issue_link" ]] && ! echo "$issue_link" | grep -q 'https://github\.com/'; then
    echo "::error file=${fragment}::issueLink must be a GitHub URL (got '${issue_link}')."
    errors=true
  fi
done

if $errors; then
  exit 1
fi

echo "Changelog check passed."
