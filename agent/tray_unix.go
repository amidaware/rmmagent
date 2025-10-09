//go:build !windows
// +build !windows

/*
Copyright 2023 AmidaWare Inc.

Licensed under the Tactical RMM License Version 1.0 (the "License").
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

// StartTrayIcon is a stub for non-Windows platforms
func StartTrayIcon(agent *Agent, version string) {
	// Tray icon is only supported on Windows
}

// QuitTray is a stub for non-Windows platforms
func QuitTray() {
	// No-op on non-Windows platforms
}
