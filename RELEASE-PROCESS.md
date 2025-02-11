# Release Branches

Release branches have a name of `release-MAJOR.MINOR`. The repository is branched from main roughly 3
weeks prior to a new release.  

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