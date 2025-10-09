#!/usr/bin/env pwsh
# Generate Windows resource files with icon embedded
# This script should be run before building the Windows executable

Write-Host "Generating Windows resource files..." -ForegroundColor Cyan

# Check if goversioninfo is installed
$goversioninfo = Get-Command goversioninfo -ErrorAction SilentlyContinue
if (-not $goversioninfo) {
    Write-Host "Installing goversioninfo..." -ForegroundColor Yellow
    go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest
}

# Generate 64-bit resource file
Write-Host "Generating resource.syso (64-bit)..." -ForegroundColor Green
& goversioninfo -64 versioninfo.json

# Generate 32-bit resource file
Write-Host "Generating resource_386.syso (32-bit)..." -ForegroundColor Green
& goversioninfo -o resource_386.syso versioninfo.json

Write-Host "Resource files generated successfully!" -ForegroundColor Green
Write-Host "  - resource.syso (amd64)" -ForegroundColor Cyan
Write-Host "  - resource_386.syso (386)" -ForegroundColor Cyan
