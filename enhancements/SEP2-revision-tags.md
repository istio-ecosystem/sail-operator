|Status                                             | Authors      | Created    | 
|---------------------------------------------------|--------------|------------|
|WIP                                                | @dgn         | 2024-07-17 |

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
* Provide revision tag support in sail-operator APIs so users don't have to use istioctl for basic revision tag operations

## Non-goals
* Compatibility with manual revision tag creation using istioctl. There might be a way to disable the operator functionality to avoid conflicts when creating revision tags manually, but that's it - you either do it yourself or let the operator do it

## Design

### User Stories
1. As a user of sail-operator's RevisionBased update strategy, I want to be able to use the `istio-injection=enabled` label on my namespaces.
1. As a platform engineer, I want my application teams to be able to use a fixed label for proxy injection without having to know which version of Istio is running in the cluster, so that I can perform upgrades in the background without the application teams having to make changes to use the new version.

### API Changes
We will add a new CRD called `IstioRevisionTag` that includes a `spec.targetRef` field. In this field, users can specify the `IstioRevision` or `Istio` resource that the `IstioRevisionTag` references.

In case of referencing a `IstioRevision` resource, the created tag will point to the exact Istio control plane revision that is represented by the `IstioRevision` resource and any update of the tag will have to be made manually by changing the `spec.targetRef` to point to another `IstioRevision`. As long as a `IstioRevisionTag` exists that references a `IstioRevision`, that `IstioRevision` will be considered "in-use" by the Sail Operator, preventing its automatic deletion during a control plane update.

If the `spec.targetRef` is used to reference an `Istio` resource, the Sail Operator will automatically update the revision tag when a new `IstioRevision` is created as part of a version update of the `Istio` resource. In this case, the `IstioRevisionTag` resource behaves like a floating tag, always referencing the latest `IstioRevision` of an `Istio` resource.

When the very first `IstioRevision` is created in a cluster from a `RevisionBased` Istio resource, the Sail Operator will create a `IstioRevisionTag` with the name `default`, referencing that `IstioRevision`. This way, the standard namespace injection label `istio-injection=enabled` will work out of the box for RevisionBased deployments (see second paragraph of the [Overview](#overview)).

We will also need to remove the `values.revisionTags` field from our API, which is how the upstream charts expose this feature.

### Architecture
We will need to update the sail-operators mechanism to detect revisions that are being used. Today, we only look at the `istio.io/rev` label's value to check which revisions are in use. But when revision tags are used, those values will be mere aliases, so we have to improve our detection mechanism. The most correct way is probably to look at the revision annotation on the pods that is set by Istio during injection. That requires inspecting every pod in the cluster, though. Another way could be to resolve the tags - the sail-operator knows which revisions the tags point to, after all - but only if tags are exclusively managed by the sail-operator.

Revision tags and revision names can be used interchangably, so they must never overlap. Therefore, we'll need a `status` on the `IstioRevisionTag` resource that can show the user an error if the name they chose is already taken by a `IstioRevision`. Another case we'll need to cover is when an `IstioRevision` is being reconciled and it would be assigned the same name as an existing tag. In this case, I suggest the `IstioRevision` wins and the tag is decommissioned, also with an error in its status. As the revision names generated for `IstioRevision` are very specific and end with the Istio control plane version, it is unlikely for this to happen, but we'll need to define the behavior for when it does.

## Alternatives Considered

### Reuse `IstioRevision`'s type field for revision tags
We could add a `type` of `Tag` to the `IstioRevision` CRD and use that to manage tags. It would have the benefit that the user could list all revisions and tags using `kubectl get istiorevisions` and name-uniqueness would be handled by Kubernetes. The disadvantage though is that revision tags share no other fields with `IstioRevision` and it would be quite confusing for users that 99% of the CRD's fields are not to be used in this case, whereas there would be one new field that is only to be used for `IstioRevisions` with type=Tag.

### managing tags in Istio resource
In a previous iteration of this SEP, the tags that point to an Istio control plane would have been managed in the `Istio` revision itself, in a `spec.updateStrategy.revisionTags` field. That would have meant that they are always referencing a `Istio` resource while at the same time being copied to every underlying `IstioRevision` resource.

### values.revisionTags
Istio has a `values.revisionTags` field that we even currently expose in our APIs. The problem is that we copy all values from the `Istio` resource to every `IstioRevision` and that means we would be facing duplicate revision tags when we create additional revisions in the sail-operator - so, some logic would be required to work around this problem. As it is a similar amount of effort, I prefer the explicit version of adding the field to the `Istio` CRD.

## Implementation Plan
- [ ] Improve revision usage detection to support revision tags
- [ ] Create IstioRevisionTag CRD and controller
- [ ] Implement support for referencing Istio resources
- [ ] Implement support for referencing IstioRevision resources
- [ ] Add logic to prevent duplications across revisions and tags
- [ ] Documentation
- [ ] Integration tests

## Test Plan
We should cover this functionality in our sail-operator integration tests.
