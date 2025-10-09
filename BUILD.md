# Building Tactical RMM Agent

This guide provides comprehensive instructions for building the Tactical RMM Agent executable from source.

## Prerequisites

- [Go](https://golang.org/dl/) (version 1.20 or later)
- Git (for cloning the repository)
- Make (optional, for using Makefile)

## Quick Start

The easiest way to build the agent is to use one of the provided build scripts:

### Option 1: Using Makefile (Recommended)

```bash
# Build for Windows (64-bit)
make build-windows

# Build for Linux (64-bit)
make build-linux

# Build for macOS (64-bit Intel)
make build-darwin

# Build for all major platforms
make build-all

# See all available options
make help

# Clean built executables
make clean
```

### Option 2: Using PowerShell Script (Windows Users)

```powershell
# Build for Windows 64-bit (default)
.\build.ps1

# Build for Windows 32-bit
.\build.ps1 -Architecture 386

# Show help
.\build.ps1 -Help
```

### Option 3: Using Bash Script (Linux/macOS/WSL Users)

```bash
# Build for your current platform
./build.sh

# Build for Windows 64-bit
./build.sh -o windows

# Build for Linux ARM64
./build.sh -o linux -a arm64

# Build for macOS Apple Silicon
./build.sh -o darwin -a arm64

# Show help
./build.sh -h
```

### Option 4: Manual Build

For maximum control, you can build manually using the Go toolchain:

```bash
# General syntax
env CGO_ENABLED=0 GOOS=<os> GOARCH=<arch> go build -ldflags "-s -w" -o <output>

# Examples:
# Windows 64-bit
env CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o tacticalrmm.exe

# Windows 32-bit
env CGO_ENABLED=0 GOOS=windows GOARCH=386 go build -ldflags "-s -w" -o tacticalrmm-386.exe

# Linux 64-bit
env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o tacticalrmm-linux-amd64

# Linux ARM64
env CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "-s -w" -o tacticalrmm-linux-arm64

# macOS Intel
env CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o tacticalrmm-darwin-amd64

# macOS Apple Silicon
env CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w" -o tacticalrmm-darwin-arm64
```

## Supported Platforms and Architectures

| OS      | Architecture | Build Command                  | Output Filename              |
|---------|--------------|--------------------------------|------------------------------|
| Windows | amd64 (64-bit) | `make build-windows`         | `tacticalrmm.exe`            |
| Windows | 386 (32-bit)   | `make build-windows-386`     | `tacticalrmm-386.exe`        |
| Linux   | amd64 (64-bit) | `make build-linux`           | `tacticalrmm-linux-amd64`    |
| Linux   | arm64          | `make build-linux-arm64`     | `tacticalrmm-linux-arm64`    |
| macOS   | amd64 (Intel)  | `make build-darwin`          | `tacticalrmm-darwin-amd64`   |
| macOS   | arm64 (M1/M2)  | `make build-darwin-arm64`    | `tacticalrmm-darwin-arm64`   |

## Build Flags Explained

- `CGO_ENABLED=0`: Disables CGO for a fully static binary (no C dependencies)
- `GOOS`: Target operating system (windows, linux, darwin)
- `GOARCH`: Target architecture (amd64, 386, arm64, arm)
- `-ldflags "-s -w"`: Strips debugging information to reduce binary size
  - `-s`: Omit the symbol table and debug information
  - `-w`: Omit the DWARF symbol table

## Cross-Compilation

Go makes it easy to build executables for different platforms from any host platform. For example, you can build a Windows executable from a Linux or macOS machine:

```bash
# Build Windows executable from Linux/macOS
env CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o tacticalrmm.exe

# Build Linux executable from Windows
$env:CGO_ENABLED="0"; $env:GOOS="linux"; $env:GOARCH="amd64"; go build -ldflags "-s -w" -o tacticalrmm-linux-amd64
```

## Output Files

After building, you'll find the executable in the root directory of the project:

- Windows: `tacticalrmm.exe` (or `tacticalrmm-386.exe` for 32-bit)
- Linux: `tacticalrmm-linux-amd64` (or other variants)
- macOS: `tacticalrmm-darwin-amd64` (or `tacticalrmm-darwin-arm64` for Apple Silicon)

## Troubleshooting

### Build Fails with "command not found: go"

Ensure Go is installed and in your PATH:
```bash
go version
```

If not installed, download from https://golang.org/dl/

### Build Fails with Module Errors

Try cleaning the Go module cache:
```bash
go clean -modcache
go mod download
```

### Permission Denied on build.sh

Make the script executable:
```bash
chmod +x build.sh
```

### Windows: PowerShell Execution Policy

If you get an execution policy error when running build.ps1:
```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
```

## Creating an Installer (Windows)

After building the Windows executable, you can create an installer using Inno Setup:

1. Build the agent: `make build-windows`
2. Place the executable in the expected location (see `build/setup.iss`)
3. Run Inno Setup with the `build/setup.iss` script

## Running the Built Agent

After building, you can run the agent with various modes:

```bash
# Show version
./tacticalrmm -version

# Show help
./tacticalrmm -h

# Run in service mode (Windows)
tacticalrmm.exe -m svc

# Run system tray icon (Windows)
tacticalrmm.exe -m tray
```

## Development Builds

For development, you may want to build without the optimization flags to include debugging information:

```bash
# Development build (with debug info)
go build -o tacticalrmm

# Production build (optimized, smaller size)
go build -ldflags "-s -w" -o tacticalrmm
```

## Advanced: Custom Build Tags

You can use build tags for conditional compilation:

```bash
# Example with custom tags
go build -tags "dev debug" -o tacticalrmm
```

## Contributing

When contributing code, please ensure your changes build successfully for all supported platforms:

```bash
make build-all
```

This will verify that your code compiles for Windows, Linux, and macOS on all supported architectures.
