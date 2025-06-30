#!/bin/bash
#
# Copyright 2024 Red Hat, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#	http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

echo "This script installs operators from OperatorHub"

oc apply -f ./resources/subscriptions.yaml
echo "Waiting till all operators pods are ready"
until oc get pods -n openshift-operators | grep servicemesh-operator3 | grep Running; do echo "Waiting for servicemesh-operator3 to be running."; sleep 10;done
until oc get pods -n openshift-operators | grep kiali-operator | grep Running; do echo "Waiting for kiali-operator to be running."; sleep 10;done
until oc get pods -n openshift-operators | grep opentelemetry-operator | grep Running; do echo "Waiting for opentelemetry-operator to be running."; sleep 10;done
until oc get pods -n openshift-operators | grep tempo-operator | grep Running; do echo "Waiting for tempo-operator to be running."; sleep 10;done

echo "All operators were installed successfully"
oc get pods -n openshift-operators