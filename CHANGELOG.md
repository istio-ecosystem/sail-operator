# Changelog

All notable changes to the Sail Operator are documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/).

The changelog for the next version is compiled from the YAML files in the
`changelog/` directory at release time, and the files are deleted afterwards.

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
