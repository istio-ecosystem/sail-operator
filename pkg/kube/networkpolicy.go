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

package kube

import (
	"context"

	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
)

var log = ctrl.Log.WithName("netpol")

// CreateNetworkPolicy creates a NetworkPolicy for the sail-operator during startup
// Uses a direct clientset since this runs before the manager cache is started
func CreateNetworkPolicy(cfg *rest.Config, namespace string, platform config.Platform) error {
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}

	networkPolicy := buildNetworkPolicy(namespace, platform)

	ctx := context.Background()

	_, err = clientset.NetworkingV1().NetworkPolicies(namespace).Get(ctx, networkPolicy.Name, metav1.GetOptions{})
	if err != nil {
		_, err = clientset.NetworkingV1().NetworkPolicies(namespace).Create(ctx, networkPolicy, metav1.CreateOptions{})
		if err != nil {
			return err
		}
		log.Info("NetworkPolicy created successfully", "name", networkPolicy.Name, "namespace", namespace)
	}

	return nil
}

// buildNetworkPolicy creates the NetworkPolicy object
func buildNetworkPolicy(namespace string, platform config.Platform) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.NetworkPolicyName,
			Namespace: namespace,
			Labels:    buildLabels(),
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"control-plane": "sail-operator",
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			Ingress: buildIngressRules(),
			Egress:  buildEgressRules(platform),
		},
	}
}

// buildLabels creates standardized labels for the NP
func buildLabels() map[string]string {
	return map[string]string{
		constants.KubernetesAppComponentKey: "sail-operator",
		constants.KubernetesAppNameKey:      constants.NetworkPolicyName,
		constants.KubernetesAppInstanceKey:  "sail-operator",
		constants.KubernetesAppManagedByKey: constants.ManagedByLabelValue,
		constants.KubernetesAppPartOfKey:    constants.KubernetesAppPartOfValue,
		"control-plane":                     "sail-operator",
	}
}

// buildIngressRules creates the ingress rules for the NP
func buildIngressRules() []networkingv1.NetworkPolicyIngressRule {
	tcpProtocol := corev1.ProtocolTCP
	metricsPort := intstr.FromInt(8443)

	return []networkingv1.NetworkPolicyIngressRule{
		{
			// Allow any source to access metrics
			Ports: []networkingv1.NetworkPolicyPort{
				{
					Protocol: &tcpProtocol,
					Port:     &metricsPort,
				},
			},
		},
	}
}

// buildEgressRules creates the egress rules for the NetworkPolicy
func buildEgressRules(platform config.Platform) []networkingv1.NetworkPolicyEgressRule {
	tcpProtocol := corev1.ProtocolTCP
	apiServerPort := intstr.FromInt(6443)

	rules := []networkingv1.NetworkPolicyEgressRule{
		{
			// Allow egress to API Server
			Ports: []networkingv1.NetworkPolicyPort{
				{
					Protocol: &tcpProtocol,
					Port:     &apiServerPort,
				},
			},
		},
	}

	// Add platform specific DNS rules
	if platform == config.PlatformOpenShift {
		rules = append(rules, buildOpenShiftDNSRule())
	} else {
		rules = append(rules, buildKubernetesDNSRule())
	}

	return rules
}

// buildOpenShiftDNSRule creates DNS egress rule for OpenShift
func buildOpenShiftDNSRule() networkingv1.NetworkPolicyEgressRule {
	tcpProtocol := corev1.ProtocolTCP
	udpProtocol := corev1.ProtocolUDP
	dnsPort := intstr.FromString("dns")
	dnsTCPPort := intstr.FromString("dns-tcp")

	return networkingv1.NetworkPolicyEgressRule{
		To: []networkingv1.NetworkPolicyPeer{
			{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"kubernetes.io/metadata.name": "openshift-dns",
					},
				},
				PodSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"dns.operator.openshift.io/daemonset-dns": "default",
					},
				},
			},
		},
		Ports: []networkingv1.NetworkPolicyPort{
			{
				Protocol: &tcpProtocol,
				Port:     &dnsTCPPort,
			},
			{
				Protocol: &udpProtocol,
				Port:     &dnsPort,
			},
		},
	}
}

// buildKubernetesDNSRule creates DNS egress rule for standard Kubernetes
// currently, this is not called, as we only create NP on OpenShift platform. This may be useful in the future.
func buildKubernetesDNSRule() networkingv1.NetworkPolicyEgressRule {
	tcpProtocol := corev1.ProtocolTCP
	udpProtocol := corev1.ProtocolUDP
	dnsPort := intstr.FromInt(53)

	return networkingv1.NetworkPolicyEgressRule{
		To: []networkingv1.NetworkPolicyPeer{
			{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"kubernetes.io/metadata.name": "kube-system",
					},
				},
				PodSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"k8s-app": "kube-dns",
					},
				},
			},
		},
		Ports: []networkingv1.NetworkPolicyPort{
			{
				Protocol: &tcpProtocol,
				Port:     &dnsPort,
			},
			{
				Protocol: &udpProtocol,
				Port:     &dnsPort,
			},
		},
	}
}
