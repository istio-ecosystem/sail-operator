// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1

import (
	k8sv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type SDSConfigToken struct {
	Aud string `json:"aud,omitempty"`
}

type CNIValues struct {
	// Configuration for the Istio CNI plugin.
	Cni *CNIConfig `json:"cni,omitempty"`

	// Part of the global configuration applicable to the Istio CNI component.
	Global *CNIGlobalConfig `json:"global,omitempty"`
}

// Configuration for CNI.
type CNIConfig struct {
	// Hub to pull the container image from. Image will be `Hub/Image:Tag-Variant`.
	Hub *string `json:"hub,omitempty"`
	// The container image tag to pull. Image will be `Hub/Image:Tag-Variant`.
	Tag *string `json:"tag,omitempty"`
	// The container image variant to pull. Options are "debug" or "distroless". Unset will use the default for the given version.
	Variant *string `json:"variant,omitempty"`
	// Image name to pull from. Image will be `Hub/Image:Tag-Variant`.
	// If Image contains a "/", it will replace the entire `image` in the pod.
	Image *string `json:"image,omitempty"`
	// Specifies the image pull policy. one of Always, Never, IfNotPresent.
	// Defaults to Always if :latest tag is specified, or IfNotPresent otherwise. Cannot be updated.
	//
	// More info: https://kubernetes.io/docs/concepts/containers/images#updating-images
	// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
	PullPolicy *k8sv1.PullPolicy `json:"pullPolicy,omitempty"`
	// The directory path within the cluster node's filesystem where the CNI binaries are to be installed. Typically /var/lib/cni/bin.
	CniBinDir *string `json:"cniBinDir,omitempty"`
	// The directory path within the cluster node's filesystem where the CNI configuration files are to be installed. Typically /etc/cni/net.d.
	CniConfDir *string `json:"cniConfDir,omitempty"`
	// The name of the CNI plugin configuration file. Defaults to istio-cni.conf.
	CniConfFileName *string `json:"cniConfFileName,omitempty"`
	// The directory path within the cluster node's filesystem where network namespaces are located.
	// Defaults to '/var/run/netns', in minikube/docker/others can be '/var/run/docker/netns'.
	CniNetnsDir *string `json:"cniNetnsDir,omitempty"`
	// List of namespaces that should be ignored by the CNI plugin.
	ExcludeNamespaces []string `json:"excludeNamespaces,omitempty"`
	// K8s affinity to set on the istio-cni Pods. Can be used to exclude istio-cni from being scheduled on specified nodes.
	Affinity *k8sv1.Affinity `json:"affinity,omitempty"`
	// Additional annotations to apply to the istio-cni Pods.
	//
	// Deprecated: Marked as deprecated in pkg/apis/values_types.proto.
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`
	// PodSecurityPolicy cluster role. No longer used anywhere.
	PspClusterRole *string `json:"psp_cluster_role,omitempty"`
	// A `key: value` mapping of environment variables to add to the pod
	Env map[string]string `json:"env,omitempty"`
	// Same as `global.logging.level`, but will override it if set
	Logging *GlobalLoggingConfig `json:"logging,omitempty"`
	// Configuration for the CNI Repair controller.
	Repair *CNIRepairConfig `json:"repair,omitempty"`
	// Configure the plugin as a chained CNI plugin. When true, the configuration is added to the CNI chain; when false,
	// the configuration is added as a standalone file in the CNI configuration directory.
	Chained *bool `json:"chained,omitempty"`
	// The resource quotas configuration for the CNI DaemonSet.
	ResourceQuotas *ResourceQuotas `json:"resource_quotas,omitempty"`
	// The k8s resource requests and limits for the istio-cni Pods.
	Resources *k8sv1.ResourceRequirements `json:"resources,omitempty"`
	// No longer used for CNI. See: https://github.com/istio/istio/issues/49004
	//
	// Deprecated: Marked as deprecated in pkg/apis/values_types.proto.
	Privileged *bool `json:"privileged,omitempty"`
	// The Container seccompProfile
	//
	// See: https://kubernetes.io/docs/tutorials/security/seccomp/
	SeccompProfile *k8sv1.SeccompProfile `json:"seccompProfile,omitempty"`
	// Configuration for Istio Ambient.
	Ambient *CNIAmbientConfig `json:"ambient,omitempty"`
	// Specifies the CNI provider. Can be either "default" or "multus". When set to "multus", an additional
	// NetworkAttachmentDefinition resource is deployed to the cluster to allow the istio-cni plugin to be
	// invoked in a cluster using the Multus CNI plugin.
	Provider *string `json:"provider,omitempty"`
	// The number of pods that can be unavailable during a rolling update of the CNI DaemonSet (see
	// `updateStrategy.rollingUpdate.maxUnavailable` here:
	// https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/daemon-set-v1/#DaemonSetSpec).
	// May be specified as a number of pods or as a percent of the total number
	// of pods at the start of the update.
	// +kubebuilder:validation:XIntOrString
	RollingMaxUnavailable *intstr.IntOrString `json:"rollingMaxUnavailable,omitempty"`
}

type CNIGlobalConfig struct { // Default k8s resources settings for all Istio control plane components.
	//
	// See https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#resource-requests-and-limits-of-pod-and-container
	//
	// Deprecated: Marked as deprecated in pkg/apis/values_types.proto.
	DefaultResources *k8sv1.ResourceRequirements `json:"defaultResources,omitempty"`

	// Specifies the docker hub for Istio images.
	Hub *string `json:"hub,omitempty"`
	// Specifies the image pull policy for the Istio images. one of Always, Never, IfNotPresent.
	// Defaults to Always if :latest tag is specified, or IfNotPresent otherwise. Cannot be updated.
	//
	// More info: https://kubernetes.io/docs/concepts/containers/images#updating-images
	// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
	ImagePullPolicy *k8sv1.PullPolicy `json:"imagePullPolicy,omitempty"`
	// ImagePullSecrets for the control plane ServiceAccount, list of secrets in the same namespace
	// to use for pulling any images in pods that reference this ServiceAccount.
	// Must be set for any cluster configured with private docker registry.
	ImagePullSecrets []string `json:"imagePullSecrets,omitempty"`

	// Specifies whether istio components should output logs in json format by adding --log_as_json argument to each container.
	LogAsJSON *bool `json:"logAsJson,omitempty"`
	// Specifies the global logging level settings for the Istio control plane components.
	Logging *GlobalLoggingConfig `json:"logging,omitempty"`

	// Specifies the tag for the Istio docker images.
	Tag *string `json:"tag,omitempty"`
	// The variant of the Istio container images to use. Options are "debug" or "distroless". Unset will use the default for the given version.
	Variant *string `json:"variant,omitempty"`

	// Platform in which Istio is deployed. Possible values are: "openshift" and "gcp"
	// An empty value means it is a vanilla Kubernetes distribution, therefore no special
	// treatment will be considered.
	Platform *string `json:"platform,omitempty"`
}

type ZTunnelValues struct {
	// Configuration for the Istio ztunnel plugin.
	ZTunnel *ZTunnelConfig `json:"ztunnel,omitempty"`

	// Part of the global configuration applicable to the Istio ztunnel component.
	Global *ZTunnelGlobalConfig `json:"global,omitempty"`
}

// Configuration for ztunnel.
type ZTunnelConfig struct {
	// Hub to pull the container image from. Image will be `Hub/Image:Tag-Variant`.
	Hub *string `json:"hub,omitempty"`
	// The container image tag to pull. Image will be `Hub/Image:Tag-Variant`.
	Tag *string `json:"tag,omitempty"`
	// The container image variant to pull. Options are "debug" or "distroless". Unset will use the default for the given version.
	Variant *string `json:"variant,omitempty"`
	// Image name to pull from. Image will be `Hub/Image:Tag-Variant`.
	// If Image contains a "/", it will replace the entire `image` in the pod.
	Image *string `json:"image,omitempty"`
	// resourceName, if set, will override the naming of resources. If not set, will default to the release name.
	// It is recommended to not set this; this is primarily for backwards compatibility.
	ResourceName *string `json:"resourceName,omitempty"`
	// Annotations to apply to all top level resources
	Annotations map[string]string `json:"Annotations,omitempty"`
	// Labels to apply to all top level resources
	Labels map[string]string `json:"Labels,omitempty"`
	// Additional volumeMounts to the ztunnel container
	VolumeMounts []k8sv1.VolumeMount `json:"volumeMounts,omitempty"`
	// Additional volumes to add to the ztunnel Pod.
	Volumes []k8sv1.Volume `json:"volumes,omitempty"`
	// Annotations added to each pod. The default annotations are required for scraping prometheus (in most environments).
	PodAnnotations map[string]string `json:"podAnnotations,omitempty"`
	// Additional labels to apply on the pod level.
	PodLabels map[string]string `json:"podLabels,omitempty"`
	// The k8s resource requests and limits for the ztunnel Pods.
	Resources *k8sv1.ResourceRequirements `json:"resources,omitempty"`
	// List of secret names to add to the service account as image pull secrets
	// to use for pulling any images in pods that reference this ServiceAccount.
	// Must be set for any cluster configured with private docker registry.
	ImagePullSecrets []string `json:"imagePullSecrets,omitempty"`
	// A `key: value` mapping of environment variables to add to the pod
	Env map[string]string `json:"env,omitempty"`
	// Specifies the image pull policy for the Istio images. one of Always, Never, IfNotPresent.
	// Defaults to Always if :latest tag is specified, or IfNotPresent otherwise. Cannot be updated.
	//
	// More info: https://kubernetes.io/docs/concepts/containers/images#updating-images
	// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
	ImagePullPolicy *k8sv1.PullPolicy `json:"imagePullPolicy,omitempty"`
	// Settings for multicluster.
	// The name of the cluster we are installing in. Note this is a user-defined name, which must be consistent
	// with Istiod configuration.
	MultiCluster *MultiClusterConfig `json:"multiCluster,omitempty"`
	// meshConfig defines runtime configuration of components.
	// For ztunnel, only defaultConfig is used, but this is nested under `meshConfig` for consistency with other components.
	MeshConfig *MeshConfig `json:"meshConfig,omitempty"`
	// This value defines:
	// 1. how many seconds kube waits for ztunnel pod to gracefully exit before forcibly terminating it (this value)
	// 2. how many seconds ztunnel waits to drain its own connections (this value - 1 sec)
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default=30
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty"`
	// Configures the revision this control plane is a part of
	Revision *string `json:"revision,omitempty"`
	// The address of the CA for CSR.
	CaAddress *string `json:"caAddress,omitempty"`
	// The customized XDS address to retrieve configuration.
	XdsAddress *string `json:"xdsAddress,omitempty"`
	// Specifies the default namespace for the Istio control plane components.
	IstioNamespace *string `json:"istioNamespace,omitempty"`
	// Configuration log level of ztunnel binary, default is info. Valid values are: trace, debug, info, warn, error.
	// +kubebuilder:default=info
	// +kubebuilder:validation:Enum=trace;debug;info;warn;error;
	LogLevel *string `json:"logLevel,omitempty"`
	// Specifies whether istio components should output logs in json format by adding --log_as_json argument to each container.
	LogAsJSON *bool `json:"logAsJson,omitempty"`
}

// ZTunnelGlobalConfig is a subset of the Global Configuration used in the Istio ztunnel chart.
type ZTunnelGlobalConfig struct { // Default k8s resources settings for all Istio control plane components.
	//
	// See https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#resource-requests-and-limits-of-pod-and-container
	//
	// Deprecated: Marked as deprecated in pkg/apis/values_types.proto.
	DefaultResources *k8sv1.ResourceRequirements `json:"defaultResources,omitempty"`

	// Specifies the docker hub for Istio images.
	Hub *string `json:"hub,omitempty"`
	// Specifies the image pull policy for the Istio images. one of Always, Never, IfNotPresent.
	// Defaults to Always if :latest tag is specified, or IfNotPresent otherwise. Cannot be updated.
	//
	// More info: https://kubernetes.io/docs/concepts/containers/images#updating-images
	// +kubebuilder:validation:Enum=Always;Never;IfNotPresent
	ImagePullPolicy *k8sv1.PullPolicy `json:"imagePullPolicy,omitempty"`
	// ImagePullSecrets for the control plane ServiceAccount, list of secrets in the same namespace
	// to use for pulling any images in pods that reference this ServiceAccount.
	// Must be set for any cluster configured with private docker registry.
	ImagePullSecrets []string `json:"imagePullSecrets,omitempty"`

	// Specifies whether istio components should output logs in json format by adding --log_as_json argument to each container.
	LogAsJSON *bool `json:"logAsJson,omitempty"`
	// Specifies the global logging level settings for the Istio control plane components.
	Logging *GlobalLoggingConfig `json:"logging,omitempty"`

	// Specifies the tag for the Istio docker images.
	Tag *string `json:"tag,omitempty"`
	// The variant of the Istio container images to use. Options are "debug" or "distroless". Unset will use the default for the given version.
	Variant *string `json:"variant,omitempty"`

	// Platform in which Istio is deployed. Possible values are: "openshift" and "gcp"
	// An empty value means it is a vanilla Kubernetes distribution, therefore no special
	// treatment will be considered.
	Platform *string `json:"platform,omitempty"`
}
