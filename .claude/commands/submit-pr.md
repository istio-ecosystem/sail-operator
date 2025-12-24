# Submit PR

Analyzes current git changes and automatically creates a pull request with an intelligently generated description based on commit subject, message, and changed files.

## Arguments

- `base` (optional): Base branch to compare against (default: `main`)
- `fixes` (optional): Issue number that this PR fixes (e.g., `fixes=123` for `Fixes #123`)
- `related` (optional): Related issue or PR number (e.g., `related=456` for `Related Issue/PR #456`)
- `title` (optional): Custom PR title (defaults to first commit subject)
- `draft` (optional): Create as draft PR (`true`/`false`, default: `false`)

## Steps

### 1. Prerequisites Check

Verify GitHub CLI is installed and authenticated:

```bash
echo "🔍 Checking prerequisites..."

# Check if gh CLI is installed
if command -v gh &> /dev/null; then
    echo "✅ GitHub CLI found"
else
    echo "❌ GitHub CLI (gh) is required but not installed"
    echo "Install it from: https://cli.github.com/"
    exit 1
fi

# Check if authenticated
if gh auth status &> /dev/null; then
    echo "✅ GitHub CLI authenticated"
else
    echo "❌ GitHub CLI not authenticated"
    echo "Run: gh auth login"
    exit 1
fi

echo "✅ GitHub CLI ready"
```

### 2. Parse Arguments and Extract Variables

Parse command arguments and set up variables:

```bash
echo "⚙️  Parsing arguments..."

# Extract variables from arguments
BASE_BRANCH="${base:-main}"
FIXES_ISSUE="$fixes"
RELATED_ISSUE="$related"
PR_TITLE="$title"
IS_DRAFT="${draft:-false}"

# Log what will be used
if [ -n "$FIXES_ISSUE" ]; then
    echo "📌 Will link to issue: #$FIXES_ISSUE"
fi

if [ -n "$RELATED_ISSUE" ]; then
    echo "🔗 Will reference related: #$RELATED_ISSUE"
fi

if [ "$IS_DRAFT" = "true" ]; then
    echo "📝 Will create as draft PR"
fi
```

### 3. Detect Fork Setup and Validate Branch

Automatically detect fork workflow and validate branch state:

```bash
echo "🔍 Detecting repository setup..."

# Detect repository setup (fork vs direct)
CURRENT_BRANCH=$(git branch --show-current)

# Check if we have an upstream remote (indicates fork workflow)
if git remote | grep -q "^upstream$"; then
    IS_FORK=true
    FORK_URL=$(git remote get-url origin)
    UPSTREAM_URL=$(git remote get-url upstream)

    echo "🍴 Fork workflow detected - will create PR from fork to upstream"
    echo "  Fork (origin): $FORK_URL"
    echo "  Upstream: $UPSTREAM_URL"

    # Extract fork owner from origin URL for PR head specification
    FORK_OWNER=$(echo "$FORK_URL" | sed -E 's/.*[\/:]([^\/]+)\/[^\/]+\.git?$/\1/')
    if [ -z "$FORK_OWNER" ]; then
        # Try alternative extraction for different URL formats
        FORK_OWNER=$(echo "$FORK_URL" | sed -E 's/.*github\.com[\/:]([^\/]+)\/.*/\1/')
    fi

    # Extract upstream owner/repo for targeting PR
    UPSTREAM_OWNER=$(echo "$UPSTREAM_URL" | sed -E 's/.*[\/:]([^\/]+)\/[^\/]+\.git?$/\1/')
    UPSTREAM_REPO=$(echo "$UPSTREAM_URL" | sed -E 's/.*[\/:]([^\/]+)\/([^\/]+)\.git?$/\2/' | sed 's/\.git$//')

    echo "  Fork owner: $FORK_OWNER"
    echo "  Target: $UPSTREAM_OWNER/$UPSTREAM_REPO"

    [ -z "$FORK_OWNER" ] || [ -z "$UPSTREAM_OWNER" ] || [ -z "$UPSTREAM_REPO" ] && {
        echo "❌ Failed to parse repository URLs"
        echo "Fork: $FORK_URL | Upstream: $UPSTREAM_URL"
        exit 1
    }

    # Use upstream for base branch validation
    BASE_REMOTE="upstream"
    TARGET_REPO="$UPSTREAM_OWNER/$UPSTREAM_REPO"
    FORK_REMOTE="origin"  # Fork is always on origin
else
    IS_FORK=false
    FORK_REMOTE="origin"
    BASE_REMOTE="origin"
    TARGET_REPO=""  # gh pr create will use current repo
    echo "📁 Single repository workflow - will create PR within same repo"
fi

if [ "$CURRENT_BRANCH" = "$BASE_BRANCH" ]; then
    echo "❌ Cannot create PR from $BASE_BRANCH branch"
    echo "Switch to a feature branch with your changes first"
    exit 1
fi

# Fetch latest from base remote to ensure accurate comparison
echo "📥 Fetching latest from $BASE_REMOTE..."
git fetch "$BASE_REMOTE" "$BASE_BRANCH" >/dev/null 2>&1 || echo "⚠️ Could not fetch $BASE_BRANCH from $BASE_REMOTE"

# Check if there are commits different from base branch
if [ "$IS_FORK" = true ]; then
    COMMIT_DIFF_COUNT=$(git rev-list --count HEAD ^$BASE_REMOTE/$BASE_BRANCH 2>/dev/null || echo "0")
    BASE_REF="$BASE_REMOTE/$BASE_BRANCH"
else
    COMMIT_DIFF_COUNT=$(git rev-list --count HEAD ^$BASE_BRANCH 2>/dev/null || echo "0")
    BASE_REF="$BASE_BRANCH"
fi
echo "🔍 Comparing against $BASE_REF"

[ "${COMMIT_DIFF_COUNT:-0}" -eq 0 ] && {
    echo "❌ No commits found different from $BASE_BRANCH"
    echo "Current: $(git rev-parse HEAD)"
    exit 1
}

echo "✅ Found $COMMIT_DIFF_COUNT commits ahead of $BASE_BRANCH"
```

### 4. Determine PR Type and Generate Description

Auto-detect the primary type of change based on commit messages and file patterns, then create the PR description:

```bash
echo "🏷️ Determining PR type and generating description..."

# Derive all variables needed for PR description generation
BASE_BRANCH="${base:-main}"
IS_FORK=$(git remote | grep -q "^upstream$" && echo true || echo false)
BASE_REF=$([ "$IS_FORK" = true ] && echo "upstream/$BASE_BRANCH" || echo "$BASE_BRANCH")
LATEST_COMMIT_SUBJECT=$(git log -1 --format='%s' HEAD)
LATEST_COMMIT_BODY=$(git log -1 --format='%b' HEAD | grep -v '^Signed-off-by:' | sed '/^$/d')
COMMIT_MESSAGES=$(git log --oneline $BASE_REF..HEAD)
CHANGED_FILES=$(git diff --name-only $BASE_REF...HEAD)
FILE_COUNT=$([ -z "$CHANGED_FILES" ] && echo "0" || echo "$CHANGED_FILES" | wc -l)
COMMIT_COUNT=$(echo "$COMMIT_MESSAGES" | wc -l)

# Get file type analysis for PR description
API_CHANGES=$(echo "$CHANGED_FILES" | grep -E '^api/' || echo "")
CONTROLLER_CHANGES=$(echo "$CHANGED_FILES" | grep -E '^controllers/' || echo "")
TEST_CHANGES=$(echo "$CHANGED_FILES" | grep -E '^tests?' || echo "")
DOC_CHANGES=$(echo "$CHANGED_FILES" | grep -E '\.(md|adoc)$' || echo "")
CHART_CHANGES=$(echo "$CHANGED_FILES" | grep -E '^chart/' || echo "")
VERSION_CHANGES=$(echo "$CHANGED_FILES" | grep -E 'versions\.yaml|go\.(mod|sum)' || echo "")

# Determine PR type for description
FULL_COMMIT_TEXT="$LATEST_COMMIT_SUBJECT $LATEST_COMMIT_BODY $COMMIT_MESSAGES"
if echo "$FULL_COMMIT_TEXT" | grep -qi -E '\b(fix|bug|issue|error|broken)\b'; then
    PR_TYPE="Bug Fix"
elif echo "$FULL_COMMIT_TEXT" | grep -qi -E '\b(refactor|cleanup|restructure|reorganize)\b'; then
    PR_TYPE="Refactor"
elif echo "$FULL_COMMIT_TEXT" | grep -qi -E '\b(optim|perf|performance|faster|improve)\b'; then
    PR_TYPE="Optimization"
elif [ -n "$TEST_CHANGES" ] && [ -z "$API_CHANGES" ] && [ -z "$CONTROLLER_CHANGES" ]; then
    PR_TYPE="Test"
elif [ -n "$DOC_CHANGES" ] && [ -z "$API_CHANGES" ] && [ -z "$CONTROLLER_CHANGES" ]; then
    PR_TYPE="Documentation Update"
else
    PR_TYPE="Enhancement / New Feature"
fi

# Get argument variables needed for description
FIXES_ISSUE="$fixes"
RELATED_ISSUE="$related"
PR_TITLE="${title:-$LATEST_COMMIT_SUBJECT}"

# Set fallback values
PR_TYPE="${PR_TYPE:-Enhancement / New Feature}"
FILE_COUNT="${FILE_COUNT:-0}"
COMMIT_COUNT="${COMMIT_COUNT:-0}"
LATEST_COMMIT_SUBJECT="${LATEST_COMMIT_SUBJECT:-No subject}"
LATEST_COMMIT_BODY="${LATEST_COMMIT_BODY:-}"

echo "📊 $FILE_COUNT files, $COMMIT_COUNT commits | Type: $PR_TYPE"

# Generate PR description efficiently
build_checkboxes() {
    local types=("Enhancement / New Feature" "Bug Fix" "Refactor" "Optimization" "Test" "Documentation Update")
    for type in "${types[@]}"; do
        if [ "$PR_TYPE" = "$type" ]; then
            echo "- [x] $type"
        else
            echo "- [ ] $type"
        fi
    done
}
PR_CHECKBOXES=$(build_checkboxes)

# Build the PR description
PR_DESCRIPTION="#### What type of PR is this?

$PR_CHECKBOXES

#### What this PR does / why we need it:"

# Use commit body as primary content, fallback to subject if body is empty
if [ -n "$LATEST_COMMIT_BODY" ]; then
    PR_DESCRIPTION="$PR_DESCRIPTION

$LATEST_COMMIT_BODY"
else
    PR_DESCRIPTION="$PR_DESCRIPTION

$LATEST_COMMIT_SUBJECT"
fi

# Add file context for multi-commit PRs
if [ "${COMMIT_COUNT:-0}" -gt 1 ]; then
    PR_DESCRIPTION="$PR_DESCRIPTION

**Multiple commits ($COMMIT_COUNT):**
$(echo "$COMMIT_MESSAGES" | sed 's/^/- /')"
fi

PR_DESCRIPTION="$PR_DESCRIPTION

#### Which issue(s) this PR fixes:"

# Handle issue references
if [ -n "$FIXES_ISSUE" ]; then
    PR_DESCRIPTION="$PR_DESCRIPTION

Fixes #$FIXES_ISSUE"
else
    PR_DESCRIPTION="$PR_DESCRIPTION

Fixes #"
fi

if [ -n "$RELATED_ISSUE" ]; then
    PR_DESCRIPTION="$PR_DESCRIPTION

Related Issue/PR #$RELATED_ISSUE"
else
    PR_DESCRIPTION="$PR_DESCRIPTION

Related Issue/PR #"
fi

# Save PR description to temp file for next block
PR_DESC_FILE="/tmp/sail-operator-pr-description-$(git rev-parse --short HEAD)"
echo "$PR_DESCRIPTION" > "$PR_DESC_FILE"
echo "📝 PR description saved to temp file for creation step"
```

### 5. Create the Pull Request

Use GitHub CLI to create the PR with fork-aware configuration:

```bash
echo "🚀 Creating pull request..."

# Derive only variables needed for repository setup and PR creation
BASE_BRANCH="${base:-main}"
CURRENT_BRANCH=$(git branch --show-current)
IS_DRAFT="${draft:-false}"
PR_TITLE="${title:-$(git log -1 --format='%s' HEAD)}"

# Detect fork setup for PR targeting
if git remote | grep -q "^upstream$"; then
    IS_FORK=true

    # Extract repository URLs
    UPSTREAM_URL=$(git remote get-url upstream)
    FORK_URL=$(git remote get-url origin)

    # Parse fork owner for PR head
    FORK_OWNER=$(echo "$FORK_URL" | sed -E 's/.*[\/:]([^\/]+)\/[^\/]+\.git?$/\1/')
    if [ -z "$FORK_OWNER" ]; then
        FORK_OWNER=$(echo "$FORK_URL" | sed -E 's/.*github\.com[\/:]([^\/]+)\/.*/\1/')
    fi

    # Parse upstream repo for PR target
    UPSTREAM_OWNER=$(echo "$UPSTREAM_URL" | sed -E 's/.*[\/:]([^\/]+)\/[^\/]+\.git?$/\1/')
    UPSTREAM_REPO=$(echo "$UPSTREAM_URL" | sed -E 's/.*[\/:]([^\/]+)\/([^\/]+)\.git?$/\2/' | sed 's/\.git$//')
    TARGET_REPO="$UPSTREAM_OWNER/$UPSTREAM_REPO"
else
    IS_FORK=false
    TARGET_REPO=""
fi

# Read PR description from temp file created in previous block
PR_DESC_FILE="/tmp/sail-operator-pr-description-$(git rev-parse --short HEAD)"
if [ ! -f "$PR_DESC_FILE" ]; then
    echo "❌ PR description file not found: $PR_DESC_FILE"
    echo "Previous step may have failed."
    exit 1
fi

# Check for existing PR on current branch
echo "🔍 Checking for existing PRs on branch $CURRENT_BRANCH..."

if [ "$IS_FORK" = "true" ]; then
    # For forks, check in the upstream repo
    EXISTING_PR_INFO=$(gh pr list --repo "$TARGET_REPO" --head "$CURRENT_BRANCH" --state open --json number,title,url --jq '.[0] // empty' 2>/dev/null || echo "")
else
    # For direct repos, check current repo
    EXISTING_PR_INFO=$(gh pr list --head "$CURRENT_BRANCH" --state open --json number,title,url --jq '.[0] // empty' 2>/dev/null || echo "")
fi

if [ -n "$EXISTING_PR_INFO" ]; then
    EXISTING_PR_NUMBER=$(echo "$EXISTING_PR_INFO" | jq -r '.number // empty' 2>/dev/null || echo "")
    EXISTING_PR_TITLE=$(echo "$EXISTING_PR_INFO" | jq -r '.title // "Unknown title"' 2>/dev/null || echo "Unknown title")
    EXISTING_PR_URL=$(echo "$EXISTING_PR_INFO" | jq -r '.url // "Unknown URL"' 2>/dev/null || echo "Unknown URL")

    if [ -n "$EXISTING_PR_NUMBER" ] && [ "$EXISTING_PR_NUMBER" != "null" ]; then
        echo ""
        echo "⚠️  Found existing PR #$EXISTING_PR_NUMBER for branch '$CURRENT_BRANCH'"
        echo "   Title: \"$EXISTING_PR_TITLE\""
        echo "   URL: $EXISTING_PR_URL"
        echo ""
        echo "Choose action:"
        echo "1) Update existing PR #$EXISTING_PR_NUMBER (recommended)"
        echo "2) Create new PR anyway"
        echo "3) Cancel and review existing PR first"
        echo ""

        # Get user choice with timeout
        read -t 30 -p "Choice [1]: " CHOICE
        CHOICE=${CHOICE:-1}

        case $CHOICE in
            1)
                echo ""
                echo "✅ Updating existing PR #$EXISTING_PR_NUMBER..."

                # Push branch updates first
                echo "📤 Pushing branch updates..."
                FORK_REMOTE=$([ "$IS_FORK" = "true" ] && echo "origin" || echo "origin")

                if git push --force-with-lease "$FORK_REMOTE" "$CURRENT_BRANCH" >/dev/null 2>&1; then
                    echo "✅ Pushed updates to $FORK_REMOTE"
                else
                    echo "❌ Failed to push branch updates"
                    rm -f "$PR_DESC_FILE"
                    exit 1
                fi

                # Prepare gh pr edit arguments
                EDIT_ARGS=("$EXISTING_PR_NUMBER" "--title" "$PR_TITLE" "--body-file" "$PR_DESC_FILE")

                if [ "$IS_FORK" = "true" ]; then
                    EDIT_ARGS=("--repo" "$TARGET_REPO" "${EDIT_ARGS[@]}")
                fi

                # Update the existing PR
                if gh pr edit "${EDIT_ARGS[@]}" 2>&1; then
                    echo "✅ Pull request #$EXISTING_PR_NUMBER updated successfully!"
                    echo "🔗 $EXISTING_PR_URL"
                    echo "📝 Updated PR #$EXISTING_PR_NUMBER: $PR_TITLE"

                    if [ "$IS_FORK" = "true" ]; then
                        echo "🍴 Cross-repository PR: $FORK_OWNER:$CURRENT_BRANCH → $TARGET_REPO:$BASE_BRANCH"
                    fi

                    # Clean up temp file on success
                    rm -f "$PR_DESC_FILE"
                    exit 0
                else
                    echo "❌ Failed to update PR #$EXISTING_PR_NUMBER"
                    rm -f "$PR_DESC_FILE"
                    exit 1
                fi
                ;;
            2)
                echo ""
                echo "📝 Proceeding to create new PR..."
                ;;
            3)
                echo ""
                echo "🚫 Cancelled. Review existing PR: $EXISTING_PR_URL"
                rm -f "$PR_DESC_FILE"
                exit 0
                ;;
            *)
                echo ""
                echo "❌ Invalid choice. Cancelled."
                rm -f "$PR_DESC_FILE"
                exit 1
                ;;
        esac
    fi
else
    echo "✅ No existing PR found for branch $CURRENT_BRANCH"
    echo "📝 Will create new PR"
fi

# Push branch for new PR creation (consolidated for both scenarios)
echo "📤 Pushing branch for new PR..."
FORK_REMOTE=$([ "$IS_FORK" = "true" ] && echo "origin" || echo "origin")

if git push -u "$FORK_REMOTE" "$CURRENT_BRANCH" >/dev/null 2>&1; then
    echo "✅ Pushed to $FORK_REMOTE"
else
    echo "❌ Failed to push branch"
    rm -f "$PR_DESC_FILE"
    exit 1
fi

# Prepare gh pr create arguments based on workflow type
GH_ARGS=("--title" "$PR_TITLE" "--body-file" "$PR_DESC_FILE" "--base" "$BASE_BRANCH")

if [ "$IS_FORK" = true ]; then
    # For forks: use fork-owner:branch format
    GH_ARGS+=("--head" "$FORK_OWNER:$CURRENT_BRANCH")

    # Target the upstream repository
    if [ -n "$TARGET_REPO" ]; then
        GH_ARGS+=("--repo" "$TARGET_REPO")
    fi
else
    # For direct repo: just use branch name
    GH_ARGS+=("--head" "$CURRENT_BRANCH")
fi

[ "$IS_DRAFT" = "true" ] && GH_ARGS+=("--draft")

# Create the PR
if PR_URL=$(gh pr create "${GH_ARGS[@]}" 2>&1); then
    PR_NUMBER=$(echo "$PR_URL" | grep -o '[0-9]\+$' || echo "unknown")

    echo "✅ Pull request created successfully!"
    echo "🔗 $PR_URL"
    echo "📝 PR #$PR_NUMBER: $PR_TITLE"

    if [ "$IS_FORK" = true ]; then
        echo "🍴 Cross-repository PR: $FORK_OWNER/$UPSTREAM_REPO:$CURRENT_BRANCH → $TARGET_REPO:$BASE_BRANCH"
    fi

    # Clean up temp file on success
    rm -f "$PR_DESC_FILE"

else
    echo "❌ PR creation failed: $PR_URL"
    echo ""
    echo "🔧 Debugging Information:"
    echo "Workflow: $([ "$IS_FORK" = true ] && echo "Fork" || echo "Direct repository")"
    echo ""
    echo "Branch: $CURRENT_BRANCH"
    echo "Remote: $(git remote -v | head -2)"
    echo ""
    echo "🚨 Try:"
    echo "1. Check auth: gh auth status"
    echo "2. Manual PR: gh pr create --web$([ "$IS_FORK" = true ] && echo " --repo $TARGET_REPO" || echo "")"
    echo ""
    echo "🚨 Common fixes:"
    if [ "$IS_FORK" = true ]; then
        echo "1. Ensure fork is up to date: git fetch upstream && git rebase upstream/$BASE_BRANCH"
        echo "2. Check fork owner extraction: echo '$FORK_OWNER' (should be your GitHub username)"
        echo "3. Verify upstream setup: git remote -v | grep upstream"
        echo "4. Check GitHub auth for both repos: gh auth status"
        echo "5. Try manual PR: gh pr create --web --repo $TARGET_REPO"
    else
        echo "1. Ensure you have commits: git log --oneline $BASE_BRANCH..HEAD"
        echo "2. Check remote exists: git remote -v"
        echo "3. Verify GitHub auth: gh auth status"
        echo "4. Try manual PR: gh pr create --web"
    fi
    exit 1
fi

# Clean up temp file
rm -f "$PR_DESC_FILE"
```

### 6. Success Summary

Display final summary of the created PR:

```bash
echo ""
echo "🎉 Pull Request Creation Complete!"
echo "=================================="
echo ""

# Derive only variables needed for summary display
BASE_BRANCH="${base:-main}"
CURRENT_BRANCH=$(git branch --show-current)
IS_FORK=$(git remote | grep -q "^upstream$" && echo true || echo false)
BASE_REF=$([ "$IS_FORK" = true ] && echo "upstream/$BASE_BRANCH" || echo "$BASE_BRANCH")

# Get analysis data for summary
CHANGED_FILES=$(git diff --name-only $BASE_REF...HEAD)
FILE_COUNT=$([ -z "$CHANGED_FILES" ] && echo "0" || echo "$CHANGED_FILES" | wc -l)
COMMIT_COUNT=$(git rev-list --count $BASE_REF..HEAD)
LATEST_COMMIT_SUBJECT=$(git log -1 --format='%s' HEAD)

# Get PR type for summary (simplified analysis)
COMMIT_MESSAGES=$(git log --oneline $BASE_REF..HEAD)
LATEST_COMMIT_BODY=$(git log -1 --format='%b' HEAD | grep -v '^Signed-off-by:' | sed '/^$/d')
FULL_COMMIT_TEXT="$LATEST_COMMIT_SUBJECT $LATEST_COMMIT_BODY $COMMIT_MESSAGES"

# File type analysis for PR type detection
API_CHANGES=$(echo "$CHANGED_FILES" | grep -E '^api/' || echo "")
CONTROLLER_CHANGES=$(echo "$CHANGED_FILES" | grep -E '^controllers/' || echo "")
TEST_CHANGES=$(echo "$CHANGED_FILES" | grep -E '^tests?' || echo "")
DOC_CHANGES=$(echo "$CHANGED_FILES" | grep -E '\.(md|adoc)$' || echo "")

# Determine PR type
if echo "$FULL_COMMIT_TEXT" | grep -qi -E '\b(fix|bug|issue|error|broken)\b'; then
    PR_TYPE="Bug Fix"
elif echo "$FULL_COMMIT_TEXT" | grep -qi -E '\b(refactor|cleanup|restructure|reorganize)\b'; then
    PR_TYPE="Refactor"
elif echo "$FULL_COMMIT_TEXT" | grep -qi -E '\b(optim|perf|performance|faster|improve)\b'; then
    PR_TYPE="Optimization"
elif [ -n "$TEST_CHANGES" ] && [ -z "$API_CHANGES" ] && [ -z "$CONTROLLER_CHANGES" ]; then
    PR_TYPE="Test"
elif [ -n "$DOC_CHANGES" ] && [ -z "$API_CHANGES" ] && [ -z "$CONTROLLER_CHANGES" ]; then
    PR_TYPE="Documentation Update"
else
    PR_TYPE="Enhancement / New Feature"
fi

# Get repository info for fork workflow display
if [ "$IS_FORK" = true ]; then
    UPSTREAM_URL=$(git remote get-url upstream)
    FORK_URL=$(git remote get-url origin)
    FORK_OWNER=$(echo "$FORK_URL" | sed -E 's/.*[\/:]([^\/]+)\/[^\/]+\.git?$/\1/')
    if [ -z "$FORK_OWNER" ]; then
        FORK_OWNER=$(echo "$FORK_URL" | sed -E 's/.*github\.com[\/:]([^\/]+)\/.*/\1/')
    fi
    UPSTREAM_OWNER=$(echo "$UPSTREAM_URL" | sed -E 's/.*[\/:]([^\/]+)\/[^\/]+\.git?$/\1/')
    UPSTREAM_REPO=$(echo "$UPSTREAM_URL" | sed -E 's/.*[\/:]([^\/]+)\/([^\/]+)\.git?$/\2/' | sed 's/\.git$//')
    TARGET_REPO="$UPSTREAM_OWNER/$UPSTREAM_REPO"
fi

# Display summary
PR_TITLE="${title:-$LATEST_COMMIT_SUBJECT}"

echo "📝 Title: $PR_TITLE"
echo "🏷️  Type: $PR_TYPE"

if [ "$IS_FORK" = true ]; then
    echo "🍴 Workflow: Fork (cross-repository)"
    echo "🌿 Source: $FORK_OWNER:$CURRENT_BRANCH"
    echo "🎯 Target: $TARGET_REPO:$BASE_BRANCH"
else
    echo "📁 Workflow: Direct repository"
    echo "🌿 Branch: $CURRENT_BRANCH → $BASE_BRANCH"
fi

echo "📊 Changes: $FILE_COUNT files, $COMMIT_COUNT commits"
echo "🔗 URL: $PR_URL"
echo ""

# Show what was automatically detected and included
echo "🔍 Auto-detected content:"
if [ -n "$FIXES_ISSUE" ]; then
    echo "   ✅ Links to issue #$FIXES_ISSUE"
fi

echo "✅ Ready for review!"
```

## Error Handling

**If GitHub CLI not installed:**
```bash
❌ GitHub CLI (gh) is required but not installed
Install it from: https://cli.github.com/
```

**If GitHub CLI not authenticated:**
```bash
❌ GitHub CLI not authenticated
Run: gh auth login
```

**If not in a git repository:**
```bash
❌ Not in a git repository
This command must be run from within the sail-operator repository.
```

**If on main/base branch:**
```bash
❌ Cannot create PR from main branch
Switch to a feature branch with your changes first
```

**If no changes to analyze:**
```bash
❌ No commits found different from main branch
Make some commits on your feature branch first
```

**If PR creation fails:**
```bash
❌ Failed to create pull request
GitHub CLI Error: [specific error from gh CLI]

Debugging steps:
1. Verify commits exist: git log --oneline main..HEAD
2. Check branch is pushed: git branch -vv
3. Test GitHub auth: gh auth status
4. Check existing PRs: gh pr list --head [branch]
5. Try manual creation: gh pr create --web
6. Verify remote tracking: git remote -v
```

## Features

- **🍴 Fork workflow**: Auto-detects and handles fork to upstream PRs
- **🔄 Smart PR handling**: Auto-detects existing PRs and prompts for update vs create new
- **🧠 Smart analysis**: Detects PR type from commits and file patterns
- **📝 Auto-generation**: Creates comprehensive PR descriptions from commits
- **🔗 Issue linking**: Links issues via arguments or commit message parsing
- **✅ Testing checklists**: Adds relevant test checkboxes based on changes
- **🚀 One command**: Complete PR creation/update with push and description

## Usage Examples

```bash
# Create PR with automatic analysis of commits
# (Auto-detects existing PRs and prompts for update/create decision)
/submit-pr

# Create PR linking to a specific issue
/submit-pr fixes=123

# Create PR with custom title
/submit-pr title="Add new authentication method"

# Create draft PR
/submit-pr draft=true

# Create PR against different base branch
/submit-pr base=develop

# Create PR with issue links and custom title
/submit-pr fixes=123 related=456 title="Fix authentication bug"

# Create draft PR against develop branch
/submit-pr base=develop draft=true fixes=789

# Complex example with all options
/submit-pr base=release-1.0 fixes=123 related=456 title="Security fix" draft=true
```

**Note:** If an existing PR is found for the current branch, you'll be prompted with options:
- Update existing PR (recommended) - updates title and description
- Create new PR anyway - creates a second PR from same branch
- Cancel and review - exits to let you review the existing PR

## Notes

- **Prerequisites**: GitHub CLI (`gh`) must be installed and authenticated
- **Repository**: Works with both fork and direct repository workflows
- **Fork Detection**: Automatically detects fork by checking for `upstream` remote
- **Branch**: Run from your feature branch, not main
- **Commits**: Ensure you have committed your changes before running
- **Commit messages**: Write descriptive commit subjects - they become the PR title
- **Issue linking**: Use `fixes=123` to automatically close issues when merged
- **Review**: Always review the generated PR description before requesting reviews
- **Testing**: Run suggested tests locally before marking the PR ready for review
