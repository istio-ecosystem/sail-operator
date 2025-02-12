# Release Branches

Release branches have a name of `release-MAJOR.MINOR`. The repository is branched from `main` roughly 3
weeks prior to a new release.  

## Steps to create a release branch

- Create the release branch off the `main` branch  

- Increment the minor version on the main branch  
  _Example_: [PR#517](https://github.com/istio-ecosystem/sail-operator/pull/517)

- Create prow jobs for new branch  
  _Example_: [PR](https://github.com/istio/test-infra/pull/5399) in test-infra repository  

- Create cherry-pick label for new branch  
  _Example_: `cherrypick/release-0.1`  

- Update [versions.yaml](versions.yaml) to contain only the expected versions  

- Update common-files to point to latest supported Istio release branch. 
  Common-files can be updated via setting `UPDATE_BRANCH` in the [Makefile.core.mk](Makefile.core.mk) and running `make update-common`.  
  _Example_: Set [UPDATE_BRANCH](https://github.com/istio-ecosystem/sail-operator/blob/release-0.1/Makefile.core.mk#L22) variable for release-0.1  

- Update dependencies to point to latest supported Istio release branch. 
  Dependencies can be updated via [update_deps.sh](tools/update_deps.sh) with the correct value for `UPDATE_BRANCH` variable.  

- Check the used versions in [go.mod](go.mod). `update_deps.sh` updates modules in `go.mod` to the latest versions in `UPDATE_BRANCH` 
  so it might not be the versions used in actual istio release.
  Ideally we should use the actual release tag in `go.mod`.  
  _Example_: `istio.io/client-go v1.23.0` instead of `istio.io/client-go v1.23.0-alpha.0.0.20240809192551-f32a7326ae19`  
  _Note_: This is not possible for `istio.io/istio` as the version tags don't contain the `v` prefix, so for that repository we'll have to use the pseudoversion. This can be done e.g. `go get -u istio.io/istio@1.23.0`  

# Feature Freeze

One week before a release, the release branch goes into a state of code freeze. At this point only critical release
blocking bugs are addressed. Additional changes that are targeted for new features and capabilities will not be merged.

# Create a release

A release can be created via the GitHub Actions workflow `Release workflow`.  

- From the `Actions` tab, select the `Release workflow` on the left menu  

- Click on `Run workflow` button on the top right and specify the parameters:  

  - Select the branch of the release  

  - Specify the version of the release. The format must be `release-MAJOR.MINOR` (eg. `release-1.0`)  

  - Specify the `Bundle channels` field  

  - Check the box `Draft release` if it is the case. Note: this will create a draft release which must be edited and published afterwards.   
  
  - Check the box `Pre-release` if needed  

  - Click on the button `Run workflow` to launch the workflow  