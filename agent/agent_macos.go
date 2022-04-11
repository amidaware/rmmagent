/*
Copyright 2022 AmidaWare LLC.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

import (
	"fmt"
	"strings"
	"syscall"

	rmm "github.com/amidaware/rmmagent/shared"
	"github.com/kardianos/service"
	psHost "github.com/shirou/gopsutil/v3/host"
	trmm "github.com/wh1te909/trmm-shared"
)

func ShowStatus(version string) {
	fmt.Println(version)
}

func (a *Agent) GetDisks() []trmm.Disk { return nil }

func (a *Agent) SystemRebootRequired() (bool, error) { return false, nil }

func (a *Agent) LoggedOnUser() string { return "" }

func (a *Agent) osString() string {
	h, err := psHost.Info()
	if err != nil {
		return "error getting host info"
	}
	return fmt.Sprintf("%s %s %s %s", strings.Title(h.Platform), h.PlatformVersion, h.KernelArch, h.KernelVersion)
}

func NewAgentConfig() *rmm.AgentConfig { return nil }

func (a *Agent) RunScript(code string, shell string, args []string, timeout int) (stdout, stderr string, exitcode int, e error) {
	return "", "", 0, nil
}

func SetDetached() *syscall.SysProcAttr { return nil }

func (a *Agent) AgentUpdate(url, inno, version string) {}

func (a *Agent) AgentUninstall(code string) {}

func (a *Agent) NixMeshNodeID() string { return "" }

func (a *Agent) getMeshNodeID() (string, error) { return "", nil }

func (a *Agent) RecoverMesh() {}

func (a *Agent) GetWMIInfo() map[string]interface{} { return nil }

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
