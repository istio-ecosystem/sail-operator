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

# Installs kube-prometheus-stack on KinD for monitoring controller e2e tests.
# Prometheus is configured to discover all ServiceMonitors and PodMonitors, matching
# the KinD validation approach documented in SEP3 / PR #2028.

set -eu -o pipefail

PROM_NAMESPACE="${PROM_NAMESPACE:-monitoring}"
PROM_RELEASE="${PROM_RELEASE:-kube-prometheus-stack}"

if helm status "${PROM_RELEASE}" -n "${PROM_NAMESPACE}" &>/dev/null; then
  echo "kube-prometheus-stack already installed in ${PROM_NAMESPACE}"
  exit 0
fi

helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

helm install "${PROM_RELEASE}" prometheus-community/kube-prometheus-stack \
  --namespace "${PROM_NAMESPACE}" \
  --create-namespace \
  --set grafana.enabled=false \
  --set alertmanager.enabled=false \
  --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false \
  --set prometheus.prometheusSpec.podMonitorSelectorNilUsesHelmValues=false \
  --wait --timeout 15m

kubectl wait --for=condition=available "deployment/${PROM_RELEASE}-operator" -n "${PROM_NAMESPACE}" --timeout=10m
kubectl rollout status "statefulset/prometheus-${PROM_RELEASE}-prometheus" -n "${PROM_NAMESPACE}" --timeout=10m

echo "kube-prometheus-stack is ready in namespace ${PROM_NAMESPACE}"
