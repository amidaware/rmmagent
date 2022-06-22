//go:build !windows
// +build !windows

package umi

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/amidaware/rmmagent/agent/utils"
	ps "github.com/elastic/go-sysinfo"
	"github.com/jaypipes/ghw"
	"github.com/shirou/gopsutil/cpu"
)

func GetInfo() (map[string]interface{}, []error) {
	info := make(map[string]interface{})
	errs := []error{}
	ips := make([]string, 0)
	disks := make([]string, 0)
	cpus := make([]string, 0)
	gpus := make([]string, 0)

	// local ips
	host, err := ps.Host()
	if err != nil {
		errs = append(errs, err)
	} else {
		for _, ip := range host.Info().IPs {
			if strings.Contains(ip, "127.0.") || strings.Contains(ip, "::1/128") {
				continue
			}
			ips = append(ips, ip)
		}
	}

	info["local_ips"] = ips
	// disks
	block, err := ghw.Block(ghw.WithDisableWarnings())
	if err != nil {
		errs = append(errs, err)
	} else {
		for _, disk := range block.Disks {
			if disk.IsRemovable || strings.Contains(disk.Name, "ram") {
				continue
			}
			ret := fmt.Sprintf("%s %s %s %s %s %s", disk.Vendor, disk.Model, disk.StorageController, disk.DriveType, disk.Name, utils.ByteCountSI(disk.SizeBytes))
			ret = strings.TrimSpace(strings.ReplaceAll(ret, "unknown", ""))
			disks = append(disks, ret)
		}
	}

	info["disks"] = disks
	// cpus
	cpuInfo, err := cpu.Info()
	if err != nil {
		errs = append(errs, err)
	} else {
		if len(cpuInfo) > 0 {
			if cpuInfo[0].ModelName != "" {
				cpus = append(cpus, cpuInfo[0].ModelName)
			}
		}
	}

	info["cpus"] = cpus
	// make/model
	info["make_model"] = ""
	chassis, err := ghw.Chassis(ghw.WithDisableWarnings())
	if err != nil {
		errs = append(errs, err)
	} else {
		if chassis.Vendor != "" || chassis.Version != "" {
			info["make_model"] = fmt.Sprintf("%s %s", chassis.Vendor, chassis.Version)
		}
	}

	// gfx cards
	gpu, err := ghw.GPU(ghw.WithDisableWarnings())
	if err != nil {
		errs = append(errs, err)
	} else {
		for _, i := range gpu.GraphicsCards {
			if i.DeviceInfo != nil {
				ret := fmt.Sprintf("%s %s", i.DeviceInfo.Vendor.Name, i.DeviceInfo.Product.Name)
				gpus = append(gpus, ret)
			}

		}
	}

	info["gpus"] = gpus
	// temp hack for ARM cpu/make/model if rasp pi
	var makeModel string
	if strings.Contains(runtime.GOARCH, "arm") {
		file, _ := os.Open("/proc/cpuinfo")
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			if strings.Contains(strings.ToLower(scanner.Text()), "raspberry") {
				model := strings.Split(scanner.Text(), ":")
				if len(model) == 2 {
					makeModel = strings.TrimSpace(model[1])
					break
				}
			}
		}
	}

	if len(cpus) == 0 {
		info["cpus"] = []string{makeModel}
	}
	if makeModel != "" && (info["make_model"] == "" || info["make_model"] == "unknown unknown") {
		info["make_model"] = makeModel
	}
	if len(gpus) == 1 && gpus[0] == "unknown unknown" {
		info["gpus"] = ""
	}

	return info, errs
}