package system

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/amidaware/rmmagent/agent/utils"
	ps "github.com/elastic/go-sysinfo"
	"github.com/jaypipes/ghw"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/process"
	psHost "github.com/shirou/gopsutil/v3/host"
	rmm "github.com/amidaware/rmmagent/shared"
	trmm "github.com/wh1te909/trmm-shared"
)

func SetDetached() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setpgid: true}
}

func ShowStatus(version string) {
	fmt.Println(version)
}

func SystemRebootRequired() (bool, error) {
	// deb
	paths := [2]string{"/var/run/reboot-required", "/run/reboot-required"}
	for _, p := range paths {
		if trmm.FileExists(p) {
			return true, nil
		}
	}
	// rhel
	bins := [2]string{"/usr/bin/needs-restarting", "/bin/needs-restarting"}
	for _, bin := range bins {
		if trmm.FileExists(bin) {
			opts := NewCMDOpts()
			// https://man7.org/linux/man-pages/man1/needs-restarting.1.html
			// -r Only report whether a full reboot is required (exit code 1) or not (exit code 0).
			opts.Command = fmt.Sprintf("%s -r", bin)
			out := CmdV2(opts)

			if out.Status.Error != nil {
				continue
			}

			if out.Status.Exit == 1 {
				return true, nil
			}

			return false, nil
		}
	}

	return false, nil
}

func LoggedOnUser() string {
	var ret string
	users, err := psHost.Users()
	if err != nil {
		return ret
	}

	// return the first logged in user
	for _, user := range users {
		if user.User != "" {
			ret = user.User
			break
		}
	}

	return ret
}

func OsString() string {
	h, err := psHost.Info()
	if err != nil {
		return "error getting host info"
	}

	return fmt.Sprintf("%s %s %s %s", strings.Title(h.Platform), h.PlatformVersion, h.KernelArch, h.KernelVersion)
}

// KillProc kills a process and its children
func KillProc(pid int32) error {
	p, err := process.NewProcess(pid)
	if err != nil {
		return err
	}

	children, err := p.Children()
	if err == nil {
		for _, child := range children {
			if err := child.Kill(); err != nil {
				continue
			}
		}
	}

	if err := p.Kill(); err != nil {
		return err
	}

	return nil
}

func RunScript(code string, shell string, args []string, timeout int) (stdout, stderr string, exitcode int, e error) {
	code = utils.RemoveWinNewLines(code)
	content := []byte(code)

	f, err := utils.CreateTmpFile()
	if err != nil {
		return "", err.Error(), 85, err
	}

	defer os.Remove(f.Name())

	if _, err := f.Write(content); err != nil {
		return "", err.Error(), 85, err
	}

	if err := f.Close(); err != nil {
		return "", err.Error(), 85, err
	}

	if err := os.Chmod(f.Name(), 0770); err != nil {
		return "", err.Error(), 85, err
	}

	opts := NewCMDOpts()
	opts.IsScript = true
	opts.Shell = f.Name()
	opts.Args = args
	opts.Timeout = time.Duration(timeout)
	out := CmdV2(opts)
	retError := ""
	if out.Status.Error != nil {
		retError += utils.CleanString(out.Status.Error.Error())
		retError += "\n"
	}

	if len(out.Stderr) > 0 {
		retError += out.Stderr
	}

	return out.Stdout, retError, out.Status.Exit, nil
}

func GetWMIInfo() map[string]interface{} {
	wmiInfo := make(map[string]interface{})
	ips := make([]string, 0)
	disks := make([]string, 0)
	cpus := make([]string, 0)
	gpus := make([]string, 0)

	// local ips
	host, err := ps.Host()
	if err != nil {
		//a.Logger.Errorln("GetWMIInfo() ps.Host()", err)
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
		//a.Logger.Errorln("ghw.Block()", err)
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

	wmiInfo["disks"] = disks

	// cpus
	cpuInfo, err := cpu.Info()
	if err != nil {
		//a.Logger.Errorln("cpu.Info()", err)
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
		//a.Logger.Errorln("ghw.Chassis()", err)
	} else {
		if chassis.Vendor != "" || chassis.Version != "" {
			wmiInfo["make_model"] = fmt.Sprintf("%s %s", chassis.Vendor, chassis.Version)
		}
	}

	// gfx cards

	gpu, err := ghw.GPU(ghw.WithDisableWarnings())
	if err != nil {
		//a.Logger.Errorln("ghw.GPU()", err)
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

func PlatVer() (string, error) { return "", nil }

func GetServiceStatus(name string) (string, error) { return "", nil }

func CreateSchedTask(st SchedTask) (bool, error) { return false, nil }

func DeleteSchedTask(name string) error { return nil }

func ListSchedTasks() []string { return []string{} }

func GetEventLog(logName string, searchLastDays int) []rmm.EventLogMsg {
	return []rmm.EventLogMsg{}
}

func CMDShell(shell string, cmdArgs []string, command string, timeout int, detached bool) (output [2]string, e error) {
	return [2]string{"", ""}, nil
}

func CMD(exe string, args []string, timeout int, detached bool) (output [2]string, e error) {
	return [2]string{"", ""}, nil
}