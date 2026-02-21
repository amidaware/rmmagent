# Contributing Guidelines

Welcome to the Tactical RMM Agent project! This guide outlines the standards, processes, and best practices for contributing to the codebase. Following these guidelines helps maintain code quality, consistency, and project sustainability.

## Getting Started

Before contributing, ensure you have completed the development environment setup:

1. **[Environment Setup](../setup/environment.md)** - Configure your development tools
2. **[Local Development](../setup/local-development.md)** - Clone and build the project
3. **[Architecture Overview](../architecture/README.md)** - Understand the system design
4. **[Testing Guide](../testing/README.md)** - Learn the testing standards

## Code Style and Conventions

The Tactical RMM Agent follows Go community standards and specific project conventions to maintain consistency and readability.

### Go Style Guidelines

**Follow standard Go conventions:**
- Use `gofmt` for code formatting
- Use `goimports` for import organization
- Follow effective Go practices from golang.org
- Use meaningful variable and function names
- Write clear, concise comments

### Formatting Standards

**Automated formatting:**
```bash
# Format all Go code
gofmt -w .

# Organize imports
goimports -w .

# Run linter
golangci-lint run

# Pre-commit formatting script
make fmt
```

**Required formatting tools:**
```bash
go install golang.org/x/tools/cmd/goimports@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### Naming Conventions

**Package Naming:**
```go
// Good: lowercase, descriptive
package agent
package systemmonitor

// Avoid: uppercase, underscores
package Agent         // Wrong
package system_monitor // Wrong
```

**Function and Variable Naming:**
```go
// Good: camelCase for unexported, PascalCase for exported
func processSystemInfo() error        // Unexported
func GetSystemInfo() (SystemInfo, error) // Exported

var maxRetries int = 3                // Unexported
var DefaultTimeout = 30 * time.Second // Exported
```

**Constants:**
```go
// Good: descriptive constants with proper grouping
const (
    DefaultCheckinInterval = 35 * time.Second
    MaxCommandTimeout     = 15 * time.Minute
    ConfigFileName        = "agent.json"
)
```

**Interface Naming:**
```go
// Good: -er suffix for single-method interfaces
type Executor interface {
    Execute(cmd Command) (*Result, error)
}

type SystemMonitor interface {
    GetSystemInfo() (SystemInfo, error)
    GetProcessList() ([]Process, error)
}
```

### File Organization

**File naming patterns:**
- `agent.go` - Core functionality
- `agent_windows.go` - Windows-specific code with `//go:build windows`
- `agent_unix.go` - Unix/Linux code with `//go:build !windows`
- `agent_test.go` - Unit tests
- `types.go` - Type definitions
- `constants.go` - Package constants

**Build constraints:**
```go
//go:build windows
// +build windows

package agent

// Windows-specific implementation
```

### Documentation Standards

**Package Documentation:**
```go
// Package agent provides cross-platform remote monitoring and management
// functionality for the Tactical RMM system. It handles system monitoring,
// command execution, and secure communication with the RMM server.
//
// The agent operates as a system service and provides the following key features:
//   - Real-time system monitoring and health checks
//   - Remote command execution with security validation
//   - Secure communication via HTTPS and NATS messaging
//   - Cross-platform support for Windows, macOS, and Linux
package agent
```

**Function Documentation:**
```go
// ExecuteCommand runs a command with the specified parameters and returns
// the result. The command is validated for security before execution and
// runs with the configured timeout and resource limits.
//
// Parameters:
//   - cmd: Command specification including script, shell, and timeout
//
// Returns:
//   - CommandResult containing stdout, stderr, and exit code
//   - error if validation fails or execution encounters an error
//
// Security: All commands are validated against malicious patterns and
// executed in a sandboxed environment with resource limits.
func (a *Agent) ExecuteCommand(cmd Command) (*CommandResult, error) {
    // Implementation...
}
```

**Comment Standards:**
```go
// Good: Explain why, not what
// Retry with exponential backoff to handle transient network failures
// and avoid overwhelming the server during outages
if err := a.retryWithBackoff(operation); err != nil {
    return fmt.Errorf("operation failed after retries: %w", err)
}

// Avoid: Obvious comments
// Set retry count to 3
retryCount := 3
```

## Branch Naming and Workflow

### Branch Naming Convention

Use descriptive branch names that follow this pattern:

**Branch Prefixes:**
- `feature/` - New features or enhancements
- `bugfix/` - Bug fixes
- `hotfix/` - Critical production fixes
- `refactor/` - Code refactoring without behavior changes
- `docs/` - Documentation updates
- `security/` - Security-related changes
- `test/` - Test improvements or additions

**Examples:**
```bash
feature/openframe-integration
bugfix/windows-service-startup
hotfix/critical-memory-leak
refactor/agent-initialization
docs/api-documentation-update
security/input-validation-enhancement
test/integration-test-improvements
```

### Git Workflow

**Standard workflow:**
```bash
# 1. Create and checkout feature branch
git checkout -b feature/new-monitoring-check

# 2. Make changes and commit frequently
git add .
git commit -m "Add CPU temperature monitoring check"

# 3. Keep branch updated with main
git fetch origin
git rebase origin/main

# 4. Push branch and create pull request
git push origin feature/new-monitoring-check
```

**Branch Management:**
- Keep branches focused on single features/fixes
- Rebase feature branches on main before merging
- Delete branches after successful merge
- Use draft PRs for work in progress

## Commit Message Format

Follow the conventional commit format for clear, searchable commit history:

### Commit Message Structure

```text
<type>(<scope>): <subject>

<body>

<footer>
```

**Commit Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Adding or modifying tests
- `chore`: Build process, auxiliary tools, dependencies
- `security`: Security improvements
- `perf`: Performance improvements

**Examples:**
```bash
feat(agent): add OpenFrame connection manager

Implement secure connection management for OpenFrame integration
including token refresh and connection pooling.

- Add OpenFrameConnectionManager struct
- Implement automatic token refresh
- Add connection health monitoring
- Update tests for new functionality

Closes #123

fix(windows): resolve service installation permissions issue

Fix service installation failing on Windows when run without
administrator privileges by improving privilege detection and
providing clear error messages.

Fixes #456

docs(security): update authentication documentation

Add comprehensive documentation for token-based authentication
including OpenFrame integration patterns and security best practices.

test(integration): add NATS communication tests

Implement integration tests for NATS message handling including
connection reliability and message processing validation.
```

### Commit Best Practices

**Good commits:**
- Atomic changes (one logical change per commit)
- Clear, descriptive messages
- Include context about why changes were made
- Reference relevant issues/PRs

**Commit frequency:**
- Commit early and often during development
- Squash related commits before merging
- Use meaningful commit messages even for work-in-progress

## Pull Request Process

### Creating Pull Requests

**PR Preparation:**
1. Ensure your branch is up to date with main
2. Run full test suite and ensure all tests pass
3. Update documentation for any API changes
4. Add or update tests for new functionality
5. Run linting and formatting tools

**PR Creation Checklist:**
```bash
# Before creating PR, run these commands:
make fmt                    # Format code
make lint                   # Run linter
make test                   # Run all tests
make test-integration       # Run integration tests
go mod tidy                 # Clean up dependencies
```

### PR Template

Use this template for all pull requests:

```markdown
## Description
Brief description of the changes and their purpose.

## Type of Change
- [ ] Bug fix (non-breaking change which fixes an issue)
- [ ] New feature (non-breaking change which adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] Documentation update
- [ ] Refactoring (no functional changes)
- [ ] Performance improvement
- [ ] Security enhancement

## Changes Made
- Detailed list of changes
- Include file names and brief descriptions
- Mention any breaking changes

## Testing
- [ ] Unit tests added/updated
- [ ] Integration tests added/updated
- [ ] Manual testing completed
- [ ] All existing tests pass

### Test Coverage
- Current coverage: XX%
- Coverage change: +/- XX%

## Security Considerations
- List any security implications
- Describe security testing performed
- Note any new attack vectors considered

## Documentation
- [ ] Code comments updated
- [ ] API documentation updated
- [ ] User documentation updated (if applicable)

## Performance Impact
- Describe any performance implications
- Include benchmark results if applicable

## Breaking Changes
List any breaking changes and migration steps required.

## Related Issues
Closes #XXX
Related to #XXX

## Screenshots (if applicable)
Include screenshots for UI changes or visual improvements.

## Checklist
- [ ] Code follows style guidelines
- [ ] Self-review completed
- [ ] Code is commented appropriately
- [ ] Tests added for new functionality
- [ ] All tests pass
- [ ] Documentation updated
- [ ] Security review completed (for security-sensitive changes)
```

### Review Process

**Review Requirements:**
- At least one approval from a maintainer
- All automated checks must pass
- Security review for security-sensitive changes
- Performance review for performance-critical changes

**Review Guidelines for Reviewers:**
- Focus on correctness, security, and maintainability
- Provide constructive feedback
- Test the changes locally when necessary
- Verify documentation accuracy
- Check for potential security vulnerabilities

**Addressing Review Comments:**
- Respond to all review comments
- Make requested changes in separate commits
- Mark resolved conversations as resolved
- Request re-review after addressing feedback

## Review Checklist

### Code Quality Checklist

**Functionality:**
- [ ] Code accomplishes the intended purpose
- [ ] Edge cases are handled appropriately
- [ ] Error handling is comprehensive and appropriate
- [ ] No obvious bugs or logical errors

**Security:**
- [ ] Input validation is implemented where needed
- [ ] No hardcoded credentials or sensitive information
- [ ] Authentication and authorization are properly implemented
- [ ] Potential security vulnerabilities are addressed

**Performance:**
- [ ] Code is efficient and doesn't introduce performance regressions
- [ ] Resource usage (memory, CPU) is reasonable
- [ ] Database queries are optimized (if applicable)
- [ ] Caching is used appropriately

**Maintainability:**
- [ ] Code is readable and well-structured
- [ ] Functions and classes have single responsibilities
- [ ] Code follows established patterns and conventions
- [ ] Comments explain complex logic and design decisions

**Testing:**
- [ ] Unit tests are comprehensive and meaningful
- [ ] Integration tests cover component interactions
- [ ] Test names are descriptive and intentions are clear
- [ ] Mock objects are used appropriately

**Documentation:**
- [ ] Public APIs are documented
- [ ] Complex algorithms are explained
- [ ] README and other docs are updated if necessary
- [ ] Code comments are clear and helpful

### Platform-Specific Review Points

**Cross-Platform Compatibility:**
- [ ] Platform-specific code uses appropriate build tags
- [ ] Platform differences are handled correctly
- [ ] File paths use filepath.Join() for cross-platform compatibility
- [ ] Platform-specific dependencies are properly managed

**Windows-Specific:**
- [ ] Windows API usage is correct and secure
- [ ] Service integration follows Windows standards
- [ ] File permissions and security contexts are appropriate
- [ ] PowerShell execution is secure

**Unix/Linux-Specific:**
- [ ] systemd integration is correct
- [ ] File permissions follow Unix standards
- [ ] Shell command execution is secure
- [ ] Signal handling is appropriate

## Development Best Practices

### Error Handling

**Consistent error handling patterns:**
```go
// Good: Wrap errors with context
func (a *Agent) ProcessTask(task Task) error {
    if err := a.validateTask(task); err != nil {
        return fmt.Errorf("task validation failed: %w", err)
    }
    
    if err := a.executeTask(task); err != nil {
        return fmt.Errorf("task execution failed for task %s: %w", task.ID, err)
    }
    
    return nil
}

// Good: Use typed errors for specific handling
type ValidationError struct {
    Field   string
    Message string
}

func (e ValidationError) Error() string {
    return fmt.Sprintf("validation failed for field %s: %s", e.Field, e.Message)
}
```

### Logging Best Practices

**Structured logging:**
```go
// Good: Use structured logging with appropriate levels
func (a *Agent) ExecuteCommand(cmd Command) (*CommandResult, error) {
    logger := a.Logger.WithFields(logrus.Fields{
        "command_id": cmd.ID,
        "shell":      cmd.Shell,
        "timeout":    cmd.Timeout,
    })
    
    logger.Info("Starting command execution")
    
    result, err := a.executeCommand(cmd)
    if err != nil {
        logger.WithError(err).Error("Command execution failed")
        return nil, err
    }
    
    logger.WithField("exit_code", result.ExitCode).Info("Command completed")
    return result, nil
}
```

### Concurrency Patterns

**Safe concurrency patterns:**
```go
// Good: Use context for cancellation and timeout
func (a *Agent) RunConcurrentChecks(ctx context.Context, checks []Check) error {
    var wg sync.WaitGroup
    errChan := make(chan error, len(checks))
    
    for _, check := range checks {
        wg.Add(1)
        go func(c Check) {
            defer wg.Done()
            
            select {
            case <-ctx.Done():
                errChan <- ctx.Err()
                return
            default:
            }
            
            if err := c.Execute(ctx); err != nil {
                errChan <- fmt.Errorf("check %s failed: %w", c.Name(), err)
            }
        }(check)
    }
    
    wg.Wait()
    close(errChan)
    
    var errors []error
    for err := range errChan {
        errors = append(errors, err)
    }
    
    if len(errors) > 0 {
        return fmt.Errorf("checks failed: %v", errors)
    }
    
    return nil
}
```

## Release Process

### Version Management

**Semantic Versioning:**
- `MAJOR.MINOR.PATCH` format (e.g., 2.9.0)
- MAJOR: Breaking changes
- MINOR: New features, backward compatible
- PATCH: Bug fixes, backward compatible

**Version Update Process:**
1. Update version in `main.go`
2. Update CHANGELOG.md
3. Create version tag
4. Build release binaries

### Release Checklist

**Pre-Release:**
- [ ] All tests pass on all supported platforms
- [ ] Security review completed
- [ ] Performance benchmarks run
- [ ] Documentation updated
- [ ] CHANGELOG.md updated
- [ ] Version numbers updated

**Release:**
- [ ] Create release tag
- [ ] Build release binaries for all platforms
- [ ] Create GitHub release with release notes
- [ ] Update documentation
- [ ] Notify relevant stakeholders

## Community Guidelines

### Code of Conduct

- Be respectful and inclusive
- Focus on constructive feedback
- Help newcomers learn and contribute
- Maintain professional communication
- Follow project guidelines and standards

### Getting Help

**Resources for Contributors:**
- Project documentation
- GitHub Issues for questions and discussions
- Code review feedback
- Community discussions

**Reporting Issues:**
- Use GitHub Issues for bug reports
- Provide detailed reproduction steps
- Include system information and logs
- Use appropriate labels and templates

Following these contributing guidelines helps maintain the quality and consistency of the Tactical RMM Agent project while fostering a collaborative development environment.