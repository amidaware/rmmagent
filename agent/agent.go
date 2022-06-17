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
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"time"

	rmm "github.com/amidaware/rmmagent/shared"
	ps "github.com/elastic/go-sysinfo"
	"github.com/go-resty/resty/v2"
	"github.com/kardianos/service"
	nats "github.com/nats-io/nats.go"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/sirupsen/logrus"
	trmm "github.com/wh1te909/trmm-shared"
)

const (
	progFilesName = "TacticalAgent"
	winExeName    = "tacticalrmm.exe"
	winSvcName    = "tacticalrmm"
	meshSvcName   = "mesh agent"
)

var natsCheckin = []string{"agent-hello", "agent-agentinfo", "agent-disks", "agent-winsvc", "agent-publicip", "agent-wmi"}

func New(logger *logrus.Logger, version string) *Agent {
	host, _ := ps.Host()
	info := host.Info()
	pd := filepath.Join(os.Getenv("ProgramFiles"), progFilesName)
	exe := filepath.Join(pd, winExeName)
	sd := os.Getenv("SystemDrive")

	var pybin string
	switch runtime.GOARCH {
	case "amd64":
		pybin = filepath.Join(pd, "py38-x64", "python.exe")
	case "386":
		pybin = filepath.Join(pd, "py38-x32", "python.exe")
	}

	ac := NewAgentConfig()

	headers := make(map[string]string)
	if len(ac.Token) > 0 {
		headers["Content-Type"] = "application/json"
		headers["Authorization"] = fmt.Sprintf("Token %s", ac.Token)
	}

	restyC := resty.New()
	restyC.SetBaseURL(ac.BaseURL)
	restyC.SetCloseConnection(true)
	restyC.SetHeaders(headers)
	restyC.SetTimeout(15 * time.Second)
	restyC.SetDebug(logger.IsLevelEnabled(logrus.DebugLevel))

	if len(ac.Proxy) > 0 {
		restyC.SetProxy(ac.Proxy)
	}
	if len(ac.Cert) > 0 {
		restyC.SetRootCertificate(ac.Cert)
	}

	var MeshSysBin string
	if len(ac.CustomMeshDir) > 0 {
		MeshSysBin = filepath.Join(ac.CustomMeshDir, "MeshAgent.exe")
	} else {
		MeshSysBin = filepath.Join(os.Getenv("ProgramFiles"), "Mesh Agent", "MeshAgent.exe")
	}

	if runtime.GOOS == "linux" {
		MeshSysBin = "/opt/tacticalmesh/meshagent"
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
			"OnFailureDelayDuration": "5s",
			"OnFailureResetPeriod":   10,
		},
	}

	return &Agent{
		Hostname:      info.Hostname,
		Arch:          info.Architecture,
		BaseURL:       ac.BaseURL,
		AgentID:       ac.AgentID,
		ApiURL:        ac.APIURL,
		Token:         ac.Token,
		AgentPK:       ac.PK,
		Cert:          ac.Cert,
		ProgramDir:    pd,
		EXE:           exe,
		SystemDrive:   sd,
		MeshInstaller: "meshagent.exe",
		MeshSystemBin: MeshSysBin,
		MeshSVC:       meshSvcName,
		PyBin:         pybin,
		Headers:       headers,
		Logger:        logger,
		Version:       version,
		Debug:         logger.IsLevelEnabled(logrus.DebugLevel),
		rClient:       restyC,
		Proxy:         ac.Proxy,
		Platform:      runtime.GOOS,
		GoArch:        runtime.GOARCH,
		ServiceConfig: svcConf,
	}
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
		a.Logger.Debugln("Killing mesh process with pid %d", pid)
		if err := utils.KillProc(int32(pid)); err != nil {
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




func (a *Agent) CreateTRMMTempDir() {
	// create the temp dir for running scripts
	dir := filepath.Join(os.TempDir(), "trmm")
	if !trmm.FileExists(dir) {
		err := os.Mkdir(dir, 0775)
		if err != nil {
			a.Logger.Errorln(err)
		}
	}
}

func (a *Agent) GetDisks() []trmm.Disk {
	return disk.GetDisks()
}