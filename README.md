### Tactical RMM Agent
https://github.com/amidaware/tacticalrmm

#### building the agent
```
env CGO_ENABLED=0 GOOS=<GOOS> GOARCH=<GOARCH> go build -ldflags "-s -w"
```

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


