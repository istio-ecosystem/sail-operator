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

# Installs or uninstalls kube-prometheus-stack for monitoring controller e2e tests.
# ServiceMonitor and PodMonitor CRDs are provided by the chart's Prometheus Operator.
# OpenShift already ships a Prometheus Operator; do not run this on OCP.
#
# Leave serviceMonitorSelectorNilUsesHelmValues / podMonitorSelectorNilUsesHelmValues at
# their chart defaults (true) so e2e matches a typical kube-prometheus-stack install.
# With those defaults, Prometheus only selects monitors that carry a matching
# release: <helm-release-name> label. Sail currently sets release: istio (upstream sample);
# wiring the label expected by kube-prometheus-stack is follow-up work.

set -eux -o pipefail

PROM_NAMESPACE="${PROM_NAMESPACE:-monitoring}"
PROM_RELEASE="${PROM_RELEASE:-kube-prometheus-stack}"
WAIT_TIMEOUT="${WAIT_TIMEOUT:-5m}"
ACTION="${1:-install}"

uninstall() {
  if helm status "${PROM_RELEASE}" -n "${PROM_NAMESPACE}" &>/dev/null; then
    helm uninstall "${PROM_RELEASE}" --namespace "${PROM_NAMESPACE}" --wait --timeout "${WAIT_TIMEOUT}"
  fi
  kubectl delete namespace "${PROM_NAMESPACE}" --ignore-not-found --wait --timeout="${WAIT_TIMEOUT}"
  echo "kube-prometheus-stack uninstalled"
}

if [ "${ACTION}" = "uninstall" ]; then
  uninstall
  exit 0
fi

if [ "${ACTION}" != "install" ]; then
  echo "usage: $0 [install|uninstall]" >&2
  exit 1
fi

if helm status "${PROM_RELEASE}" -n "${PROM_NAMESPACE}" &>/dev/null; then
  echo "kube-prometheus-stack already installed in ${PROM_NAMESPACE}"
  exit 0
fi

helm repo add prometheus-community https://prometheus-community.github.io/helm-charts --force-update
helm repo update

# Admission webhooks require a certgen Job that talks to the webhook Service. On KinD
# that Job frequently times out (the webhook Service is not reachable until kube-proxy
# is healthy), which would also make helm --wait hang. The monitoring e2e only needs CRDs
# and scrape targets, so webhooks are disabled. --wait then tracks the operator Deployment.
helm install "${PROM_RELEASE}" prometheus-community/kube-prometheus-stack \
  --namespace "${PROM_NAMESPACE}" \
  --create-namespace \
  --wait \
  --timeout "${WAIT_TIMEOUT}" \
  --set grafana.enabled=false \
  --set alertmanager.enabled=false \
  --set prometheusOperator.admissionWebhooks.enabled=false \
  --set prometheusOperator.tls.enabled=false

kubectl wait --for=condition=Established crd/servicemonitors.monitoring.coreos.com --timeout="${WAIT_TIMEOUT}"
kubectl wait --for=condition=Established crd/podmonitors.monitoring.coreos.com --timeout="${WAIT_TIMEOUT}"

# The Prometheus Operator creates the StatefulSet asynchronously from the Prometheus CR,
# so helm --wait on the operator Deployment is not enough for scrape-target tests.
prometheus_sts="prometheus-${PROM_RELEASE}-prometheus"
echo "Waiting for StatefulSet/${prometheus_sts} to be created..."
deadline=$((SECONDS + 300))
until kubectl get "statefulset/${prometheus_sts}" -n "${PROM_NAMESPACE}" &>/dev/null; do
  if (( SECONDS >= deadline )); then
    echo "Timed out waiting for StatefulSet/${prometheus_sts}" >&2
    exit 1
  fi
  sleep 2
done
kubectl rollout status "statefulset/${prometheus_sts}" -n "${PROM_NAMESPACE}" --timeout="${WAIT_TIMEOUT}"

echo "kube-prometheus-stack is ready in namespace ${PROM_NAMESPACE}"
