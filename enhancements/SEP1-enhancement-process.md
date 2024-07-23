|Status                                             | Authors      | Created    | 
|---------------------------------------------------|--------------|------------|
|Done                                               | @dgn         | 2024-07-09 |

# Sail Enhancement Proposal (SEP) Process

## Overview


## Goals
For non-trivial (for definition see below) changes to the Sail Operator, we're going to use an open design process that allows us to:
1. discuss designs and potential APIs in detail where GitHub issues might not be sufficient,
1. record important architectural decisions so we can find and reference them later on,
1. identify functional and non-functional requirements before implementation,
1. conduct our design in the open so that community members can participate and
1. use asynchronous communication throughout to minimize meeting load.

## Non-goals
1. Overcomplicating the engineering process. Simple things should stay simple - just submit a PR to fix that bug. It's the hard things we want to make sure to get right using this process. But that doesn't mean you have to write pages of prose. Keep the SEPs short and concise, it will make reviews easier and speed up the process.

## Design

### Applicability
The SEP process should be used for non-trivial features or epics and any API changes. Changes without user impact will most of the time not need a SEP, but if they're big enough or result in changes to the development workflows (e.g. a rewrite of our integration testing framework) it's probably a good idea to create one.

If your Proposal includes work that has to be done in Istio, depending on the size of the patchset it might be required to create an Istio design doc. If you do create one, make sure to link to it in the SEP.

### Storage
Initially, SEPs will be stored in the enhancements/ directory of the sail-operator repository in Markdown format. We might migrate them to a separate repository later on.

### Creating a SEP
Whenever somebody working on the Sail Operator identifies a feature that involves a lot of code and/or API changes, they should use [the template](./SEP0-template.md) to create a SEP in WIP state for it that captures the essence of the problem and a first draft of a design that will be used as a basis for discussion. Diagrams are often helpful when discussing complex issues; if your SEP makes use of them, they should be pushed to the [diagrams subdirectory](./diagrams/). Proof-of-Concept PRs are great too - make sure to link them in the SEP.

If possible, team up with others to create the SEP!

### Review Process
We're using a normal GitHub review workflow. For now, every member of the team can approve these PRs and merge them. We can adjust that later if we find that it would be better to have a smaller group of people with approval rights.

### Accepted SEP
A SEP should only be marked as accepted once all relevant sections have been filled out, especially the design, implementation and testing plan parts.

Once a SEP is accepted, it is considered read-only. Should a change be urgently required (maybe as a result of a team discussion), there can be exceptions to this rule, though - especially if it's a small change that doesn't warrant a separate SEP. Any post-acceptance changes should include a dated (ISO format: yyyy-mm-dd) one-line summary of the changeset (can be the same as the commit message) at the bottom of the document.

## Alternatives Considered

* Using GDocs for storage - good for collaboration and search but hard to "lock down" a document once it has been approved. Also there's a disconnect between the code and the documents
* ad-hoc meetings and slack discussions - this is what we've done up until now but it's not community-friendly and hard to do across time zones

## Implementation Plan

We will start using this process ASAP. This SEP can stay in WIP state for a while though, until we're sure we have fine-tuned everything for our needs.

## Test Plan

N/A
