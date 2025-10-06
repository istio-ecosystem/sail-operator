[Return to Project Root](../README.md)

# Table of Contents

- [Istio nftables backend](#istio-nftables-backend)
  - [Prerequisites](#prerequisites)
  - [Installation](#installation)
	  - [Install with Sail Operator on OpenShift](#install-with-sail-operator-on-openshift)
    - [Install in Ambient Mode](#install-in-ambient-mode)
  - [Validation](#validation)
  - [Upgrade](#Upgrade)
    - [Upgrade with Sail Operator on OpenShift](#upgrade-with-sail-operator-on-openshift)
    - [Upgrade in Ambient Mode](#upgrade-in-ambient-mode)


## Istio nftables backend

This document outlines the configuration steps for the nftables backend in Istio. As the official successor to iptables, nftables offers a modern, high-performance alternative for transparently redirecting traffic to and from the Envoy sidecar proxy. Many major Linux distributions are actively moving towards adopting native nftables support.
### Prerequisites

- **nftables version**: Requires `nft` binary version 1.0.1 or later.

### Installation

The support for native nftables in Istio sidecar mode was implemented in the upstream istio [release-1.27](https://github.com/istio/istio/blob/master/releasenotes/notes/nftables-sidecar.yaml). It is disabled by default. To enable it, you can set a feature flag as `values.global.nativeNftables=true`.

#### Install with Sail Operator on OpenShift

When you install IstioCNI and Istio resources with Sail Operator, you can create an instance of them with `spec.values.global.nativeNftables=true`. This feature configures Istio to use the `nftables` backend instead of `iptables` for traffic redirection.

To enable the native nftables feature, using the following steps:

1. Create the `istio-system` and `istio-cni` namespaces.

```sh
kubectl create namespace istio-system
kubectl create namespace istio-cni
```

2. Create the `IstioCNI` resource with `spec.values.global.nativeNftables=true`:

```sh
cat <<EOF | kubectl apply -f-
kind: IstioCNI
apiVersion: sailoperator.io/v1
metadata:
  name: default
spec:
  version: master
  namespace: istio-cni
  values:
    global:
      nativeNftables: true
EOF
```

3. Create the `Istio` resource with `spec.values.global.nativeNftables=true`:

```sh
cat <<EOF | kubectl apply -f-
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  version: master
  namespace: istio-system
  values:
    global:
      nativeNftables: true
EOF
```

#### Install in Ambient Mode

The support for native nftables in Istio ambient mode was implemented in the upstream istio `release-1.28`. To enable native nftables in ambient mode, you can set the same feature flag with ambient profile. For example,

1. Create the `istio-system`, `istio-cni` and `ztunnel` namespaces.

```sh
kubectl create namespace istio-system
kubectl create namespace istio-cni
kubectl create namespace ztunnel
```

2. Create the `IstioCNI` resource with `profile: ambient` and `spec.values.global.nativeNftables=true`:

```sh
cat <<EOF | kubectl apply -f-
apiVersion: sailoperator.io/v1
kind: IstioCNI
metadata:
  name: default
spec:
  version: master
  namespace: istio-cni
  profile: ambient
  values:
    global:
      nativeNftables: true
EOF
```

3. Create the `Istio` resource with `profile: ambient` and `spec.values.global.nativeNftables=true`:

```sh
cat <<EOF | kubectl apply -f-
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  version: master
  namespace: istio-system
  profile: ambient
  values:
    pilot:
      trustedZtunnelNamespace: ztunnel
    global:
      nativeNftables: true
EOF
```

4. Create the `ZTunnel` resource:

```sh
cat <<EOF | kubectl apply -f-
apiVersion: sailoperator.io/v1alpha1
kind: ZTunnel
metadata:
  name: default
spec:
  version: master
  namespace: ztunnel
  profile: ambient
EOF
```

### Validation

When using the `nftables` backend, you can verify the traffic redirection rules using the `nft list ruleset` command in a data plane application or sidecar container. You can find all rules are in the `inet` table. The following example installs a sample application `curl` in a data plane namespace `test-ns`.

```sh
kubectl create ns test-ns
```

Enable sidecar injection for the namespace `test-ns` when using sidecar mode:

```sh
kubectl label namespace test-ns istio-injection=enabled
```

As an alternative, enable ambient mode for the namespace `test-ns`:

```sh
kubectl label namespace test-ns istio.io/dataplane-mode=ambient
```

Deploy a sample application:

```sh
kubectl apply -n test-ns -f https://raw.githubusercontent.com/istio/istio/refs/heads/master/samples/curl/curl.yaml
```

Attach a debug container and you can see the nftable rules in the `inet` table:

```sh
kubectl -n test-ns debug --image istio/base --profile netadmin --attach -t -i \
  "$(kubectl -n test-ns get pod -l app=curl -o jsonpath='{.items..metadata.name}')"

root@curl-6c88b89ddf-kbzn6:$ nft list ruleset

```

Verify the connectivity between two pods is working. For example, deploy a httpbin application using the following step:

```sh
kubectl apply -n test-ns -f https://raw.githubusercontent.com/istio/istio/refs/heads/master/samples/httpbin/httpbin.yaml

kubectl exec -n test-ns "$(kubectl get pod -l app=curl -n test-ns -o jsonpath={.items..metadata.name})" -c curl -n test-ns -- curl http://httpbin.test-ns:8000/ip -s -o /dev/null -w "%{http_code}\n"

200
```

More guidelines: [Debugging Guidelines](https://github.com/istio/istio/tree/master/tools/istio-nftables/pkg#debugging-guidelines)

### Upgrade

The migration from iptables backend to nftables backend can be done by upgrading `Istio` and `IstioCNI` resources. Because the CNI component runs as a cluster singleton, it is recommended to operate and upgrade the CNI component seperately from the Istio control plane.

#### Upgrade with Sail Operator on OpenShift

To upgrade an iptable based Istio service mesh, using the following steps:

1. Check existing `Istio` and `IstioCNI` resources' state are Healthy.

2. Upgrade the IstioCNI and Istio control plane with `spec.values.global.nativeNftables=true`. More details about the Update Strategy are described in [update-strategy.adoc](../update-strategy/update-strategy.adoc). For example,

```sh
cat <<EOF | kubectl apply -f-
apiVersion: sailoperator.io/v1
kind: IstioCNI
apiVersion: sailoperator.io/v1
metadata:
  name: default
spec:
  version: master
  namespace: istio-cni
  values:
    global:
      nativeNftables: true
EOF
```

```sh
cat <<EOF | kubectl apply -f-
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  namespace: istio-system
  updateStrategy:
    type: InPlace
  version: master
  values:
    global:
      nativeNftables: true
EOF
```

3. Update the data plane namespace `test-ns` by restarting all deployments. For example,

```sh
kubectl rollout restart deployment -n test-ns
```

#### Upgrade in Ambient mode

To upgrade `IstioCNI` and `Istio` resources in Ambient mode, you can set the same feature flag with ambient profile. For example,

```sh
cat <<EOF | kubectl apply -f-
apiVersion: sailoperator.io/v1
kind: IstioCNI
apiVersion: sailoperator.io/v1
metadata:
  name: default
spec:
  version: master
  namespace: istio-cni
  profile: ambient
  values:
    global:
      nativeNftables: true
EOF
```

```sh
cat <<EOF | kubectl apply -f-
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  namespace: istio-system
  updateStrategy:
    type: InPlace
  version: master
  profile: ambient
  values:
    global:
      nativeNftables: true
EOF
```

You don't need to restart deployments in the data place namespace `test-ns`. Check the `ZTunnel` DaemonSet pods and verify there is no error. And then you can follow same steps in the [Validation](#validation) section to validate traffic redirection is working. 
