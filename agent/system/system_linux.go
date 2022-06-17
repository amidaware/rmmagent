package system

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/amidaware/rmmagent/agent/utils"
	"github.com/shirou/gopsutil/process"
	psHost "github.com/shirou/gopsutil/v3/host"
	"github.com/wh1te909/trmm-shared"
)

func NewCMDOpts() *CmdOptions {
	return &CmdOptions{
		Shell:   "/bin/bash",
		Timeout: 30,
	}
}

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
