# Local Development Guide

This guide walks you through cloning, building, running, and debugging the Tactical RMM Agent locally. It covers both development workflows and testing scenarios for contributing to the project.

## Prerequisites

Before starting, ensure you have completed the [Environment Setup](environment.md) guide and have the following ready:

- Go 1.20+ installed and configured
- Git configured with your credentials
- Development IDE set up (VS Code, GoLand, or Vim)
- Required build tools for your platform

## Clone and Setup

### Clone the Repository

```bash
# Clone the main repository
git clone https://github.com/amidaware/rmmagent.git
cd rmmagent

# Or if you have a fork
git clone https://github.com/YOUR-USERNAME/rmmagent.git
cd rmmagent
git remote add upstream https://github.com/amidaware/rmmagent.git
```

### Initialize Go Module

```bash
# Initialize and download dependencies
go mod download
go mod tidy

# Verify dependencies
go mod verify
```

### Project Structure Overview

Familiarize yourself with the project layout:

```text
rmmagent/
├── main.go                 # Entry point and CLI interface
├── go.mod                  # Go module definition
├── go.sum                  # Dependency checksums
├── agent/                  # Core agent package
│   ├── agent.go           # Main agent struct and functionality
│   ├── agent_windows.go   # Windows-specific implementations
│   ├── agent_unix.go      # Unix/Linux-specific implementations  
│   ├── checkin.go         # Server check-in logic
│   ├── checks.go          # Health checks and monitoring
│   ├── install.go         # Installation routines
│   ├── rpc.go            # NATS RPC communication
│   └── ...               # Additional agent components
├── shared/                # Shared types and utilities
└── scripts/              # Build and utility scripts
```

## Building the Agent

### Basic Build

```bash
# Build for current platform
go build -o rmmagent

# Build with race detection (development)
go build -race -o rmmagent-debug

# Cross-compile for different platforms
GOOS=windows GOARCH=amd64 go build -o rmmagent.exe
GOOS=linux GOARCH=amd64 go build -o rmmagent-linux
GOOS=darwin GOARCH=amd64 go build -o rmmagent-darwin
```

### Advanced Build Options

**Development build with debug symbols:**
```bash
go build -gcflags="all=-N -l" -o rmmagent-debug
```

**Production build (optimized):**
```bash
go build -ldflags="-w -s" -o rmmagent-prod
```

**Build with custom version:**
```bash
go build -ldflags="-X main.version=2.9.0-dev" -o rmmagent
```

### Makefile Targets (if available)

```bash
# Check if Makefile exists
ls Makefile

# Common make targets
make build          # Build for current platform
make test          # Run all tests
make lint          # Run linter
make clean         # Clean build artifacts
make cross-compile # Build for all platforms
```

## Running the Agent Locally

### Development Mode

For local development and testing, you can run the agent with various modes:

**Check version and basic functionality:**
```bash
./rmmagent -version
```

**Test installation mode (dry run):**
```bash
# This won't actually install, but tests the installation logic
./rmmagent -m install \
  -api "https://demo.tacticalrmm.com" \
  -client-id 1 \
  -site-id 1 \
  -auth "test-token" \
  -desc "Development Test Agent"
```

**Run specific operational modes:**
```bash
# Run system information collection
./rmmagent -m winupdater  # Windows only
./rmmagent -m getprocs    # Get running processes
./rmmagent -m rebootnow   # Test reboot logic (don't run on dev machine!)
./rmmagent -m checkrunner # Run health checks
```

### Safe Development Testing

**Safe commands for local testing:**

```bash
# System information (safe, read-only)
./rmmagent -m sysinfo

# Process list (safe, read-only)  
./rmmagent -m getprocs

# Software inventory (safe, read-only)
./rmmagent -m software

# Network checks (safe, read-only)
./rmmagent -m checkrunner

# Version and help (always safe)
./rmmagent -version
./rmmagent -h
```

**⚠️ Commands to avoid in development:**
```bash
# DON'T run these on your development machine:
./rmmagent -m install    # Will try to install agent as service
./rmmagent -m rebootnow  # Will reboot your machine
./rmmagent -m uninstall  # Will try to uninstall system components
```

## Development Workflow

### Hot Reload Development

For rapid development iteration:

**Option 1: Build and run cycle**
```bash
# Terminal 1: Watch for changes and rebuild
while true; do
  go build -o rmmagent
  sleep 2
done

# Terminal 2: Run tests
./rmmagent -version
```

**Option 2: Use go run for quick testing**
```bash
# Run directly without building binary
go run main.go -version
go run main.go -m sysinfo
```

**Option 3: Use file watchers (if you have them)**
```bash
# Install air for hot reloading (optional)
go install github.com/cosmtrek/air@latest

# Create .air.toml configuration
air init
air  # Start hot reload server
```

### Testing Changes

**Unit Testing:**
```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -v -coverprofile=coverage.out ./...

# View coverage report
go tool cover -html=coverage.out

# Run specific tests
go test ./agent -run TestAgent
```

**Integration Testing:**
```bash
# Test platform-specific code
go test -tags integration ./agent

# Test with race detection
go test -race ./...

# Benchmark tests
go test -bench=. ./agent
```

### Code Quality Checks

Run these commands before committing:

```bash
# Format code
goimports -w .

# Check for issues
golangci-lint run

# Vet code
go vet ./...

# Check for security issues
gosec ./...

# Static analysis
staticcheck ./...
```

## Debugging

### VS Code Debugging

**Set breakpoints** in VS Code and use the configured launch profiles:

1. Open `main.go` or any Go file
2. Set breakpoints by clicking in the gutter
3. Press F5 or use "Run and Debug" panel
4. Select "Launch Agent" configuration

**Debug configuration (`.vscode/launch.json`):**
```json
{
    "name": "Debug Agent",
    "type": "go", 
    "request": "launch",
    "mode": "auto",
    "program": "${workspaceFolder}/main.go",
    "args": ["-m", "sysinfo"],
    "env": {
        "TACTICAL_DEV": "1"
    },
    "console": "integratedTerminal"
}
```

### Delve Command-Line Debugging

**Start debugging session:**
```bash
# Debug the main application
dlv debug -- -version

# Debug specific test
dlv test ./agent -- -test.run TestAgent

# Debug running process (if agent is running)
dlv attach $(pgrep rmmagent)
```

**Common Delve commands:**
```text
(dlv) break main.main          # Set breakpoint
(dlv) continue                 # Continue execution  
(dlv) next                     # Step over
(dlv) step                     # Step into
(dlv) print variableName       # Print variable
(dlv) goroutines              # List goroutines
(dlv) help                    # Show help
```

### Debug Logging

Enable verbose logging for development:

```bash
# Set log level to DEBUG
./rmmagent -log DEBUG -logto stdout -m sysinfo

# Enable development mode (if implemented)
TACTICAL_DEV=1 ./rmmagent -log DEBUG -logto stdout -m sysinfo
```

**Add debug logging in code:**
```go
package main

import (
    "github.com/sirupsen/logrus"
)

func debugFunction() {
    log := logrus.New()
    log.SetLevel(logrus.DebugLevel)
    
    log.Debug("Debug message for development")
    log.WithFields(logrus.Fields{
        "component": "agent",
        "operation": "test",
    }).Debug("Structured debug message")
}
```

## Testing Different Scenarios

### Mock Server Testing

For testing communication features without a real Tactical RMM server:

**Simple HTTP server for API testing:**
```bash
# Create a simple test server
cat > test-server.go << EOF
package main

import (
    "encoding/json"
    "net/http"
    "log"
)

func main() {
    http.HandleFunc("/api/v3/checkin/", func(w http.ResponseWriter, r *http.Request) {
        response := map[string]interface{}{
            "status": "ok",
            "tasks": []string{},
        }
        json.NewEncoder(w).Encode(response)
    })
    
    log.Println("Test server running on :8080")
    http.ListenAndServe(":8080", nil)
}
EOF

# Run test server
go run test-server.go &

# Test agent against mock server
./rmmagent -api "http://localhost:8080" -m sysinfo
```

### Platform-Specific Testing

**Test Windows-specific code on Linux (limited):**
```bash
# Build Windows binary on Linux
GOOS=windows GOARCH=amd64 go build -o rmmagent.exe

# Run unit tests for Windows code
go test -tags windows ./agent
```

**Test macOS-specific code:**
```bash
# Build macOS binary
GOOS=darwin GOARCH=amd64 go build -o rmmagent-darwin

# Test macOS-specific functionality
go test -tags darwin ./agent
```

## Performance Profiling

### CPU Profiling

```bash
# Build with profiling
go build -o rmmagent

# Run with CPU profiling
./rmmagent -m sysinfo -cpuprofile=cpu.prof

# Analyze profile
go tool pprof cpu.prof
```

### Memory Profiling

```bash
# Run with memory profiling
./rmmagent -m sysinfo -memprofile=mem.prof

# Analyze profile
go tool pprof mem.prof
```

### Benchmarking

```bash
# Run benchmark tests
go test -bench=. -benchmem ./agent

# Profile benchmarks
go test -bench=. -cpuprofile=bench.prof ./agent
```

## Development Best Practices

### Code Organization

- **Keep platform-specific code in separate files** using build tags
- **Use interfaces** for testability and platform abstraction
- **Group related functionality** in logical packages
- **Follow Go naming conventions** and package structure

### Error Handling

```go
// Good: Wrap errors with context
func (a *Agent) performTask() error {
    if err := a.validate(); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }
    
    if err := a.execute(); err != nil {
        return fmt.Errorf("execution failed: %w", err) 
    }
    
    return nil
}

// Good: Use structured logging
log.WithError(err).WithField("component", "agent").Error("Task failed")
```

### Testing Practices

```go
// Good: Use table-driven tests
func TestAgentValidation(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr bool
    }{
        {"valid input", "valid", false},
        {"invalid input", "", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := validateInput(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("validateInput() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

## Troubleshooting Development Issues

### Common Build Issues

| Error | Solution |
|-------|----------|
| **`package not found`** | Run `go mod download` and `go mod tidy` |
| **`CGO errors`** | Install build tools (gcc, build-essential, Xcode) |
| **`permission denied`** | Check file permissions, run with appropriate privileges |
| **`module checksum mismatch`** | Run `go clean -modcache` and re-download |

### Runtime Issues

```bash
# Check for race conditions
go run -race main.go -m sysinfo

# Enable detailed GC logging
GODEBUG=gctrace=1 ./rmmagent -m sysinfo

# Check memory usage
GODEBUG=madvdontneed=1 ./rmmagent -m sysinfo
```

### Network Issues

```bash
# Test local connectivity
curl -I http://localhost:8080/api/v3/

# Check DNS resolution
nslookup your-rmm-server.com

# Test NATS connectivity
nc -zv your-rmm-server.com 4222
```

## Next Steps

After setting up local development:

1. **[Architecture Overview](../architecture/README.md)** - Understand the system design
2. **[Testing Guide](../testing/README.md)** - Learn the testing strategy
3. **[Contributing Guidelines](../contributing/guidelines.md)** - Review contribution process
4. **[Security Practices](../security/README.md)** - Understand security requirements

You're now ready to develop, test, and contribute to the Tactical RMM Agent project!