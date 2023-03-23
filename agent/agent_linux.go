/*
Copyright 2022 AmidaWare LLC.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

import (
	"os"
	"syscall"
)

func tmpNoExec() bool {
	var stat syscall.Statfs_t
	var noexec bool

	tmpdir := os.TempDir()
	if err := syscall.Statfs(tmpdir, &stat); err == nil {
		if stat.Flags&syscall.MS_NOEXEC != 0 {
			noexec = true
		}
	}
	return noexec
}
