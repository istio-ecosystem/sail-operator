[Return to Project Root](../../README.md)

# Table of Contents

- [Dual-stack Support](#dual-stack-support)
  - [Prerequisites](#prerequisites)
  - [Installation Steps](#installation-steps)
  - [Validation](#validation)
  - [Cleanup](#cleanup)

## Dual-stack Support

Kubernetes supports dual-stack networking as a stable feature starting from
[v1.23](https://kubernetes.io/docs/concepts/services-networking/dual-stack/), allowing clusters to handle both
IPv4 and IPv6 traffic. With many cloud providers also beginning to offer dual-stack Kubernetes clusters, it's easier
than ever to run services that function across both address types. Istio introduced dual-stack as an experimental
feature in version 1.17, and promoted it to [Alpha](https://istio.io/latest/news/releases/1.24.x/announcing-1.24/change-notes/) in
version 1.24. With Istio in dual-stack mode, services can communicate over both IPv4 and IPv6 endpoints, which helps
organizations transition to IPv6 while still maintaining compatibility with their existing IPv4 infrastructure.

When Kubernetes is configured for dual-stack, it automatically assigns an IPv4 and an IPv6 address to each pod,
enabling them to communicate over both IP families. For services, however, you can control how they behave using
the `ipFamilyPolicy` setting.

Service.Spec.ipFamilyPolicy can take the following values
- SingleStack: Only one IP family is configured for the service, which can be either IPv4 or IPv6.
- PreferDualStack: Both IPv4 and IPv6 cluster IPs are assigned to the Service when dual-stack is enabled.
                   However, if dual-stack is not enabled or supported, it falls back to singleStack behavior.
- RequireDualStack: The service will be created only if both IPv4 and IPv6 addresses can be assigned.

This allows you to specify the type of service, providing flexibility in managing your network configuration.
For more details, you can refer to the Kubernetes [documentation](https://kubernetes.io/docs/concepts/services-networking/dual-stack/#services).

### Prerequisites

- Kubernetes 1.23 or later configured with dual-stack support.
- Sail Operator is installed.

### Installation Steps

You can use any existing Kind cluster that supports dual-stack networking or, alternatively, install one using the following command.

```bash
kind create cluster --name istio-ds --config - <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
 ipFamily: dual
EOF
```

Note: If you installed the KinD cluster using the command above, install the [Sail Operator](../../docs/general/getting-started.md#getting-started) before proceeding with the next steps.

1. Create the `Istio` resource with dual-stack configuration.

   ```bash { name=create-istio-dual-stack tag=dual-stack }
   kubectl get ns istio-system || kubectl create namespace istio-system
   kubectl apply -f - <<EOF
   apiVersion: sailoperator.io/v1
   kind: Istio
   metadata:
     name: default
   spec:
     values:
       meshConfig:
         defaultConfig:
           proxyMetadata:
             ISTIO_DUAL_STACK: "true"
       pilot:
         ipFamilyPolicy: RequireDualStack
         env:
           ISTIO_DUAL_STACK: "true"
     namespace: istio-system
   EOF
   kubectl wait --for=jsonpath='{.status.revisions.ready}'=1 istios/default --timeout=3m
   ```

2. If running on OpenShift platform, create the IstioCNI resource as well.

   ```bash
   kubectl get ns istio-cni || kubectl create namespace istio-cni
   kubectl apply -f - <<EOF
   apiVersion: sailoperator.io/v1
   kind: IstioCNI
   metadata:
     name: default
   spec:
     namespace: istio-cni
   EOF
   kubectl wait --for=condition=Ready pod -n istio-cni -l k8s-app=istio-cni-node --timeout=60s
   ```

### Validation

1. Create the following namespaces, each hosting the tcp-echo service with the specified configuration.

   - dual-stack: which includes a tcp-echo service that listens on both IPv4 and IPv6 address.
   - ipv4: which includes a tcp-echo service listening only on IPv4 address.
   - ipv6: which includes a tcp-echo service listening only on IPv6 address.

   ```bash { name=create-namespaces tag=dual-stack }
   kubectl get ns dual-stack || kubectl create namespace dual-stack
   kubectl get ns ipv4 || kubectl create namespace ipv4
   kubectl get ns ipv6 ||  kubectl create namespace ipv6
   kubectl get ns sleep || kubectl create namespace sleep
   ```

2. Label the namespaces for sidecar injection.
   ```bash { name=label-namespaces tag=dual-stack }
   kubectl label --overwrite namespace dual-stack istio-injection=enabled
   kubectl label --overwrite namespace ipv4 istio-injection=enabled
   kubectl label --overwrite namespace ipv6 istio-injection=enabled
   kubectl label --overwrite namespace sleep istio-injection=enabled
   ```

3. Deploy the pods and services in their respective namespaces.
   ```bash { name=deploy-pods-and-services tag=dual-stack }
   kubectl apply -n dual-stack -f https://raw.githubusercontent.com/istio/istio/release-1.23/samples/tcp-echo/tcp-echo-dual-stack.yaml
   kubectl apply -n ipv4 -f https://raw.githubusercontent.com/istio/istio/release-1.23/samples/tcp-echo/tcp-echo-ipv4.yaml
   kubectl apply -n ipv6 -f https://raw.githubusercontent.com/istio/istio/release-1.23/samples/tcp-echo/tcp-echo-ipv6.yaml
   kubectl apply -n sleep -f https://raw.githubusercontent.com/istio/istio/release-1.23/samples/sleep/sleep.yaml
   kubectl wait --for=condition=Ready pod -n sleep -l app=sleep --timeout=60s
   kubectl wait --for=condition=Ready pod -n dual-stack -l app=tcp-echo --timeout=60s
   kubectl wait --for=condition=Ready pod -n ipv4 -l app=tcp-echo --timeout=60s
   kubectl wait --for=condition=Ready pod -n ipv6 -l app=tcp-echo --timeout=60s
   ```

4. Ensure that the tcp-echo service in the dual-stack namespace is configured with `ipFamilyPolicy` of RequireDualStack.
   ```console
   kubectl get service tcp-echo -n dual-stack -o=jsonpath='{.spec.ipFamilyPolicy}'
   RequireDualStack
   ```
<!-- ```bash { name=validation-ipfamilypolicy tag=dual-stack}
    response=$(kubectl get service tcp-echo -n dual-stack -o=jsonpath='{.spec.ipFamilyPolicy}')
    echo $response
    if [ "$response" = "RequireDualStack" ]; then
        echo "ipFamilyPolicy is set to RequireDualStack as expected"
    else
        echo "ipFamilyPolicy is not set to RequireDualStack"
        exit 1
    fi
``` -->
5. Verify that sleep pod is able to reach the dual-stack pods.
   ```console
   kubectl exec -n sleep "$(kubectl get pod -n sleep -l app=sleep -o jsonpath='{.items[0].metadata.name}')" -- sh -c "echo dualstack | nc tcp-echo.dual-stack 9000"
   hello dualstack
   ```
<!-- ```bash { name=validation-sleep-reach-dual-stack tag=dual-stack}
    response=$(kubectl exec -n sleep "$(kubectl get pod -n sleep -l app=sleep -o jsonpath='{.items[0].metadata.name}')" -- sh -c "echo dualstack | nc tcp-echo.dual-stack 9000")
    echo $response
    if [ "$response" = "hello dualstack" ]; then
        echo "Sleep can reach tcp-echo.dual-stack pod as expected"
    else
        echo "tcp-echo.dual-stack pod is not reachable from sleep pod"
        exit 1
    fi
``` -->
6. Similarly verify that sleep pod is able to reach both ipv4 pods as well as ipv6 pods.
   ```console
   kubectl exec -n sleep "$(kubectl get pod -n sleep -l app=sleep -o jsonpath='{.items[0].metadata.name}')" -- sh -c "echo ipv4 | nc tcp-echo.ipv4 9000"
   hello ipv4
   ```
<!-- ```bash { name=validation-sleep-reach-ipv4-pod tag=dual-stack}
    response=$(kubectl exec -n sleep "$(kubectl get pod -n sleep -l app=sleep -o jsonpath='{.items[0].metadata.name}')" -- sh -c "echo ipv4 | nc tcp-echo.ipv4 9000")
    echo $response
    if [ "$response" = "hello ipv4" ]; then
        echo "Sleep can reach tcp-echo.ipv4 pod as expected"
    else
        echo "tcp-echo.ipv4 pod is not reachable from sleep pod"
        exit 1
    fi
``` -->
   ```console
   kubectl exec -n sleep "$(kubectl get pod -n sleep -l app=sleep -o jsonpath='{.items[0].metadata.name}')" -- sh -c "echo ipv6 | nc tcp-echo.ipv6 9000"
   hello ipv6
   ```
<!-- ```bash { name=validation-sleep-reach-ipv4-pod tag=dual-stack}
    response=$(kubectl exec -n sleep "$(kubectl get pod -n sleep -l app=sleep -o jsonpath='{.items[0].metadata.name}')" -- sh -c "echo ipv6 | nc tcp-echo.ipv6 9000")
    echo $response
    if [ "$response" = "hello ipv6" ]; then
        echo "Sleep can reach tcp-echo.ipv6 pod as expected"
    else
        echo "tcp-echo.ipv6 pod is not reachable from sleep pod"
        exit 1
    fi
``` -->
### Cleanup
To clean up the resources created during this example, you can run the following commands:
   ```bash
   kubectl delete istios default
   kubectl delete ns istio-system
   kubectl delete istiocni default
   kubectl delete ns istio-cni
   kubectl delete ns dual-stack ipv4 ipv6 sleep
   ```
