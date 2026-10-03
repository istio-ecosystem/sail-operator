---
base_branch: main
languages: [go, bash]
key_paths:
  - pkg/install/
  - pkg/config/
  - controllers/
  - api/v1/
  - tests/e2e/
  - tests/integration/
  - hack/
---

Sail Operator: Kubernetes operator managing Istio service mesh lifecycle via CRDs (Istio, IstioRevision, IstioCNI, ZTunnel). Built with Kubebuilder/controller-runtime. Linted with golangci-lint, tested with standard Go testing (unit) and Ginkgo/Gomega (integration/e2e).
