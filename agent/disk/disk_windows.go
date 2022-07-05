package disk

import (
	"unsafe"

	"github.com/amidaware/rmmagent/agent/utils"
	"github.com/shirou/gopsutil/disk"
	"golang.org/x/sys/windows"
)

var (
	getDriveType = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetDriveTypeW")
)

// GetDisks returns a list of fixed disks
func GetDisks() ([]Disk, error) {
	ret := make([]Disk, 0)
	partitions, err := disk.Partitions(false)
	if err != nil {
		return ret, err
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

		d := Disk{
			Device:  p.Device,
			Fstype:  p.Fstype,
			Total:   utils.ByteCountSI(usage.Total),
			Used:    utils.ByteCountSI(usage.Used),
			Free:    utils.ByteCountSI(usage.Free),
			Percent: int(usage.UsedPercent),
		}

		ret = append(ret, d)
	}

	return ret, err
}
