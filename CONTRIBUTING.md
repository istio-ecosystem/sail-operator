# Contribution guidelines to the Sail Operator project

So you want to make contributions to the Sail Operator project, please take a look at the following guidelines to help you get started.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Contributing to the Sail Operator](#contributing-to-the-sail-operator)
- [Way of Working](#way-of-working)
- [Community meetings](#community-meetings)
- [Security Issues](#security-issues)
- [Writing Documentation](#writing-documentation)

## Code of Conduct

As a contributor, you are expected to adhere to the [Code of Conduct](https://github.com/cncf/foundation/blob/main/code-of-conduct.md) from CNCF as Istio community does. Please read the Code of Conduct before contributing to the project.

## Contributing to the Sail Operator

We are open to contributions from the community. If you want to contribute to the Sail Operator project, you can start by opening a new issue in the [Sail Operator GitHub repository](https://github.com/istio-ecosystem/sail-operator/issues) or starting a discussion in the [Sail Operator Discussion](https://github.com/istio-ecosystem/sail-operator/discussions). You can also join the Istio Slack workspace by visiting the [Istio Slack invitation page](https://slack.istio.io/) and finding the `#sail-operator` channel.

## Way of Working

If you want to contribute to the Sail Operator project, you can follow some rules that we have defined to help you get started:

- Discuss your changes before you start working on them. You can open a new issue in the [Sail Operator GitHub repository](https://github.com/istio-ecosystem/sail-operator/issues) or start a discussion in the [Sail Operator Discussion](https://github.com/istio-ecosystem/sail-operator/discussions). By this way, you can get feedback from the community and ensure that your changes are aligned with the project goals.
- Use of Labels: We use labels in the issues to help us track the progress of the issues. You can use the labels to help you understand the status of the issue and what is needed to move forward. Those labels are:
  - `backport/backport-handled`: Use this label to indicate that the issue has been backported to the appropriate branches.
  - `test`: Use this label to indicate that the issue is related to test or add `test-needed` when a issue needs a test to be added related. Can be used in combination with other labels to mark the proper test type, for example: `test-e2e`, `test-unit`, `test-integration`.
  - `good first issue`: Use this label to indicate that the issue is a good first issue for new contributors.
  - `help wanted`: Use this label to indicate that the issue needs help from the community.
  - `enhancement`: Use this label to indicate that the issue is an enhancement related to a new feature or improvement.
- Commit should contains Header and Body explanation of the change for the future references.

## Community meetings

The Sail Operator project has a weekly contributor call every Thursday at 4PM CET / 10AM EST (Check your local time) to discuss the project status, features, isues, and other topics related to the project. You can join the meeting by using the [link](https://meet.google.com/uxg-wcfp-opv), also the agenda and notes are available in the following [link](https://docs.google.com/document/d/1p1gx7dC8XQwFtv6l0zQbZjObAVJVTOBH2PvLVX6wU_0/edit?usp=sharing), if you want to suggest a topic please add it to the agenda.

## Security Issues

If you find a security issue in the Sail Operator project, please refer to the [Security Policy](https://github.com/istio-ecosystem/sail-operator/security/policy) for more information on how to report security issues. Please do not report security issues in the public GitHub repository.

## Writing Documentation
If you want to add new documentation or examples to our existing documentation please take a look at the [documentation guidelines](docs/guidelines/guidelines.md) to help you get started. Take into account that we run automation test over the documentation examples, so please ensure that the examples are correct and follows the guidelines to ensure it will be properly tested.
