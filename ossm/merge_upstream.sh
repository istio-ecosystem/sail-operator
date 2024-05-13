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

merge() {
  git remote add -f -t "$MERGE_BRANCH" upstream "$MERGE_REPOSITORY"
  echo "Using branch $MERGE_BRANCH"

  set +e # git returns a non-zero exit code on merge failure, which fails the script
  if [ "${MERGE_STRATEGY}" == "merge" ]; then
    git -c "user.name=$GIT_USERNAME" -c "user.email=$GIT_EMAIL" merge --no-ff -m "$GIT_COMMIT_MESSAGE" --log upstream/"$MERGE_BRANCH"
  else
    git -c "user.name=$GIT_USERNAME" -c "user.email=$GIT_EMAIL" rebase upstream/"$MERGE_BRANCH"
  fi
  return $?
}

main () {  
  if ! merge; then
    set -e
    echo "Conflicts detected, attempting to run 'make gen' to resolve."
    rm -rf bundle/**/*.yaml resources bundle.Dockerfile
    make gen
    git add bundle resources chart bundle.Dockerfile
    git -c "user.name=$GIT_USERNAME" -c "user.email=$GIT_EMAIL" commit --no-edit
  fi
}

main
