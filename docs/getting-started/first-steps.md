# First Steps After Installation

Congratulations! Your Tactical RMM Agent is now installed and running. This guide covers the first 5 essential steps to configure, verify, and begin using your new agent effectively.

## Step 1: Verify Agent Status and Communication

### Check Service Status

First, confirm the agent service is running properly:

**Linux (systemd):**
```bash
# Check service status
systemctl status tacticalrmm

# View recent logs
journalctl -u tacticalrmm -f --lines=20
```

**macOS:**
```bash
# Check if service is loaded
sudo launchctl list | grep tacticalrmm

# View log files
tail -f /opt/tacticalagent/logs/agent.log
```

**Windows:**
```powershell
# Check service status
Get-Service -Name "Tactical RMM Agent" | Format-List

# View recent event logs
Get-EventLog -LogName Application -Source "Tactical RMM Agent" -Newest 20
```

### Verify Web Console Connection

1. Log into your Tactical RMM web console
2. Navigate to **Agents** or **Devices**
3. Look for your newly installed agent (it should appear within 2-3 minutes)
4. Check that the status shows **Online** with a green indicator

### Check Communication Health

The agent performs automatic check-ins every 35-120 seconds. Verify communication is working:

```text
Expected log entries:
- "Checkin successful"
- "Connected to NATS"
- "System info collected"
- "Checks completed"
```

## Step 2: Review Initial System Information

Once connected, the agent automatically collects comprehensive system information:

### System Metrics Dashboard

In your web console, review the automatically collected data:

| Category | Information Collected |
|----------|-----------------------|
| **Hardware** | CPU model, cores, memory capacity, disk drives |
| **Operating System** | OS version, architecture, hostname, domain |
| **Network** | IP addresses, network adapters, connectivity status |
| **Software** | Installed programs, Windows updates, services |
| **Performance** | CPU usage, memory usage, disk space utilization |

### Verify Data Accuracy

Compare the displayed information with your system:

**Check CPU and Memory:**
```bash
# Linux/macOS
lscpu
free -h
```

```powershell
# Windows  
Get-WmiObject -Class Win32_Processor | Select-Object Name,NumberOfCores
Get-WmiObject -Class Win32_ComputerSystem | Select-Object TotalPhysicalMemory
```

**Check Disk Space:**
```bash
# Linux/macOS
df -h
```

```powershell
# Windows
Get-WmiObject -Class Win32_LogicalDisk | Select-Object DeviceID,Size,FreeSpace
```

## Step 3: Configure Basic Monitoring and Checks

### Enable Standard Health Checks

The agent comes with built-in health checks. Enable and configure the essential ones:

#### Disk Space Monitoring
- **Threshold**: Set alerts for disk usage > 85%
- **Frequency**: Check every 15 minutes
- **Action**: Alert administrators when exceeded

#### CPU and Memory Monitoring  
- **CPU Threshold**: Alert when usage > 90% for 5+ minutes
- **Memory Threshold**: Alert when usage > 95%
- **Frequency**: Monitor every 5 minutes

#### Service Monitoring
Monitor critical services specific to your system:

**Windows Examples:**
```text
- Windows Update (wuauserv)
- Windows Defender (WinDefend)  
- Remote Desktop (TermService)
- DNS Client (Dnscache)
```

**Linux Examples:**
```text
- SSH daemon (sshd)
- Network Manager (NetworkManager)
- System logging (rsyslog)
- Cron daemon (crond)
```

### Test Check Execution

Manually run a check to verify functionality:

**From Web Console:**
1. Navigate to your agent
2. Go to **Checks** tab
3. Click **Run All Checks**
4. Verify results appear within 1-2 minutes

## Step 4: Test Remote Command Execution

One of the most powerful features is remote command execution. Test this capability safely:

### Basic Command Tests

**Safe read-only commands to test:**

```bash
# Linux/macOS test commands
whoami
pwd
ls -la /tmp
df -h
ps aux | head -10
```

```powershell
# Windows test commands
whoami
Get-Date
Get-Location
Get-Process | Select-Object -First 10
Get-Service | Where-Object {$_.Status -eq "Running"} | Select-Object -First 5
```

### Execute Commands via Web Console

1. Navigate to your agent in the web console
2. Find the **Run Command** or **Scripts** section
3. Enter a test command from above
4. Execute and verify output appears correctly
5. Check execution time and exit codes

### Script Execution Test

Test script execution capabilities:

**PowerShell Script Test (Windows):**
```powershell
# Simple system info script
Get-ComputerInfo | Select-Object WindowsProductName, WindowsVersion, TotalPhysicalMemory
```

**Bash Script Test (Linux/macOS):**
```bash
#!/bin/bash
echo "System Information:"
echo "Hostname: $(hostname)"
echo "Uptime: $(uptime)"
echo "Disk Usage:"
df -h / | tail -1
```

## Step 5: Explore Key Management Features

### Software Inventory

The agent automatically inventories installed software:

1. Check the **Software** tab in your web console
2. Verify installed applications are listed correctly
3. Note version numbers and installation dates
4. Review for any unexpected or unauthorized software

### Windows Update Management (Windows Only)

For Windows systems, verify update management:

1. Navigate to **Windows Updates** section
2. Check for pending updates
3. Review update history
4. Test update installation (if appropriate for your environment)

### File Management and Transfers

Test file management capabilities:

**Safe File Operations:**
- List directory contents: `/tmp` (Linux/macOS) or `C:\Temp` (Windows)
- Check file permissions and ownership
- View file contents (for configuration files)

> **Caution**: Only perform file operations you understand. Always backup important files before making changes.

## Initial Configuration Checklist

Complete these configuration tasks within your first week:

- [ ] Verified agent appears online in web console
- [ ] Reviewed and validated system information accuracy  
- [ ] Configured disk space monitoring thresholds
- [ ] Set up CPU and memory alerting
- [ ] Enabled critical service monitoring
- [ ] Tested remote command execution
- [ ] Verified software inventory accuracy
- [ ] Set up basic automated maintenance tasks
- [ ] Configured appropriate alert recipients
- [ ] Documented agent configuration for team reference

## Common Initial Tasks

### Set Agent-Specific Policies

Based on your agent type (server vs workstation), configure appropriate policies:

**Server Agents:**
- More frequent monitoring (every 5 minutes)
- Lower resource usage thresholds  
- Critical service monitoring
- Automated security updates
- Performance baseline establishment

**Workstation Agents:**
- Standard monitoring intervals (every 15 minutes)
- User-friendly maintenance schedules
- Software deployment capabilities
- Remote support tools configuration

### Create Maintenance Windows

Set up maintenance schedules for:
- **Updates**: Automatically install updates during off-hours
- **Reboots**: Schedule restart windows for update application  
- **Cleanups**: Disk cleanup and temporary file removal
- **Backups**: If agent handles backup verification

## Troubleshooting First-Time Issues

### Agent Shows Offline

**Check network connectivity:**
```bash
# Test server connection
curl -I https://your-rmm-server.com
nc -zv your-rmm-server.com 4222
```

**Restart the service:**
```bash
# Linux
sudo systemctl restart tacticalrmm

# macOS  
sudo launchctl unload /Library/LaunchDaemons/tacticalrmm.plist
sudo launchctl load /Library/LaunchDaemons/tacticalrmm.plist

# Windows
Restart-Service "Tactical RMM Agent"
```

### Commands Not Executing

1. Verify agent is online and communicating
2. Check command syntax for platform compatibility
3. Ensure adequate permissions for command execution
4. Review agent logs for error messages

### Missing System Information

1. Wait 5-10 minutes for complete data collection
2. Check agent logs for collection errors
3. Verify agent has appropriate system permissions
4. Manually trigger a full system scan if available

## Next Steps

After completing these first steps:

1. **Explore Advanced Features**: Learn about script deployment, automated patching, and custom checks
2. **Set Up Monitoring Policies**: Create comprehensive monitoring rules for your environment
3. **Configure Alert Notifications**: Set up email, SMS, or webhook notifications for critical events
4. **Plan Maintenance Schedules**: Establish regular maintenance windows and automated tasks
5. **Team Training**: Ensure team members understand how to use the web console effectively

## Getting Help

If you encounter issues during these first steps:

- Review agent log files for specific error messages
- Check server connectivity and firewall settings  
- Verify installation parameters were correct
- Consult with your Tactical RMM server administrator
- Reference the development documentation for advanced troubleshooting

Your Tactical RMM Agent is now ready for production use. Regular monitoring and maintenance will ensure optimal performance and security for your managed systems.