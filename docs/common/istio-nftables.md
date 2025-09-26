[Return to Project Root](../README.md)

# Table of Contents

- [Istio nftables backend](#istio-nftables-backend)
  - [Prerequisites](#prerequisites)
  - [Installation](#installation)
	  - [Install with Sail Operator on OpenShift](#install-with-sail-operator-on-openshift)
    - [Install in Ambient Mode](#install-in-ambient-mode)
  - [Validation](#validation)
  - [Upgrade](#upgrade)
    - [Upgrade with Sail Operator on OpenShift](#upgrade-with-sail-operator-on-openshift)
    - [Upgrade in Ambient Mode](#upgrade-in-ambient-mode)


## Istio nftables backend

This document outlines the configuration steps for the nftables backend in Istio. As the official successor to iptables, nftables offers a modern, high-performance alternative for transparently redirecting traffic to and from the Envoy sidecar proxy. Many major Linux distributions are actively moving towards adopting native nftables support.
### Prerequisites

- **nftables version**: Requires `nft` binary version 1.0.1 or later.

### Installation

The support for native nftables when using Istio sidecar mode was implemented in the upstream istio release-1.27 [release note](https://github.com/istio/istio/blob/master/releasenotes/notes/nftables-sidecar.yaml). It is disabled by default. To enable it, you can set a feature flag as `values.global.nativeNftables=true`. For example,

Installation with Istioctl

```sh
istioctl install --set values.global.nativeNftables=true -y
```

Installation with Helm

```sh
helm install istiod-canary istio/istiod \
  --set values.global.nativeNftables=true \
  -n istio-system
```

#### Install with Sail Operator on OpenShift

When you install an Istio resource with Sail Operator, you can create an instance of the `Istio` resource with `spec.values.global.nativeNftables=true`.

```sh
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  version: v1.28-alpha.24646157
  namespace: istio-system
  updateStrategy:
    type: InPlace
    inactiveRevisionDeletionGracePeriodSeconds: 30
  values:
    global:
      nativeNftables: true
```

This feature configures Istio to use the `nftables` backend instead of `iptables` for traffic redirection.

To enable the Istio native nftables feature, using the following steps:

1. Create the `Subscription` object with a 1.27 channel.

```sh
kubectl apply -f - <<EOF
    apiVersion: operators.coreos.com/v1alpha1
    kind: Subscription
    metadata:
      name: sailoperator
      namespace: openshift-operators
    spec:
      channel: "1.27-nightly"
      installPlanApproval: Automatic
      name: sailoperator
      source: community-operators
      sourceNamespace: openshift-marketplace
EOF
```

2. Verify that the installation succeeded by inspecting the CSV status.

```sh
NAME                                      DISPLAY         VERSION                     REPLACES                                  PHASE
sailoperator.v1.27.0-nightly-2025-08-15   Sail Operator   1.27.0-nightly-2025-08-15   sailoperator.v1.27.0-nightly-2025-08-14   Succeeded
```

Succeeded should appear in the sailoperator CSV PHASE column.

3. Create the `istio-system` and `istio-cni` namespaces.

```sh
kubectl create namespace istio-system
kubectl create namespace istio-cni
```

4. Create the `Istio` resource with `spec.values.global.nativeNftables=true`:

```sh
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  version: v1.28-alpha.24646157
  namespace: istio-system
  updateStrategy:
    type: InPlace
    inactiveRevisionDeletionGracePeriodSeconds: 30
  values:
    global:
      nativeNftables: true
```

5. Create the `IstioCNI` resource with `spec.values.global.nativeNftables=true`:

```sh
kind: IstioCNI
apiVersion: sailoperator.io/v1
metadata:
  name: default
spec:
  namespace: istio-cni
  version: v1.28-alpha.24646157
  values:
    global:
      nativeNftables: true
```

#### Install in Ambient Mode

### Validation

When using the `nftables` backend, you can verify the traffic redirection rules using the `nft list ruleset` command in the `istio-proxy` container. When using the nftables backend, You get the table inet rules in the istio-proxy container. The following example installs a sample application `curl` in a data plane namespace `test-ns`.

```sh
kubectl create ns test-ns
kubectl label namespace test-ns istio-injection=enabled
kubectl apply -n test-ns -f https://raw.githubusercontent.com/istio/istio/refs/heads/master/samples/curl/curl.yaml
```

Attach a debug container and you will get the following rules in the `istio-proxy` container:

```sh
kubectl -n test-ns debug --image istio/base --profile netadmin --attach -t -i \
  "$(kubectl -n test-ns get pod -l app=curl -o jsonpath='{.items..metadata.name}')"
root@curl-6c88b89ddf-kbzn6:$ nft list ruleset

```

Verify the connectivity between two pods is working. For example, deploy a httpbin application using the following step:

```sh
kubectl apply -n test-ns -f samples/httpbin/httpbin.yaml
kubectl exec -n test-ns "$(kubectl get pod -l app=curl -n test-ns -o jsonpath={.items..metadata.name})" -c curl -n test-ns -- curl http://httpbin.test-ns:8000/ip -s -o /dev/null -w "%{http_code}\n"

200
```

More guidelines: [Debugging Guidelines](https://github.com/istio/istio/tree/master/tools/istio-nftables/pkg#debugging-guidelines)

### Upgrade

The migration of using the existing Istio iptables backend to nftables backend can be done by upgrading Istio. The following example installs an Istio control plane with the iptables backend and a sample application curl in a data plane namespace test-ns.

```sh
istioctl install -y

kubectl create ns test-ns
kubectl label namespace test-ns istio-injection=enabled
kubectl apply -n test-ns -f samples/curl/curl.yaml
```

You may create another Istio control plane with a revision value and gradually migrate data plane traffic to the new revision Istio control plane. This canary upgrade approach is much safer than doing an in-place upgrade. For example,

1. Install a canary version of Istio with the nftables backend.

```sh
istioctl install --set revision=canary --set values.global.nativeNftables=true -y
```

2. Upgrade the data plane and restart the deployment

```sh
kubectl label namespace test-ns istio-injection- istio.io/rev=canary
kubectl rollout restart deployment -n test-ns
```

3. Check the nftables backend running in the curl application pod. When using the nftables backend, You get the table inet rules in the istio-proxy container. For example, attach a debug container and run nft list ruleset command:

```sh
kubectl -n test-ns debug --image istio/base --profile netadmin --attach -t -i \
  "$(kubectl -n test-ns get pod -l app=curl -o jsonpath='{.items..metadata.name}')"

root@curl-6c88b89ddf-kbzn6:$ nft list ruleset
```

4. Verify the connectivity between two pods is working. For example, deploy a httpbin application using the following step:

```sh
kubectl apply -n test-ns -f samples/httpbin/httpbin.yaml
kubectl exec -n test-ns "$(kubectl get pod -l app=curl -n test-ns -o jsonpath={.items..metadata.name})" -c curl -n test-ns -- curl http://httpbin.test-ns:8000/ip -s -o /dev/null -w "%{http_code}\n"

200
```

5. After upgrading both the control plane and data plane, you can uninstall the old control plane in this example.

```sh
istioctl uninstall --revision default -y
```

When upgrading Istio with the CNI node agent, you can install a canary version of Istio control plane and upgrade the istio-cni node agent separately. For example, there is an Istio CNI component running in the istio-cni namespace, you can upgrade and enable the nftables backend using the following steps:

1. Install a canary version of Istiod control plane with the nftables backend.

```sh
helm install istiod-canary istio/istiod \
  --set revision=canary \
  --set values.global.nativeNftables=true \
  -n istio-system
```

2. Upgrade the CNI component separately from the revisioned control plane.

```sh
helm upgrade istio-cni istio/cni \
  --set values.global.nativeNftables=true \
  -n istio-cni --wait
```

#### Upgrade with Sail Operator on OpenShift

#### Upgrade in Ambient mode

