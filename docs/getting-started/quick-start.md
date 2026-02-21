# Quick Start Guide

Get the Tactical RMM Agent up and running in 5 minutes with this streamlined installation process.

## TL;DR Installation

For experienced administrators who want the fastest path to a working agent:

```bash
# Download and install in one command (Linux/macOS)
curl -sSL https://your-rmm-server.com/agents/rmmagent | sudo ./rmmagent -m install \
  -api "https://your-rmm-server.com" \
  -client-id 1 \
  -site-id 2 \
  -auth "your-installation-token" \
  -desc "Quick Start Server"
```

```powershell
# Download and install (Windows - Run as Administrator)
Invoke-WebRequest -Uri "https://your-rmm-server.com/agents/rmmagent.exe" -OutFile "rmmagent.exe"
.\rmmagent.exe -m install -api "https://your-rmm-server.com" -client-id 1 -site-id 2 -auth "your-token" -desc "Quick Start Server"
```

## Step-by-Step Installation

### Step 1: Download the Agent

**Option A: Direct Download**
Download the appropriate binary for your platform from your Tactical RMM server's agent download page.

**Option B: Command Line Download**

```bash
# Linux/macOS
wget https://your-rmm-server.com/agents/rmmagent
# or
curl -O https://your-rmm-server.com/agents/rmmagent
```

```powershell
# Windows (PowerShell)
Invoke-WebRequest -Uri "https://your-rmm-server.com/agents/rmmagent.exe" -OutFile "rmmagent.exe"
```

### Step 2: Make Executable (Linux/macOS only)

```bash
chmod +x rmmagent
```

### Step 3: Install the Agent

Run the installation command with your server-specific parameters:

**Linux/macOS:**
```bash
sudo ./rmmagent -m install \
  -api "https://your-rmm-server.com" \
  -client-id 1 \
  -site-id 2 \
  -auth "your-installation-token" \
  -desc "Production Server" \
  -agent-type "server"
```

**Windows (Run as Administrator):**
```powershell
.\rmmagent.exe -m install -api "https://your-rmm-server.com" -client-id 1 -site-id 2 -auth "your-installation-token" -desc "Production Server" -agent-type "server"
```

### Step 4: Verify Installation

Check if the service is running:

**Linux (systemd):**
```bash
systemctl status tacticalrmm
```

**macOS:**
```bash
sudo launchctl list | grep tacticalrmm
```

**Windows:**
```powershell
Get-Service -Name "Tactical RMM Agent"
```

### Step 5: Verify Communication

Check the agent logs to confirm successful communication:

**Linux/macOS:**
```bash
# View recent log entries
sudo tail -f /opt/tacticalagent/logs/agent.log
```

**Windows:**
```powershell
# View Windows Event Log entries
Get-EventLog -LogName Application -Source "Tactical RMM Agent" -Newest 10
```

## Installation Parameters Reference

| Parameter | Required | Description | Example |
|-----------|----------|-------------|---------|
| `-m install` | ✅ | Installation mode | `install` |
| `-api` | ✅ | Server API URL | `"https://rmm.company.com"` |
| `-client-id` | ✅ | Organization ID | `1` |
| `-site-id` | ✅ | Site ID | `2` |
| `-auth` | ✅ | Installation token | `"abc123def456..."` |
| `-desc` | ✅ | Device description | `"Web Server 01"` |
| `-agent-type` | ❌ | Agent type | `"server"` or `"workstation"` |
| `-timeout` | ❌ | Installation timeout | `900` (seconds) |
| `-silent` | ❌ | Silent installation | No value needed |
| `-power` | ❌ | Disable sleep/hibernate | No value needed |
| `-rdp` | ❌ | Enable RDP | No value needed |
| `-ping` | ❌ | Enable ping | No value needed |

## Expected Output

During successful installation, you should see output similar to:

```text
Tactical RMM Agent v2.9.0
Installing agent...
Creating service...
Starting service...
Agent installation completed successfully
Service running: OK
Communication test: OK
```

## Verification Checklist

After installation, verify these indicators of successful deployment:

- [ ] Service is running and enabled for auto-start
- [ ] Agent appears in your Tactical RMM web console
- [ ] System information is being collected (CPU, memory, disk)
- [ ] Network connectivity check shows "Online" status
- [ ] Basic system checks are executing successfully

## Common Installation Options

### Silent Installation
For automated deployments without user interaction:

```bash
./rmmagent -m install -api "https://server.com" -client-id 1 -site-id 2 -auth "token" -desc "Auto Deploy" -silent
```

### Server vs Workstation
Specify the agent type for appropriate monitoring profiles:

```bash
# For servers (default)
-agent-type "server"

# For desktop/laptop systems
-agent-type "workstation"
```

### Enable Additional Features
```bash
# Enable RDP, ping, and disable power management
./rmmagent -m install -api "https://server.com" -client-id 1 -site-id 2 -auth "token" -desc "Server" -rdp -ping -power
```

## Troubleshooting Quick Fixes

### Installation Failed

**Check 1: Privileges**
```bash
# Linux/macOS: Ensure running as root/sudo
whoami

# Windows: Check Administrator status
net session
```

**Check 2: Network Connectivity**
```bash
# Test server connectivity
curl -I https://your-rmm-server.com

# Test NATS port
nc -zv your-rmm-server.com 4222
```

**Check 3: Parameters**
- Verify API URL is correct and accessible
- Confirm client-id and site-id are valid numbers
- Check that installation token hasn't expired

### Service Won't Start

```bash
# Linux: Check systemd status
systemctl status tacticalrmm
journalctl -u tacticalrmm -n 20

# macOS: Check launchd status  
sudo launchctl load /Library/LaunchDaemons/tacticalrmm.plist

# Windows: Check service status
Get-Service "Tactical RMM Agent" | Format-List *
```

### Agent Not Appearing in Console

1. Wait 2-3 minutes for initial check-in
2. Check server logs for connection attempts
3. Verify firewall isn't blocking outbound connections
4. Confirm installation token is valid and not expired

## Next Steps

After successful installation:

1. **[First Steps Guide](first-steps.md)** - Configure and explore key features
2. Check your Tactical RMM web console to see the new agent
3. Review system information and initial health checks
4. Set up automated tasks and monitoring policies

## Advanced Installation

For complex environments requiring:
- Custom NATS ports
- Proxy configuration  
- OpenFrame integration
- Custom certificate authorities

Refer to the development documentation for detailed configuration options.