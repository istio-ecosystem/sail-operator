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
	"fmt"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"istio.io/istio/pkg/ptr"
)

var log = ctrl.Log.WithName("netpol")

// CreateIstiodNetworkPolicy creates or updates a NetworkPolicy for istiod
func CreateIstiodNetworkPolicy(ctx context.Context, cl client.Client, rev *v1.IstioRevision) error {
	np := BuildIstiodNetworkPolicy(rev)

	// Try to get existing NetworkPolicy
	existing := &networkingv1.NetworkPolicy{}
	err := cl.Get(ctx, types.NamespacedName{
		Name:      np.Name,
		Namespace: np.Namespace,
	}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			if err := cl.Create(ctx, np); err != nil {
				return fmt.Errorf("failed to create NetworkPolicy %s/%s: %w", np.Namespace, np.Name, err)
			}
			log.Info("NetworkPolicy created successfully", "name", np.Name, "namespace", np.Namespace)
			return nil
		}
		return fmt.Errorf("failed to get NetworkPolicy %s/%s: %w", np.Namespace, np.Name, err)
	}

	// Update existing NetworkPolicy if needed
	np.ResourceVersion = existing.ResourceVersion
	if err := cl.Update(ctx, np); err != nil {
		return fmt.Errorf("failed to update NetworkPolicy %s/%s: %w", np.Namespace, np.Name, err)
	}
	log.Info("NetworkPolicy updated successfully", "name", np.Name, "namespace", np.Namespace)

	return nil
}

// DeleteIstiodNetworkPolicy deletes a NetworkPolicy for istiod
func DeleteIstiodNetworkPolicy(ctx context.Context, cl client.Client, rev *v1.IstioRevision) error {
	np := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetIstiodNetworkPolicyName(rev),
			Namespace: rev.Spec.Namespace,
		},
	}

	err := cl.Delete(ctx, np)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete NetworkPolicy %s/%s: %w", np.Namespace, np.Name, err)
	}
	if err == nil {
		log.Info("NetworkPolicy deleted successfully", "name", np.Name, "namespace", np.Namespace)
	}
	return nil
}

// GetIstiodNetworkPolicyName returns the NetworkPolicy name for an IstioRevision
func GetIstiodNetworkPolicyName(rev *v1.IstioRevision) string {
	name := "istio-istiod"
	if rev.Spec.Values != nil && rev.Spec.Values.Revision != nil && *rev.Spec.Values.Revision != "" {
		name += "-" + *rev.Spec.Values.Revision
	}
	return name
}

// BuildIstiodNetworkPolicy creates the NetworkPolicy object for istiod
func BuildIstiodNetworkPolicy(rev *v1.IstioRevision) *networkingv1.NetworkPolicy {
	name := GetIstiodNetworkPolicyName(rev)
	appLabel := constants.IstiodChartName

	ownerReference := metav1.OwnerReference{
		APIVersion:         v1.GroupVersion.String(),
		Kind:               v1.IstioRevisionKind,
		Name:               rev.Name,
		UID:                rev.UID,
		Controller:         ptr.Of(true),
		BlockOwnerDeletion: ptr.Of(true),
	}

	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       rev.Spec.Namespace,
			OwnerReferences: []metav1.OwnerReference{ownerReference},
			Labels: map[string]string{
				"app": appLabel,
			},
		},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": appLabel,
				},
			},
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
			Ingress: buildIstiodIngressRules(),
			Egress:  buildIstiodEgressRules(),
		},
	}
}

// buildIstiodIngressRules creates the ingress rules for istiod NetworkPolicy
// Rule 0: Webhook from kube-apiserver (port 15017)
// Rule 1: xDS from all namespaces (ports 15010, 15011, 15012, 8080, 15014)
// Rule 2: Allow from Kiali in same namespace (ports 8080, 15014)
func buildIstiodIngressRules() []networkingv1.NetworkPolicyIngressRule {
	port15017 := intstr.FromInt(15017)
	port15010 := intstr.FromInt(15010)
	port15011 := intstr.FromInt(15011)
	port15012 := intstr.FromInt(15012)
	port8080 := intstr.FromInt(8080)
	port15014 := intstr.FromInt(15014)

	return []networkingv1.NetworkPolicyIngressRule{
		{
			// Webhook from kube-apiserver
			From: []networkingv1.NetworkPolicyPeer{
				{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"kubernetes.io/metadata.name": "kube-system",
						},
					},
				},
			},
			Ports: []networkingv1.NetworkPolicyPort{
				{
					Protocol: ptr.Of(corev1.ProtocolTCP),
					Port:     &port15017,
				},
			},
		},
		{
			// xDS from all namespaces
			From: []networkingv1.NetworkPolicyPeer{
				{
					NamespaceSelector: &metav1.LabelSelector{},
				},
			},
			Ports: []networkingv1.NetworkPolicyPort{
				{
					Protocol: ptr.Of(corev1.ProtocolTCP),
					Port:     &port15010,
				},
				{
					Protocol: ptr.Of(corev1.ProtocolTCP),
					Port:     &port15011,
				},
				{
					Protocol: ptr.Of(corev1.ProtocolTCP),
					Port:     &port15012,
				},
				{
					Protocol: ptr.Of(corev1.ProtocolTCP),
					Port:     &port8080,
				},
				{
					Protocol: ptr.Of(corev1.ProtocolTCP),
					Port:     &port15014,
				},
			},
		},
		{
			// Redundant, but keeping for clarity and potential future use.
			From: []networkingv1.NetworkPolicyPeer{
				{
					PodSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app.kubernetes.io/name": "kiali",
						},
					},
				},
			},
			Ports: []networkingv1.NetworkPolicyPort{
				{
					Protocol: ptr.Of(corev1.ProtocolTCP),
					Port:     &port8080,
				},
				{
					Protocol: ptr.Of(corev1.ProtocolTCP),
					Port:     &port15014,
				},
			},
		},
	}
}

// buildIstiodEgressRules creates the egress rules for istiod NetworkPolicy
func buildIstiodEgressRules() []networkingv1.NetworkPolicyEgressRule {
	rules := []networkingv1.NetworkPolicyEgressRule{
		buildKubernetesAPIRule(),
		buildExternalHTTPSRule(),
		buildOpenTelemetryRule(),
	}

	rules = append(rules, buildOpenShiftDNSRule(), buildKubernetesDNSRule())

	return rules
}

// buildOpenShiftDNSRule creates DNS egress rule for OpenShift
func buildOpenShiftDNSRule() networkingv1.NetworkPolicyEgressRule {
	port53 := intstr.FromInt(53)

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
				Protocol: ptr.Of(corev1.ProtocolUDP),
				Port:     &port53,
			},
			{
				Protocol: ptr.Of(corev1.ProtocolTCP),
				Port:     &port53,
			},
		},
	}
}

// buildKubernetesDNSRule creates DNS egress rule for standard Kubernetes
func buildKubernetesDNSRule() networkingv1.NetworkPolicyEgressRule {
	port53 := intstr.FromInt(53)

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
				Protocol: ptr.Of(corev1.ProtocolTCP),
				Port:     &port53,
			},
			{
				Protocol: ptr.Of(corev1.ProtocolUDP),
				Port:     &port53,
			},
		},
	}
}

// buildKubernetesAPIRule creates Kubernetes API egress rule
func buildKubernetesAPIRule() networkingv1.NetworkPolicyEgressRule {
	port6443 := intstr.FromInt(6443)
	port443 := intstr.FromInt(443)

	return networkingv1.NetworkPolicyEgressRule{
		To: []networkingv1.NetworkPolicyPeer{},
		Ports: []networkingv1.NetworkPolicyPort{
			{
				Protocol: ptr.Of(corev1.ProtocolTCP),
				Port:     &port6443,
			},
			{
				Protocol: ptr.Of(corev1.ProtocolTCP),
				Port:     &port443,
			},
		},
	}
}

// buildExternalHTTPSRule creates external HTTPS egress rule
func buildExternalHTTPSRule() networkingv1.NetworkPolicyEgressRule {
	port443 := intstr.FromInt(443)

	return networkingv1.NetworkPolicyEgressRule{
		To: []networkingv1.NetworkPolicyPeer{
			{
				IPBlock: &networkingv1.IPBlock{
					CIDR: "0.0.0.0/0",
				},
			},
		},
		Ports: []networkingv1.NetworkPolicyPort{
			{
				Protocol: ptr.Of(corev1.ProtocolTCP),
				Port:     &port443,
			},
		},
	}
}

// buildOpenTelemetryRule creates OpenTelemetry collector egress rule
func buildOpenTelemetryRule() networkingv1.NetworkPolicyEgressRule {
	port4317 := intstr.FromInt(4317)

	return networkingv1.NetworkPolicyEgressRule{
		To: []networkingv1.NetworkPolicyPeer{
			{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"kubernetes.io/metadata.name": "opentelemetrycollector",
					},
				},
			},
		},
		Ports: []networkingv1.NetworkPolicyPort{
			{
				Protocol: ptr.Of(corev1.ProtocolTCP),
				Port:     &port4317,
			},
		},
	}
}
