# Release Branches

Release branches have a name of `release-MAJOR.MINOR`. The repository is branched from `main` roughly 3
weeks prior to a new release.  

# New minor release preparations
Please clone this [GitHub issue](https://github.com/istio-ecosystem/sail-operator/issues/1333) and follow all the steps of the checklist to prepare the release.

# New z-stream release preparations
Please clone this [GitHub issue](https://github.com/istio-ecosystem/sail-operator/issues/1434) and follow all the steps of the checklist to prepare the release.

# Feature Freeze

One week before a release, the release branch goes into a state of code freeze. At this point only critical release
blocking bugs are addressed. Additional changes that are targeted for new features and capabilities will not be merged.

# Create a release

When all preparations are done, a release can be created via the GitHub Actions workflow `Release workflow`.

Clone this [GitHub issue](https://github.com/istio-ecosystem/sail-operator/issues/1436) and follow all steps of the checklist.
