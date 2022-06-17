/*
Copyright 2022 AmidaWare LLC.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	rmm "github.com/amidaware/rmmagent/shared"
	ps "github.com/elastic/go-sysinfo"
	"github.com/go-resty/resty/v2"
	"github.com/jaypipes/ghw"
	"github.com/kardianos/service"
	"github.com/shirou/gopsutil/v3/cpu"
	trmm "github.com/wh1te909/trmm-shared"
)

func (a *Agent) GetWMIInfo() map[string]interface{} {
	wmiInfo := make(map[string]interface{})
	ips := make([]string, 0)
	disks := make([]string, 0)
	cpus := make([]string, 0)
	gpus := make([]string, 0)

	// local ips
	host, err := ps.Host()
	if err != nil {
		a.Logger.Errorln("GetWMIInfo() ps.Host()", err)
	} else {
		for _, ip := range host.Info().IPs {
			if strings.Contains(ip, "127.0.") || strings.Contains(ip, "::1/128") {
				continue
			}
			ips = append(ips, ip)
		}
	}
	wmiInfo["local_ips"] = ips

	// disks
	block, err := ghw.Block(ghw.WithDisableWarnings())
	if err != nil {
		a.Logger.Errorln("ghw.Block()", err)
	} else {
		for _, disk := range block.Disks {
			if disk.IsRemovable || strings.Contains(disk.Name, "ram") {
				continue
			}
			ret := fmt.Sprintf("%s %s %s %s %s %s", disk.Vendor, disk.Model, disk.StorageController, disk.DriveType, disk.Name, ByteCountSI(disk.SizeBytes))
			ret = strings.TrimSpace(strings.ReplaceAll(ret, "unknown", ""))
			disks = append(disks, ret)
		}
	}
	wmiInfo["disks"] = disks

	// cpus
	cpuInfo, err := cpu.Info()
	if err != nil {
		a.Logger.Errorln("cpu.Info()", err)
	} else {
		if len(cpuInfo) > 0 {
			if cpuInfo[0].ModelName != "" {
				cpus = append(cpus, cpuInfo[0].ModelName)
			}
		}
	}
	wmiInfo["cpus"] = cpus

	// make/model
	wmiInfo["make_model"] = ""
	chassis, err := ghw.Chassis(ghw.WithDisableWarnings())
	if err != nil {
		a.Logger.Errorln("ghw.Chassis()", err)
	} else {
		if chassis.Vendor != "" || chassis.Version != "" {
			wmiInfo["make_model"] = fmt.Sprintf("%s %s", chassis.Vendor, chassis.Version)
		}
	}

	// gfx cards

	gpu, err := ghw.GPU(ghw.WithDisableWarnings())
	if err != nil {
		a.Logger.Errorln("ghw.GPU()", err)
	} else {
		for _, i := range gpu.GraphicsCards {
			if i.DeviceInfo != nil {
				ret := fmt.Sprintf("%s %s", i.DeviceInfo.Vendor.Name, i.DeviceInfo.Product.Name)
				gpus = append(gpus, ret)
			}

		}
	}
	wmiInfo["gpus"] = gpus

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
		wmiInfo["cpus"] = []string{makeModel}
	}
	if makeModel != "" && (wmiInfo["make_model"] == "" || wmiInfo["make_model"] == "unknown unknown") {
		wmiInfo["make_model"] = makeModel
	}
	if len(gpus) == 1 && gpus[0] == "unknown unknown" {
		wmiInfo["gpus"] = ""
	}

	return wmiInfo
}

// windows only below TODO add into stub file

func (a *Agent) PlatVer() (string, error) { return "", nil }

func (a *Agent) SendSoftware() {}

func (a *Agent) UninstallCleanup() {}

func (a *Agent) RunMigrations() {}

func GetServiceStatus(name string) (string, error) { return "", nil }

func (a *Agent) GetPython(force bool) {}

type SchedTask struct{ Name string }

func (a *Agent) PatchMgmnt(enable bool) error { return nil }

func (a *Agent) CreateSchedTask(st SchedTask) (bool, error) { return false, nil }

func DeleteSchedTask(name string) error { return nil }

func ListSchedTasks() []string { return []string{} }

func (a *Agent) GetEventLog(logName string, searchLastDays int) []rmm.EventLogMsg {
	return []rmm.EventLogMsg{}
}

func (a *Agent) GetServiceDetail(name string) trmm.WindowsService { return trmm.WindowsService{} }

func (a *Agent) ControlService(name, action string) rmm.WinSvcResp {
	return rmm.WinSvcResp{Success: false, ErrorMsg: "/na"}
}

func (a *Agent) EditService(name, startupType string) rmm.WinSvcResp {
	return rmm.WinSvcResp{Success: false, ErrorMsg: "/na"}
}

func (a *Agent) GetInstalledSoftware() []trmm.WinSoftwareList { return []trmm.WinSoftwareList{} }

func (a *Agent) ChecksRunning() bool { return false }

func (a *Agent) RunTask(id int) error { return nil }

func (a *Agent) InstallChoco() {}

func (a *Agent) InstallWithChoco(name string) (string, error) { return "", nil }

func (a *Agent) GetWinUpdates() {}

func (a *Agent) InstallUpdates(guids []string) {}

func (a *Agent) installMesh(meshbin, exe, proxy string) (string, error) {
	return "not implemented", nil
}

func CMDShell(shell string, cmdArgs []string, command string, timeout int, detached bool) (output [2]string, e error) {
	return [2]string{"", ""}, nil
}

func CMD(exe string, args []string, timeout int, detached bool) (output [2]string, e error) {
	return [2]string{"", ""}, nil
}

func (a *Agent) GetServices() []trmm.WindowsService { return []trmm.WindowsService{} }

func (a *Agent) Start(_ service.Service) error { return nil }

func (a *Agent) Stop(_ service.Service) error { return nil }

func (a *Agent) InstallService() error { return nil }
