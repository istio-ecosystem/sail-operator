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
# ServiceMonitor and PodMonitor CRDs are provided by the chart's Prometheus Operator.
# Prometheus uses the chart defaults (release label selectors); Sail Istio CR
# spec.monitoring.monitoredBy must match the helm release name for discovery.

set -eu -o pipefail

PROM_NAMESPACE="${PROM_NAMESPACE:-monitoring}"
PROM_RELEASE="${PROM_RELEASE:-kube-prometheus-stack}"
WAIT_TIMEOUT="${WAIT_TIMEOUT:-5m}"

if helm status "${PROM_RELEASE}" -n "${PROM_NAMESPACE}" &>/dev/null; then
  echo "kube-prometheus-stack already installed in ${PROM_NAMESPACE}"
  exit 0
fi

helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

# Install without --wait; readiness is verified via targeted kubectl waits below.
helm install "${PROM_RELEASE}" prometheus-community/kube-prometheus-stack \
  --namespace "${PROM_NAMESPACE}" \
  --create-namespace \
  --set grafana.enabled=false \
  --set alertmanager.enabled=false

kubectl wait --for=condition=Established crd/servicemonitors.monitoring.coreos.com --timeout="${WAIT_TIMEOUT}"
kubectl wait --for=condition=Established crd/podmonitors.monitoring.coreos.com --timeout="${WAIT_TIMEOUT}"
kubectl wait --for=condition=available "deployment/${PROM_RELEASE}-operator" -n "${PROM_NAMESPACE}" --timeout="${WAIT_TIMEOUT}"
kubectl rollout status "statefulset/prometheus-${PROM_RELEASE}-prometheus" -n "${PROM_NAMESPACE}" --timeout="${WAIT_TIMEOUT}"

echo "kube-prometheus-stack is ready in namespace ${PROM_NAMESPACE}"
