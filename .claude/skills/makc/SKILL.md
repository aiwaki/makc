```markdown
# makc Development Patterns

> Auto-generated skill from repository analysis

## Overview
This skill teaches the core development patterns and conventions used in the `makc` Go codebase. It covers file naming, import/export styles, commit conventions, and testing patterns, providing a comprehensive guide for contributing to or maintaining the repository.

## Coding Conventions

### File Naming
- All files use **snake_case**.
  - Example: `my_module.go`, `user_service.go`

### Import Style
- **Relative imports** are used for importing local packages.
  - Example:
    ```go
    import "../utils"
    ```

### Export Style
- **Named exports** are preferred. Exported identifiers start with an uppercase letter.
  - Example:
    ```go
    // In user_service.go
    package user

    func GetUser(id int) *User {
        // ...
    }
    ```

### Commit Patterns
- **Conventional commits** are used, with the primary prefix being `fix`.
- Commit messages are concise, averaging 29 characters.
  - Example:
    ```
    fix: correct user ID validation
    ```

## Workflows

### Fixing Bugs
**Trigger:** When a bug or issue is identified in the codebase.
**Command:** `/fix-bug`

1. Create a new branch for the fix.
2. Locate the problematic code using relative imports as needed.
3. Apply the fix, following snake_case file naming and named exports.
4. Write or update tests in `*.test.*` files.
5. Commit your changes using the `fix:` prefix.
6. Open a pull request for review.

### Adding a New Module
**Trigger:** When introducing new functionality.
**Command:** `/add-module`

1. Create a new file using snake_case (e.g., `new_feature.go`).
2. Use relative imports to include dependencies.
3. Export functions/types with uppercase names.
4. Add corresponding tests in a `new_feature.test.go` file.
5. Commit with a clear, conventional message.
6. Submit for review.

## Testing Patterns

- Test files follow the pattern: `*.test.*` (e.g., `user_service.test.go`).
- The specific testing framework is not specified, but tests should be placed in files matching this pattern.
- Example test file:
  ```go
  // user_service.test.go
  package user

  import "testing"

  func TestGetUser(t *testing.T) {
      // test logic here
  }
  ```

## Commands
| Command      | Purpose                                 |
|--------------|-----------------------------------------|
| /fix-bug     | Workflow for fixing bugs                |
| /add-module  | Workflow for adding a new module        |
```
