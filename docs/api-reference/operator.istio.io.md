# API Reference

## Packages
- [operator.istio.io/v1alpha1](#operatoristioiov1alpha1)


## operator.istio.io/v1alpha1

Package v1alpha1 contains API Schema definitions for the operator.istio.io v1alpha1 API group

### Resource Types
- [Istio](#istio)
- [IstioCNI](#istiocni)
- [IstioCNIList](#istiocnilist)
- [IstioList](#istiolist)
- [IstioRevision](#istiorevision)
- [IstioRevisionList](#istiorevisionlist)
- [RemoteIstio](#remoteistio)
- [RemoteIstioList](#remoteistiolist)



#### ArchConfig



ArchConfig specifies the pod scheduling target architecture(amd64, ppc64le, s390x, arm64)
for all the Istio control plane components.



_Appears in:_
- [GlobalConfig](#globalconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `amd64` _integer_ | Sets pod scheduling weight for amd64 arch |  |  |
| `ppc64le` _integer_ | Sets pod scheduling weight for ppc64le arch. |  |  |
| `s390x` _integer_ | Sets pod scheduling weight for s390x arch. |  |  |
| `arm64` _integer_ | Sets pod scheduling weight for arm64 arch. |  |  |


#### AuthenticationPolicy

_Underlying type:_ _string_

AuthenticationPolicy defines how the proxy is authenticated when it connects to the control plane.
It can be set for two different scopes, mesh-wide or set on a per-pod basis using the ProxyConfig annotation.
Mesh policy cannot be INHERIT.

_Validation:_
- Enum: [NONE MUTUAL_TLS INHERIT]

_Appears in:_
- [MeshConfigProxyConfig](#meshconfigproxyconfig)

| Field | Description |
| --- | --- |
| `NONE` | Do not encrypt proxy to control plane traffic.  |
| `MUTUAL_TLS` | Proxy to control plane traffic is wrapped into mutual TLS connections.  |
| `INHERIT` | Use the policy defined by the parent scope. Should not be used for mesh policy.  |


#### BaseConfig







_Appears in:_
- [Values](#values)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `validationURL` _string_ | URL to use for validating webhook. |  |  |
| `validationCABundle` _string_ | validation webhook CA bundle |  |  |




#### CNIConfig



Configuration for CNI.



_Appears in:_
- [CNIValues](#cnivalues)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `hub` _string_ | Hub to pull the container image from. Image will be `Hub/Image:Tag-Variant`. |  |  |
| `tag` _string_ | The container image tag to pull. Image will be `Hub/Image:Tag-Variant`. |  |  |
| `variant` _string_ | The container image variant to pull. Options are "debug" or "distroless". Unset will use the default for the given version. |  |  |
| `image` _string_ | Image name to pull from. Image will be `Hub/Image:Tag-Variant`. If Image contains a "/", it will replace the entire `image` in the pod. |  |  |
| `pullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#pullpolicy-v1-core)_ | Specifies the image pull policy. one of Always, Never, IfNotPresent. Defaults to Always if :latest tag is specified, or IfNotPresent otherwise. Cannot be updated.  More info: https://kubernetes.io/docs/concepts/containers/images#updating-images |  | Enum: [Always Never IfNotPresent]   |
| `cniBinDir` _string_ | The directory path within the cluster node's filesystem where the CNI binaries are to be installed. Typically /var/lib/cni/bin. |  |  |
| `cniConfDir` _string_ | The directory path within the cluster node's filesystem where the CNI configuration files are to be installed. Typically /etc/cni/net.d. |  |  |
| `cniConfFileName` _string_ | The name of the CNI plugin configuration file. Defaults to istio-cni.conf. |  |  |
| `cniNetnsDir` _string_ | The directory path within the cluster node's filesystem where network namespaces are located. Defaults to '/var/run/netns', in minikube/docker/others can be '/var/run/docker/netns'. |  |  |
| `excludeNamespaces` _string array_ | List of namespaces that should be ignored by the CNI plugin. |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#affinity-v1-core)_ | K8s affinity to set on the istio-cni Pods. Can be used to exclude istio-cni from being scheduled on specified nodes. |  |  |
| `podAnnotations` _object (keys:string, values:string)_ | Additional annotations to apply to the istio-cni Pods.  Deprecated: Marked as deprecated in pkg/apis/istio/v1alpha1/values_types.proto. |  |  |
| `psp_cluster_role` _string_ | PodSecurityPolicy cluster role. No longer used anywhere. |  |  |
| `logging` _[GlobalLoggingConfig](#globalloggingconfig)_ | Same as `global.logging.level`, but will override it if set |  |  |
| `repair` _[CNIRepairConfig](#cnirepairconfig)_ | Configuration for the CNI Repair controller. |  |  |
| `chained` _boolean_ | Configure the plugin as a chained CNI plugin. When true, the configuration is added to the CNI chain; when false, the configuration is added as a standalone file in the CNI configuration directory. |  |  |
| `resource_quotas` _[ResourceQuotas](#resourcequotas)_ | The resource quotas configration for the CNI DaemonSet. |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#resourcerequirements-v1-core)_ | The k8s resource requests and limits for the istio-cni Pods. |  |  |
| `privileged` _boolean_ | No longer used for CNI. See: https://github.com/istio/istio/issues/49004  Deprecated: Marked as deprecated in pkg/apis/istio/v1alpha1/values_types.proto. |  |  |
| `seccompProfile` _[SeccompProfile](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#seccompprofile-v1-core)_ | The Container seccompProfile  See: https://kubernetes.io/docs/tutorials/security/seccomp/ |  |  |
| `provider` _string_ | Specifies the CNI provider. Can be either "default" or "multus". When set to "multus", an additional NetworkAttachmentDefinition resource is deployed to the cluster to allow the istio-cni plugin to be invoked in a cluster using the Multus CNI plugin. |  |  |
| `rollingMaxUnavailable` _[IntOrString](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#intorstring-intstr-util)_ | The number of pods that can be unavailable during a rolling update of the CNI DaemonSet (see `updateStrategy.rollingUpdate.maxUnavailable` here: https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/daemon-set-v1/#DaemonSetSpec). May be specified as a number of pods or as a percent of the total number of pods at the start of the update. |  | XIntOrString: \{\}   |


#### CNIGlobalConfig



CNIGlobalConfig is a subset of the Global Configuration used in the Istio CNI chart.



_Appears in:_
- [CNIValues](#cnivalues)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `defaultResources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#resourcerequirements-v1-core)_ | See https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#resource-requests-and-limits-of-pod-and-container  Deprecated: Marked as deprecated in pkg/apis/istio/v1alpha1/values_types.proto. |  |  |
| `hub` _string_ | Specifies the docker hub for Istio images. |  |  |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#pullpolicy-v1-core)_ | Specifies the image pull policy for the Istio images. one of Always, Never, IfNotPresent. Defaults to Always if :latest tag is specified, or IfNotPresent otherwise. Cannot be updated.  More info: https://kubernetes.io/docs/concepts/containers/images#updating-images |  | Enum: [Always Never IfNotPresent]   |
| `imagePullSecrets` _string array_ | ImagePullSecrets for the control plane ServiceAccount, list of secrets in the same namespace to use for pulling any images in pods that reference this ServiceAccount. Must be set for any cluster configured with private docker registry. |  |  |
| `logAsJson` _boolean_ | Specifies whether istio components should output logs in json format by adding --log_as_json argument to each container. |  |  |
| `logging` _[GlobalLoggingConfig](#globalloggingconfig)_ | Specifies the global logging level settings for the Istio control plane components. |  |  |
| `tag` _string_ | Specifies the tag for the Istio docker images. |  |  |
| `variant` _string_ | The variant of the Istio container images to use. Options are "debug" or "distroless". Unset will use the default for the given version. |  |  |


#### CNIRepairConfig







_Appears in:_
- [CNIConfig](#cniconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Controls whether repair behavior is enabled. |  |  |
| `hub` _string_ | Hub to pull the container image from. Image will be `Hub/Image:Tag-Variant`. |  |  |
| `tag` _string_ | The container image tag to pull. Image will be `Hub/Image:Tag-Variant`. |  |  |
| `image` _string_ | Image name to pull from. Image will be `Hub/Image:Tag-Variant`. If Image contains a "/", it will replace the entire `image` in the pod. |  |  |
| `labelPods` _boolean_ | The Repair controller has 3 modes (labelPods, deletePods, and repairPods). Pick which one meets your use cases. Note only one may be used. The mode defines the action the controller will take when a pod is detected as broken. If labelPods is true, the controller will label all broken pods with <brokenPodLabelKey>=<brokenPodLabelValue>. This is only capable of identifying broken pods; the user is responsible for fixing them (generally, by deleting them). Note this gives the DaemonSet a relatively high privilege, as modifying pod metadata/status can have wider impacts. |  |  |
| `repairPods` _boolean_ | The Repair controller has 3 modes (labelPods, deletePods, and repairPods). Pick which one meets your use cases. Note only one may be used. The mode defines the action the controller will take when a pod is detected as broken. If repairPods is true, the controller will dynamically repair any broken pod by setting up the pod networking configuration even after it has started. Note the pod will be crashlooping, so this may take a few minutes to become fully functional based on when the retry occurs. This requires no RBAC privilege, but will require the CNI agent to run as a privileged pod. |  |  |
| `createEvents` _string_ | No longer used.  Deprecated: Marked as deprecated in pkg/apis/istio/v1alpha1/values_types.proto. |  |  |
| `deletePods` _boolean_ | The Repair controller has 3 modes (labelPods, deletePods, and repairPods). Pick which one meets your use cases. Note only one may be used. The mode defines the action the controller will take when a pod is detected as broken. If deletePods is true, the controller will delete the broken pod. The pod will then be rescheduled, hopefully onto a node that is fully ready. Note this gives the DaemonSet a relatively high privilege, as it can delete any Pod. |  |  |
| `brokenPodLabelKey` _string_ | The label key to apply to a broken pod when the controller is in labelPods mode. |  |  |
| `brokenPodLabelValue` _string_ | The label value to apply to a broken pod when the controller is in labelPods mode. |  |  |
| `initContainerName` _string_ | The name of the init container to use for the repairPods mode. |  |  |


#### CNIUsageConfig







_Appears in:_
- [PilotConfig](#pilotconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Controls whether CNI should be used. |  |  |
| `provider` _string_ | Specifies the CNI provider. Can be either "default" or "multus". When set to "multus", an annotation `k8s.v1.cni.cncf.io/networks` is set on injected pods to point to a NetworkAttachmentDefinition |  |  |


#### CNIValues







_Appears in:_
- [IstioCNISpec](#istiocnispec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `cni` _[CNIConfig](#cniconfig)_ | Configuration for the Istio CNI plugin. |  |  |
| `global` _[CNIGlobalConfig](#cniglobalconfig)_ | Part of the global configuration applicable to the Istio CNI component. |  |  |




#### ClientTLSSettings

_Underlying type:_ _[struct{Mode ClientTLSSettingsTLSmode "json:\"mode,omitempty\""; ClientCertificate string "json:\"clientCertificate,omitempty\""; PrivateKey string "json:\"privateKey,omitempty\""; CaCertificates string "json:\"caCertificates,omitempty\""; CredentialName string "json:\"credentialName,omitempty\""; SubjectAltNames []string "json:\"subjectAltNames,omitempty\""; Sni string "json:\"sni,omitempty\""; InsecureSkipVerify *bool "json:\"insecureSkipVerify,omitempty\""; CaCrl string "json:\"caCrl,omitempty\""}](#struct{mode-clienttlssettingstlsmode-"json:\"mode,omitempty\"";-clientcertificate-string-"json:\"clientcertificate,omitempty\"";-privatekey-string-"json:\"privatekey,omitempty\"";-cacertificates-string-"json:\"cacertificates,omitempty\"";-credentialname-string-"json:\"credentialname,omitempty\"";-subjectaltnames-[]string-"json:\"subjectaltnames,omitempty\"";-sni-string-"json:\"sni,omitempty\"";-insecureskipverify-*bool-"json:\"insecureskipverify,omitempty\"";-cacrl-string-"json:\"cacrl,omitempty\""})_

SSL/TLS related settings for upstream connections. See Envoy's [TLS
context](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/transport_sockets/tls/v3/common.proto.html#common-tls-configuration)
for more details. These settings are common to both HTTP and TCP upstreams.


For example, the following rule configures a client to use mutual TLS
for connections to upstream database cluster.


```yaml
apiVersion: networking.istio.io/v1
kind: DestinationRule
metadata:
  name: db-mtls
spec:
  host: mydbserver.prod.svc.cluster.local
  trafficPolicy:
    tls:
      mode: MUTUAL
      clientCertificate: /etc/certs/myclientcert.pem
      privateKey: /etc/certs/client_private_key.pem
      caCertificates: /etc/certs/rootcacerts.pem
```


The following rule configures a client to use TLS when talking to a
foreign service whose domain matches *.foo.com.


```yaml
apiVersion: networking.istio.io/v1
kind: DestinationRule
metadata:
  name: tls-foo
spec:
  host: "*.foo.com"
  trafficPolicy:
    tls:
      mode: SIMPLE
```


The following rule configures a client to use Istio mutual TLS when talking
to rating services.


```yaml
apiVersion: networking.istio.io/v1
kind: DestinationRule
metadata:
  name: ratings-istio-mtls
spec:
  host: ratings.prod.svc.cluster.local
  trafficPolicy:
    tls:
      mode: ISTIO_MUTUAL
```



_Appears in:_
- [ConfigSource](#configsource)
- [MeshConfigCA](#meshconfigca)
- [RemoteService](#remoteservice)
- [Tracing](#tracing)





#### ConfigSource



ConfigSource describes information about a configuration store inside a
mesh. A single control plane instance can interact with one or more data
sources.



_Appears in:_
- [MeshConfig](#meshconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `address` _string_ | Address of the server implementing the Istio Mesh Configuration protocol (MCP). Can be IP address or a fully qualified DNS name. Use xds:// to specify a grpc-based xds backend, k8s:// to specify a k8s controller or fs:/// to specify a file-based backend with absolute path to the directory. |  |  |
| `tlsSettings` _[ClientTLSSettings](#clienttlssettings)_ | Use the tls_settings to specify the tls mode to use. If the MCP server uses Istio mutual TLS and shares the root CA with Pilot, specify the TLS mode as `ISTIO_MUTUAL`. |  |  |
| `subscribedResources` _[Resource](#resource) array_ | Describes the source of configuration, if nothing is specified default is MCP |  | Enum: [SERVICE_REGISTRY]   |


#### ConnectionPoolSettingsTCPSettingsTcpKeepalive



TCP keepalive.



_Appears in:_
- [MeshConfig](#meshconfig)
- [RemoteService](#remoteservice)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `probes` _integer_ | Maximum number of keepalive probes to send without response before deciding the connection is dead. Default is to use the OS level configuration (unless overridden, Linux defaults to 9.) |  |  |
| `time` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#duration-v1-meta)_ | The time duration a connection needs to be idle before keep-alive probes start being sent. Default is to use the OS level configuration (unless overridden, Linux defaults to 7200s (ie 2 hours.) |  |  |
| `interval` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#duration-v1-meta)_ | The time duration between keep-alive probes. Default is to use the OS level configuration (unless overridden, Linux defaults to 75s.) |  |  |


#### DefaultPodDisruptionBudgetConfig



DefaultPodDisruptionBudgetConfig specifies the default pod disruption budget configuration.


See https://kubernetes.io/docs/concepts/workloads/pods/disruptions/



_Appears in:_
- [GlobalConfig](#globalconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Controls whether a PodDisruptionBudget with a default minAvailable value of 1 is created for each deployment. |  |  |




#### ForwardClientCertDetails

_Underlying type:_ _string_

ForwardClientCertDetails controls how the x-forwarded-client-cert (XFCC)
header is handled by the gateway proxy.
See [Envoy XFCC](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/filters/network/http_connection_manager/v3/http_connection_manager.proto.html#enum-extensions-filters-network-http-connection-manager-v3-httpconnectionmanager-forwardclientcertdetails)
header handling for more details.

_Validation:_
- Enum: [UNDEFINED SANITIZE FORWARD_ONLY APPEND_FORWARD SANITIZE_SET ALWAYS_FORWARD_ONLY]

_Appears in:_
- [ProxyConfigProxyHeaders](#proxyconfigproxyheaders)
- [Topology](#topology)

| Field | Description |
| --- | --- |
| `UNDEFINED` | Field is not set  |
| `SANITIZE` | Do not send the XFCC header to the next hop. This is the default value.  |
| `FORWARD_ONLY` | When the client connection is mTLS (Mutual TLS), forward the XFCC header in the request.  |
| `APPEND_FORWARD` | When the client connection is mTLS, append the client certificate information to the request’s XFCC header and forward it.  |
| `SANITIZE_SET` | When the client connection is mTLS, reset the XFCC header with the client certificate information and send it to the next hop.  |
| `ALWAYS_FORWARD_ONLY` | Always forward the XFCC header in the request, regardless of whether the client connection is mTLS.  |


#### GlobalConfig



Global Configuration for Istio components.



_Appears in:_
- [Values](#values)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `arch` _[ArchConfig](#archconfig)_ | Specifies pod scheduling arch(amd64, ppc64le, s390x, arm64) and weight as follows:    0 - Never scheduled   1 - Least preferred   2 - No preference   3 - Most preferred  Deprecated: replaced by the affinity k8s settings which allows architecture nodeAffinity configuration of this behavior.  Deprecated: Marked as deprecated in pkg/apis/istio/v1alpha1/values_types.proto. |  |  |
| `certSigners` _string array_ | List of certSigners to allow "approve" action in the ClusterRole |  |  |
| `configValidation` _boolean_ | Controls whether the server-side validation is enabled. |  |  |
| `defaultNodeSelector` _object (keys:string, values:string)_ | Default k8s node selector for all the Istio control plane components  See https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#nodeselector  Deprecated: Marked as deprecated in pkg/apis/istio/v1alpha1/values_types.proto. |  |  |
| `defaultPodDisruptionBudget` _[DefaultPodDisruptionBudgetConfig](#defaultpoddisruptionbudgetconfig)_ | Specifies the default pod disruption budget configuration.  Deprecated: Marked as deprecated in pkg/apis/istio/v1alpha1/values_types.proto. |  |  |
| `defaultResources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#resourcerequirements-v1-core)_ | Default k8s resources settings for all Istio control plane components.  See https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#resource-requests-and-limits-of-pod-and-container  Deprecated: Marked as deprecated in pkg/apis/istio/v1alpha1/values_types.proto. |  |  |
| `defaultTolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#toleration-v1-core) array_ | Default node tolerations to be applied to all deployments so that all pods can be scheduled to nodes with matching taints. Each component can overwrite these default values by adding its tolerations block in the relevant section below and setting the desired values. Configure this field in case that all pods of Istio control plane are expected to be scheduled to particular nodes with specified taints.  Deprecated: Marked as deprecated in pkg/apis/istio/v1alpha1/values_types.proto. |  |  |
| `hub` _string_ | Specifies the docker hub for Istio images. |  |  |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#pullpolicy-v1-core)_ | Specifies the image pull policy for the Istio images. one of Always, Never, IfNotPresent. Defaults to Always if :latest tag is specified, or IfNotPresent otherwise. Cannot be updated.  More info: https://kubernetes.io/docs/concepts/containers/images#updating-images |  | Enum: [Always Never IfNotPresent]   |
| `imagePullSecrets` _string array_ | ImagePullSecrets for the control plane ServiceAccount, list of secrets in the same namespace to use for pulling any images in pods that reference this ServiceAccount. Must be set for any cluster configured with private docker registry. |  |  |
| `istioNamespace` _string_ | Specifies the default namespace for the Istio control plane components. |  |  |
| `logAsJson` _boolean_ | Specifies whether istio components should output logs in json format by adding --log_as_json argument to each container. |  |  |
| `logging` _[GlobalLoggingConfig](#globalloggingconfig)_ | Specifies the global logging level settings for the Istio control plane components. |  |  |
| `meshID` _string_ | The Mesh Identifier. It should be unique within the scope where meshes will interact with each other, but it is not required to be globally/universally unique. For example, if any of the following are true, then two meshes must have different Mesh IDs: - Meshes will have their telemetry aggregated in one place - Meshes will be federated together - Policy will be written referencing one mesh from the other  If an administrator expects that any of these conditions may become true in the future, they should ensure their meshes have different Mesh IDs assigned.  Within a multicluster mesh, each cluster must be (manually or auto) configured to have the same Mesh ID value. If an existing cluster 'joins' a multicluster mesh, it will need to be migrated to the new mesh ID. Details of migration TBD, and it may be a disruptive operation to change the Mesh ID post-install.  If the mesh admin does not specify a value, Istio will use the value of the mesh's Trust Domain. The best practice is to select a proper Trust Domain value. |  |  |
| `meshNetworks` _object (keys:string, values:[Network](#network))_ | Configure the mesh networks to be used by the Split Horizon EDS.  The following example defines two networks with different endpoints association methods. For `network1` all endpoints that their IP belongs to the provided CIDR range will be mapped to network1. The gateway for this network example is specified by its public IP address and port. The second network, `network2`, in this example is defined differently with all endpoints retrieved through the specified Multi-Cluster registry being mapped to network2. The gateway is also defined differently with the name of the gateway service on the remote cluster. The public IP for the gateway will be determined from that remote service (only LoadBalancer gateway service type is currently supported, for a NodePort type gateway service, it still need to be configured manually).  meshNetworks:    network1:     endpoints:     - fromCidr: "192.168.0.1/24"     gateways:     - address: 1.1.1.1       port: 80   network2:     endpoints:     - fromRegistry: reg1     gateways:     - registryServiceName: istio-ingressgateway.istio-system.svc.cluster.local       port: 443 |  |  |
| `multiCluster` _[MultiClusterConfig](#multiclusterconfig)_ | Specifies the Configuration for Istio mesh across multiple clusters through Istio gateways. |  |  |
| `network` _string_ | Network defines the network this cluster belong to. This name corresponds to the networks in the map of mesh networks. |  |  |
| `podDNSSearchNamespaces` _string array_ | Custom DNS config for the pod to resolve names of services in other clusters. Use this to add additional search domains, and other settings. see https://kubernetes.io/docs/concepts/services-networking/dns-pod-service/#dns-config This does not apply to gateway pods as they typically need a different set of DNS settings than the normal application pods (e.g. in multicluster scenarios). |  |  |
| `omitSidecarInjectorConfigMap` _boolean_ | Controls whether the creation of the sidecar injector ConfigMap should be skipped. Defaults to false. When set to true, the sidecar injector ConfigMap will not be created. |  |  |
| `operatorManageWebhooks` _boolean_ | Controls whether the WebhookConfiguration resource(s) should be created. The current behavior of Istiod is to manage its own webhook configurations. When this option is set to true, Istio Operator, instead of webhooks, manages the webhook configurations. When this option is set as false, webhooks manage their own webhook configurations. |  |  |
| `priorityClassName` _string_ | Specifies the k8s priorityClassName for the istio control plane components.  See https://kubernetes.io/docs/concepts/configuration/pod-priority-preemption/#priorityclass  Deprecated: Marked as deprecated in pkg/apis/istio/v1alpha1/values_types.proto. |  |  |
| `proxy` _[ProxyConfig](#proxyconfig)_ | Specifies how proxies are configured within Istio. |  |  |
| `proxy_init` _[ProxyInitConfig](#proxyinitconfig)_ | Specifies the Configuration for proxy_init container which sets the pods' networking to intercept the inbound/outbound traffic. |  |  |
| `sds` _[SDSConfig](#sdsconfig)_ | Specifies the Configuration for the SecretDiscoveryService instead of using K8S secrets to mount the certificates. |  |  |
| `tag` _string_ | Specifies the tag for the Istio docker images. |  |  |
| `variant` _string_ | The variant of the Istio container images to use. Options are "debug" or "distroless". Unset will use the default for the given version. |  |  |
| `tracer` _[TracerConfig](#tracerconfig)_ | Specifies the Configuration for each of the supported tracers. |  |  |
| `remotePilotAddress` _string_ | Specifies the Istio control plane’s pilot Pod IP address or remote cluster DNS resolvable hostname. |  |  |
| `istiod` _[IstiodConfig](#istiodconfig)_ | Specifies the configution of istiod |  |  |
| `pilotCertProvider` _string_ | Configure the Pilot certificate provider. Currently, four providers are supported: "kubernetes", "istiod", "custom" and "none". |  |  |
| `jwtPolicy` _string_ | Configure the policy for validating JWT. This is deprecated and has no effect.  Deprecated: Marked as deprecated in pkg/apis/istio/v1alpha1/values_types.proto. |  |  |
| `sts` _[STSConfig](#stsconfig)_ | Specifies the configuration for Security Token Service. |  |  |
| `revision` _string_ | Configures the revision this control plane is a part of |  |  |
| `mountMtlsCerts` _boolean_ | Controls whether the in-cluster MTLS key and certs are loaded from the secret volume mounts. |  |  |
| `caAddress` _string_ | The address of the CA for CSR. |  |  |
| `externalIstiod` _boolean_ | Controls whether one external istiod is enabled. |  |  |
| `configCluster` _boolean_ | Controls whether a remote cluster is the config cluster for an external istiod |  |  |
| `caName` _string_ | The name of the CA for workloads. For example, when caName=GkeWorkloadCertificate, GKE workload certificates will be used as the certificates for workloads. The default value is "" and when caName="", the CA will be configured by other mechanisms (e.g., environmental variable CA_PROVIDER). |  |  |
| `platform` _string_ | Platform in which Istio is deployed. Possible values are: "openshift" and "gcp" An empty value means it is a vanilla Kubernetes distribution, therefore no special treatment will be considered. |  |  |
| `ipFamilies` _string array_ | Defines which IP family to use for single stack or the order of IP families for dual-stack. Valid list items are "IPv4", "IPv6". More info: https://kubernetes.io/docs/concepts/services-networking/dual-stack/#services |  |  |
| `ipFamilyPolicy` _string_ | Controls whether Services are configured to use IPv4, IPv6, or both. Valid options are PreferDualStack, RequireDualStack, and SingleStack. More info: https://kubernetes.io/docs/concepts/services-networking/dual-stack/#services |  |  |
| `waypoint` _[WaypointConfig](#waypointconfig)_ | Specifies how waypoints are configured within Istio. |  |  |


#### GlobalLoggingConfig



GlobalLoggingConfig specifies the global logging level settings for the Istio control plane components.



_Appears in:_
- [CNIConfig](#cniconfig)
- [CNIGlobalConfig](#cniglobalconfig)
- [GlobalConfig](#globalconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `level` _string_ | Comma-separated minimum per-scope logging level of messages to output, in the form of <scope>:<level>,<scope>:<level> The control plane has different scopes depending on component, but can configure default log level across all components If empty, default scope and level will be used as configured in code |  |  |


#### HTTPRetry



Describes the retry policy to use when a HTTP request fails. For
example, the following rule sets the maximum number of retries to 3 when
calling ratings:v1 service, with a 2s timeout per retry attempt.
A retry will be attempted if there is a connect-failure, refused_stream
or when the upstream server responds with Service Unavailable(503).


```yaml
apiVersion: networking.istio.io/v1
kind: VirtualService
metadata:
  name: ratings-route
spec:
  hosts:
  - ratings.prod.svc.cluster.local
  http:
  - route:
    - destination:
        host: ratings.prod.svc.cluster.local
        subset: v1
    retries:
      attempts: 3
      perTryTimeout: 2s
      retryOn: gateway-error,connect-failure,refused-stream
```



_Appears in:_
- [MeshConfig](#meshconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `attempts` _integer_ | Number of retries to be allowed for a given request. The interval between retries will be determined automatically (25ms+). When request `timeout` of the [HTTP route](https://istio.io/docs/reference/config/networking/virtual-service/#HTTPRoute) or `per_try_timeout` is configured, the actual number of retries attempted also depends on the specified request `timeout` and `per_try_timeout` values. MUST BE >= 0. If `0`, retries will be disabled. The maximum possible number of requests made will be 1 + `attempts`. |  |  |
| `perTryTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#duration-v1-meta)_ | Timeout per attempt for a given request, including the initial call and any retries. Format: 1h/1m/1s/1ms. MUST BE >=1ms. Default is same value as request `timeout` of the [HTTP route](https://istio.io/docs/reference/config/networking/virtual-service/#HTTPRoute), which means no timeout. |  |  |
| `retryOn` _string_ | Specifies the conditions under which retry takes place. One or more policies can be specified using a ‘,’ delimited list. See the [retry policies](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/router_filter#x-envoy-retry-on) and [gRPC retry policies](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/router_filter#x-envoy-retry-grpc-on) for more details.  In addition to the policies specified above, a list of HTTP status codes can be passed, such as `retryOn: "503,reset"`. Note these status codes refer to the actual responses received from the destination. For example, if a connection is reset, Istio will translate this to 503 for it's response. However, the destination did not return a 503 error, so this would not match `"503"` (it would, however, match `"reset"`).  If not specified, this defaults to `connect-failure,refused-stream,unavailable,cancelled,503`. |  |  |
| `retryRemoteLocalities` _boolean_ | Flag to specify whether the retries should retry to other localities. See the [retry plugin configuration](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/http/http_connection_management#retry-plugin-configuration) for more details. |  |  |




#### Istio



Istio represents an Istio Service Mesh deployment consisting of one or more
control plane instances (represented by one or more IstioRevision objects).
To deploy an Istio Service Mesh, a user creates an Istio object with the
desired Istio version and configuration. The operator then creates
an IstioRevision object, which in turn creates the underlying Deployment
objects for istiod and other control plane components, similar to how a
Deployment object in Kubernetes creates ReplicaSets that create the Pods.



_Appears in:_
- [IstioList](#istiolist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `operator.istio.io/v1alpha1` | | |
| `kind` _string_ | `Istio` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[IstioSpec](#istiospec)_ |  | \{ namespace:istio-system updateStrategy:map[type:InPlace] version:v1.23.0 \} |  |
| `status` _[IstioStatus](#istiostatus)_ |  |  |  |


#### IstioCNI



IstioCNI represents a deployment of the Istio CNI component.



_Appears in:_
- [IstioCNIList](#istiocnilist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `operator.istio.io/v1alpha1` | | |
| `kind` _string_ | `IstioCNI` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[IstioCNISpec](#istiocnispec)_ |  | \{ namespace:istio-cni version:v1.23.0 \} |  |
| `status` _[IstioCNIStatus](#istiocnistatus)_ |  |  |  |


#### IstioCNICondition



IstioCNICondition represents a specific observation of the IstioCNI object's state.



_Appears in:_
- [IstioCNIStatus](#istiocnistatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[IstioCNIConditionType](#istiocniconditiontype)_ | The type of this condition. |  |  |
| `status` _[ConditionStatus](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#conditionstatus-v1-meta)_ | The status of this condition. Can be True, False or Unknown. |  |  |
| `reason` _[IstioCNIConditionReason](#istiocniconditionreason)_ | Unique, single-word, CamelCase reason for the condition's last transition. |  |  |
| `message` _string_ | Human-readable message indicating details about the last transition. |  |  |
| `lastTransitionTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#time-v1-meta)_ | Last time the condition transitioned from one status to another. |  |  |


#### IstioCNIConditionReason

_Underlying type:_ _string_

IstioCNIConditionReason represents a short message indicating how the condition came
to be in its present state.



_Appears in:_
- [IstioCNICondition](#istiocnicondition)
- [IstioCNIStatus](#istiocnistatus)

| Field | Description |
| --- | --- |
| `ReconcileError` | IstioCNIReasonReconcileError indicates that the reconciliation of the resource has failed, but will be retried.  |
| `DaemonSetNotReady` | IstioCNIDaemonSetNotReady indicates that the istio-cni-node DaemonSet is not ready.  |
| `ReadinessCheckFailed` | IstioCNIReasonReadinessCheckFailed indicates that the DaemonSet readiness status could not be ascertained.  |
| `Healthy` | IstioCNIReasonHealthy indicates that the control plane is fully reconciled and that all components are ready.  |


#### IstioCNIConditionType

_Underlying type:_ _string_

IstioCNIConditionType represents the type of the condition.  Condition stages are:
Installed, Reconciled, Ready



_Appears in:_
- [IstioCNICondition](#istiocnicondition)

| Field | Description |
| --- | --- |
| `Reconciled` | IstioCNIConditionReconciled signifies whether the controller has successfully reconciled the resources defined through the CR.  |
| `Ready` | IstioCNIConditionReady signifies whether the istio-cni-node DaemonSet is ready.  |


#### IstioCNIList



IstioCNIList contains a list of IstioCNI





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `operator.istio.io/v1alpha1` | | |
| `kind` _string_ | `IstioCNIList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[IstioCNI](#istiocni) array_ |  |  |  |


#### IstioCNISpec



IstioCNISpec defines the desired state of IstioCNI



_Appears in:_
- [IstioCNI](#istiocni)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `version` _string_ | Defines the version of Istio to install. Must be one of: v1.23.0, v1.22.3, v1.21.5, latest. | v1.23.0 | Enum: [v1.23.0 v1.22.3 v1.21.5 latest]   |
| `profile` _string_ | The built-in installation configuration profile to use. The 'default' profile is always applied. On OpenShift, the 'openshift' profile is also applied on top of 'default'. Must be one of: ambient, default, demo, empty, external, openshift-ambient, openshift, preview, stable. |  | Enum: [ambient default demo empty external openshift-ambient openshift preview stable]   |
| `namespace` _string_ | Namespace to which the Istio CNI component should be installed. | istio-cni |  |
| `values` _[CNIValues](#cnivalues)_ | Defines the values to be passed to the Helm charts when installing Istio CNI. |  |  |


#### IstioCNIStatus



IstioCNIStatus defines the observed state of IstioCNI



_Appears in:_
- [IstioCNI](#istiocni)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the most recent generation observed for this IstioCNI object. It corresponds to the object's generation, which is updated on mutation by the API Server. The information in the status pertains to this particular generation of the object. |  |  |
| `conditions` _[IstioCNICondition](#istiocnicondition) array_ | Represents the latest available observations of the object's current state. |  |  |
| `state` _[IstioCNIConditionReason](#istiocniconditionreason)_ | Reports the current state of the object. |  |  |


#### IstioCondition



IstioCondition represents a specific observation of the IstioCondition object's state.



_Appears in:_
- [IstioStatus](#istiostatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[IstioConditionType](#istioconditiontype)_ | The type of this condition. |  |  |
| `status` _[ConditionStatus](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#conditionstatus-v1-meta)_ | The status of this condition. Can be True, False or Unknown. |  |  |
| `reason` _[IstioConditionReason](#istioconditionreason)_ | Unique, single-word, CamelCase reason for the condition's last transition. |  |  |
| `message` _string_ | Human-readable message indicating details about the last transition. |  |  |
| `lastTransitionTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#time-v1-meta)_ | Last time the condition transitioned from one status to another. |  |  |


#### IstioConditionReason

_Underlying type:_ _string_

IstioConditionReason represents a short message indicating how the condition came
to be in its present state.



_Appears in:_
- [IstioCondition](#istiocondition)
- [IstioStatus](#istiostatus)

| Field | Description |
| --- | --- |
| `ReconcileError` | IstioReasonReconcileError indicates that the reconciliation of the resource has failed, but will be retried.  |
| `ActiveRevisionNotFound` | IstioReasonRevisionNotFound indicates that the active IstioRevision is not found.  |
| `FailedToGetActiveRevision` | IstioReasonFailedToGetActiveRevision indicates that a failure occurred when getting the active IstioRevision  |
| `IstiodNotReady` | IstioReasonIstiodNotReady indicates that the control plane is fully reconciled, but istiod is not ready.  |
| `ReadinessCheckFailed` | IstioReasonReadinessCheckFailed indicates that readiness could not be ascertained.  |
| `Healthy` | IstioReasonHealthy indicates that the control plane is fully reconciled and that all components are ready.  |


#### IstioConditionType

_Underlying type:_ _string_

IstioConditionType represents the type of the condition.  Condition stages are:
Installed, Reconciled, Ready



_Appears in:_
- [IstioCondition](#istiocondition)

| Field | Description |
| --- | --- |
| `Reconciled` | IstioConditionReconciled signifies whether the controller has successfully reconciled the resources defined through the CR.  |
| `Ready` | IstioConditionReady signifies whether any Deployment, StatefulSet, etc. resources are Ready.  |


#### IstioList



IstioList contains a list of Istio





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `operator.istio.io/v1alpha1` | | |
| `kind` _string_ | `IstioList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[Istio](#istio) array_ |  |  |  |


#### IstioRevision



IstioRevision represents a single revision of an Istio Service Mesh deployment.
Users shouldn't create IstioRevision objects directly. Instead, they should
create an Istio object and allow the operator to create the underlying
IstioRevision object(s).



_Appears in:_
- [IstioRevisionList](#istiorevisionlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `operator.istio.io/v1alpha1` | | |
| `kind` _string_ | `IstioRevision` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[IstioRevisionSpec](#istiorevisionspec)_ |  |  |  |
| `status` _[IstioRevisionStatus](#istiorevisionstatus)_ |  |  |  |


#### IstioRevisionCondition



IstioRevisionCondition represents a specific observation of the IstioRevision object's state.



_Appears in:_
- [IstioRevisionStatus](#istiorevisionstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[IstioRevisionConditionType](#istiorevisionconditiontype)_ | The type of this condition. |  |  |
| `status` _[ConditionStatus](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#conditionstatus-v1-meta)_ | The status of this condition. Can be True, False or Unknown. |  |  |
| `reason` _[IstioRevisionConditionReason](#istiorevisionconditionreason)_ | Unique, single-word, CamelCase reason for the condition's last transition. |  |  |
| `message` _string_ | Human-readable message indicating details about the last transition. |  |  |
| `lastTransitionTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#time-v1-meta)_ | Last time the condition transitioned from one status to another. |  |  |


#### IstioRevisionConditionReason

_Underlying type:_ _string_

IstioRevisionConditionReason represents a short message indicating how the condition came
to be in its present state.



_Appears in:_
- [IstioRevisionCondition](#istiorevisioncondition)
- [IstioRevisionStatus](#istiorevisionstatus)

| Field | Description |
| --- | --- |
| `ReconcileError` | IstioRevisionReasonReconcileError indicates that the reconciliation of the resource has failed, but will be retried.  |
| `IstiodNotReady` | IstioRevisionReasonIstiodNotReady indicates that the control plane is fully reconciled, but istiod is not ready.  |
| `RemoteIstiodNotReady` | IstioRevisionReasonRemoteIstiodNotReady indicates that the remote istiod is not ready.  |
| `ReadinessCheckFailed` | IstioRevisionReasonReadinessCheckFailed indicates that istiod readiness status could not be ascertained.  |
| `ReferencedByWorkloads` | IstioRevisionReasonReferencedByWorkloads indicates that the revision is referenced by at least one pod or namespace.  |
| `NotReferencedByAnything` | IstioRevisionReasonNotReferenced indicates that the revision is not referenced by any pod or namespace.  |
| `UsageCheckFailed` | IstioRevisionReasonUsageCheckFailed indicates that the operator could not check whether any workloads use the revision.  |
| `Healthy` | IstioRevisionReasonHealthy indicates that the control plane is fully reconciled and that all components are ready.  |


#### IstioRevisionConditionType

_Underlying type:_ _string_

IstioRevisionConditionType represents the type of the condition.  Condition stages are:
Installed, Reconciled, Ready



_Appears in:_
- [IstioRevisionCondition](#istiorevisioncondition)

| Field | Description |
| --- | --- |
| `Reconciled` | IstioRevisionConditionReconciled signifies whether the controller has successfully reconciled the resources defined through the CR.  |
| `Ready` | IstioRevisionConditionReady signifies whether any Deployment, StatefulSet, etc. resources are Ready.  |
| `InUse` | IstioRevisionConditionInUse signifies whether any workload is configured to use the revision.  |


#### IstioRevisionList



IstioRevisionList contains a list of IstioRevision





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `operator.istio.io/v1alpha1` | | |
| `kind` _string_ | `IstioRevisionList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[IstioRevision](#istiorevision) array_ |  |  |  |


#### IstioRevisionSpec



IstioRevisionSpec defines the desired state of IstioRevision



_Appears in:_
- [IstioRevision](#istiorevision)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[IstioRevisionType](#istiorevisiontype)_ | Type indicates whether this revision represents a local or a remote control plane installation. | Local |  |
| `version` _string_ | Defines the version of Istio to install. Must be one of: v1.23.0, v1.22.3, v1.21.5, latest. |  | Enum: [v1.23.0 v1.22.3 v1.21.5 latest]   |
| `namespace` _string_ | Namespace to which the Istio components should be installed. |  |  |
| `values` _[Values](#values)_ | Defines the values to be passed to the Helm charts when installing Istio. |  |  |


#### IstioRevisionStatus



IstioRevisionStatus defines the observed state of IstioRevision



_Appears in:_
- [IstioRevision](#istiorevision)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the most recent generation observed for this IstioRevision object. It corresponds to the object's generation, which is updated on mutation by the API Server. The information in the status pertains to this particular generation of the object. |  |  |
| `conditions` _[IstioRevisionCondition](#istiorevisioncondition) array_ | Represents the latest available observations of the object's current state. |  |  |
| `state` _[IstioRevisionConditionReason](#istiorevisionconditionreason)_ | Reports the current state of the object. |  |  |


#### IstioRevisionType

_Underlying type:_ _string_





_Appears in:_
- [IstioRevisionSpec](#istiorevisionspec)

| Field | Description |
| --- | --- |
| `Local` | IstioRevisionTypeLocal indicates that the revision represents a local control plane installation.  |
| `Remote` | IstioRevisionTypeRemote indicates that the revision represents a remote control plane installation.  |


#### IstioSpec



IstioSpec defines the desired state of Istio



_Appears in:_
- [Istio](#istio)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `version` _string_ | Defines the version of Istio to install. Must be one of: v1.23.0, v1.22.3, v1.21.5, latest. | v1.23.0 | Enum: [v1.23.0 v1.22.3 v1.21.5 latest]   |
| `updateStrategy` _[IstioUpdateStrategy](#istioupdatestrategy)_ | Defines the update strategy to use when the version in the Istio CR is updated. | \{ type:InPlace \} |  |
| `profile` _string_ | The built-in installation configuration profile to use. The 'default' profile is always applied. On OpenShift, the 'openshift' profile is also applied on top of 'default'. Must be one of: ambient, default, demo, empty, external, openshift-ambient, openshift, preview, stable. |  | Enum: [ambient default demo empty external openshift-ambient openshift preview stable]   |
| `namespace` _string_ | Namespace to which the Istio components should be installed. | istio-system |  |
| `values` _[Values](#values)_ | Defines the values to be passed to the Helm charts when installing Istio. |  |  |


#### IstioStatus



IstioStatus defines the observed state of Istio



_Appears in:_
- [Istio](#istio)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the most recent generation observed for this Istio object. It corresponds to the object's generation, which is updated on mutation by the API Server. The information in the status pertains to this particular generation of the object. |  |  |
| `conditions` _[IstioCondition](#istiocondition) array_ | Represents the latest available observations of the object's current state. |  |  |
| `state` _[IstioConditionReason](#istioconditionreason)_ | Reports the current state of the object. |  |  |
| `revisions` _[RevisionSummary](#revisionsummary)_ | Reports information about the underlying IstioRevisions. |  |  |


#### IstioUpdateStrategy



IstioUpdateStrategy defines how the control plane should be updated when the version in
the Istio CR is updated.



_Appears in:_
- [IstioSpec](#istiospec)
- [RemoteIstioSpec](#remoteistiospec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[UpdateStrategyType](#updatestrategytype)_ | Type of strategy to use. Can be "InPlace" or "RevisionBased". When the "InPlace" strategy is used, the existing Istio control plane is updated in-place. The workloads therefore don't need to be moved from one control plane instance to another. When the "RevisionBased" strategy is used, a new Istio control plane instance is created for every change to the Istio.spec.version field. The old control plane remains in place until all workloads have been moved to the new control plane instance.  The "InPlace" strategy is the default.  TODO: change default to "RevisionBased" | InPlace | Enum: [InPlace RevisionBased]   |
| `inactiveRevisionDeletionGracePeriodSeconds` _integer_ | Defines how many seconds the operator should wait before removing a non-active revision after all the workloads have stopped using it. You may want to set this value on the order of minutes. The minimum and the default value is 30. |  | Minimum: 30   |
| `updateWorkloads` _boolean_ | Defines whether the workloads should be moved from one control plane instance to another automatically. If updateWorkloads is true, the operator moves the workloads from the old control plane instance to the new one after the new control plane is ready. If updateWorkloads is false, the user must move the workloads manually by updating the istio.io/rev labels on the namespace and/or the pods. Defaults to false. |  |  |


#### IstiodConfig







_Appears in:_
- [GlobalConfig](#globalconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enableAnalysis` _boolean_ | If enabled, istiod will perform config analysis |  |  |


#### IstiodRemoteConfig







_Appears in:_
- [Values](#values)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `injectionURL` _string_ | URL to use for sidecar injector webhook. |  |  |
| `injectionPath` _string_ | Path to use for the sidecar injector webhook service. |  |  |
| `injectionCABundle` _string_ | injector ca bundle |  |  |


#### LocalityLoadBalancerSetting



Locality-weighted load balancing allows administrators to control the
distribution of traffic to endpoints based on the localities of where the
traffic originates and where it will terminate. These localities are
specified using arbitrary labels that designate a hierarchy of localities in
{region}/{zone}/{sub-zone} form. For additional detail refer to
[Locality Weight](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/upstream/load_balancing/locality_weight)
The following example shows how to setup locality weights mesh-wide.


Given a mesh with workloads and their service deployed to "us-west/zone1/*"
and "us-west/zone2/*". This example specifies that when traffic accessing a
service originates from workloads in "us-west/zone1/*", 80% of the traffic
will be sent to endpoints in "us-west/zone1/*", i.e the same zone, and the
remaining 20% will go to endpoints in "us-west/zone2/*". This setup is
intended to favor routing traffic to endpoints in the same locality.
A similar setting is specified for traffic originating in "us-west/zone2/*".


```yaml
  distribute:
    - from: us-west/zone1/*
      to:
        "us-west/zone1/*": 80
        "us-west/zone2/*": 20
    - from: us-west/zone2/*
      to:
        "us-west/zone1/*": 20
        "us-west/zone2/*": 80
```


If the goal of the operator is not to distribute load across zones and
regions but rather to restrict the regionality of failover to meet other
operational requirements an operator can set a 'failover' policy instead of
a 'distribute' policy.


The following example sets up a locality failover policy for regions.
Assume a service resides in zones within us-east, us-west & eu-west
this example specifies that when endpoints within us-east become unhealthy
traffic should failover to endpoints in any zone or sub-zone within eu-west
and similarly us-west should failover to us-east.


```yaml
  failover:
    - from: us-east
      to: eu-west
    - from: us-west
      to: us-east
```
Locality load balancing settings.



_Appears in:_
- [MeshConfig](#meshconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `distribute` _[LocalityLoadBalancerSettingDistribute](#localityloadbalancersettingdistribute) array_ | Optional: only one of distribute, failover or failoverPriority can be set. Explicitly specify loadbalancing weight across different zones and geographical locations. Refer to [Locality weighted load balancing](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/upstream/load_balancing/locality_weight) If empty, the locality weight is set according to the endpoints number within it. |  |  |
| `failover` _[LocalityLoadBalancerSettingFailover](#localityloadbalancersettingfailover) array_ | Optional: only one of distribute, failover or failoverPriority can be set. Explicitly specify the region traffic will land on when endpoints in local region becomes unhealthy. Should be used together with OutlierDetection to detect unhealthy endpoints. Note: if no OutlierDetection specified, this will not take effect. |  |  |
| `failoverPriority` _string array_ | failoverPriority is an ordered list of labels used to sort endpoints to do priority based load balancing. This is to support traffic failover across different groups of endpoints. Two kinds of labels can be specified:    - Specify only label keys `[key1, key2, key3]`, istio would compare the label values of client with endpoints.     Suppose there are total N label keys `[key1, key2, key3, ...keyN]` specified:      1. Endpoints matching all N labels with the client proxy have priority P(0) i.e. the highest priority.     2. Endpoints matching the first N-1 labels with the client proxy have priority P(1) i.e. second highest priority.     3. By extension of this logic, endpoints matching only the first label with the client proxy has priority P(N-1) i.e. second lowest priority.     4. All the other endpoints have priority P(N) i.e. lowest priority.    - Specify labels with key and value `[key1=value1, key2=value2, key3=value3]`, istio would compare the labels with endpoints.     Suppose there are total N labels `[key1=value1, key2=value2, key3=value3, ...keyN=valueN]` specified:      1. Endpoints matching all N labels have priority P(0) i.e. the highest priority.     2. Endpoints matching the first N-1 labels have priority P(1) i.e. second highest priority.     3. By extension of this logic, endpoints matching only the first label has priority P(N-1) i.e. second lowest priority.     4. All the other endpoints have priority P(N) i.e. lowest priority.  Note: For a label to be considered for match, the previous labels must match, i.e. nth label would be considered matched only if first n-1 labels match.  It can be any label specified on both client and server workloads. The following labels which have special semantic meaning are also supported:    - `topology.istio.io/network` is used to match the network metadata of an endpoint, which can be specified by pod/namespace label `topology.istio.io/network`, sidecar env `ISTIO_META_NETWORK` or MeshNetworks.   - `topology.istio.io/cluster` is used to match the clusterID of an endpoint, which can be specified by pod label `topology.istio.io/cluster` or pod env `ISTIO_META_CLUSTER_ID`.   - `topology.kubernetes.io/region` is used to match the region metadata of an endpoint, which maps to Kubernetes node label `topology.kubernetes.io/region` or the deprecated label `failure-domain.beta.kubernetes.io/region`.   - `topology.kubernetes.io/zone` is used to match the zone metadata of an endpoint, which maps to Kubernetes node label `topology.kubernetes.io/zone` or the deprecated label `failure-domain.beta.kubernetes.io/zone`.   - `topology.istio.io/subzone` is used to match the subzone metadata of an endpoint, which maps to Istio node label `topology.istio.io/subzone`.   - `kubernetes.io/hostname` is used to match the current node of an endpoint, which maps to Kubernetes node label `kubernetes.io/hostname`.  The below topology config indicates the following priority levels:  ```yaml failoverPriority: - "topology.istio.io/network" - "topology.kubernetes.io/region" - "topology.kubernetes.io/zone" - "topology.istio.io/subzone" ```  1. endpoints match same [network, region, zone, subzone] label with the client proxy have the highest priority. 2. endpoints have same [network, region, zone] label but different [subzone] label with the client proxy have the second highest priority. 3. endpoints have same [network, region] label but different [zone] label with the client proxy have the third highest priority. 4. endpoints have same [network] but different [region] labels with the client proxy have the fourth highest priority. 5. all the other endpoints have the same lowest priority.  Suppose a service associated endpoints reside in multi clusters, the below example represents: 1. endpoints in `clusterA` and has `version=v1` label have P(0) priority. 2. endpoints not in `clusterA` but has `version=v1` label have P(1) priority. 2. all the other endpoints have P(2) priority.  ```yaml failoverPriority: - "version=v1" - "topology.istio.io/cluster=clusterA" ```  Optional: only one of distribute, failover or failoverPriority can be set. And it should be used together with `OutlierDetection` to detect unhealthy endpoints, otherwise has no effect. |  |  |
| `enabled` _boolean_ | enable locality load balancing, this is DestinationRule-level and will override mesh wide settings in entirety. e.g. true means that turn on locality load balancing for this DestinationRule no matter what mesh wide settings is. |  |  |


#### LocalityLoadBalancerSettingDistribute



Describes how traffic originating in the 'from' zone or sub-zone is
distributed over a set of 'to' zones. Syntax for specifying a zone is
{region}/{zone}/{sub-zone} and terminal wildcards are allowed on any
segment of the specification. Examples:


`*` - matches all localities


`us-west/*` - all zones and sub-zones within the us-west region


`us-west/zone-1/*` - all sub-zones within us-west/zone-1



_Appears in:_
- [LocalityLoadBalancerSetting](#localityloadbalancersetting)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `from` _string_ | Originating locality, '/' separated, e.g. 'region/zone/sub_zone'. |  |  |
| `to` _object (keys:string, values:integer)_ | Map of upstream localities to traffic distribution weights. The sum of all weights should be 100. Any locality not present will receive no traffic. |  |  |


#### LocalityLoadBalancerSettingFailover



Specify the traffic failover policy across regions. Since zone and sub-zone
failover is supported by default this only needs to be specified for
regions when the operator needs to constrain traffic failover so that
the default behavior of failing over to any endpoint globally does not
apply. This is useful when failing over traffic across regions would not
improve service health or may need to be restricted for other reasons
like regulatory controls.



_Appears in:_
- [LocalityLoadBalancerSetting](#localityloadbalancersetting)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `from` _string_ | Originating region. |  |  |
| `to` _string_ | Destination region the traffic will fail over to when endpoints in the 'from' region becomes unhealthy. |  |  |


#### MeshConfig



MeshConfig defines mesh-wide settings for the Istio service mesh.



_Appears in:_
- [Values](#values)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `proxyListenPort` _integer_ | Port on which Envoy should listen for all outbound traffic to other services. Default port is 15001. |  |  |
| `proxyInboundListenPort` _integer_ | Port on which Envoy should listen for all inbound traffic to the pod/vm will be captured to. Default port is 15006. |  |  |
| `proxyHttpPort` _integer_ | Port on which Envoy should listen for HTTP PROXY requests if set. |  |  |
| `connectTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#duration-v1-meta)_ | Connection timeout used by Envoy. (MUST BE >=1ms) Default timeout is 10s. |  |  |
| `tcpKeepalive` _[ConnectionPoolSettingsTCPSettingsTcpKeepalive](#connectionpoolsettingstcpsettingstcpkeepalive)_ | If set then set `SO_KEEPALIVE` on the socket to enable TCP Keepalives. |  |  |
| `ingressClass` _string_ | Class of ingress resources to be processed by Istio ingress controller. This corresponds to the value of `kubernetes.io/ingress.class` annotation. |  |  |
| `ingressService` _string_ | Name of the Kubernetes service used for the istio ingress controller. If no ingress controller is specified, the default value `istio-ingressgateway` is used. |  |  |
| `ingressControllerMode` _[MeshConfigIngressControllerMode](#meshconfigingresscontrollermode)_ | Defines whether to use Istio ingress controller for annotated or all ingress resources. Default mode is `STRICT`. |  | Enum: [UNSPECIFIED OFF DEFAULT STRICT]   |
| `ingressSelector` _string_ | Defines which gateway deployment to use as the Ingress controller. This field corresponds to the Gateway.selector field, and will be set as `istio: INGRESS_SELECTOR`. By default, `ingressgateway` is used, which will select the default IngressGateway as it has the `istio: ingressgateway` labels. It is recommended that this is the same value as ingress_service. |  |  |
| `enableTracing` _boolean_ | Flag to control generation of trace spans and request IDs. Requires a trace span collector defined in the proxy configuration. |  |  |
| `accessLogFile` _string_ | File address for the proxy access log (e.g. /dev/stdout). Empty value disables access logging. |  |  |
| `accessLogFormat` _string_ | Format for the proxy access log Empty value results in proxy's default access log format |  |  |
| `accessLogEncoding` _[MeshConfigAccessLogEncoding](#meshconfigaccesslogencoding)_ | Encoding for the proxy access log (`TEXT` or `JSON`). Default value is `TEXT`. |  | Enum: [TEXT JSON]   |
| `enableEnvoyAccessLogService` _boolean_ | This flag enables Envoy's gRPC Access Log Service. See [Access Log Service](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/access_loggers/grpc/v3/als.proto) for details about Envoy's gRPC Access Log Service API. Default value is `false`. |  |  |
| `disableEnvoyListenerLog` _boolean_ | This flag disables Envoy Listener logs. See [Listener Access Log](https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/listener/v3/listener.proto#envoy-v3-api-field-config-listener-v3-listener-access-log) Istio Enables Envoy's listener access logs on "NoRoute" response flag. Default value is `false`. |  |  |
| `defaultConfig` _[MeshConfigProxyConfig](#meshconfigproxyconfig)_ | Default proxy config used by gateway and sidecars. In case of Kubernetes, the proxy config is applied once during the injection process, and remain constant for the duration of the pod. The rest of the mesh config can be changed at runtime and config gets distributed dynamically. On Kubernetes, this can be overridden on individual pods with the `proxy.istio.io/config` annotation. |  |  |
| `outboundTrafficPolicy` _[MeshConfigOutboundTrafficPolicy](#meshconfigoutboundtrafficpolicy)_ | Set the default behavior of the sidecar for handling outbound traffic from the application.  Can be overridden at a Sidecar level by setting the `OutboundTrafficPolicy` in the [Sidecar API](https://istio.io/docs/reference/config/networking/sidecar/#OutboundTrafficPolicy).  Default mode is `ALLOW_ANY`, which means outbound traffic to unknown destinations will be allowed. |  |  |
| `inboundTrafficPolicy` _[MeshConfigInboundTrafficPolicy](#meshconfiginboundtrafficpolicy)_ | Set the default behavior of the sidecar for handling inbound traffic to the application.  If your application listens on localhost, you will need to set this to `LOCALHOST`. |  |  |
| `configSources` _[ConfigSource](#configsource) array_ | ConfigSource describes a source of configuration data for networking rules, and other Istio configuration artifacts. Multiple data sources can be configured for a single control plane. |  |  |
| `enableAutoMtls` _boolean_ | This flag is used to enable mutual `TLS` automatically for service to service communication within the mesh, default true. If set to true, and a given service does not have a corresponding `DestinationRule` configured, or its `DestinationRule` does not have ClientTLSSettings specified, Istio configures client side TLS configuration appropriately. More specifically, If the upstream authentication policy is in `STRICT` mode, use Istio provisioned certificate for mutual `TLS` to connect to upstream. If upstream service is in plain text mode, use plain text. If the upstream authentication policy is in PERMISSIVE mode, Istio configures clients to use mutual `TLS` when server sides are capable of accepting mutual `TLS` traffic. If service `DestinationRule` exists and has `ClientTLSSettings` specified, that is always used instead. |  |  |
| `trustDomain` _string_ | The trust domain corresponds to the trust root of a system. Refer to [SPIFFE-ID](https://github.com/spiffe/spiffe/blob/master/standards/SPIFFE-ID.md#21-trust-domain) |  |  |
| `trustDomainAliases` _string array_ | The trust domain aliases represent the aliases of `trust_domain`. For example, if we have ```yaml trustDomain: td1 trustDomainAliases: ["td2", "td3"] ``` Any service with the identity `td1/ns/foo/sa/a-service-account`, `td2/ns/foo/sa/a-service-account`, or `td3/ns/foo/sa/a-service-account` will be treated the same in the Istio mesh. |  |  |
| `caCertificates` _[MeshConfigCertificateData](#meshconfigcertificatedata) array_ | The extra root certificates for workload-to-workload communication. The plugin certificates (the 'cacerts' secret) or self-signed certificates (the 'istio-ca-secret' secret) are automatically added by Istiod. The CA certificate that signs the workload certificates is automatically added by Istio Agent. |  |  |
| `defaultServiceExportTo` _string array_ | The default value for the ServiceEntry.export_to field and services imported through container registry integrations, e.g. this applies to Kubernetes Service resources. The value is a list of namespace names and reserved namespace aliases. The allowed namespace aliases are: ``` * - All Namespaces . - Current Namespace ~ - No Namespace ``` If not set the system will use "*" as the default value which implies that services are exported to all namespaces.  `All namespaces` is a reasonable default for implementations that don't need to restrict access or visibility of services across namespace boundaries. If that requirement is present it is generally good practice to make the default `Current namespace` so that services are only visible within their own namespaces by default. Operators can then expand the visibility of services to other namespaces as needed. Use of `No Namespace` is expected to be rare but can have utility for deployments where dependency management needs to be precise even within the scope of a single namespace.  For further discussion see the reference documentation for `ServiceEntry`, `Sidecar`, and `Gateway`. |  |  |
| `defaultVirtualServiceExportTo` _string array_ | The default value for the VirtualService.export_to field. Has the same syntax as `default_service_export_to`.  If not set the system will use "*" as the default value which implies that virtual services are exported to all namespaces |  |  |
| `defaultDestinationRuleExportTo` _string array_ | The default value for the `DestinationRule.export_to` field. Has the same syntax as `default_service_export_to`.  If not set the system will use "*" as the default value which implies that destination rules are exported to all namespaces |  |  |
| `rootNamespace` _string_ | The namespace to treat as the administrative root namespace for Istio configuration. When processing a leaf namespace Istio will search for declarations in that namespace first and if none are found it will search in the root namespace. Any matching declaration found in the root namespace is processed as if it were declared in the leaf namespace.  The precise semantics of this processing are documented on each resource type. |  |  |
| `localityLbSetting` _[LocalityLoadBalancerSetting](#localityloadbalancersetting)_ | Locality based load balancing distribution or failover settings. If unspecified, locality based load balancing will be enabled by default. However, this requires outlierDetection to actually take effect for a particular service, see https://istio.io/latest/docs/tasks/traffic-management/locality-load-balancing/failover/ |  |  |
| `dnsRefreshRate` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#duration-v1-meta)_ | Configures DNS refresh rate for Envoy clusters of type `STRICT_DNS` Default refresh rate is `60s`. |  |  |
| `h2UpgradePolicy` _[MeshConfigH2UpgradePolicy](#meshconfigh2upgradepolicy)_ | Specify if http1.1 connections should be upgraded to http2 by default. if sidecar is installed on all pods in the mesh, then this should be set to `UPGRADE`. If one or more services or namespaces do not have sidecar(s), then this should be set to `DO_NOT_UPGRADE`. It can be enabled by destination using the `destinationRule.trafficPolicy.connectionPool.http.h2UpgradePolicy` override. |  | Enum: [DO_NOT_UPGRADE UPGRADE]   |
| `inboundClusterStatName` _string_ | Name to be used while emitting statistics for inbound clusters. The same pattern is used while computing stat prefix for network filters like TCP and Redis. By default, Istio emits statistics with the pattern `inbound\|<port>\|<port-name>\|<service-FQDN>`. For example `inbound\|7443\|grpc-reviews\|reviews.prod.svc.cluster.local`. This can be used to override that pattern.  A Pattern can be composed of various pre-defined variables. The following variables are supported.  - `%SERVICE%` - Will be substituted with short hostname of the service. - `%SERVICE_NAME%` - Will be substituted with name of the service. - `%SERVICE_FQDN%` - Will be substituted with FQDN of the service. - `%SERVICE_PORT%` - Will be substituted with port of the service. - `%TARGET_PORT%`  - Will be substituted with the target port of the service. - `%SERVICE_PORT_NAME%` - Will be substituted with port name of the service.  Following are some examples of supported patterns for reviews:  - `%SERVICE_FQDN%_%SERVICE_PORT%` will use reviews.prod.svc.cluster.local_7443 as the stats name. - `%SERVICE%` will use reviews.prod as the stats name. |  |  |
| `outboundClusterStatName` _string_ | Name to be used while emitting statistics for outbound clusters. The same pattern is used while computing stat prefix for network filters like TCP and Redis. By default, Istio emits statistics with the pattern `outbound\|<port>\|<subsetname>\|<service-FQDN>`. For example `outbound\|8080\|v2\|reviews.prod.svc.cluster.local`. This can be used to override that pattern.  A Pattern can be composed of various pre-defined variables. The following variables are supported.  - `%SERVICE%` - Will be substituted with short hostname of the service. - `%SERVICE_NAME%` - Will be substituted with name of the service. - `%SERVICE_FQDN%` - Will be substituted with FQDN of the service. - `%SERVICE_PORT%` - Will be substituted with port of the service. - `%SERVICE_PORT_NAME%` - Will be substituted with port name of the service. - `%SUBSET_NAME%` - Will be substituted with subset.  Following are some examples of supported patterns for reviews:  - `%SERVICE_FQDN%_%SERVICE_PORT%` will use `reviews.prod.svc.cluster.local_7443` as the stats name. - `%SERVICE%` will use reviews.prod as the stats name. |  |  |
| `enablePrometheusMerge` _boolean_ | If enabled, Istio agent will merge metrics exposed by the application with metrics from Envoy and Istio agent. The sidecar injection will replace `prometheus.io` annotations present on the pod and redirect them towards Istio agent, which will then merge metrics of from the application with Istio metrics. This relies on the annotations `prometheus.io/scrape`, `prometheus.io/port`, and `prometheus.io/path` annotations. If you are running a separately managed Envoy with an Istio sidecar, this may cause issues, as the metrics will collide. In this case, it is recommended to disable aggregation on that deployment with the `prometheus.istio.io/merge-metrics: "false"` annotation. If not specified, this will be enabled by default. |  |  |
| `extensionProviders` _[MeshConfigExtensionProvider](#meshconfigextensionprovider) array_ | Defines a list of extension providers that extend Istio's functionality. For example, the AuthorizationPolicy can be used with an extension provider to delegate the authorization decision to a custom authorization system. |  | MaxItems: 1000   |
| `defaultProviders` _[MeshConfigDefaultProviders](#meshconfigdefaultproviders)_ | Specifies extension providers to use by default in Istio configuration resources. |  |  |
| `discoverySelectors` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#labelselector-v1-meta) array_ | A list of Kubernetes selectors that specify the set of namespaces that Istio considers when computing configuration updates for sidecars. This can be used to reduce Istio's computational load by limiting the number of entities (including services, pods, and endpoints) that are watched and processed. If omitted, Istio will use the default behavior of processing all namespaces in the cluster. Elements in the list are disjunctive (OR semantics), i.e. a namespace will be included if it matches any selector. The following example selects any namespace that matches either below: 1. The namespace has both of these labels: `env: prod` and `region: us-east1` 2. The namespace has label `app` equal to `cassandra` or `spark`. ```yaml discoverySelectors:   - matchLabels:     env: prod     region: us-east1   - matchExpressions:   - key: app     operator: In     values:   - cassandra   - spark  ``` Refer to the [Kubernetes selector docs](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors) for additional detail on selector semantics. |  |  |
| `pathNormalization` _[MeshConfigProxyPathNormalization](#meshconfigproxypathnormalization)_ | ProxyPathNormalization configures how URL paths in incoming and outgoing HTTP requests are normalized by the sidecars and gateways. The normalized paths will be used in all aspects through the requests' lifetime on the sidecars and gateways, which includes routing decisions in outbound direction (client proxy), authorization policy match and enforcement in inbound direction (server proxy), and the URL path proxied to the upstream service. If not set, the NormalizationType.DEFAULT configuration will be used. |  |  |
| `defaultHttpRetryPolicy` _[HTTPRetry](#httpretry)_ | Configure the default HTTP retry policy. The default number of retry attempts is set at 2 for these errors:    "connect-failure,refused-stream,unavailable,cancelled,retriable-status-codes".  Setting the number of attempts to 0 disables retry policy globally. This setting can be overridden on a per-host basis using the Virtual Service API. All settings in the retry policy except `perTryTimeout` can currently be configured globally via this field. |  |  |
| `meshMTLS` _[MeshConfigTLSConfig](#meshconfigtlsconfig)_ | The below configuration parameters can be used to specify TLSConfig for mesh traffic. For example, a user could enable min TLS version for ISTIO_MUTUAL traffic and specify a curve for non ISTIO_MUTUAL traffic like below: ```yaml meshConfig:    meshMTLS:     minProtocolVersion: TLSV1_3   tlsDefaults:     Note: applicable only for non ISTIO_MUTUAL scenarios     ecdhCurves:       - P-256       - P-512  ``` Configuration of mTLS for traffic between workloads with ISTIO_MUTUAL TLS traffic.  Note: Mesh mTLS does not respect ECDH curves. |  |  |
| `tlsDefaults` _[MeshConfigTLSConfig](#meshconfigtlsconfig)_ | Configuration of TLS for all traffic except for ISTIO_MUTUAL mode. Currently, this supports configuration of ecdh_curves and cipher_suites only. For ISTIO_MUTUAL TLS settings, use meshMTLS configuration. |  |  |


#### MeshConfigAccessLogEncoding

_Underlying type:_ _string_



_Validation:_
- Enum: [TEXT JSON]

_Appears in:_
- [MeshConfig](#meshconfig)

| Field | Description |
| --- | --- |
| `TEXT` | text encoding for the proxy access log  |
| `JSON` | json encoding for the proxy access log  |




#### MeshConfigCA







_Appears in:_
- [MeshConfig](#meshconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `address` _string_ | REQUIRED. Address of the CA server implementing the Istio CA gRPC API. Can be IP address or a fully qualified DNS name with port Eg: custom-ca.default.svc.cluster.local:8932, 192.168.23.2:9000 |  | Required: \{\}   |
| `tlsSettings` _[ClientTLSSettings](#clienttlssettings)_ | Use the tls_settings to specify the tls mode to use. Regarding tls_settings: - DISABLE MODE is legitimate for the case Istiod is making the request via an Envoy sidecar. DISABLE MODE can also be used for testing - TLS MUTUAL MODE be on by default. If the CA certificates (cert bundle to verify the CA server's certificate) is omitted, Istiod will use the system root certs to verify the CA server's certificate. |  |  |
| `requestTimeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#duration-v1-meta)_ | timeout for forward CSR requests from Istiod to External CA Default: 10s |  |  |
| `istiodSide` _boolean_ | Use istiod_side to specify CA Server integrate to Istiod side or Agent side Default: true |  |  |


#### MeshConfigCertificateData







_Appears in:_
- [MeshConfig](#meshconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `pem` _string_ | The PEM data of the certificate. |  |  |
| `spiffeBundleUrl` _string_ | The SPIFFE bundle endpoint URL that complies to: https://github.com/spiffe/spiffe/blob/master/standards/SPIFFE_Trust_Domain_and_Bundle.md#the-spiffe-trust-domain-and-bundle The endpoint should support authentication based on Web PKI: https://github.com/spiffe/spiffe/blob/master/standards/SPIFFE_Trust_Domain_and_Bundle.md#521-web-pki The certificate is retrieved from the endpoint. |  |  |
| `certSigners` _string array_ | when Istiod is acting as RA(registration authority) If set, they are used for these signers. Otherwise, this trustAnchor is used for all signers. |  |  |
| `trustDomains` _string array_ | Optional. Specify the list of trust domains to which this trustAnchor data belongs. If set, they are used for these trust domains. Otherwise, this trustAnchor is used for default trust domain and its aliases. Note that we can have multiple trustAnchor data for a same trust_domain. In that case, trustAnchors with a same trust domain will be merged and used together to verify peer certificates. If neither cert_signers nor trust_domains is set, this trustAnchor is used for all trust domains and all signers. If only trust_domains is set, this trustAnchor is used for these trust_domains and all signers. If only cert_signers is set, this trustAnchor is used for these cert_signers and all trust domains. If both cert_signers and trust_domains is set, this trustAnchor is only used for these signers and trust domains. |  |  |


#### MeshConfigDefaultProviders



Holds the name references to the providers that will be used by default
in other Istio configuration resources if the provider is not specified.


These names must match a provider defined in `extension_providers` that is
one of the supported tracing providers.



_Appears in:_
- [MeshConfig](#meshconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `tracing` _string array_ | Name of the default provider(s) for tracing. |  |  |
| `metrics` _string array_ | Name of the default provider(s) for metrics. |  |  |
| `accessLogging` _string array_ | Name of the default provider(s) for access logging. |  |  |


#### MeshConfigExtensionProvider







_Appears in:_
- [MeshConfig](#meshconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | REQUIRED. A unique name identifying the extension provider. |  | Required: \{\}   |
| `envoyExtAuthzHttp` _[MeshConfigExtensionProviderEnvoyExternalAuthorizationHttpProvider](#meshconfigextensionproviderenvoyexternalauthorizationhttpprovider)_ | Configures an external authorizer that implements the Envoy ext_authz filter authorization check service using the HTTP API. |  |  |
| `envoyExtAuthzGrpc` _[MeshConfigExtensionProviderEnvoyExternalAuthorizationGrpcProvider](#meshconfigextensionproviderenvoyexternalauthorizationgrpcprovider)_ | Configures an external authorizer that implements the Envoy ext_authz filter authorization check service using the gRPC API. |  |  |
| `zipkin` _[MeshConfigExtensionProviderZipkinTracingProvider](#meshconfigextensionproviderzipkintracingprovider)_ | Configures a tracing provider that uses the Zipkin API. |  |  |
| `datadog` _[MeshConfigExtensionProviderDatadogTracingProvider](#meshconfigextensionproviderdatadogtracingprovider)_ | Configures a Datadog tracing provider. |  |  |
| `stackdriver` _[MeshConfigExtensionProviderStackdriverProvider](#meshconfigextensionproviderstackdriverprovider)_ | Configures a Stackdriver provider. |  |  |
| `skywalking` _[MeshConfigExtensionProviderSkyWalkingTracingProvider](#meshconfigextensionproviderskywalkingtracingprovider)_ | Configures a Apache SkyWalking provider. |  |  |
| `opentelemetry` _[MeshConfigExtensionProviderOpenTelemetryTracingProvider](#meshconfigextensionprovideropentelemetrytracingprovider)_ | Configures an OpenTelemetry tracing provider. |  |  |
| `prometheus` _[MeshConfigExtensionProviderPrometheusMetricsProvider](#meshconfigextensionproviderprometheusmetricsprovider)_ | Configures a Prometheus metrics provider. |  |  |
| `envoyFileAccessLog` _[MeshConfigExtensionProviderEnvoyFileAccessLogProvider](#meshconfigextensionproviderenvoyfileaccesslogprovider)_ | Configures an Envoy File Access Log provider. |  |  |
| `envoyHttpAls` _[MeshConfigExtensionProviderEnvoyHttpGrpcV3LogProvider](#meshconfigextensionproviderenvoyhttpgrpcv3logprovider)_ | Configures an Envoy Access Logging Service provider for HTTP traffic. |  |  |
| `envoyTcpAls` _[MeshConfigExtensionProviderEnvoyTcpGrpcV3LogProvider](#meshconfigextensionproviderenvoytcpgrpcv3logprovider)_ | Configures an Envoy Access Logging Service provider for TCP traffic. |  |  |
| `envoyOtelAls` _[MeshConfigExtensionProviderEnvoyOpenTelemetryLogProvider](#meshconfigextensionproviderenvoyopentelemetrylogprovider)_ | Configures an Envoy Open Telemetry Access Logging Service provider. |  |  |


#### MeshConfigExtensionProviderDatadogTracingProvider



Defines configuration for a Datadog tracer.



_Appears in:_
- [MeshConfigExtensionProvider](#meshconfigextensionprovider)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `service` _string_ | REQUIRED. Specifies the service for the Datadog agent. The format is `[<Namespace>/]<Hostname>`. The specification of `<Namespace>` is required only when it is insufficient to unambiguously resolve a service in the service registry. The `<Hostname>` is a fully qualified host name of a service defined by the Kubernetes service or ServiceEntry.  Example: "datadog.default.svc.cluster.local" or "bar/datadog.example.com". |  | Required: \{\}   |
| `port` _integer_ | REQUIRED. Specifies the port of the service. |  | Required: \{\}   |
| `maxTagLength` _integer_ | Optional. Controls the overall path length allowed in a reported span. NOTE: currently only controls max length of the path tag. |  |  |


#### MeshConfigExtensionProviderEnvoyExternalAuthorizationGrpcProvider







_Appears in:_
- [MeshConfigExtensionProvider](#meshconfigextensionprovider)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `service` _string_ | REQUIRED. Specifies the service that implements the Envoy ext_authz gRPC authorization service. The format is `[<Namespace>/]<Hostname>`. The specification of `<Namespace>` is required only when it is insufficient to unambiguously resolve a service in the service registry. The `<Hostname>` is a fully qualified host name of a service defined by the Kubernetes service or ServiceEntry.  Example: "my-ext-authz.foo.svc.cluster.local" or "bar/my-ext-authz.example.com". |  | Required: \{\}   |
| `port` _integer_ | REQUIRED. Specifies the port of the service. |  | Required: \{\}   |
| `timeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#duration-v1-meta)_ | The maximum duration that the proxy will wait for a response from the provider, this is the timeout for a specific request (default timeout: 600s). When this timeout condition is met, the proxy marks the communication to the authorization service as failure. In this situation, the response sent back to the client will depend on the configured `fail_open` field. |  |  |
| `failOpen` _boolean_ | If true, the HTTP request or TCP connection will be allowed even if the communication with the authorization service has failed, or if the authorization service has returned a HTTP 5xx error. Default is false. For HTTP request, it will be rejected with 403 (HTTP Forbidden). For TCP connection, it will be closed immediately. |  |  |
| `statusOnError` _string_ | Sets the HTTP status that is returned to the client when there is a network error to the authorization service. The default status is "403" (HTTP Forbidden). |  |  |
| `includeRequestBodyInCheck` _[MeshConfigExtensionProviderEnvoyExternalAuthorizationRequestBody](#meshconfigextensionproviderenvoyexternalauthorizationrequestbody)_ | If set, the client request body will be included in the authorization request sent to the authorization service. |  |  |


#### MeshConfigExtensionProviderEnvoyExternalAuthorizationHttpProvider







_Appears in:_
- [MeshConfigExtensionProvider](#meshconfigextensionprovider)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `service` _string_ | REQUIRED. Specifies the service that implements the Envoy ext_authz HTTP authorization service. The format is `[<Namespace>/]<Hostname>`. The specification of `<Namespace>` is required only when it is insufficient to unambiguously resolve a service in the service registry. The `<Hostname>` is a fully qualified host name of a service defined by the Kubernetes service or ServiceEntry.  Example: "my-ext-authz.foo.svc.cluster.local" or "bar/my-ext-authz.example.com". |  | Required: \{\}   |
| `port` _integer_ | REQUIRED. Specifies the port of the service. |  | Required: \{\}   |
| `timeout` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#duration-v1-meta)_ | The maximum duration that the proxy will wait for a response from the provider (default timeout: 600s). When this timeout condition is met, the proxy marks the communication to the authorization service as failure. In this situation, the response sent back to the client will depend on the configured `fail_open` field. |  |  |
| `pathPrefix` _string_ | Sets a prefix to the value of authorization request header *Path*. For example, setting this to "/check" for an original user request at path "/admin" will cause the authorization check request to be sent to the authorization service at the path "/check/admin" instead of "/admin". |  |  |
| `failOpen` _boolean_ | If true, the user request will be allowed even if the communication with the authorization service has failed, or if the authorization service has returned a HTTP 5xx error. Default is false and the request will be rejected with "Forbidden" response. |  |  |
| `statusOnError` _string_ | Sets the HTTP status that is returned to the client when there is a network error to the authorization service. The default status is "403" (HTTP Forbidden). |  |  |
| `includeHeadersInCheck` _string array_ | DEPRECATED. Use include_request_headers_in_check instead.  Deprecated: Marked as deprecated in mesh/v1alpha1/config.proto. |  |  |
| `includeRequestHeadersInCheck` _string array_ | List of client request headers that should be included in the authorization request sent to the authorization service. Note that in addition to the headers specified here following headers are included by default: 1. *Host*, *Method*, *Path* and *Content-Length* are automatically sent. 2. *Content-Length* will be set to 0 and the request will not have a message body. However, the authorization request can include the buffered client request body (controlled by include_request_body_in_check setting), consequently the value of Content-Length of the authorization request reflects the size of its payload size.  Exact, prefix and suffix matches are supported (similar to the [authorization policy rule syntax](https://istio.io/latest/docs/reference/config/security/authorization-policy/#Rule) except the presence match): - Exact match: "abc" will match on value "abc". - Prefix match: "abc*" will match on value "abc" and "abcd". - Suffix match: "*abc" will match on value "abc" and "xabc". |  |  |
| `includeAdditionalHeadersInCheck` _object (keys:string, values:string)_ | Set of additional fixed headers that should be included in the authorization request sent to the authorization service. Key is the header name and value is the header value. Note that client request of the same key or headers specified in include_request_headers_in_check will be overridden. |  |  |
| `includeRequestBodyInCheck` _[MeshConfigExtensionProviderEnvoyExternalAuthorizationRequestBody](#meshconfigextensionproviderenvoyexternalauthorizationrequestbody)_ | If set, the client request body will be included in the authorization request sent to the authorization service. |  |  |
| `headersToUpstreamOnAllow` _string array_ | List of headers from the authorization service that should be added or overridden in the original request and forwarded to the upstream when the authorization check result is allowed (HTTP code 200). If not specified, the original request will not be modified and forwarded to backend as-is. Note, any existing headers will be overridden.  Exact, prefix and suffix matches are supported (similar to the [authorization policy rule syntax](https://istio.io/latest/docs/reference/config/security/authorization-policy/#Rule) except the presence match): - Exact match: "abc" will match on value "abc". - Prefix match: "abc*" will match on value "abc" and "abcd". - Suffix match: "*abc" will match on value "abc" and "xabc". |  |  |
| `headersToDownstreamOnDeny` _string array_ | List of headers from the authorization service that should be forwarded to downstream when the authorization check result is not allowed (HTTP code other than 200). If not specified, all the authorization response headers, except *Authority (Host)* will be in the response to the downstream. When a header is included in this list, *Path*, *Status*, *Content-Length*, *WWWAuthenticate* and *Location* are automatically added. Note, the body from the authorization service is always included in the response to downstream.  Exact, prefix and suffix matches are supported (similar to the [authorization policy rule syntax](https://istio.io/latest/docs/reference/config/security/authorization-policy/#Rule) except the presence match): - Exact match: "abc" will match on value "abc". - Prefix match: "abc*" will match on value "abc" and "abcd". - Suffix match: "*abc" will match on value "abc" and "xabc". |  |  |
| `headersToDownstreamOnAllow` _string array_ | List of headers from the authorization service that should be forwarded to downstream when the authorization check result is allowed (HTTP code 200). If not specified, the original response will not be modified and forwarded to downstream as-is. Note, any existing headers will be overridden.  Exact, prefix and suffix matches are supported (similar to the [authorization policy rule syntax](https://istio.io/latest/docs/reference/config/security/authorization-policy/#Rule) except the presence match): - Exact match: "abc" will match on value "abc". - Prefix match: "abc*" will match on value "abc" and "abcd". - Suffix match: "*abc" will match on value "abc" and "xabc". |  |  |




#### MeshConfigExtensionProviderEnvoyFileAccessLogProvider



Defines configuration for Envoy-based access logging that writes to
local files (and/or standard streams).



_Appears in:_
- [MeshConfigExtensionProvider](#meshconfigextensionprovider)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `path` _string_ | Path to a local file to write the access log entries. This may be used to write to streams, via `/dev/stderr` and `/dev/stdout` If unspecified, defaults to `/dev/stdout`. |  |  |
| `logFormat` _[MeshConfigExtensionProviderEnvoyFileAccessLogProviderLogFormat](#meshconfigextensionproviderenvoyfileaccesslogproviderlogformat)_ | Optional. Allows overriding of the default access log format. |  |  |




#### MeshConfigExtensionProviderEnvoyHttpGrpcV3LogProvider



Defines configuration for an Envoy [Access Logging Service](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/access_loggers/grpc/v3/als.proto#grpc-access-log-service-als)
integration for HTTP traffic.



_Appears in:_
- [MeshConfigExtensionProvider](#meshconfigextensionprovider)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `service` _string_ | REQUIRED. Specifies the service that implements the Envoy ALS gRPC authorization service. The format is `[<Namespace>/]<Hostname>`. The specification of `<Namespace>` is required only when it is insufficient to unambiguously resolve a service in the service registry. The `<Hostname>` is a fully qualified host name of a service defined by the Kubernetes service or ServiceEntry.  Example: "envoy-als.foo.svc.cluster.local" or "bar/envoy-als.example.com". |  | Required: \{\}   |
| `port` _integer_ | REQUIRED. Specifies the port of the service. |  | Required: \{\}   |
| `logName` _string_ | Optional. The friendly name of the access log. Defaults: -  "http_envoy_accesslog" -  "listener_envoy_accesslog" |  |  |
| `filterStateObjectsToLog` _string array_ | Optional. Additional filter state objects to log. |  |  |
| `additionalRequestHeadersToLog` _string array_ | Optional. Additional request headers to log. |  |  |
| `additionalResponseHeadersToLog` _string array_ | Optional. Additional response headers to log. |  |  |
| `additionalResponseTrailersToLog` _string array_ | Optional. Additional response trailers to log. |  |  |


#### MeshConfigExtensionProviderEnvoyOpenTelemetryLogProvider



Defines configuration for an Envoy [OpenTelemetry (gRPC) Access Log](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/access_loggers/open_telemetry/v3/logs_service.proto)



_Appears in:_
- [MeshConfigExtensionProvider](#meshconfigextensionprovider)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `service` _string_ | REQUIRED. Specifies the service that implements the Envoy ALS gRPC authorization service. The format is `[<Namespace>/]<Hostname>`. The specification of `<Namespace>` is required only when it is insufficient to unambiguously resolve a service in the service registry. The `<Hostname>` is a fully qualified host name of a service defined by the Kubernetes service or ServiceEntry.  Example: "envoy-als.foo.svc.cluster.local" or "bar/envoy-als.example.com". |  | Required: \{\}   |
| `port` _integer_ | REQUIRED. Specifies the port of the service. |  | Required: \{\}   |
| `logName` _string_ | Optional. The friendly name of the access log. Defaults: - "otel_envoy_accesslog" |  |  |
| `logFormat` _[MeshConfigExtensionProviderEnvoyOpenTelemetryLogProviderLogFormat](#meshconfigextensionproviderenvoyopentelemetrylogproviderlogformat)_ | Optional. Format for the proxy access log Empty value results in proxy's default access log format, following Envoy access logging formatting. |  |  |




#### MeshConfigExtensionProviderEnvoyTcpGrpcV3LogProvider



Defines configuration for an Envoy [Access Logging Service](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/access_loggers/grpc/v3/als.proto#grpc-access-log-service-als)
integration for TCP traffic.



_Appears in:_
- [MeshConfigExtensionProvider](#meshconfigextensionprovider)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `service` _string_ | REQUIRED. Specifies the service that implements the Envoy ALS gRPC authorization service. The format is `[<Namespace>/]<Hostname>`. The specification of `<Namespace>` is required only when it is insufficient to unambiguously resolve a service in the service registry. The `<Hostname>` is a fully qualified host name of a service defined by the Kubernetes service or ServiceEntry.  Example: "envoy-als.foo.svc.cluster.local" or "bar/envoy-als.example.com". |  | Required: \{\}   |
| `port` _integer_ | REQUIRED. Specifies the port of the service. |  | Required: \{\}   |
| `logName` _string_ | Optional. The friendly name of the access log. Defaults: - "tcp_envoy_accesslog" - "listener_envoy_accesslog" |  |  |
| `filterStateObjectsToLog` _string array_ | Optional. Additional filter state objects to log. |  |  |


#### MeshConfigExtensionProviderHttpHeader







_Appears in:_
- [MeshConfigExtensionProviderHttpService](#meshconfigextensionproviderhttpservice)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | REQUIRED. The HTTP header name. |  | Required: \{\}   |
| `value` _string_ | REQUIRED. The HTTP header value. |  | Required: \{\}   |




#### MeshConfigExtensionProviderLightstepTracingProvider



Defines configuration for a Lightstep tracer.
Note: Lightstep has moved to OpenTelemetry-based integrations. Istio 1.15+
will generate OpenTelemetry-compatible configuration when using this option.



_Appears in:_
- [MeshConfigExtensionProvider](#meshconfigextensionprovider)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `service` _string_ | REQUIRED. Specifies the service for the Lightstep collector. The format is `[<Namespace>/]<Hostname>`. The specification of `<Namespace>` is required only when it is insufficient to unambiguously resolve a service in the service registry. The `<Hostname>` is a fully qualified host name of a service defined by the Kubernetes service or ServiceEntry.  Example: "lightstep.default.svc.cluster.local" or "bar/lightstep.example.com". |  | Required: \{\}   |
| `port` _integer_ | REQUIRED. Specifies the port of the service. |  | Required: \{\}   |
| `accessToken` _string_ | The Lightstep access token. |  |  |
| `maxTagLength` _integer_ | Optional. Controls the overall path length allowed in a reported span. NOTE: currently only controls max length of the path tag. |  |  |


#### MeshConfigExtensionProviderOpenCensusAgentTracingProvider



Defines configuration for an OpenCensus tracer writing to an OpenCensus backend.


WARNING: OpenCensusAgentTracingProviders should be used with extreme care. Configuration of
OpenCensus providers CANNOT be changed during the course of proxy's lifetime due to a limitation
in the implementation of OpenCensus driver in Envoy. This means only a single provider configuration
may be used for OpenCensus at any given time for a proxy or group of proxies AND that any change to the provider
configuration MUST be accompanied by a restart of all proxies that will use that configuration.


NOTE: Stackdriver tracing uses OpenCensus configuration under the hood and, as a result, cannot be used
alongside OpenCensus provider configuration.



_Appears in:_
- [MeshConfigExtensionProvider](#meshconfigextensionprovider)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `service` _string_ | REQUIRED. Specifies the service for the OpenCensusAgent. The format is `[<Namespace>/]<Hostname>`. The specification of `<Namespace>` is required only when it is insufficient to unambiguously resolve a service in the service registry. The `<Hostname>` is a fully qualified host name of a service defined by the Kubernetes service or ServiceEntry.  Example: "ocagent.default.svc.cluster.local" or "bar/ocagent.example.com". |  | Required: \{\}   |
| `port` _integer_ | REQUIRED. Specifies the port of the service. |  | Required: \{\}   |
| `context` _[MeshConfigExtensionProviderOpenCensusAgentTracingProviderTraceContext](#meshconfigextensionprovideropencensusagenttracingprovidertracecontext) array_ | Specifies the set of context propagation headers used for distributed tracing. Default is `["W3C_TRACE_CONTEXT"]`. If multiple values are specified, the proxy will attempt to read each header for each request and will write all headers. |  | Enum: [UNSPECIFIED W3C_TRACE_CONTEXT GRPC_BIN CLOUD_TRACE_CONTEXT B3]   |
| `maxTagLength` _integer_ | Optional. Controls the overall path length allowed in a reported span. NOTE: currently only controls max length of the path tag. |  |  |




#### MeshConfigExtensionProviderOpenTelemetryTracingProvider



Defines configuration for an OpenTelemetry tracing backend. Istio 1.16.1 or higher is needed.



_Appears in:_
- [MeshConfigExtensionProvider](#meshconfigextensionprovider)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `service` _string_ | REQUIRED. Specifies the OpenTelemetry endpoint that will receive OTLP traces. The format is `[<Namespace>/]<Hostname>`. The specification of `<Namespace>` is required only when it is insufficient to unambiguously resolve a service in the service registry. The `<Hostname>` is a fully qualified host name of a service defined by the Kubernetes service or ServiceEntry.  Example: "otlp.default.svc.cluster.local" or "bar/otlp.example.com". |  | Required: \{\}   |
| `port` _integer_ | REQUIRED. Specifies the port of the service. |  | Required: \{\}   |
| `maxTagLength` _integer_ | Optional. Controls the overall path length allowed in a reported span. NOTE: currently only controls max length of the path tag. |  |  |
| `http` _[MeshConfigExtensionProviderHttpService](#meshconfigextensionproviderhttpservice)_ | Optional. Specifies the configuration for exporting OTLP traces via HTTP. When empty, traces will be exported via gRPC.  The following example shows how to configure the OpenTelemetry ExtensionProvider to export via HTTP:  1. Add/change the OpenTelemetry extension provider in `MeshConfig` ```yaml   - name: otel-tracing     opentelemetry:     port: 443     service: my.olly-backend.com     http:     path: "/api/otlp/traces"     timeout: 10s     headers:   - name: "my-custom-header"     value: "some value"  ```  2. Deploy a `ServiceEntry` for the observability back-end ```yaml apiVersion: networking.istio.io/v1alpha3 kind: ServiceEntry metadata:    name: my-olly-backend  spec:    hosts:   - my.olly-backend.com   ports:   - number: 443     name: https-port     protocol: HTTPS   resolution: DNS   location: MESH_EXTERNAL  --- apiVersion: networking.istio.io/v1alpha3 kind: DestinationRule metadata:    name: my-olly-backend  spec:    host: my.olly-backend.com   trafficPolicy:     portLevelSettings:     - port:         number: 443       tls:         mode: SIMPLE  ``` |  |  |
| `resourceDetectors` _[MeshConfigExtensionProviderResourceDetectors](#meshconfigextensionproviderresourcedetectors)_ | Optional. Specifies [Resource Detectors](https://opentelemetry.io/docs/specs/otel/resource/sdk/) to be used by the OpenTelemetry Tracer. When multiple resources are provided, they are merged according to the OpenTelemetry [Resource specification](https://opentelemetry.io/docs/specs/otel/resource/sdk/#merge).  The following example shows how to configure the Environment Resource Detector, that will read the attributes from the environment variable `OTEL_RESOURCE_ATTRIBUTES`:  ```yaml   - name: otel-tracing     opentelemetry:     port: 443     service: my.olly-backend.com     resource_detectors:     environment: \{\}  ``` |  |  |
| `dynatraceSampler` _[MeshConfigExtensionProviderOpenTelemetryTracingProviderDynatraceSampler](#meshconfigextensionprovideropentelemetrytracingproviderdynatracesampler)_ | The Dynatrace adaptive traffic management (ATM) sampler.  Example configuration:  ```yaml   - name: otel-tracing     opentelemetry:     port: 443     service: "\{your-environment-id\}.live.dynatrace.com"     http:     path: "/api/v2/otlp/v1/traces"     timeout: 10s     headers:   - name: "Authorization"     value: "Api-Token dt0c01."     resource_detectors:     dynatrace: \{\}     dynatrace_sampler:     tenant: "\{your-environment-id\}"     cluster_id: 1234 |  |  |




#### MeshConfigExtensionProviderOpenTelemetryTracingProviderDynatraceSamplerDynatraceApi







_Appears in:_
- [MeshConfigExtensionProviderOpenTelemetryTracingProviderDynatraceSampler](#meshconfigextensionprovideropentelemetrytracingproviderdynatracesampler)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `service` _string_ | REQUIRED. Specifies the Dynatrace environment to obtain the sampling configuration. The format is `<Hostname>`, where `<Hostname>` is the fully qualified Dynatrace environment host name defined in the ServiceEntry.  Example: "\{your-environment-id\}.live.dynatrace.com". |  | Required: \{\}   |
| `port` _integer_ | REQUIRED. Specifies the port of the service. |  | Required: \{\}   |
| `http` _[MeshConfigExtensionProviderHttpService](#meshconfigextensionproviderhttpservice)_ | REQUIRED. Specifies sampling configuration URI. |  | Required: \{\}   |


#### MeshConfigExtensionProviderPrometheusMetricsProvider







_Appears in:_
- [MeshConfigExtensionProvider](#meshconfigextensionprovider)





#### MeshConfigExtensionProviderResourceDetectorsDynatraceResourceDetector



Dynatrace Resource Detector.
The resource detector reads from the Dynatrace enrichment files
and adds host/process related attributes to the OpenTelemetry resource.


See: [Enrich ingested data with Dynatrace-specific dimensions](https://docs.dynatrace.com/docs/shortlink/enrichment-files)



_Appears in:_
- [MeshConfigExtensionProviderResourceDetectors](#meshconfigextensionproviderresourcedetectors)



#### MeshConfigExtensionProviderResourceDetectorsEnvironmentResourceDetector



OpenTelemetry Environment Resource Detector.
The resource detector reads attributes from the environment variable `OTEL_RESOURCE_ATTRIBUTES`
and adds them to the OpenTelemetry resource.


See: [Resource specification](https://opentelemetry.io/docs/specs/otel/resource/sdk/#specifying-resource-information-via-an-environment-variable)



_Appears in:_
- [MeshConfigExtensionProviderResourceDetectors](#meshconfigextensionproviderresourcedetectors)



#### MeshConfigExtensionProviderSkyWalkingTracingProvider



Defines configuration for a SkyWalking tracer.



_Appears in:_
- [MeshConfigExtensionProvider](#meshconfigextensionprovider)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `service` _string_ | REQUIRED. Specifies the service for the SkyWalking receiver. The format is `[<Namespace>/]<Hostname>`. The specification of `<Namespace>` is required only when it is insufficient to unambiguously resolve a service in the service registry. The `<Hostname>` is a fully qualified host name of a service defined by the Kubernetes service or ServiceEntry.  Example: "skywalking.default.svc.cluster.local" or "bar/skywalking.example.com". |  | Required: \{\}   |
| `port` _integer_ | REQUIRED. Specifies the port of the service. |  | Required: \{\}   |
| `accessToken` _string_ | Optional. The SkyWalking OAP access token. |  |  |


#### MeshConfigExtensionProviderStackdriverProvider



Defines configuration for Stackdriver.


WARNING: Stackdriver tracing uses OpenCensus configuration under the hood and, as a result, cannot be used
alongside any OpenCensus provider configuration. This is due to a limitation in the implementation of OpenCensus
driver in Envoy.



_Appears in:_
- [MeshConfigExtensionProvider](#meshconfigextensionprovider)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `maxTagLength` _integer_ | Optional. Controls the overall path length allowed in a reported span. NOTE: currently only controls max length of the path tag. |  |  |
| `logging` _[MeshConfigExtensionProviderStackdriverProviderLogging](#meshconfigextensionproviderstackdriverproviderlogging)_ | Optional. Controls Stackdriver logging behavior. |  |  |




#### MeshConfigExtensionProviderZipkinTracingProvider



Defines configuration for a Zipkin tracer.



_Appears in:_
- [MeshConfigExtensionProvider](#meshconfigextensionprovider)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `service` _string_ | REQUIRED. Specifies the service that the Zipkin API. The format is `[<Namespace>/]<Hostname>`. The specification of `<Namespace>` is required only when it is insufficient to unambiguously resolve a service in the service registry. The `<Hostname>` is a fully qualified host name of a service defined by the Kubernetes service or ServiceEntry.  Example: "zipkin.default.svc.cluster.local" or "bar/zipkin.example.com". |  | Required: \{\}   |
| `port` _integer_ | REQUIRED. Specifies the port of the service. |  | Required: \{\}   |
| `maxTagLength` _integer_ | Optional. Controls the overall path length allowed in a reported span. NOTE: currently only controls max length of the path tag. |  |  |
| `enable64bitTraceId` _boolean_ | Optional. A 128 bit trace id will be used in Istio. If true, will result in a 64 bit trace id being used. |  |  |


#### MeshConfigH2UpgradePolicy

_Underlying type:_ _string_

Default Policy for upgrading http1.1 connections to http2.

_Validation:_
- Enum: [DO_NOT_UPGRADE UPGRADE]

_Appears in:_
- [MeshConfig](#meshconfig)

| Field | Description |
| --- | --- |
| `DO_NOT_UPGRADE` | Do not upgrade connections to http2.  |
| `UPGRADE` | Upgrade the connections to http2.  |


#### MeshConfigInboundTrafficPolicy







_Appears in:_
- [MeshConfig](#meshconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `mode` _[MeshConfigInboundTrafficPolicyMode](#meshconfiginboundtrafficpolicymode)_ |  |  | Enum: [PASSTHROUGH LOCALHOST]   |


#### MeshConfigInboundTrafficPolicyMode

_Underlying type:_ _string_



_Validation:_
- Enum: [PASSTHROUGH LOCALHOST]

_Appears in:_
- [MeshConfigInboundTrafficPolicy](#meshconfiginboundtrafficpolicy)

| Field | Description |
| --- | --- |
| `PASSTHROUGH` | inbound traffic will be passed through to the destination listening on Pod IP. This matches the behavior without Istio enabled at all allowing proxy to be transparent.  |
| `LOCALHOST` | inbound traffic will be sent to the destinations listening on localhost.  |


#### MeshConfigIngressControllerMode

_Underlying type:_ _string_



_Validation:_
- Enum: [UNSPECIFIED OFF DEFAULT STRICT]

_Appears in:_
- [MeshConfig](#meshconfig)

| Field | Description |
| --- | --- |
| `UNSPECIFIED` | Unspecified Istio ingress controller.  |
| `OFF` | Disables Istio ingress controller.  |
| `DEFAULT` | Istio ingress controller will act on ingress resources that do not contain any annotation or whose annotations match the value specified in the ingress_class parameter described earlier. Use this mode if Istio ingress controller will be the default ingress controller for the entire Kubernetes cluster.  |
| `STRICT` | Istio ingress controller will only act on ingress resources whose annotations match the value specified in the ingress_class parameter described earlier. Use this mode if Istio ingress controller will be a secondary ingress controller (e.g., in addition to a cloud-provided ingress controller).  |


#### MeshConfigOutboundTrafficPolicy



`OutboundTrafficPolicy` sets the default behavior of the sidecar for
handling unknown outbound traffic from the application.



_Appears in:_
- [MeshConfig](#meshconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `mode` _[MeshConfigOutboundTrafficPolicyMode](#meshconfigoutboundtrafficpolicymode)_ |  |  | Enum: [REGISTRY_ONLY ALLOW_ANY]   |


#### MeshConfigOutboundTrafficPolicyMode

_Underlying type:_ _string_



_Validation:_
- Enum: [REGISTRY_ONLY ALLOW_ANY]

_Appears in:_
- [MeshConfigOutboundTrafficPolicy](#meshconfigoutboundtrafficpolicy)

| Field | Description |
| --- | --- |
| `REGISTRY_ONLY` | In `REGISTRY_ONLY` mode, unknown outbound traffic will be dropped. Traffic destinations must be explicitly declared into the service registry through `ServiceEntry` configurations. Note: Istio [does not offer an outbound traffic security policy](https://istio.io/latest/docs/ops/best-practices/security/#understand-traffic-capture-limitations). This option does not act as one, or as any form of an outbound firewall. Instead, this option exists primarily to offer users a way to detect missing `ServiceEntry` configurations by explicitly failing.  |
| `ALLOW_ANY` | In `ALLOW_ANY` mode, any traffic to unknown destinations will be allowed. Unknown destination traffic will have limited functionality, however, such as reduced observability. This mode allows users that do not have all possible egress destinations registered through `ServiceEntry` configurations to still connect to arbitrary destinations.  |


#### MeshConfigProxyConfig



ProxyConfig defines variables for individual Envoy instances. This can be configured on a per-workload basis
as well as by the mesh-wide defaults.
To set the mesh wide defaults, configure the `defaultConfig` section of `meshConfig`. For example:


```
meshConfig:
  defaultConfig:
    discoveryAddress: istiod:15012
```


This can also be configured on a per-workload basis by configuring the `proxy.istio.io/config` annotation on the pod. For example:


```
annotations:
  proxy.istio.io/config: |
    discoveryAddress: istiod:15012
```


If both are configured, the two are merged with per field semantics; the field set in annotation will fully replace the field from mesh config defaults.
This is different than a deep merge provided by protobuf.
For example, `"tracing": { "sampling": 5 }` would completely override a setting configuring a tracing provider
such as `"tracing": { "zipkin": { "address": "..." } }`.


Note: fields in ProxyConfig are not dynamically configured; changes will require restart of workloads to take effect.



_Appears in:_
- [MeshConfig](#meshconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `configPath` _string_ | Path to the generated configuration file directory. Proxy agent generates the actual configuration and stores it in this directory. |  |  |
| `binaryPath` _string_ | Path to the proxy binary |  |  |
| `serviceCluster` _string_ | Service cluster defines the name for the `service_cluster` that is shared by all Envoy instances. This setting corresponds to `--service-cluster` flag in Envoy.  In a typical Envoy deployment, the `service-cluster` flag is used to identify the caller, for source-based routing scenarios.  Since Istio does not assign a local `service/service` version to each Envoy instance, the name is same for all of them.  However, the source/caller's identity (e.g., IP address) is encoded in the `--service-node` flag when launching Envoy.  When the RDS service receives API calls from Envoy, it uses the value of the `service-node` flag to compute routes that are relative to the service instances located at that IP address. |  |  |
| `tracingServiceName` _[ProxyConfigTracingServiceName](#proxyconfigtracingservicename)_ | Used by Envoy proxies to assign the values for the service names in trace spans. |  | Enum: [APP_LABEL_AND_NAMESPACE CANONICAL_NAME_ONLY CANONICAL_NAME_AND_NAMESPACE]   |
| `drainDuration` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#duration-v1-meta)_ | restart. MUST be >=1s (e.g., _1s/1m/1h_) Default drain duration is `45s`. |  |  |
| `discoveryAddress` _string_ | Address of the discovery service exposing xDS with mTLS connection. The inject configuration may override this value. |  |  |
| `zipkinAddress` _string_ | Address of the Zipkin service (e.g. _zipkin:9411_). DEPRECATED: Use [tracing][istio.mesh.v1alpha1.ProxyConfig.tracing] instead.  Deprecated: Marked as deprecated in mesh/v1alpha1/proxy.proto. |  |  |
| `statsdUdpAddress` _string_ | IP Address and Port of a statsd UDP listener (e.g. `10.75.241.127:9125`). |  |  |
| `proxyAdminPort` _integer_ | Port on which Envoy should listen for administrative commands. Default port is `15000`. |  |  |
| `controlPlaneAuthPolicy` _[AuthenticationPolicy](#authenticationpolicy)_ | AuthenticationPolicy defines how the proxy is authenticated when it connects to the control plane. Default is set to `MUTUAL_TLS`. |  | Enum: [NONE MUTUAL_TLS INHERIT]   |
| `customConfigFile` _string_ | File path of custom proxy configuration, currently used by proxies in front of Mixer and Pilot. |  |  |
| `statNameLength` _integer_ | Maximum length of name field in Envoy's metrics. The length of the name field is determined by the length of a name field in a service and the set of labels that comprise a particular version of the service. The default value is set to 189 characters. Envoy's internal metrics take up 67 characters, for a total of 256 character name per metric. Increase the value of this field if you find that the metrics from Envoys are truncated. |  |  |
| `concurrency` _integer_ | The number of worker threads to run. If unset, which is recommended, this will be automatically determined based on CPU requests/limits. If set to 0, all cores on the machine will be used, ignoring CPU requests or limits. This can lead to major performance issues if CPU limits are also set. |  |  |
| `proxyBootstrapTemplatePath` _string_ | Path to the proxy bootstrap template file |  |  |
| `interceptionMode` _[ProxyConfigInboundInterceptionMode](#proxyconfiginboundinterceptionmode)_ | The mode used to redirect inbound traffic to Envoy. |  | Enum: [REDIRECT TPROXY NONE]   |
| `tracing` _[Tracing](#tracing)_ | Tracing configuration to be used by the proxy. |  |  |
| `envoyAccessLogService` _[RemoteService](#remoteservice)_ | Address of the service to which access logs from Envoys should be sent. (e.g. `accesslog-service:15000`). See [Access Log Service](https://www.envoyproxy.io/docs/envoy/latest/api-v2/config/accesslog/v2/als.proto) for details about Envoy's gRPC Access Log Service API. |  |  |
| `envoyMetricsService` _[RemoteService](#remoteservice)_ | Address of the Envoy Metrics Service implementation (e.g. `metrics-service:15000`). See [Metric Service](https://www.envoyproxy.io/docs/envoy/latest/api-v2/config/metrics/v2/metrics_service.proto) for details about Envoy's Metrics Service API. |  |  |
| `proxyMetadata` _object (keys:string, values:string)_ | Additional environment variables for the proxy. Names starting with `ISTIO_META_` will be included in the generated bootstrap and sent to the XDS server. |  |  |
| `runtimeValues` _object (keys:string, values:string)_ | Envoy [runtime configuration](https://www.envoyproxy.io/docs/envoy/latest/intro/arch_overview/operations/runtime) to set during bootstrapping. This enables setting experimental, unsafe, unsupported, and deprecated features that should be used with extreme caution. |  |  |
| `statusPort` _integer_ | Port on which the agent should listen for administrative commands such as readiness probe. Default is set to port `15020`. |  |  |
| `extraStatTags` _string array_ | An additional list of tags to extract from the in-proxy Istio telemetry. These extra tags can be added by configuring the telemetry extension. Each additional tag needs to be present in this list. Extra tags emitted by the telemetry extensions must be listed here so that they can be processed and exposed as Prometheus metrics. Deprecated: `istio.stats` is a native filter now, this field is no longer needed. |  |  |
| `gatewayTopology` _[Topology](#topology)_ | Topology encapsulates the configuration which describes where the proxy is located i.e. behind a (or N) trusted proxy (proxies) or directly exposed to the internet. This configuration only effects gateways and is applied to all the gateways in the cluster unless overridden via annotations of the gateway workloads. |  |  |
| `terminationDrainDuration` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#duration-v1-meta)_ | The amount of time allowed for connections to complete on proxy shutdown. On receiving `SIGTERM` or `SIGINT`, `istio-agent` tells the active Envoy to start gracefully draining, discouraging any new connections and allowing existing connections to complete. It then sleeps for the `termination_drain_duration` and then kills any remaining active Envoy processes. If not set, a default of `5s` will be applied. |  |  |
| `meshId` _string_ | The unique identifier for the [service mesh](https://istio.io/docs/reference/glossary/#service-mesh) All control planes running in the same service mesh should specify the same mesh ID. Mesh ID is used to label telemetry reports for cases where telemetry from multiple meshes is mixed together. |  |  |
| `readinessProbe` _[Probe](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#probe-v1-core)_ | VM Health Checking readiness probe. This health check config exactly mirrors the kubernetes readiness probe configuration both in schema and logic. Only one health check method of 3 can be set at a time. |  |  |
| `proxyStatsMatcher` _[ProxyConfigProxyStatsMatcher](#proxyconfigproxystatsmatcher)_ | Proxy stats matcher defines configuration for reporting custom Envoy stats. To reduce memory and CPU overhead from Envoy stats system, Istio proxies by default create and expose only a subset of Envoy stats. This option is to control creation of additional Envoy stats with prefix, suffix, and regex expressions match on the name of the stats. This replaces the stats inclusion annotations (`sidecar.istio.io/statsInclusionPrefixes`, `sidecar.istio.io/statsInclusionRegexps`, and `sidecar.istio.io/statsInclusionSuffixes`). For example, to enable stats for circuit breakers, request retries, upstream connections, and request timeouts, you can specify stats matcher as follows: ```yaml proxyStatsMatcher:    inclusionRegexps:     - .*outlier_detection.*     - .*upstream_rq_retry.*     - .*upstream_cx_.*   inclusionSuffixes:     - upstream_rq_timeout  ``` Note including more Envoy stats might increase number of time series collected by prometheus significantly. Care needs to be taken on Prometheus resource provision and configuration to reduce cardinality. |  |  |
| `holdApplicationUntilProxyStarts` _boolean_ | Boolean flag for enabling/disabling the holdApplicationUntilProxyStarts behavior. This feature adds hooks to delay application startup until the pod proxy is ready to accept traffic, mitigating some startup race conditions. Default value is 'false'. |  |  |
| `caCertificatesPem` _string array_ | The PEM data of the extra root certificates for workload-to-workload communication. This includes the certificates defined in MeshConfig and any other certificates that Istiod uses as CA. The plugin certificates (the 'cacerts' secret), self-signed certificates (the 'istio-ca-secret' secret) are added automatically by Istiod. |  |  |
| `image` _[ProxyImage](#proxyimage)_ | Specifies the details of the proxy image. |  |  |
| `privateKeyProvider` _[PrivateKeyProvider](#privatekeyprovider)_ | Specifies the details of the Private Key Provider configuration for gateway and sidecar proxies. |  |  |
| `proxyHeaders` _[ProxyConfigProxyHeaders](#proxyconfigproxyheaders)_ | Define the set of headers to add/modify for HTTP request/responses.  To enable an optional header, simply set the field. If no specific configuration is required, an empty object (`\{\}`) will enable it. Note: currently all headers are enabled by default.  Below shows an example of customizing the `server` header and disabling the `X-Envoy-Attempt-Count` header:  ```yaml proxyHeaders:    server:     value: "my-custom-server"   requestId: \{\} // Explicitly enable Request IDs. As this is the default, this has no effect.   attemptCount:     disabled: true  ```  Some headers are enabled by default, and require explicitly disabling. See below for an example of disabling all default-enabled headers:  ```yaml proxyHeaders:    forwardedClientCert: SANITIZE   server:     disabled: true   requestId:     disabled: true   attemptCount:     disabled: true   envoyDebugHeaders:     disabled: true   metadataExchangeHeaders:     mode: IN_MESH  ``` |  |  |


#### MeshConfigProxyPathNormalization







_Appears in:_
- [MeshConfig](#meshconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `normalization` _[MeshConfigProxyPathNormalizationNormalizationType](#meshconfigproxypathnormalizationnormalizationtype)_ |  |  | Enum: [DEFAULT NONE BASE MERGE_SLASHES DECODE_AND_MERGE_SLASHES]   |


#### MeshConfigProxyPathNormalizationNormalizationType

_Underlying type:_ _string_



_Validation:_
- Enum: [DEFAULT NONE BASE MERGE_SLASHES DECODE_AND_MERGE_SLASHES]

_Appears in:_
- [MeshConfigProxyPathNormalization](#meshconfigproxypathnormalization)

| Field | Description |
| --- | --- |
| `DEFAULT` | Apply default normalizations. Currently, this is BASE.  |
| `NONE` | No normalization, paths are used as is.  |
| `BASE` | Normalize according to [RFC 3986](https://tools.ietf.org/html/rfc3986). For Envoy proxies, this is the [`normalize_path`](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/filters/network/http_connection_manager/v3/http_connection_manager.proto.html) option. For example, `/a/../b` normalizes to `/b`.  |
| `MERGE_SLASHES` | In addition to the `BASE` normalization, consecutive slashes are also merged. For example, `/a//b` normalizes to `a/b`.  |
| `DECODE_AND_MERGE_SLASHES` | In addition to normalization in `MERGE_SLASHES`, slash characters are UTF-8 decoded (case insensitive) prior to merging. This means `%2F`, `%2f`, `%5C`, and `%5c` sequences in the request path will be rewritten to `/` or `\`. For example, `/a%2f/b` normalizes to `a/b`.  |




#### MeshConfigServiceSettingsSettings



Settings for the selected services.



_Appears in:_
- [MeshConfigServiceSettings](#meshconfigservicesettings)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `clusterLocal` _boolean_ | If true, specifies that the client and service endpoints must reside in the same cluster. By default, in multi-cluster deployments, the Istio control plane assumes all service endpoints to be reachable from any client in any of the clusters which are part of the mesh. This configuration option limits the set of service endpoints visible to a client to be cluster scoped.  There are some common scenarios when this can be useful:    - A service (or group of services) is inherently local to the cluster and has local storage     for that cluster. For example, the kube-system namespace (e.g. the Kube API Server).   - A mesh administrator wants to slowly migrate services to Istio. They might start by first     having services cluster-local and then slowly transition them to mesh-wide. They could do     this service-by-service (e.g. mysvc.myns.svc.cluster.local) or as a group     (e.g. *.myns.svc.cluster.local).  By default Istio will consider kubernetes.default.svc (i.e. the API Server) as well as all services in the kube-system namespace to be cluster-local, unless explicitly overridden here. |  |  |


#### MeshConfigTLSConfig







_Appears in:_
- [MeshConfig](#meshconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `minProtocolVersion` _[MeshConfigTLSConfigTLSProtocol](#meshconfigtlsconfigtlsprotocol)_ | Optional: the minimum TLS protocol version. The default minimum TLS version will be TLS 1.2. As servers may not be Envoy and be set to TLS 1.2 (e.g., workloads using mTLS without sidecars), the minimum TLS version for clients may also be TLS 1.2. In the current Istio implementation, the maximum TLS protocol version is TLS 1.3. |  | Enum: [TLS_AUTO TLSV1_2 TLSV1_3]   |
| `ecdhCurves` _string array_ | Optional: If specified, the TLS connection will only support the specified ECDH curves for the DH key exchange. If not specified, the default curves enforced by Envoy will be used. For details about the default curves, refer to [Ecdh Curves](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/transport_sockets/tls/v3/common.proto). |  |  |
| `cipherSuites` _string array_ | Optional: If specified, the TLS connection will only support the specified cipher list when negotiating TLS 1.0-1.2. If not specified, the following cipher suites will be used: ``` ECDHE-ECDSA-AES256-GCM-SHA384 ECDHE-RSA-AES256-GCM-SHA384 ECDHE-ECDSA-AES128-GCM-SHA256 ECDHE-RSA-AES128-GCM-SHA256 AES256-GCM-SHA384 AES128-GCM-SHA256 ``` |  |  |


#### MeshConfigTLSConfigTLSProtocol

_Underlying type:_ _string_

TLS protocol versions.

_Validation:_
- Enum: [TLS_AUTO TLSV1_2 TLSV1_3]

_Appears in:_
- [MeshConfigTLSConfig](#meshconfigtlsconfig)

| Field | Description |
| --- | --- |
| `TLS_AUTO` | Automatically choose the optimal TLS version.  |
| `TLSV1_2` | TLS version 1.2  |
| `TLSV1_3` | TLS version 1.3  |




#### MultiClusterConfig



MultiClusterConfig specifies the Configuration for Istio mesh across multiple clusters through the istio gateways.



_Appears in:_
- [GlobalConfig](#globalconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enables the connection between two kubernetes clusters via their respective ingressgateway services. Use if the pods in each cluster cannot directly talk to one another. |  |  |
| `clusterName` _string_ | The name of the cluster this installation will run in. This is required for sidecar injection to properly label proxies |  |  |
| `globalDomainSuffix` _string_ | The suffix for global service names. |  |  |
| `includeEnvoyFilter` _boolean_ | Enable envoy filter to translate `globalDomainSuffix` to cluster local suffix for cross cluster communication. |  |  |


#### Network



Network provides information about the endpoints in a routable L3
network. A single routable L3 network can have one or more service
registries. Note that the network has no relation to the locality of the
endpoint. The endpoint locality will be obtained from the service
registry.



_Appears in:_
- [GlobalConfig](#globalconfig)
- [MeshNetworks](#meshnetworks)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `endpoints` _[NetworkNetworkEndpoints](#networknetworkendpoints) array_ | The list of endpoints in the network (obtained through the constituent service registries or from CIDR ranges). All endpoints in the network are directly accessible to one another. |  |  |
| `gateways` _[NetworkIstioNetworkGateway](#networkistionetworkgateway) array_ | Set of gateways associated with the network. |  |  |


#### NetworkIstioNetworkGateway



The gateway associated with this network. Traffic from remote networks
will arrive at the specified gateway:port. All incoming traffic must
use mTLS.



_Appears in:_
- [Network](#network)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `registryServiceName` _string_ | A fully qualified domain name of the gateway service.  Pilot will lookup the service from the service registries in the network and obtain the endpoint IPs of the gateway from the service registry. Note that while the service name is a fully qualified domain name, it need not be resolvable outside the orchestration platform for the registry. e.g., this could be istio-ingressgateway.istio-system.svc.cluster.local. |  |  |
| `address` _string_ | IP address or externally resolvable DNS address associated with the gateway. |  |  |
| `port` _integer_ |  |  |  |
| `locality` _string_ | The locality associated with an explicitly specified gateway (i.e. ip) |  |  |


#### NetworkNetworkEndpoints



NetworkEndpoints describes how the network associated with an endpoint
should be inferred. An endpoint will be assigned to a network based on
the following rules:


1. Implicitly: If the registry explicitly provides information about
the network to which the endpoint belongs to. In some cases, its
possible to indicate the network associated with the endpoint by
adding the `ISTIO_META_NETWORK` environment variable to the sidecar.


2. Explicitly:


  a. By matching the registry name with one of the "fromRegistry"
  in the mesh config. A "from_registry" can only be assigned to a
  single network.


  b. By matching the IP against one of the CIDR ranges in a mesh
  config network. The CIDR ranges must not overlap and be assigned to
  a single network.


(2) will override (1) if both are present.



_Appears in:_
- [Network](#network)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `fromCidr` _string_ | A CIDR range for the set of endpoints in this network. The CIDR ranges for endpoints from different networks must not overlap. |  |  |
| `fromRegistry` _string_ | Add all endpoints from the specified registry into this network. The names of the registries should correspond to the kubeconfig file name inside the secret that was used to configure the registry (Kubernetes multicluster) or supplied by MCP server. |  |  |




#### OutboundTrafficPolicyConfigMode

_Underlying type:_ _string_

Specifies the sidecar's default behavior when handling outbound traffic from the application.

_Validation:_
- Enum: [ALLOW_ANY REGISTRY_ONLY]

_Appears in:_
- [OutboundTrafficPolicyConfig](#outboundtrafficpolicyconfig)

| Field | Description |
| --- | --- |
| `ALLOW_ANY` | Outbound traffic to unknown destinations will be allowed, in case there are no services or ServiceEntries for the destination port  |
| `REGISTRY_ONLY` | Restrict outbound traffic to services defined in the service registry as well as those defined through ServiceEntries  |


#### PilotConfig



Configuration for Pilot.



_Appears in:_
- [Values](#values)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Controls whether Pilot is enabled. |  |  |
| `autoscaleEnabled` _boolean_ | Controls whether a HorizontalPodAutoscaler is installed for Pilot. |  |  |
| `autoscaleMin` _integer_ | Minimum number of replicas in the HorizontalPodAutoscaler for Pilot. |  |  |
| `autoscaleMax` _integer_ | Maximum number of replicas in the HorizontalPodAutoscaler for Pilot. |  |  |
| `autoscaleBehavior` _[HorizontalPodAutoscalerBehavior](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#horizontalpodautoscalerbehavior-v2-autoscaling)_ | See https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/#configurable-scaling-behavior |  |  |
| `replicaCount` _integer_ | Number of replicas in the Pilot Deployment.  Deprecated: Marked as deprecated in pkg/apis/istio/v1alpha1/values_types.proto. |  |  |
| `image` _string_ | Image name used for Pilot.  This can be set either to image name if hub is also set, or can be set to the full hub:name string.  Examples: custom-pilot, docker.io/someuser:custom-pilot |  |  |
| `traceSampling` _float_ | Trace sampling fraction.  Used to set the fraction of time that traces are sampled. Higher values are more accurate but add CPU overhead.  Allowed values: 0.0 to 1.0 |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#resourcerequirements-v1-core)_ | K8s resources settings.  See https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#resource-requests-and-limits-of-pod-and-container  Deprecated: Marked as deprecated in pkg/apis/istio/v1alpha1/values_types.proto. |  |  |
| `cpu` _[TargetUtilizationConfig](#targetutilizationconfig)_ | Target CPU utilization used in HorizontalPodAutoscaler.  See https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/  Deprecated: Marked as deprecated in pkg/apis/istio/v1alpha1/values_types.proto. |  |  |
| `nodeSelector` _object (keys:string, values:string)_ | K8s node selector.  See https://kubernetes.io/docs/concepts/configuration/assign-pod-node/#nodeselector  Deprecated: Marked as deprecated in pkg/apis/istio/v1alpha1/values_types.proto. |  |  |
| `keepaliveMaxServerConnectionAge` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#duration-v1-meta)_ | Maximum duration that a sidecar can be connected to a pilot.  This setting balances out load across pilot instances, but adds some resource overhead.  Examples: 300s, 30m, 1h |  |  |
| `deploymentLabels` _object (keys:string, values:string)_ | Labels that are added to Pilot deployment.  See https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/ |  |  |
| `podLabels` _object (keys:string, values:string)_ | Labels that are added to Pilot pods.  See https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/ |  |  |
| `configMap` _boolean_ | Configuration settings passed to Pilot as a ConfigMap.  This controls whether the mesh config map, generated from values.yaml is generated. If false, pilot wil use default values or user-supplied values, in that order of preference. |  |  |
| `env` _object (keys:string, values:string)_ | Environment variables passed to the Pilot container.  Examples: env:    ENV_VAR_1: value1   ENV_VAR_2: value2 |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#affinity-v1-core)_ | K8s affinity to set on the Pilot Pods. |  |  |
| `rollingMaxSurge` _[IntOrString](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#intorstring-intstr-util)_ | K8s rolling update strategy  Deprecated: Marked as deprecated in pkg/apis/istio/v1alpha1/values_types.proto. |  | XIntOrString: \{\}   |
| `rollingMaxUnavailable` _[IntOrString](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#intorstring-intstr-util)_ | The number of pods that can be unavailable during a rolling update (see `strategy.rollingUpdate.maxUnavailable` here: https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/deployment-v1/#DeploymentSpec). May be specified as a number of pods or as a percent of the total number of pods at the start of the update.  Deprecated: Marked as deprecated in pkg/apis/istio/v1alpha1/values_types.proto. |  | XIntOrString: \{\}   |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#toleration-v1-core) array_ | The node tolerations to be applied to the Pilot deployment so that it can be scheduled to particular nodes with matching taints. More info: https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#scheduling  Deprecated: Marked as deprecated in pkg/apis/istio/v1alpha1/values_types.proto. |  |  |
| `podAnnotations` _object (keys:string, values:string)_ | K8s annotations for pods.  See: https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/  Deprecated: Marked as deprecated in pkg/apis/istio/v1alpha1/values_types.proto. |  |  |
| `serviceAnnotations` _object (keys:string, values:string)_ | K8s annotations for the Service.  See: https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/ |  |  |
| `serviceAccountAnnotations` _object (keys:string, values:string)_ | K8s annotations for the service account |  |  |
| `jwksResolverExtraRootCA` _string_ | Specifies an extra root certificate in PEM format. This certificate will be trusted by pilot when resolving JWKS URIs. |  |  |
| `hub` _string_ | Hub to pull the container image from. Image will be `Hub/Image:Tag-Variant`. |  |  |
| `tag` _string_ | The container image tag to pull. Image will be `Hub/Image:Tag-Variant`. |  |  |
| `variant` _string_ | The container image variant to pull. Options are "debug" or "distroless". Unset will use the default for the given version. |  |  |
| `seccompProfile` _[SeccompProfile](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#seccompprofile-v1-core)_ | The seccompProfile for the Pilot container.  See: https://kubernetes.io/docs/tutorials/security/seccomp/ |  |  |
| `topologySpreadConstraints` _[TopologySpreadConstraint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#topologyspreadconstraint-v1-core) array_ | The k8s topologySpreadConstraints for the Pilot pods. |  |  |
| `extraContainerArgs` _string array_ | Additional container arguments for the Pilot container. |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#volumemount-v1-core) array_ | Additional volumeMounts to add to the Pilot container. |  |  |
| `volumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#volume-v1-core) array_ | Additional volumes to add to the Pilot Pod. |  |  |
| `ipFamilies` _string array_ | Defines which IP family to use for single stack or the order of IP families for dual-stack. Valid list items are "IPv4", "IPv6". More info: https://kubernetes.io/docs/concepts/services-networking/dual-stack/#services |  |  |
| `ipFamilyPolicy` _string_ | Controls whether Services are configured to use IPv4, IPv6, or both. Valid options are PreferDualStack, RequireDualStack, and SingleStack. More info: https://kubernetes.io/docs/concepts/services-networking/dual-stack/#services |  |  |
| `memory` _[TargetUtilizationConfig](#targetutilizationconfig)_ | Target memory utilization used in HorizontalPodAutoscaler.  See https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/  Deprecated: Marked as deprecated in pkg/apis/istio/v1alpha1/values_types.proto. |  |  |
| `cni` _[CNIUsageConfig](#cniusageconfig)_ | Configures whether to use an existing CNI installation for workloads |  |  |
| `taint` _[PilotTaintControllerConfig](#pilottaintcontrollerconfig)_ |  |  |  |
| `trustedZtunnelNamespace` _string_ | If set, `istiod` will allow connections from trusted node proxy ztunnels in the provided namespace. |  |  |




#### PilotTaintControllerConfig







_Appears in:_
- [PilotConfig](#pilotconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enable the untaint controller for new nodes. This aims to solve a race for CNI installation on new nodes. For this to work, the newly added nodes need to have the istio CNI taint as they are added to the cluster. This is usually done by configuring the cluster infra provider. |  |  |
| `namespace` _string_ | The namespace of the CNI daemonset, incase it's not the same as istiod. |  |  |








#### PrivateKeyProvider



PrivateKeyProvider defines private key configuration for gateways and sidecars. This can be configured
mesh wide or individual per-workload basis.



_Appears in:_
- [MeshConfigProxyConfig](#meshconfigproxyconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `cryptomb` _[PrivateKeyProviderCryptoMb](#privatekeyprovidercryptomb)_ | Use CryptoMb private key provider |  |  |
| `qat` _[PrivateKeyProviderQAT](#privatekeyproviderqat)_ | Use QAT private key provider |  |  |


#### PrivateKeyProviderCryptoMb

_Underlying type:_ _[struct{PollDelay *k8s.io/apimachinery/pkg/apis/meta/v1.Duration "json:\"pollDelay,omitempty\""; Fallback *bool "json:\"fallback,omitempty\""}](#struct{polldelay-*k8sioapimachinerypkgapismetav1duration-"json:\"polldelay,omitempty\"";-fallback-*bool-"json:\"fallback,omitempty\""})_

CryptoMb PrivateKeyProvider configuration



_Appears in:_
- [PrivateKeyProvider](#privatekeyprovider)



#### PrivateKeyProviderQAT

_Underlying type:_ _[struct{PollDelay *k8s.io/apimachinery/pkg/apis/meta/v1.Duration "json:\"pollDelay,omitempty\""; Fallback *bool "json:\"fallback,omitempty\""}](#struct{polldelay-*k8sioapimachinerypkgapismetav1duration-"json:\"polldelay,omitempty\"";-fallback-*bool-"json:\"fallback,omitempty\""})_

QAT (QuickAssist Technology) PrivateKeyProvider configuration



_Appears in:_
- [PrivateKeyProvider](#privatekeyprovider)



#### ProxyConfig



Configuration for Proxy.



_Appears in:_
- [GlobalConfig](#globalconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `autoInject` _string_ | Controls the 'policy' in the sidecar injector. |  |  |
| `clusterDomain` _string_ | Domain for the cluster, default: "cluster.local".  K8s allows this to be customized, see https://kubernetes.io/docs/tasks/administer-cluster/dns-custom-nameservers/ |  |  |
| `componentLogLevel` _string_ | Per Component log level for proxy, applies to gateways and sidecars.  If a component level is not set, then the global "logLevel" will be used. If left empty, "misc:error" is used. |  |  |
| `enableCoreDump` _boolean_ | Enables core dumps for newly injected sidecars.  If set, newly injected sidecars will have core dumps enabled. |  |  |
| `excludeInboundPorts` _string_ | Specifies the Istio ingress ports not to capture. |  |  |
| `excludeIPRanges` _string_ | Lists the excluded IP ranges of Istio egress traffic that the sidecar captures. |  |  |
| `image` _string_ | Image name or path for the proxy, default: "proxyv2".  If registry or tag are not specified, global.hub and global.tag are used.  Examples: my-proxy (uses global.hub/tag), docker.io/myrepo/my-proxy:v1.0.0 |  |  |
| `includeIPRanges` _string_ | Lists the IP ranges of Istio egress traffic that the sidecar captures.  Example: "172.30.0.0/16,172.20.0.0/16" This would only capture egress traffic on those two IP Ranges, all other outbound traffic would # be allowed by the sidecar." |  |  |
| `logLevel` _string_ | Log level for proxy, applies to gateways and sidecars. If left empty, "warning" is used. Expected values are: trace\\|debug\\|info\\|warning\\|error\\|critical\\|off |  |  |
| `outlierLogPath` _string_ | Path to the file to which the proxy will write outlier detection logs.  Example: "/dev/stdout" This would write the logs to standard output. |  |  |
| `privileged` _boolean_ | Enables privileged securityContext for the istio-proxy container.  See https://kubernetes.io/docs/tasks/configure-pod-container/security-context/ |  |  |
| `readinessInitialDelaySeconds` _integer_ | Sets the initial delay for readiness probes in seconds. |  |  |
| `readinessPeriodSeconds` _integer_ | Sets the interval between readiness probes in seconds. |  |  |
| `readinessFailureThreshold` _integer_ | Sets the number of successive failed probes before indicating readiness failure. |  |  |
| `startupProbe` _[StartupProbe](#startupprobe)_ | Configures the startup probe for the istio-proxy container. |  |  |
| `statusPort` _integer_ | Default port used for the Pilot agent's health checks. |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#resourcerequirements-v1-core)_ | K8s resources settings.  See https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#resource-requests-and-limits-of-pod-and-container  Deprecated: Marked as deprecated in pkg/apis/istio/v1alpha1/values_types.proto. |  |  |
| `tracer` _[Tracer](#tracer)_ | Specify which tracer to use. One of: zipkin, lightstep, datadog, stackdriver. If using stackdriver tracer outside GCP, set env GOOGLE_APPLICATION_CREDENTIALS to the GCP credential file. |  | Enum: [zipkin lightstep datadog stackdriver openCensusAgent none]   |
| `excludeOutboundPorts` _string_ | A comma separated list of outbound ports to be excluded from redirection to Envoy. |  |  |
| `lifecycle` _[Lifecycle](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#lifecycle-v1-core)_ | The k8s lifecycle hooks definition (pod.spec.containers.lifecycle) for the proxy container. More info: https://kubernetes.io/docs/concepts/containers/container-lifecycle-hooks/#container-hooks |  |  |
| `holdApplicationUntilProxyStarts` _boolean_ | Controls if sidecar is injected at the front of the container list and blocks the start of the other containers until the proxy is ready  Deprecated: replaced by ProxyConfig setting which allows per-pod configuration of this behavior.  Deprecated: Marked as deprecated in pkg/apis/istio/v1alpha1/values_types.proto. |  |  |
| `includeInboundPorts` _string_ | A comma separated list of inbound ports for which traffic is to be redirected to Envoy. The wildcard character '*' can be used to configure redirection for all ports. |  |  |
| `includeOutboundPorts` _string_ | A comma separated list of outbound ports for which traffic is to be redirected to Envoy, regardless of the destination IP. |  |  |


#### ProxyConfigInboundInterceptionMode

_Underlying type:_ _string_

The mode used to redirect inbound traffic to Envoy.
This setting has no effect on outbound traffic: iptables `REDIRECT` is always used for
outbound connections.

_Validation:_
- Enum: [REDIRECT TPROXY NONE]

_Appears in:_
- [MeshConfigProxyConfig](#meshconfigproxyconfig)

| Field | Description |
| --- | --- |
| `REDIRECT` | The `REDIRECT` mode uses iptables `REDIRECT` to `NAT` and redirect to Envoy. This mode loses source IP addresses during redirection. This is the default redirection mode.  |
| `TPROXY` | The `TPROXY` mode uses iptables `TPROXY` to redirect to Envoy. This mode preserves both the source and destination IP addresses and ports, so that they can be used for advanced filtering and manipulation. This mode also configures the sidecar to run with the `CAP_NET_ADMIN` capability, which is required to use `TPROXY`.  |
| `NONE` | The `NONE` mode does not configure redirect to Envoy at all. This is an advanced configuration that typically requires changes to user applications.  |


#### ProxyConfigProxyHeaders







_Appears in:_
- [MeshConfigProxyConfig](#meshconfigproxyconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `forwardedClientCert` _[ForwardClientCertDetails](#forwardclientcertdetails)_ | Controls the `X-Forwarded-Client-Cert` header for inbound sidecar requests. To set this on gateways, use the `Topology` setting. To disable the header, configure either `SANITIZE` (to always remove the header, if present) or `FORWARD_ONLY` (to leave the header as-is). By default, `APPEND_FORWARD` will be used. |  | Enum: [UNDEFINED SANITIZE FORWARD_ONLY APPEND_FORWARD SANITIZE_SET ALWAYS_FORWARD_ONLY]   |
| `requestId` _[ProxyConfigProxyHeadersRequestId](#proxyconfigproxyheadersrequestid)_ | Controls the `X-Request-Id` header. If enabled, a request ID is generated for each request if one is not already set. This applies to all types of traffic (inbound, outbound, and gateways). If disabled, no request ID will be generate for the request. If it is already present, it will be preserved. Warning: request IDs are a critical component to mesh tracing and logging, so disabling this is not recommended. This header is enabled by default if not configured. |  |  |
| `server` _[ProxyConfigProxyHeadersServer](#proxyconfigproxyheadersserver)_ | Controls the `server` header. If enabled, the `Server: istio-envoy` header is set in response headers for inbound traffic (including gateways). If disabled, the `Server` header is not modified. If it is already present, it will be preserved. |  |  |
| `attemptCount` _[ProxyConfigProxyHeadersAttemptCount](#proxyconfigproxyheadersattemptcount)_ | Controls the `X-Envoy-Attempt-Count` header. If enabled, this header will be added on outbound request headers (including gateways) that have retries configured. If disabled, this header will not be set. If it is already present, it will be preserved. This header is enabled by default if not configured. |  |  |
| `envoyDebugHeaders` _[ProxyConfigProxyHeadersEnvoyDebugHeaders](#proxyconfigproxyheadersenvoydebugheaders)_ | Controls various `X-Envoy-*` headers, such as `X-Envoy-Overloaded` and `X-Envoy-Upstream-Service-Time`. If enabled, these headers will be included. If disabled, these headers will not be set. If they are already present, they will be preserved. See the [Envoy documentation](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/filters/http/router/v3/router.proto#envoy-v3-api-field-extensions-filters-http-router-v3-router-suppress-envoy-headers) for more details. These headers are enabled by default if not configured. |  |  |
| `metadataExchangeHeaders` _[ProxyConfigProxyHeadersMetadataExchangeHeaders](#proxyconfigproxyheadersmetadataexchangeheaders)_ | Controls Istio metadata exchange headers `X-Envoy-Peer-Metadata` and `X-Envoy-Peer-Metadata-Id`. By default, the behavior is unspecified. If IN_MESH, these headers will not be appended to outbound requests from sidecars to services not in-mesh. |  |  |


#### ProxyConfigProxyHeadersAttemptCount

_Underlying type:_ _[struct{Disabled *bool "json:\"disabled,omitempty\""}](#struct{disabled-*bool-"json:\"disabled,omitempty\""})_





_Appears in:_
- [ProxyConfigProxyHeaders](#proxyconfigproxyheaders)



#### ProxyConfigProxyHeadersEnvoyDebugHeaders

_Underlying type:_ _[struct{Disabled *bool "json:\"disabled,omitempty\""}](#struct{disabled-*bool-"json:\"disabled,omitempty\""})_





_Appears in:_
- [ProxyConfigProxyHeaders](#proxyconfigproxyheaders)



#### ProxyConfigProxyHeadersMetadataExchangeHeaders

_Underlying type:_ _[struct{Mode ProxyConfigProxyHeadersMetadataExchangeMode "json:\"mode,omitempty\""}](#struct{mode-proxyconfigproxyheadersmetadataexchangemode-"json:\"mode,omitempty\""})_





_Appears in:_
- [ProxyConfigProxyHeaders](#proxyconfigproxyheaders)





#### ProxyConfigProxyHeadersRequestId

_Underlying type:_ _[struct{Disabled *bool "json:\"disabled,omitempty\""}](#struct{disabled-*bool-"json:\"disabled,omitempty\""})_





_Appears in:_
- [ProxyConfigProxyHeaders](#proxyconfigproxyheaders)



#### ProxyConfigProxyHeadersServer

_Underlying type:_ _[struct{Disabled *bool "json:\"disabled,omitempty\""; Value string "json:\"value,omitempty\""}](#struct{disabled-*bool-"json:\"disabled,omitempty\"";-value-string-"json:\"value,omitempty\""})_





_Appears in:_
- [ProxyConfigProxyHeaders](#proxyconfigproxyheaders)



#### ProxyConfigProxyStatsMatcher



Proxy stats name matchers for stats creation. Note this is in addition to
the minimum Envoy stats that Istio generates by default.



_Appears in:_
- [MeshConfigProxyConfig](#meshconfigproxyconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `inclusionPrefixes` _string array_ | Proxy stats name prefix matcher for inclusion. |  |  |
| `inclusionSuffixes` _string array_ | Proxy stats name suffix matcher for inclusion. |  |  |
| `inclusionRegexps` _string array_ | Proxy stats name regexps matcher for inclusion. |  |  |


#### ProxyConfigTracingServiceName

_Underlying type:_ _string_

Allows specification of various Istio-supported naming schemes for the
Envoy `service_cluster` value. The `servce_cluster` value is primarily used
by Envoys to provide service names for tracing spans.

_Validation:_
- Enum: [APP_LABEL_AND_NAMESPACE CANONICAL_NAME_ONLY CANONICAL_NAME_AND_NAMESPACE]

_Appears in:_
- [MeshConfigProxyConfig](#meshconfigproxyconfig)

| Field | Description |
| --- | --- |
| `APP_LABEL_AND_NAMESPACE` | Default scheme. Uses the `app` label and workload namespace to construct a cluster name. If the `app` label does not exist `istio-proxy` is used.  |
| `CANONICAL_NAME_ONLY` | Uses the canonical name for a workload (*excluding namespace*).  |
| `CANONICAL_NAME_AND_NAMESPACE` | Uses the canonical name and namespace for a workload.  |


#### ProxyImage



The following values are used to construct proxy image url.
format: `${hub}/${image_name}/${tag}-${image_type}`,
example: `docker.io/istio/proxyv2:1.11.1` or `docker.io/istio/proxyv2:1.11.1-distroless`.
This information was previously part of the Values API.



_Appears in:_
- [MeshConfigProxyConfig](#meshconfigproxyconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `imageType` _string_ | The image type of the image. Istio publishes default, debug, and distroless images. Other values are allowed if those image types (example: centos) are published to the specified hub. supported values: default, debug, distroless. |  |  |


#### ProxyInitConfig



Configuration for proxy_init container which sets the pods' networking to intercept the inbound/outbound traffic.



_Appears in:_
- [GlobalConfig](#globalconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ | Specifies the image for the proxy_init container. |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#resourcerequirements-v1-core)_ | K8s resources settings.  See https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#resource-requests-and-limits-of-pod-and-container  Deprecated: Marked as deprecated in pkg/apis/istio/v1alpha1/values_types.proto. |  |  |


#### RemoteIstio



RemoteIstio represents a remote Istio Service Mesh deployment consisting of one or more
remote control plane instances (represented by one or more IstioRevision objects).



_Appears in:_
- [RemoteIstioList](#remoteistiolist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `operator.istio.io/v1alpha1` | | |
| `kind` _string_ | `RemoteIstio` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[RemoteIstioSpec](#remoteistiospec)_ |  | \{ namespace:istio-system updateStrategy:map[type:InPlace] version:v1.23.0 \} |  |
| `status` _[RemoteIstioStatus](#remoteistiostatus)_ |  |  |  |


#### RemoteIstioCondition



RemoteIstioCondition represents a specific observation of the RemoteIstioCondition object's state.



_Appears in:_
- [RemoteIstioStatus](#remoteistiostatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[RemoteIstioConditionType](#remoteistioconditiontype)_ | The type of this condition. |  |  |
| `status` _[ConditionStatus](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#conditionstatus-v1-meta)_ | The status of this condition. Can be True, False or Unknown. |  |  |
| `reason` _[RemoteIstioConditionReason](#remoteistioconditionreason)_ | Unique, single-word, CamelCase reason for the condition's last transition. |  |  |
| `message` _string_ | Human-readable message indicating details about the last transition. |  |  |
| `lastTransitionTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#time-v1-meta)_ | Last time the condition transitioned from one status to another. |  |  |


#### RemoteIstioConditionReason

_Underlying type:_ _string_

RemoteIstioConditionReason represents a short message indicating how the condition came
to be in its present state.



_Appears in:_
- [RemoteIstioCondition](#remoteistiocondition)
- [RemoteIstioStatus](#remoteistiostatus)

| Field | Description |
| --- | --- |
| `ReconcileError` | RemoteIstioReasonReconcileError indicates that the reconciliation of the resource has failed, but will be retried.  |
| `ActiveRevisionNotFound` | RemoteIstioReasonRevisionNotFound indicates that the active IstioRevision is not found.  |
| `FailedToGetActiveRevision` | RemoteIstioReasonFailedToGetActiveRevision indicates that a failure occurred when getting the active IstioRevision  |
| `IstiodNotReady` | RemoteIstioReasonIstiodNotReady indicates that the control plane is fully reconciled, but istiod is not ready.  |
| `ReadinessCheckFailed` | RemoteIstioReasonReadinessCheckFailed indicates that readiness could not be ascertained.  |
| `Healthy` | RemoteIstioReasonHealthy indicates that the control plane is fully reconciled and that all components are ready.  |


#### RemoteIstioConditionType

_Underlying type:_ _string_

RemoteIstioConditionType represents the type of the condition.  Condition stages are:
Installed, Reconciled, Ready



_Appears in:_
- [RemoteIstioCondition](#remoteistiocondition)

| Field | Description |
| --- | --- |
| `Reconciled` | RemoteIstioConditionReconciled signifies whether the controller has successfully reconciled the resources defined through the CR.  |
| `Ready` | RemoteIstioConditionReady signifies whether any Deployment, StatefulSet, etc. resources are Ready.  |


#### RemoteIstioList



RemoteIstioList contains a list of RemoteIstio





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `operator.istio.io/v1alpha1` | | |
| `kind` _string_ | `RemoteIstioList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents. Servers may infer this from the endpoint the client submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object. Servers should convert recognized schemas to the latest internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[RemoteIstio](#remoteistio) array_ |  |  |  |


#### RemoteIstioSpec



RemoteIstioSpec defines the desired state of RemoteIstio



_Appears in:_
- [RemoteIstio](#remoteistio)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `version` _string_ | Defines the version of Istio to install. Must be one of: v1.23.0, v1.22.3, v1.21.5, latest. | v1.23.0 | Enum: [v1.23.0 v1.22.3 v1.21.5 latest]   |
| `updateStrategy` _[IstioUpdateStrategy](#istioupdatestrategy)_ | Defines the update strategy to use when the version in the RemoteIstio CR is updated. | \{ type:InPlace \} |  |
| `profile` _string_ | The built-in installation configuration profile to use. The 'default' profile is always applied. On OpenShift, the 'openshift' profile is also applied on top of 'default'. Must be one of: ambient, default, demo, empty, external, openshift-ambient, openshift, preview, stable. |  | Enum: [ambient default demo empty external openshift-ambient openshift preview stable]   |
| `namespace` _string_ | Namespace to which the Istio components should be installed. | istio-system |  |
| `values` _[Values](#values)_ | Defines the values to be passed to the Helm charts when installing Istio. |  |  |


#### RemoteIstioStatus



RemoteIstioStatus defines the observed state of RemoteIstio



_Appears in:_
- [RemoteIstio](#remoteistio)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the most recent generation observed for this RemoteIstio object. It corresponds to the object's generation, which is updated on mutation by the API Server. The information in the status pertains to this particular generation of the object. |  |  |
| `conditions` _[RemoteIstioCondition](#remoteistiocondition) array_ | Represents the latest available observations of the object's current state. |  |  |
| `state` _[RemoteIstioConditionReason](#remoteistioconditionreason)_ | Reports the current state of the object. |  |  |
| `revisions` _[RevisionSummary](#revisionsummary)_ | Reports information about the underlying IstioRevisions. |  |  |


#### RemoteService







_Appears in:_
- [MeshConfigProxyConfig](#meshconfigproxyconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `address` _string_ | Address of a remove service used for various purposes (access log receiver, metrics receiver, etc.). Can be IP address or a fully qualified DNS name. |  |  |
| `tlsSettings` _[ClientTLSSettings](#clienttlssettings)_ | Use the `tls_settings` to specify the tls mode to use. If the remote service uses Istio mutual TLS and shares the root CA with Pilot, specify the TLS mode as `ISTIO_MUTUAL`. |  |  |
| `tcpKeepalive` _[ConnectionPoolSettingsTCPSettingsTcpKeepalive](#connectionpoolsettingstcpsettingstcpkeepalive)_ | If set then set `SO_KEEPALIVE` on the socket to enable TCP Keepalives. |  |  |


#### Resource

_Underlying type:_ _string_

Resource describes the source of configuration

_Validation:_
- Enum: [SERVICE_REGISTRY]

_Appears in:_
- [ConfigSource](#configsource)

| Field | Description |
| --- | --- |
| `SERVICE_REGISTRY` | Set to only receive service entries that are generated by the platform. These auto generated service entries are combination of services and endpoints that are generated by a specific platform e.g. k8  |


#### ResourceQuotas



Configuration for the resource quotas for the CNI DaemonSet.



_Appears in:_
- [CNIConfig](#cniconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Controls whether to create resource quotas or not for the CNI DaemonSet. |  |  |
| `pods` _integer_ | The hard limit on the number of pods in the namespace where the CNI DaemonSet is deployed. |  |  |




#### RevisionSummary



RevisionSummary contains information on the number of IstioRevisions associated with this Istio.



_Appears in:_
- [IstioStatus](#istiostatus)
- [RemoteIstioStatus](#remoteistiostatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `total` _integer_ | Total number of IstioRevisions currently associated with this Istio. |  |  |
| `ready` _integer_ | Number of IstioRevisions that are Ready. |  |  |
| `inUse` _integer_ | Number of IstioRevisions that are currently in use. |  |  |




#### SDSConfig



Configuration for the SecretDiscoveryService instead of using K8S secrets to mount the certificates.



_Appears in:_
- [GlobalConfig](#globalconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `token` _[SDSConfigToken](#sdsconfigtoken)_ | Deprecated: Marked as deprecated in pkg/apis/istio/v1alpha1/values_types.proto. |  |  |


#### SDSConfigToken







_Appears in:_
- [SDSConfig](#sdsconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `aud` _string_ |  |  |  |


#### STSConfig



Configuration for Security Token Service (STS) server.


See https://tools.ietf.org/html/draft-ietf-oauth-token-exchange-16



_Appears in:_
- [GlobalConfig](#globalconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `servicePort` _integer_ |  |  |  |




#### SidecarInjectorConfig



SidecarInjectorConfig is described in istio.io documentation.



_Appears in:_
- [Values](#values)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enableNamespacesByDefault` _boolean_ | Enables sidecar auto-injection in namespaces by default. |  |  |
| `reinvocationPolicy` _string_ | Setting this to `IfNeeded` will result in the sidecar injector being run again if additional mutations occur. Default: Never |  |  |
| `neverInjectSelector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#labelselector-v1-meta) array_ | Instructs Istio to not inject the sidecar on those pods, based on labels that are present in those pods.  Annotations in the pods have higher precedence than the label selectors. Order of evaluation: Pod Annotations → NeverInjectSelector → AlwaysInjectSelector → Default Policy. See https://istio.io/docs/setup/kubernetes/additional-setup/sidecar-injection/#more-control-adding-exceptions |  |  |
| `alwaysInjectSelector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#labelselector-v1-meta) array_ | See NeverInjectSelector. |  |  |
| `rewriteAppHTTPProbe` _boolean_ | If true, webhook or istioctl injector will rewrite PodSpec for liveness health check to redirect request to sidecar. This makes liveness check work even when mTLS is enabled. |  |  |
| `injectedAnnotations` _object (keys:string, values:string)_ | injectedAnnotations are additional annotations that will be added to the pod spec after injection This is primarily to support PSP annotations. |  |  |
| `injectionURL` _string_ | Configure the injection url for sidecar injector webhook |  |  |
| `templates` _object (keys:string, values:string)_ | Templates defines a set of custom injection templates that can be used. For example, defining:  templates:    hello: \|     metadata:       labels:         hello: world  Then starting a pod with the `inject.istio.io/templates: hello` annotation, will result in the pod being injected with the hello=world labels. This is intended for advanced configuration only; most users should use the built in template |  |  |
| `defaultTemplates` _string array_ | defaultTemplates: ["sidecar", "hello"] |  |  |


#### StartupProbe







_Appears in:_
- [ProxyConfig](#proxyconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enables or disables a startup probe. For optimal startup times, changing this should be tied to the readiness probe values.  If the probe is enabled, it is recommended to have delay=0s,period=15s,failureThreshold=4. This ensures the pod is marked ready immediately after the startup probe passes (which has a 1s poll interval), and doesn't spam the readiness endpoint too much  If the probe is disabled, it is recommended to have delay=1s,period=2s,failureThreshold=30. This ensures the startup is reasonable fast (polling every 2s). 1s delay is used since the startup is not often ready instantly. |  |  |
| `failureThreshold` _integer_ | Minimum consecutive failures for the probe to be considered failed after having succeeded. |  |  |


#### TargetUtilizationConfig



Configuration for CPU or memory target utilization for HorizontalPodAutoscaler target.



_Appears in:_
- [PilotConfig](#pilotconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `targetAverageUtilization` _integer_ | K8s utilization setting for HorizontalPodAutoscaler target.  See https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/ |  |  |


#### TelemetryConfig



Controls telemetry configuration



_Appears in:_
- [Values](#values)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Controls whether telemetry is exported for Pilot. |  |  |
| `v2` _[TelemetryV2Config](#telemetryv2config)_ | Configuration for Telemetry v2. |  |  |


#### TelemetryV2Config



Controls whether pilot will configure telemetry v2.



_Appears in:_
- [TelemetryConfig](#telemetryconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Controls whether pilot will configure telemetry v2. |  |  |
| `prometheus` _[TelemetryV2PrometheusConfig](#telemetryv2prometheusconfig)_ | Telemetry v2 settings for prometheus. |  |  |
| `stackdriver` _[TelemetryV2StackDriverConfig](#telemetryv2stackdriverconfig)_ | Telemetry v2 settings for stackdriver. |  |  |


#### TelemetryV2PrometheusConfig



Controls telemetry v2 prometheus settings.



_Appears in:_
- [TelemetryV2Config](#telemetryv2config)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Controls whether stats envoyfilter would be enabled or not. |  |  |


#### TelemetryV2StackDriverConfig



TelemetryV2StackDriverConfig controls telemetry v2 stackdriver settings.



_Appears in:_
- [TelemetryV2Config](#telemetryv2config)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ |  |  |  |


#### Topology



Topology describes the configuration for relative location of a proxy with
respect to intermediate trusted proxies and the client. These settings
control how the client attributes are retrieved from the incoming traffic by
the gateway proxy and propagated to the upstream services in the cluster.



_Appears in:_
- [MeshConfigProxyConfig](#meshconfigproxyconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `numTrustedProxies` _integer_ | Number of trusted proxies deployed in front of the Istio gateway proxy. When this option is set to value N greater than zero, the trusted client address is assumed to be the Nth address from the right end of the X-Forwarded-For (XFF) header from the incoming request. If the X-Forwarded-For (XFF) header is missing or has fewer than N addresses, the gateway proxy falls back to using the immediate downstream connection's source address as the trusted client address. Note that the gateway proxy will append the downstream connection's source address to the X-Forwarded-For (XFF) address and set the X-Envoy-External-Address header to the trusted client address before forwarding it to the upstream services in the cluster. The default value of num_trusted_proxies is 0. See [Envoy XFF](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_conn_man/headers#config-http-conn-man-headers-x-forwarded-for) header handling for more details. |  |  |
| `forwardClientCertDetails` _[ForwardClientCertDetails](#forwardclientcertdetails)_ | Configures how the gateway proxy handles x-forwarded-client-cert (XFCC) header in the incoming request. |  | Enum: [UNDEFINED SANITIZE FORWARD_ONLY APPEND_FORWARD SANITIZE_SET ALWAYS_FORWARD_ONLY]   |
| `proxyProtocol` _[TopologyProxyProtocolConfiguration](#topologyproxyprotocolconfiguration)_ | Enables [PROXY protocol](http://www.haproxy.org/download/1.5/doc/proxy-protocol.txt) for downstream connections on a gateway. |  |  |


#### TopologyProxyProtocolConfiguration

_Underlying type:_ _[struct{}](#struct{})_

PROXY protocol configuration.



_Appears in:_
- [Topology](#topology)



#### Tracer

_Underlying type:_ _string_

Specifies which tracer to use.

_Validation:_
- Enum: [zipkin lightstep datadog stackdriver openCensusAgent none]

_Appears in:_
- [ProxyConfig](#proxyconfig)

| Field | Description |
| --- | --- |
| `zipkin` |  |
| `lightstep` |  |
| `datadog` |  |
| `stackdriver` |  |
| `openCensusAgent` |  |
| `none` |  |


#### TracerConfig



Configuration for each of the supported tracers.



_Appears in:_
- [GlobalConfig](#globalconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `datadog` _[TracerDatadogConfig](#tracerdatadogconfig)_ | Configuration for the datadog tracing service. |  |  |
| `lightstep` _[TracerLightStepConfig](#tracerlightstepconfig)_ | Configuration for the lightstep tracing service. |  |  |
| `zipkin` _[TracerZipkinConfig](#tracerzipkinconfig)_ | Configuration for the zipkin tracing service. |  |  |
| `stackdriver` _[TracerStackdriverConfig](#tracerstackdriverconfig)_ | Configuration for the stackdriver tracing service. |  |  |


#### TracerDatadogConfig



Configuration for the datadog tracing service.



_Appears in:_
- [TracerConfig](#tracerconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `address` _string_ | Address in host:port format for reporting trace data to the Datadog agent. |  |  |


#### TracerLightStepConfig



Configuration for the lightstep tracing service.



_Appears in:_
- [TracerConfig](#tracerconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `address` _string_ | Sets the lightstep satellite pool address in host:port format for reporting trace data. |  |  |
| `accessToken` _string_ | Sets the lightstep access token. |  |  |


#### TracerStackdriverConfig



Configuration for the stackdriver tracing service.



_Appears in:_
- [TracerConfig](#tracerconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `debug` _boolean_ | enables trace output to stdout. |  |  |
| `maxNumberOfAttributes` _integer_ | The global default max number of attributes per span. |  |  |
| `maxNumberOfAnnotations` _integer_ | The global default max number of annotation events per span. |  |  |
| `maxNumberOfMessageEvents` _integer_ | The global default max number of message events per span. |  |  |


#### TracerZipkinConfig



Configuration for the zipkin tracing service.



_Appears in:_
- [TracerConfig](#tracerconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `address` _string_ | Address of zipkin instance in host:port format for reporting trace data.  Example: <zipkin-collector-service>.<zipkin-collector-namespace>:941 |  |  |


#### Tracing



Tracing defines configuration for the tracing performed by Envoy instances.



_Appears in:_
- [MeshConfigProxyConfig](#meshconfigproxyconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `zipkin` _[TracingZipkin](#tracingzipkin)_ | Use a Zipkin tracer. |  |  |
| `datadog` _[TracingDatadog](#tracingdatadog)_ | Use a Datadog tracer. |  |  |
| `stackdriver` _[TracingStackdriver](#tracingstackdriver)_ | Use a Stackdriver tracer. |  |  |
| `openCensusAgent` _[TracingOpenCensusAgent](#tracingopencensusagent)_ | Use an OpenCensus tracer exporting to an OpenCensus agent. |  |  |
| `sampling` _float_ | The percentage of requests (0.0 - 100.0) that will be randomly selected for trace generation, if not requested by the client or not forced. Default is 1.0. |  |  |
| `tlsSettings` _[ClientTLSSettings](#clienttlssettings)_ | Use the tls_settings to specify the tls mode to use. If the remote tracing service uses Istio mutual TLS and shares the root CA with Pilot, specify the TLS mode as `ISTIO_MUTUAL`. |  |  |




#### TracingDatadog

_Underlying type:_ _[struct{Address string "json:\"address,omitempty\""}](#struct{address-string-"json:\"address,omitempty\""})_

Datadog defines configuration for a Datadog tracer.



_Appears in:_
- [Tracing](#tracing)









#### TracingOpenCensusAgent

_Underlying type:_ _[struct{Address string "json:\"address,omitempty\""; Context []TracingOpenCensusAgentTraceContext "json:\"context,omitempty\""}](#struct{address-string-"json:\"address,omitempty\"";-context-[]tracingopencensusagenttracecontext-"json:\"context,omitempty\""})_

OpenCensusAgent defines configuration for an OpenCensus tracer writing to
an OpenCensus agent backend. See
[Envoy's OpenCensus trace configuration](https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/trace/v3/opencensus.proto)
and
[OpenCensus trace config](https://github.com/census-instrumentation/opencensus-proto/blob/master/src/opencensus/proto/trace/v1/trace_config.proto)
for details.



_Appears in:_
- [Tracing](#tracing)







#### TracingStackdriver

_Underlying type:_ _[struct{Debug bool "json:\"debug,omitempty\""; MaxNumberOfAttributes *int64 "json:\"maxNumberOfAttributes,omitempty\""; MaxNumberOfAnnotations *int64 "json:\"maxNumberOfAnnotations,omitempty\""; MaxNumberOfMessageEvents *int64 "json:\"maxNumberOfMessageEvents,omitempty\""}](#struct{debug-bool-"json:\"debug,omitempty\"";-maxnumberofattributes-*int64-"json:\"maxnumberofattributes,omitempty\"";-maxnumberofannotations-*int64-"json:\"maxnumberofannotations,omitempty\"";-maxnumberofmessageevents-*int64-"json:\"maxnumberofmessageevents,omitempty\""})_

Stackdriver defines configuration for a Stackdriver tracer.
See [Envoy's OpenCensus trace configuration](https://www.envoyproxy.io/docs/envoy/latest/api-v3/config/trace/v3/opencensus.proto)
and
[OpenCensus trace config](https://github.com/census-instrumentation/opencensus-proto/blob/master/src/opencensus/proto/trace/v1/trace_config.proto) for details.



_Appears in:_
- [Tracing](#tracing)



#### TracingZipkin

_Underlying type:_ _[struct{Address string "json:\"address,omitempty\""}](#struct{address-string-"json:\"address,omitempty\""})_

Zipkin defines configuration for a Zipkin tracer.



_Appears in:_
- [Tracing](#tracing)



#### UpdateStrategyType

_Underlying type:_ _string_





_Appears in:_
- [IstioUpdateStrategy](#istioupdatestrategy)

| Field | Description |
| --- | --- |
| `InPlace` |  |
| `RevisionBased` |  |


#### Values







_Appears in:_
- [IstioRevisionSpec](#istiorevisionspec)
- [IstioSpec](#istiospec)
- [RemoteIstioSpec](#remoteistiospec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `global` _[GlobalConfig](#globalconfig)_ | Global configuration for Istio components. |  |  |
| `pilot` _[PilotConfig](#pilotconfig)_ | Configuration for the Pilot component. |  |  |
| `telemetry` _[TelemetryConfig](#telemetryconfig)_ | Controls whether telemetry is exported for Pilot. |  |  |
| `sidecarInjectorWebhook` _[SidecarInjectorConfig](#sidecarinjectorconfig)_ | Configuration for the sidecar injector webhook. |  |  |
| `revision` _string_ | Identifies the revision this installation is associated with. |  |  |
| `meshConfig` _[MeshConfig](#meshconfig)_ | Defines runtime configuration of components, including Istiod and istio-agent behavior. See https://istio.io/docs/reference/config/istio.mesh.v1alpha1/ for all available options. TODO can this import the real mesh config API? |  |  |
| `base` _[BaseConfig](#baseconfig)_ | Configuration for the base component. |  |  |
| `istiodRemote` _[IstiodRemoteConfig](#istiodremoteconfig)_ | Configuration for istiod-remote. |  |  |
| `revisionTags` _string array_ | Specifies the aliases for the Istio control plane revision. A MutatingWebhookConfiguration is created for each alias. |  |  |
| `defaultRevision` _string_ | The name of the default revision in the cluster. |  |  |
| `profile` _string_ | Specifies which installation configuration profile to apply. |  |  |
| `compatibilityVersion` _string_ | Specifies the compatibility version to use. When this is set, the control plane will be configured with the same defaults as the specified version. |  |  |
| `experimental` _[RawMessage](#rawmessage)_ | Specifies experimental helm fields that could be removed or changed in the future |  | Schemaless: \{\}   |


#### WaypointConfig



Configuration for Waypoint proxies.



_Appears in:_
- [GlobalConfig](#globalconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.25/#resourcerequirements-v1-core)_ | K8s resource settings.  See https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#resource-requests-and-limits-of-pod-and-container |  |  |








