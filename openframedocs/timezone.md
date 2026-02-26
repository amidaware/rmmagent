# Timezone Support

## Overview

In Openframe mode, the agent sends timezone information to the backend as part of the `agent-agentinfo` check-in message (runs every ~15 minutes).

## Implementation

The `agent-agentinfo` message is extended with a `timezone` field containing the IANA timezone identifier.

### Example payload

```json
{
  "agent_id": "xxx",
  "logged_in_username": "user",
  "hostname": "workstation-1",
  "operating_system": "Microsoft Windows 11 Pro",
  "plat": "windows",
  "total_ram": 16.0,
  "boot_time": 1708934400,
  "needs_reboot": false,
  "goarch": "amd64",
  "timezone": "Europe/Kyiv"
}
```

### Timezone format

Uses IANA timezone identifiers (cross-platform):
- `Europe/Kyiv`
- `America/New_York`
- `Asia/Tokyo`
- `UTC`

### Library

Uses `github.com/thlib/go-timezone-local/tzlocal` which:
- On Windows: reads from registry and converts to IANA format
- On macOS/Linux: reads from `/etc/localtime` or `TZ` environment variable

## Files changed

- `agent/checkin.go` - Added `AgentInfoOpenframe` struct and timezone logic
