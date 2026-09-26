# Perses tooling

Helpers for vendoring Istio Perses dashboards and validating them on OpenShift.

| Script | Purpose |
|--------|---------|
| [`update-dashboards.sh`](update-dashboards.sh) | Refresh YAML from [perses/community-mixins](https://github.com/perses/community-mixins) (source of truth) into `resources/perses/dashboards/` |
| [`setup-openshift.sh`](setup-openshift.sh) | Dev/validation setup on OpenShift (UWM, Perses, Istio, bookinfo) |

Manifests used by the setup script live in [`manifests/`](manifests/).

## Prerequisites (OpenShift)

1. `oc login` to the cluster
2. Install **Cluster Observability Operator** from OperatorHub (manual)
3. A project to host the operator image (examples below use `sail-operator`)

## Build and deploy the operator image

`make docker-build` always uses **Docker** (not Podman). Prefer Docker for build; for push to an OpenShift registry with a self-signed route cert, transfer the image to Podman and push with `--tls-verify=false`.

Pods must pull via the **internal** registry URL. Push via the **external** route.

```bash
cd /path/to/sail-operator   # branch with Perses dashboard changes

# External registry host (from: oc get route default-route -n openshift-image-registry -o jsonpath='{.spec.host}')
HOST=$(oc get route default-route -n openshift-image-registry -o jsonpath='{.spec.host}')
export HUB=${HOST}/sail-operator
export TAG=perses-new          # use a fresh tag each rebuild
export NAMESPACE=sail-operator

# Ensure the ImageStream project exists and the operator SA can pull
oc new-project sail-operator 2>/dev/null || true
oc policy add-role-to-user system:image-puller \
  system:serviceaccount:sail-operator:sail-operator \
  -n sail-operator

# 1) Build (Docker). Force a fresh binary if you need to invalidate cache:
#    rm -f out/linux_amd64/sail-operator
BUILD_WITH_CONTAINER=0 make docker-build
docker images | grep "$TAG"    # must show the tag above

# 2) Push: Docker → Podman (avoids TLS errors on the OpenShift route)
IMG=${HUB}/sail-operator:${TAG}
docker save "$IMG" | podman load
oc whoami -t | podman login --tls-verify=false -u unused --password-stdin "$HOST"
podman push --tls-verify=false "$IMG"

# 3) Confirm the tag exists in OpenShift BEFORE deploy
oc get istag -n sail-operator
# expect: sail-operator:${TAG}

# 4) Deploy with the INTERNAL registry URL (pods cannot use the default-route host)
export HUB=image-registry.openshift-image-registry.svc:5000/sail-operator
BUILD_WITH_CONTAINER=0 make deploy
oc delete pod -n sail-operator --all
oc rollout status deploy/sail-operator -n sail-operator

# 5) Sanity-check logs (must NOT mention MetricsIntegration)
oc logs -n sail-operator deploy/sail-operator --tail=50 | grep -iE 'Perses|MetricsIntegration|error'
# expect: "Waiting for PersesDashboard CRD" and/or "PersesDashboard CRD is ready"
```

**Do not** run `make deploy` until `oc get istag` lists your new tag.  
**Do not** retag an old image (e.g. `metrics-perses-dev`) as the new tag — rebuild from this branch.

## Cluster setup (Perses / mesh / sample)

With the operator already running:

```bash
export ISTIO_VERSION=v1.31.1          # must be a version your binary supports
export OPERATOR_NAMESPACE=sail-operator

hack/perses/setup-openshift.sh uwm
hack/perses/setup-openshift.sh perses
hack/perses/setup-openshift.sh istio
hack/perses/setup-openshift.sh monitoring
hack/perses/setup-openshift.sh bookinfo
hack/perses/setup-openshift.sh validate
```

Order: UWM → Perses datasource (operator NS) → Istio/CNI → monitors → bookinfo → validate.

Skip `setup-openshift.sh operator` / `all` unless you have adapted that step; prefer the manual build/push/deploy flow above.

## Datasource

Dashboards assume a `PersesDatasource` named `prometheus-datasource` in the **Sail Operator namespace** (default `sail-operator`). The setup script applies `manifests/perses-datasource.yaml`, which points at the OpenShift Thanos Querier (`thanos-querier.openshift-monitoring.svc.cluster.local:9091`) with TLS via `/ca/service-ca.crt`. The Perses operator creates the companion `prometheus-datasource-secret` for the HTTP proxy.

## Validate

`setup-openshift.sh validate` checks:

- `persesdashboards.perses.dev` CRD exists
- `prometheus-datasource` exists in `OPERATOR_NAMESPACE`
- All six `PersesDashboard` CRs exist in `OPERATOR_NAMESPACE`

Then generate bookinfo traffic (hit the `productpage` route) and open **Observe → Monitoring** (Perses UI plugin) in the OpenShift console.

```bash
oc get persesdashboards -n sail-operator
oc get persesdatasource -n sail-operator
```

## Update vendored dashboards

```bash
hack/perses/update-dashboards.sh            # default pinned mixins ref
hack/perses/update-dashboards.sh v0.7.0     # explicit ref
```
