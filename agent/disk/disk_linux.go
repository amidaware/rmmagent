package disk

import (
	"strings"

	d "github.com/shirou/gopsutil/v3/disk"
	trmm "github.com/wh1te909/trmm-shared"
	"github.com/amidaware/rmmagent/agent/utils"
)

func GetDisks() []trmm.Disk {
	ret := make([]trmm.Disk, 0)
	partitions, err := d.Partitions(false)
	if err != nil {
		return nil
	}

	for _, p := range partitions {
		if strings.Contains(p.Device, "dev/loop") {
			continue
		}
		usage, err := d.Usage(p.Mountpoint)
		if err != nil {
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
