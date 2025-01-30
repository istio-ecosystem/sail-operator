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

set -euo pipefail

if [ -e "$GITHUB_TOKEN_PATH" ]; then
  GITHUB_TOKEN=${GITHUB_TOKEN:-"$(cat "$GITHUB_TOKEN_PATH")"}
fi

GITHUB_TOKEN=${GITHUB_TOKEN:-""}

if [ -z "${GIT_USERNAME:-}" ]; then
  GIT_USERNAME="$(curl -sSfLH "Authorization: token $GITHUB_TOKEN" "https://api.github.com/user" | jq --raw-output ".login")"
fi

if [ -z "${GIT_EMAIL:-}" ]; then
  GIT_EMAIL="$(curl -sSfLH "Authorization: token $GITHUB_TOKEN" "https://api.github.com/user" | jq --raw-output ".email")"
fi

GIT_COMMIT_MESSAGE=${GIT_COMMIT_MESSAGE:-"Automated merge"}

MERGE_STRATEGY=${MERGE_STRATEGY:-"merge"}
MERGE_REPOSITORY=${MERGE_REPOSITORY:-"https://github.com/istio-ecosystem/sail-operator.git"}
MERGE_BRANCH=${MERGE_BRANCH:-"main"}

VERSIONS_YAML_FILE=${VERSIONS_YAML_FILE:-"ossm/versions.yaml"}
HELM_VALUES_FILE=${HELM_VALUES_FILE:-"ossm/values.yaml"}

merge() {
  git remote add -f -t "$MERGE_BRANCH" upstream "$MERGE_REPOSITORY"
  echo "Using branch $MERGE_BRANCH"

  set +e # git returns a non-zero exit code on merge failure, which fails the script
  if [ "${MERGE_STRATEGY}" == "merge" ]; then
    git -c "user.name=$GIT_USERNAME" -c "user.email=$GIT_EMAIL" merge --no-ff -m "$GIT_COMMIT_MESSAGE" --log upstream/"$MERGE_BRANCH" --strategy-option theirs
  else
    git -c "user.name=$GIT_USERNAME" -c "user.email=$GIT_EMAIL" rebase upstream/"$MERGE_BRANCH"
  fi
  return $?
}

updateVersionsInOssmValuesYaml() {
    latest_version=$(yq '.versions[0].version' < "${VERSIONS_YAML_FILE}")
    minor_version=${latest_version%.*}
    latest_version_underscore=${latest_version//./_}
    minor_version_underscore=${minor_version//./_}
    sed -i -e "s/${minor_version}\.[0-9]\+/${latest_version}/g" -e "s/${minor_version_underscore}_[0-9]\+/${latest_version_underscore}/g" "${HELM_VALUES_FILE}"
}

main () {
  merge
  merge_rc=$?
  set -e

  # generate everything regardless of detected conflicts
  rm -rf bundle/**/*.yaml resources bundle.Dockerfile
  updateVersionsInOssmValuesYaml
  make gen
  git add .

  # even when using '--strategy-option theirs' the merge can fail when removing/renaming files
  if [ $merge_rc -eq 0 ]; then
    echo "Conflicts were resolved automatically but we still need to commit possible changes after 'make gen'"
    git diff-index --quiet HEAD || git -c "user.name=$GIT_USERNAME" -c "user.email=$GIT_EMAIL" commit -m "Automated regeneration"
  else
    echo "Conflicts were NOT resolved automatically but 'make gen' passed, finishing the merge"
    git -c "user.name=$GIT_USERNAME" -c "user.email=$GIT_EMAIL" commit --no-edit
  fi

}

main
