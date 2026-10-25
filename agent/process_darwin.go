//go:build darwin
// +build darwin

/*
Copyright 2026 Amidaware Inc.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

import (
	"bytes"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/elastic/go-sysinfo/types"
)

// from <sys/proc_info.h>
const (
	procPIDTaskAllInfo   = 2
	procPIDVnodePathInfo = 9
	vnodePathInfoSize    = 2352 // sizeof(struct proc_vnodepathinfo)
)

// struct proc_bsdinfo
type procBSDInfo struct {
	Flags     uint32
	Status    uint32
	Xstatus   uint32
	Pid       uint32
	Ppid      uint32
	UID       uint32
	GID       uint32
	RUID      uint32
	RGID      uint32
	SvUID     uint32
	SvGID     uint32
	Rfu1      uint32
	Comm      [16]byte
	Name      [32]byte
	Nfiles    uint32
	Pgid      uint32
	Pjobc     uint32
	Tdev      uint32
	Tpgid     uint32
	Nice      int32
	StartSec  uint64
	StartUsec uint64
}

// struct proc_taskinfo
type procTaskInfo struct {
	VirtualSize   uint64
	ResidentSize  uint64
	TotalUser     uint64
	TotalSystem   uint64
	ThreadsUser   uint64
	ThreadsSystem uint64
	Policy        int32
	Faults        int32
	Pageins       int32
	CowFaults     int32
	MsgSent       int32
	MsgReceived   int32
	SyscallsMach  int32
	SyscallsUnix  int32
	Csw           int32
	Threadnum     int32
	Numrunning    int32
	Priority      int32
}

// struct proc_taskallinfo
type procTaskAllInfo struct {
	Bsd  procBSDInfo
	Task procTaskInfo
}

var (
	procPidinfoOnce sync.Once
	procPidinfo     func(pid int32, flavor int32, arg uint64, buf unsafe.Pointer, size int32) int32
)

func loadProcPidinfo() bool {
	procPidinfoOnce.Do(func() {
		lib, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err != nil {
			return
		}
		purego.RegisterLibFunc(&procPidinfo, lib, "proc_pidinfo")
	})
	return procPidinfo != nil
}

func procNameAndMemory(process types.Process, info types.ProcessInfo) (string, uint64, bool) {
	if !loadProcPidinfo() {
		m, _ := process.Memory()
		return info.Name, m.Resident, true
	}

	pid := int32(info.PID)
	var t procTaskAllInfo
	size := int32(unsafe.Sizeof(t))
	if procPidinfo(pid, procPIDTaskAllInfo, 0, unsafe.Pointer(&t), size) != size {
		return "", 0, false
	}

	var vnode [vnodePathInfoSize]byte
	if procPidinfo(pid, procPIDVnodePathInfo, 0, unsafe.Pointer(&vnode[0]), vnodePathInfoSize) != vnodePathInfoSize {
		return "", 0, false
	}

	name := t.Bsd.Name[:]
	if i := bytes.IndexByte(name, 0); i >= 0 {
		name = name[:i]
	}
	return string(name), t.Task.ResidentSize, true
}
