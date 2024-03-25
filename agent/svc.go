/*
Copyright 2023 AmidaWare Inc.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

import (
	"fmt"
	"runtime"
	"sync"
	"time"

	nats "github.com/nats-io/nats.go"
)

func (a *Agent) RunAsService(nc *nats.Conn) {
	var wg sync.WaitGroup
	wg.Add(1)
	go a.AgentSvc(nc)
	go a.CheckRunner()
	wg.Wait()
}

type AgentCheckInConfig struct {
	Hello                  int    `json:"checkin_hello"`
	AgentInfo              int    `json:"checkin_agentinfo"`
	WinSvc                 int    `json:"checkin_winsvc"`
	PubIP                  int    `json:"checkin_pubip"`
	Disks                  int    `json:"checkin_disks"`
	SW                     int    `json:"checkin_sw"`
	WMI                    int    `json:"checkin_wmi"`
	SyncMesh               int    `json:"checkin_syncmesh"`
	LimitData              bool   `json:"limit_data"`
	InstallNushell         bool   `json:"install_nushell"`
	InstallNushellVersion  string `json:"install_nushell_version"`
	InstallNushellUrl      string `json:"install_nushell_url"`
	NushellEnableConfig    bool   `json:"nushell_enable_config"`
	InstallDeno            bool   `json:"install_deno"`
	InstallDenoVersion     string `json:"install_deno_version"`
	InstallDenoUrl         string `json:"install_deno_url"`
	DenoDefaultPermissions string `json:"deno_default_permissions"`
}

func (a *Agent) AgentSvc(nc *nats.Conn) {
	if runtime.GOOS == "windows" {
		go a.GetPython(false)

		err := createWinTempDir()
		if err != nil {
			a.Logger.Errorln("AgentSvc() createWinTempDir():", err)
		}
	}

	a.RunMigrations()

	sleepDelay := randRange(7, 25)
	a.Logger.Debugf("AgentSvc() sleeping for %v seconds", sleepDelay)
	time.Sleep(time.Duration(sleepDelay) * time.Second)

	if runtime.GOOS == "windows" {
		a.KillHungUpdates()
		time.Sleep(1 * time.Second)
		a.CleanupAgentUpdates()
	}

	// Windows has GetAgentCheckInConfig() while unix has a stub GetAgentCheckInConfig()
	conf := a.GetAgentCheckInConfig(a.GetCheckInConfFromAPI())
	a.Logger.Debugf("AgentCheckInConf: %+v\n", conf)
	for _, s := range natsCheckin {
		if conf.LimitData && stringInSlice(s, limitNatsData) {
			continue
		} else {
			a.NatsMessage(nc, s)
			time.Sleep(time.Duration(randRange(100, 400)) * time.Millisecond)
		}
	}

	// The server conf check is also done in the functions to keep the parameters the same.
	// Don't force a download when restarting the service.
	if conf.InstallNushell {
		go a.InstallNushell(false)
	}
	if conf.InstallDeno {
		go a.InstallDeno(false)
	}

	go a.SyncMeshNodeID()

	time.Sleep(time.Duration(randRange(1, 3)) * time.Second)
	if runtime.GOOS == "windows" && !conf.LimitData {
		a.AgentStartup()
		a.SendSoftware()
	}

	if runtime.GOOS == "darwin" {
		go a.FixVenturaMesh()
	}

	checkInHelloTicker := time.NewTicker(time.Duration(conf.Hello) * time.Second)
	checkInAgentInfoTicker := time.NewTicker(time.Duration(conf.AgentInfo) * time.Second)
	checkInWinSvcTicker := time.NewTicker(time.Duration(conf.WinSvc) * time.Second)
	checkInPubIPTicker := time.NewTicker(time.Duration(conf.PubIP) * time.Second)
	checkInDisksTicker := time.NewTicker(time.Duration(conf.Disks) * time.Second)
	checkInSWTicker := time.NewTicker(time.Duration(conf.SW) * time.Second)
	checkInWMITicker := time.NewTicker(time.Duration(conf.WMI) * time.Second)
	syncMeshTicker := time.NewTicker(time.Duration(conf.SyncMesh) * time.Second)

	for {
		select {
		case <-checkInHelloTicker.C:
			a.NatsMessage(nc, "agent-hello")
		case <-checkInAgentInfoTicker.C:
			a.NatsMessage(nc, "agent-agentinfo")
		case <-checkInWinSvcTicker.C:
			a.NatsMessage(nc, "agent-winsvc")
		case <-checkInPubIPTicker.C:
			a.NatsMessage(nc, "agent-publicip")
		case <-checkInDisksTicker.C:
			a.NatsMessage(nc, "agent-disks")
		case <-checkInSWTicker.C:
			a.SendSoftware()
		case <-checkInWMITicker.C:
			a.NatsMessage(nc, "agent-wmi")
		case <-syncMeshTicker.C:
			a.SyncMeshNodeID()
		}
	}
}

func (a *Agent) AgentStartup() {
	url := "/api/v3/checkin/"
	payload := map[string]interface{}{"agent_id": a.AgentID}
	_, err := a.rClient.R().SetBody(payload).Post(url)
	if err != nil {
		a.Logger.Debugln("AgentStartup()", err)
	}
}

func (a *Agent) GetCheckInConfFromAPI() AgentCheckInConfig {
	ret := AgentCheckInConfig{}
	url := fmt.Sprintf("/api/v3/%s/config/", a.AgentID)
	r, err := a.rClient.R().SetResult(&AgentCheckInConfig{}).Get(url)
	if err != nil {
		a.Logger.Debugln("GetAgentCheckInConfig()", err)
		ret.Hello = randRange(30, 60)
		ret.AgentInfo = randRange(200, 400)
		ret.WinSvc = randRange(2400, 3000)
		ret.PubIP = randRange(300, 500)
		ret.Disks = randRange(1000, 2000)
		ret.SW = randRange(2800, 3500)
		ret.WMI = randRange(3000, 4000)
		ret.SyncMesh = randRange(800, 1200)
		ret.LimitData = false
		ret.InstallNushell = false
		ret.InstallNushellVersion = ""
		ret.InstallNushellUrl = ""
		ret.NushellEnableConfig = false
		ret.InstallDeno = false
		ret.InstallDenoVersion = ""
		ret.InstallDenoUrl = ""
		ret.DenoDefaultPermissions = ""
	} else {
		ret.Hello = r.Result().(*AgentCheckInConfig).Hello
		ret.AgentInfo = r.Result().(*AgentCheckInConfig).AgentInfo
		ret.WinSvc = r.Result().(*AgentCheckInConfig).WinSvc
		ret.PubIP = r.Result().(*AgentCheckInConfig).PubIP
		ret.Disks = r.Result().(*AgentCheckInConfig).Disks
		ret.SW = r.Result().(*AgentCheckInConfig).SW
		ret.WMI = r.Result().(*AgentCheckInConfig).WMI
		ret.SyncMesh = r.Result().(*AgentCheckInConfig).SyncMesh
		ret.LimitData = r.Result().(*AgentCheckInConfig).LimitData
		ret.InstallNushell = r.Result().(*AgentCheckInConfig).InstallNushell
		ret.InstallNushellVersion = r.Result().(*AgentCheckInConfig).InstallNushellVersion
		ret.InstallNushellUrl = r.Result().(*AgentCheckInConfig).InstallNushellUrl
		ret.NushellEnableConfig = r.Result().(*AgentCheckInConfig).NushellEnableConfig
		ret.InstallDeno = r.Result().(*AgentCheckInConfig).InstallDeno
		ret.InstallDenoVersion = r.Result().(*AgentCheckInConfig).InstallDenoVersion
		ret.InstallDenoUrl = r.Result().(*AgentCheckInConfig).InstallDenoUrl
		ret.DenoDefaultPermissions = r.Result().(*AgentCheckInConfig).DenoDefaultPermissions
	}
	return ret
}
