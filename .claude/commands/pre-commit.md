# Pre-Commit Check

Run linting, tests, and generation checks to validate changes before committing.

## Steps

### 1. Check for Uncommitted Changes

Run:
```bash
git status --short
```

Show which files have been modified to give context for what's being validated.

### 2. Run Linting

Run:
```bash
make lint
```

This runs all linters including:
- Go linting (golangci-lint)
- YAML linting
- Helm chart linting
- Script linting
- License and copyright checks
- Secret scanning (gitleaks)
- Spell checking

**If linting fails:**

1. First, analyze the lint errors to understand what needs fixing
2. Attempt to fix the issues:
   - For Go lint errors: Read the affected files and apply fixes directly
   - For formatting issues: Run `go fmt` on affected files
   - For import ordering: Run `goimports` if available, or fix manually
   - For simple issues (unused variables, missing error checks, etc.): Fix the code directly
3. After applying fixes, re-run `make lint` to verify
4. If fixes were applied, inform the user what was changed
5. Only stop and report if issues cannot be automatically fixed (e.g., complex logic errors, design issues)

### 3. Run Unit Tests

Run:
```bash
make test.unit
```

This runs all unit tests in the repository.

If tests fail, show the failures and stop. Suggest running specific failing tests with verbose output for debugging.

### 4. Run Generation Check

Run:
```bash
make gen-check
```

This verifies that all generated files (CRDs, API docs, code) are up-to-date with the source.

**If gen-check fails:**

1. Run `make gen` to regenerate all files
2. Run `make gen-check` again to verify the fix worked
3. Show the user which files were regenerated using `git status --short`
4. Inform the user these generated files need to be staged with their commit

### 5. Show Summary

Display a summary of results:

```
Pre-Commit Check Complete
=========================

Modified Files: [count]
Lint: Passed
Unit Tests: Passed
Gen Check: Passed

Ready to commit!

Next steps:
1. Stage your changes: git add <files>
2. Commit with sign-off: git commit -s -m "Your message"
```

## Error Handling

If any step fails, stop and show:

```
Pre-Commit Check Failed
=======================

Failed at: [Lint/Unit Tests/Gen Check]

Error:
[show error output]

How to fix:
[provide specific guidance based on the failure type]
```

### Common Fixes

**Lint failures:**
- Go lint: Review the specific linter message and fix the code
- Run `make lint-go` to re-check just Go files
- Some issues can be auto-fixed: `golangci-lint run --fix`

**Test failures:**
- Run failing test with verbose output: `go test -v ./path/to/package -run TestName`
- Check test logs for assertion failures

**Gen-check failures:**
- Run `make gen` to regenerate all files
- Stage the generated changes with your commit
- Common cause: API type changes without running generation

## Notes

- This command does NOT run integration or e2e tests (those require a cluster)
- For a full validation including integration tests, run `make test`
- Always sign commits with `-s` flag as required by the project