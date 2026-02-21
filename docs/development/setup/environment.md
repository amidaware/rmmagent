# Development Environment Setup

This guide helps you set up a complete development environment for the Tactical RMM Agent, including IDE configuration, essential tools, and recommended extensions for productive Go development.

## Required Software and Tools

### Go Development Environment

| Tool | Version | Purpose | Installation |
|------|---------|---------|-------------|
| **Go** | 1.20+ | Core language runtime | [golang.org/dl](https://golang.org/dl) |
| **Git** | 2.30+ | Version control | [git-scm.com](https://git-scm.com) |
| **Make** | Latest | Build automation | Package manager or build tools |
| **GCC/Build Tools** | Latest | CGO compilation | Platform-specific |

### Install Go

**Linux/macOS:**
```bash
# Download and install Go 1.20+
wget https://golang.org/dl/go1.20.linux-amd64.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.20.linux-amd64.tar.gz

# Add to PATH in ~/.bashrc or ~/.zshrc
export PATH=$PATH:/usr/local/go/bin
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin
```

**Windows:**
```powershell
# Download installer from golang.org/dl or use chocolatey
choco install golang

# Or use winget
winget install GoLang.Go
```

**macOS (Homebrew):**
```bash
brew install go
```

### Verify Go Installation

```bash
go version
go env GOPATH
go env GOROOT
```

Expected output:
```text
go version go1.20.x linux/amd64
/home/user/go
/usr/local/go
```

## IDE and Editor Setup

### Visual Studio Code (Recommended)

VS Code provides excellent Go support with the official Go extension.

**Installation:**
1. Download from [code.visualstudio.com](https://code.visualstudio.com)
2. Install the Go extension (`Go Team at Google`)

**Essential Extensions:**

| Extension | Purpose | Install Command |
|-----------|---------|----------------|
| **Go** | Go language support | `code --install-extension golang.Go` |
| **Go Test Explorer** | Test management | `code --install-extension premparihar.gotestexplorer` |
| **Error Lens** | Inline error display | `code --install-extension usernamehw.errorlens` |
| **GitLens** | Enhanced Git integration | `code --install-extension eamodio.gitlens` |
| **REST Client** | API testing | `code --install-extension humao.rest-client` |

**VS Code Configuration:**

Create `.vscode/settings.json` in your project:

```json
{
    "go.useLanguageServer": true,
    "go.toolsManagement.autoUpdate": true,
    "go.lintTool": "golangci-lint",
    "go.formatTool": "goimports",
    "go.testFlags": ["-v", "-race"],
    "go.buildFlags": ["-race"],
    "go.vetFlags": ["-all"],
    "editor.formatOnSave": true,
    "editor.codeActionsOnSave": {
        "source.organizeImports": true
    },
    "files.eol": "\n"
}
```

**Launch Configuration:**

Create `.vscode/launch.json` for debugging:

```json
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Launch Agent",
            "type": "go",
            "request": "launch",
            "mode": "auto",
            "program": "${workspaceFolder}/main.go",
            "env": {
                "CGO_ENABLED": "1"
            },
            "args": ["-version"]
        },
        {
            "name": "Debug Tests",
            "type": "go",
            "request": "launch",
            "mode": "test",
            "program": "${workspaceFolder}"
        }
    ]
}
```

### GoLand (JetBrains)

For advanced IDE features and commercial support:

1. Download from [jetbrains.com/go](https://jetbrains.com/go)
2. Configure Go SDK path
3. Enable Go modules support
4. Set up run configurations for testing and debugging

### Vim/Neovim

For terminal-based development:

**Install vim-go plugin:**
```vim
" Add to .vimrc
Plug 'fatih/vim-go', { 'do': ':GoUpdateBinaries' }
```

**Essential vim-go settings:**
```vim
let g:go_fmt_command = "goimports"
let g:go_highlight_types = 1
let g:go_highlight_fields = 1
let g:go_highlight_functions = 1
let g:go_highlight_function_calls = 1
```

## Go Development Tools

Install essential Go development tools:

```bash
# Core Go tools
go install golang.org/x/tools/cmd/goimports@latest
go install golang.org/x/tools/cmd/godoc@latest
go install golang.org/x/tools/gopls@latest

# Linting and analysis
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install honnef.co/go/tools/cmd/staticcheck@latest
go install github.com/securecodewarrior/go-crypto-hunter@latest

# Testing tools
go install github.com/onsi/ginkgo/v2/ginkgo@latest
go install gotest.tools/gotestsum@latest

# Debugging
go install github.com/go-delve/delve/cmd/dlv@latest

# Code generation
go install github.com/golang/mock/mockgen@latest
```

### Tool Configuration

**golangci-lint configuration (`.golangci.yml`):**

```yaml
linters-settings:
  govet:
    check-shadowing: true
  gocyclo:
    min-complexity: 15
  maligned:
    suggest-new: true
  dupl:
    threshold: 100
  goconst:
    min-len: 2
    min-occurrences: 2

linters:
  enable:
    - bodyclose
    - deadcode
    - depguard
    - dogsled
    - dupl
    - errcheck
    - gochecknoinits
    - goconst
    - gocyclo
    - gofmt
    - goimports
    - golint
    - gomnd
    - goprintffuncname
    - gosec
    - gosimple
    - govet
    - ineffassign
    - interfacer
    - lll
    - misspell
    - nakedret
    - rowserrcheck
    - scopelint
    - staticcheck
    - structcheck
    - stylecheck
    - typecheck
    - unconvert
    - unparam
    - unused
    - varcheck
    - whitespace

run:
  timeout: 5m
```

## Platform-Specific Development Setup

### Windows Development

**Required Tools:**
```powershell
# Install build tools
choco install mingw
choco install make

# Or install Microsoft C++ Build Tools
# Visual Studio Installer -> Individual Components -> MSVC v143 - VS 2022 C++ Build Tools
```

**CGO Requirements:**
- TDM-GCC or MinGW-w64 for CGO compilation
- Windows SDK for Windows API development

### macOS Development

**Required Tools:**
```bash
# Install Xcode Command Line Tools
xcode-select --install

# Install additional tools via Homebrew
brew install make
brew install coreutils
```

**CGO Requirements:**
- Xcode Command Line Tools provide necessary headers and libraries

### Linux Development

**Ubuntu/Debian:**
```bash
sudo apt update
sudo apt install build-essential git curl wget

# For specific distributions, additional packages may be required
sudo apt install libc6-dev
```

**CentOS/RHEL:**
```bash
sudo yum groupinstall "Development Tools"
sudo yum install git curl wget
```

## Environment Variables

Set up essential environment variables for development:

**Essential Variables:**
```bash
# Go environment
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin
export GO111MODULE=on

# CGO for cross-compilation
export CGO_ENABLED=1

# Development flags
export GODEBUG=netdns=cgo
export GOOS=linux    # or windows, darwin
export GOARCH=amd64  # or 386, arm64
```

**Add to shell profile (`.bashrc`, `.zshrc`, etc.):**
```bash
# Go development environment
export GOPATH=$HOME/go
export PATH=$PATH:/usr/local/go/bin:$GOPATH/bin
export GO111MODULE=on
export CGO_ENABLED=1

# Editor preferences  
export EDITOR=code  # or vim, nano, etc.

# Project-specific
export TACTICAL_DEV=1
```

## Verification and Testing

### Verify Development Environment

Run these commands to ensure your environment is properly configured:

```bash
# Check Go installation
go version
go env

# Check installed tools
goimports -h
golangci-lint version
dlv version

# Test build capability
cd /tmp
go mod init test
echo 'package main; import "fmt"; func main() { fmt.Println("Hello") }' > main.go
go run main.go
rm -rf /tmp/go.mod /tmp/main.go
```

### IDE Functionality Test

1. **Syntax Highlighting**: Open a Go file and verify syntax is highlighted
2. **IntelliSense**: Type `fmt.` and verify completion suggestions appear
3. **Go to Definition**: Ctrl+click on a function name to navigate to definition
4. **Format on Save**: Save a Go file and verify it's automatically formatted
5. **Error Detection**: Introduce a syntax error and verify it's highlighted

### Build Test

Test cross-platform build capabilities:

```bash
# Test building for different platforms
GOOS=linux GOARCH=amd64 go build -o test-linux main.go
GOOS=windows GOARCH=amd64 go build -o test-windows.exe main.go
GOOS=darwin GOARCH=amd64 go build -o test-darwin main.go

# Clean up test files
rm test-*
```

## Common Development Tasks

### Code Quality Workflow

```bash
# Format code
goimports -w .

# Run linter  
golangci-lint run

# Run tests with coverage
go test -v -race -coverprofile=coverage.out ./...

# View coverage
go tool cover -html=coverage.out -o coverage.html
```

### Debug Configuration

**VS Code Debug Tasks:**
```json
{
    "name": "Debug Agent Installation",
    "type": "go",
    "request": "launch",
    "mode": "auto", 
    "program": "${workspaceFolder}/main.go",
    "args": [
        "-m", "install",
        "-api", "https://localhost:8000", 
        "-client-id", "1",
        "-site-id", "1",
        "-auth", "test-token",
        "-desc", "Debug Agent"
    ],
    "env": {
        "TACTICAL_DEV": "1"
    }
}
```

## Troubleshooting Environment Issues

### Common Issues

| Problem | Solution |
|---------|----------|
| **`go: command not found`** | Add Go binary to `$PATH`, verify installation |
| **CGO compilation errors** | Install build tools (gcc, build-essential, Xcode) |
| **Module download failures** | Check `GOPROXY` and network connectivity |
| **Permission errors** | Ensure `$GOPATH` has proper write permissions |
| **Cross-compilation issues** | Install target platform toolchain |

### Reset Development Environment

If you encounter persistent issues:

```bash
# Clean Go module cache
go clean -modcache

# Reinstall Go tools
go install -a std

# Reset workspace
cd $GOPATH
rm -rf pkg/ bin/
```

Your development environment is now ready for Tactical RMM Agent development. Proceed to the [Local Development Guide](local-development.md) to clone and build the project.