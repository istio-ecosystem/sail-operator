# OpenShift Service Mesh 2.6 Network Policy migration to 3.0

In OpenShift Service Mesh 2.6, Network Policies are created by default when `spec.security.manageNetworkPolicy=true` in the ServiceMeshControlPlane config. During migration to Service Mesh 3.0, these Network Policies will be removed and will need to be recreated manually if you wish to maintain identical NetworkPolicies.

## Network Policies created by 2.6:

When `spec.security.manageNetworkPolicy=true`, the following Network Policies are created:

### 1. Istiod Network Policy
- **Purpose**: Controls incoming traffic to the webhook port of istiod pod(s)
- **Location**: Created in SMCP namespace
- **Sample YAML**:
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: istio-istiod-basic      # Name format: istio-istiod-<revision>
  namespace: istio-system       # Your SMCP namespace
  labels:
    maistra-version: "2.6.5"    # Version label
    app: istiod                 # Identifies istiod component
    istio: pilot                # Identifies as Istio pilot component
    istio.io/rev: basic         # Revision identifier
    release: istio              
  annotations:
    "maistra.io/internal": "true"
spec:
  ingress:
    - {}
  podSelector:
    matchLabels:
      app: istiod
      istio.io/rev: basic
  policyTypes:
    - Ingress
 ```
> **_NOTE:_** When recreating this Network Policy, users should take note that the `istio.io/rev:` label value will change during the migration process and should update their `matchLabels` values accordingly.

### 2. Expose Route Network Policy
- **Purpose**: Allows traffic from OpenShift ingress namespaces to pods labeled with `maistra.io/expose-route: "true"`
- **Location**: Created in both:
    - SMCP namespace
    - Any namespace where ServiceMeshMember (SMM) is created
- **Sample YAML**:
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: istio-expose-route-basic     # Name format: istio-expose-route-<revision>
  namespace: istio-system            # Your SMCP or member namespace
  labels:
    maistra-version: "2.6.5"
    app: istio
    release: istio
spec: 
  podSelector:
    matchLabels:
      maistra.io/expose-route: "true"
  ingress:
  - from:
    - namespaceSelector:    # Allows traffic from OpenShift ingress
        matchLabels:
          network.openshift.io/policy-group: ingress
  policyTypes:
    - Ingress
```

### 3. Default Mesh Network Policy
- **Purpose**: Restricts traffic to pods only from namespaces explicitly labeled as part of the mesh (using label `maistra.io/member-of: <mesh-namespace>`)
- **Location**: Created in both:
    - SMCP namespace
    - Any namespace where ServiceMeshMember is created
- **Sample YAML**:
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: istio-mesh-basic          # Name format: istio-mesh-<revision>
  namespace: istio-system         # Your SMCP or member namespace
  labels:
    maistra-version: "2.6.5"
    app: istio
    release: istio
spec:
  ingress:
  - from:
    - namespaceSelector:          # Only allows traffic from mesh members
        matchLabels:
          maistra.io/member-of: istio-system
  podSelector: {}
  policyTypes:
    - Ingress

```

> **_NOTE:_** When recreating this Network Policy, the label `maistra.io/member-of: $SMCP_NS` must be replaced with a new label in the Network Policy and mesh namespaces labeled with the new label. This is because the aforementioned label was created automatically on namespaces by the SMCP. After migration is complete and the 2.6 mesh is removed, this label will be removed. 

### 4. Ingress Gateway Network Policy
- **Purpose**: Allows inbound traffic from any source to ingress gateway pods
- **Note**: This is only created when the ingress gateway is created through SMCP spec (not through gateway injection).
    So if you have followed the other steps in the checklist, this will not exist.
- **Location**: Created in SMCP namespace
- **Sample YAML**:
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: istio-ingressgateway             # Gateway name
  namespace: istio-system               
  labels:
    maistra-version: "2.6.5"
    app: istio-ingressgateway            # Gateway app label
    istio: ingressgateway                # Gateway component label
  annotations:
    "maistra.io/internal": "true"
spec:
  podSelector:                           # Targets ingress gateway pods
    matchLabels:
      app: istio-ingressgateway
      istio: ingressgateway
  ingress:
    - {}                                 # Empty rule allows all ingress traffic
  policyTypes:
    - Ingress
```

## How to Migrate Network Policies to 3.0

### Migration Steps

#### For SMCP Namespace
Recreate necessary Network Policies in the new Service Mesh 3.0 control plane namespace:
- Istiod Network Policy
- Default Mesh Network Policy
- Expose Route Network Policy
- Ingress Gateway Network Policy (if you were previously using SMCP-created gateways)

#### For Workload Namespaces
For each namespace that was part of the 2.6 mesh:

1. Recreate the following Network Policies:
    - Default Mesh Network Policy
    - Expose Route Network Policy

2. Update labels:
    - Consider replacing the `maistra.io/expose-route: "true"` label with a new label scheme
    - Update corresponding Network Policy selectors to match new labels

### Important Notes
- Simply removing ownerReferences from existing Network Policies won't prevent their deletion during migration.
- When ServiceMeshMember is removed from a namespace, the `maistra.io/member-of` label is automatically removed from the namespace.
- Duplicate NetworkPolicies in the same namespace should not cause issues.

### Best Practices
1. Test Network Policies in a non-production environment first.
2. If possible, create Network Policies after migrating workloads to ensure continuous protection. If it is not possible due to security policies to migrate workloads without Network Policies, see below for tips.

### Example Network Policies with both 2.6 and 3.0 present

Some users may not be able to go without Network Policies during the migration process due to security concerns. 
This can be tricky, as during the migration both control planes must have access to all workloads and vice versa. Created Network Policies must not block traffic for either control plane.

To assist users in this situation, here is an example scenario with example Network Policies.

1. **Label  Namespaces**

    Before creating Network Policies, you need to choose and apply appropriate namespace labels. While you could use a generic label like `service-mesh: enabled`, we recommend using a label scoped specifically to your mesh that can be reused for discovery selectors. 
    
    This approach:
    
    - Avoids additional namespace labeling requirements
    - Provides better isolation between different service meshes
    - Can be reused in discovery selectors
    
    This step is necessary because the label `maistra.io/member-of: istio-system` will be removed from the namespaces during migration.
    For more details on label selection, see the [discoverySelector documentation](./../create-mesh/README.md).

    For simplicity, in our scenario the user labels the mesh namespaces with `service-mesh: enabled`.

2. **Create Network Policies**

    A user has a 2.6 control plane with the name `basic` in the namespace `istio-system` and workloads in namespaces `httpbin-2` and `httbin-3`. They need to ensure Network Policies remain active to fulfill security obligations. 
    
   In order to keep Network Policies, before they disable `manageNetworkPolicy` they create Policies:
    
    Istiod Network Policy in Mesh namespace
    ```yaml
    apiVersion: networking.k8s.io/v1
    kind: NetworkPolicy
    metadata:
      name: istiod-basic
      namespace: istio-system
    spec:
      ingress:
        - {}
      podSelector:
        matchLabels:
          app: istiod
          istio.io/rev: basic
      policyTypes:
        - Ingress
    ```
    
    Expose Route Policy in Mesh namespace
    
    ```yaml
    apiVersion: networking.k8s.io/v1
    kind: NetworkPolicy
    metadata:
      name: expose-route-basic
      namespace: istio-system
    spec:
      podSelector:
        matchLabels:
          maistra.io/expose-route: "true"
      ingress:
        - from:
            - namespaceSelector:
                matchLabels:
                  network.openshift.io/policy-group: ingress
      policyTypes:
        - Ingress
    ```
    
    Default Mesh Network Policy in Mesh namespace
    
    ```yaml
    apiVersion: networking.k8s.io/v1
    kind: NetworkPolicy
    metadata:
      name: istio-mesh
      namespace: istio-system
    spec:
      ingress:
        - from:
            - namespaceSelector:
                matchLabels:
                  service-mesh: enabled
      podSelector: {}
      policyTypes:
        - Ingress
    ```
    
    Expose Route Network Policy for httpbin-2 namespace
    ```yaml
    apiVersion: networking.k8s.io/v1
    kind: NetworkPolicy
    metadata:
      name: istio-expose-route
      namespace: httpbin-2
    spec:
      podSelector:
        matchLabels:
          maistra.io/expose-route: "true"
      ingress:
        - from:
            - namespaceSelector:
                matchLabels:
                  network.openshift.io/policy-group: ingress
      policyTypes:
        - Ingress
    ```
    
    Mesh Network Policy for httpbin-2 namespace
    ```yaml
    apiVersion: networking.k8s.io/v1
    kind: NetworkPolicy
    metadata:
      name: istio-mesh
      namespace: httpbin-2
    spec:
      ingress:
        - from:
            - namespaceSelector:
                matchLabels:
                  service-mesh: enabled
      podSelector: {}
      policyTypes:
        - Ingress
    ```
    
    Expose Route Network Policy for httpbin-3 namespace
    ```yaml
    apiVersion: networking.k8s.io/v1
    kind: NetworkPolicy
    metadata:
      name: istio-expose-route
      namespace: httpbin-3
    spec:
      podSelector:
        matchLabels:
          maistra.io/expose-route: "true"
      ingress:
        - from:
            - namespaceSelector:
                matchLabels:
                  network.openshift.io/policy-group: ingress
      policyTypes:
        - Ingress
    ```
    
    Mesh Network Policy for httpbin-3 namespace
    ```yaml
    apiVersion: networking.k8s.io/v1
    kind: NetworkPolicy
    metadata:
      name: istio-mesh
      namespace: httpbin-3
    spec:
      ingress:
        - from:
            - namespaceSelector:
                matchLabels:
                  service-mesh: enabled
      podSelector: {}
      policyTypes:
        - Ingress
    ```

3. **Disable manageNetworkPolicy**

   After creating these policies, the user can safely disable the SMCP `manageNetworkPolicy`.

4. **Prepare for 3.0 Mesh Creation**

    Now the user is about to create the 3.0 mesh which will be named `v3` and created in `istio-system`.
    
    Before creating the `istio` resource and the istiod pod, the user makes:
    
    Istiod Network Policy for 3.0:
    ```yaml
    apiVersion: networking.k8s.io/v1
    kind: NetworkPolicy
    metadata:
      name: istio-istiod-v3
      namespace: istio-system
    spec:
      ingress:
        - {}
      podSelector:
        matchLabels:
          app: istiod
          istio.io/rev: v3
      policyTypes:
        - Ingress
    ```
    
    to ensure that they are consistent in Network Policies with the 3.0 mesh before continuing on with the migration steps. 

