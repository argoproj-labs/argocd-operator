# Submitting code contributions to Argo CD Operator

## Preface

The Argo CD Operator project continuously grows, both in terms of features and community size. Thus, we need to take great care with any changes that affect compatibility, performance, scalability, stability and security of the Argo CD Operator. For this reason, every new feature or larger enhancement must be properly designed and discussed before it gets accepted into the code base.

We do welcome and encourage everyone to participate in the Argo CD Operator project, but please understand that we can't accept each and every contribution from the community, for various reasons.

If you want to submit code for a great new feature or enhancement, we kindly ask you to take a look at the
enhancement process outlined below before you start to write code or submit a PR. This will ensure that your idea is well aligned with the project's strategy and technical requirements, and it will help greatly in getting your code merged into our code base.

Before submitting code for a new feature (and also, to some extent, for more complex bug fixes) please
[raise a Feature Request (enhancement proposal) or Bug Issue](https://github.com/argoproj-labs/argocd-operator/issues/new/choose)
first.

_Please_ do not spend too much time on larger features or refactorings before the corresponding enhancement has been triaged. This may save everyone some amount of frustration and time, as the enhancement proposal might be rejected, and the code would never get merged. However, sometimes it's helpful to have some PoC code along with a proposal. 

We will do our best to triage incoming enhancement proposals quickly, with one of the following outcomes:

* Accepted
* Declined
* Needs Discussion

Depending on how many enhancement proposals we receive at given times, it may take some time until we can look at yours.

## Quick start

If you want a quick start contributing to Argo CD Operator, take a look at issues that are labeled with
[help wanted](https://github.com/argoproj-labs/argocd-operator/labels/help%20wanted) or [good first issue](https://github.com/argoproj-labs/argocd-operator/labels/good%20first%20issue). These are issues that were already triaged and accepted.

## Proposal states

**Accepted Proposals:** When a proposal is considered _Accepted_, it was decided that this enhancement would be valuable to the community at large and fits into the overall strategic roadmap of the project. Implementation of the issue may be started, either by the proposal's creator or another community member (including maintainers of the project). The issue should be refined enough by now to contain any concerns and guidelines to be taken into consideration during implementation.

**Declined proposals:** We don't decline proposals lightly, and we will do our best to give a proper reasoning why we think that the proposal does not fit with the future of the project. Reasons for declining proposals may be (amongst others) that the change would be breaking for many, or that it does not meet the strategic direction of the project. Usually, discussion will be facilitated with the enhancement's creator before declining a proposal. Once a proposal is in _Declined_ state it's unlikely that we will accept code contributions for its implementation.

**Needs discussion:** Sometimes, we can't completely understand a proposal from its GitHub issue and thus require more information on the original intent or more details about the implementation. If we are confronted with such an issue during the triage we expect the issue's creator to supply more information on their idea.

## Design documents

For some enhancement proposals (especially those that will change behavior of Argo CD Operator substantially, are attached with some caveats, or where upgrade/downgrade paths are not clear), a more formal design document will be required in order to fully discuss and understand the enhancement in the broader community. This requirement is usually determined during triage. If you submitted an enhancement proposal, we may ask you to provide this more formal write down, along with some concerns or topics that need to be addressed. Please consider adding visuals wherever possible to increase understanding of complex issues; we recommend using [Miro](https://miro.com/) as a visual aid tool. 

Design documents are usually submitted as a PR and use [this template](https://github.com/argoproj-labs/argocd-operator/blob/master/docs/proposals/001-proposal-template.md) as a guide what kind of information we're looking for. Discussion will take place in the review process. When a design document gets merged, we consider it as approved and code can be written and submitted to implement this specific design.