package service

import (
	"runtime"

	"github.com/amidaware/rmmagent/agent/disk"
	"github.com/amidaware/rmmagent/agent/network"
	"github.com/amidaware/rmmagent/agent/services"
	"github.com/amidaware/rmmagent/agent/system"
	"github.com/amidaware/rmmagent/agent/tactical/config"
	"github.com/amidaware/rmmagent/agent/wmi"
	"github.com/nats-io/nats.go"
	"github.com/ugorji/go/codec"
)

func NatsMessage(version string, nc *nats.Conn, mode string) {
	config := config.NewAgentConfig()
	var resp []byte
	var payload interface{}
	ret := codec.NewEncoderBytes(&resp, new(codec.MsgpackHandle))

	switch mode {
	case "agent-hello":
		payload = CheckInNats{
			Agentid: config.AgentID,
			Version: version,
		}
	case "agent-winsvc":
		svcs, _, _ := services.GetServices()
		payload = WinSvcNats{
			Agentid: config.AgentID,
			WinSvcs: svcs,
		}
	case "agent-agentinfo":
		osinfo := system.OsString()
		reboot, err := system.SystemRebootRequired()
		if err != nil {
			reboot = false
		}
		payload = AgentInfoNats{
			Agentid:      config.AgentID,
			Username:     system.LoggedOnUser(),
			Hostname:     system.GetHostname(),
			OS:           osinfo,
			Platform:     runtime.GOOS,
			TotalRAM:     system.TotalRAM(),
			BootTime:     system.BootTime(),
			RebootNeeded: reboot,
			GoArch:       runtime.GOARCH,
		}
	case "agent-wmi":
		wmiinfo, _ := wmi.GetWMIInfo()
		payload = WinWMINats{
			Agentid: config.AgentID,
			WMI:     wmiinfo,
		}
	case "agent-disks":
		disks, _ := disk.GetDisks()
		payload = WinDisksNats{
			Agentid: config.AgentID,
			Disks:   disks,
		}
	case "agent-publicip":
		payload = PublicIPNats{
			Agentid:  config.AgentID,
			PublicIP: network.PublicIP(config.Proxy),
		}
	}

	ret.Encode(payload)
	nc.PublishRequest(config.AgentID, mode, resp)
}