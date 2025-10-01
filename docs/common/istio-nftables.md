[Return to Project Root](../README.md)

# Table of Contents

- [Istio nftables backend](#istio-nftables-backend)
  - [Prerequisites](#prerequisites)
  - [Installation](#installation)
	  - [Install with Sail Operator on OpenShift](#install-with-sail-operator-on-openshift)
    - [Install in Ambient Mode](#install-in-ambient-mode)
  - [Validation](#validation)
  - [Upgrade with Sail Operator on OpenShift](#upgrade-with-sail-operator-on-openshift)
    - [Upgrade in Ambient Mode](#upgrade-in-ambient-mode)


## Istio nftables backend

This document outlines the configuration steps for the nftables backend in Istio. As the official successor to iptables, nftables offers a modern, high-performance alternative for transparently redirecting traffic to and from the Envoy sidecar proxy. Many major Linux distributions are actively moving towards adopting native nftables support.
### Prerequisites

- **nftables version**: Requires `nft` binary version 1.0.1 or later.

### Installation

The support for native nftables when using Istio sidecar mode was implemented in the upstream istio [release-1.27](https://github.com/istio/istio/blob/master/releasenotes/notes/nftables-sidecar.yaml). It is disabled by default. To enable it, you can set a feature flag as `values.global.nativeNftables=true`. For example,

Installation with Istioctl

```sh
istioctl install --set values.global.nativeNftables=true -y
```

Installation with Helm

```sh
helm install istiod istio/istiod \
  --set values.global.nativeNftables=true \
  -n istio-system
```

#### Install with Sail Operator on OpenShift

When you install an Istio resource with Sail Operator, you can create an instance of the `Istio` resource with `spec.values.global.nativeNftables=true`. This feature configures Istio to use the `nftables` backend instead of `iptables` for traffic redirection.

To enable the Istio native nftables feature, using the following steps:

1. Create the `istio-system` and `istio-cni` namespaces.

```sh
kubectl create namespace istio-system
kubectl create namespace istio-cni
```

2. Create the `Istio` resource with `spec.values.global.nativeNftables=true`:

```sh
cat <<EOF | kubectl apply -f-
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  version: v1.27.1
  namespace: istio-system
  values:
    global:
      nativeNftables: true
EOF
```

3. Create the `IstioCNI` resource with `spec.values.global.nativeNftables=true`:

```sh
cat <<EOF | kubectl apply -f-
kind: IstioCNI
apiVersion: sailoperator.io/v1
metadata:
  name: default
spec:
  version: v1.27.1
  namespace: istio-cni
  values:
    global:
      nativeNftables: true
EOF
```

#### Install in Ambient Mode

To enable native nftables in Ambient mode, you can set the same feature flag with ambient profile. For example,

Installation with Sail Operator on OpenShift

```sh
cat <<EOF | kubectl apply -f-
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  version: v1.28.0
  namespace: istio-system
  profile: ambient
  values:
    pilot:
      trustedZtunnelNamespace: ztunnel
    global:
      nativeNftables: true
EOF
```

```sh
cat <<EOF | kubectl apply -f-
apiVersion: sailoperator.io/v1
kind: IstioCNI
metadata:
  name: default
spec:
  version: v1.28.0
  namespace: istio-cni
  profile: ambient
values:
    pilot:
      trustedZtunnelNamespace: ztunnel
    global:
      nativeNftables: true
EOF
```

```sh
cat <<EOF | kubectl apply -f-
apiVersion: sailoperator.io/v1alpha1
kind: ZTunnel
metadata:
  name: default
spec:
  version: v1.28.0
  namespace: ztunnel
  profile: ambient
EOF
```

### Validation

When using the `nftables` backend, you can verify the traffic redirection rules using the `nft list ruleset` command in the `istio-proxy` container. When using the nftables backend, You get the table inet rules in the istio-proxy container. The following example installs a sample application `curl` in a data plane namespace `test-ns`.

```sh
kubectl create ns test-ns
kubectl label namespace test-ns istio-injection=enabled
kubectl apply -n test-ns -f https://raw.githubusercontent.com/istio/istio/refs/heads/master/samples/curl/curl.yaml
```

Attach a debug container and you can see the nftable rules from the `istio-proxy` container:

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

### Upgrade with Sail Operator on OpenShift

The migration from iptables backend to nftables backend can be done by upgrading Istio. The following example installs an Istio control plane with the iptables backend and a sample curl application in test-ns namespace.

1. Check existing `Istio` and `IstioCNI` resources' state is Healthy.

2. Create a data plane namespace and using curl application for sending traffic.

```sh
kubectl create ns test-ns
kubectl label namespace test-ns istio-injection=enabled
kubectl apply -n test-ns -f samples/curl/curl.yaml
```

3. Upgrade the Istio control plane with `spec.values.global.nativeNftables=true`. More details about the Update Strategy are described in [update-strategy.adoc](../update-strategy/update-strategy.adoc). For example,

```sh
cat <<EOF | kubectl apply -f-
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: istio-canary
spec:
  namespace: istio-system
  updateStrategy:
    type: RevisionBased
  version: v1.27.1
  values:
    global:
      nativeNftables: true
EOF
```

```sh
cat <<EOF | kubectl apply -f-
apiVersion: sailoperator.io/v1
kind: IstioCNI
apiVersion: sailoperator.io/v1
metadata:
  name: default
spec:
  version: v1.27.1
  namespace: istio-cni
  values:
    global:
      nativeNftables: true
EOF
```

4. Upgrade the data plane and restart the deployment

```sh
kubectl label namespace test-ns istio-injection- istio.io/rev=istio-canary-v1-27-1
kubectl rollout restart deployment -n test-ns
```

5. Check the nftables backend running in the curl application pod. When using the nftables backend, You get the table inet rules in the istio-proxy container. For example, attach a debug container and run nft list ruleset command:

```sh
kubectl -n test-ns debug --image istio/base --profile netadmin --attach -t -i \
  "$(kubectl -n test-ns get pod -l app=curl -o jsonpath='{.items..metadata.name}')"

root@curl-6c88b89ddf-kbzn6:$ nft list ruleset
```

6. Verify the connectivity between two pods is working. For example, deploy a httpbin application using the following step:

```sh
kubectl apply -n test-ns -f samples/httpbin/httpbin.yaml
kubectl exec -n test-ns "$(kubectl get pod -l app=curl -n test-ns -o jsonpath={.items..metadata.name})" -c curl -n test-ns -- curl http://httpbin.test-ns:8000/ip -s -o /dev/null -w "%{http_code}\n"

200
```

7. After upgrading both the control plane and data plane, you can uninstall the old control plane in this example.

#### Upgrade in Ambient mode

When upgrading Istio with canary upgrade in Ambient mode, because the CNI component runs as a cluster singleton, it is recommended to operate and upgrade the CNI component seperately from the revisioned control plane.

