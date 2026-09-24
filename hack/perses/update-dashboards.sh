#!/usr/bin/env bash
# Copyright Istio Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -euo pipefail

# Regenerates vendored PersesDashboard manifests from perses/community-mixins.
#
# Usage:
#   hack/perses/update-dashboards.sh [COMMUNITY_MIXINS_REF]
#
# COMMUNITY_MIXINS_REF defaults to the pinned release below.

COMMUNITY_MIXINS_REF="${1:-v0.7.0}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUTPUT_DIR="${REPO_ROOT}/resources/perses/dashboards"
README="${OUTPUT_DIR}/README.md"
TMP_DIR="$(mktemp -d)"
DASHBOARD_FILES=(
  istio-control-plane.yaml
  istio-mesh-dashboard.yaml
  istio-performance.yaml
  istio-service-dashboard.yaml
  istio-workload-dashboard.yaml
  istio-ztunnel-dashboard.yaml
)
VENDORED_DATE="$(date -u +"%Y-%m-%d")"

cleanup() {
  rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

echo "Downloading community-mixins dashboards from ${COMMUNITY_MIXINS_REF}"
git clone --depth 1 --branch "${COMMUNITY_MIXINS_REF}" https://github.com/perses/community-mixins.git "${TMP_DIR}/community-mixins"

mkdir -p "${OUTPUT_DIR}"
for file in "${DASHBOARD_FILES[@]}"; do
  cp "${TMP_DIR}/community-mixins/examples/dashboards/operator/istio/${file}" "${OUTPUT_DIR}/${file}"
done

cat > "${README}" <<EOF
# Istio Perses dashboards

Vendored \`PersesDashboard\` manifests from [perses/community-mixins](https://github.com/perses/community-mixins).

| Field | Value |
|-------|-------|
| Source | \`examples/dashboards/operator/istio/\` |
| Ref | \`${COMMUNITY_MIXINS_REF}\` |
| Last updated | \`${VENDORED_DATE}\` |

## Bundled dashboards

| Dashboard ID | Display name |
|--------------|--------------|
| \`istio-control-plane\` | Istio Control Plane Dashboard |
| \`istio-mesh-dashboard\` | Istio Mesh Dashboard |
| \`istio-performance\` | Istio Performance Dashboard |
| \`istio-service-dashboard\` | Istio Service Dashboard |
| \`istio-workload-dashboard\` | Istio Workload Dashboard |
| \`istio-ztunnel-dashboard\` | Istio Ztunnel Dashboard |

Dashboard IDs must remain stable for Kiali and other consumers.

## Behavior

When the \`PersesDashboard\` CRD is present in the cluster, the Sail Operator creates these dashboards in its own namespace (the namespace where the operator pod runs). Missing CRDs do not block operator installation or report a failure.

Reconciliation is create-if-not-exists: existing dashboards are left unchanged. Dashboards are not deleted and have no ownerReferences.

## PersesDatasource requirement

The Sail Operator does **not** create or manage \`PersesDatasource\` resources. You must create one named \`prometheus-datasource\` in the operator namespace before the dashboards can display data. Support for \`PersesGlobalDatasource\` is out of scope.

Example:

\`\`\`yaml
apiVersion: perses.dev/v1alpha2
kind: PersesDatasource
metadata:
  name: prometheus-datasource   # required name
  namespace: sail-operator      # operator namespace
spec:
  # configure your Prometheus/Thanos endpoint
\`\`\`

Regenerate these files with \`hack/perses/update-dashboards.sh [COMMUNITY_MIXINS_REF]\`.
EOF

echo "Updated ${OUTPUT_DIR}"
echo "Updated ${README}"
