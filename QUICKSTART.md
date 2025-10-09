# Quick Start: Compile an EXE

This is the fastest way to build a Windows executable for the Tactical RMM Agent.

## Prerequisites

You need [Go](https://golang.org/dl/) installed on your system. That's it!

## Build Windows EXE - Three Easy Ways

### 1. Using Make (Easiest)

```bash
make build-windows
```

Your executable will be: `tacticalrmm.exe`

### 2. Using PowerShell (Windows)

```powershell
.\build.ps1
```

Your executable will be: `tacticalrmm.exe`

### 3. Using Bash (Linux/macOS/WSL)

```bash
./build.sh -o windows
```

Your executable will be: `tacticalrmm.exe`

### 4. Manual Command (Any Platform)

```bash
env CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o tacticalrmm.exe
```

## That's It!

After running any of the above commands, you'll have `tacticalrmm.exe` (approximately 11MB) ready to use.

## Need Other Platforms?

| Platform | Command |
|----------|---------|
| Windows 32-bit | `make build-windows-386` |
| Linux 64-bit | `make build-linux` |
| macOS Intel | `make build-darwin` |
| macOS Apple Silicon | `make build-darwin-arm64` |
| All platforms | `make build-all` |

## Troubleshooting

**"make: command not found"** → Use one of the script methods (`build.ps1` or `build.sh`)

**"go: command not found"** → Install Go from https://golang.org/dl/

**Need more help?** → See [BUILD.md](BUILD.md) for detailed documentation

---

**Questions?** Check out the full documentation in [BUILD.md](BUILD.md)
