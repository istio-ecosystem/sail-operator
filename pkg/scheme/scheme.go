// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package scheme

import (
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/api/v1alpha1"
	multusv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	configv1 "github.com/openshift/api/config/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	networkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
)

// RhobsAPIGroup is the API group for Cluster Observability Operator (COO) monitoring resources.
// COO uses monitoring.rhobs/v1 instead of monitoring.coreos.com/v1.
const RhobsAPIGroup = "monitoring.rhobs"

var Scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(Scheme))
	utilruntime.Must(multusv1.AddToScheme(Scheme))
	utilruntime.Must(networkingv1alpha3.AddToScheme(Scheme))
	utilruntime.Must(configv1.AddToScheme(Scheme))

	utilruntime.Must(v1alpha1.AddToScheme(Scheme))
	utilruntime.Must(v1.AddToScheme(Scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(Scheme))

	// Register prometheus-operator monitoring types with the rhobs API group.
	// This allows using typed Go objects while targeting the monitoring.rhobs/v1 API
	// which is used by the Cluster Observability Operator (COO) on OpenShift.
	addRhobsMonitoringTypes(Scheme)

	// +kubebuilder:scaffold:scheme
}

// addRhobsMonitoringTypes registers prometheus-operator monitoring types with the monitoring.rhobs API group.
func addRhobsMonitoringTypes(scheme *runtime.Scheme) {
	rhobsGV := schema.GroupVersion{Group: RhobsAPIGroup, Version: "v1"}

	scheme.AddKnownTypes(rhobsGV,
		&monitoringv1.ServiceMonitor{},
		&monitoringv1.ServiceMonitorList{},
		&monitoringv1.PodMonitor{},
		&monitoringv1.PodMonitorList{},
	)
	metav1.AddToGroupVersion(scheme, rhobsGV)
}
