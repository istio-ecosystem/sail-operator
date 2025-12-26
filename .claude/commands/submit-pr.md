# Submit PR

Analyzes current git changes and automatically creates a pull request with an intelligently generated description based on commit subject, message, and changed files.

## Arguments

- `base` (optional): Base branch to compare against (default: `main`)
- `fixes` (optional): Issue number that this PR fixes (e.g., `fixes=123`)
- `related` (optional): Related issue or PR number (e.g., `related=456`)
- `title` (optional): Custom PR title (defaults to latest commit subject)
- `draft` (optional): Create as draft PR (`true`/`false`, default: `false`)

## Implementation

The command performs a systematic analysis of the current branch and repository state to create or update pull requests intelligently. It automatically detects fork workflows, generates appropriate PR descriptions based on commit content and file patterns, and handles existing PR updates gracefully.

### Workflow Detection and Repository Analysis

The command begins by analyzing the repository structure to determine the workflow type (fork vs direct) and validate the current state for PR creation.

**Repository Type Detection:**
The system checks for an upstream remote to identify fork workflows. Fork repositories require different targeting strategies since PRs must be created against the upstream repository rather than the fork itself.

```bash
# Detect repository workflow
git remote show upstream
```

**Branch Validation:**
Ensures the current branch contains meaningful changes and isn't the base branch. The command validates that commits exist between the current branch and the intended base branch.

```bash
# Validate branch state
git rev-list --count HEAD ^<base-branch>
```

**Prerequisites Verification:**
Confirms GitHub CLI availability and authentication status before attempting PR operations.

```bash
# Verify GitHub CLI setup
gh auth status
```

### Intelligent Content Analysis

The command analyzes commit messages and changed files to automatically categorize the PR and generate appropriate descriptions.

**Commit Analysis:**
Examines the latest commit subject and body, along with the full commit history since the base branch. This provides context for both the primary change description and multi-commit summaries.

**File Pattern Recognition:**
Analyzes changed files to detect the nature of modifications:
- API changes (`api/` directory patterns)
- Controller modifications (`controllers/` directory patterns)
- Test additions (`tests/` or `_test.go` patterns)
- Documentation updates (`.md`, `.adoc` file patterns)

**Type Classification:**
Uses commit message keywords and file patterns to automatically classify PRs:
- **Enhancement / New Feature**: Default classification for new features and improvements
- **Bug Fix**: Detects keywords like "fix", "bug", "error" in commit messages
- **Refactor**: Identifies code restructuring with keywords like "refactor", "cleanup", "restructure"
- **Optimization**: Detects performance improvements with keywords like "optimize", "performance", "faster"
- **Test**: Identifies test-only changes with no production code modifications
- **Documentation**: Recognizes documentation-only updates

```bash
# Analyze commit content
git log -1 --format='%s %b' HEAD
git diff --name-only <base-branch>...HEAD
```

### Existing PR Detection and Conflict Resolution

Before creating new PRs, the command searches for existing open PRs on the current branch to prevent duplicates and enable updates.

**PR Discovery:**
Uses GitHub CLI to query for existing open PRs targeting the current branch. The command uses a consistent head reference format across both fork and direct repository workflows.

```bash
# Check for existing PRs
gh pr list --head <current-branch> --state open
```

**Fork-Aware Queries:**
For fork workflows, the command targets the upstream repository for the search while maintaining the same head reference format, allowing GitHub to automatically resolve the appropriate fork context.

**Update Decision Process:**
When existing PRs are found, the command provides a 20-second countdown allowing users to abort before automatically proceeding with updates. This balance provides automation while preserving user control for edge cases.

### PR Description Generation

Generates comprehensive PR descriptions using a structured template that includes type classification, content analysis, and issue linking.

**Template Structure:**
- **Type Classification**: Checkbox list with automatic selection based on analysis
- **Description Content**: Primary content from commit body or subject as fallback
- **Multi-commit Context**: Additional context for PRs containing multiple commits
- **Issue References**: Integration with `fixes` and `related` arguments for automated issue linking

**Content Prioritization:**
The system prioritizes commit body content for descriptions, falling back to commit subjects when detailed commit messages aren't available. For multi-commit PRs, it provides a summary list of all included commits.

### PR Creation and Update Execution

The final step executes the appropriate action based on the analysis results, handling both creation and update scenarios.

**Branch Synchronization:**
For both new PRs and updates, the command ensures the remote branch reflects current local state using appropriate push strategies based on the situation.

**Fork Workflow Handling:**
Fork repositories require specific targeting with head specifications including the fork owner reference and targeting the upstream repository for PR creation.

```bash
# Create PR with fork awareness
gh pr create --repo <upstream-repo> --head <fork-owner>:<branch> --base <base-branch>
```

**Update Operations:**
For existing PRs, the command updates both the remote branch content and PR metadata (title and description) to reflect current state.

```bash
# Update existing PR
git push --force-with-lease origin <current-branch>
gh pr edit <pr-number> --title "<title>" --body-file <description-file>
```

**Error Handling:**
Provides clear feedback for common failure scenarios and debugging guidance for manual resolution when automated processes encounter issues.

## Features

- **Intelligent Workflow Detection**: Automatically identifies fork vs direct repository workflows
- **Smart Content Analysis**: Categorizes PRs based on commit messages and file patterns
- **Existing PR Management**: Detects and updates existing PRs with user confirmation
- **Comprehensive Descriptions**: Auto-generates structured PR descriptions with type classification
- **Issue Integration**: Links PRs to issues using `fixes` and `related` arguments
- **Fork-Aware Operations**: Handles cross-repository PR creation seamlessly

## Usage Examples

```bash
# Basic PR creation with automatic analysis
/submit-pr

# PR with issue linking
/submit-pr fixes=123

# Custom title and related issue
/submit-pr title="Add authentication feature" related=456

# Draft PR against different base
/submit-pr draft=true base=develop
```

## Error Scenarios

**GitHub CLI Authentication:**
If GitHub CLI isn't authenticated, the command will fail with clear instructions to run `gh auth login`.

**No Commits Available:**
When the current branch has no commits ahead of the base branch, the command exits with guidance to make commits first.

**Branch Validation:**
Attempting to create PRs from the base branch (e.g., `main`) results in an error with instructions to switch to a feature branch.

**Network Issues:**
Remote fetch failures are handled gracefully with fallback behavior where possible, and clear error messages for manual resolution when required.
