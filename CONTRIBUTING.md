# Contribution guidelines to the Sail Operator project

So you want to make contributions to the Sail Operator project, please take a look at the following guidelines to help you get started.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Contributing to the Sail Operator](#contributing-to-the-sail-operator)
- [Way of Working](#way-of-working)
- [Community meetings](#community-meetings)
- [Security Issues](#security-issues)

## Code of Conduct

As a contributor, you are expected to adhere to the [Code of Conduct](https://github.com/cncf/foundation/blob/main/code-of-conduct.md) from CNCF as Istio community does. Please read the Code of Conduct before contributing to the project.

## Contributing to the Sail Operator

We are open to contributions from the community. If you want to contribute to the Sail Operator project, you can start by opening a new issue in the [Sail Operator GitHub repository](https://github.com/istio-ecosystem/sail-operator/issues) or starting a discussion in the [Sail Operator Discussion](https://github.com/istio-ecosystem/sail-operator/discussions). You can also join the Istio Slack workspace by visiting the [Istio Slack invitation page](https://slack.istio.io/) and finding the `#sail-operator` channel.

## Way of Working

If you want to contribute to the Sail Operator project, you can follow some rules that we have defined to help you get started:

- Discuss your changes before you start working on them. You can open a new issue in the [Sail Operator GitHub repository](https://github.com/istio-ecosystem/sail-operator/issues) or start a discussion in the [Sail Operator Discussion](https://github.com/istio-ecosystem/sail-operator/discussions). By this way, you can get feedback from the community and ensure that your changes are aligned with the project goals.
- Use of Labels: We use labels in the issues to help us track the progress of the issues. You can use the labels to help you understand the status of the issue and what is needed to move forward. Those labels are:
  - `backport/backport-handled`: Use this label to indicate that the issue has been backported to the appropriate branches.
  - `testing`: Use this label to indicate that the issue is related to testing. Can be used in combination with other labels to mark the proper testing type, for example: `testing/e2e`, `testing/unit`, `testing/integration`.
  - `good first issue`: Use this label to indicate that the issue is a good first issue for new contributors.
  - `help wanted`: Use this label to indicate that the issue needs help from the community.
  - `enhancement`: Use this label to indicate that the issue is an enhancement related to a new feature or improvement.
- Pull Requests: When you open a pull request, you can follow this template to help you provide the necessary information to the maintainers:
  - **What type of PR is this?**
  - **What this PR does / why we need it:**
  - **Which issue(s) this PR fixes:** (Mark with Fixes #12345, with this the issue will be autoclosed when the PR is merged)
  - **Special notes for your reviewer:**
  - **Does this PR introduce a user-facing change?**
  - **Additional documentation:**
  - **Does this PR introduce a breaking change?**
  - **Other information:**
  - Labels: You can use the labels to help you track the status of the PR. The labels are the same as the issue labels. Additionally, you can use the `cleanup/refactor` to indicate that the PR is a cleanup or refactor of the codebase. Having the label just helps with filtering pull requests. It also is a hint that this work does not need an entry in the changelog

## Community meetings

This is not defined yet. We are working on defining the community meetings and how the community can participate in them. We will update this section once we have more information.

## Security Issues

If you find a security issue in the Sail Operator project, please refer to the [Security Policy](https://github.com/istio-ecosystem/sail-operator/security/policy) for more information on how to report security issues. Please do not report security issues in the public GitHub repository.