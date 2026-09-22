# Changelog

All notable changes to the Sail Operator are documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/).

The changelog for the next version is compiled from the YAML files in the
`changelog/` directory at release time, and the files are deleted afterwards.

## v1.31.0 - 2026-09-21

### Added
- Add support for Istio 1.30.4 and 1.29.7

- Enable NetworkPolicy defaults for fresh installs on OCP 5+ clusters
  On OCP 5+ clusters, fresh istiod, istio-cni, and ztunnel Helm releases
  enable NetworkPolicy by default. Existing releases are not changed during
  an OpenShift or operator upgrade. Set spec.values.global.networkPolicy.enabled
  explicitly to override the default; explicit false settings are preserved.
  The OpenShift version is detected when the operator starts. Restart the
  operator after upgrading OpenShift if new releases should receive the OCP 5
  default immediately.

### Changed
- Add correct ciphersuite and ECDH curves on OpenShift
  Removes cipher suite list from meshConfig when minProtocolVersion is TLS 1.3
  and correctly sets ECDH curves from the OpenShift TLS profile.

- Remove `TLS12_ENABLED` from ZTunnel for Istio 1.31+
  Istio 1.31's OpenSSL backend supports FIPS 140-3 natively and no longer needs
  the explicit TLS 1.2 enablement flag.

- Resolve cipher suites directly from tlsProfile
  Fixes cipher suites being filtered out by the controller-runtime library when
  TLS min version is 1.3. The operator now reads them directly from the profile.

- Pass TLS 1.3 cipher suites to Envoy via proxy metadata
  Allows passing TLS 1.3 cipher suites to envoy on OpenShift by passing
  the env var `OPENSSL_TLS1_3_CIPHERSUITES` through proxy metadata. Syncs
  this with the APIServer TLS settings.

- Detect remote istiod webhook failures from cluster events instead of probing
  The webhook controller no longer actively probes the remote istiod's readiness
  endpoint. It now passively detects webhook call failures from cluster events and
  marks the webhook and its owning IstioRevision as not-ready for a short degraded
  window after a failure. The length of this window is configurable via the
  WEBHOOK_DEGRADED_WINDOW environment variable (default 2 minutes).

### Fixed
- Fix race condition in `ToDiscoveryClient` ([#2260](https://github.com/istio-ecosystem/sail-operator/issues/2260))
  Concurrent calls shared the same config object; the fix copies it before
  creating the discovery client.

- Reconcile OwnerRef on IstioRevision object during update ([#2083](https://github.com/istio-ecosystem/sail-operator/issues/2083))
  When revision.CreateOrUpdate reconciles during update, the OwnerReference
  is not being updated, which could lead to a state, where the orphaned
  revision is never pruned or reflected in "status.revisions".
  Add OwnerReference during reconcile update.

## v1.30.0 - 2026-05-28

### Added
- Add support for Istio 1.30.0, 1.29.3 and 1.28.7

- Expose "dnsConfig" and "dnsPolicy" Ztunnel values to Sail Operator
  Allows users to customize DNS settings for ZTunnel pods directly through the
  operator API.

- Make managed-by label value configurable via ChartManagerOption
  Downstream consumers of the Sail library can now set their own
  `app.kubernetes.io/managed-by` label instead of the hardcoded value.

- Add kubebuilder validation for revisionTagTargetRef
  The `kind` field on `IstioRevisionTagTargetReference` now only accepts "Istio"
  or "IstioRevision" via proper enum validation.

- Add targetRef field to ZTunnel CRD
  Allows a ZTunnel resource to reference an Istio or IstioRevision, keeping
  version, namespace, and values in sync and reducing configuration duplication.

- Add documentation for resource customization
  Documents how to use the `sailoperator.io/ignore` annotation to customize
  Helm-managed resources without the operator reverting changes.

- Add operator `TLSConfig` and sync with APIServer TLS profile on openshift
  On OpenShift, reads TLS settings from the cluster's APIServer resource and
  applies them to all managed Istio resources and the operator's metrics endpoint.

### Changed
- Use registry.istio.io
  Migrates all container image references from `gcr.io/istio-testing` and
  `docker.io/istio` to the new `registry.istio.io` registry.

- Handle errors in Helm discovery client
  The Helm REST client getter now properly returns errors from the discovery
  client instead of silently ignoring them.

- Migrate from Helm v3 to Helm v4
  Upgrades all Helm dependencies to v4.1.0, adapting to API changes including
  the new `release.Releaser` interface and moved package paths.

- Sync min tls version from `TLSConfig` to `Istio`
  Extends the TLS profile sync to also propagate the minimum TLS version from
  the operator's TLSConfig to Istio resources.

### Fixed
- Fix infinite reconciliation on webhook resources
  Istiod updates `CABundle` and `FailurePolicy` on webhook configurations; the
  controller now ignores those field changes to avoid continuous re-reconciliation.

- Fix missing MaxConcurrentReconciles in ZTunnel controller
  The ZTunnel controller now respects the `MaxConcurrentReconciles` setting,
  which was already used by the Istio and IstioCNI controllers.

- Write correct helm value for FIPS-140-2 support
  The `TLS12_ENABLED` environment variable was written to the wrong Helm values
  path and silently ignored.

- Fix infinite reconcile loop when Istio version is EOL ([#1689](https://github.com/istio-ecosystem/sail-operator/issues/1689))
  EOL version errors are now treated as non-retriable, so the reconciler sets
  the error status once and stops instead of retrying with exponential backoff.

- Use operator name as prefix in metrics-reader clusterrole
  Avoids naming collisions when multiple operator instances are deployed in the
  same cluster.

- Ensure base validator is created for default rev
  The validating webhook was not created for the default revision when
  `defaultRevision` was set to a non-empty value.

## v1.29.2 - 2026-05-08

### Added
- Add support for Istio 1.29.2, 1.28.6 and 1.27.9

### Changed
- Use registry.istio.io for image references

- Improve OpenShift platform configuration handling

### Fixed
- Fix infinite reconcile loop when Istio version is EOL ([#1689](https://github.com/istio-ecosystem/sail-operator/issues/1689))

## v1.29.1 - 2026-03-12

### Added
- Add support for istio 1.29.1, 1.28.5 and 1.27.8

### Fixed
- Fix infinite reconciliation on webhook resources

## v1.29.0 - 2026-02-26

### Added
- Expose "peerCaCrl" Ztunnel param added in Helm
  Allows users to configure a CRL for peer CA validation in ZTunnel.

- Enable TLSv1.2 for ZTunnel when in FIPS mode
  Sets the `TLS12_ENABLED` environment variable on ZTunnel pods when the
  cluster is running in FIPS mode.

### Changed
- Add ZTunnel v1 CRD version
  Promotes the ZTunnel CRD from v1alpha1 to v1.

- Set preserve-unknown-fields on gatewayClasses
  Prevents validation errors when GatewayClass resources contain fields not
  known to the operator's schema.

### Fixed
- Minimize wildcard use in operator ClusterRoles
  Replaces wildcard RBAC permissions with explicit resource/verb lists for
  least-privilege compliance.

- Fix profile column status

### Removed
- Remove Profile printcolumn from ztunnel status

## v1.28.3 - 2026-02-04

### Added
- Add support for Istio 1.28.3

## v1.28.2 - 2026-01-12

### Added
- Add support for Istio 1.28.2, 1.27.5 and 1.26.8

### Changed
- Set preserve-unknown-fields on gatewayClasses

## v1.28.1 - 2025-12-11

### Added
- Add support for Istio 1.28.1, 1.27.4 and 1.26.7

## v1.28.0 - 2025-11-26

### Added
- Add ZTunnel v1 CRD version

### Removed
- Remove Profile printcolumn from ztunnel status
