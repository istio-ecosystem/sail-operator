# SMCP Fields No Longer Supported in Istio 3.0

Note that these fields being removed does not always indicate a removal of functionality. In many cases these fields are no longer configurable in upstream istio as only default values are used for fields that used to be configurable.

## Addon Management
- spec.addons.3scale
- spec.addons.grafana
- spec.addons.jaeger
- spec.addons.kiali
- spec.addons.prometheus
- spec.addons.stackdriver (partial - some config still available in `istio`)

## Cluster Configuration
- cluster.meshExpansion.ilbGateway
- cluster.multiCluster.meshNetworks.gateways.service

## Gateway Management
- All gateway configuration sections must now be configured separately
- spec.gateways (entire section)
- Route-specific configurations on ingress gateway spec

## OpenShift-Specific Features
- spec.openshiftRoute (IOR) configuration
- spec.security.identity (OpenShift-specific security)

## Policy Controls
- spec.policy.type (Istiod is now the only option)
- spec.policy.mixer (Mixer deprecated)
- spec.policy.remote
- All adapter configurations

## Security
- security.certificateAuthority.cert-manager.pilotSecretName
- security.certificateAuthority.cert-manager.rootCAConfigMapName
- security.certificateAuthority.istiod.privateKey.rootCADir
- security.certificateAuthority.istiod.selfSigned.checkPeriod
- security.certificateAuthority.istiod.selfSigned.enableJitter
- security.certificateAuthority.istiod.selfSigned.gracePeriod
- security.certificateAuthority.istiod.selfSigned.ttl
- security.certificateAuthority.istiod.workloadCertTTLDefault
- security.certificateAuthority.istiod.workloadCertTTLMax
- security.controlPlane.tls.maxProtocolVersion
- security.identity.thirdParty.issuer
- security.identity.type
- security.manageNetworkPolicy

## Network Management
- Network policies must be managed manually
- proxy.networking.initialization.type
- proxy.networking.initialization.initContainer.runtime.env
- proxy.networking.protocol.autoDetect
- proxy.networking.protocol.inbound
- proxy.networking.protocol.outbound

## Runtime Configuration
- runtime.components.deployment.strategy.type
- runtime.defaults.deployment.podDisruption.maxUnavailable
- runtime.defaults.deployment.podDisruption.minAvailable

## Telemetry
- spec.telemetry.type (only native telemetry in 3.0)
- spec.telemetry.mixer (removed)
- spec.telemetry.remote (replaced by extensionProviders)
- spec.tracing.type