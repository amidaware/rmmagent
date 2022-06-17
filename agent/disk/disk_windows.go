package disk

import (
	"unsafe"

	"github.com/amidaware/rmmagent/agent/utils"
	"github.com/shirou/gopsutil/disk"
	trmm "github.com/wh1te909/trmm-shared"
	"golang.org/x/sys/windows"
)

var (
	getDriveType = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetDriveTypeW")
)

// GetDisks returns a list of fixed disks
func GetDisks() []trmm.Disk {
	ret := make([]trmm.Disk, 0)
	partitions, err := disk.Partitions(false)
	if err != nil {
		//a.Logger.Debugln(err)
		return ret
	}

	for _, p := range partitions {
		typepath, _ := windows.UTF16PtrFromString(p.Device)
		typeval, _, _ := getDriveType.Call(uintptr(unsafe.Pointer(typepath)))
		// https://docs.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getdrivetypea
		if typeval != 3 {
			continue
		}

		usage, err := disk.Usage(p.Mountpoint)
		if err != nil {
			//a.Logger.Debugln(err)
			continue
		}

		d := trmm.Disk{
			Device:  p.Device,
			Fstype:  p.Fstype,
			Total:   utils.ByteCountSI(usage.Total),
			Used:    utils.ByteCountSI(usage.Used),
			Free:    utils.ByteCountSI(usage.Free),
			Percent: int(usage.UsedPercent),
		}
		ret = append(ret, d)
	}
	return ret
}