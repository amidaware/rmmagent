package system

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"os/exec"
	"time"

	"github.com/amidaware/rmmagent/agent/utils"
	ps "github.com/elastic/go-sysinfo"
	gocmd "github.com/go-cmd/cmd"
)

type CmdStatus struct {
	Status gocmd.Status
	Stdout string
	Stderr string
}

func NewCMDOpts() *CmdOptions {
	return &CmdOptions{
		Shell:   "/bin/bash",
		Timeout: 30,
	}
}

func CmdV2(c *CmdOptions) CmdStatus {

	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout*time.Second)
	defer cancel()

	// Disable output buffering, enable streaming
	cmdOptions := gocmd.Options{
		Buffered:  false,
		Streaming: true,
	}

	// have a child process that is in a different process group so that
	// parent terminating doesn't kill child
	if c.Detached {
		cmdOptions.BeforeExec = []func(cmd *exec.Cmd){
			func(cmd *exec.Cmd) {
				cmd.SysProcAttr = SetDetached()
			},
		}
	}

	var envCmd *gocmd.Cmd
	if c.IsScript {
		envCmd = gocmd.NewCmdOptions(cmdOptions, c.Shell, c.Args...) // call script directly
	} else if c.IsExecutable {
		envCmd = gocmd.NewCmdOptions(cmdOptions, c.Shell, c.Command) // c.Shell: bin + c.Command: args as one string
	} else {
		envCmd = gocmd.NewCmdOptions(cmdOptions, c.Shell, "-c", c.Command) // /bin/bash -c 'ls -l /var/log/...'
	}

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	// Print STDOUT and STDERR lines streaming from Cmd
	doneChan := make(chan struct{})
	go func() {
		defer close(doneChan)
		// Done when both channels have been closed
		// https://dave.cheney.net/2013/04/30/curious-channels
		for envCmd.Stdout != nil || envCmd.Stderr != nil {
			select {
			case line, open := <-envCmd.Stdout:
				if !open {
					envCmd.Stdout = nil
					continue
				}

				fmt.Fprintln(&stdoutBuf, line)

			case line, open := <-envCmd.Stderr:
				if !open {
					envCmd.Stderr = nil
					continue
				}

				fmt.Fprintln(&stderrBuf, line)
			}
		}
	}()

	// Run and wait for Cmd to return, discard Status
	envCmd.Start()

	go func() {
		select {
		case <-doneChan:
			return
		case <-ctx.Done():
			pid := envCmd.Status().PID
			utils.KillProc(int32(pid))
		}
	}()

	// Wait for goroutine to print everything
	<-doneChan
	ret := CmdStatus{
		Status: envCmd.Status(),
		Stdout: utils.CleanString(stdoutBuf.String()),
		Stderr: utils.CleanString(stderrBuf.String()),
	}

	return ret
}

func RunPythonCode(code string, timeout int, args []string) (string, error) {
	content := []byte(code)
	dir, err := ioutil.TempDir("", "tacticalpy")
	if err != nil {
		//a.Logger.Debugln(err)
		return "", err
	}

	defer os.RemoveAll(dir)
	tmpfn, _ := ioutil.TempFile(dir, "*.py")
	if _, err := tmpfn.Write(content); err != nil {
		//a.Logger.Debugln(err)
		return "", err
	}

	if err := tmpfn.Close(); err != nil {
		//a.Logger.Debugln(err)
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	var outb, errb bytes.Buffer
	cmdArgs := []string{tmpfn.Name()}
	if len(args) > 0 {
		cmdArgs = append(cmdArgs, args...)
	}

	//a.Logger.Debugln(cmdArgs)
	cmd := exec.CommandContext(ctx, GetPythonBin(), cmdArgs...)
	cmd.Stdout = &outb
	cmd.Stderr = &errb

	cmdErr := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		//a.Logger.Debugln("RunPythonCode:", ctx.Err())
		return "", ctx.Err()
	}

	if cmdErr != nil {
		//a.Logger.Debugln("RunPythonCode:", cmdErr)
		return "", cmdErr
	}

	if errb.String() != "" {
		//a.Logger.Debugln(errb.String())
		return errb.String(), errors.New("RunPythonCode stderr")
	}

	return outb.String(), nil
}

func GetHostname() string {
	host, _ := ps.Host()
	info := host.Info()
	return info.Hostname
}

// TotalRAM returns total RAM in GB
func TotalRAM() float64 {
	host, err := ps.Host()
	if err != nil {
		return 8.0
	}

	mem, err := host.Memory()
	if err != nil {
		return 8.0
	}

	return math.Ceil(float64(mem.Total) / 1073741824.0)
}

// BootTime returns system boot time as a unix timestamp
func BootTime() int64 {
	host, err := ps.Host()
	if err != nil {
		return 1000
	}

	info := host.Info()
	return info.BootTime.Unix()
}
