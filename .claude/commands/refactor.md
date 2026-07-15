# Refactor Code

This command helps refactor code while maintaining quality, testability, and adherence to project conventions.

## Tasks

1. **Understand the current code**:
   - Read the file(s) or code sections to be refactored
   - Understand the current functionality and behavior
   - Identify any existing tests that cover this code
   - Note any dependencies and usage patterns

2. **Identify refactoring opportunities**:
   - Look for code smells (duplication, complex functions, unclear naming, etc.)
   - Check for violations of SOLID principles or Go best practices
   - Identify areas that could benefit from better separation of concerns
   - Consider testability improvements
   - Look for opportunities to reduce complexity
   - Check side effects in the current code that might be lost if logic is moved

3. **Plan the refactoring**:
   - Define what will be changed and why
   - Ensure the refactoring maintains existing functionality (no behavior changes)
   - Consider backwards compatibility and API stability
   - Plan how to verify the refactoring doesn't break anything
   - For significant refactorings, consider breaking into smaller steps

4. **Execute the refactoring**:
   - Make the code changes following Go best practices
   - Follow the Sail Operator code conventions:
     - Use descriptive variable and function names
     - Keep functions focused and small
     - Separate business logic from Kubernetes controller logic when possible
     - Use interfaces for testability
     - Add error handling where appropriate
   - Preserve existing comments that are still relevant
   - Update or add comments only where the logic isn't self-evident
   - If a line of code is fine as-is, don't change it just for the sake of change. Focus on the core improvement areas.

5. **Update or add tests**:
   - Ensure existing tests still pass
   - Add new tests if the refactoring exposed previously untestable code
   - Update tests if function signatures or behavior changed
   - Run `make test` to verify all unit tests pass

6. **Verify the changes**:
   - Run `make lint` to ensure code style compliance
   - Run `make test` for unit tests
   - For controller changes, consider running `make test.integration`
   - Review the diff to ensure no unintended changes were made

7. **Summarize the refactoring**:
   - List the files changed
   - Describe the improvements made
   - Note any potential impacts or follow-up work needed
   - Suggest commit message following the format below

## Commit Message Format

When suggesting a commit message for refactored code, use this format:

```
<brief description of the refactoring>

<detailed explanation of what was refactored and why>

Co-authored-by: Claude Code <noreply@anthropic.com>
```

**Example:**
```
Refactor reconciliation error handling

Extract common error handling logic into a shared helper function
to reduce code duplication across controllers. This improves
maintainability and ensures consistent error handling behavior.

Co-authored-by: Claude Code <noreply@anthropic.com>
```

## Important Notes

- **No behavior changes**: Refactoring should preserve existing functionality
- **Keep it focused**: Don't mix refactoring with new features or bug fixes
- **Test coverage**: Ensure tests still pass and cover the refactored code
- **Incremental approach**: For large refactorings, break into smaller, reviewable chunks
- **Avoid over-engineering**: Don't add abstractions or patterns that aren't currently needed
- **API compatibility**: Be extra careful with changes to public APIs or CRD types. Prioritize refactoring internal logic over changing exported function signatures unless specifically requested.
- **Sign commits**: Remember to use `-s` flag when committing
- **Attribution**: Always include `Co-authored-by: Claude Code <noreply@anthropic.com>` in commit messages

## Common Refactoring Patterns

### Extract Function
When a function is too long or does multiple things:
```go
// Before
func ProcessRequest(req Request) error {
    // 50 lines of code doing multiple things
}

// After
func ProcessRequest(req Request) error {
    if err := validateRequest(req); err != nil {
        return err
    }
    return executeRequest(req)
}
```

### Remove Duplication
When similar code appears in multiple places:
```go
// Before: Duplicated error handling in multiple functions

// After: Extracted to a shared helper
func handleReconcileError(err error, resource string) {
    // Common error handling logic
}
```

### Simplify Conditionals
When conditions are complex or nested:
```go
// Before
if !isEnabled || (config != nil && config.Mode == "disabled") {
    return
}

// After
if shouldSkip(isEnabled, config) {
    return
}
```

### Improve Names
When variable or function names are unclear:
```go
// Before
func proc(d []byte) error { ... }

// After
func processManifest(manifestData []byte) error { ... }
```
