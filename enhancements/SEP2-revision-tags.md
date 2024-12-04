|Status                                             | Authors      | Created    | 
|---------------------------------------------------|--------------|------------|
|Implementation                                     | @dgn         | 2024-07-17 |

# Revision Tag Support

## Overview
Upstream Istio supports the use of [stable revision tags](https://istio.io/latest/blog/2021/revision-tags/) for multi-revision deployments and canary upgrades of Istio control planes. These tags serve as aliases for revisions and allow users to use stable revision names (e.g. `prod` or `default`), so they don't have to change their namespace and pod labels (in this case `istio.io/rev=prod` or `istio-injection=enabled`) when switching to a new version. Instead, by tagging a new revision with the correct tag and restarting their workloads, they can perform an Istio update without having to change their labels. This is especially useful in situations where the team managing the Istio control plane is separate from the teams managing the workloads.

Revision tags can have any name, there is only one special case: revisions tagged `default` are treated as if they had an empty revision name, thereby allowing the use of the standard namespace injection label `istio-injection=enabled`.

Each revision tag only ever points to exactly one Istio revision. Upstream, revision tags are created manually using `istioctl` and- as they only affect injection- are represented in the cluster by a MutatingWebhookConfiguration.

Here's an example how to create a `default` revision tag that points to the `1-21-1` revision using `istioctl`:

```bash
istioctl tag set default --revision 1-21-1
```

## Goals
* Provide revision tag support in Sail Operator APIs so users don't have to use istioctl for basic revision tag operations

## Non-goals
* Compatibility with manual revision tag creation using istioctl. There might be a way to disable the operator functionality to avoid conflicts when creating revision tags manually, but that's it - you either do it yourself or let the operator do it

## Design

### User Stories
1. As a user of Sail Operator's RevisionBased update strategy, I want to be able to use the `istio-injection=enabled` label on my namespaces.
1. As a platform engineer, I want my application teams to be able to use a fixed label for proxy injection without having to know which version of Istio is running in the cluster, so that I can perform upgrades in the background without the application teams having to make changes to use the new version.

### API Changes
We will add a new CRD called `IstioRevisionTag` that consistly most of a `spec.targetRef` field and a `status` subresource.

#### IstioRevisionTag resource
Here's an example YAML for the new resource:
```yaml
apiGroup: sailoperator.io/v1alpha1
kind: IstioRevisionTag
metadata:
  name: default
spec:
  targetRef:
    kind: Istio # can also be IstioRevision
    name: default
status:
  observedGeneration: 1
  conditions: []
  state: Healthy
  istiodNamespace: istio-system
  istioRevision: default-v1.24.0
```

In the `spec.targetRef` field, users can specify the `IstioRevision` or `Istio` resource that the `IstioRevisionTag` references. In case of referencing a `IstioRevision` resource, the created tag will point to the exact Istio control plane revision that is represented by the `IstioRevision` resource and any update of the tag will have to be made manually by changing the `spec.targetRef` to point to another `IstioRevision`. As long as a `IstioRevisionTag` exists that references a `IstioRevision`, that `IstioRevision` will be considered "in-use" by the Sail Operator, preventing its automatic deletion during a control plane update (see details below under [InUse detection](#inuse-detection))

If the `spec.targetRef` is used to reference an `Istio` resource, the Sail Operator will automatically update the revision tag when a new `IstioRevision` is created as part of a version update of the `Istio` resource. In this case, the `IstioRevisionTag` resource behaves like a floating tag, always referencing the active `IstioRevision` of an `Istio` resource. When it comes to InUse detection, the existence of a floating tag will also cause the active `IstioRevision` of the `Istio` resource to be considered InUse. However, it will not prevent automatic deletion, because the reference is updated immediately when the active revision changes during an update.

#### IstioRevisionTag Status

The `status.state` field gives a quick hint as to whether a tag has been reconciled and is InUse ("Healthy") or if there are any problems with it.

The `status.istiodNamespace` and `status.istioRevision` fields are used by the Sail Operator controllers to store information about the Istio control plane that is referenced by this `IstioRevisionTag`. This is especially useful when it is referencing an `Istio` resource, to see which underlying `IstioRevision` is considered referenced by the operator.

Possible conditions for `status.conditions` are:

##### Reconciled
`true` when the tag's helm chart has been installed successfully. Possible error reasons are:
__RefNotFound__: the resource referenced by the `spec.targetRef` field was not found
__NameAlreadyExists__: there already is an `IstioRevision` with this name
__ReconcileError__: there was an error installing the chart

##### InUse
`true` when the `IstioRevisionTag` is referenced by a namespace or pod. Possible reasons when `false` are:
__NotReferencedByAnything__: no namespace or pod is referencing the tag
__UsageCheckFailed__: there was a problem during InUse detection

#### InUse detection

An `IstioRevisionTag` is considered InUse when

* there's a pod or namespace that explicitly references the `IstioRevisionTag` in an `istio.io/rev` label
* the name of the `IstioRevisionTag` name is 'default' and there's a pod or namespace with the `istio-injection: enabled` label

Note that a pod's `istio.io/rev` annotation will not be considered as that will always have the name of the referenced `IstioRevision` rather than the name of the `IstioRevisionTag`! The labels however are added by users and reflect usage intent, ie the user will use the name of the `IstioRevisionTag`.

Even if the referenced `IstioRevision` of an `IstioRevisionTag` is considered InUse, that does not suffice to make the `IstioRevisionTag` considered InUse by the operator! It is considered an unused alias for an InUse `IstioRevision`.

Additionally, the introduction of `IstioRevisionTag` also adds another condition to the InUse detection of `IstioRevision`: being referenced by an `IstioRevisionTag` will now always lead to an `IstioRevision` being considered InUse! For this, it does not matter if the `IstioRevisionTag` is itself considered InUse.

For completeness' sake, here's an overview of the conditions for an `IstioRevision` to be considered InUse (new condition in bold):

* there's a pod that explicitly references the `IstioRevision` in an `istio.io/rev` annotation or label
* there's a namespace that explicitly references the `IstioRevision` in an `istio.io/rev` label
* the name of the `IstioRevision` is 'default' and there's a pod or namespace with the `istio-injection: enabled` label
* __there's an `IstioRevisionTag` referencing this `IstioRevision`__

#### Changes to existing APIs
We will need to remove the `values.revisionTags` field from our API, which is how the upstream charts expose this feature.

### Architecture
We will need to update the mechanism to detect revisions that are being used. Today, we only look at the `istio.io/rev` annotation's value to check which revisions are in use. But when revision tags are used, those values will point to the referenced revision instead of the tags, so we have to improve our detection mechanism. The most correct way is probably to look at the revision label on the pods and namespace that is set to configure injection.

Revision tags and revision names can be used interchangably, so they must never overlap. Therefore, we'll need a `status` on the `IstioRevisionTag` resource that can show the user an error if the name they chose is already taken by a `IstioRevision`. Another case that needs to be covered is when an `IstioRevision` is being reconciled and it would be assigned the same name as an existing tag. In this case, reconciliation of the `IstioRevision` should fail, with an error message that tells the users why this happened, ie "the name is already used by an `IstioRevisionTag`".

## Alternatives Considered

### Reuse `IstioRevision`'s type field for revision tags
We could add a `type` of `Tag` to the `IstioRevision` CRD and use that to manage tags. It would have the benefit that the user could list all revisions and tags using `kubectl get istiorevisions` and name-uniqueness would be handled by Kubernetes. The disadvantage though is that revision tags share no other fields with `IstioRevision` and it would be quite confusing for users that 99% of the CRD's fields are not to be used in this case, whereas there would be one new field that is only to be used for `IstioRevisions` with type=Tag.

Note that the `type` field has since been removed with the removal of `RemoteIstio`.

### managing tags in Istio resource
In a previous iteration of this SEP, the tags that point to an Istio control plane would have been managed in the `Istio` revision itself, in a `spec.updateStrategy.revisionTags` field. That would have meant that they are always referencing a `Istio` resource while at the same time being copied to every underlying `IstioRevision` resource.

### values.revisionTags
Istio has a `values.revisionTags` field that we even currently expose in our APIs. The problem is that we copy all values from the `Istio` resource to every `IstioRevision` and that means we would be facing duplicate revision tags when we create additional revisions in the Sail Operator - so, some logic would be required to work around this problem. As it is a similar amount of effort, I prefer the explicit version of adding the field to the `Istio` CRD.

### Automatic creation of default revision tag
Previously, we had this paragraph in the Architecture section:
> When the very first `IstioRevision` is created in a cluster from a `RevisionBased` Istio resource, the Sail Operator will create a `IstioRevisionTag` with the name `default`, referencing that `IstioRevision`. This way, the standard namespace injection label `istio-injection=enabled` will work out of the box for RevisionBased deployments (see second paragraph of the [Overview](#overview)).

We have since dropped this from the design as we faced some problems with this approach. Most importantly, it is very hard to detect whether the user is not creating a default IstioRevisionTag or we have simply not seen its creation event yet. We have discussed multiple possible solutions to this, among them usage of a 'virtual' revision tag that is not created on the API server but only exists in-memory, leading to the creation of the respective Kubernetes resources only. This would avoid a race between the operator and the user trying to create tags with the same name simultaneously.

Due to the complexity of the task we have moved it into a separate ticket: [#439 Create default revision tag automatically](https://github.com/istio-ecosystem/sail-operator/issues/439).

## Implementation Plan
v1alpha1
- [x] Initial implementation & tests (https://github.com/istio-ecosystem/sail-operator/pull/413)
- [x] Documentation (https://github.com/istio-ecosystem/sail-operator/pull/511)

v1beta1
- [ ] [#439 Create default revision tag automatically](https://github.com/istio-ecosystem/sail-operator/issues/439)
- [ ] [#471 Support revision tags in multicluster topologies](https://github.com/istio-ecosystem/sail-operator/issues/471)

## Test Plan
Functionality will be tested in integration tests.
