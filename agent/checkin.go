/*
Copyright 2023 AmidaWare Inc.

Licensed under the Tactical RMM License Version 1.0 (the “License”).
You may only use the Licensed Software in accordance with the License.
A copy of the License is available at:

https://license.tacticalrmm.com

*/

package agent

import (
	"runtime"
	"time"

	nats "github.com/nats-io/nats.go"
	"github.com/ugorji/go/codec"
	trmm "github.com/wh1te909/trmm-shared"
)

func (a *Agent) NatsMessage(nc *nats.Conn, mode string) {
	var resp []byte
	var payload interface{}
	ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))

	switch mode {
	case "agent-hello":
		payload = trmm.CheckInNats{
			Agentid: a.AgentID,
			Version: a.Version,
		}
	case "agent-winsvc":
		payload = trmm.WinSvcNats{
			Agentid: a.AgentID,
			WinSvcs: a.GetServices(),
		}
	case "agent-agentinfo":
		osinfo := a.osString()
		reboot, err := a.SystemRebootRequired()
		if err != nil {
			reboot = false
		}
		payload = trmm.AgentInfoNats{
			Agentid:      a.AgentID,
			Username:     a.LoggedOnUser(),
			Hostname:     a.Hostname,
			OS:           osinfo,
			Platform:     runtime.GOOS,
			TotalRAM:     a.TotalRAM(),
			BootTime:     a.BootTime(),
			RebootNeeded: reboot,
			GoArch:       a.GoArch,
		}
	case "agent-wmi":
		payload = trmm.WinWMINats{
			Agentid: a.AgentID,
			WMI:     a.GetWMIInfo(),
		}
	case "agent-disks":
		payload = trmm.WinDisksNats{
			Agentid: a.AgentID,
			Disks:   a.GetDisks(),
		}
	case "agent-publicip":
		payload = trmm.PublicIPNats{
			Agentid:  a.AgentID,
			PublicIP: a.PublicIP(),
		}
	}

	a.Logger.Debugln(mode, payload)
	ret.Encode(payload)
	nc.PublishRequest(a.AgentID, mode, resp)
}

func (a *Agent) DoNatsCheckIn() {
	opts := a.setupNatsOptions()
	nc, err := nats.Connect(a.NatsServer, opts...)
	if err != nil {
		a.Logger.Errorln(err)
		return
	}

	for _, s := range natsCheckin {
		time.Sleep(time.Duration(randRange(100, 400)) * time.Millisecond)
		a.NatsMessage(nc, s)
	}
	nc.Close()
}
