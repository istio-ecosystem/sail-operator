# Add Missing Istio Versions across all N-2 release branches

This command automates the process of adding all missing Istio versions across one or more release branches. It discovers which branches have missing versions, lets the user choose which branches to update, then processes each branch sequentially — handling version discovery, dependency updates, versions.yaml maintenance, and go.mod verification.

## Prerequisites

- The working directory should be clean (no uncommitted changes)
- `gh` CLI must be installed and authenticated — verify with:
  ```bash
  gh auth status
  ```
  If not authenticated, inform the user and STOP
- Go toolchain must be available
- Docker must be running — `update_deps.sh` and `make` targets require Docker. Verify with `docker info` before proceeding

## Tasks

---

### Phase 1: Discovery and Branch Selection

---

### 1. Detect remotes, save current state, and fetch latest

- **Detect the upstream remote** — find the remote whose URL contains `istio-ecosystem/sail-operator`. Do NOT hardcode the remote name `upstream`; users may name it differently:
  ```bash
  git remote -v | grep 'istio-ecosystem/sail-operator' | grep '(fetch)' | awk '{print $1}' | head -1
  ```
  Store this as `UPSTREAM_REMOTE` (e.g., `upstream`). If no remote matches, ask the user which remote points to the upstream repo, or STOP.
- **Detect the fork remote** — find the remote whose URL does NOT contain `istio-ecosystem/sail-operator` (i.e., the user's fork). If there are multiple candidates, prefer `origin`:
  ```bash
  git remote -v | grep 'sail-operator' | grep -v 'istio-ecosystem/sail-operator' | grep '(push)' | awk '{print $1}' | head -1
  ```
  Store this as `FORK_REMOTE` (e.g., `origin`). Also extract the fork owner (GitHub username) from the URL:
  ```bash
  git remote get-url <FORK_REMOTE> | sed -E 's|.*[:/]([^/]+)/sail-operator.*|\1|'
  ```
  Store this as `FORK_OWNER`.
- Save the current branch name so we can return to it at the end:
  ```bash
  git rev-parse --abbrev-ref HEAD
  ```
- Verify the working directory is clean:
  ```bash
  git status --porcelain
  ```
  If not clean, warn the user and STOP
- Fetch latest from the upstream remote:
  ```bash
  git fetch <UPSTREAM_REMOTE>
  ```

Use `UPSTREAM_REMOTE`, `FORK_REMOTE`, and `FORK_OWNER` throughout all subsequent steps instead of hardcoded remote names.

### 2. Discover release branches

- List all release branches on the upstream remote matching the `release-X.YY` pattern:
  ```bash
  git branch -r | grep -E '<UPSTREAM_REMOTE>/release-1\.[0-9]+$' | sed 's|.*<UPSTREAM_REMOTE>/||' | sort -t. -k2 -n
  ```
- Filter to only the active (non-EOL) branches — the n-2 rule means the 3 highest minor versions
- For each active branch, read its current versions.yaml **without switching branches**:
  ```bash
  git show <UPSTREAM_REMOTE>/release-X.YY:pkg/istioversion/versions.yaml
  ```
- Also read each branch's `Makefile.core.mk` to get its current VERSION:
  ```bash
  git show <UPSTREAM_REMOTE>/release-X.YY:Makefile.core.mk | grep -E '^(VERSION|PREVIOUS_VERSION) \?='
  ```

### 3. Fetch upstream Istio releases and identify missing versions

- Fetch all stable Istio releases from GitHub **once** (this result is reused for all branches):
  ```bash
  gh api repos/istio/istio/releases --paginate --jq '.[].tag_name' | grep -E '^[0-9]+\.[0-9]+\.[0-9]+$' | sort -V
  ```
- Only consider stable releases (no alpha, beta, or rc versions)
- For each active branch, determine which minor version series it supports (n-2 from the branch's own minor version), then compare the branch's versions.yaml against upstream releases to find missing versions
- Present a summary table to the user, e.g.:
  ```
  Branch          | Missing versions
  ----------------|------------------
  release-1.30    | 1.30.3, 1.29.4, 1.28.10
  release-1.29    | 1.29.4, 1.28.10
  release-1.28    | 1.28.10
  ```
- **If no versions are missing on any branch**, inform the user and STOP
- Ask the user which branches to update — suggest all branches with missing versions as default, but let the user deselect any they want to skip

---

### Phase 2: Per-Branch Processing

---

For each selected branch, **processed from highest minor version to lowest** (e.g., release-1.30 first, then release-1.29, then release-1.28):

### 4. Create a temporary working branch

- Inform the user: "Processing branch release-X.YY..."
- Determine the temporary branch name. The format is `release-X.YY_add_istio_VERSIONS` where VERSIONS lists the new primary version(s) being added. For example:
  - If adding 1.30.3 (and 1.29.6 as a secondary): `release-1.30_add_istio_1.30.3`
  - If adding 1.29.3-1.29.6 (and 1.28.7-1.28.10): `release-1.29_add_istio_1.29.6`
  - The branch name uses only the highest patch version from the branch's primary minor series
- Create the temporary branch from the upstream remote:
  ```bash
  git checkout -b release-X.YY_add_istio_A.BB.CC <UPSTREAM_REMOTE>/release-X.YY
  ```
- If a local branch with that name already exists, delete it first (after confirming with the user) and re-create from the upstream remote
- Verify clean state after checkout

### 5. Determine the branch's supported minor versions

- Extract the minor version from the branch name (e.g., `1.30` from `release-1.30`)
- Read `Makefile.core.mk` to get current `VERSION` and `PREVIOUS_VERSION`
- Read `pkg/istioversion/versions.yaml` to understand which versions are already present
- Identify which minor version series are supported (non-EOL) on this branch (n-2: current + two previous minor versions)

### 6. Identify missing versions for this branch

- Using the upstream releases fetched in Step 3, compare with the branch's versions.yaml
- List all missing stable versions for each supported minor series
- **If no versions are missing on this branch**, inform the user and skip to the next branch

### 7. Determine if the operator version needs incrementing

- If the latest upstream Istio version for the primary minor series (e.g., 1.30.x) is newer than the current `VERSION` in `Makefile.core.mk`, the operator version needs to be updated
- The new `VERSION` should match the latest Istio patch version for the primary minor series (e.g., if latest is 1.30.3, set VERSION=1.30.3)
- **PREVIOUS_VERSION** should only be updated on the **active release branch** (the branch matching the highest minor version among all release branches, e.g., `release-1.30` when 1.30 is the latest). On older branches (e.g., `release-1.28`), PREVIOUS_VERSION should NOT be changed because those branches are not used for new OLM releases.
  - The active branch was already determined in Step 2 (highest minor version)
  - If the current branch is NOT the active release branch, ask the user: "This is not the active release branch. Do you also plan to release this change in upstream as new Sail Operator version? If yes, PREVIOUS_VERSION will be updated; if no, only VERSION will be updated."
  - If the user confirms they plan to release, update `PREVIOUS_VERSION` to the old `VERSION` value
  - If not, leave `PREVIOUS_VERSION` unchanged
- Present the proposed version changes to the user for confirmation before proceeding

### 8. Update VERSION and PREVIOUS_VERSION in Makefile.core.mk

- Update the `VERSION ?=` line in `Makefile.core.mk` to the new version
- Only update `PREVIOUS_VERSION ?=` if this is the active release branch or the user confirmed they plan to release (see Step 7)
- Example (active release branch):
  ```
  # Before:
  VERSION ?= 1.30.0
  PREVIOUS_VERSION ?= 1.29.2

  # After (if 1.30.3 is the latest):
  VERSION ?= 1.30.3
  PREVIOUS_VERSION ?= 1.30.0
  ```
- Example (older branch, no release planned):
  ```
  # Before:
  VERSION ?= 1.28.8
  PREVIOUS_VERSION ?= 1.28.5

  # After (if 1.28.10 is the latest):
  VERSION ?= 1.28.10
  PREVIOUS_VERSION ?= 1.28.5  (unchanged)
  ```

### 9. Run update_deps.sh

- Run the dependency update script with the appropriate branch:
  ```bash
  UPDATE_BRANCH=release-X.YY PIN_MINOR=true ./tools/update_deps.sh
  ```
  where `X.YY` matches the current branch (e.g., `release-1.30`)
- This script will update common files, go dependencies, tool versions, and add missing Istio versions to versions.yaml
- Wait for this to complete (it can take several minutes)
- **If the script fails**: It may fail on non-critical steps like tool version checks (e.g., misspell, gitleaks, or other GitHub API lookups). In that case:
  1. Check how far the script got by reviewing its output
  2. The critical operations are `make update-istio` and `make gen` — if the script failed before reaching them, run them manually:
     ```bash
     make update-istio
     make gen
     ```
  3. Tool version updates in `Makefile.core.mk` (e.g., MISSPELL_VERSION, GITLEAKS_VERSION) can be skipped or updated manually later — they are not essential for adding Istio versions

### 10. Clean up versions.yaml

After `update_deps.sh` runs, verify and clean up `versions.yaml`:

#### 10a. Remove alpha/beta/rc versions
- Scan all entries for versions containing `-alpha`, `-beta`, `-rc`, or any other pre-release suffix
- Remove those entries entirely
- If any `-latest` ref points to a pre-release version, update it to point to the latest stable version instead

#### 10b. Fill in missing patch versions (no gaps)
- For each supported minor version series, check that all patch versions are present with no gaps
- For example, if 1.29.0, 1.29.1, 1.29.3, 1.29.5 exist, then 1.29.2 and 1.29.4 are missing and need to be added
- Verify each missing version actually exists as a release upstream before adding it:
  ```bash
  gh api repos/istio/istio/releases/tags/VERSION_NUMBER --jq '.tag_name' 2>/dev/null
  ```
- Add missing versions with the standard entry format:
  ```yaml
  - name: v1.29.2
    version: 1.29.2
    repo: https://github.com/istio/istio
    commit: 1.29.2
    charts:
      - https://istio-release.storage.googleapis.com/charts/base-1.29.2.tgz
      - https://istio-release.storage.googleapis.com/charts/istiod-1.29.2.tgz
      - https://istio-release.storage.googleapis.com/charts/gateway-1.29.2.tgz
      - https://istio-release.storage.googleapis.com/charts/cni-1.29.2.tgz
      - https://istio-release.storage.googleapis.com/charts/ztunnel-1.29.2.tgz
  ```
- Versions must be ordered: within each minor series, latest patch first (descending). Minor series are ordered latest first (e.g., 1.30.x before 1.29.x)

#### 10c. Update `-latest` refs
- Ensure each `vX.YY-latest` ref entry points to the actual latest stable patch version for that series

### 11. Run make gen

```bash
make gen
```

This regenerates CRDs, charts, manifests, and all other generated code based on the updated versions.

### 12. Commit the new Istio versions

- Stage and commit all changes so far so the added Istio versions are captured in a separate commit before dependency validation
- Use a descriptive commit message listing the versions added, e.g.:
  ```
  Add Istio versions 1.30.3, 1.29.6, 1.28.8
  ```
- Include the `Co-Authored-By` trailer using the current model name dynamically (do not hardcode a specific model name)
- Sign the commit with `-s` flag as required by project policy
- This makes it easy to see exactly which Istio versions were added, separate from any go.mod dependency fixes in the next step

### 13. Verify and fix go.mod dependencies

The go.mod `istio.io/*` dependencies must match the most recent Istio version in versions.yaml. Check and fix each:

#### 13a. istio.io/client-go
- Should use the actual release tag, NOT a pseudoversion
- Example: `istio.io/client-go v1.30.3` (not a pseudoversion like `v1.30.3-alpha.0.0.20240809...`)
- Update if needed:
  ```bash
  go get istio.io/client-go@v1.30.3
  ```

#### 13b. istio.io/istio
- This is special: Istio release tags do NOT have a `v` prefix, so Go cannot resolve them as normal version tags
- Must use a pseudoversion, but it MUST point to the tagged release commit
- Update by running:
  ```bash
  go get -u istio.io/istio@1.30.3
  ```
  (note: no `v` prefix in the tag)
- Verify the resulting pseudoversion (e.g., `v0.0.0-<timestamp>-<commit>`) points to the correct release commit:
  ```bash
  # The commit hash in the pseudoversion should match the tagged commit
  gh api repos/istio/istio/git/refs/tags/1.30.3 --jq '.object.sha' | head -c 12
  ```

#### 13c. istio.io/api
- May also use a pseudoversion; verify it corresponds to the correct release
- If update_deps.sh set it to a non-release version, update it:
  ```bash
  go get istio.io/api@v1.30.3
  ```

#### 13d. Run go mod tidy and make gen
```bash
go mod tidy
make gen
```

### 14. Commit go.mod dependency fixes

- If go.mod or go.sum changed in Step 13, stage and commit them separately:
  ```
  Fix istio.io/istio pseudoversion for X.YY.Z
  ```
- Include the `Co-Authored-By` trailer using the current model name dynamically (do not hardcode a specific model name)
- Sign the commit with `-s` flag as required by project policy
- If nothing changed in Step 13 (dependencies were already correct), skip this commit

### 15. Final verification for this branch

- Run `make all` to ensure everything builds, lints, and tests pass
- If any step fails, fix the issue and re-run `make all`
- **If a failure cannot be resolved**, ask the user whether to:
  1. Skip this branch and continue with the remaining branches
  2. Stop processing entirely

### 16. Push branch and create PR

- Push the temporary branch to the fork remote:
  ```bash
  git push -u <FORK_REMOTE> release-X.YY_add_istio_A.BB.CC
  ```
- Create a pull request against the upstream release branch using `gh`:
  ```bash
  gh pr create \
    --repo istio-ecosystem/sail-operator \
    --base release-X.YY \
    --head <FORK_OWNER>:release-X.YY_add_istio_A.BB.CC \
    --title "[release-X.YY] Add Istio versions A.BB.CC, ..." \
    --body "$(cat <<'EOF'
  ## Summary
  - Add missing Istio versions: <list all versions added>
  - Update VERSION to <new version>
  - Update go.mod dependencies to match latest Istio version

  Co-Authored-By: <current model name> <noreply@anthropic.com>
  EOF
  )"
  ```
- `FORK_OWNER` was determined in Step 1 from the fork remote URL
- Save the PR URL for the final summary

---

### Phase 3: Summary

---

### 17. Return to original branch and show cross-branch summary

- Switch back to the branch saved in Step 1:
  ```bash
  git checkout <original-branch>
  ```
- Show a per-branch summary including PR links, e.g.:
  ```
  === Cross-Branch Summary ===

  release-1.30:
    Branch: release-1.30_add_istio_1.30.3
    Versions added: 1.30.3, 1.29.6
    VERSION: 1.30.0 → 1.30.3
    PREVIOUS_VERSION: 1.29.2 → 1.30.0
    go.mod: istio.io/client-go v1.30.3, istio.io/istio@1.30.3
    PR: https://github.com/istio-ecosystem/sail-operator/pull/XXXX

  release-1.29:
    Branch: release-1.29_add_istio_1.29.6
    Versions added: 1.29.3, 1.29.4, 1.29.5, 1.29.6, 1.28.7, 1.28.8, 1.28.9, 1.28.10
    VERSION: 1.29.2 → 1.29.6
    PREVIOUS_VERSION: unchanged
    go.mod: istio.io/client-go v1.29.6, istio.io/istio@1.29.6
    PR: https://github.com/istio-ecosystem/sail-operator/pull/YYYY
  ```
- If any branches were skipped due to failures, list them with the reason

## Important Notes

- **Version ordering in versions.yaml**: The first entry is the default version. Within each minor series, versions are ordered from newest to oldest. Minor series are ordered from newest to oldest. EOL versions come last.
- **Chart URLs follow a strict pattern**: `https://istio-release.storage.googleapis.com/charts/<chart>-<version>.tgz` with charts: base, istiod, gateway, cni, ztunnel
- **The go.mod comment in versions.yaml** (lines 8-12) explains that istio.io/istio and istio.io/api dependencies must match the most recent version - this is critical for correct CRD generation
- **istio.io/istio tags have no v prefix**: Unlike other Go modules, istio/istio uses tags like `1.30.3` not `v1.30.3`, so pseudoversions are required in go.mod
- **Always confirm version changes with the user** before modifying Makefile.core.mk
- **Do not remove EOL entries** from versions.yaml - they must remain for API compatibility
- **Run make all at the end of each branch** to verify everything is consistent
- **Branch processing order**: Always process from highest minor version to lowest — this ensures the active branch determination is consistent
- **Clean state between branches**: Before switching branches, verify there are no uncommitted changes. If there are leftover changes from a failed step, warn the user before proceeding.
- **Upstream releases are fetched once**: The list of upstream Istio releases from GitHub is fetched once in Step 3 and reused across all branches to avoid redundant API calls
- **Remote names are detected dynamically**: The upstream remote is identified by matching the URL `istio-ecosystem/sail-operator`, not by assuming the name `upstream`. The fork remote is similarly auto-detected. This ensures the skill works regardless of how users name their remotes.
- **Temporary branches are created from the upstream remote**: Each working branch (e.g., `release-1.30_add_istio_1.30.3`) is created directly from `<UPSTREAM_REMOTE>/release-X.YY`, not from a local branch, to ensure it's based on the latest upstream code
