[Return to Project Root](../README.md)

# Table of Contents

- [Istio nftables backend](#istio-nftables-backend)
  - [Prerequisites](#prerequisites)
  - [Installation steps](#installation-steps)
  - [Validation](#validation)

## Istio nftables backend

This document outlines the configuration steps for the nftables backend in Istio. As the official successor to iptables, nftables offers a modern, high-performance alternative for transparently redirecting traffic to and from the Envoy sidecar proxy. Many major Linux distributions are actively moving towards adopting native nftables support. At present, this backend supports Istio sidecar mode only, with ambient mode support currently under development.

### Prerequisites

- **nftables version**: Requires `nft` binary version 1.0.1 or later.

### Installation Steps

The support for native nftables when using Istio sidecar mode was implemented in the upstream istio release-1.27 [release note](https://github.com/istio/istio/blob/master/releasenotes/notes/nftables-sidecar.yaml). It is disabled by default. To enable it, you can create an instance of the `Istio` resource with `spec.values.global.nativeNftables=true`:

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

### Validation

When using the `nftables` backend, you can verify the traffic redirection rules using the `nft list ruleset` command in the `istio-proxy` container. The following example installs a sample application `curl` in a data plane namespace `test-ns`.

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

table inet istio-proxy-nat {
	chain prerouting {
		type nat hook prerouting priority dstnat; policy accept;
		meta l4proto tcp jump istio-inbound
	}

	chain output {
		type nat hook output priority dstnat; policy accept;
		jump istio-output
	}

	chain istio-inbound {
		tcp dport 15008 return
		tcp dport 15090 return
		tcp dport 15021 return
		tcp dport 15020 return
		meta l4proto tcp jump istio-in-redirect
	}

	chain istio-redirect {
		meta l4proto tcp redirect to :15001
	}

	chain istio-in-redirect {
		meta l4proto tcp redirect to :15006
	}

	chain istio-output {
		oifname "lo" ip saddr 127.0.0.6 return
		oifname "lo" ip daddr != 127.0.0.1 tcp dport != 15008 meta skuid 1337 jump istio-in-redirect
		oifname "lo" meta skuid != 1337 return
		meta skuid 1337 return
		oifname "lo" ip daddr != 127.0.0.1 tcp dport != 15008 meta skgid 1337 jump istio-in-redirect
		oifname "lo" meta skgid != 1337 return
		meta skgid 1337 return
		ip daddr 127.0.0.1 return
		jump istio-redirect
	}
}
```

More guidelines: [Debugging Guidelines](https://github.com/istio/istio/tree/master/tools/istio-nftables/pkg#debugging-guidelines)
