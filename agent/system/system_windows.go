package system

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/amidaware/rmmagent/agent/utils"
	ps "github.com/elastic/go-sysinfo"
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"github.com/shirou/gopsutil/process"
	wapf "github.com/wh1te909/go-win64api"
	trmm "github.com/wh1te909/trmm-shared"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	ProgFilesName = "TacticalAgent"
	winExeName    = "tacticalrmm.exe"
)

func RunScript(code string, shell string, args []string, timeout int) (stdout, stderr string, exitcode int, e error) {
	content := []byte(code)
	dir := filepath.Join(os.TempDir(), "trmm")
	if !trmm.FileExists(dir) {
		utils.CreateTRMMTempDir()
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
		return "", err.Error(), 85, err
	}

	defer os.Remove(tmpfn.Name())
	if _, err := tmpfn.Write(content); err != nil {
		return "", err.Error(), 85, err
	}

	if err := tmpfn.Close(); err != nil {
		return "", err.Error(), 85, err
	}

	switch shell {
	case "powershell":
		exe = "Powershell"
		cmdArgs = []string{"-NonInteractive", "-NoProfile", "-ExecutionPolicy", "Bypass", tmpfn.Name()}
	case "python":
		exe = GetPythonBin()
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
		stdout = utils.CleanString(outb.String())
		stderr = fmt.Sprintf("%s\nScript timed out after %d seconds", utils.CleanString(errb.String()), timeout)
		exitcode = 98
		//a.Logger.Debugln("Script check timeout:", ctx.Err())
	} else {
		stdout = utils.CleanString(outb.String())
		stderr = utils.CleanString(errb.String())

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
		return [2]string{"", ""},
			fmt.Errorf("%s: %s", err, utils.CleanString(errb.String()))
	}

	if ctx.Err() == context.DeadlineExceeded {
		return [2]string{"", ""}, ctx.Err()
	}

	return [2]string{
		utils.CleanString(outb.String()),
		utils.CleanString(errb.String()),
	}, nil
}

func SetDetached() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
	}
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
		return [2]string{
				utils.CleanString(outb.String()),
				utils.CleanString(errb.String())},
			ctx.Err()
	}

	if err != nil {
		return [2]string{
				utils.CleanString(outb.String()),
				utils.CleanString(errb.String())},
			err
	}

	return [2]string{
			utils.CleanString(outb.String()),
			utils.CleanString(errb.String())},
		nil
}

func GetProgramDirectory() string {
	pd := filepath.Join(os.Getenv("ProgramFiles"), ProgFilesName)
	return pd
}

func GetProgramBin() string {
	exe := filepath.Join(GetProgramDirectory(), winExeName)
	return exe
}

func GetPythonBin() string {
	var pybin string
	switch runtime.GOARCH {
	case "amd64":
		pybin = filepath.Join(GetProgramDirectory(), "py38-x64", "python.exe")
	case "386":
		pybin = filepath.Join(GetProgramDirectory(), "py38-x32", "python.exe")
	}

	return pybin
}

// LoggedOnUser returns the first logged on user it finds
func LoggedOnUser() string {
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
	user, err := RunPythonCode(pyCode, 5, []string{})
	if err == nil && user != "notascii" {
		return user
	}

	users, err := wapf.ListLoggedInUsers()
	if err != nil {
		//a.Logger.Debugln("LoggedOnUser error", err)
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

func PlatVer() (string, error) {
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
func SystemRebootRequired() (bool, error) {
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

func KillHungUpdates() {
	procs, err := ps.Processes()
	if err != nil {
		return
	}

	for _, process := range procs {
		p, err := process.Info()
		if err != nil {
			continue
		}
		if strings.Contains(p.Exe, "winagent-v") {
			//a.Logger.Debugln("killing process", p.Exe)
			KillProc(int32(p.PID))
		}
	}
}

func OsString() string {
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
	platver, err := PlatVer()
	if err != nil {
		osFullName = fmt.Sprintf("%s, %s (build %s)", osInf.Name, arch, osInf.Build)
	} else {
		osFullName = fmt.Sprintf("%s, %s v%s (build %s)", osInf.Name, arch, platver, osInf.Build)
	}

	return osFullName
}

func AddDefenderExlusions() error {
	code := `
Add-MpPreference -ExclusionPath 'C:\Program Files\TacticalAgent\*'
Add-MpPreference -ExclusionPath 'C:\Windows\Temp\winagent-v*.exe'
Add-MpPreference -ExclusionPath 'C:\Windows\Temp\trmm\*'
Add-MpPreference -ExclusionPath 'C:\Program Files\Mesh Agent\*'
`
	_, _, _, err := RunScript(code, "powershell", []string{}, 20)
	if err != nil {
		return err
	}

	return nil
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
