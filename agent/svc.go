/*
Copyright 2022 AmidaWare LLC.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

import (
	"runtime"
	"sync"
	"time"

	nats "github.com/nats-io/nats.go"
)

func (a *Agent) RunAsService() {
	var wg sync.WaitGroup
	wg.Add(1)
	go a.AgentSvc()
	go a.CheckRunner()
	wg.Wait()
}

func (a *Agent) AgentSvc() {
	if runtime.GOOS == "windows" {
		go a.GetPython(false)

		err := createWinTempDir()
		if err != nil {
			a.Logger.Errorln("AgentSvc() createWinTempDir():", err)
		}
	}
	a.RunMigrations()

	sleepDelay := randRange(14, 22)
	a.Logger.Debugf("AgentSvc() sleeping for %v seconds", sleepDelay)
	time.Sleep(time.Duration(sleepDelay) * time.Second)

	opts := a.setupNatsOptions()
	nc, err := nats.Connect(a.NatsServer, opts...)
	if err != nil {
		a.Logger.Fatalln("AgentSvc() nats.Connect()", err)
	}

	for _, s := range natsCheckin {
		a.NatsMessage(nc, s)
		time.Sleep(time.Duration(randRange(100, 400)) * time.Millisecond)
	}

	go a.SyncMeshNodeID()

	time.Sleep(time.Duration(randRange(1, 3)) * time.Second)
	if runtime.GOOS == "windows" {
		a.AgentStartup()
		a.SendSoftware()
	}

	checkInHelloTicker := time.NewTicker(time.Duration(randRange(30, 60)) * time.Second)
	checkInAgentInfoTicker := time.NewTicker(time.Duration(randRange(200, 400)) * time.Second)
	checkInWinSvcTicker := time.NewTicker(time.Duration(randRange(2400, 3000)) * time.Second)
	checkInPubIPTicker := time.NewTicker(time.Duration(randRange(300, 500)) * time.Second)
	checkInDisksTicker := time.NewTicker(time.Duration(randRange(1000, 2000)) * time.Second)
	checkInSWTicker := time.NewTicker(time.Duration(randRange(2800, 3500)) * time.Second)
	checkInWMITicker := time.NewTicker(time.Duration(randRange(3000, 4000)) * time.Second)
	syncMeshTicker := time.NewTicker(time.Duration(randRange(800, 1200)) * time.Second)

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
