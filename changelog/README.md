# Changelog Fragments

Each pull request should include a YAML file in this directory describing the change. At release time, these fragments are collected into `CHANGELOG.md` and deleted. Using individual files instead of editing a shared changelog avoids merge conflicts during cherry-picks.

## File Format

Create a file named `<short-slug>.yaml` with the following fields:

```yaml
category: added | changed | fixed | removed
title: Short one-line description of the change
description: |                    # optional: multi-line detail
  Extended explanation that will appear indented under the title
  in the rendered changelog.
issueLink: https://github.com/istio-ecosystem/sail-operator/issues/NNN  # required for 'fixed'
```

The `category` and `title` fields are required. The `issueLink` field is required when the category is `fixed`. Use `skip-changelog` label on the PR to opt out.

## Fragments on main after a release

A fix made on main can also land in a release branch (via cherry-pick) and ship in a patch release before main cuts its own next release. Its fragment stays on main until then, so it doesn't get lost.

The daily changelog sync job (`hack/sync-changelog-releases.sh`) pulls released version sections from release branches into main's `CHANGELOG.md`. If a synced version is a new minor release (`x.y.0`), it deletes the matching fragments from main: that release already covers them, and main's next release will be a different minor.

If the fix only shipped in a patch release (`x.y.1`, `x.y.2`, ...), its fragment is left in place, so it still gets included when main cuts its own next minor release.
