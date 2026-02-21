# Contributing to Tactical RMM Agent

Welcome to the Tactical RMM Agent project! This guide outlines the standards, processes, and best practices for contributing to the codebase. Following these guidelines helps maintain code quality, consistency, and project sustainability.

## Table of Contents

- [Getting Started](#getting-started)
- [Code Style and Conventions](#code-style-and-conventions)
- [Development Workflow](#development-workflow)
- [Pull Request Process](#pull-request-process)
- [Review Guidelines](#review-guidelines)
- [Security Considerations](#security-considerations)
- [Community Guidelines](#community-guidelines)

## Getting Started

Before contributing, ensure you have completed the development environment setup:

1. **Environment Setup** - Configure your development tools (Go 1.20+, IDE, linting tools)
2. **Local Development** - Clone and build the project locally
3. **Architecture Understanding** - Review the system design and component relationships
4. **Testing Standards** - Learn the testing requirements and standards

### Prerequisites

- **Go 1.20+** with proper GOPATH/module setup
- **Development Tools**: gofmt, goimports, golangci-lint
- **Platform Knowledge**: Understanding of Windows, macOS, and Linux system APIs
- **Version Control**: Git with proper commit signing (recommended)

### Development Environment

**Required Tools:**
```bash
# Install formatting and linting tools
go install golang.org/x/tools/cmd/goimports@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Verify installations
gofmt -version
goimports -help
golangci-lint version
```

**Project Setup:**
```bash
# Clone repository
git clone https://github.com/amidaware/rmmagent.git
cd rmmagent

# Install dependencies
go mod download

# Run tests
go test ./...

# Build locally
go build -o rmmagent
```

## Code Style and Conventions

The Tactical RMM Agent follows Go community standards and specific project conventions to maintain consistency and readability.

### Go Style Guidelines

**Follow standard Go conventions:**
- Use `gofmt` for code formatting (required before commits)
- Use `goimports` for import organization
- Follow effective Go practices from golang.org
- Use meaningful variable and function names
- Write clear, concise comments explaining complex logic

### Formatting Standards

**Pre-commit formatting (required):**
```bash
# Format all Go code
gofmt -w .

# Organize imports
goimports -w .

# Run linter
golangci-lint run

# Combined pre-commit script
make fmt  # If Makefile exists
```

### Naming Conventions

**Package Naming:**
```go
// ✅ Good: lowercase, descriptive
package agent
package systemmonitor

// ❌ Avoid: uppercase, underscores
package Agent         // Wrong
package system_monitor // Wrong
```

**Function and Variable Naming:**
```go
// ✅ Good: camelCase for unexported, PascalCase for exported
func processSystemInfo() error               // Unexported
func GetSystemInfo() (SystemInfo, error)   // Exported

var maxRetries int = 3                      // Unexported  
var DefaultTimeout = 30 * time.Second      // Exported
```

**Interface Naming:**
```go
// ✅ Good: -er suffix for single-method interfaces
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

**Build constraints (required for platform-specific code):**
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

**Function Documentation (required for exported functions):**
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

## Development Workflow

### Branch Naming Convention

Use descriptive branch names with appropriate prefixes:

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

**Standard development workflow:**
```bash
# 1. Create and checkout feature branch
git checkout -b feature/new-monitoring-check

# 2. Make changes and commit frequently with descriptive messages
git add .
git commit -m "feat(agent): add CPU temperature monitoring check

Implement CPU temperature monitoring for system health checks
including platform-specific sensor reading and threshold alerting.

- Add temperature sensor detection
- Implement cross-platform temperature reading
- Add configurable threshold alerts
- Update tests for new functionality"

# 3. Keep branch updated with main
git fetch origin
git rebase origin/main

# 4. Push branch and create pull request
git push origin feature/new-monitoring-check
```

### Commit Message Format

Follow the conventional commit format for clear, searchable commit history:

**Commit Message Structure:**
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

**Good Commit Examples:**
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
```

## Pull Request Process

### Creating Pull Requests

**PR Preparation Checklist:**
```bash
# Before creating PR, run these commands:
gofmt -w .                     # Format code
goimports -w .                 # Organize imports
golangci-lint run             # Run linter
go test ./...                 # Run all unit tests
go test -tags integration ./... # Run integration tests (if available)
go mod tidy                   # Clean up dependencies
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
- Mention any breaking changes or migration steps

## Testing
- [ ] Unit tests added/updated
- [ ] Integration tests added/updated (if applicable)
- [ ] Manual testing completed on target platforms
- [ ] All existing tests pass

### Test Coverage
- Current coverage: XX%
- Coverage change: +/- XX%

## Security Considerations
- List any security implications
- Describe security testing performed  
- Note any new attack vectors considered
- Confirm input validation is implemented

## Platform Compatibility
- [ ] Windows testing completed
- [ ] macOS testing completed
- [ ] Linux testing completed
- [ ] Cross-platform compatibility verified

## Documentation
- [ ] Code comments updated
- [ ] API documentation updated (if applicable)
- [ ] User documentation updated (if applicable)
- [ ] CHANGELOG.md updated (for releases)

## Performance Impact
- Describe any performance implications
- Include benchmark results if applicable
- Note resource usage changes

## Breaking Changes
List any breaking changes and migration steps required.

## Related Issues
Closes #XXX
Related to #XXX

## Checklist
- [ ] Code follows style guidelines
- [ ] Self-review completed
- [ ] Code is commented appropriately
- [ ] Tests added for new functionality
- [ ] All tests pass
- [ ] Documentation updated
- [ ] Security review completed (for security-sensitive changes)
```

### Review Requirements

**All PRs must have:**
- At least one approval from a maintainer
- All automated checks passing
- Security review for security-sensitive changes
- Performance review for performance-critical changes
- Platform testing for cross-platform changes

## Review Guidelines

### For Code Reviewers

**Review Focus Areas:**
- **Correctness**: Code accomplishes intended purpose without bugs
- **Security**: Input validation, authentication, no hardcoded credentials
- **Performance**: No performance regressions, efficient resource usage
- **Maintainability**: Code is readable, well-structured, follows patterns
- **Testing**: Comprehensive unit/integration tests with clear intentions

**Platform-Specific Review Points:**
- **Cross-Platform Compatibility**: Proper use of build tags and platform abstraction
- **Windows**: Correct Windows API usage, service integration, PowerShell security
- **Unix/Linux**: systemd integration, file permissions, shell command security  
- **macOS**: launchd integration, security framework compliance

### For PR Authors

**Addressing Review Comments:**
- Respond to all review comments constructively
- Make requested changes in separate, well-documented commits
- Mark resolved conversations as resolved
- Request re-review after addressing all feedback
- Explain design decisions when requested changes conflict with requirements

## Security Considerations

### Security Best Practices

**Input Validation:**
```go
// ✅ Good: Validate all external input
func (a *Agent) ExecuteCommand(cmd Command) (*CommandResult, error) {
    if err := a.validateCommand(cmd); err != nil {
        return nil, fmt.Errorf("command validation failed: %w", err)
    }
    
    // Sanitize command parameters
    sanitizedCmd := a.sanitizeCommand(cmd)
    
    // Execute with timeout and resource limits
    return a.executeWithLimits(sanitizedCmd)
}
```

**Error Handling:**
```go
// ✅ Good: Don't expose sensitive information in errors
func (a *Agent) authenticate(token string) error {
    if !a.isValidToken(token) {
        a.Logger.Warn("Authentication failed") // Log internally
        return errors.New("authentication failed") // Generic external error
    }
    return nil
}
```

**Logging Security:**
```go
// ✅ Good: Never log sensitive information
func (a *Agent) processCredentials(username, password string) {
    a.Logger.WithField("username", username).Info("Processing credentials")
    // Never log password or token values
}
```

### Security Review Requirements

**Changes requiring security review:**
- Authentication and authorization logic
- Input validation and sanitization
- Cryptographic operations
- Network communication
- File system operations
- Command execution
- Privilege escalation

## Development Best Practices

### Error Handling Patterns

```go
// ✅ Good: Consistent error wrapping with context
func (a *Agent) ProcessTask(task Task) error {
    if err := a.validateTask(task); err != nil {
        return fmt.Errorf("task validation failed: %w", err)
    }
    
    if err := a.executeTask(task); err != nil {
        return fmt.Errorf("task execution failed for task %s: %w", task.ID, err)
    }
    
    return nil
}

// ✅ Good: Use typed errors for specific handling
type ValidationError struct {
    Field   string
    Message string
}

func (e ValidationError) Error() string {
    return fmt.Sprintf("validation failed for field %s: %s", e.Field, e.Message)
}
```

### Concurrency Patterns

```go
// ✅ Good: Safe concurrency with context and proper cleanup
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

### Testing Requirements

**Unit Testing:**
```go
func TestAgentExecuteCommand(t *testing.T) {
    tests := []struct {
        name        string
        cmd         Command
        expectError bool
        expectOut   string
    }{
        {
            name: "valid command execution",
            cmd: Command{
                Script:  "echo 'hello'",
                Shell:   "bash",
                Timeout: 30,
            },
            expectError: false,
            expectOut:   "hello\n",
        },
        {
            name: "invalid command validation",
            cmd: Command{
                Script: "rm -rf /", // Malicious command
                Shell:  "bash",
            },
            expectError: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            agent := &Agent{Logger: logrus.New()}
            result, err := agent.ExecuteCommand(tt.cmd)
            
            if tt.expectError {
                assert.Error(t, err)
                return
            }
            
            assert.NoError(t, err)
            assert.Equal(t, tt.expectOut, result.Stdout)
        })
    }
}
```

## Community Guidelines

### Code of Conduct

- **Be Respectful**: Treat all contributors with respect and professionalism
- **Be Inclusive**: Welcome contributors of all backgrounds and experience levels
- **Be Constructive**: Provide helpful, actionable feedback in reviews
- **Help Others Learn**: Mentor newcomers and share knowledge
- **Follow Guidelines**: Adhere to project standards and best practices

### Getting Help

**Resources for Contributors:**
- **Documentation**: Comprehensive guides in `/docs/` directory
- **GitHub Issues**: Use for questions, discussions, and bug reports
- **Code Reviews**: Learn from feedback and provide constructive reviews
- **Architecture Docs**: Understanding system design and component relationships

**Reporting Issues:**
- Use GitHub Issues with appropriate templates
- Provide detailed reproduction steps and system information
- Include relevant logs and error messages
- Use appropriate labels (`bug`, `feature-request`, `question`)

### Recognition

We value all contributions to the Tactical RMM Agent project. Contributors are recognized through:
- GitHub contributor statistics
- Changelog acknowledgments for significant contributions
- Community recognition for helping others
- Maintainer opportunities for consistent, high-quality contributors

Thank you for contributing to the Tactical RMM Agent! Your efforts help make enterprise endpoint management more secure, efficient, and accessible.