```markdown
# makc Development Patterns

> Auto-generated skill from repository analysis

## Overview
This skill teaches the core development patterns and conventions used in the `makc` Go codebase. It covers file organization, code style, commit conventions, and testing patterns. By following these guidelines, contributors can maintain consistency and quality across the project.

## Coding Conventions

### File Naming
- Use **snake_case** for all file names.
  - Example: `user_service.go`, `data_parser.go`

### Import Style
- Use **relative imports** within the project.
  - Example:
    ```go
    import "../utils"
    ```

### Export Style
- Use **named exports** for functions, types, and variables.
  - Example:
    ```go
    // In user_service.go
    package user

    func CreateUser(name string) *User {
        // ...
    }
    ```

### Commit Patterns
- Follow **conventional commit** types.
- Common prefixes: `refactor`, `chore`, `docs`
- Commit messages are concise, averaging 45 characters.
  - Example:
    ```
    refactor: update user creation logic
    chore: update dependencies
    docs: add API usage instructions
    ```

## Workflows

### Refactoring Code
**Trigger:** When improving code structure or readability without changing external behavior  
**Command:** `/refactor`

1. Identify code that needs restructuring.
2. Make changes using snake_case file naming and relative imports.
3. Use a commit message starting with `refactor:`.
4. Push your changes and open a pull request.

### Updating Documentation
**Trigger:** When adding or updating documentation  
**Command:** `/docs`

1. Edit or create documentation files.
2. Use clear, concise language.
3. Commit with a message starting with `docs:`.
4. Push and submit for review.

### Maintenance Tasks
**Trigger:** For dependency updates, build scripts, or non-code changes  
**Command:** `/chore`

1. Make the necessary maintenance changes.
2. Use a commit message starting with `chore:`.
3. Push your changes.

## Testing Patterns

- Test files follow the pattern: `*.test.*`
  - Example: `user_service.test.go`
- The specific testing framework is not specified; follow Go's standard testing practices unless otherwise noted.
- Example test file:
  ```go
  package user

  import "testing"

  func TestCreateUser(t *testing.T) {
      user := CreateUser("Alice")
      if user.Name != "Alice" {
          t.Errorf("Expected name Alice, got %s", user.Name)
      }
  }
  ```

## Commands
| Command    | Purpose                                 |
|------------|-----------------------------------------|
| /refactor  | Start a code refactoring workflow       |
| /docs      | Update or add documentation             |
| /chore     | Perform maintenance or dependency tasks |
```
