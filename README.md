### Tactical RMM Agent
https://github.com/amidaware/tacticalrmm

## Building the Agent

There are multiple ways to build the Tactical RMM Agent executable:

> 📖 **For detailed build instructions, see [BUILD.md](BUILD.md)**

### Quick Start - Build for Your Platform

**Using Make (recommended):**
```bash
make build-windows    # Build for Windows (amd64)
make build-linux      # Build for Linux (amd64)
make build-darwin     # Build for macOS (amd64)
make build-all        # Build for all platforms
make help             # Show all available targets
```

**Using PowerShell (Windows):**
```powershell
.\build.ps1                    # Build for Windows (amd64)
.\build.ps1 -Architecture 386  # Build for Windows 32-bit
.\build.ps1 -Help              # Show help
```

**Using Bash (Linux/macOS):**
```bash
./build.sh                              # Build for current OS (amd64)
./build.sh -o windows                   # Build for Windows (amd64)
./build.sh -o linux -a arm64            # Build for Linux ARM64
./build.sh -o darwin -a arm64           # Build for macOS Apple Silicon
./build.sh -h                           # Show help
```

### Manual Build

If you prefer to build manually:
```bash
env CGO_ENABLED=0 GOOS=<GOOS> GOARCH=<GOARCH> go build -ldflags "-s -w"
```

Examples:
```bash
# Windows (64-bit)
env CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o tacticalrmm.exe

# Linux (64-bit)
env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o tacticalrmm

# macOS (Apple Silicon)
env CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w" -o tacticalrmm
```

### Supported Platforms

- **Windows:** amd64, 386, arm64
- **Linux:** amd64, 386, arm64, arm
- **macOS:** amd64, arm64

#### Windows System Tray Icon
The agent now includes a Windows system tray icon that provides quick access to agent status and information.

To run the tray icon:
```
tacticalrmm.exe -m tray
```

The tray icon provides the following features:
- **Show Status**: Displays the current status of the Tactical RMM Agent and Mesh Agent services
- **About**: Shows version and platform information
- **Exit**: Closes the tray icon (the agent service continues to run in the background)

**Note**: The tray icon runs independently from the agent service. The service runs in the background, and the tray icon is just a UI element for user convenience.


