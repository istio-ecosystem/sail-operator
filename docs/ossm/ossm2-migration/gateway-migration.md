# Gateway Migration Guide

Migrating gateways between Istio control planes during a version upgrade from 2.6 to 3.0 is very similar to migrating regular workloads, here is some information on how to migrate your gateways.

## Migration Scenarios

### Gateway Canary Migration (Recommended)

For gradual rollout using multiple gateway versions with maximum control over gateway rollout:

1. Label the gateway namespace to ensure injection from the new mesh is enabled for the namespace (this differs between multitenancy and cluster-wide meshes), ensuring to add the `maistra.io/ignore-namespace: "true"` label as well as remove `istio-injection=enabled` if needed.

> **_NOTE:_** Look back at the migrating workloads step in your migration guide to understand which labels you should use in your specific migration case and understand why the `maistra.io/ignore-namespace: "true"` label is needed. 

2. Deploy a canary gateway (example):
   ```yaml
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     name: istio-ingressgateway-canary
     namespace: istio-ingress # in the same namespace as your existing gateway
   spec:
     selector:
       matchLabels:
         istio: ingressgateway # must match your existing gateway service selector
     template:
       metadata:
         annotations:
           inject.istio.io/templates: gateway
         labels:
           istio: ingressgateway
           istio.io/rev: canary # Set to your 3.0 control plane revision
       spec:
         containers:
         - name: istio-proxy
           image: auto
   ```

3. Ensure that the new gateway deployment is running with new revision and is handling requests.
   - Check that pods are running and ready
   - Try `istioctl ps -n istio-ingress` to check that the gateway is running the new revision
   - Test a sample route through the gateway

4. Gradually shift traffic between deployments:
   ```bash
   # Increase replicas for new gateway
   oc scale -n istio-ingress deployment/<new_gateway_deployment> --replicas <new_number_of_replicas>
   
   # Decrease replicas for old gateway
   oc scale -n istio-ingress deployment/<old_gateway_deployment> --replicas <new_number_of_replicas>
   ```

   Repeat this process, incrementally adjusting replica counts until the new gateway handles all traffic to the gateway Service.

Note that this process is near identical to migrating from SMCP-Defined gateways to gateway injection, if the user migrated previously using [this guide](https://docs.redhat.com/en/documentation/openshift_container_platform/4.17/html/service_mesh/service-mesh-2-x#ossm-migrating-from-smcp-defined-gateways-to-gateway-injection_gateway-migration), this process should be familiar.

### Dedicated Application Gateway Migration (In Place)

If less fine grained control is needed over the gateway migration, the user can do the gateway migration in place.

For namespaces with dedicated gateways:

1. Label the gateway namespace to ensure injection from the new mesh is enabled for the namespace (this differs between multitenancy and cluster-wide meshes), ensuring to add the `maistra.io/ignore-namespace: "true"` label as well as remove `istio-injection=enabled` if needed. For example:
   ```bash
   oc label namespace ${APP_NAMESPACE} istio.io/rev=${ISTIO_REVISION} maistra.io/ignore-namespace="true"
   ```
> **_NOTE:_** Look back at the migrating workloads step in your migration guide to understand which labels you should use in your specific migration case and understand why the `maistra.io/ignore-namespace: "true"` label is needed.


2. Restart the gateway deployment:
   ```bash
   oc -n ${APP_NAMESPACE} rollout restart deployment ${GATEWAY_NAME}
   ```

3. Validation steps:
   - Try `istioctl ps -n istio-ingress` to check that the gateway is running the new revision
   - Test application-specific routes

### Shared Gateway Migration (In Place)

For environments using a centralized gateway shared across multiple namespaces (in this example `istio-ingress`):

1. Label the gateway namespace to ensure injection from the new mesh is enabled for the namespace (this differs between multitenancy and cluster-wide meshes), ensuring to add the `maistra.io/ignore-namespace: "true"` label as well as remove `istio-injection=enabled` if needed. For example:
   ```bash
   oc label namespace istio-ingress istio.io/rev=${ISTIO_REVISION} maistra.io/ignore-namespace="true"
   ```
> **_NOTE:_** Look back at the migrating workloads step in your migration guide to understand which labels you should use in your specific migration case and understand why the `maistra.io/ignore-namespace: "true"` label is needed.


2. Restart the gateway deployment:
   ```bash
   oc -n istio-ingress rollout restart deployment ${GATEWAY_NAME}
   ```
   
3. Validation steps:
   - Try `istioctl ps -n istio-ingress` to check that the gateway is running the new revision
   - Verify gateway pod is running with new revision
   - Test application-specific routes