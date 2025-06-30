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

echo "This script set up the whole OSSM3 demo."

echo "Installing Minio for Tempo"
oc new-project tracing-system
oc apply -f ./resources/TempoOtel/minio.yaml -n tracing-system
oc wait --for condition=Available deployment/minio --timeout 150s -n tracing-system
echo "Installing TempoCR"
oc apply -f ./resources/TempoOtel/tempo.yaml -n tracing-system
oc wait --for condition=Ready TempoStack/sample --timeout 150s -n tracing-system
oc wait --for condition=Available deployment/tempo-sample-compactor --timeout 150s -n tracing-system
echo "Exposing Jaeger UI route (will be used in kiali ui)"
oc expose svc tempo-sample-query-frontend --port=jaeger-ui --name=tracing-ui -n tracing-system

echo "Installing OpenTelemetryCollector..."
oc new-project opentelemetrycollector
oc apply -f ./resources/TempoOtel/opentelemetrycollector.yaml -n opentelemetrycollector
oc wait --for condition=Available deployment/otel-collector --timeout 60s -n opentelemetrycollector

echo "Installing OSSM3..."
oc new-project istio-system
echo "Installing IstioCR..."
oc apply -f ./resources/OSSM3/istiocr.yaml  -n istio-system
oc wait --for condition=Ready istio/default --timeout 60s  -n istio-system
echo "Installing Telemetry resource..."
oc apply -f ./resources/TempoOtel/istioTelemetry.yaml  -n istio-system
echo "Adding OTEL namespace as a part of the mesh"
oc label namespace opentelemetrycollector istio-injection=enabled

echo "Installing IstioCNI..."
oc new-project istio-cni
oc apply -f ./resources/OSSM3/istioCni.yaml -n istio-cni
oc wait --for condition=Ready istiocni/default --timeout 60s -n istio-cni

echo "Creating ingress gateway..."
oc new-project istio-ingress
echo "Adding istio-ingress namespace as a part of the mesh"
oc label namespace istio-ingress istio-injection=enabled
oc apply -f ./resources/OSSM3/istioIngressGateway.yaml  -n istio-ingress
oc wait --for condition=Available deployment/istio-ingressgateway --timeout 60s -n istio-ingress
echo "Exposing Istio ingress route"
oc expose svc istio-ingressgateway --port=http2 --name=istio-ingressgateway -n istio-ingress

echo "Enabling user workload monitoring in OCP"
oc apply -f ./resources/Monitoring/ocpUserMonitoring.yaml
echo "Enabling service monitor in istio-system namespace"
oc apply -f ./resources/Monitoring/serviceMonitor.yaml -n istio-system
echo "Enabling pod monitor in istio-system namespace"
oc apply -f ./resources/Monitoring/podMonitor.yaml -n istio-system
echo "Enabling pod monitor in istio-ingress namespace"
oc apply -f ./resources/Monitoring/podMonitor.yaml -n istio-ingress

echo "Installing Kiali..."
oc project istio-system
echo "Creating cluster role binding for kiali to read ocp monitoring"
oc apply -f ./resources/Kiali/kialiCrb.yaml -n istio-system
echo "Installing KialiCR..."
TRACING_INGRESS_ROUTE="http://$(oc get -n tracing-system route tracing-ui -o jsonpath='{.spec.host}')"
export TRACING_INGRESS_ROUTE
< ./resources/Kiali/kialiCr.yaml JAEGERROUTE="${TRACING_INGRESS_ROUTE}" envsubst | oc -n istio-system apply -f - 
oc wait --for condition=Successful kiali/kiali --timeout 150s -n istio-system 
oc annotate route kiali haproxy.router.openshift.io/timeout=60s -n istio-system 

echo "Installing Bookinfo..."
oc new-project bookinfo
echo "Adding bookinfo namespace as a part of the mesh"
oc label namespace bookinfo istio-injection=enabled
echo "Enabling pod monitor in bookinfo namespace"
oc apply -f ./resources/Monitoring/podMonitor.yaml -n bookinfo
echo "Installing Bookinfo"
oc apply -f ./resources/Bookinfo/bookinfo.yaml -n bookinfo
oc apply -f ./resources/Bookinfo/bookinfo-gateway.yaml -n bookinfo
oc wait --for=condition=Ready pods --all -n bookinfo --timeout 60s

echo "Installation finished!"
echo "NOTE: Kiali will show metrics of bookinfo app right after pod monitor will be ready. You can check it in OCP console Observe->Metrics"

echo "[optional] Kiali OSSMC..."
oc apply -f ./resources/Kiali/kialiOssmcCr.yaml -n istio-system
#oc wait -n istio-system --for=condition=Successful OSSMConsole ossmconsole --timeout 120s

# this env will be used in traffic generator
INGRESSHOST=$(oc get route istio-ingressgateway -n istio-ingress -o=jsonpath='{.spec.host}')
export INGRESSHOST
KIALI_HOST=$(oc get route kiali -n istio-system -o=jsonpath='{.spec.host}')

echo "[optional] Installing Bookinfo traffic generator..."
< ./resources/Bookinfo/traffic-generator-configmap.yaml ROUTE="http://${INGRESSHOST}/productpage" envsubst | oc -n bookinfo apply -f - 
oc apply -f ./resources/Bookinfo/traffic-generator.yaml -n bookinfo

echo "===================================================================================================="
echo -e "Ingress route for bookinfo is: \033[1;34mhttp://${INGRESSHOST}/productpage\033[0m"
echo -e "Kiali route is: \033[1;34mhttps://${KIALI_HOST}\033[0m"
echo "===================================================================================================="
