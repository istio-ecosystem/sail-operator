---
description: Cherry-pick a failed automated cherry-pick from a bot-created issue
argument-hint: "<issue-number>"
---

# Cherry-Pick Fix

Automates resolving failed cherry-pick issues created by the cherry-pick bot. Parses the issue, cherry-picks the original commit onto the target release branch, intelligently resolves conflicts, runs tests, and pauses for review before pushing and creating a PR.

## Prerequisites

- `gh` CLI must be installed and authenticated with permission to push to the user's fork.

## Argument

- `<issue-number>`: Required. The GitHub issue number created by the cherry-pick bot (e.g. `1928`).

## Fork Remote Configuration

Before pushing, the skill needs to know which git remote points to the user's fork. On first run (or if not yet configured), ask the user which remote is their fork and save the answer to the auto-memory file `cherry_pick_fork_remote.md` with type `reference`. On subsequent runs, read this memory file to get the fork remote name.

## Implementation

### 1. Parse the issue

Fetch the issue from GitHub using `gh issue view <issue-number>` with JSON output. Extract:

- **Target branch**: from the issue body, look for the branch name in the phrase `failed to apply on top of branch "<branch>"` (e.g. `release-1.29`).
- **Original PR number**: from the issue body, look for `#<number>` referencing the original PR (e.g. `#1891`).
- **Issue title**: used later for the cherry-pick PR title.

Then fetch the original PR using `gh pr view <pr-number>` with JSON output to get the **merge commit SHA**.

Validate all three values were extracted. If any are missing, report what was found and what's missing, then stop.

### 2. Set up a worktree

Use the `EnterWorktree` tool to create an isolated worktree. This avoids disturbing the user's current branch.

After entering the worktree, fetch the target branch from upstream and reset the worktree to it:

```bash
git fetch upstream <target-branch>
git checkout -B <target-branch> upstream/<target-branch>
```

Create a new branch for the cherry-pick:

```bash
git checkout -b cherry-pick-<original-pr-number>-<target-branch>
```

### 3. Attempt the cherry-pick

Run `git cherry-pick <merge-commit-sha>`.

**If it succeeds with no conflicts**: skip to step 5.

**If it fails with conflicts**: proceed to step 4.

### 4. Resolve conflicts

For each conflicting file (identified from `git diff --name-only --diff-filter=U`):

1. **Read the conflicted file** to see the conflict markers.
2. **Analyze the conflict** by understanding both sides:
   - The HEAD side (target release branch) represents the current state of that branch.
   - The cherry-picked side represents changes from main that may reference APIs, imports, or code that doesn't exist on the release branch.
3. **Resolve intelligently** by applying these principles:
   - **Keep release-branch dependencies**: If main has upgraded a dependency (e.g. Helm v3 to v4), keep the release branch's version and adapt the cherry-picked code accordingly.
   - **Drop unrelated additions**: If the cherry-picked diff includes code from other PRs that were merged to main before this commit (e.g. TLSConfig tests, new features), drop those sections. Only keep changes that are part of the original PR being cherry-picked.
   - **Adapt API differences**: If the cherry-picked code uses types or APIs that differ between branches (e.g. `StatusCondition` vs `IstioRevisionCondition`), use the release branch's version.
   - **Preserve the intent**: The goal is to apply the same logical fix/feature, adapted to the release branch's codebase.
4. **Verify resolution**: After editing, check that no conflict markers (`<<<<<<<`, `=======`, `>>>>>>>`) remain in any file.
5. **Stage resolved files**: `git add` each resolved file.
6. **Continue the cherry-pick**: `git cherry-pick --continue`.

If any helper functions or utilities were added in the original commit but depend on code that's also added in the commit, make sure they are included. Check for undefined references.

### 5. Generate, test, and lint

First run `make gen` to regenerate any generated code (CRDs, deepcopy, etc.) that may need updating after the cherry-pick. Then run `make test` to verify the cherry-pick doesn't break anything. This runs both unit tests and integration tests. Then run `make lint` to ensure the code passes linting checks.

- **If all pass**: proceed to step 6.
- **If any fail**: analyze the failure, fix the issue, amend the commit, and re-run. Repeat until all pass or report the failure to the user if it can't be resolved.

### 6. Pause for review

Show the user a summary of what was done:

- The original PR and commit that was cherry-picked.
- The target branch.
- Which files had conflicts and how they were resolved (brief summary of adaptations made).
- Test results.
- The diff of the cherry-pick commit (`git show --stat` and `git diff upstream/<target-branch>..HEAD`).

Then ask the user: "Ready to push and create the PR?" with options:
- **Push and create PR**: proceed to step 7.
- **Let me review first**: stop and let the user inspect. They can re-invoke or manually continue.

### 7. Push and create PR

1. **Determine fork remote**: Read from memory file `cherry_pick_fork_remote.md`. If not found, ask the user which remote is their fork and save it.

2. **Push the branch**:
   ```bash
   git push <fork-remote> cherry-pick-<original-pr-number>-<target-branch>
   ```
   **IMPORTANT: NEVER push directly to the upstream repo. Always push to the user's fork.** If the push fails for any reason (token scope, authentication, etc.), stop and ask the user what they want to do. Do not attempt alternative remotes or workarounds without explicit user approval.

3. **Determine upstream repo**: Extract the upstream GitHub repo from `git remote get-url upstream` (e.g. `istio-ecosystem/sail-operator`).

4. **Get the fork owner**: Extract from the fork remote URL.

5. **Create the PR**:

   Attempt to create the PR directly:
   ```bash
   gh pr create \
     --repo <upstream-repo> \
     --base <target-branch> \
     --head <fork-owner>:cherry-pick-<original-pr-number>-<target-branch> \
     --title "<issue-title>" \
     --body "$(cat <<'EOF'
   ## Summary
   Cherry-pick of #<original-pr-number> to <target-branch>.

   <brief description of what the original PR does, from the PR body>

   <if there were conflicts, explain what was adapted>

   Fixes #<issue-number>
   EOF
   )"
   ```

   If the PR creation fails (e.g. due to token permissions), ask the user how they'd like to proceed. Do not retry with different parameters without user approval.

6. **Report the PR URL** to the user.

### 8. Clean up

After the branch has been pushed to the fork, immediately remove the worktree using `ExitWorktree` with action `remove` and `discard_changes: true`. The work is safely on the remote fork, so there is no need to keep the worktree or ask the user.

## Error Handling

- **Issue not found or wrong format**: Report what was expected and what was found. The bot issues have a specific format; if the issue doesn't match, tell the user.
- **Cherry-pick fails with non-content conflicts** (e.g. file deletions, renames): Report the situation and let the user decide.
- **Tests fail after resolution**: Show the failure output and ask the user how to proceed.
- **Push fails**: Ask the user what they want to do. NEVER fall back to pushing to upstream or any other remote automatically.

## Example Usage

```
/cherry-pick 1928
```

This will:
1. Parse issue #1928 to find original PR #1891 and target branch `release-1.29`
2. Create a worktree and cherry-pick commit `a484fdb5` onto `release-1.29`
3. Resolve any conflicts (adapting imports, APIs, tests to the release branch)
4. Run `make test`
5. Show a summary and wait for approval
6. Push and create a PR that fixes #1928
