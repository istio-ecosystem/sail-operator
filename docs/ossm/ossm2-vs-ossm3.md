# Compared to OpenShift Service Mesh 2

OpenShift Service Mesh 3 includes many important differences to be aware of coming from OpenShift Service Mesh 2.

OpenShift Service Mesh 3 is a major update with a feature set closer to the [Istio](https://istio.io/) project. It is based directly on Istio, rather than the midstream Maistra project that OpenShift Service Mesh 2 was based on. It is managed using a different, simplified operator and provides greater support for the latest stable features of Istio. This alignment with the Istio project along with lessons learned in the first two major releases of OpenShift Service Mesh have resulted in the following changes.

## From Maistra to Istio

While OpenShift Service Mesh 1 and 2 releases were based on Istio, they included additional functionality that was maintained as part of the midstream Maistra project, which itself was based on the upstream Istio project. Maistra maintained several features that were not part of the upstream Istio project. While this provided extra features to OpenShift Service Mesh users, the effort to maintain Maistra meant that OpenShift Service Mesh was usually several releases behind Istio with support omitted for major features like multi-cluster. Meanwhile, Istio has matured to cover most of the use cases addressed by Maistra. Basing OpenShift Service Mesh directly on Istio ensures that it will support users on the latest stable Istio features while Red Hat is able to contribute directly to the Istio community on behalf of its customers. 

## The OpenShift Service Mesh 3 operator

OpenShift Service Mesh 3 uses an operator that is maintained upstream as the Sail Operator in the istio-ecosystem organization. This operator is smaller in scope and includes significant changes from the operator used in OpenShift Service Mesh 2 that was maintained as part of the Maistra.io project.

## Observability integrations rather than addons

A significant change in OpenShift Service Mesh 3 compared to OpenShift Service Mesh 2 is that the operator no longer installs and manages observability components such as Prometheus and Grafana with the service mesh control plane. It also no longer installs and manages distributed tracing components such as Tempo and OpenTelemetry (Jaeger and Elasticsearch in the past) or the Kiali dashboard. 

The OpenShift Service Mesh 3 operator limits its scope to Istio related resources, with observability components supported and managed by the independent operators that make up [Red Hat OpenShift Observability](https://docs.redhat.com/en/documentation/openshift_container_platform/#Observability) such as [logging](https://docs.redhat.com/en/documentation/openshift_container_platform/4.17/html/logging/index), user workload [monitoring](https://docs.redhat.com/en/documentation/openshift_container_platform/4.17/html/monitoring/index), [distributed tracing](https://docs.redhat.com/en/documentation/openshift_container_platform/4.17/html/distributed_tracing/index). The [Kiali dashboard](https://docs.openshift.com/service-mesh/3.0.0tp1/kiali/ossm-kiali-assembly.html) along with OpenShift Service Mesh Console will continue to be supported with the Kiali operator.

This simplification greatly reduces the footprint and complexity of OpenShift Service Mesh, while providing better, production-grade support for observability through Red Hat OpenShift Observability.

## The `Istio` resource replaces the `ServiceMeshControlPlane` resource

While OpenShift Service Mesh 2 used a resource called `ServiceMeshControlPlane` to configure Istio, OpenShift Service Mesh 3 uses a resource called `Istio`. 

The `Istio` resource contains a `spec.values` field that derives its schema from Istio’s Helm chart values. While this is a different configuration schema than `ServiceMeshControlPlane` uses, the fact that it is derived from Istio’s configuration means that configuration examples from the community Istio documentation can often be applied directly to Red Hat OpenShift Service Mesh’s `Istio` resource. The `spec.values` field in the `IstioOperator` resource (which is not part of OpenShift Service Mesh) has a similar format. The `Istio` resource provides an additional validation schema enabling the ability to explore the resource using the OpenShift CLI command `oc explain istios.spec.values`.

## New resource: `IstioCNI`

The Istio CNI node agent is used to configure traffic redirection for pods in the mesh. It runs as a DaemonSet, on every node, with elevated privileges. 

In OpenShift Service Mesh 2, the operator deployed an Istio CNI instance for each minor version of Istio present in the cluster and pods were automatically annotated during sidecar injection, such that they picked up the correct Istio CNI. While this meant that the management of Istio CNI was mostly hidden from users, it obscured the fact that the Istio CNI agent has a lifecycle that is independent of the Istio control plane and in some cases, must be upgraded separately.

For these reasons, the OpenShift Service Mesh 3 operator manages Istio CNI with a separate resource called `IstioCNI`. A single instance of this resource is shared by all Istio control planes (managed by `Istio` resources). The `IstioCNI` version 1.x is compatible with control plane version 1.x an 1.x+1, meaning that the control planes must be upgraded before Istio CNI, with their version difference keeping within one minor version.

## Scoping of the Mesh: Discovery Selectors and labels replace `ServiceMeshMemberRoll` and `ServiceMeshMember`

OpenShift Service Mesh 2 used the two resources `ServiceMeshMemberRoll` and `ServiceMeshMember` to indicate which namespaces were to be included in the mesh. When a mesh was created, it would only be scoped to the namespaces listed in the `ServiceMeshMemberRoll` or containing a `ServiceMeshMember` instance. This made it simple to include multiple service meshes in a cluster with each mesh tightly scoped, referred to as a “multitenant” configuration. 

In OpenShift Service Mesh 2.4, a “cluster-wide” mode was introduced to allow a mesh to be cluster-scoped, with the option to limit the mesh using an Istio feature called `discoverySelectors`, which limits the Istio control plane's visibility to a set of namespaces defined with a [label selector](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/). This is aligned with how community Istio worked, and allowed Istio to manage cluster-level resources.

OpenShift Service Mesh 3 converges with Istio in making all meshes “cluster-wide”. This means that Istio control planes are all cluster-scoped resources and the resources `ServiceMeshMemberRoll` and `ServiceMeshMember` are no longer present, with control planes watching ("discovering") the entire cluster by default. The control plane's discovery of namespaces can be limited using the `discoverySelectors` feature. 

Note that even though the Istio control plane discovers a namespace, the workloads present in that namespace still require sidecar proxies to be included as workloads in the service mesh, and to be able to use Istio's many features. Changes to sidecar injection - the process of adding a sidecar proxy container to an existing pod, are described below. 

## Sidecar Injection: New Considerations

OpenShift Service Mesh 2 supported using pod annotations and labels to configure sidecar injection and there was no need to indicate which control plane a workload belonged to.

Sidecar injection in OpenShift Service Mesh 3 works the same way as it does for Istio - with pod or namespace labels used to trigger sidecar injection and it may be necessary to include a label that indicates which control plane the workload belongs to. Note that Istio has deprecated pod annotations in favor of labels for sidecar injection.

When an `Istio` resource has the name “default” and `InPlace` upgrades are used (as opposed to `RevisionBased` described below), there will be a single `IstioRevision` with the name "default" and the label `istio-injection=enabled` may be used for injection.

However, when an `IstioRevision` resource has a name other than “default” - as required when multiple control plane instances are present and/or a canary-style control plane upgrade is in progress, it is necessary to use a label that indicates which control plane (revision) the workload(s) belong to - namely, `istio.io/rev=<IstioRevision-name>`. These labels may be applied at the workload or namespace level. Available revisions may be inspected with the command `oc get istiorevision`. 

## Multiple Control Plane Support

OpenShift Service Mesh 3 supports multiple service meshes in the same cluster, but in a different manner than in OpenShift Service Mesh 2. A cluster administrator must create multiple `Istio` instances and then configure `discoverySelectors` appropriately to ensure that there is no overlap between mesh namespaces. 

As `Istio` resources are cluster-scoped, they must have unique names to represent unique meshes within the same cluster. The OpenShift Service Mesh 3 operator uses this unique name to create a resource called `IstioRevision` with a name in the format of `{Istio name}` or `{Istio name}-{Istio version}`. Each instance of `IstioRevision` is responsible for managing a single control plane. Workloads are assigned to a specific control plane using Istio's revision labels of the format `istio.io/rev={IstioRevision name}`. The name with the version identifier becomes important to support canary-style control plane upgrades (more on this in the upgrades section below).

## Independently Managed Gateways

In Istio, gateways are used to manage traffic entering (ingress) and exiting (egress) the mesh. While by default, OpenShift Service Mesh 2 deployed and managed an Ingress Gateway and an Egress Gateway with the service mesh control plane, configured in the `ServiceMeshControlPlane` resource, the OpenShift Service Mesh 3 operator will no longer create or manage gateways. 

In OpenShift Service Mesh 3, gateways are created and managed independent of the operator and control plane using gateway injection or Kubernetes Gateway API. This provides much greater flexibility than was possible with the `ServiceMeshControlPlane` resource and ensures that gateways can be fully customized and managed as part of a GitOps pipeline. This allows the gateways deployed and managed alongside their applications with the same lifecycle.

This change was made because, as a good practice, gateways are better managed together with their corresponding workloads than with the service mesh control plane. This change also means starting with a gateway configuration that can expand over time to meet the more robust needs of a production environment, which was not possible with the default gateways in OpenShift Service Mesh 2. 

Note that even with this change, gateways may continue to be deployed onto nodes or namespaces independent of applications (for example, a centralized gateway node).  Istio gateways also remain eligible to be deployed on OpenShift infrastructure nodes.

## OpenShift Routes must be explicitly created

An OpenShift `Route` resource allows an application to be exposed with a public URL using OpenShift's Ingress operator for managing HAProxy based Ingress controllers. OpenShift Service Mesh 2 included a feature called Istio OpenShift Routing (IOR) that automatically created and managed OpenShift routes for Istio gateways. While this was convenient, as the operator would manage these routes for the user, it often caused confusion around ownership as many `Route` resources are managed by administrators. The feature also lacked the configurability of an independent `Route` resource, created unnecessary routes, and exhibited unpredictable behavior during upgrades.

Thus, in OpenShift Service Mesh 3, when a `Route` is desired to expose an Istio gateway, it must be created and managed by the user. Note that it is also possible to expose an Istio gateway through a Kubernetes service of type LoadBalancer if a route is not desired.

## Introducing Canary Upgrades

OpenShift Service Mesh 2 supported only one approach for upgrades - an in-place style upgrade, where the control plane was upgraded, then all gateways and workloads needed to be restarted for the proxies could be upgraded. While this is a simple approach, it can create risk for large meshes where once the control plane was upgraded, all workloads must upgrade to the new control plane version without a simple way to roll back if something goes wrong.

While OpenShift Service Mesh 3 retains support for simple in-place style upgrades, it adds support for canary-style upgrades of the service mesh control plane using Istio’s revision feature. This is supported by the `Istio` resource which manages Istio revision labels using the `IstioRevision` resource. When the `Istio` resource's `updateStrategy` is set to type `RevisionBased`, it will create Istio revision labels using the `Istio` resource's name combined with the Istio version (e.g. “mymesh-v1-21-2”). During an upgrade, a new `IstioRevision` will deploy the new control plane with an updated revision label (e.g. “mymesh-v1-22-0”). Workloads may then be migrated between control planes using the revision label on namespaces or workloads (e.g. “istio.io/rev=mymesh-v1-22-0”).

## Multi-cluster Topologies

OpenShift Service Mesh 2 supported one form of multi-cluster: federation, which was introduced in version 2.1. In this topology, each cluster maintains its own independent control plane, with services only shared between those meshes on an as-needed basis. Communication between federated meshes is entirely through Istio gateways, meaning that there was no need for service mesh control planes to watch remote Kubernetes control planes, as is the case with Istio's multi-cluster service mesh topologies. Federation is ideal where service meshes are loosely coupled - managed by different administrative teams.

OpenShift Service Mesh 3 includes support for Istio's multi-cluster topologies, namely: Multi-Primary, Primary-Remote and external control planes. These topologies effectively stretch a single unified service mesh across multiple clusters. This is ideal when all clusters involved are managed by the same administrative team. Istio's multi-cluster topologies are ideal for implementing high-availability or failover use cases across a commonly managed set of applications. 

## Istioctl

OpenShift Service Mesh 1 and 2 did not include support for Istioctl, the command line utility for the Istio project that includes many diagnostic and debugging utilties. OpenShift Service Mesh 3 introduces support for Istioctl for select commands. Installation and management of Istio will only be supported by the OpenShift Service Mesh 3 operator.

## Kubernetes Network Policy Management

By default, OpenShift Service Mesh 2 created Kubernetes `NetworkPolicy` resources that:
1. Ensured network applications and the control plane can communicate with each other.
2. Restricts ingress for mesh applications to only member projects.

OpenShift Service Mesh 3 does not create these policies, leaving it to the user to configure the level of isolation required for their environment. Istio provides fine grained access control of service mesh workloads through [Authorization Policies](https://istio.io/latest/docs/reference/config/security/authorization-policy/).  