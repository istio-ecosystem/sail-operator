# Field Mappings: SMCP 2.6 to Istio 3.0

Many of the SMCP's spec options are still configurable in the `istio` resource, however they are found in different locations. 

To help with migration, here is a table of where the fields of the `servicemeshcontrolplane.spec` can be found in `istio`. 

## Addons Configuration

| SMCP 2.6           | Istio 3.0              |
|--------------------|------------------------|
| spec.addons.3scale | Not directly available |
| spec.addons.grafana | Not directly available |
| spec.addons.jaeger | Not directly available |
| spec.addons.kiali | Not directly available |
| spec.addons.prometheus | Not directly available |
| spec.addons.stackdriver | Not directly available |

Addons are no longer configured through the SMCP and instead should be configured separately. 

For details on how to enable addon integrations, see [Observability Integration](https://docs.redhat.com/en/documentation/red_hat_openshift_service_mesh/3.0.0tp1/html/observability/index).

## Cluster Configuration

| SMCP 2.6 | Istio 3.0 |
|----------|-----------|
| cluster.meshExpansion.ilbGateway | Not directly available (entire section no longer exists) |
| cluster.multiCluster.enabled | spec.values.global.multiCluster.enabled |
| cluster.multiCluster.meshNetworks | spec.values.global.meshNetworks |
| cluster.multiCluster.meshNetworks.endpoints | spec.values.global.meshNetworks.endpoints |
| cluster.multiCluster.meshNetworks.endpoints.fromCIDR | spec.values.global.meshNetworks.endpoints.fromCidr |
| cluster.multiCluster.meshNetworks.endpoints.fromRegistry | spec.values.global.meshNetworks.endpoints.fromRegistry |
| cluster.multiCluster.meshNetworks.gateways | spec.values.global.meshNetworks.gateways |
| cluster.multiCluster.meshNetworks.gateways.address | spec.values.global.meshNetworks.gateways.address |
| cluster.multiCluster.meshNetworks.gateways.port | spec.values.global.meshNetworks.gateways.port |
| cluster.multiCluster.meshNetworks.gateways.registryServiceName | spec.values.global.meshNetworks.gateways.registryServiceName |
| cluster.multiCluster.meshNetworks.gateways.service | Not directly available |
| cluster.name | spec.values.global.multiCluster.clusterName |
| cluster.network | spec.values.global.network |

## Gateways Configuration
Entire section is no longer directly available, see [Independently Managed Gateways](../../ossm2-vs-ossm3.md#independently-managed-gateways) in the ossm2 vs ossm3 documentation for details.

## General Configuration

| SMCP 2.6 | Istio 3.0 |
|----------|-----------|
| general.logging.componentLevels | spec.values.global.logging.componentLevels |
| general.logging.logAsJSON | spec.values.global.logAsJson |
| general.validationMessages | spec.values.global.istiod.enableAnalysis |

## MeshConfig Configuration

| SMCP 2.6 | Istio 3.0 |
|----------|-----------|
| meshConfig.discoverySelectors | spec.meshConfig.discoverySelectors |
| meshConfig.extensionProviders | spec.meshConfig.extensionProviders |

## Mode Configuration
In SMCP, takes "multitenant", "clusterwide", "federation". There is no direct mapping in the `istio` resource, however:

- Clusterwide is default in 3.0
- Federation setup is TBD
- Multitenancy can be configured through discoverySelectors, see [here](./../../create-mesh/README.md) for more details

## Policy Configuration

| SMCP 2.6 | Istio 3.0 |
|----------|-----------|
| spec.policy.type | No direct equivalent (Istiod is now the only option) |
| spec.policy.mixer | Removed (Mixer deprecated) |
| spec.policy.remote | Removed (Remote policy checking replaced) |

## Profiles Configuration
Same location, profile options are now:
- ambient
- default
- demo
- empty
- openshift-ambient
- openshift
- preview
- stable

## Proxy Configuration

### Access Logging Config

| SMCP 2.6 | Istio 3.0 |
|----------|-----------|
| proxy.accessLogging.envoyService.address | spec.meshConfig.defaultConfig.envoyAccessLogService.address |
| proxy.accessLogging.envoyService.enabled | spec.meshConfig.enableEnvoyAccessLogService |
| proxy.accessLogging.envoyService.tcpKeepalive | spec.meshConfig.defaultConfig.envoyAccessLogService.tcpKeepalive |
| proxy.accessLogging.envoyService.tcpKeepalive.interval | spec.meshConfig.defaultConfig.envoyAccessLogService.tcpKeepalive.interval |
| proxy.accessLogging.envoyService.tcpKeepalive.probes | spec.meshConfig.defaultConfig.envoyAccessLogService.tcpKeepalive.probes |
| proxy.accessLogging.envoyService.tcpKeepalive.time | spec.meshConfig.defaultConfig.envoyAccessLogService.tcpKeepalive.time |
| proxy.accessLogging.envoyService.tlsSettings | spec.meshConfig.defaultConfig.envoyAccessLogService.tlsSettings |
| proxy.accessLogging.file.encoding | spec.meshConfig.accessLogEncoding |
| proxy.accessLogging.file.format | spec.meshConfig.accessLogFormat |
| proxy.accessLogging.file.name | spec.meshConfig.accessLogFile |

### Basic Proxy Config

| SMCP 2.6 | Istio 3.0 |
|----------|-----------|
| proxy.adminPort | spec.meshConfig.defaultConfig.proxyAdminPort |
| proxy.concurrency | spec.meshConfig.defaultConfig.concurrency |

### Envoy Metrics Service

| SMCP 2.6 | Istio 3.0 |
|----------|-----------|
| proxy.envoyMetricsService.address | spec.meshConfig.defaultConfig.envoyMetricsService.address |
| proxy.envoyMetricsService.enabled | spec.meshConfig.enableEnvoyAccessLogService |
| proxy.envoyMetricsService.tcpKeepalive | spec.meshConfig.defaultConfig.envoyMetricsService.tcpKeepalive |
| proxy.envoyMetricsService.tlsSettings | spec.meshConfig.defaultConfig.envoyMetricsService.tlsSettings |

### Injection Config

| SMCP 2.6 | Istio 3.0 |
|----------|-----------|
| proxy.injection.alwaysInjectSelector | spec.sidecarInjectorWebhook.alwaysInjectSelector |
| proxy.injection.neverInjectSelector | spec.sidecarInjectorWebhook.neverInjectSelector |
| proxy.injection.injectedAnnotations | spec.sidecarInjectorWebhook.injectedAnnotations |
| proxy.injection.autoInject | spec.values.global.proxy.autoInject |

### Proxy Logging Config

| SMCP 2.6 | Istio 3.0 |
|----------|-----------|
| proxy.logging.componentLevels | spec.values.global.proxy.componentLogLevel |
| proxy.logging.level | spec.values.global.logging.level |

### Proxy Networking Config

| SMCP 2.6 | Istio 3.0 |
|----------|-----------|
| proxy.networking.clusterDomain | spec.values.global.proxy.clusterDomain |
| proxy.networking.connectionTimeout | spec.meshConfig.connectTimeout |
| proxy.networking.dns.refreshRate | spec.meshConfig.dnsRefreshRate |
| proxy.networking.dns.searchSuffixes | spec.values.global.podDNSSearchNamespaces |
| proxy.networking.initialization.type | Not directly available |
| proxy.networking.initialization.initContainer.runtime.env | Not directly available |
| proxy.networking.initialization.initContainer.runtime.imageName | spec.values.global.proxy_init.image |
| proxy.networking.initialization.initContainer.runtime.imagePullPolicy | spec.values.global.imagePullPolicy |
| proxy.networking.initialization.initContainer.runtime.imagePullSecrets | spec.values.global.imagePullSecrets |
| proxy.networking.initialization.initContainer.runtime.imageRegistry | spec.values.global.hub |
| proxy.networking.initialization.initContainer.runtime.imageTag | spec.values.global.tag |
| proxy.networking.initialization.initContainer.runtime.resources | spec.values.global.proxy_init.resources |
| proxy.networking.maxConnectionAge | spec.values.pilot.keepaliveMaxServerConnectionAge |
| proxy.networking.protocol.autoDetect | Not directly available |
| proxy.networking.protocol.inbound | Not directly available |
| proxy.networking.protocol.outbound | Not directly available |
| proxy.networking.protocol.timeout | spec.meshConfig.protocolDetectionTimeout |

### Traffic Control Config

| SMCP 2.6 | Istio 3.0 |
|----------|-----------|
| proxy.networking.trafficControl.inbound.excludedPorts | spec.values.global.proxy.excludeInboundPorts |
| proxy.networking.trafficControl.inbound.includedPorts | spec.values.global.proxy.includeInboundPorts |
| proxy.networking.trafficControl.inbound.interceptionMode | spec.values.meshConfig.defaultConfig.interceptionMode |
| proxy.networking.trafficControl.outbound.excludedIPRanges | spec.values.global.proxy.excludeIPRanges |
| proxy.networking.trafficControl.outbound.excludedPorts | spec.values.global.proxy.excludeOutboundPorts |
| proxy.networking.trafficControl.outbound.includedIPRanges | spec.values.global.proxy.includeIPRanges |
| proxy.networking.trafficControl.outbound.policy | spec.meshConfig.outboundTrafficPolicy.mode |

### Proxy Runtime Config

| SMCP 2.6 | Istio 3.0 |
|----------|-----------|
| proxy.runtime.container.env | spec.values.global.proxy.env |
| proxy.runtime.container.imageName | spec.values.global.proxy.image |
| proxy.runtime.container.imagePullPolicy | spec.values.global.imagePullPolicy |
| proxy.runtime.container.imagePullSecrets | spec.values.global.imagePullSecrets |
| proxy.runtime.container.imageRegistry | spec.values.global.hub |
| proxy.runtime.container.imageTag | spec.values.global.tag |
| proxy.runtime.container.resources | spec.values.global.proxy.resources |
| proxy.runtime.readiness.failureThreshold | spec.values.global.proxy.readinessFailureThreshold |
| proxy.runtime.readiness.initialDelaySeconds | spec.values.global.proxy.readinessInitialDelaySeconds |
| proxy.runtime.readiness.periodSeconds | spec.values.global.proxy.readinessPeriodSeconds |
| proxy.runtime.readiness.rewriteApplicationProbes | spec.values.sidecarInjectorWebhook.rewriteAppHTTPProbe |
| proxy.runtime.readiness.statusPort | spec.values.global.proxy.statusPort |

## Runtime Configuration

### Container Configs

| SMCP 2.6 | Istio 3.0 |
|----------|-----------|
| runtime.components.container.env | spec.values.pilot.env |
| runtime.components.container.imageName | spec.values.pilot.image |
| runtime.components.container.imagePullPolicy | spec.values.global.imagePullPolicy |
| runtime.components.container.imagePullSecrets | spec.values.global.imagePullSecrets |
| runtime.components.container.imageRegistry | spec.values.global.hub |
| runtime.components.container.imageTag | spec.values.pilot.tag |
| runtime.components.container.resources | spec.values.pilot.resources |

### Deployment Configs

| SMCP 2.6 | Istio 3.0 |
|----------|-----------|
| runtime.components.deployment.autoScaling.enabled | spec.values.pilot.autoscaleEnabled |
| runtime.components.deployment.autoScaling.maxReplicas | spec.values.pilot.autoscaleMax |
| runtime.components.deployment.autoScaling.minReplicas | spec.values.pilot.autoscaleMin |
| runtime.components.deployment.autoScaling.targetCPUUtilizationPercentage | spec.values.pilot.cpu.targetAverageUtilization |
| runtime.components.deployment.replicas | spec.values.pilot.replicaCount |
| runtime.components.deployment.strategy.rollingUpdate.maxSurge | spec.values.pilot.rollingMaxSurge |
| runtime.components.deployment.strategy.rollingUpdate.maxUnavailable | spec.values.pilot.rollingMaxUnavailable |
| runtime.components.deployment.strategy.type | Not directly available |

### Pod Configs

| SMCP 2.6 | Istio 3.0 |
|----------|-----------|
| runtime.components.pod.affinity | spec.values.pilot.affinity |
| runtime.components.pod.affinity.nodeAffinity | spec.values.pilot.affinity.nodeAffinity |
| runtime.components.pod.affinity.podAffinity | spec.values.pilot.affinity.podAffinity |
| runtime.components.pod.affinity.podAntiAffinity | spec.values.pilot.affinity.podAntiAffinity |
| runtime.components.pod.metadata.annotations | spec.values.pilot.podAnnotations |
| runtime.components.pod.metadata.labels | spec.values.pilot.podLabels |
| runtime.components.pod.nodeSelector | spec.values.pilot.nodeSelector |
| runtime.components.pod.priorityClassName | spec.values.pilot.priorityClassName |
| runtime.components.pod.tolerations | spec.values.pilot.tolerations |

### Defaults Configs

| SMCP 2.6 | Istio 3.0 |
|----------|-----------|
| runtime.defaults.container.imagePullPolicy | spec.values.global.imagePullPolicy |
| runtime.defaults.container.imagePullSecrets | spec.values.global.imagePullSecrets |
| runtime.defaults.container.imageRegistry | spec.values.global.hub |
| runtime.defaults.container.imageTag | spec.values.global.tag |
| runtime.defaults.container.resources | spec.values.global.defaultResources |
| runtime.defaults.deployment.podDisruption.enabled | spec.values.global.defaultPodDisruptionBudget.enabled |
| runtime.defaults.deployment.podDisruption.maxUnavailable | Not directly available |
| runtime.defaults.deployment.podDisruption.minAvailable | Not directly available |
| runtime.defaults.pod.nodeSelector | spec.values.global.defaultNodeSelector |
| runtime.defaults.pod.priorityClassName | spec.values.global.priorityClassName |
| runtime.defaults.pod.tolerations | spec.values.global.defaultTolerations |

## Security Configuration

### Certificate Authority

| SMCP 2.6 | Istio 3.0 |
|----------|-----------|
| security.certificateAuthority.cert-manager | spec.meshConfig.ca AND spec.values.global.pilotCertProvider |
| security.certificateAuthority.cert-manager.address | spec.meshConfig.ca.address |
| security.certificateAuthority.cert-manager.pilotSecretName | Not directly available |
| security.certificateAuthority.cert-manager.rootCAConfigMapName | Not directly available |
| security.certificateAuthority.custom.address | spec.meshConfig.ca.address |

### Istiod CA

| SMCP 2.6 | Istio 3.0 |
|----------|-----------|
| security.certificateAuthority.istiod.privateKey.rootCADir | Not directly available |
| security.certificateAuthority.istiod.type | spec.values.global.pilotCertProvider |
| security.certificateAuthority.istiod.selfSigned.checkPeriod | Not directly available |
| security.certificateAuthority.istiod.selfSigned.enableJitter | Not directly available |
| security.certificateAuthority.istiod.selfSigned.gracePeriod | Not directly available |
| security.certificateAuthority.istiod.selfSigned.ttl | Not directly available |
| security.certificateAuthority.istiod.workloadCertTTLDefault | Not directly available |
| security.certificateAuthority.istiod.workloadCertTTLMax | Not directly available |

### Control Plane Security

| SMCP 2.6 | Istio 3.0 |
|----------|-----------|
| security.controlPlane.certProvider | spec.values.global.pilotCertProvider |
| security.controlPlane.mtls | spec.meshConfig.enableAutoMtls |
| security.controlPlane.tls.cipherSuites | spec.meshConfig.tlsDefaults.cipherSuites |
| security.controlPlane.tls.ecdhCurves | spec.meshConfig.tlsDefaults.ecdhCurves |
| security.controlPlane.tls.maxProtocolVersion | Not directly available |
| security.controlPlane.tls.minProtocolVersion | spec.meshConfig.tlsDefaults.minProtocolVersion |

### Data Plane Security

| SMCP 2.6 | Istio 3.0 |
|----------|-----------|
| security.dataPlane.automtls | spec.meshConfig.enableAutoMtls |
| security.dataPlane.mtls | spec.meshConfig.meshMTLS.enabled |

### Identity Config

| SMCP 2.6 | Istio 3.0 |
|----------|-----------|
| security.identity.thirdParty.audience | spec.values.global.sds.token.aud |
| security.identity.thirdParty.issuer | Not directly available |
| security.identity.type | Not directly available |

### Other Security

| SMCP 2.6 | Istio 3.0 |
|----------|-----------|
| security.jwksResolverCA | spec.values.pilot.jwksResolverExtraRootCA |
| security.manageNetworkPolicy | Not directly available |
| security.trust.domain | spec.meshConfig.trustDomain |
| security.trust.additionalDomains | spec.meshConfig.trustDomainAliases |

## Telemetry Configuration

| SMCP 2.6 | Istio 3.0 |
|----------|-----------|
| spec.telemetry.type | Not directly available (Only native telemetry in 3.0) |
| spec.telemetry.mixer | Removed (Mixer deprecated) |
| spec.telemetry.remote | Replaced by extensionProviders |

## Tracing Configuration

| SMCP 2.6 | Istio 3.0 |
|----------|-----------|
| spec.tracing.sampling | spec.values.pilot.traceSampling |
| spec.tracing.type | Not directly available |
