package system

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/amidaware/rmmagent/agent/utils"
	gocmd "github.com/go-cmd/cmd"
)

type CmdStatus struct {
	Status gocmd.Status
	Stdout string
	Stderr string
}

func CmdV2(c *CmdOptions) CmdStatus {

	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout * time.Second)
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
			KillProc(int32(pid))
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
