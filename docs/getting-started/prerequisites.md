# Prerequisites

Before installing the Tactical RMM Agent, ensure your system meets the requirements and you have the necessary information from your Tactical RMM server administrator.

## System Requirements

### Minimum Hardware Requirements

| Component | Requirement |
|-----------|-------------|
| **CPU** | 1 core, x86_64 or arm64 architecture |
| **RAM** | 512 MB available memory |
| **Disk Space** | 100 MB free space for agent and dependencies |
| **Network** | Persistent internet connection with outbound access |

### Supported Operating Systems

| Platform | Versions | Architecture |
|----------|----------|--------------|
| **Windows** | Windows 7 SP1, 8.1, 10, 11, Server 2008 R2+, Server 2012+, Server 2016+, Server 2019+, Server 2022+ | x86_64, x86 (386) |
| **macOS** | macOS 10.13 High Sierra and later | x86_64, arm64 (Apple Silicon) |
| **Linux** | Ubuntu 16.04+, RHEL/CentOS 7+, Debian 9+, SUSE 12+, Alpine 3.8+ | x86_64, arm64 |

### Network Requirements

The agent requires outbound network access to communicate with your Tactical RMM server:

| Protocol | Port | Purpose | Required |
|----------|------|---------|----------|
| **HTTPS** | 443 (default) | REST API communication | ✅ Yes |
| **NATS** | 4222 (default) | Real-time messaging | ✅ Yes |
| **HTTP** | 80 | Optional for non-HTTPS setups | ❌ Optional |
| **Custom** | Variable | Custom NATS port configuration | ❌ Optional |

> **Note**: The agent only makes outbound connections. No inbound firewall rules are required.

## Required Information

Before installation, obtain the following information from your Tactical RMM server administrator:

### Server Configuration

| Parameter | Description | Example |
|-----------|-------------|---------|
| **API URL** | Base URL of your Tactical RMM server | `https://rmm.yourcompany.com` |
| **Client ID** | Numeric identifier for your organization | `1` |
| **Site ID** | Numeric identifier for the device's site | `2` |
| **Authentication Token** | Installation token provided by server admin | `abc123def456...` |
| **Agent Description** | Human-readable name for this device | `"Production Web Server"` |
| **Agent Type** | Either "server" or "workstation" | `server` |

### Optional Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| **NATS Port** | Custom NATS messaging port | `4222` |
| **Proxy Settings** | HTTP proxy configuration if required | None |
| **Mesh Integration** | Path to MeshCentral agent (if used) | Auto-detected |
| **Custom CA Certificate** | Path to custom domain CA .pem file | System certificates |

## User Privileges

The installation process requires different privilege levels depending on your platform:

### Windows
- **Administrator** privileges required for:
  - Service installation and registration
  - Registry modifications
  - System directory access
  - Windows Update integration

### macOS
- **sudo** privileges required for:
  - Service (launchd) registration
  - System directory access
  - Security framework integration

### Linux
- **root** privileges required for:
  - systemd service installation
  - System directory access
  - Package management integration

## Environment Variables

The following environment variables can be set for advanced configuration:

| Variable | Purpose | Example |
|----------|---------|---------|
| `` `$TMPDIR` `` | Temporary file storage location | `/tmp` (Linux/macOS) |
| `` `$TEMP` `` | Temporary file storage location | `C:\Windows\Temp` (Windows) |
| `` `$HOME` `` | User home directory | `/home/user` or `C:\Users\user` |
| `` `$PATH` `` | System PATH for command execution | System-dependent |

## Software Dependencies

The agent handles most dependencies automatically, but certain features require:

### Windows-Specific
- **PowerShell** 2.0+ (included with modern Windows)
- **Windows Management Instrumentation (WMI)** (system component)
- **Windows Update Agent** (for patch management)

### macOS-Specific
- **bash** shell (system default)
- **System Integrity Protection (SIP)** considerations for system modifications
- **Xcode Command Line Tools** (for certain development-related checks)

### Linux-Specific
- **systemd** (for service management on supported distributions)
- **bash** shell
- **Package managers** (apt, yum, dnf, zypper) for software inventory

## Verification Commands

Use these commands to verify your system meets the prerequisites:

### Check System Information

```bash
# Check OS version and architecture
uname -a

# Check available disk space
df -h /

# Check available memory
free -h
```

```powershell
# Windows: Check system information
systeminfo

# Check available disk space
Get-WmiObject -Class Win32_LogicalDisk | Select-Object Size,FreeSpace,DeviceID

# Check available memory
Get-WmiObject -Class Win32_ComputerSystem | Select-Object TotalPhysicalMemory
```

### Test Network Connectivity

```bash
# Test HTTPS connectivity to your server
curl -I https://your-rmm-server.com/api/v3/

# Test NATS port (if different from 4222)
nc -zv your-rmm-server.com 4222
```

### Verify Privileges

```bash
# Linux/macOS: Check if running as root or can use sudo
id
sudo -v
```

```powershell
# Windows: Check if running as Administrator
net session
```

## OpenFrame Mode Prerequisites

If your organization uses OpenFrame integration for enhanced security:

| Requirement | Description |
|-------------|-------------|
| **OpenFrame Token** | Pre-shared token for secure communication |
| **Token File Path** | Secure location for storing the OpenFrame token |
| **Connection Manager** | Access to OpenFrame connection management endpoint |

## Pre-Installation Checklist

Before proceeding to installation, verify you have:

- [ ] Confirmed system meets minimum hardware requirements
- [ ] Verified operating system compatibility
- [ ] Obtained all required server configuration parameters
- [ ] Tested network connectivity to Tactical RMM server
- [ ] Confirmed appropriate user privileges
- [ ] Prepared installation command with all required parameters
- [ ] Reviewed security and firewall considerations

## Troubleshooting Prerequisites

### Common Issues

| Issue | Solution |
|-------|----------|
| **Network blocked** | Check firewall rules for outbound HTTPS/NATS access |
| **Insufficient privileges** | Run installation as Administrator/root |
| **Invalid token** | Verify token with server administrator |
| **DNS resolution** | Ensure server hostname resolves correctly |
| **Proxy issues** | Configure proxy settings if required |

## What's Next?

Once you've verified all prerequisites:

1. **[Quick Start Guide](quick-start.md)** - Begin the installation process
2. **[First Steps](first-steps.md)** - Post-installation configuration and verification

If you encounter any issues with prerequisites, consult your system administrator or Tactical RMM server operator for assistance.