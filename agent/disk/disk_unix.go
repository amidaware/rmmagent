//go:build !windows
// +build !windows

package disk

import (
	"strings"

	d "github.com/shirou/gopsutil/v3/disk"
	"github.com/amidaware/rmmagent/agent/utils"
)

func GetDisks() ([]Disk, error) {
	ret := make([]Disk, 0)
	partitions, err := d.Partitions(false)
	if err != nil {
		return []Disk{}, nil
	}

	for _, p := range partitions {
		if strings.Contains(p.Device, "dev/loop") {
			continue
		}
		usage, err := d.Usage(p.Mountpoint)
		if err != nil {
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

	return ret, nil
}
