//go:build !windows
// +build !windows

package tray

import (
	"fmt"
)

func InitTrayIcon() {
	fmt.Println("This feature is only supported on windows at this time.")
}
