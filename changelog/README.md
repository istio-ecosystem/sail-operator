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
