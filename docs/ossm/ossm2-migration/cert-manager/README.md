# OpenShift Service Mesh 2.6 --> 3.0 Migration with Cert-Manager

When migrating from OpenShift Service Mesh 2.6 --> 3.0 while using Cert-Manager you can largely follow the [ClusterWide](TODO) or [MultiTenant](../multi-tenancy/README.md) migration guides. This document details a few necessary additional steps to follow before creating your `Istio` resource to ensure your cert manager configuration works with 3.0. 

<!--

Steps for testing:

1. Install cert-manager operator

1. Install cluster issuer:

```yaml
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: selfsigned-root-issuer
  namespace: cert-manager
spec:
  selfSigned: {}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: root-ca
  namespace: cert-manager
spec:
  isCA: true
  duration: 21600h # 900d
  secretName: root-ca
  commonName: root-ca.my-company.net
  subject:
    organizations:
    - my-company.net
  issuerRef:
    name: selfsigned-root-issuer
    kind: Issuer
    group: cert-manager.io
---
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: root-ca
spec:
  ca:
    secretName: root-ca
```

1. Install istio-ca

```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: istio-ca
  namespace: istio-system
spec:
  isCA: true
  duration: 21600h
  secretName: istio-ca
  commonName: istio-ca.my-company.net
  subject:
    organizations:
    - my-company.net
  issuerRef:
    name: root-ca
    kind: ClusterIssuer
    group: cert-manager.io
---
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: istio-ca
  namespace: istio-system
spec:
  ca:
    secretName: istio-ca
```

1. Helm install istio-csr

values.yaml

```yaml
image:
  repository: quay.io/jetstack/cert-manager-istio-csr

app:
  certmanager:
    namespace: istio-system
    issuer:
      group: cert-manager.io
      kind: Issuer
      name: istio-ca

  controller:
    leaderElectionNamespace: istio-system

  istio:
    namespace: istio-system
    revisions: ["basic"]

  server:
    maxCertificateDuration: 5m

  tls:
    certificateDNSNames:
    # This DNS name must be set in the SMCP spec.security.certificateAuthority.cert-manager.address
    - cert-manager-istio-csr.istio-system.svc
```

```console
helm repo add jetstack https://charts.jetstack.io --force-update
helm upgrade cert-manager-istio-csr jetstack/cert-manager-istio-csr \
    --install \
    --namespace istio-system \
    --wait \
    -f values.yaml
```


1. Create SMCP

```yaml
apiVersion: maistra.io/v2
kind: ServiceMeshControlPlane
metadata:
  name: basic
  namespace: istio-system
spec:
  addons:
    grafana:
      enabled: false
    kiali:
      enabled: false
    prometheus:
      enabled: false
  gateways:
    enabled: false
    openshiftRoute:
      enabled: false
  profiles:
    - default
  security:
    certificateAuthority:
      cert-manager:
        address: cert-manager-istio-csr.istio-system.svc:443
      type: cert-manager
    dataPlane:
      mtls: true
    identity:
      type: ThirdParty
    manageNetworkPolicy: false
  tracing:
    type: None
  version: v2.6
```

1. Update istio-csr with future Istio revision.

```console
helm upgrade cert-manager-istio-csr jetstack/cert-manager-istio-csr \
    --install \
    --reuse-values \
    --namespace istio-system \
    --wait \
    --set "app.istio.revisions={basic,ossm3-v1-24-1}"
```

1. Create Istio resource

```yaml
apiVersion: sailoperator.io/v1alpha1
kind: Istio
metadata:
  generation: 3
  name: ossm3
spec:
  namespace: istio-system
  updateStrategy:
    type: RevisionBased
  values:
    global:
      caAddress: cert-manager-istio-csr.istio-system.svc:443
    pilot:
      env:
        ENABLE_CA_SERVER: "false"
  version: v1.24.1
```

1. Create bookinfo namespace

```console
oc create ns bookinfo
```

1. Create smmr and add bookinfo to it
```yaml
apiVersion: maistra.io/v1
kind: ServiceMeshMemberRoll
metadata:
  name: default
  namespace: istio-system
spec:
  members:
    - bookinfo
```

1. Deploy bookinfo
```console
oc apply -n bookinfo -f https://raw.githubusercontent.com/Maistra/istio/maistra-2.6/samples/bookinfo/platform/kube/bookinfo.yaml
```

1. Ensure proxies injected

```console
oc get pods -n bookinfo
NAME                              READY   STATUS    RESTARTS   AGE
details-v1-9979968fb-776jq        2/2     Running   0          33m
productpage-v1-8669b4d5c8-hshtz   2/2     Running   0          33m
ratings-v1-bbb89988d-tcgvp        2/2     Running   0          33m
reviews-v1-75b6949cf4-7kbdm       2/2     Running   0          33m
reviews-v2-64f68558b-gsxc4        2/2     Running   0          33m
reviews-v3-596954cfd6-jnb6n       2/2     Running   0          33m
```

1. Migrate to 3.0

```console
oc label ns bookinfo istio.io/rev=ossm3-v1-24-1 maistra.io/ignore-namespace="true" istio-injection- --overwrite=true
oc rollout restart deployment -n bookinfo
```

1. Ensure proxies connected to correct control plane
```console
istioctl ps -n bookinfo
NAME                                         CLUSTER        CDS        LDS        EDS        RDS        ECDS     ISTIOD                                   VERSION
details-v1-9979968fb-776jq.bookinfo          Kubernetes     SYNCED     SYNCED     SYNCED     SYNCED              istiod-ossm3-v1-24-1-d5b9b4c89-ccz4v     1.24.1
productpage-v1-8669b4d5c8-hshtz.bookinfo     Kubernetes     SYNCED     SYNCED     SYNCED     SYNCED              istiod-ossm3-v1-24-1-d5b9b4c89-ccz4v     1.24.1
ratings-v1-bbb89988d-tcgvp.bookinfo          Kubernetes     SYNCED     SYNCED     SYNCED     SYNCED              istiod-ossm3-v1-24-1-d5b9b4c89-ccz4v     1.24.1
reviews-v1-75b6949cf4-7kbdm.bookinfo         Kubernetes     SYNCED     SYNCED     SYNCED     SYNCED              istiod-ossm3-v1-24-1-d5b9b4c89-ccz4v     1.24.1
reviews-v2-64f68558b-gsxc4.bookinfo          Kubernetes     SYNCED     SYNCED     SYNCED     SYNCED              istiod-ossm3-v1-24-1-d5b9b4c89-ccz4v     1.24.1
reviews-v3-596954cfd6-jnb6n.bookinfo         Kubernetes     SYNCED     SYNCED     SYNCED     SYNCED              istiod-ossm3-v1-24-1-d5b9b4c89-ccz4v     1.24.1
```
 -->

Starting with a `ServiceMeshControlPlane` with cert-manager configured:

```yaml
apiVersion: maistra.io/v2
kind: ServiceMeshControlPlane
metadata:
  name: basic
  namespace: istio-system
spec:
  ...
  security:
    certificateAuthority:
      cert-manager:
        address: cert-manager-istio-csr.istio-system.svc:443
      type: cert-manager
    dataPlane:
      mtls: true
    identity:
      type: ThirdParty
    manageNetworkPolicy: false
```

You will need to perform these updates to your istio-csr deployment:

- The `app.istio.revisions` field needs to include your 3.0 control plane revision _before_ you create your `Istio` resource.

  Adding your 3.0 control plane revision to your istio-csr deployment will ensure that proxies can properly communicate with the 3.0 control plane.

  ```console
  helm upgrade cert-manager-istio-csr jetstack/cert-manager-istio-csr \
      --install \
      --reuse-values \
      --namespace istio-system \
      --wait \
      --set "app.istio.revisions={basic,ossm3-v1-24-1}"
  ```

  Depending on whether you will use a `RevisionBased` update strategy or an `InPlace` update strategy, your revision name will vary. If using an `InPlace` strategy, your revision name will match your `Istio` name. If using a `RevisionBased` strategy, revision names use the following format, `<istio-name>-v<major_version>-<minor_version>-<patch_version>`. For example: `ossm3-v1-24-1`.

- The `app.controller.configmapNamespaceSelector` field needs to be either unset _before_ the migration begins or updated _after_ you have completed your migration.

  If you have set the `app.controller.configmapNamespaceSelector` field on your istio-csr deployment to `maistra.io/member-of`, you will need to update this accordingly. If you haven't set this field, you can keep it unset.

  If the `configmapNamespaceSelector` field on your istio-csr deployment is set, the istio CA configmap will only be injected into namespaces that match the label selector. `MultiTenant` deployments with more than one `ServiceMeshControlPlane` in the cluster should not remove this field since the wrong CA configmap would likely get written to the namespace. `ClusterWide` deployments with only a single `SMCP` can choose to leave this unset. If you are keeping the field set, you need to wait until **after** you have completed your migration to update the `configmapNamespaceSelector` field. Otherwise namespaces without the injection label will no longer have the configmap CA injected.

  To unset this field:

  ```console
  helm upgrade cert-manager-istio-csr jetstack/cert-manager-istio-csr \
      --install \
      --reuse-values \
      --namespace istio-system \
      --wait \
      --set "app.controller.configmapNamespaceSelector="
  ```

  To update this field:

  > **_NOTE:_** Before updating, ensure you have completely finished your migration and the new injection label, in this example `istio-injection=enabled`, is present on all workload namespaces before updating istio-csr.

  ```console
  helm upgrade cert-manager-istio-csr jetstack/cert-manager-istio-csr \
     --install \
     --reuse-values \
     --namespace istio-system \
     --wait \
     --set "app.controller.configmapNamespaceSelector=istio-injection=enabled"
  ```

After updating your istio-csr deployment, you can create the `Istio` resource with the following settings to work with cert-manager. Similar to the 2.6 controlplane, these settings disable the built-in CA server and instead use the istio-csr address.

- Create Istio Resource.

  ```yaml
  apiVersion: sailoperator.io/v1alpha1
  kind: Istio
  metadata:
    name: ossm3
  spec:
    ...
    namespace: istio-system
    values:
      global:
        caAddress: cert-manager-istio-csr.istio-system.svc:443
      pilot:
        env:
          ENABLE_CA_SERVER: "false"
  ```

That's it. From here you can follow the [MultiTenant](../multi-tenancy/README.md) or [ClusterWide](TODO) guides for migrating workloads from your 2.6 control plane to the 3.0 control plane.
