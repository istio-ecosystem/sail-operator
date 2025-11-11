# Update EOL Istio Versions

This command automates the process of marking End-of-Life (EOL) Istio versions in the versions.yaml file.

## Tasks

1. **Fetch supported Istio versions** from istio.io:
   - Visit https://istio.io/latest/docs/releases/supported-releases/ to get the list of currently supported Istio versions
   - Identify which major.minor versions are still supported upstream
   - Note the EOL dates for versions that are no longer supported

2. **Analyze what changes are needed**:
   - Read `pkg/istioversion/versions.yaml` (or the file specified by VERSIONS_YAML_FILE env var)
   - Compare the current state with the upstream supported versions
   - Identify versions that need to be marked as EOL (not supported upstream but missing `eol: true`)
   - Identify versions that need EOL removed (supported upstream but have `eol: true`)
   - **If no changes are needed**, inform the user that all versions are already up-to-date and STOP
   - Only proceed to the next steps if changes are required

3. **Create a new git branch** for this update (only if changes are needed):
   - Branch name should be: `update-eol-versions-YYYY-MM-DD` (use today's date)
   - Ensure the working directory is clean before creating the branch
   - If there are uncommitted changes, ask the user what to do

4. **Update versions.yaml**:
   - For each version entry that needs changes:
     - If the version is NOT supported upstream and doesn't already have `eol: true`, add `eol: true` to that version entry
     - If a version has `eol: true` but is still supported upstream, remove the `eol: true` flag (this handles corrections)
   - Preserve the YAML structure and comments
   - For EOL versions, keep only `name:`, `eol:` and `ref:` sections


5. **Run code generation**:
   - Execute `make gen` to regenerate all necessary code and manifests
   - This ensures CRDs and other generated files are updated

6. **Show summary**:
   - List all versions that were marked as EOL
   - List all versions that had EOL status removed (if any)
   - Show the git diff of changes made to versions.yaml
   - Provide next steps for the user (review changes, run tests, commit, create PR)

## Important Notes

- Only mark versions as EOL if they are confirmed to be EOL upstream Istio [project](https://istio.io/latest/docs/releases/supported-releases/)
- Preserve all existing version entries - do not remove them from the file
- The `eol: true` flag makes versions uninstallable but keeps them as valid spec.version values for API compatibility
- For EOL versions, keep only `name:`, `eol:` and `ref:` sections
- Show the changes and ask the user to confirm the changes before committing

## Example Version Entry

Before (supported version):
```yaml
- name: v1.25.0
  version: 1.25.0
  repo: https://github.com/istio/istio
  commit: 1.25.0
  charts:
    - https://istio-release.storage.googleapis.com/charts/base-1.25.0.tgz
    - https://istio-release.storage.googleapis.com/charts/istiod-1.25.0.tgz
```

After (EOL version):
```yaml
- name: v1.25.0
  eol: true
```
