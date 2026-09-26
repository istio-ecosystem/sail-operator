# Security Review — sail-operator

**Start by reading `AGENTS.md § Security`** for the project's baseline security requirements.

## 1. Secret scanning
- Run `make lint-secrets` to scan for secrets. The hook is opt-in via `make git-hook`.
- Run `git log --all --oneline` and check for any commit that may have introduced secrets.
- No credentials, tokens, or keys in any committed file — including YAML manifests and test fixtures.

## 2. RBAC
- RBAC is defined by `+kubebuilder:rbac` markers in `controllers/*` and generated into a single operator ClusterRole at `chart/templates/rbac/role.yaml`. Review both the markers and the generated role.
- No `*` verbs on cluster-scoped resources — flag any wildcard verb with **Critical** severity.
- New permissions must be justified by an actual code path in the reconciler that requires them.
- Check for resource-level constraints (`resourceNames`) where possible.

## 3. Webhook configuration
- `WebhookController` (`controllers/webhook/`) manages `MutatingWebhookConfiguration`.
- Verify `failurePolicy: Fail` is used for security-critical webhooks (sidecar injection).
- Confirm CA bundle injection is wired correctly — a misconfigured CA breaks injection silently.
- Check `IstioRevisionTagController` (`controllers/istiorevisiontag/`) for `ValidatingWebhookConfiguration` changes.

## 4. Secret handling in controller code
- Search changed files for `log.*secret`, `fmt.Sprintf.*token`, or any logging statement that may include secret values.
- Controller reconcilers must use `corev1.EnvFromSource` with `SecretKeyRef` — not inline string values.
- Secrets must never be stored in ConfigMaps.

## 5. Network exposure
- New `Service` resources created by controllers: verify `type` is `ClusterIP` unless explicitly required otherwise.
- New ports opened on controller pods must be reflected in the operator chart (`chart/templates/`) and any corresponding RBAC/NetworkPolicy.

## 6. Image and supply-chain
- Check `Dockerfile` and bundle manifests for `latest` tags — flag any as **High**.
- Base images must use digest pins (`FROM image@sha256:...`).
- No new external registries without team approval.

## Output
Report each finding as:
| Severity | Location | Issue | Remediation |
|---|---|---|---|
