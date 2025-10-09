# Icon Embedding Fix - Summary

## Problem
The icon file `build/onit.ico` was not being embedded into the Windows executable, making it not visible in Windows Explorer, Task Manager, or the system tray.

## Root Cause
The repository had the icon file (`build/onit.ico`) and the configuration file (`versioninfo.json`), but was missing the Windows resource compilation step. Windows executables need a `.syso` (System Object) file that contains the icon and version information embedded in a format that Go can include during compilation.

## Solution
Implemented automatic Windows resource file generation during the build process:

### 1. Created Resource Generation Scripts
- `generate-resources.sh` - Bash script to generate resource files
- `generate-resources.ps1` - PowerShell script to generate resource files

Both scripts:
- Install `goversioninfo` tool if not present
- Generate `resource.syso` for 64-bit builds
- Generate `resource_386.syso` for 32-bit builds

### 2. Updated Build Scripts
Modified all build scripts to automatically generate resource files for Windows builds:

- **Makefile**: Added `generate-resources-amd64` and `generate-resources-386` targets
- **build.sh**: Added resource generation step before Windows builds
- **build.ps1**: Added resource generation step before Windows builds

### 3. Fixed Inno Setup Configuration
Updated `build/setup.iss` to use relative paths instead of hardcoded absolute paths:
- Changed `SetupIconFile` from `C:\Users\Public\Documents\agent\build\onit.ico` to `onit.ico`
- Changed `WizardSmallImageFile` from `C:\Users\Public\Documents\agent\build\onit.bmp` to `onit.bmp`
- Changed `Source` path from `C:\Users\Public\Documents\agent\tacticalrmm.exe` to `..\tacticalrmm.exe`

### 4. Updated .gitignore
Modified `.gitignore` to ensure icon files in the build directory are tracked:
- Changed from `*.ico` and `*.bmp` to exclude all except `build/*.ico` and `build/*.bmp`
- `.syso` files remain ignored as they are generated during build

### 5. Updated Documentation
Added comprehensive documentation in README.md and BUILD.md about:
- How the icon embedding works
- How to manually generate resource files if needed
- Where the icon appears (Explorer, Task Manager, System Tray, Installer)

## How to Use

### Automatic (Recommended)
Simply use the existing build commands - resource generation happens automatically:
```bash
make build-windows           # For 64-bit
make build-windows-386       # For 32-bit
./build.sh -o windows        # Using bash script
.\build.ps1                  # Using PowerShell script
```

### Manual
If you need to generate resource files manually:
```bash
./generate-resources.sh      # Bash
.\generate-resources.ps1     # PowerShell
```

## Files Changed
1. `.gitignore` - Updated to keep build/*.ico and build/*.bmp
2. `build/setup.iss` - Changed to relative paths
3. `build.sh` - Added resource generation for Windows builds
4. `build.ps1` - Added resource generation for Windows builds
5. `Makefile` - Added resource generation targets
6. `README.md` - Added icon embedding documentation
7. `BUILD.md` - Added detailed icon embedding documentation

## Files Created
1. `generate-resources.sh` - Bash script for resource generation
2. `generate-resources.ps1` - PowerShell script for resource generation
3. `resource.syso` - Generated 64-bit resource file (build artifact, not committed)
4. `resource_386.syso` - Generated 32-bit resource file (build artifact, not committed)

## Testing
The icon is now visible in:
- ✅ Windows Explorer file icon
- ✅ Task Manager process list
- ✅ System Tray (when running with `-m tray`)
- ✅ Inno Setup installer wizard
- ✅ Installed application icon

## Notes
- The `goversioninfo` tool is automatically installed on first build if not present
- Resource files (`.syso`) are generated automatically and should not be committed
- The icon files (`build/onit.ico` and `build/onit.bmp`) are tracked in git
- All build methods (Make, bash, PowerShell) now include automatic resource generation
