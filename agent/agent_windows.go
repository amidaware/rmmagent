/*
Copyright 2022 AmidaWare LLC.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	rmm "github.com/amidaware/rmmagent/shared"
	ps "github.com/elastic/go-sysinfo"
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/go-resty/resty/v2"
	"github.com/gonutz/w32/v2"
	"github.com/kardianos/service"
	"github.com/shirou/gopsutil/v3/disk"
	wapf "github.com/wh1te909/go-win64api"
	trmm "github.com/wh1te909/trmm-shared"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

var (
	getDriveType = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetDriveTypeW")
)

func NewAgentConfig() *rmm.AgentConfig {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\TacticalRMM`, registry.ALL_ACCESS)
	if err != nil {
		return &rmm.AgentConfig{}
	}

	baseurl, _, _ := k.GetStringValue("BaseURL")
	agentid, _, _ := k.GetStringValue("AgentID")
	apiurl, _, _ := k.GetStringValue("ApiURL")
	token, _, _ := k.GetStringValue("Token")
	agentpk, _, _ := k.GetStringValue("AgentPK")
	pk, _ := strconv.Atoi(agentpk)
	cert, _, _ := k.GetStringValue("Cert")
	proxy, _, _ := k.GetStringValue("Proxy")
	customMeshDir, _, _ := k.GetStringValue("MeshDir")

	return &rmm.AgentConfig{
		BaseURL:       baseurl,
		AgentID:       agentid,
		APIURL:        apiurl,
		Token:         token,
		AgentPK:       agentpk,
		PK:            pk,
		Cert:          cert,
		Proxy:         proxy,
		CustomMeshDir: customMeshDir,
	}
}

func (a *Agent) RunScript(code string, shell string, args []string, timeout int) (stdout, stderr string, exitcode int, e error) {

	content := []byte(code)

	dir := filepath.Join(os.TempDir(), "trmm")
	if !trmm.FileExists(dir) {
		a.CreateTRMMTempDir()
	}

	const defaultExitCode = 1

	var (
		outb    bytes.Buffer
		errb    bytes.Buffer
		exe     string
		ext     string
		cmdArgs []string
	)

	switch shell {
	case "powershell":
		ext = "*.ps1"
	case "python":
		ext = "*.py"
	case "cmd":
		ext = "*.bat"
	}

	tmpfn, err := ioutil.TempFile(dir, ext)
	if err != nil {
		a.Logger.Errorln(err)
		return "", err.Error(), 85, err
	}
	defer os.Remove(tmpfn.Name())

	if _, err := tmpfn.Write(content); err != nil {
		a.Logger.Errorln(err)
		return "", err.Error(), 85, err
	}
	if err := tmpfn.Close(); err != nil {
		a.Logger.Errorln(err)
		return "", err.Error(), 85, err
	}

	switch shell {
	case "powershell":
		exe = "Powershell"
		cmdArgs = []string{"-NonInteractive", "-NoProfile", "-ExecutionPolicy", "Bypass", tmpfn.Name()}
	case "python":
		exe = a.PyBin
		cmdArgs = []string{tmpfn.Name()}
	case "cmd":
		exe = tmpfn.Name()
	}

	if len(args) > 0 {
		cmdArgs = append(cmdArgs, args...)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	var timedOut bool = false
	cmd := exec.Command(exe, cmdArgs...)
	cmd.Stdout = &outb
	cmd.Stderr = &errb

	if cmdErr := cmd.Start(); cmdErr != nil {
		a.Logger.Debugln(cmdErr)
		return "", cmdErr.Error(), 65, cmdErr
	}
	pid := int32(cmd.Process.Pid)

	// custom context handling, we need to kill child procs if this is a batch script,
	// otherwise it will hang forever
	// the normal exec.CommandContext() doesn't work since it only kills the parent process
	go func(p int32) {

		<-ctx.Done()

		_ = KillProc(p)
		timedOut = true
	}(pid)

	cmdErr := cmd.Wait()

	if timedOut {
		stdout = CleanString(outb.String())
		stderr = fmt.Sprintf("%s\nScript timed out after %d seconds", CleanString(errb.String()), timeout)
		exitcode = 98
		a.Logger.Debugln("Script check timeout:", ctx.Err())
	} else {
		stdout = CleanString(outb.String())
		stderr = CleanString(errb.String())

		// get the exit code
		if cmdErr != nil {
			if exitError, ok := cmdErr.(*exec.ExitError); ok {
				if ws, ok := exitError.Sys().(syscall.WaitStatus); ok {
					exitcode = ws.ExitStatus()
				} else {
					exitcode = defaultExitCode
				}
			} else {
				exitcode = defaultExitCode
			}

		} else {
			if ws, ok := cmd.ProcessState.Sys().(syscall.WaitStatus); ok {
				exitcode = ws.ExitStatus()
			} else {
				exitcode = 0
			}
		}
	}
	return stdout, stderr, exitcode, nil
}

func SetDetached() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
	}
}

func CMD(exe string, args []string, timeout int, detached bool) (output [2]string, e error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	var outb, errb bytes.Buffer
	cmd := exec.CommandContext(ctx, exe, args...)
	if detached {
		cmd.SysProcAttr = &windows.SysProcAttr{
			CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
		}
	}
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	err := cmd.Run()
	if err != nil {
		return [2]string{"", ""}, fmt.Errorf("%s: %s", err, CleanString(errb.String()))
	}

	if ctx.Err() == context.DeadlineExceeded {
		return [2]string{"", ""}, ctx.Err()
	}

	return [2]string{CleanString(outb.String()), CleanString(errb.String())}, nil
}

func CMDShell(shell string, cmdArgs []string, command string, timeout int, detached bool) (output [2]string, e error) {
	var (
		outb     bytes.Buffer
		errb     bytes.Buffer
		cmd      *exec.Cmd
		timedOut = false
	)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	if len(cmdArgs) > 0 && command == "" {
		switch shell {
		case "cmd":
			cmdArgs = append([]string{"/C"}, cmdArgs...)
			cmd = exec.Command("cmd.exe", cmdArgs...)
		case "powershell":
			cmdArgs = append([]string{"-NonInteractive", "-NoProfile"}, cmdArgs...)
			cmd = exec.Command("powershell.exe", cmdArgs...)
		}
	} else {
		switch shell {
		case "cmd":
			cmd = exec.Command("cmd.exe")
			cmd.SysProcAttr = &windows.SysProcAttr{
				CmdLine: fmt.Sprintf("cmd.exe /C %s", command),
			}
		case "powershell":
			cmd = exec.Command("Powershell", "-NonInteractive", "-NoProfile", command)
		}
	}

	// https://docs.microsoft.com/en-us/windows/win32/procthread/process-creation-flags
	if detached {
		cmd.SysProcAttr = &windows.SysProcAttr{
			CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
		}
	}
	cmd.Stdout = &outb
	cmd.Stderr = &errb
	cmd.Start()

	pid := int32(cmd.Process.Pid)

	go func(p int32) {

		<-ctx.Done()

		_ = KillProc(p)
		timedOut = true
	}(pid)

	err := cmd.Wait()

	if timedOut {
		return [2]string{CleanString(outb.String()), CleanString(errb.String())}, ctx.Err()
	}

	if err != nil {
		return [2]string{CleanString(outb.String()), CleanString(errb.String())}, err
	}

	return [2]string{CleanString(outb.String()), CleanString(errb.String())}, nil
}

// GetDisks returns a list of fixed disks
func (a *Agent) GetDisks() []trmm.Disk {
	ret := make([]trmm.Disk, 0)
	partitions, err := disk.Partitions(false)
	if err != nil {
		a.Logger.Debugln(err)
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
			a.Logger.Debugln(err)
			continue
		}

		d := trmm.Disk{
			Device:  p.Device,
			Fstype:  p.Fstype,
			Total:   ByteCountSI(usage.Total),
			Used:    ByteCountSI(usage.Used),
			Free:    ByteCountSI(usage.Free),
			Percent: int(usage.UsedPercent),
		}
		ret = append(ret, d)
	}
	return ret
}

// LoggedOnUser returns the first logged on user it finds
func (a *Agent) LoggedOnUser() string {
	pyCode := `
import psutil

try:
	u = psutil.users()[0].name
	if u.isascii():
		print(u, end='')
	else:
		print('notascii', end='')
except Exception as e:
	print("None", end='')

`
	// try with psutil first, if fails, fallback to golang
	user, err := a.RunPythonCode(pyCode, 5, []string{})
	if err == nil && user != "notascii" {
		return user
	}

	users, err := wapf.ListLoggedInUsers()
	if err != nil {
		a.Logger.Debugln("LoggedOnUser error", err)
		return "None"
	}

	if len(users) == 0 {
		return "None"
	}

	for _, u := range users {
		// remove the computername or domain
		return strings.Split(u.FullUser(), `\`)[1]
	}
	return "None"
}

// ShowStatus prints windows service status
// If called from an interactive desktop, pops up a message box
// Otherwise prints to the console
func ShowStatus(version string) {
	statusMap := make(map[string]string)
	svcs := []string{winSvcName, meshSvcName}

	for _, service := range svcs {
		status, err := GetServiceStatus(service)
		if err != nil {
			statusMap[service] = "Not Installed"
			continue
		}
		statusMap[service] = status
	}

	window := w32.GetForegroundWindow()
	if window != 0 {
		_, consoleProcID := w32.GetWindowThreadProcessId(window)
		if w32.GetCurrentProcessId() == consoleProcID {
			w32.ShowWindow(window, w32.SW_HIDE)
		}
		var handle w32.HWND
		msg := fmt.Sprintf("Agent: %s\n\nMesh Agent: %s", statusMap[winSvcName], statusMap[meshSvcName])
		w32.MessageBox(handle, msg, fmt.Sprintf("Tactical RMM v%s", version), w32.MB_OK|w32.MB_ICONINFORMATION)
	} else {
		fmt.Println("Tactical RMM Version", version)
		fmt.Println("Tactical Agent:", statusMap[winSvcName])
		fmt.Println("Mesh Agent:", statusMap[meshSvcName])
	}
}

// PatchMgmnt enables/disables automatic update
// 0 - Enable Automatic Updates (Default)
// 1 - Disable Automatic Updates
// https://docs.microsoft.com/en-us/previous-versions/windows/it-pro/windows-server-2008-R2-and-2008/dd939844(v=ws.10)?redirectedfrom=MSDN
func (a *Agent) PatchMgmnt(enable bool) error {
	var val uint32
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, `SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate\AU`, registry.ALL_ACCESS)
	if err != nil {
		return err
	}

	if enable {
		val = 1
	} else {
		val = 0
	}

	err = k.SetDWordValue("AUOptions", val)
	if err != nil {
		return err
	}

	return nil
}

func (a *Agent) PlatVer() (string, error) {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.ALL_ACCESS)
	if err != nil {
		return "n/a", err
	}
	defer k.Close()

	dv, _, err := k.GetStringValue("DisplayVersion")
	if err == nil {
		return dv, nil
	}

	relid, _, err := k.GetStringValue("ReleaseId")
	if err != nil {
		return "n/a", err
	}
	return relid, nil
}

// EnablePing enables ping
func EnablePing() {
	args := make([]string, 0)
	cmd := `netsh advfirewall firewall add rule name="ICMP Allow incoming V4 echo request" protocol=icmpv4:8,any dir=in action=allow`
	_, err := CMDShell("cmd", args, cmd, 10, false)
	if err != nil {
		fmt.Println(err)
	}
}

// EnableRDP enables Remote Desktop
func EnableRDP() {
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Terminal Server`, registry.ALL_ACCESS)
	if err != nil {
		fmt.Println(err)
	}
	defer k.Close()

	err = k.SetDWordValue("fDenyTSConnections", 0)
	if err != nil {
		fmt.Println(err)
	}

	args := make([]string, 0)
	cmd := `netsh advfirewall firewall set rule group="remote desktop" new enable=Yes`
	_, cerr := CMDShell("cmd", args, cmd, 10, false)
	if cerr != nil {
		fmt.Println(cerr)
	}
}

// DisableSleepHibernate disables sleep and hibernate
func DisableSleepHibernate() {
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Session Manager\Power`, registry.ALL_ACCESS)
	if err != nil {
		fmt.Println(err)
	}
	defer k.Close()

	err = k.SetDWordValue("HiberbootEnabled", 0)
	if err != nil {
		fmt.Println(err)
	}

	args := make([]string, 0)

	var wg sync.WaitGroup
	currents := []string{"ac", "dc"}
	for _, i := range currents {
		wg.Add(1)
		go func(c string) {
			defer wg.Done()
			_, _ = CMDShell("cmd", args, fmt.Sprintf("powercfg /set%svalueindex scheme_current sub_buttons lidaction 0", c), 5, false)
			_, _ = CMDShell("cmd", args, fmt.Sprintf("powercfg /x -standby-timeout-%s 0", c), 5, false)
			_, _ = CMDShell("cmd", args, fmt.Sprintf("powercfg /x -hibernate-timeout-%s 0", c), 5, false)
			_, _ = CMDShell("cmd", args, fmt.Sprintf("powercfg /x -disk-timeout-%s 0", c), 5, false)
			_, _ = CMDShell("cmd", args, fmt.Sprintf("powercfg /x -monitor-timeout-%s 0", c), 5, false)
		}(i)
	}
	wg.Wait()
	_, _ = CMDShell("cmd", args, "powercfg -S SCHEME_CURRENT", 5, false)
}

// NewCOMObject creates a new COM object for the specifed ProgramID.
func NewCOMObject(id string) (*ole.IDispatch, error) {
	unknown, err := oleutil.CreateObject(id)
	if err != nil {
		return nil, fmt.Errorf("unable to create initial unknown object: %v", err)
	}
	defer unknown.Release()

	obj, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return nil, fmt.Errorf("unable to create query interface: %v", err)
	}

	return obj, nil
}

// SystemRebootRequired checks whether a system reboot is required.
func (a *Agent) SystemRebootRequired() (bool, error) {
	regKeys := []string{
		`SOFTWARE\Microsoft\Windows\CurrentVersion\WindowsUpdate\Auto Update\RebootRequired`,
	}
	for _, key := range regKeys {
		k, err := registry.OpenKey(registry.LOCAL_MACHINE, key, registry.QUERY_VALUE)
		if err == nil {
			k.Close()
			return true, nil
		} else if err != registry.ErrNotExist {
			return false, err
		}
	}
	return false, nil
}

func (a *Agent) SendSoftware() {
	sw := a.GetInstalledSoftware()
	a.Logger.Debugln(sw)

	payload := map[string]interface{}{"agent_id": a.AgentID, "software": sw}
	_, err := a.rClient.R().SetBody(payload).Post("/api/v3/software/")
	if err != nil {
		a.Logger.Debugln(err)
	}
}

func (a *Agent) UninstallCleanup() {
	registry.DeleteKey(registry.LOCAL_MACHINE, `SOFTWARE\TacticalRMM`)
	a.PatchMgmnt(false)
	a.CleanupAgentUpdates()
	CleanupSchedTasks()
}

func (a *Agent) AgentUpdate(url, inno, version string) {
	time.Sleep(time.Duration(randRange(1, 15)) * time.Second)
	a.KillHungUpdates()
	a.CleanupAgentUpdates()
	updater := filepath.Join(a.ProgramDir, inno)
	a.Logger.Infof("Agent updating from %s to %s", a.Version, version)
	a.Logger.Infoln("Downloading agent update from", url)

	rClient := resty.New()
	rClient.SetCloseConnection(true)
	rClient.SetTimeout(15 * time.Minute)
	rClient.SetDebug(a.Debug)
	if len(a.Proxy) > 0 {
		rClient.SetProxy(a.Proxy)
	}
	r, err := rClient.R().SetOutput(updater).Get(url)
	if err != nil {
		a.Logger.Errorln(err)
		CMD("net", []string{"start", winSvcName}, 10, false)
		return
	}
	if r.IsError() {
		a.Logger.Errorln("Download failed with status code", r.StatusCode())
		CMD("net", []string{"start", winSvcName}, 10, false)
		return
	}

	dir, err := ioutil.TempDir("", "tacticalrmm")
	if err != nil {
		a.Logger.Errorln("Agentupdate create tempdir:", err)
		CMD("net", []string{"start", winSvcName}, 10, false)
		return
	}

	innoLogFile := filepath.Join(dir, "tacticalrmm.txt")

	args := []string{"/C", updater, "/VERYSILENT", fmt.Sprintf("/LOG=%s", innoLogFile)}
	cmd := exec.Command("cmd.exe", args...)
	cmd.SysProcAttr = &windows.SysProcAttr{
		CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
	}
	cmd.Start()
	time.Sleep(1 * time.Second)
}

func (a *Agent) osString() string {
	host, _ := ps.Host()
	info := host.Info()
	osInf := info.OS

	var arch string
	switch info.Architecture {
	case "x86_64":
		arch = "64 bit"
	case "x86":
		arch = "32 bit"
	}

	var osFullName string
	platver, err := a.PlatVer()
	if err != nil {
		osFullName = fmt.Sprintf("%s, %s (build %s)", osInf.Name, arch, osInf.Build)
	} else {
		osFullName = fmt.Sprintf("%s, %s v%s (build %s)", osInf.Name, arch, platver, osInf.Build)
	}
	return osFullName
}

func (a *Agent) AgentUninstall(code string) {
	a.KillHungUpdates()
	tacUninst := filepath.Join(a.ProgramDir, a.GetUninstallExe())
	args := []string{"/C", tacUninst, "/VERYSILENT"}
	cmd := exec.Command("cmd.exe", args...)
	cmd.SysProcAttr = &windows.SysProcAttr{
		CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
	}
	cmd.Start()
}

func (a *Agent) addDefenderExlusions() {
	code := `
Add-MpPreference -ExclusionPath 'C:\Program Files\TacticalAgent\*'
Add-MpPreference -ExclusionPath 'C:\Windows\Temp\winagent-v*.exe'
Add-MpPreference -ExclusionPath 'C:\Windows\Temp\trmm\*'
Add-MpPreference -ExclusionPath 'C:\Program Files\Mesh Agent\*'
`
	_, _, _, err := a.RunScript(code, "powershell", []string{}, 20)
	if err != nil {
		a.Logger.Debugln(err)
	}
}

// RunMigrations cleans up unused stuff from older agents
func (a *Agent) RunMigrations() {
	for _, i := range []string{"nssm.exe", "nssm-x86.exe"} {
		nssm := filepath.Join(a.ProgramDir, i)
		if trmm.FileExists(nssm) {
			os.Remove(nssm)
		}
	}
}

func (a *Agent) installMesh(meshbin, exe, proxy string) (string, error) {
	var meshNodeID string
	meshInstallArgs := []string{"-fullinstall"}
	if len(proxy) > 0 {
		meshProxy := fmt.Sprintf("--WebProxy=%s", proxy)
		meshInstallArgs = append(meshInstallArgs, meshProxy)
	}
	a.Logger.Debugln("Mesh install args:", meshInstallArgs)

	meshOut, meshErr := CMD(meshbin, meshInstallArgs, int(90), false)
	if meshErr != nil {
		fmt.Println(meshOut[0])
		fmt.Println(meshOut[1])
		fmt.Println(meshErr)
	}

	fmt.Println(meshOut)
	a.Logger.Debugln("Sleeping for 5")
	time.Sleep(5 * time.Second)

	meshSuccess := false

	for !meshSuccess {
		a.Logger.Debugln("Getting mesh node id")
		pMesh, pErr := CMD(exe, []string{"-nodeid"}, int(30), false)
		if pErr != nil {
			a.Logger.Errorln(pErr)
			time.Sleep(5 * time.Second)
			continue
		}
		if pMesh[1] != "" {
			a.Logger.Errorln(pMesh[1])
			time.Sleep(5 * time.Second)
			continue
		}
		meshNodeID = StripAll(pMesh[0])
		a.Logger.Debugln("Node id:", meshNodeID)
		if strings.Contains(strings.ToLower(meshNodeID), "not defined") {
			a.Logger.Errorln(meshNodeID)
			time.Sleep(5 * time.Second)
			continue
		}
		meshSuccess = true
	}

	return meshNodeID, nil
}

// ChecksRunning prevents duplicate checks from running
// Have to do it this way, can't use atomic because they can run from both rpc and tacticalagent services
func (a *Agent) ChecksRunning() bool {
	running := false
	procs, err := ps.Processes()
	if err != nil {
		return running
	}

Out:
	for _, process := range procs {
		p, err := process.Info()
		if err != nil {
			continue
		}
		if p.PID == 0 {
			continue
		}
		if p.Exe != a.EXE {
			continue
		}

		for _, arg := range p.Args {
			if arg == "runchecks" || arg == "checkrunner" {
				running = true
				break Out
			}
		}
	}
	return running
}

func (a *Agent) GetPython(force bool) {
	if trmm.FileExists(a.PyBin) && !force {
		return
	}

	var archZip string
	var folder string
	switch runtime.GOARCH {
	case "amd64":
		archZip = "py38-x64.zip"
		folder = "py38-x64"
	case "386":
		archZip = "py38-x32.zip"
		folder = "py38-x32"
	}
	pyFolder := filepath.Join(a.ProgramDir, folder)
	pyZip := filepath.Join(a.ProgramDir, archZip)
	a.Logger.Debugln(pyZip)
	a.Logger.Debugln(a.PyBin)
	defer os.Remove(pyZip)

	if force {
		os.RemoveAll(pyFolder)
	}

	rClient := resty.New()
	rClient.SetTimeout(20 * time.Minute)
	rClient.SetRetryCount(10)
	rClient.SetRetryWaitTime(1 * time.Minute)
	rClient.SetRetryMaxWaitTime(15 * time.Minute)
	if len(a.Proxy) > 0 {
		rClient.SetProxy(a.Proxy)
	}

	url := fmt.Sprintf("https://github.com/amidaware/rmmagent/releases/download/v2.0.0/%s", archZip)
	a.Logger.Debugln(url)
	r, err := rClient.R().SetOutput(pyZip).Get(url)
	if err != nil {
		a.Logger.Errorln("Unable to download py3.zip from github.", err)
		return
	}
	if r.IsError() {
		a.Logger.Errorln("Unable to download py3.zip from github. Status code", r.StatusCode())
		return
	}

	err = Unzip(pyZip, a.ProgramDir)
	if err != nil {
		a.Logger.Errorln(err)
	}
}

func (a *Agent) RecoverMesh() {
	a.Logger.Infoln("Attempting mesh recovery")
	defer CMD("net", []string{"start", a.MeshSVC}, 60, false)

	_, _ = CMD("net", []string{"stop", a.MeshSVC}, 60, false)
	a.ForceKillMesh()
	a.SyncMeshNodeID()
}

func (a *Agent) getMeshNodeID() (string, error) {
	out, err := CMD(a.MeshSystemEXE, []string{"-nodeid"}, 10, false)
	if err != nil {
		a.Logger.Debugln(err)
		return "", err
	}

	stdout := out[0]
	stderr := out[1]

	if stderr != "" {
		a.Logger.Debugln(stderr)
		return "", err
	}

	if stdout == "" || strings.Contains(strings.ToLower(StripAll(stdout)), "not defined") {
		a.Logger.Debugln("Failed getting mesh node id", stdout)
		return "", errors.New("failed to get mesh node id")
	}

	return stdout, nil
}

func (a *Agent) Start(_ service.Service) error {
	go a.RunRPC()
	return nil
}

func (a *Agent) Stop(_ service.Service) error {
	return nil
}

func (a *Agent) InstallService() error {
	if serviceExists(winSvcName) {
		return nil
	}

	// skip on first call of inno setup if this is a new install
	_, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\TacticalRMM`, registry.ALL_ACCESS)
	if err != nil {
		return nil
	}

	s, err := service.New(a, a.ServiceConfig)
	if err != nil {
		return err
	}

	return service.Control(s, "install")
}

// TODO add to stub
func (a *Agent) NixMeshNodeID() string {
	return "not implemented"
}
