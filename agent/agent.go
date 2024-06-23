/*
Copyright 2023 AmidaWare Inc.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"time"

	rmm "github.com/amidaware/rmmagent/shared"
	ps "github.com/elastic/go-sysinfo"
	gocmd "github.com/go-cmd/cmd"
	"github.com/go-resty/resty/v2"
	"github.com/kardianos/service"
	nats "github.com/nats-io/nats.go"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/sirupsen/logrus"
	trmm "github.com/wh1te909/trmm-shared"
)

// Agent struct
type Agent struct {
	Hostname           string
	Arch               string
	AgentID            string
	BaseURL            string
	ApiURL             string
	Token              string
	AgentPK            int
	Cert               string
	ProgramDir         string
	EXE                string
	SystemDrive        string
	WinTmpDir          string
	WinRunAsUserTmpDir string
	MeshInstaller      string
	MeshSystemEXE      string
	MeshSVC            string
	PyBin              string
	PyVer              string
	PyBaseDir          string
	PyDir              string
	NuBin              string
	DenoBin            string
	AgentHeader        string
	Headers            map[string]string
	Logger             *logrus.Logger
	Version            string
	Debug              bool
	rClient            *resty.Client
	Proxy              string
	LogTo              string
	LogFile            *os.File
	Platform           string
	GoArch             string
	ServiceConfig      *service.Config
	NatsServer         string
	NatsProxyPath      string
	NatsProxyPort      string
	NatsPingInterval   int
	NatsWSCompression  bool
	Insecure           bool
}

const (
	progFilesName        = "TacticalAgent"
	winExeName           = "tacticalrmm.exe"
	winSvcName           = "tacticalrmm"
	meshSvcName          = "mesh agent"
	etcConfig            = "/etc/tacticalagent"
	nixAgentDir          = "/opt/tacticalagent"
	nixMeshDir           = "/opt/tacticalmesh"
	nixAgentBin          = nixAgentDir + "/tacticalagent"
	nixAgentBinDir       = nixAgentDir + "/bin"
	nixAgentEtcDir       = nixAgentDir + "/etc"
	nixMeshAgentBin      = nixMeshDir + "/meshagent"
	macPlistPath         = "/Library/LaunchDaemons/tacticalagent.plist"
	macPlistName         = "tacticalagent"
	defaultMacMeshSvcDir = "/usr/local/mesh_services"
)

var defaultWinTmpDir = filepath.Join(os.Getenv("PROGRAMDATA"), "TacticalRMM")
var winMeshDir = filepath.Join(os.Getenv("PROGRAMFILES"), "Mesh Agent")
var natsCheckin = []string{"agent-hello", "agent-agentinfo", "agent-disks", "agent-winsvc", "agent-publicip", "agent-wmi"}
var limitNatsData = []string{"agent-winsvc", "agent-wmi"}

func New(logger *logrus.Logger, version string) *Agent {
	host, _ := ps.Host()
	info := host.Info()
	pd := filepath.Join(os.Getenv("ProgramFiles"), progFilesName)
	exe := filepath.Join(pd, winExeName)
	sd := os.Getenv("SystemDrive")
	winTempDir := defaultWinTmpDir
	winRunAsUserTmpDir := defaultWinTmpDir

	hostname, err := os.Hostname()
	if err != nil {
		hostname = info.Hostname
	}

	pyver := "n/a"
	pybin := "n/a"
	pyBaseDir := "n/a"
	pydir := "n/a"

	if runtime.GOOS == "windows" {
		major := info.OS.Major
		minor := info.OS.Minor
		if major > 6 || (major == 6 && minor >= 3) {
			// Windows 8.1 or higher
			pyver = "3.11.9"
		} else {
			pyver = "3.8.7"
		}

		pydir = "py" + pyver + "_" + runtime.GOARCH
		pyBaseDir = filepath.Join(pd, "python")
		pybin = filepath.Join(pyBaseDir, pydir, "python.exe")
	}

	var nuBin string
	switch runtime.GOOS {
	case "windows":
		nuBin = filepath.Join(pd, "bin", "nu.exe")
	default:
		nuBin = filepath.Join(nixAgentBinDir, "nu")
	}

	var denoBin string
	switch runtime.GOOS {
	case "windows":
		denoBin = filepath.Join(pd, "bin", "deno.exe")
	default:
		denoBin = filepath.Join(nixAgentBinDir, "deno")
	}

	ac := NewAgentConfig()

	agentHeader := fmt.Sprintf("trmm/%s/%s/%s", version, runtime.GOOS, runtime.GOARCH)
	headers := make(map[string]string)
	if len(ac.Token) > 0 {
		headers["Content-Type"] = "application/json"
		headers["Authorization"] = fmt.Sprintf("Token %s", ac.Token)
	}

	insecure := ac.Insecure == "true"

	restyC := resty.New()
	restyC.SetBaseURL(ac.BaseURL)
	restyC.SetCloseConnection(true)
	restyC.SetHeaders(headers)
	restyC.SetTimeout(15 * time.Second)
	restyC.SetDebug(logger.IsLevelEnabled(logrus.DebugLevel))
	if insecure {
		insecureConf := &tls.Config{
			InsecureSkipVerify: true,
		}
		restyC.SetTLSClientConfig(insecureConf)
	}

	if len(ac.Proxy) > 0 {
		restyC.SetProxy(ac.Proxy)
	}
	if len(ac.Cert) > 0 {
		restyC.SetRootCertificate(ac.Cert)
	}

	if len(ac.WinTmpDir) > 0 {
		winTempDir = ac.WinTmpDir
	}

	if len(ac.WinRunAsUserTmpDir) > 0 {
		winRunAsUserTmpDir = ac.WinRunAsUserTmpDir
	}

	var MeshSysExe string
	switch runtime.GOOS {
	case "windows":
		if len(ac.CustomMeshDir) > 0 {
			MeshSysExe = filepath.Join(ac.CustomMeshDir, "MeshAgent.exe")
		} else {
			MeshSysExe = filepath.Join(os.Getenv("ProgramFiles"), "Mesh Agent", "MeshAgent.exe")
		}
	case "darwin":
		if trmm.FileExists(nixMeshAgentBin) {
			MeshSysExe = nixMeshAgentBin
		} else {
			MeshSysExe = "/usr/local/mesh_services/meshagent/meshagent"
		}
	default:
		MeshSysExe = nixMeshAgentBin
	}

	svcConf := &service.Config{
		Executable:  exe,
		Name:        winSvcName,
		DisplayName: "TacticalRMM Agent Service",
		Arguments:   []string{"-m", "svc"},
		Description: "TacticalRMM Agent Service",
		Option: service.KeyValue{
			"StartType":              "automatic",
			"OnFailure":              "restart",
			"OnFailureDelayDuration": "12s",
			"OnFailureResetPeriod":   10,
		},
	}

	var natsProxyPath, natsProxyPort string
	if ac.NatsProxyPath == "" {
		natsProxyPath = "natsws"
	}

	if ac.NatsProxyPort == "" {
		natsProxyPort = "443"
	}

	// check if using nats standard tcp, otherwise use nats websockets by default
	var natsServer string
	var natsWsCompression bool
	if ac.NatsStandardPort != "" {
		natsServer = fmt.Sprintf("tls://%s:%s", ac.APIURL, ac.NatsStandardPort)
	} else {
		natsServer = fmt.Sprintf("wss://%s:%s", ac.APIURL, natsProxyPort)
		natsWsCompression = true
	}

	var natsPingInterval int
	if ac.NatsPingInterval == 0 {
		natsPingInterval = randRange(35, 45)
	} else {
		natsPingInterval = ac.NatsPingInterval
	}

	return &Agent{
		Hostname:           hostname,
		BaseURL:            ac.BaseURL,
		AgentID:            ac.AgentID,
		ApiURL:             ac.APIURL,
		Token:              ac.Token,
		AgentPK:            ac.PK,
		Cert:               ac.Cert,
		ProgramDir:         pd,
		EXE:                exe,
		SystemDrive:        sd,
		WinTmpDir:          winTempDir,
		WinRunAsUserTmpDir: winRunAsUserTmpDir,
		MeshInstaller:      "meshagent.exe",
		MeshSystemEXE:      MeshSysExe,
		MeshSVC:            meshSvcName,
		PyBin:              pybin,
		PyVer:              pyver,
		PyBaseDir:          pyBaseDir,
		PyDir:              pydir,
		NuBin:              nuBin,
		DenoBin:            denoBin,
		Headers:            headers,
		AgentHeader:        agentHeader,
		Logger:             logger,
		Version:            version,
		Debug:              logger.IsLevelEnabled(logrus.DebugLevel),
		rClient:            restyC,
		Proxy:              ac.Proxy,
		Platform:           runtime.GOOS,
		GoArch:             runtime.GOARCH,
		ServiceConfig:      svcConf,
		NatsServer:         natsServer,
		NatsProxyPath:      natsProxyPath,
		NatsProxyPort:      natsProxyPort,
		NatsPingInterval:   natsPingInterval,
		NatsWSCompression:  natsWsCompression,
		Insecure:           insecure,
	}
}

type CmdStatus struct {
	Status gocmd.Status
	Stdout string
	Stderr string
}

type CmdOptions struct {
	Shell        string
	Command      string
	Args         []string
	Timeout      time.Duration
	IsScript     bool
	IsExecutable bool
	Detached     bool
	EnvVars      []string
}

func (a *Agent) NewCMDOpts() *CmdOptions {
	return &CmdOptions{
		Shell:   "/bin/bash",
		Timeout: 60,
	}
}

func (a *Agent) CmdV2(c *CmdOptions) CmdStatus {

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
		cmdOptions.BeforeExec = append(cmdOptions.BeforeExec, func(cmd *exec.Cmd) {
			cmd.SysProcAttr = SetDetached()
		})
	}

	if len(c.EnvVars) > 0 {
		cmdOptions.BeforeExec = append(cmdOptions.BeforeExec, func(cmd *exec.Cmd) {
			cmd.Env = os.Environ()
			cmd.Env = append(cmd.Env, c.EnvVars...)
		})
	}

	var envCmd *gocmd.Cmd
	if c.IsScript {
		envCmd = gocmd.NewCmdOptions(cmdOptions, c.Shell, c.Args...) // call script directly
	} else if c.IsExecutable {
		envCmd = gocmd.NewCmdOptions(cmdOptions, c.Shell, c.Command) // c.Shell: bin + c.Command: args as one string
	} else {
		commandArray := append(strings.Fields(c.Shell), "-c", c.Command)
		envCmd = gocmd.NewCmdOptions(cmdOptions, commandArray[0], commandArray[1:]...) // /bin/bash -c 'ls -l /var/log/...'
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
				a.Logger.Debugln(line)

			case line, open := <-envCmd.Stderr:
				if !open {
					envCmd.Stderr = nil
					continue
				}
				fmt.Fprintln(&stderrBuf, line)
				a.Logger.Debugln(line)
			}
		}
	}()

	statusChan := make(chan gocmd.Status, 1)
	// workaround for https://github.com/golang/go/issues/22315
	go func() {
		for i := 0; i < 5; i++ {
			finalStatus := <-envCmd.Start()
			if errors.Is(finalStatus.Error, syscall.ETXTBSY) {
				a.Logger.Errorln("CmdV2 syscall.ETXTBSY, retrying...")
				time.Sleep(500 * time.Millisecond)
				continue
			}
			statusChan <- finalStatus
			return
		}
	}()

	var finalStatus gocmd.Status

	select {
	case <-ctx.Done():
		a.Logger.Debugf("Command timed out after %d seconds\n", c.Timeout)
		pid := envCmd.Status().PID
		a.Logger.Debugln("Killing process with PID", pid)
		KillProc(int32(pid))
		finalStatus.Exit = 98
		ret := CmdStatus{
			Status: finalStatus,
			Stdout: CleanString(stdoutBuf.String()),
			Stderr: fmt.Sprintf("%s\nTimed out after %d seconds", CleanString(stderrBuf.String()), c.Timeout),
		}
		a.Logger.Debugf("%+v\n", ret)
		return ret
	case finalStatus = <-statusChan:
		// done
	}

	// Wait for goroutine to print everything
	<-doneChan

	ret := CmdStatus{
		Status: finalStatus,
		Stdout: CleanString(stdoutBuf.String()),
		Stderr: CleanString(stderrBuf.String()),
	}
	a.Logger.Debugf("%+v\n", ret)
	return ret
}

func (a *Agent) GetCPULoadAvg() int {
	fallback := false
	pyCode := `
import psutil
try:
	print(int(round(psutil.cpu_percent(interval=10))), end='')
except:
	print("pyerror", end='')
`
	pypercent, err := a.RunPythonCode(pyCode, 13, []string{})
	if err != nil || pypercent == "pyerror" {
		fallback = true
	}

	i, err := strconv.Atoi(pypercent)
	if err != nil {
		fallback = true
	}

	if fallback {
		percent, err := cpu.Percent(10*time.Second, false)
		if err != nil {
			a.Logger.Debugln("Go CPU Check:", err)
			return 0
		}
		return int(math.Round(percent[0]))
	}
	return i
}

// ForceKillMesh kills all mesh agent related processes
func (a *Agent) ForceKillMesh() {
	pids := make([]int, 0)

	procs, err := ps.Processes()
	if err != nil {
		return
	}

	for _, process := range procs {
		p, err := process.Info()
		if err != nil {
			continue
		}
		if strings.Contains(strings.ToLower(p.Name), "meshagent") {
			pids = append(pids, p.PID)
		}
	}

	for _, pid := range pids {
		a.Logger.Debugln("Killing mesh process with pid:", pid)
		if err := KillProc(int32(pid)); err != nil {
			a.Logger.Debugln(err)
		}
	}
}

func (a *Agent) SyncMeshNodeID() {

	id, err := a.getMeshNodeID()
	if err != nil {
		a.Logger.Errorln("SyncMeshNodeID() getMeshNodeID()", err)
		return
	}

	payload := rmm.MeshNodeID{
		Func:    "syncmesh",
		Agentid: a.AgentID,
		NodeID:  StripAll(id),
	}

	_, err = a.rClient.R().SetBody(payload).Post("/api/v3/syncmesh/")
	if err != nil {
		a.Logger.Debugln("SyncMesh:", err)
	}
}

func (a *Agent) setupNatsOptions() []nats.Option {
	reconnectWait := randRange(2, 8)
	opts := make([]nats.Option, 0)
	opts = append(opts, nats.Name(a.AgentID))
	opts = append(opts, nats.UserInfo(a.AgentID, a.Token))
	opts = append(opts, nats.ReconnectWait(time.Duration(reconnectWait)*time.Second))
	opts = append(opts, nats.RetryOnFailedConnect(true))
	opts = append(opts, nats.IgnoreAuthErrorAbort())
	opts = append(opts, nats.PingInterval(time.Duration(a.NatsPingInterval)*time.Second))
	opts = append(opts, nats.Compression(a.NatsWSCompression))
	opts = append(opts, nats.MaxReconnects(-1))
	opts = append(opts, nats.ReconnectBufSize(-1))
	opts = append(opts, nats.ProxyPath(a.NatsProxyPath))
	opts = append(opts, nats.ReconnectJitter(500*time.Millisecond, 4*time.Second))
	opts = append(opts, nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
		a.Logger.Debugln("NATS disconnected:", err)
		a.Logger.Debugf("%+v\n", nc.Statistics)
	}))
	opts = append(opts, nats.ReconnectHandler(func(nc *nats.Conn) {
		a.Logger.Debugln("NATS reconnected")
		a.Logger.Debugf("%+v\n", nc.Statistics)
	}))
	opts = append(opts, nats.ErrorHandler(func(nc *nats.Conn, sub *nats.Subscription, err error) {
		a.Logger.Errorln("NATS error:", err)
		a.Logger.Errorf("%+v\n", sub)
	}))
	if a.Insecure {
		insecureConf := &tls.Config{
			InsecureSkipVerify: true,
		}
		opts = append(opts, nats.Secure(insecureConf))
	}
	return opts
}

func (a *Agent) GetUninstallExe() string {
	cderr := os.Chdir(a.ProgramDir)
	if cderr == nil {
		files, err := filepath.Glob("unins*.exe")
		if err == nil {
			for _, f := range files {
				if strings.Contains(f, "001") {
					return f
				}
			}
		}
	}
	return "unins000.exe"
}

func (a *Agent) CleanupAgentUpdates() {
	// TODO remove a.ProgramDir, updates are now in winTempDir
	dirs := [3]string{a.WinTmpDir, os.Getenv("TMP"), a.ProgramDir}
	for _, dir := range dirs {
		err := os.Chdir(dir)
		if err != nil {
			a.Logger.Debugln("CleanupAgentUpdates()", dir, err)
			continue
		}

		// TODO winagent-v* is deprecated
		globs := [3]string{"tacticalagent-v*", "is-*.tmp", "winagent-v*"}
		for _, glob := range globs {
			files, err := filepath.Glob(glob)
			if err == nil {
				for _, f := range files {
					a.Logger.Debugln("CleanupAgentUpdates() Removing file:", f)
					os.Remove(f)
				}
			}
		}
	}

	err := os.Chdir(os.Getenv("TMP"))
	if err == nil {
		dirs, err := filepath.Glob("tacticalrmm*")
		if err == nil {
			for _, f := range dirs {
				os.RemoveAll(f)
			}
		}
	}
}

func (a *Agent) RunPythonCode(code string, timeout int, args []string) (string, error) {
	content := []byte(code)
	tmpfn, _ := os.CreateTemp(a.WinTmpDir, "*.py")
	if _, err := tmpfn.Write(content); err != nil {
		a.Logger.Debugln(err)
		return "", err
	}
	defer os.Remove(tmpfn.Name())
	if err := tmpfn.Close(); err != nil {
		a.Logger.Debugln(err)
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	var outb, errb bytes.Buffer
	cmdArgs := []string{tmpfn.Name()}
	if len(args) > 0 {
		cmdArgs = append(cmdArgs, args...)
	}
	a.Logger.Debugln(cmdArgs)
	cmd := exec.CommandContext(ctx, a.PyBin, cmdArgs...)
	cmd.Stdout = &outb
	cmd.Stderr = &errb

	cmdErr := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		a.Logger.Debugln("RunPythonCode:", ctx.Err())
		return "", ctx.Err()
	}

	if cmdErr != nil {
		a.Logger.Debugln("RunPythonCode:", cmdErr)
		return "", cmdErr
	}

	if errb.String() != "" {
		a.Logger.Debugln(errb.String())
		return errb.String(), errors.New("RunPythonCode stderr")
	}

	return outb.String(), nil

}

func createWinTempDir() error {
	if !trmm.FileExists(defaultWinTmpDir) {
		err := os.Mkdir(defaultWinTmpDir, 0775)
		if err != nil {
			return err
		}
	}
	return nil
}
