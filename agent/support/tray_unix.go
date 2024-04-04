//go:build !windows
// +build !windows

package support

import (
	"fmt"
)

func InitTrayIcon() {
	fmt.Println("This feature is only supported on windows at this time.")
}