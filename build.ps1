#!/usr/bin/env pwsh
# Build script for Tactical RMM Agent
# This script builds the agent executable for Windows

param(
    [Parameter(Mandatory=$false)]
    [ValidateSet("amd64", "386", "arm64")]
    [string]$Architecture = "amd64",
    
    [Parameter(Mandatory=$false)]
    [switch]$Help
)

function Show-Help {
    Write-Host "Tactical RMM Agent - Build Script" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "Usage: .\build.ps1 [-Architecture <arch>] [-Help]"
    Write-Host ""
    Write-Host "Parameters:"
    Write-Host "  -Architecture   Target architecture (amd64, 386, arm64). Default: amd64"
    Write-Host "  -Help          Show this help message"
    Write-Host ""
    Write-Host "Examples:"
    Write-Host "  .\build.ps1                    # Build for Windows amd64"
    Write-Host "  .\build.ps1 -Architecture 386  # Build for Windows 32-bit"
    Write-Host ""
}

if ($Help) {
    Show-Help
    exit 0
}

# Set output filename
$OutputFile = "tacticalrmm.exe"
if ($Architecture -ne "amd64") {
    $OutputFile = "tacticalrmm-$Architecture.exe"
}

Write-Host "Building Tactical RMM Agent..." -ForegroundColor Green
Write-Host "  OS: Windows" -ForegroundColor Yellow
Write-Host "  Architecture: $Architecture" -ForegroundColor Yellow
Write-Host "  Output: $OutputFile" -ForegroundColor Yellow
Write-Host ""

# Set environment variables and build
$env:CGO_ENABLED = "0"
$env:GOOS = "windows"
$env:GOARCH = $Architecture

try {
    go build -ldflags "-s -w" -o $OutputFile
    
    if ($LASTEXITCODE -eq 0) {
        Write-Host "Build completed successfully!" -ForegroundColor Green
        Write-Host "Output file: $OutputFile" -ForegroundColor Cyan
        
        # Show file size
        $fileInfo = Get-Item $OutputFile
        $fileSizeMB = [math]::Round($fileInfo.Length / 1MB, 2)
        Write-Host "File size: $fileSizeMB MB" -ForegroundColor Cyan
    } else {
        Write-Host "Build failed with exit code: $LASTEXITCODE" -ForegroundColor Red
        exit $LASTEXITCODE
    }
} catch {
    Write-Host "Error during build: $_" -ForegroundColor Red
    exit 1
}
