|Status                                             | Authors      | Created    | 
|---------------------------------------------------|--------------|------------|
|WIP                                                | @dgn         | 2024-07-17 |

# Revision Tag Support

## Overview
Upstream Istio supports the use of revision tags for multi-revision deployments and canary upgrades of Istio control planes. These tags serve as aliases for revisions and allow users to use stable revision names (e.g. `prod` or `default`), so they don't have to change their namespace and pod labels (in this case `istio.io/rev=prod` or `istio-injection=enabled`) when switching to a new version. Instead, by tagging a new revision with the correct tag and restarting their workloads, they can perform an Istio update without having to change their labels. This is especially useful in situations where the team managing the Istio control plane is separate from the teams managing the workloads.

Each revision tag only ever points to exactly one Istio revision. Upstream, revision tags are created manually using `istioctl` and- as they only affect injection- are represented in the cluster by a MutatingWebhookConfiguration.

## Goals
* Provide revision tag support in sail-operator APIs so users don't have to use istioctl for basic revision tag operations

## Non-goals
* Compatibility with manual revision tag creation using istioctl. There might be a way to disable the operator functionality to avoid conflicts when creating revision tags manually, but that's it - you either do it yourself or let the operator do it

## Design

### User Stories
1. As a user of sail-operator's RevisionBased update strategy, I want to be able to use the `istio-injection=enabled` label on my namespaces and pods.
1. As a platform engineer, I want my application teams to be able to use a fixed label for proxy injection without having to know which version of Istio is running in the cluster, so that I can perform upgrades in the background without the application teams having to make changes to use the new version.

### API Changes
We will add a new field `revisionTags` of type `[]string` to the `Istio` CRD. It will be located under `spec.updateStrategy.revisionTags`. Whenever the sail-operator deploys a new revision, it will update all the tags listed in the `revisionTags` field to point to this new revision. The default value for the field will be `{"default"}` - this way, the standard injection label `istio-injection=true` will work out of the box for RevisionBased deployments.

We will also need to remove the `values.revisionTags` field (which is how the upstream charts expose this feature). We can still set that field to trigger creation of the tags, but we should not expose it to users.

### Architecture
We will need to update the sail-operators mechanism to detect revisions that are being used. Today, we only look at the `istio.io/rev` label's value to check which revisions are in use. But when revision tags are used, those values will be mere aliases, so we have to improve our detection mechanism. The most correct way is probably to look at the revision annotation on the pods that is set by Istio during injection. That requires inspecting every pod in the cluster, though. Another way could be to resolve the tags - the sail-operator knows which revisions the tags point to, after all - but only if tags are exclusively managed by the sail-operator.

Revision tags must never overlap between `Istio` resources, so we'll need to make sure during reconciliation that they are unique and report an error in the status if they are not.

## Alternatives Considered
### values.revisionTags
Istio has a `values.revisionTags` field that we even currently expose in our APIs. The problem is that we copy all values from the `Istio` resource to every `IstioRevision` and that means we would be facing duplicate revision tags when we create additional revisions in the sail-operator - so, some logic would be required to work around this problem. As it is a similar amount of effort, I prefer the explicit version of adding the field to the `Istio` CRD.

## Implementation Plan
- [ ] Improve revision usage detection to support revision tags
- [ ] Add field revisionTags to Istio CRD
- [ ] Implement tag creation and update
- [ ] Add logic to prevent duplicate tags

## Test Plan
We should cover this functionality in our sail-operator integration tests.
