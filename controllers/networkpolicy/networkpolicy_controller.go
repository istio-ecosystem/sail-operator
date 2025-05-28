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

package networkpolicy

import (
	"context"
	"fmt"
	"reflect"

	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/constants"
	"github.com/istio-ecosystem/sail-operator/pkg/reconciler"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type Reconciler struct {
	*reconciler.StandardReconciler[*networkingv1.NetworkPolicy]
	client            client.Client
	scheme            *runtime.Scheme
	config            config.ReconcilerConfig
	operatorNamespace string
}

func NewReconciler(cfg config.ReconcilerConfig, cl client.Client, scheme *runtime.Scheme, operatorNamespace string) *Reconciler {
	r := &Reconciler{
		client:            cl,
		scheme:            scheme,
		config:            cfg,
		operatorNamespace: operatorNamespace,
	}

	r.StandardReconciler = reconciler.NewStandardReconciler[*networkingv1.NetworkPolicy](
		cl,
		r.reconcileNetworkPolicy,
	)

	return r
}

// SetupWithManager sets up the controller with the Manager
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Set up the controller
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&networkingv1.NetworkPolicy{}).
		Complete(r.StandardReconciler); err != nil {
		return fmt.Errorf("failed to set up controller: %w", err)
	}

	// Use a post-start hook to ensure the NetworkPolicy is created
	if err := mgr.Add(&networkPolicyEnsurer{
		reconciler: r,
		mgr:        mgr,
	}); err != nil {
		return fmt.Errorf("failed to add network policy ensurer: %w", err)
	}

	return nil
}

// networkPolicyEnsurer ensures the NetworkPolicy is created
type networkPolicyEnsurer struct {
	reconciler *Reconciler
	mgr        ctrl.Manager
}

func (npe *networkPolicyEnsurer) Start(ctx context.Context) error {
	log := logf.FromContext(ctx).WithName("networkpolicy").WithName("ensurer")

	// Wait for the cache to sync before proceeding
	log.Info("Waiting for cache to sync")
	if !npe.mgr.GetCache().WaitForCacheSync(ctx) {
		return fmt.Errorf("failed to wait for cache sync")
	}

	log.Info("Ensuring NetworkPolicy exists")

	np := npe.reconciler.buildNetworkPolicy()

	// Try to get the NetworkPolicy
	existing := &networkingv1.NetworkPolicy{}
	err := npe.reconciler.client.Get(ctx, client.ObjectKeyFromObject(np), existing)
	if err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to check for existing network policy: %w", err)
		}

		// NetworkPolicy doesn't exist, create it
		log.Info("Creating NetworkPolicy", "name", np.Name, "namespace", np.Namespace)
		if err := npe.reconciler.client.Create(ctx, np); err != nil {
			return fmt.Errorf("failed to create network policy: %w", err)
		}
		log.Info("NetworkPolicy created successfully")
	} else {
		log.V(2).Info("NetworkPolicy already exists", "name", np.Name, "namespace", np.Namespace)
	}

	return nil
}

func (npe *networkPolicyEnsurer) NeedLeaderElection() bool {
	return true
}

// +kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch;delete

// reconcileNetworkPolicy handles reconciliation of NetworkPolicy objects
func (r *Reconciler) reconcileNetworkPolicy(ctx context.Context, np *networkingv1.NetworkPolicy) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Only reconcile our specific NetworkPolicy
	if np.Name != constants.NetworkPolicyName || np.Namespace != r.operatorNamespace {
		return ctrl.Result{}, nil
	}

	log.Info("Reconciling NetworkPolicy", "name", np.Name, "namespace", np.Namespace)

	// Ensure the NetworkPolicy has the correct spec
	desired := r.buildNetworkPolicy()
	if !r.networkPolicySpecsEqual(np.Spec, desired.Spec) {
		log.Info("NetworkPolicy spec differs from desired state, updating")
		np.Spec = desired.Spec
		np.Labels = desired.Labels

		if err := r.client.Update(ctx, np); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to update network policy: %w", err)
		}
		log.Info("NetworkPolicy updated successfully")
	} else {
		log.V(2).Info("NetworkPolicy is up to date")
	}

	return ctrl.Result{}, nil
}

func (r *Reconciler) buildNetworkPolicy() *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.NetworkPolicyName,
			Namespace: r.operatorNamespace,
			Labels:    r.buildLabels(),
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
			Ingress: r.buildIngressRules(),
			Egress:  r.buildEgressRules(),
		},
	}
}

// buildLabels creates the standard labels for the NP
func (r *Reconciler) buildLabels() map[string]string {
	return map[string]string{
		constants.KubernetesAppComponentKey: "sail-operator",
		constants.KubernetesAppNameKey:      constants.NetworkPolicyName,
		constants.KubernetesAppInstanceKey:  "sail-operator",
		constants.KubernetesAppManagedByKey: constants.ManagedByLabelValue,
		constants.KubernetesAppPartOfKey:    constants.KubernetesAppPartOfValue,
		"control-plane":                     "sail-operator",
	}
}

// buildIngressRules creates the ingress rules for the NetworkPolicy
func (r *Reconciler) buildIngressRules() []networkingv1.NetworkPolicyIngressRule {
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
func (r *Reconciler) buildEgressRules() []networkingv1.NetworkPolicyEgressRule {
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

	// Add platform-specific DNS rules
	if r.config.Platform == config.PlatformOpenShift {
		rules = append(rules, r.buildOpenShiftDNSRule())
	} else {
		rules = append(rules, r.buildKubernetesDNSRule())
	}

	return rules
}

// buildOpenShiftDNSRule creates DNS egress rule for OpenShift
func (r *Reconciler) buildOpenShiftDNSRule() networkingv1.NetworkPolicyEgressRule {
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
func (r *Reconciler) buildKubernetesDNSRule() networkingv1.NetworkPolicyEgressRule {
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

// networkPolicySpecsEqual compares two NetworkPolicy specs for equality
func (r *Reconciler) networkPolicySpecsEqual(a, b networkingv1.NetworkPolicySpec) bool {
	return reflect.DeepEqual(a, b)
}
